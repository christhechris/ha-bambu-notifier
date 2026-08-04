package notifier

import (
	"fmt"
	"strings"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// formatTitle turns an event type into a human-readable title, e.g.
// "print_started" → "Print started".
func formatTitle(t event.Type) string {
	s := string(t)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ToUpper(s[:1]) + s[1:]
	return s
}

// summaryLine builds the compact one-line summary shared by all
// notifiers: "<filename> · <progress>% · ETA <eta>[ · <reason>][ ·
// <error>]", skipping empty parts.
func summaryLine(ev event.Event) string {
	parts := make([]string, 0, 5)

	if ev.Filename != "" {
		parts = append(parts, ev.Filename)
	}
	parts = append(parts, fmt.Sprintf("%d%%", ev.Progress))
	if ev.ETA != "" {
		parts = append(parts, "ETA "+ev.ETA)
	}
	if ev.Reason != "" {
		parts = append(parts, ev.Reason)
	}
	if ev.ErrorMsg != "" && ev.ErrorMsg != ev.Reason {
		parts = append(parts, ev.ErrorMsg)
	}

	return strings.Join(parts, " · ")
}
