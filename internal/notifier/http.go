package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/l33t0/bambu-notifier/internal/event"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// postJSON marshals payload as JSON and POSTs it to url. If secret is
// non-empty, an HMAC-SHA256 signature is added as X-Hub-Signature-256.
func postJSON(
	ctx context.Context,
	url string,
	payload any,
	secret string,
	label string,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s: marshal payload: %w", label, err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("%s: create request: %w", label, err)
	}
	req.Header.Set("Content-Type", "application/json")

	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", computeHMAC(body, secret))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: send request: %w", label, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: unexpected status %d", label, resp.StatusCode)
	}
	return nil
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func formatAMSSlots(slots []event.AMSSlot) string {
	var sb strings.Builder
	for _, s := range slots {
		inUse := ""
		if s.InUse {
			inUse = " [IN USE]"
		}
		fmt.Fprintf(&sb, "Slot %d: %s (#%s)%s\n",
			s.SlotID, s.Material, s.ColorHex, inUse,
		)
	}
	return sb.String()
}
