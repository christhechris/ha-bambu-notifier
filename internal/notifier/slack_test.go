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

			require.NotEmpty(t, payload.Blocks)

			// First block is the header.
			assert.Equal(t, "header", payload.Blocks[0].Type)
			assert.NotNil(t, payload.Blocks[0].Text)
			assert.Equal(t, "plain_text", payload.Blocks[0].Text.Type)
			assert.NotEmpty(t, payload.Blocks[0].Text.Text)

			// Second block is a section with fields.
			assert.Equal(t, "section", payload.Blocks[1].Type)
			require.NotEmpty(t, payload.Blocks[1].Fields)

			fieldTexts := ""
			for _, f := range payload.Blocks[1].Fields {
				fieldTexts += f.Text + " "
			}
			assert.Contains(t, fieldTexts, "X1C-Workshop")
			assert.Contains(t, fieldTexts, "benchy.3mf")
			assert.Contains(t, fieldTexts, "42%")

			// Divider block.
			assert.Equal(t, "divider", payload.Blocks[2].Type)

			// Thermals section.
			assert.Equal(t, "section", payload.Blocks[3].Type)
			assert.Contains(t, payload.Blocks[3].Text.Text, "220.5")
			assert.Contains(t, payload.Blocks[3].Text.Text, "Thermals")

			// AMS section.
			found := false
			for _, b := range payload.Blocks {
				if b.Text != nil && b.Type == "section" && strings.Contains(b.Text.Text, "AMS") {
					found = true
					assert.Contains(t, b.Text.Text, "PLA")
					assert.Contains(t, b.Text.Text, "IN USE")
				}
			}
			assert.True(t, found, "should have AMS section")

			if tt.wantError {
				errorFound := false
				for _, b := range payload.Blocks {
					if b.Text != nil && strings.Contains(b.Text.Text, "nozzle clog") {
						errorFound = true
					}
				}
				assert.True(t, errorFound, "should have error block")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	s := NewSlack("test", srv.URL, "")
	err := s.Send(context.Background(), testEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 502")
}

