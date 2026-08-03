package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlack_Send(t *testing.T) {
	tests := []struct {
		name      string
		event     event.Event
		secret    string
		wantError bool
	}{
		{
			name:  "print started without secret",
			event: testEvent(),
		},
		{
			name:      "print failed with error section",
			event:     testErrorEvent(),
			wantError: true,
		},
		{
			name:   "with HMAC secret",
			event:  testEvent(),
			secret: "slack-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotBody   []byte
				gotHeader http.Header
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var err error
				gotBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)

				gotHeader = r.Header
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			s := NewSlack("test-slack", srv.URL, tt.secret)
			err := s.Send(context.Background(), tt.event)
			require.NoError(t, err)

			var payload slackPayload
			require.NoError(t, json.Unmarshal(gotBody, &payload))

			// A single compact section block.
			require.Len(t, payload.Blocks, 1)
			block := payload.Blocks[0]
			assert.Equal(t, "section", block.Type)
			require.NotNil(t, block.Text)
			assert.Equal(t, "mrkdwn", block.Text.Type)

			text := block.Text.Text
			assert.Contains(t, text, "X1C-Workshop")
			assert.Contains(t, text, "benchy.3mf")
			assert.Contains(t, text, "42%")
			assert.Contains(t, text, "ETA 1h 23m")
			assert.Len(t, strings.Split(text, "\n"), 2,
				"message should be exactly two lines")

			if tt.wantError {
				assert.Contains(t, text, "nozzle clog")
			}

			if tt.secret != "" {
				verifyHMAC(t, gotBody, tt.secret, gotHeader.Get("X-Hub-Signature-256"))
			} else {
				assert.Empty(t, gotHeader.Get("X-Hub-Signature-256"))
			}
		})
	}
}

func TestSlack_Name(t *testing.T) {
	s := NewSlack("my-slack", "http://example.com", "")
	assert.Equal(t, "my-slack", s.Name())
}

func TestSlack_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	s := NewSlack("test", srv.URL, "")
	err := s.Send(context.Background(), testEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 502")
}
