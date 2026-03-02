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
	"mime/multipart"
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

	resp, err := httpClient.Do(req) //nolint:gosec // webhook URL is from trusted config, not user input
	if err != nil {
		return fmt.Errorf("%s: send request: %w", label, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: unexpected status %d", label, resp.StatusCode)
	}
	return nil
}

// postMultipart sends a multipart/form-data POST with text fields and
// a single file attachment. If secret is non-empty, an HMAC-SHA256
// signature is added as X-Hub-Signature-256.
func postMultipart(
	ctx context.Context,
	url string,
	fields map[string]string,
	fileField string,
	fileData []byte,
	fileName string,
	secret string,
	label string,
) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return fmt.Errorf("%s: write field %s: %w", label, k, err)
		}
	}

	part, partErr := w.CreateFormFile(fileField, fileName)
	if partErr != nil {
		return fmt.Errorf("%s: create file part: %w", label, partErr)
	}
	if _, writeErr := part.Write(fileData); writeErr != nil {
		return fmt.Errorf("%s: write file data: %w", label, writeErr)
	}

	if closeErr := w.Close(); closeErr != nil {
		return fmt.Errorf("%s: close multipart: %w", label, closeErr)
	}

	body := buf.Bytes()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("%s: create request: %w", label, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", computeHMAC(body, secret))
	}

	resp, err := httpClient.Do(req) //nolint:gosec // webhook URL is from trusted config, not user input
	if err != nil {
		return fmt.Errorf("%s: send request: %w", label, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
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
