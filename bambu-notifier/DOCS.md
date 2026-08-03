# Bambu Notifier

Watches your Bambu Lab printers over their local MQTT interface (TLS, port
8883) and sends notifications to Discord, Slack, and/or Telegram when print
lifecycle events occur: start, finish, failure, pause/resume, filament
runout, HMS alerts, and more. If a camera port is configured, start/finish/
failed/paused notifications include a camera snapshot.

The add-on talks directly to each printer on your LAN — it does not use
Home Assistant's MQTT broker and opens no ports.

## Configuration

Configure at least one notifier (`discord`, `slack`, or `telegram` — any
combination; leave unused ones empty), and either enable discovery or list
printers manually. Because printers are a nested list, Home Assistant shows
this add-on's configuration in the YAML editor (three-dots menu →
*Edit in YAML*).

Notifiers are configured once at the top level and apply to **all**
printers.

### Automatic printer setup via ha-bambulab

If you use the [ha-bambulab](https://github.com/greghesp/ha-bambulab)
integration, the add-on can import your printers from it — no manual
printer entries needed:

```yaml
discovery:
  enabled: true
  camera_port: 6000   # optional: snapshots for imported P1/A1 printers
slack:
  - webhook_url: https://hooks.slack.com/services/...
discord: []
telegram: []
printer: []
```

At startup the add-on reads the integration's stored printer list (name,
host, serial, access code) and connects to each. Printers set up
cloud-only in ha-bambulab (no LAN host/access code) are skipped with a
warning — this add-on needs LAN access. Restart the add-on after
adding/changing printers in ha-bambulab.

A manual `printer` entry with the same serial number takes precedence
over the discovered one.

### Manual printer setup

```yaml
log:
  level: info        # debug | info | warn | error
  format: text       # text | json
tail: false          # log every raw MQTT report (debugging)
discovery:
  enabled: false
discord:
  - name: print-alerts
    webhook_url: https://discord.com/api/webhooks/...
slack: []
telegram:
  - name: home-alerts
    bot_token: "123456:ABC-DEF..."
    chat_id: "-100123456789"
printer:
  - name: Bambu H2S Combo
    host: 192.168.1.50
    serial_number: AABBCC112233
    access_code: "12345678"          # 8-digit code from the printer LCD
    tls_insecure_skip_verify: true   # required — Bambu uses self-signed certs
    camera_port: 6000                # omit or 0 to disable snapshots
```

### Printer options

| Option | Required | Default | Description |
|---|---|---|---|
| `name` | yes | — | Display name used in notifications and logs |
| `host` | yes | — | Printer IP or hostname on your LAN |
| `serial_number` | yes | — | Printer serial (Settings → Device on the printer) |
| `access_code` | yes | — | LAN access code from the printer LCD |
| `port` | no | `8883` | MQTT/TLS port |
| `tls_insecure_skip_verify` | no | `false` | Set `true` — Bambu printers use self-signed certificates |
| `camera_port` | no | `0` | Camera snapshot port (typically `6000`); `0` disables snapshots. **P1/A1 series only** — X1/H2 series expose an RTSPS video stream instead, which this add-on does not support |
| `reconnect_delay_seconds` | no | `30` | Base reconnect delay (exponential backoff, capped at 5 min) |

### Notifier options

- **discord** / **slack**: `webhook_url` (required), optional `name`, and
  optional `secret` — when set, payloads are signed with HMAC-SHA256 in an
  `X-Hub-Signature-256` header.
- **telegram**: `bot_token` and `chat_id` (both required), optional `name`.

Every notifier also takes an optional `events` list to limit which
event types it receives. Omit it to receive everything. Example —
only start, finish, and stopped-due-to-error:

```yaml
slack:
  - webhook_url: https://hooks.slack.com/services/...
    events:
      - print_started
      - print_finished
      - print_failed
```

Valid values: `print_started`, `print_finished`, `print_failed`,
`print_paused`, `print_resumed`, `critical_error`, `filament_runout`,
`ams_change`, `nozzle_temp_anomaly`, `hms_alert`. See the repository
README's "Supported Events" table for what triggers each.

Setup guides for each service are in the
[repository docs](https://github.com/christhechris/ha-bambu-notifier/tree/master/docs).

Per-printer notifier routing is available when running the daemon
standalone with a TOML config; the add-on applies the global lists to
every printer.

### Global options

| Option | Default | Description |
|---|---|---|
| `log.level` | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | `text` | `text` or `json` |
| `tail` | `false` | Also log every parsed MQTT report — useful when debugging a printer connection |
| `discovery.enabled` | `false` | Import printers from the ha-bambulab integration |
| `discovery.camera_port` | `0` | Camera snapshot port for imported printers (`6000` to enable). Applied only to P1/A1-series models; X1/H2 series are skipped automatically |

## Troubleshooting

- **"no printers configured and none discovered"** — the printer list is
  empty and discovery found nothing; add a printer or set up
  ha-bambulab with LAN access.
- **Connection refused / TLS errors** — check the printer is in LAN Mode
  (or LAN access enabled), the access code matches the LCD, and
  `tls_insecure_skip_verify: true` is set.
- Set `log.level: debug` and `tail: true` to see every report the printer
  sends.
