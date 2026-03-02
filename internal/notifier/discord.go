package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// Discord embed sidebar colors.
const (
	colorGreen  = 0x00FF00
	colorBlue   = 0x0000FF
	colorRed    = 0xFF0000
	colorYellow = 0xFFFF00
)

// discord sends notifications via a Discord webhook with rich embeds.
type discord struct {
	name       string
	webhookURL string
	secret     string
	client     *http.Client
}

// NewDiscord returns a Notifier that posts rich embeds to a Discord webhook.
func NewDiscord(name, webhookURL, secret string) Notifier {
	return &discord{
		name:       name,
		webhookURL: webhookURL,
		secret:     secret,
		client:     http.DefaultClient,
	}
}

var _ Notifier = (*discord)(nil)

func (d *discord) Name() string { return d.name }

func (d *discord) Send(ctx context.Context, ev event.Event) error {
	payload := d.buildPayload(ev)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if d.secret != "" {
		sig := computeHMAC(body, d.secret)
		req.Header.Set("X-Hub-Signature-256", sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title  string       `json:"title"`
	Color  int          `json:"color"`
	Fields []embedField `json:"fields"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func (d *discord) buildPayload(ev event.Event) discordPayload {
	title := formatTitle(ev.Type)
	color := eventColor(ev.Type)

	fields := []embedField{
		{Name: "Printer Name", Value: ev.PrinterName, Inline: true},
		{Name: "Filename", Value: ev.Filename, Inline: true},
		{Name: "Progress", Value: fmt.Sprintf("%d%%", ev.Progress), Inline: true},
		{Name: "ETA", Value: ev.ETA, Inline: true},
		{Name: "Nozzle Type", Value: ev.NozzleType, Inline: true},
		{Name: "Source", Value: ev.Source, Inline: true},
		{Name: "Thermals", Value: fmt.Sprintf(
			"Nozzle: %.1f/%.1f °C\nBed: %.1f/%.1f °C",
			ev.Thermals.NozzleActual, ev.Thermals.NozzleTarget,
			ev.Thermals.BedActual, ev.Thermals.BedTarget,
		), Inline: false},
	}

	if len(ev.AMSSlots) > 0 {
		var sb strings.Builder
		for _, s := range ev.AMSSlots {
			inUse := ""
			if s.InUse {
				inUse = " [IN USE]"
			}
			fmt.Fprintf(&sb, "Slot %d: %s (#%s)%s\n", s.SlotID, s.Material, s.ColorHex, inUse)
		}
		fields = append(fields, embedField{Name: "AMS Slots", Value: sb.String(), Inline: false})
	}

	if ev.ErrorMsg != "" {
		fields = append(fields, embedField{Name: "Error", Value: ev.ErrorMsg, Inline: false})
	}

	return discordPayload{
		Embeds: []discordEmbed{
			{Title: title, Color: color, Fields: fields},
		},
	}
}

func eventColor(t event.Type) int {
	switch t {
	case event.PrintStarted, event.PrintResumed:
		return colorGreen
	case event.PrintFinished:
		return colorBlue
	case event.PrintFailed, event.CriticalError:
		return colorRed
	case event.PrintPaused:
		return colorYellow
	default:
		return colorYellow
	}
}

func formatTitle(t event.Type) string {
	s := string(t)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ToUpper(s[:1]) + s[1:]
	return s
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
