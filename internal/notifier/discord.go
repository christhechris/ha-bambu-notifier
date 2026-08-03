package notifier

import (
	"context"
	"encoding/json"
	"fmt"

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
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Color       int           `json:"color"`
	Image       *discordImage `json:"image,omitempty"`
}

type discordImage struct {
	URL string `json:"url"`
}

func (d *discord) buildPayload(ev event.Event) discordPayload {
	return discordPayload{
		Embeds: []discordEmbed{
			{
				Title: fmt.Sprintf("%s — %s",
					formatTitle(ev.Type), ev.PrinterName,
				),
				Description: summaryLine(ev),
				Color:       eventColor(ev.Type),
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
