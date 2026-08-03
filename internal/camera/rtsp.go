package camera

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultRTSPPort = "322"
	defaultRTSPPath = "/streaming/live/1"
	rtspTimeout     = 20 * time.Second
)

// CaptureRTSPFrame grabs a single JPEG frame from a printer's RTSPS
// live stream (X1/X2/H2/P2 series) by invoking the ffmpeg binary.
// reportedURL is the rtsp_url the printer advertised in its MQTT
// report (may be empty); its path and port are kept but the host is
// always replaced with host since firmware sometimes reports a wrong
// IP, and bblp credentials are injected.
func CaptureRTSPFrame(
	host, accessCode, reportedURL string,
) ([]byte, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf(
			"ffmpeg not found in PATH (required for RTSP " +
				"camera snapshots)",
		)
	}

	streamURL := buildRTSPURL(host, accessCode, reportedURL)

	ctx, cancel := context.WithTimeout(
		context.Background(), rtspTimeout,
	)
	defer cancel()

	//nolint:gosec // args are built from validated config, not user-facing input
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-rtsp_transport", "tcp",
		"-i", streamURL,
		"-frames:v", "1",
		"-q:v", "2",
		"-f", "image2pipe",
		"-c:v", "mjpeg",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("rtsp frame grab: %s", msg)
	}

	frame := stdout.Bytes()
	if len(frame) == 0 {
		return nil, fmt.Errorf("rtsp frame grab: empty frame")
	}

	return frame, nil
}

// buildRTSPURL assembles the authenticated stream URL. The port and
// path come from the printer-reported URL when it is a parseable
// rtsp(s) URL, and fall back to the Bambu defaults otherwise.
func buildRTSPURL(host, accessCode, reportedURL string) string {
	port := defaultRTSPPort
	path := defaultRTSPPath

	if u, err := url.Parse(reportedURL); err == nil &&
		strings.HasPrefix(u.Scheme, "rtsp") {
		if p := u.Port(); p != "" {
			port = p
		}
		if u.Path != "" {
			path = u.Path
		}
	}

	u := url.URL{
		Scheme: "rtsps",
		User:   url.UserPassword("bblp", accessCode),
		Host:   host + ":" + port,
		Path:   path,
	}

	return u.String()
}
