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

// telegram sends notifications via the Telegram Bot API.
type telegram struct {
	name   string
	token  string
	chatID string
	apiURL string
	client *http.Client
}

// NewTelegram returns a Notifier that sends HTML-formatted messages via Telegram Bot API.
func NewTelegram(name, botToken, chatID string) Notifier {
	return &telegram{
		name:   name,
		token:  botToken,
		chatID: chatID,
		apiURL: fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken),
		client: http.DefaultClient,
	}
}

var _ Notifier = (*telegram)(nil)

func (t *telegram) Name() string { return t.name }

func (t *telegram) Send(ctx context.Context, ev event.Event) error {
	msg := t.buildMessage(ev)

	payload := telegramPayload{
		ChatID:    t.chatID,
		Text:      msg,
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *telegram) buildMessage(ev event.Event) string {
	title := formatTitle(ev.Type)

	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>%s</b>\n\n", title)
	fmt.Fprintf(&sb, "<b>Printer:</b> <code>%s</code>\n", ev.PrinterName)
	fmt.Fprintf(&sb, "<b>Filename:</b> <code>%s</code>\n", ev.Filename)
	fmt.Fprintf(&sb, "<b>Progress:</b> %d%%\n", ev.Progress)
	fmt.Fprintf(&sb, "<b>ETA:</b> %s\n", ev.ETA)
	fmt.Fprintf(&sb, "<b>Nozzle Type:</b> %s\n", ev.NozzleType)
	fmt.Fprintf(&sb, "<b>Source:</b> %s\n", ev.Source)
	fmt.Fprintf(&sb, "\n<b>Thermals</b>\n")
	fmt.Fprintf(&sb, "<i>Nozzle:</i> %.1f/%.1f °C\n", ev.Thermals.NozzleActual, ev.Thermals.NozzleTarget)
	fmt.Fprintf(&sb, "<i>Bed:</i> %.1f/%.1f °C\n", ev.Thermals.BedActual, ev.Thermals.BedTarget)

	if len(ev.AMSSlots) > 0 {
		sb.WriteString("\n<b>AMS Slots</b>\n")
		for _, slot := range ev.AMSSlots {
			inUse := ""
			if slot.InUse {
				inUse = " [IN USE]"
			}
			fmt.Fprintf(&sb, "Slot %d: %s (#%s)%s\n", slot.SlotID, slot.Material, slot.ColorHex, inUse)
		}
	}

	if ev.ErrorMsg != "" {
		fmt.Fprintf(&sb, "\n<b>Error:</b> %s\n", ev.ErrorMsg)
	}

	return sb.String()
}
