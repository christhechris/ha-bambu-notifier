package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storageFixture = `{
  "version": 1,
  "minor_version": 5,
  "key": "core.config_entries",
  "data": {
    "entries": [
      {
        "entry_id": "aaa",
        "domain": "bambu_lab",
        "title": "AABBCC112233",
        "data": {"device_type": "P1S", "serial": "AABBCC112233"},
        "options": {
          "region": "Europe",
          "email": "user@example.com",
          "username": "u_123",
          "name": "Workshop P1S",
          "host": "192.168.1.50",
          "local_mqtt": true,
          "auth_token": "tok",
          "access_code": "12345678"
        }
      },
      {
        "entry_id": "bbb",
        "domain": "bambu_lab",
        "title": "DDEEFF445566",
        "data": {"device_type": "X1C", "serial": "DDEEFF445566"},
        "options": {
          "region": "Europe",
          "name": "Cloud Only X1C",
          "host": "",
          "auth_token": "tok",
          "access_code": ""
        }
      },
      {
        "entry_id": "ccc",
        "domain": "hue",
        "title": "Philips Hue",
        "data": {},
        "options": {}
      }
    ]
  }
}`

func writeStorage(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "core.config_entries")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestPrinters(t *testing.T) {
	t.Parallel()

	t.Run("imports LAN printers, skips cloud-only and other domains", func(t *testing.T) {
		t.Parallel()

		printers, err := Printers(writeStorage(t, storageFixture), nil)
		require.NoError(t, err)

		require.Len(t, printers, 1)
		p := printers[0]
		assert.Equal(t, "Workshop P1S", p.Name)
		assert.Equal(t, "192.168.1.50", p.Host)
		assert.Equal(t, "AABBCC112233", p.Serial)
		assert.Equal(t, "12345678", p.AccessCode)
	})

	t.Run("falls back to serial for missing name", func(t *testing.T) {
		t.Parallel()

		noName := `{
  "data": {
    "entries": [
      {
        "domain": "bambu_lab",
        "title": "AABBCC112233",
        "data": {"serial": "AABBCC112233"},
        "options": {"host": "10.0.0.5", "access_code": "87654321"}
      }
    ]
  }
}`
		printers, err := Printers(writeStorage(t, noName), nil)
		require.NoError(t, err)

		require.Len(t, printers, 1)
		assert.Equal(t, "AABBCC112233", printers[0].Name)
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()

		_, err := Printers("/nonexistent/core.config_entries", nil)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "reading HA storage")
	})

	t.Run("returns error for malformed storage", func(t *testing.T) {
		t.Parallel()

		_, err := Printers(writeStorage(t, "{{invalid"), nil)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "parsing HA storage")
	})

	t.Run("returns error when no default path exists", func(t *testing.T) {
		t.Parallel()

		_, err := Printers("", nil)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "storage not found")
	})
}
