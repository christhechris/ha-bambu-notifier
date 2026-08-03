package notifier

import (
	"context"
	"fmt"

	"github.com/l33t0/bambu-notifier/internal/event"
)

type telegram struct {
	name    string
	baseURL string
	chatID  string
}

// NewTelegram returns a Notifier that sends HTML-formatted messages
// via Telegram Bot API.
func NewTelegram(name, botToken, chatID string) Notifier {
	return &telegram{
		name: name,
		baseURL: fmt.Sprintf(
			"https://api.telegram.org/bot%s",
			botToken,
		),
		chatID: chatID,
	}
}

var _ Notifier = (*telegram)(nil)

func (t *telegram) Name() string { return t.name }

func (t *telegram) Send(ctx context.Context, ev event.Event) error {
	if ev.Snapshot != nil {
		return t.sendPhoto(ctx, ev)
	}
	return t.sendMessage(ctx, ev)
}

func (t *telegram) sendMessage(ctx context.Context, ev event.Event) error {
	payload := telegramPayload{
		ChatID:    t.chatID,
		Text:      t.buildMessage(ev),
		ParseMode: "HTML",
	}
	return postJSON(ctx, t.baseURL+"/sendMessage", payload, "", "telegram")
}

func (t *telegram) sendPhoto(ctx context.Context, ev event.Event) error {
	return postMultipart(ctx, t.baseURL+"/sendPhoto",
		map[string]string{
			"chat_id":    t.chatID,
			"caption":    t.buildMessage(ev),
			"parse_mode": "HTML",
		},
		"photo", ev.Snapshot, "snapshot.jpg",
		"", "telegram",
	)
}

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *telegram) buildMessage(ev event.Event) string {
	return fmt.Sprintf("<b>%s</b> — %s\n%s",
		formatTitle(ev.Type), ev.PrinterName, summaryLine(ev),
	)
}
