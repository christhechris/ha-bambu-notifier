package notifier

import (
	"context"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// WithEventFilter wraps n so that only events whose type is in
// allowed are sent; other events are silently dropped. An empty
// allowed list means no filtering and n is returned unchanged.
func WithEventFilter(n Notifier, allowed []event.Type) Notifier {
	if len(allowed) == 0 {
		return n
	}

	m := make(map[event.Type]bool, len(allowed))
	for _, t := range allowed {
		m[t] = true
	}

	return &filtered{inner: n, allowed: m}
}

type filtered struct {
	inner   Notifier
	allowed map[event.Type]bool
}

func (f *filtered) Name() string { return f.inner.Name() }

func (f *filtered) Send(ctx context.Context, evt event.Event) error {
	if !f.allowed[evt.Type] {
		return nil
	}
	return f.inner.Send(ctx, evt)
}
