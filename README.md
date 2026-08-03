# bambu-notifier

[![CI](https://github.com/l33t0/bambu-notifier/actions/workflows/ci.yml/badge.svg)](https://github.com/l33t0/bambu-notifier/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Daemon that connects to Bambu Lab 3D printers via local MQTT (TLS, port 8883) and dispatches notifications to Discord, Slack, and Telegram when print lifecycle events occur.

## How It Works

```
 ┌──────────┐   MQTT/TLS    ┌────────────────┐   webhooks   ┌──────────┐
 │  Bambu   │──────────────▶│                │─────────────▶│ Discord  │
 │  Printer │   port 8883   │ bambu-notifier │─────────────▶│ Slack    │
 └──────────┘               │                │─────────────▶│ Telegram │
                            └────────────────┘              └──────────┘
```

bambu-notifier connects to each printer's local MQTT broker over TLS, subscribes to status reports, and tracks state transitions. When a print lifecycle event is detected (start, finish, failure, pause, etc.), it fans out notifications concurrently to all configured channels.

Each printer runs as an independent goroutine with its own MQTT connection and automatic reconnection with exponential backoff.

## Quick Start

### Home Assistant Add-on

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**, open the
   **⋮** menu → **Repositories**, and add
   `https://github.com/christhechris/ha-bambu-notifier`.
2. Install the **Bambu Notifier** add-on.
3. Configure your printers on the add-on's Configuration tab (YAML editor)
   and start it — see the add-on's
   [documentation](bambu-notifier/DOCS.md) for the options format.

### Docker Compose

```yaml
services:
  bambu-notifier:
    image: ghcr.io/l33t0/bambu-notifier:latest
    restart: unless-stopped
    volumes:
      - ./config.toml:/config/config.toml:ro
```

```bash
docker compose up -d
```

### Build from Source

```bash
go install github.com/l33t0/bambu-notifier/cmd/bambu-notifier@latest
bambu-notifier --config config.toml
```

Or clone and build manually:

```bash
git clone https://github.com/l33t0/bambu-notifier.git
cd bambu-notifier
go build -o bin/bambu-notifier ./cmd/bambu-notifier
./bin/bambu-notifier --config config.toml
```

## Configuration

Copy `config.example.toml` to `config.toml` and fill in your values:

```toml
[log]
format = "text"   # "text" or "json"
level  = "info"   # "debug", "info", "warn", "error"

[[printer]]
name                     = "Bambu H2S Combo"
host                     = "192.168.1.50"
serial_number            = "AABBCC112233"
access_code              = "12345678"      # 8-digit code from printer LCD
tls_insecure_skip_verify = true            # required — Bambu uses self-signed certs

[printer.notifiers]
  [[printer.notifiers.discord]]
  name        = "print-alerts"
  webhook_url = "https://discord.com/api/webhooks/..."

  [[printer.notifiers.slack]]
  name        = "ops-channel"
  webhook_url = "https://hooks.slack.com/services/..."

  [[printer.notifiers.telegram]]
  name      = "home-alerts"
  bot_token = "123456:ABC-DEF..."
  chat_id   = "-100123456789"
```

Multiple `[[printer]]` blocks run as independent, concurrent MQTT connections. Each printer fans out to its own set of notifiers.

See [`config.example.toml`](config.example.toml) for all available options with defaults.

### Optional Fields

| Field | Default | Description |
|---|---|---|
| `port` | `8883` | MQTT broker port |
| `reconnect_delay_seconds` | `30` | Base delay before reconnection (exponential backoff, max 5 min) |
| `secret` | `""` | HMAC-SHA256 signing secret for webhook payloads |

### Environment Variables

| Variable | Overrides | Values |
|---|---|---|
| `LOG_FORMAT` | `[log] format` | `text`, `json` |
| `LOG_LEVEL` | `[log] level` | `debug`, `info`, `warn`, `error` |

## Supported Events

| Event | Trigger |
|---|---|
| `print_started` | State transitions to `RUNNING` or `PREPARE` (not from `PAUSE`) |
| `print_finished` | State transitions to `FINISH` |
| `print_failed` | State transitions to `FAILED` |
| `print_paused` | State transitions to `PAUSE` |
| `print_resumed` | State transitions from `PAUSE` to `RUNNING` |
| `critical_error` | `print_error` becomes non-zero (deduplicated per error code) |
| `filament_runout` | Error code `0x07008001` detected |
| `ams_change` | Active AMS tray changes between reports |
| `nozzle_temp_anomaly` | Nozzle temp deviates >20 °C from target while printing |
| `hms_alert` | New HMS error entries appear (clears when resolved, re-fires on recurrence) |

All events are deduplicated — repeated identical states do not re-fire.

## Webhook Setup Guides

- [Discord Webhook Setup](docs/discord-webhook.md)
- [Slack Webhook Setup](docs/slack-webhook.md)
- [Telegram Bot Setup](docs/telegram-setup.md)

## Development

```bash
go test -race ./...            # run tests
go vet ./...                   # static analysis
golangci-lint run              # lint
go build -o bin/bambu-notifier ./cmd/bambu-notifier
```

## License

[MIT](LICENSE)
