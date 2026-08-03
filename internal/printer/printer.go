package printer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/l33t0/bambu-notifier/internal/camera"
	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/l33t0/bambu-notifier/internal/notifier"
)

var pushallPayload = []byte(
	`{"pushing":{"sequence_id":"0","command":"pushall"}}`,
)

// Printer manages a single MQTT connection to a Bambu printer.
type Printer interface {
	Start(ctx context.Context) error
	Stop()
}

// Config holds configuration for a single Bambu printer connection.
type Config struct {
	Name                  string
	Host                  string
	Port                  int
	SerialNumber          string
	AccessCode            string
	CameraPort            int
	CameraRTSP            bool
	TLSInsecureSkipVerify bool
	ReconnectDelaySeconds int
	TailMode              bool
}

// printer implements the Printer interface.
type printer struct {
	cfg       Config
	notifiers []notifier.Notifier
	logger    *slog.Logger
	tracker   *stateTracker

	msgCount         atomic.Uint64
	snapshotDisabled atomic.Bool

	// rtspURL is the last stream URL the printer advertised in a
	// report. Only touched from handleMessage, which runs on the
	// single readLoop goroutine.
	rtspURL string

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var _ Printer = (*printer)(nil)

// New creates a new Printer for the given config, fanning out events
// to notifiers.
func New(
	cfg Config,
	notifiers []notifier.Notifier,
	logger *slog.Logger,
) Printer {
	if logger == nil {
		logger = slog.Default()
	}

	return &printer{
		cfg:       cfg,
		notifiers: notifiers,
		logger:    logger.With("printer", cfg.Name),
		tracker:   newStateTracker(cfg.Name),
	}
}

// Start connects to the printer's MQTT broker and begins listening
// for reports. It blocks until the context is cancelled.
func (p *printer) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.runLoop(ctx)
	}()

	return nil
}

// Stop cancels the printer loop and waits for it to finish.
func (p *printer) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

func (p *printer) runLoop(ctx context.Context) {
	baseDelay := time.Duration(
		p.cfg.ReconnectDelaySeconds,
	) * time.Second
	maxDelay := 5 * time.Minute
	attempt := 0

	for {
		if ctx.Err() != nil {
			return
		}

		client := p.newClient()

		p.logger.Info("connecting to printer",
			"host", p.cfg.Host, "port", p.cfg.Port,
		)
		if err := client.connect(); err != nil {
			p.logger.Error("mqtt connect failed",
				"error", err, "attempt", attempt,
			)
			p.backoff(ctx, baseDelay, maxDelay, attempt)
			attempt++
			continue
		}

		p.logger.Info("connected, subscribing to reports")
		attempt = 0

		topic := fmt.Sprintf(
			"device/%s/report", p.cfg.SerialNumber,
		)
		if err := client.subscribe(topic); err != nil {
			p.logger.Error("mqtt subscribe failed", "error", err)
			_ = client.close()
			p.backoff(ctx, baseDelay, maxDelay, 0)
			continue
		}

		reqTopic := fmt.Sprintf(
			"device/%s/request", p.cfg.SerialNumber,
		)
		if err := client.publish(reqTopic, pushallPayload); err != nil {
			p.logger.Warn("failed to send pushall", "error", err)
		}

		p.tracker.reset()

		// Per-connection context so keepAlive exits when
		// readLoop returns.
		connCtx, connCancel := context.WithCancel(ctx)

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.keepAlive(connCtx, client)
		}()

		connStart := time.Now()
		msgsBefore := p.msgCount.Load()

		if err := client.readLoop(); err != nil {
			p.logger.Error("mqtt read loop exited", "error", err)
		}

		if p.msgCount.Load() == msgsBefore &&
			time.Since(connStart) < 10*time.Second {
			p.logger.Warn(
				"printer dropped the connection right after " +
					"subscribe without sending any reports — " +
					"if this is an X1 series, enable LAN Mode " +
					"Developer Mode in the printer's network " +
					"settings (required by newer firmware for " +
					"third-party access); also verify the " +
					"access code matches the printer LCD",
			)
		}

		connCancel()
		_ = client.close()

		if ctx.Err() != nil {
			return
		}

		p.logger.Info("reconnecting after disconnect")
		p.backoff(ctx, baseDelay, maxDelay, 0)
	}
}

func (p *printer) newClient() *mqttClient {
	opts := make([]mqttOption, 0, 4)
	opts = append(opts,
		withCredentials("bblp", p.cfg.AccessCode),
		withOnPublish(p.handleMessage),
		withLogger(p.logger),
	)

	tlsCfg := &tls.Config{
		InsecureSkipVerify: p.cfg.TLSInsecureSkipVerify, //nolint:gosec // intentional: Bambu printers use self-signed certificates
	}
	opts = append(opts, withTLSConfig(tlsCfg))

	return newMQTTClient(
		p.cfg.Host, p.cfg.Port, mqttClientID(), opts...,
	)
}

// mqttClientID returns a unique MQTT client identifier. The printer
// serial must NOT be used here: X1-series firmware's own internal
// MQTT client appears to identify with the serial on the embedded
// broker, so connecting with the same id triggers an MQTT session
// takeover fight — the printer kicks our connection a couple of
// seconds after we connect, over and over. A random id (like
// ha-bambulab's "ha-bambulab-<uuid>") coexists fine.
func mqttClientID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf(
			"bambu-notifier-%d", time.Now().UnixNano(),
		)
	}
	return "bambu-notifier-" + hex.EncodeToString(b[:])
}

func (p *printer) handleMessage(_ string, payload []byte) {
	p.msgCount.Add(1)

	report, err := parseReport(payload)
	if err != nil {
		p.logger.Debug("ignoring unparseable report", "error", err)
		return
	}

	if p.cfg.TailMode {
		p.logReport(report)
	}

	events := p.tracker.update(report)
	if len(events) == 0 {
		return
	}

	if p.cfg.TailMode {
		for _, evt := range events {
			p.logger.Info("EVENT",
				"type", evt.Type,
				"file", evt.Filename,
				"progress", fmt.Sprintf("%d%%", evt.Progress),
			)
		}
	}

	if report.RTSPUrl != "" {
		p.rtspURL = report.RTSPUrl
	}

	if p.snapshotEnabled() && hasSnapshotEvent(events) {
		snap, snapErr := p.captureSnapshot()
		if snapErr != nil {
			p.handleSnapshotError(snapErr)
		} else {
			for i := range events {
				events[i].Snapshot = snap
			}
		}
	}

	for _, evt := range events {
		p.fanOut(evt)
	}
}

func (p *printer) snapshotEnabled() bool {
	if p.snapshotDisabled.Load() {
		return false
	}
	return p.cfg.CameraRTSP || p.cfg.CameraPort > 0
}

// captureSnapshot grabs a JPEG using whichever camera protocol this
// printer is configured for: the P1/A1 chamber-image service on
// camera_port, or an ffmpeg frame-grab from the RTSPS stream.
func (p *printer) captureSnapshot() ([]byte, error) {
	if p.cfg.CameraRTSP {
		return camera.CaptureRTSPFrame(
			p.cfg.Host, p.cfg.AccessCode, p.rtspURL,
		)
	}
	return camera.CaptureFrame(
		p.cfg.Host, p.cfg.CameraPort,
		p.cfg.AccessCode, p.cfg.TLSInsecureSkipVerify,
	)
}

func (p *printer) handleSnapshotError(err error) {
	msg := err.Error()

	switch {
	case !p.cfg.CameraRTSP &&
		strings.Contains(msg, "handshake failure"):
		// The printer refused the TLS handshake outright: this
		// model does not speak the P1/A1 JPEG chamber-image
		// protocol, so retrying is pointless.
		p.snapshotDisabled.Store(true)
		p.logger.Warn(
			"camera snapshots disabled: printer rejected the "+
				"TLS handshake — the camera_port JPEG protocol "+
				"only exists on P1/A1-series printers; for "+
				"X1/H2 series set camera_rtsp instead",
			"camera_port", p.cfg.CameraPort,
		)
	case strings.Contains(msg, "ffmpeg not found"):
		p.snapshotDisabled.Store(true)
		p.logger.Warn(
			"camera snapshots disabled: ffmpeg is not " +
				"installed (required for RTSP frame grabs)",
		)
	case p.cfg.CameraRTSP:
		p.logger.Warn("camera snapshot failed — check that "+
			"LAN Mode Liveview is enabled on the printer",
			"error", err,
		)
	default:
		p.logger.Warn("camera snapshot failed", "error", err)
	}
}

func hasSnapshotEvent(events []event.Event) bool {
	for _, e := range events {
		switch e.Type {
		case event.PrintStarted, event.PrintFinished,
			event.PrintFailed, event.PrintPaused:
			return true
		}
	}
	return false
}

func (p *printer) logReport(r *parsedReport) {
	attrs := []any{
		"state", r.GcodeState,
		"file", r.Filename,
		"progress", fmt.Sprintf("%d%%", r.Progress),
		"eta", r.ETA,
		"nozzle", fmt.Sprintf("%.1f/%.1f°C",
			r.Thermals.NozzleActual, r.Thermals.NozzleTarget),
		"bed", fmt.Sprintf("%.1f/%.1f°C",
			r.Thermals.BedActual, r.Thermals.BedTarget),
	}
	if r.NozzleType != "" {
		attrs = append(attrs, "nozzle_type", r.NozzleType)
	}
	if r.ErrorCode != 0 {
		attrs = append(attrs, "error_code", r.ErrorCode)
	}
	if r.ErrorMsg != "" {
		attrs = append(attrs, "error_msg", r.ErrorMsg)
	}
	if len(r.HMSErrors) > 0 {
		attrs = append(attrs, "hms", r.HMSErrors)
	}
	if len(r.AMSSlots) > 0 {
		attrs = append(attrs, "ams_slots", len(r.AMSSlots))
	}
	p.logger.Info("report", attrs...)
}

func (p *printer) fanOut(evt event.Event) {
	var wg sync.WaitGroup
	for _, n := range p.notifiers {
		wg.Add(1)
		go func(n notifier.Notifier) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(
				context.Background(), 15*time.Second,
			)
			defer cancel()
			if err := n.Send(ctx, evt); err != nil {
				p.logger.Error("notifier send failed",
					"notifier", n.Name(),
					"event", evt.Type,
					"error", err,
				)
			}
		}(n)
	}
	wg.Wait()
}

func (p *printer) keepAlive(
	ctx context.Context,
	client *mqttClient,
) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = client.close() // unblock readLoop
			return
		case <-ticker.C:
			if err := client.ping(); err != nil {
				p.logger.Warn("ping failed", "error", err)
				_ = client.close()
				return
			}
		}
	}
}

func (p *printer) backoff(
	ctx context.Context,
	base, max time.Duration,
	attempt int,
) {
	delay := time.Duration(
		float64(base) * math.Pow(2, float64(attempt)),
	)
	if delay > max {
		delay = max
	}

	p.logger.Debug("backoff", "delay", delay, "attempt", attempt)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
