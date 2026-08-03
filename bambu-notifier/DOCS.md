# Bambu Notifier

Watches your Bambu Lab printers over their local MQTT interface (TLS, port
8883) and sends notifications to Discord, Slack, and/or Telegram when print
lifecycle events occur: start, finish, failure, pause/resume, filament
runout, HMS alerts, and more. If a camera port is configured, start/finish/
failed/paused notifications include a camera snapshot.

The add-on talks directly to each printer on your LAN — it does not use
Home Assistant's MQTT broker and opens no ports.

## Configuration

Because printers are a nested list, Home Assistant shows this add-on's
configuration in the YAML editor (three-dots menu → *Edit in YAML*).

```yaml
log:
  level: info        # debug | info | warn | error
  format: text       # text | json
tail: false          # log every raw MQTT report (debugging)
printer:
  - name: Bambu H2S Combo
    host: 192.168.1.50
    serial_number: AABBCC112233
    access_code: "12345678"          # 8-digit code from the printer LCD
    tls_insecure_skip_verify: true   # required — Bambu uses self-signed certs
    camera_port: 6000                # omit or 0 to disable snapshots
    discord:
      - name: print-alerts
        webhook_url: https://discord.com/api/webhooks/...
    slack: []
    telegram:
      - name: home-alerts
        bot_token: "123456:ABC-DEF..."
        chat_id: "-100123456789"
```

The add-on will log a configuration error and stop until at least one
printer is configured. If you use only some notifier types, set the others
to an empty list (`[]`).

### Printer options

| Option | Required | Default | Description |
|---|---|---|---|
| `name` | yes | — | Display name used in notifications and logs |
| `host` | yes | — | Printer IP or hostname on your LAN |
| `serial_number` | yes | — | Printer serial (Settings → Device on the printer) |
| `access_code` | yes | — | LAN access code from the printer LCD |
| `port` | no | `8883` | MQTT/TLS port |
| `tls_insecure_skip_verify` | no | `false` | Set `true` — Bambu printers use self-signed certificates |
| `camera_port` | no | `0` | Camera snapshot port (typically `6000`); `0` disables snapshots |
| `reconnect_delay_seconds` | no | `30` | Base reconnect delay (exponential backoff, capped at 5 min) |

### Notifier options

Each printer takes `discord`, `slack`, and `telegram` lists:

- **discord** / **slack**: `webhook_url` (required), `name`, and optional
  `secret` — when set, payloads are signed with HMAC-SHA256 in an
  `X-Hub-Signature-256` header.
- **telegram**: `bot_token` and `chat_id` (both required), optional `name`.

Setup guides for each service are in the
[repository docs](https://github.com/christhechris/ha-bambu-notifier/tree/master/docs).

### Global options

| Option | Default | Description |
|---|---|---|
| `log.level` | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | `text` | `text` or `json` |
| `tail` | `false` | Also log every parsed MQTT report — useful when debugging a printer connection |

## Troubleshooting

- **"at least one [[printer]] is required"** — the printer list is empty;
  add a printer in the configuration.
- **Connection refused / TLS errors** — check the printer is in LAN Mode
  (or LAN access enabled), the access code matches the LCD, and
  `tls_insecure_skip_verify: true` is set.
- Set `log.level: debug` and `tail: true` to see every report the printer
  sends.
