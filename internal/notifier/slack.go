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
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *slack) buildPayload(ev event.Event) slackPayload {
	return slackPayload{
		Blocks: []slackBlock{
			{
				Type: "section",
				Text: &slackText{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*%s* — %s\n%s",
						formatTitle(ev.Type),
						ev.PrinterName,
						summaryLine(ev),
					),
				},
			},
		},
	}
}
