package printer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

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

// PrinterConfig holds configuration for a single Bambu printer
// connection.
type PrinterConfig struct {
	Name                  string
	Host                  string
	Port                  int
	SerialNumber          string
	AccessCode            string
	TLSInsecureSkipVerify bool
	ReconnectDelaySeconds int
}

// printer implements the Printer interface.
type printer struct {
	cfg       PrinterConfig
	notifiers []notifier.Notifier
	logger    *slog.Logger
	tracker   *stateTracker

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var _ Printer = (*printer)(nil)

// New creates a new Printer for the given config, fanning out events
// to notifiers.
func New(
	cfg PrinterConfig,
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
			client.close()
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

		if err := client.readLoop(); err != nil {
			p.logger.Error("mqtt read loop exited", "error", err)
		}

		connCancel()
		client.close()

		if ctx.Err() != nil {
			return
		}

		p.logger.Info("reconnecting after disconnect")
		p.backoff(ctx, baseDelay, maxDelay, 0)
	}
}

func (p *printer) newClient() *mqttClient {
	opts := []mqttOption{
		withCredentials("bblp", p.cfg.AccessCode),
		withOnPublish(p.handleMessage),
		withLogger(p.logger),
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: p.cfg.TLSInsecureSkipVerify,
	}
	opts = append(opts, withTLSConfig(tlsCfg))

	return newMQTTClient(
		p.cfg.Host, p.cfg.Port, p.cfg.SerialNumber, opts...,
	)
}

func (p *printer) handleMessage(topic string, payload []byte) {
	report, err := parseReport(payload)
	if err != nil {
		p.logger.Debug("ignoring unparseable report", "error", err)
		return
	}

	events := p.tracker.update(report)
	for _, evt := range events {
		p.fanOut(evt)
	}
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
			return
		case <-ticker.C:
			if err := client.ping(); err != nil {
				p.logger.Warn("ping failed", "error", err)
				client.close()
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
