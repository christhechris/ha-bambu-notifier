package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegram_Send(t *testing.T) {
	tests := []struct {
		name      string
		event     event.Event
		wantError bool
	}{
		{
			name:  "print started",
			event: testEvent(),
		},
		{
			name:      "print failed with error",
			event:     testErrorEvent(),
			wantError: true,
		},
		{
			name: "event without AMS slots",
			event: func() event.Event {
				ev := testEvent()
				ev.AMSSlots = nil
				return ev
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var err error
				gotBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			tg := NewTelegram("test-telegram", "123:ABC", "456789").(*telegram)
			tg.apiURL = srv.URL

			err := tg.Send(context.Background(), tt.event)
			require.NoError(t, err)

			var payload telegramPayload
			require.NoError(t, json.Unmarshal(gotBody, &payload))

			assert.Equal(t, "456789", payload.ChatID)
			assert.Equal(t, "HTML", payload.ParseMode)
			assert.NotEmpty(t, payload.Text)

			assert.Contains(t, payload.Text, "<b>")
			assert.Contains(t, payload.Text, "X1C-Workshop")
			assert.Contains(t, payload.Text, "benchy.3mf")
			assert.Contains(t, payload.Text, "42%")
			assert.Contains(t, payload.Text, "220.5")

			if tt.event.AMSSlots != nil {
				assert.Contains(t, payload.Text, "AMS Slots")
				assert.Contains(t, payload.Text, "PLA")
				assert.Contains(t, payload.Text, "IN USE")
			} else {
				assert.NotContains(t, payload.Text, "AMS Slots")
			}

			if tt.wantError {
				assert.Contains(t, payload.Text, "nozzle clog")
			}
		})
	}
}

func TestTelegram_Name(t *testing.T) {
	tg := NewTelegram("my-telegram", "token", "chat")
	assert.Equal(t, "my-telegram", tg.Name())
}

func TestTelegram_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tg := NewTelegram("test", "bad-token", "456789").(*telegram)
	tg.apiURL = srv.URL

	err := tg.Send(context.Background(), testEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 401")
}
