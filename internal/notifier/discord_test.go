package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEvent() event.Event {
	return event.Event{
		Type:        event.PrintStarted,
		PrinterName: "X1C-Workshop",
		Filename:    "benchy.3mf",
		Progress:    42,
		ETA:         "1h 23m",
		NozzleType:  "0.4mm Hardened Steel",
		Source:       "LAN",
		Thermals: event.Thermals{
			NozzleActual: 220.5,
			NozzleTarget: 230.0,
			BedActual:    58.3,
			BedTarget:    60.0,
		},
		AMSSlots: []event.AMSSlot{
			{SlotID: 1, Material: "PLA", ColorHex: "FF0000", InUse: true},
			{SlotID: 2, Material: "PETG", ColorHex: "00FF00", InUse: false},
		},
		Timestamp: time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
	}
}

func testErrorEvent() event.Event {
	ev := testEvent()
	ev.Type = event.PrintFailed
	ev.ErrorMsg = "nozzle clog detected"
	return ev
}

func verifyHMAC(t *testing.T, body []byte, secret, header string) {
	t.Helper()
	require.NotEmpty(t, header, "X-Hub-Signature-256 header must be present")
	assert.True(t, len(header) > 7 && header[:7] == "sha256=", "header must start with sha256= prefix")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, header)
}

func TestDiscord_Send(t *testing.T) {
	tests := []struct {
		name      string
		event     event.Event
		secret    string
		wantColor int
		wantError bool
	}{
		{
			name:      "print started with green color",
			event:     testEvent(),
			secret:    "",
			wantColor: colorGreen,
		},
		{
			name:      "print failed with red color and error field",
			event:     testErrorEvent(),
			secret:    "",
			wantColor: colorRed,
			wantError: true,
		},
		{
			name: "print finished with blue color",
			event: func() event.Event {
				ev := testEvent()
				ev.Type = event.PrintFinished
				return ev
			}(),
			secret:    "",
			wantColor: colorBlue,
		},
		{
			name: "print paused with yellow color",
			event: func() event.Event {
				ev := testEvent()
				ev.Type = event.PrintPaused
				return ev
			}(),
			secret:    "",
			wantColor: colorYellow,
		},
		{
			name:      "with HMAC secret",
			event:     testEvent(),
			secret:    "supersecret",
			wantColor: colorGreen,
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
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			d := NewDiscord("test-discord", srv.URL, tt.secret).(*discord)
			err := d.Send(context.Background(), tt.event)
			require.NoError(t, err)

			var payload discordPayload
			require.NoError(t, json.Unmarshal(gotBody, &payload))

			require.Len(t, payload.Embeds, 1)
			embed := payload.Embeds[0]

			assert.NotEmpty(t, embed.Title)
			assert.Equal(t, tt.wantColor, embed.Color)
			assert.GreaterOrEqual(t, len(embed.Fields), 7, "should have at least 7 base fields")

			fieldNames := make(map[string]string)
			for _, f := range embed.Fields {
				fieldNames[f.Name] = f.Value
			}

			assert.Equal(t, "X1C-Workshop", fieldNames["Printer Name"])
			assert.Equal(t, "benchy.3mf", fieldNames["Filename"])
			assert.Contains(t, fieldNames["Progress"], "42%")
			assert.Contains(t, fieldNames["Thermals"], "220.5")
			assert.Contains(t, fieldNames["AMS Slots"], "PLA")
			assert.Contains(t, fieldNames["AMS Slots"], "IN USE")

			if tt.wantError {
				assert.Contains(t, fieldNames["Error"], "nozzle clog")
			}

			if tt.secret != "" {
				verifyHMAC(t, gotBody, tt.secret, gotHeader.Get("X-Hub-Signature-256"))
			} else {
				assert.Empty(t, gotHeader.Get("X-Hub-Signature-256"), "no signature when secret is empty")
			}
		})
	}
}

func TestDiscord_Name(t *testing.T) {
	d := NewDiscord("my-discord", "http://example.com", "")
	assert.Equal(t, "my-discord", d.Name())
}

func TestDiscord_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewDiscord("test", srv.URL, "")
	err := d.Send(context.Background(), testEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}
