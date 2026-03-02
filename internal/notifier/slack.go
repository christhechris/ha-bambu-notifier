package notifier

import (
	"context"
	"fmt"

	"github.com/l33t0/bambu-notifier/internal/event"
)

type slack struct {
	name       string
	webhookURL string
	secret     string
}

// NewSlack returns a Notifier that posts Block Kit messages to a Slack
// webhook.
func NewSlack(name, webhookURL, secret string) Notifier {
	return &slack{
		name:       name,
		webhookURL: webhookURL,
		secret:     secret,
	}
}

var _ Notifier = (*slack)(nil)

func (s *slack) Name() string { return s.name }

func (s *slack) Send(ctx context.Context, ev event.Event) error {
	return postJSON(ctx, s.webhookURL, s.buildPayload(ev),
		s.secret, "slack")
}

type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *slack) buildPayload(ev event.Event) slackPayload {
	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{
				Type: "plain_text",
				Text: formatTitle(ev.Type),
			},
		},
		{
			Type: "section",
			Fields: []slackText{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Printer:*\n%s", ev.PrinterName)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Filename:*\n%s", ev.Filename)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Progress:*\n%d%%", ev.Progress)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*ETA:*\n%s", ev.ETA)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Nozzle Type:*\n%s", ev.NozzleType)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Source:*\n%s", ev.Source)},
			},
		},
		{Type: "divider"},
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf(
					"*Thermals*\nNozzle: %.1f/%.1f °C | Bed: %.1f/%.1f °C",
					ev.Thermals.NozzleActual, ev.Thermals.NozzleTarget,
					ev.Thermals.BedActual, ev.Thermals.BedTarget,
				),
			},
		},
	}

	if len(ev.AMSSlots) > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: "*AMS Slots*\n" + formatAMSSlots(ev.AMSSlots),
			},
		})
	}

	if ev.ErrorMsg != "" {
		blocks = append(blocks,
			slackBlock{Type: "divider"},
			slackBlock{
				Type: "section",
				Text: &slackText{
					Type: "mrkdwn",
					Text: fmt.Sprintf(
						":warning: *Error:* %s", ev.ErrorMsg,
					),
				},
			},
		)
	}

	return slackPayload{Blocks: blocks}
}
