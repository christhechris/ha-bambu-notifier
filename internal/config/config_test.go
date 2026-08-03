package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	return writeConfigNamed(t, "config.toml", content)
}

func writeConfigNamed(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestLoad(t *testing.T) {
	t.Parallel()

	validConfig := `
[log]
level  = "info"
format = "json"

[[printer]]
name                     = "X1C"
host                     = "192.168.1.100"
serial_number            = "AABBCC112233"
access_code              = "12345678"
tls_insecure_skip_verify = true
reconnect_delay_seconds  = 15

  [[printer.notifiers.discord]]
  name        = "print-alerts"
  webhook_url = "https://discord.com/api/webhooks/123/abc"
  secret      = "mysecret"

  [[printer.notifiers.telegram]]
  name      = "home-alerts"
  bot_token = "123:ABC"
  chat_id   = "-100123"
`

	t.Run("loads a valid configuration", func(t *testing.T) {
		t.Parallel()

		cfg, err := Load(writeConfig(t, validConfig))
		require.NoError(t, err)

		assert.Equal(t, LogLevelInfo, cfg.Log.Level)
		assert.Equal(t, LogFormatJSON, cfg.Log.Format)
		assert.Len(t, cfg.Printers, 1)

		p := cfg.Printers[0]
		assert.Equal(t, "X1C", p.Name)
		assert.Equal(t, "192.168.1.100", p.Host)
		assert.Equal(t, 8883, p.Port)
		assert.Equal(t, "AABBCC112233", p.SerialNumber)
		assert.Equal(t, "12345678", p.AccessCode)
		assert.True(t, p.TLSInsecureSkipVerify)
		assert.Equal(t, 15, p.ReconnectDelaySeconds)
		assert.Len(t, p.Notifiers.Discord, 1)
		assert.Equal(t, "print-alerts", p.Notifiers.Discord[0].Name)
		assert.Equal(t, "mysecret", p.Notifiers.Discord[0].Secret)
		assert.Len(t, p.Notifiers.Telegram, 1)
	})

	t.Run("applies default port and reconnect delay", func(t *testing.T) {
		t.Parallel()

		minimal := `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`
		cfg, err := Load(writeConfig(t, minimal))
		require.NoError(t, err)

		assert.Equal(t, 8883, cfg.Printers[0].Port)
		assert.Equal(t, 30, cfg.Printers[0].ReconnectDelaySeconds)
		assert.Equal(t, 0, cfg.Printers[0].CameraPort, "camera_port defaults to 0 (disabled)")
	})

	t.Run("loads camera_port when set", func(t *testing.T) {
		t.Parallel()

		withCamera := `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
camera_port   = 6000
`
		cfg, err := Load(writeConfig(t, withCamera))
		require.NoError(t, err)

		assert.Equal(t, 6000, cfg.Printers[0].CameraPort)
	})

	t.Run("loads multiple printers", func(t *testing.T) {
		t.Parallel()

		multi := `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"

[[printer]]
name          = "P2"
host          = "10.0.0.2"
serial_number = "S2"
access_code   = "T2"
`
		cfg, err := Load(writeConfig(t, multi))
		require.NoError(t, err)

		assert.Len(t, cfg.Printers, 2)
		assert.Equal(t, "P1", cfg.Printers[0].Name)
		assert.Equal(t, "P2", cfg.Printers[1].Name)
	})

	t.Run("returns error for missing config file", func(t *testing.T) {
		t.Parallel()

		_, err := Load("/nonexistent/config.toml")
		require.Error(t, err)

		assert.Contains(t, err.Error(), "reading config file")
	})

	t.Run("returns error for invalid TOML", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, "{{invalid"))
		require.Error(t, err)

		assert.Contains(t, err.Error(), "parsing config file")
	})

	t.Run("returns error when no printers are defined", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, `[log]
level = "info"
`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"at least one [[printer]] is required")
	})
}

func TestLoad_json(t *testing.T) {
	t.Parallel()

	optionsJSON := `{
  "log": {"level": "debug", "format": "json"},
  "tail": true,
  "printer": [
    {
      "name": "X1C",
      "host": "192.168.1.100",
      "serial_number": "AABBCC112233",
      "access_code": "12345678",
      "tls_insecure_skip_verify": true,
      "camera_port": 6000,
      "discord": [
        {
          "name": "print-alerts",
          "webhook_url": "https://discord.com/api/webhooks/123/abc",
          "secret": "mysecret"
        }
      ],
      "slack": [
        {
          "name": "ops",
          "webhook_url": "https://hooks.slack.com/services/T/B/x"
        }
      ],
      "telegram": [
        {"name": "home", "bot_token": "123:ABC", "chat_id": "-100123"}
      ]
    }
  ]
}`

	t.Run("loads Home Assistant options.json shape", func(t *testing.T) {
		t.Parallel()

		cfg, err := Load(writeConfigNamed(t, "options.json", optionsJSON))
		require.NoError(t, err)

		assert.Equal(t, LogLevelDebug, cfg.Log.Level)
		assert.Equal(t, LogFormatJSON, cfg.Log.Format)
		assert.True(t, cfg.Tail)
		require.Len(t, cfg.Printers, 1)

		p := cfg.Printers[0]
		assert.Equal(t, "X1C", p.Name)
		assert.Equal(t, 8883, p.Port, "default port applies to JSON configs")
		assert.Equal(t, 30, p.ReconnectDelaySeconds, "default reconnect delay applies")
		assert.Equal(t, 6000, p.CameraPort)
		assert.True(t, p.TLSInsecureSkipVerify)

		require.Len(t, p.Notifiers.Discord, 1, "flat discord list merges into Notifiers")
		assert.Equal(t, "print-alerts", p.Notifiers.Discord[0].Name)
		assert.Equal(t, "mysecret", p.Notifiers.Discord[0].Secret)
		require.Len(t, p.Notifiers.Slack, 1)
		require.Len(t, p.Notifiers.Telegram, 1)
		assert.Equal(t, "-100123", p.Notifiers.Telegram[0].ChatID)

		assert.Nil(t, p.FlatDiscord)
		assert.Nil(t, p.FlatSlack)
		assert.Nil(t, p.FlatTelegram)
	})

	t.Run("accepts nested notifiers object in JSON", func(t *testing.T) {
		t.Parallel()

		nested := `{
  "printer": [
    {
      "name": "P1",
      "host": "10.0.0.1",
      "serial_number": "S1",
      "access_code": "T1",
      "notifiers": {
        "discord": [
          {"name": "d", "webhook_url": "https://discord.com/api/webhooks/1/a"}
        ]
      }
    }
  ]
}`
		cfg, err := Load(writeConfigNamed(t, "config.json", nested))
		require.NoError(t, err)

		require.Len(t, cfg.Printers[0].Notifiers.Discord, 1)
		assert.Equal(t, "d", cfg.Printers[0].Notifiers.Discord[0].Name)
	})

	t.Run("extension check is case-insensitive", func(t *testing.T) {
		t.Parallel()

		minimal := `{"printer": [{"name": "P1", "host": "10.0.0.1", "serial_number": "S1", "access_code": "T1"}]}`

		cfg, err := Load(writeConfigNamed(t, "options.JSON", minimal))
		require.NoError(t, err)

		assert.Equal(t, "P1", cfg.Printers[0].Name)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfigNamed(t, "options.json", "{{invalid"))
		require.Error(t, err)

		assert.Contains(t, err.Error(), "parsing config file")
	})

	t.Run("validation errors match TOML paths", func(t *testing.T) {
		t.Parallel()

		missing := `{
  "printer": [
    {
      "name": "P1",
      "host": "10.0.0.1",
      "serial_number": "S1",
      "access_code": "T1",
      "discord": [{"name": "d"}]
    }
  ]
}`
		_, err := Load(writeConfigNamed(t, "options.json", missing))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"printer[0].notifiers.discord[0]: webhook_url is required")
	})

	t.Run("returns error when no printers are defined", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfigNamed(t, "options.json", `{"printer": []}`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"at least one [[printer]] is required")
	})
}

func TestLoad_discovery(t *testing.T) {
	t.Parallel()

	t.Run("allows zero printers when discovery is enabled", func(t *testing.T) {
		t.Parallel()

		cfg, err := Load(writeConfig(t, `
[discovery]
enabled = true

  [[discovery.notifiers.discord]]
  name        = "print-alerts"
  webhook_url = "https://discord.com/api/webhooks/123/abc"
`))
		require.NoError(t, err)

		assert.True(t, cfg.Discovery.Enabled)
		assert.Empty(t, cfg.Printers)
		require.Len(t, cfg.Discovery.Notifiers.Discord, 1)
	})

	t.Run("loads flat discovery notifiers from JSON", func(t *testing.T) {
		t.Parallel()

		optionsJSON := `{
  "discovery": {
    "enabled": true,
    "camera_port": 6000,
    "discord": [
      {"name": "d", "webhook_url": "https://discord.com/api/webhooks/1/a"}
    ]
  }
}`
		cfg, err := Load(writeConfigNamed(t, "options.json", optionsJSON))
		require.NoError(t, err)

		assert.True(t, cfg.Discovery.Enabled)
		assert.Equal(t, 6000, cfg.Discovery.CameraPort)
		require.Len(t, cfg.Discovery.Notifiers.Discord, 1)
		assert.Nil(t, cfg.Discovery.FlatDiscord)
	})

	t.Run("validates discovery notifiers", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, `
[discovery]
enabled = true

  [[discovery.notifiers.telegram]]
  name = "t"
`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"discovery.notifiers.telegram[0]: bot_token is required")
	})

	t.Run("still requires a printer when discovery is disabled", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, `
[discovery]
enabled = false
`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"at least one [[printer]] is required")
	})
}

func TestLoad_globalNotifiers(t *testing.T) {
	t.Parallel()

	t.Run("global notifiers apply to every printer (TOML)", func(t *testing.T) {
		t.Parallel()

		cfg, err := Load(writeConfig(t, `
[[notifiers.slack]]
webhook_url = "https://hooks.slack.com/services/T/B/x"

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"

[[printer]]
name          = "P2"
host          = "10.0.0.2"
serial_number = "S2"
access_code   = "T2"

  [[printer.notifiers.discord]]
  webhook_url = "https://discord.com/api/webhooks/1/a"
`))
		require.NoError(t, err)

		require.Len(t, cfg.Printers, 2)
		assert.Len(t, cfg.Printers[0].Notifiers.Slack, 1)
		assert.Empty(t, cfg.Printers[0].Notifiers.Discord)
		assert.Len(t, cfg.Printers[1].Notifiers.Slack, 1)
		assert.Len(t, cfg.Printers[1].Notifiers.Discord, 1,
			"per-printer notifiers are kept alongside global ones")
	})

	t.Run("top-level flat lists apply to printers (HA options.json)", func(t *testing.T) {
		t.Parallel()

		// Shape written by the add-on schema: notifiers only at
		// the top level, printers without notifier lists.
		optionsJSON := `{
  "log": {"level": "info", "format": "text"},
  "tail": false,
  "printer": [
    {
      "name": "SAIL X1C",
      "host": "10.196.68.126",
      "access_code": "965f267e",
      "serial_number": "00M09D562300170",
      "tls_insecure_skip_verify": true
    }
  ],
  "slack": [
    {"webhook_url": "https://hooks.slack.com/services/T/B/x"}
  ]
}`
		cfg, err := Load(writeConfigNamed(t, "options.json", optionsJSON))
		require.NoError(t, err)

		require.Len(t, cfg.Printers, 1)
		require.Len(t, cfg.Printers[0].Notifiers.Slack, 1)
		assert.Empty(t, cfg.Printers[0].Notifiers.Discord)
		assert.Empty(t, cfg.Printers[0].Notifiers.Telegram)
		assert.Nil(t, cfg.FlatSlack)
	})

	t.Run("loads notifier event filters", func(t *testing.T) {
		t.Parallel()

		cfg, err := Load(writeConfig(t, `
[[notifiers.slack]]
webhook_url = "https://hooks.slack.com/services/T/B/x"
events      = ["print_started", "print_finished", "print_failed"]

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`))
		require.NoError(t, err)

		require.Len(t, cfg.Printers[0].Notifiers.Slack, 1)
		assert.Equal(t,
			[]string{
				"print_started", "print_finished", "print_failed",
			},
			cfg.Printers[0].Notifiers.Slack[0].Events,
		)
	})

	t.Run("rejects unknown event types", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, `
[[notifiers.slack]]
webhook_url = "https://hooks.slack.com/services/T/B/x"
events      = ["print_done"]

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			`notifiers.slack[0].events[0]: unknown event type "print_done"`)
	})

	t.Run("validates global notifiers", func(t *testing.T) {
		t.Parallel()

		_, err := Load(writeConfig(t, `
[[notifiers.discord]]
name = "d"

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`))
		require.Error(t, err)

		assert.Contains(t, err.Error(),
			"notifiers.discord[0]: webhook_url is required")
	})
}

func TestMergeDiscovered(t *testing.T) {
	t.Parallel()

	discord := Discord{
		Name:       "d",
		WebhookURL: "https://discord.com/api/webhooks/1/a",
	}

	t.Run("adds discovered printers with discovery settings", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Discovery: Discovery{
				Enabled:    true,
				CameraPort: 6000,
				Notifiers:  Notifiers{Discord: []Discord{discord}},
			},
		}

		added := cfg.MergeDiscovered([]DiscoveredPrinter{
			{
				Name:       "Workshop P1S",
				Host:       "192.168.1.50",
				Serial:     "AABBCC112233",
				AccessCode: "12345678",
			},
		})

		assert.Equal(t, 1, added)
		require.Len(t, cfg.Printers, 1)

		p := cfg.Printers[0]
		assert.Equal(t, "Workshop P1S", p.Name)
		assert.Equal(t, 8883, p.Port, "defaults applied to discovered printers")
		assert.Equal(t, 30, p.ReconnectDelaySeconds)
		assert.Equal(t, 6000, p.CameraPort)
		assert.True(t, p.TLSInsecureSkipVerify)
		require.Len(t, p.Notifiers.Discord, 1)
	})

	t.Run("manual printer wins on serial collision", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Printers: []Printer{
				{
					Name:         "Manual",
					Host:         "10.0.0.1",
					SerialNumber: "AABBCC112233",
					AccessCode:   "T1",
				},
			},
		}

		added := cfg.MergeDiscovered([]DiscoveredPrinter{
			{
				Name:       "Discovered",
				Host:       "192.168.1.50",
				Serial:     "AABBCC112233",
				AccessCode: "12345678",
			},
			{
				Name:       "New",
				Host:       "192.168.1.51",
				Serial:     "DDEEFF445566",
				AccessCode: "87654321",
			},
		})

		assert.Equal(t, 1, added)
		require.Len(t, cfg.Printers, 2)
		assert.Equal(t, "Manual", cfg.Printers[0].Name)
		assert.Equal(t, "New", cfg.Printers[1].Name)
	})

	t.Run("camera port only applies to chamber-image models", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Discovery: Discovery{Enabled: true, CameraPort: 6000},
		}

		added := cfg.MergeDiscovered([]DiscoveredPrinter{
			{Name: "P1S", Host: "10.0.0.1", Serial: "S1",
				AccessCode: "a", Model: "P1S"},
			{Name: "X1C", Host: "10.0.0.2", Serial: "S2",
				AccessCode: "b", Model: "X1C"},
			{Name: "H2D", Host: "10.0.0.3", Serial: "S3",
				AccessCode: "c", Model: "H2D"},
			{Name: "Unknown", Host: "10.0.0.4", Serial: "S4",
				AccessCode: "d"},
		})

		assert.Equal(t, 4, added)
		assert.Equal(t, 6000, cfg.Printers[0].CameraPort, "P1S supports chamber image")
		assert.False(t, cfg.Printers[0].CameraRTSP)
		assert.Equal(t, 0, cfg.Printers[1].CameraPort, "X1C uses RTSPS")
		assert.True(t, cfg.Printers[1].CameraRTSP)
		assert.Equal(t, 0, cfg.Printers[2].CameraPort, "H2D uses RTSPS")
		assert.True(t, cfg.Printers[2].CameraRTSP)
		assert.Equal(t, 6000, cfg.Printers[3].CameraPort, "unknown model still attempts")
		assert.False(t, cfg.Printers[3].CameraRTSP)
	})

	t.Run("discovered printers get discovery and global notifiers", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Discovery: Discovery{
				Enabled:   true,
				Notifiers: Notifiers{Discord: []Discord{discord}},
			},
			Notifiers: Notifiers{
				Slack: []Slack{{
					WebhookURL: "https://hooks.slack.com/services/T/B/x",
				}},
			},
		}

		added := cfg.MergeDiscovered([]DiscoveredPrinter{
			{
				Name:       "Workshop P1S",
				Host:       "192.168.1.50",
				Serial:     "AABBCC112233",
				AccessCode: "12345678",
			},
		})

		assert.Equal(t, 1, added)
		require.Len(t, cfg.Printers, 1)
		assert.Len(t, cfg.Printers[0].Notifiers.Discord, 1)
		assert.Len(t, cfg.Printers[0].Notifiers.Slack, 1)
	})
}

func TestLoad_missingPrinterFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "missing printer name",
			config: `
[[printer]]
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`,
			wantErr: "printer[0]: name is required",
		},
		{
			name: "missing printer host",
			config: `
[[printer]]
name          = "P1"
serial_number = "S1"
access_code   = "T1"
`,
			wantErr: "printer[0]: host is required",
		},
		{
			name: "missing printer serial_number",
			config: `
[[printer]]
name        = "P1"
host        = "10.0.0.1"
access_code = "T1"
`,
			wantErr: "printer[0]: serial_number is required",
		},
		{
			name: "missing printer access_code",
			config: `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
`,
			wantErr: "printer[0]: access_code is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeConfig(t, tt.config))
			require.Error(t, err)

			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_missingNotifierFields(t *testing.T) {
	t.Parallel()

	base := func(notifierBlock string) string {
		return `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
` + notifierBlock
	}

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "missing discord webhook_url",
			config: base(`
  [[printer.notifiers.discord]]
  name = "test"
`),
			wantErr: "printer[0].notifiers.discord[0]: webhook_url is required",
		},
		{
			name: "missing slack webhook_url",
			config: base(`
  [[printer.notifiers.slack]]
  name = "test"
`),
			wantErr: "printer[0].notifiers.slack[0]: webhook_url is required",
		},
		{
			name: "missing telegram bot_token",
			config: base(`
  [[printer.notifiers.telegram]]
  name    = "test"
  chat_id = "-100"
`),
			wantErr: "printer[0].notifiers.telegram[0]: bot_token is required",
		},
		{
			name: "missing telegram chat_id",
			config: base(`
  [[printer.notifiers.telegram]]
  name      = "test"
  bot_token = "123:ABC"
`),
			wantErr: "printer[0].notifiers.telegram[0]: chat_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeConfig(t, tt.config))
			require.Error(t, err)

			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_envOverrides(t *testing.T) {
	validConfig := `
[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"

[log]
level  = "info"
format = "text"
`

	t.Run("LOG_LEVEL overrides config value", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "debug")

		cfg, err := Load(writeConfig(t, validConfig))
		require.NoError(t, err)

		assert.Equal(t, LogLevelDebug, cfg.Log.Level)
	})

	t.Run("LOG_FORMAT overrides config value", func(t *testing.T) {
		t.Setenv("LOG_FORMAT", "json")

		cfg, err := Load(writeConfig(t, validConfig))
		require.NoError(t, err)

		assert.Equal(t, LogFormatJSON, cfg.Log.Format)
	})

	t.Run("env overrides are case-insensitive", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "WARN")

		cfg, err := Load(writeConfig(t, validConfig))
		require.NoError(t, err)

		assert.Equal(t, LogLevelWarn, cfg.Log.Level)
	})
}

func TestLoad_invalidLogValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "invalid log level",
			config: `
[log]
level = "verbose"

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`,
			wantErr: `invalid log level "verbose"`,
		},
		{
			name: "invalid log format",
			config: `
[log]
format = "xml"

[[printer]]
name          = "P1"
host          = "10.0.0.1"
serial_number = "S1"
access_code   = "T1"
`,
			wantErr: `invalid log format "xml"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeConfig(t, tt.config))
			require.Error(t, err)

			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
