package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// slack sends notifications via a Slack incoming webhook using Block Kit.
type slack struct {
	name       string
	webhookURL string
	secret     string
	client     *http.Client
}

// NewSlack returns a Notifier that posts Block Kit messages to a Slack webhook.
func NewSlack(name, webhookURL, secret string) Notifier {
	return &slack{
		name:       name,
		webhookURL: webhookURL,
		secret:     secret,
		client:     http.DefaultClient,
	}
}

var _ Notifier = (*slack)(nil)

func (s *slack) Name() string { return s.name }

func (s *slack) Send(ctx context.Context, ev event.Event) error {
	payload := s.buildPayload(ev)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.secret != "" {
		sig := computeHMAC(body, s.secret)
		req.Header.Set("X-Hub-Signature-256", sig)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type    string      `json:"type"`
	Text    *slackText  `json:"text,omitempty"`
	Fields  []slackText `json:"fields,omitempty"`
	BlockID string      `json:"block_id,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *slack) buildPayload(ev event.Event) slackPayload {
	title := formatTitle(ev.Type)

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{Type: "plain_text", Text: title},
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
		var sb strings.Builder
		sb.WriteString("*AMS Slots*\n")
		for _, slot := range ev.AMSSlots {
			inUse := ""
			if slot.InUse {
				inUse = " `IN USE`"
			}
			fmt.Fprintf(&sb, "Slot %d: %s (#%s)%s\n", slot.SlotID, slot.Material, slot.ColorHex, inUse)
		}
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: sb.String()},
		})
	}

	if ev.ErrorMsg != "" {
		blocks = append(blocks, slackBlock{Type: "divider"})
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf(":warning: *Error:* %s", ev.ErrorMsg)},
		})
	}

	return slackPayload{Blocks: blocks}
}
