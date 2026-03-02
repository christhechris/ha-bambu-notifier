package notifier

import (
	"context"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// Notifier sends a notification for a print lifecycle event.
type Notifier interface {
	Name() string
	Send(ctx context.Context, event event.Event) error
}
