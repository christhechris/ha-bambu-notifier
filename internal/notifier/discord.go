package notifier

import (
	"context"
	"encoding/json"
	"fmt"
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

type discord struct {
	name       string
	webhookURL string
	secret     string
}

// NewDiscord returns a Notifier that posts rich embeds to a Discord
// webhook.
func NewDiscord(name, webhookURL, secret string) Notifier {
	return &discord{
		name:       name,
		webhookURL: webhookURL,
		secret:     secret,
	}
}

var _ Notifier = (*discord)(nil)

func (d *discord) Name() string { return d.name }

func (d *discord) Send(ctx context.Context, ev event.Event) error {
	payload := d.buildPayload(ev)

	if ev.Snapshot != nil {
		payload.Embeds[0].Image = &discordImage{
			URL: "attachment://snapshot.jpg",
		}

		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("discord: marshal payload: %w", err)
		}

		return postMultipart(ctx, d.webhookURL,
			map[string]string{"payload_json": string(jsonBytes)},
			"files[0]", ev.Snapshot, "snapshot.jpg",
			d.secret, "discord",
		)
	}

	return postJSON(ctx, d.webhookURL, payload, d.secret, "discord")
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title  string        `json:"title"`
	Color  int           `json:"color"`
	Fields []embedField  `json:"fields"`
	Image  *discordImage `json:"image,omitempty"`
}

type discordImage struct {
	URL string `json:"url"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func (d *discord) buildPayload(ev event.Event) discordPayload {
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
		fields = append(fields, embedField{
			Name:  "AMS Slots",
			Value: formatAMSSlots(ev.AMSSlots),
		})
	}

	if ev.ErrorMsg != "" {
		fields = append(fields, embedField{
			Name: "Error", Value: ev.ErrorMsg,
		})
	}

	return discordPayload{
		Embeds: []discordEmbed{
			{
				Title:  formatTitle(ev.Type),
				Color:  eventColor(ev.Type),
				Fields: fields,
			},
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
