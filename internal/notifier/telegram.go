package notifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/l33t0/bambu-notifier/internal/event"
)

type telegram struct {
	name   string
	apiURL string
	chatID string
}

// NewTelegram returns a Notifier that sends HTML-formatted messages
// via Telegram Bot API.
func NewTelegram(name, botToken, chatID string) Notifier {
	return &telegram{
		name: name,
		apiURL: fmt.Sprintf(
			"https://api.telegram.org/bot%s/sendMessage",
			botToken,
		),
		chatID: chatID,
	}
}

var _ Notifier = (*telegram)(nil)

func (t *telegram) Name() string { return t.name }

func (t *telegram) Send(ctx context.Context, ev event.Event) error {
	payload := telegramPayload{
		ChatID:    t.chatID,
		Text:      t.buildMessage(ev),
		ParseMode: "HTML",
	}
	return postJSON(ctx, t.apiURL, payload, "", "telegram")
}

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *telegram) buildMessage(ev event.Event) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>%s</b>\n\n", formatTitle(ev.Type))
	fmt.Fprintf(&sb, "<b>Printer:</b> <code>%s</code>\n", ev.PrinterName)
	fmt.Fprintf(&sb, "<b>Filename:</b> <code>%s</code>\n", ev.Filename)
	fmt.Fprintf(&sb, "<b>Progress:</b> %d%%\n", ev.Progress)
	fmt.Fprintf(&sb, "<b>ETA:</b> %s\n", ev.ETA)
	fmt.Fprintf(&sb, "<b>Nozzle Type:</b> %s\n", ev.NozzleType)
	fmt.Fprintf(&sb, "<b>Source:</b> %s\n", ev.Source)
	fmt.Fprintf(&sb, "\n<b>Thermals</b>\n")
	fmt.Fprintf(&sb, "<i>Nozzle:</i> %.1f/%.1f °C\n",
		ev.Thermals.NozzleActual, ev.Thermals.NozzleTarget)
	fmt.Fprintf(&sb, "<i>Bed:</i> %.1f/%.1f °C\n",
		ev.Thermals.BedActual, ev.Thermals.BedTarget)

	if len(ev.AMSSlots) > 0 {
		sb.WriteString("\n<b>AMS Slots</b>\n")
		sb.WriteString(formatAMSSlots(ev.AMSSlots))
	}

	if ev.ErrorMsg != "" {
		fmt.Fprintf(&sb, "\n<b>Error:</b> %s\n", ev.ErrorMsg)
	}

	return sb.String()
}
