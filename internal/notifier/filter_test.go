package notifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/l33t0/bambu-notifier/internal/event"
)

type recordingNotifier struct {
	sent []event.Type
}

func (r *recordingNotifier) Name() string { return "recorder" }

func (r *recordingNotifier) Send(
	_ context.Context, evt event.Event,
) error {
	r.sent = append(r.sent, evt.Type)
	return nil
}

func TestWithEventFilter(t *testing.T) {
	t.Parallel()

	t.Run("empty filter returns notifier unchanged", func(t *testing.T) {
		t.Parallel()

		rec := &recordingNotifier{}
		n := WithEventFilter(rec, nil)

		assert.Same(t, rec, n)
	})

	t.Run("only allowed event types are sent", func(t *testing.T) {
		t.Parallel()

		rec := &recordingNotifier{}
		n := WithEventFilter(rec, []event.Type{
			event.PrintStarted,
			event.PrintFinished,
			event.PrintFailed,
		})

		assert.Equal(t, "recorder", n.Name())

		ctx := context.Background()
		for _, typ := range []event.Type{
			event.PrintStarted,
			event.PrintPaused,
			event.AMSChange,
			event.PrintFinished,
			event.HMSAlert,
			event.PrintFailed,
		} {
			require.NoError(t, n.Send(ctx, event.Event{Type: typ}))
		}

		assert.Equal(t, []event.Type{
			event.PrintStarted,
			event.PrintFinished,
			event.PrintFailed,
		}, rec.sent)
	})
}
