# bambu-notifier

Daemon that connects to Bambu Lab 3D printers via local MQTT (TLS, port 8883) and dispatches notifications to Discord, Slack, and Telegram when print lifecycle events occur.

## Quick Start

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

### Binary

```bash
go install github.com/l33t0/bambu-notifier/cmd/bambu-notifier@latest
bambu-notifier --config config.toml
```

## Configuration

Create a `config.toml` (see `config.example.toml` for a full reference):

```toml
[log]
format = "text"   # or "json" — overridden by LOG_FORMAT env var
level  = "info"   # overridden by LOG_LEVEL env var

[[printer]]
name                     = "Bambu H2S Combo"
host                     = "192.168.1.50"
port                     = 8883           # default, optional
serial_number            = "AABBCC112233"
access_code              = "12345678"
tls_insecure_skip_verify = true           # Bambu uses self-signed certs
reconnect_delay_seconds  = 30             # default, optional

# Each printer can fan out to multiple notifiers simultaneously.

[printer.notifiers]
  [[printer.notifiers.discord]]
  name        = "print-alerts"
  webhook_url = "https://discord.com/api/webhooks/..."
  secret      = ""   # optional HMAC-SHA256 signing secret

  [[printer.notifiers.slack]]
  name        = "ops-channel"
  webhook_url = "https://hooks.slack.com/services/..."
  secret      = ""

  [[printer.notifiers.telegram]]
  name      = "home-alerts"
  bot_token = "123456:ABC-DEF..."
  chat_id   = "-100123456789"
```

Multiple `[[printer]]` blocks run as independent, concurrent MQTT connections.

## Environment Variables

| Variable     | Default | Description                              |
|-------------|---------|------------------------------------------|
| `LOG_FORMAT` | `text`  | Log output format: `json` or `text`      |
| `LOG_LEVEL`  | `info`  | Log verbosity: `debug`, `info`, `warn`, `error` |

Environment variables override values set in `config.toml`.

## Supported Events

| Event                 | Trigger                                                       |
|-----------------------|---------------------------------------------------------------|
| `print_started`       | `gcode_state` transitions to `RUNNING` (not from `PAUSE`)    |
| `print_finished`      | `gcode_state` transitions to `FINISH`                         |
| `print_failed`        | `gcode_state` transitions to `FAILED`                         |
| `print_paused`        | `gcode_state` transitions to `PAUSE`                          |
| `print_resumed`       | `gcode_state` transitions from `PAUSE` to `RUNNING`           |
| `critical_error`      | `print_error` field becomes non-zero (fires once per error)   |
| `filament_runout`     | Error code `0x07008001` while printing                        |
| `ams_change`          | Active AMS slot changes between reports                       |
| `nozzle_temp_anomaly` | Nozzle temp deviates >20 °C from target while printing        |
| `hms_alert`           | New HMS error entries appear                                  |

Duplicate events are suppressed — repeated identical states do not re-fire.

## Webhook Setup Guides

- [Discord Webhook Setup](docs/discord-webhook.md)
- [Slack Webhook Setup](docs/slack-webhook.md)

## License

MIT
