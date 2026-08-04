# Bambu Notifier — Home Assistant Add-on

[![CI](https://github.com/christhechris/ha-bambu-notifier/actions/workflows/ci.yml/badge.svg)](https://github.com/christhechris/ha-bambu-notifier/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Home Assistant add-on that watches your Bambu Lab printers over their
**local** MQTT (TLS, port 8883) and sends Discord, Slack, and Telegram
notifications for print lifecycle events — start, finish, failure,
pause (with the reason), filament runout, HMS alerts, and more. No
cloud dependency, no ports opened, and it does not use Home
Assistant's MQTT broker.

Based on [l33t0/bambu-notifier](https://github.com/l33t0/bambu-notifier),
which provides the standalone daemon this add-on is built around.

## Highlights

- **Zero-config printers** — enable discovery and the add-on imports
  every printer (host, serial, access code) already set up in the
  [ha-bambulab](https://github.com/greghesp/ha-bambulab) integration.
- **Any notifier combination** — Discord, Slack, and Telegram in any
  mix, with optional per-notifier event filtering
  (`events: [print_started, print_finished, print_failed]`).
- **Compact messages** — two lines: event + printer, then
  filename · progress · ETA, plus the pause reason or error when
  there is one.
- **Camera snapshots** (optional) — attached to start/finish/failed/
  paused notifications on Discord and Telegram. P1/A1 series use the
  chamber-image port; X1/X2/H2/P2 series use an ffmpeg grab from
  their RTSPS stream (X1 requires the LAN Only Liveview toggle).
- **Signed webhooks** — optional HMAC-SHA256 `X-Hub-Signature-256`
  signatures for Discord/Slack payloads.

## Installation

[![Open your Home Assistant instance and add this add-on repository.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fchristhechris%2Fha-bambu-notifier)

Or manually: **Settings → Add-ons → Add-on Store → ⋮ → Repositories**,
add `https://github.com/christhechris/ha-bambu-notifier`, then install
**Bambu Notifier**.

## Configuration

With ha-bambulab installed, the minimal setup is discovery plus one
notifier (add-on Configuration tab, YAML editor):

```yaml
discovery:
  enabled: true
slack:
  - webhook_url: https://hooks.slack.com/services/...
    events: [print_started, print_finished, print_failed]
discord: []
telegram: []
printer: []
```

Printers can also be listed manually, and notifiers apply to all
printers. See the add-on's [full documentation](bambu-notifier/DOCS.md)
for every option, and the setup guides for each service:

- [Discord Webhook Setup](docs/discord-webhook.md)
- [Slack Webhook Setup](docs/slack-webhook.md)
- [Telegram Bot Setup](docs/telegram-setup.md)

## Supported Events

| Event | Trigger |
|---|---|
| `print_started` | State transitions to `RUNNING` or `PREPARE` (not from `PAUSE`) |
| `print_finished` | State transitions to `FINISH` |
| `print_failed` | State transitions to `FAILED` |
| `print_paused` | State transitions to `PAUSE` — includes the pause reason when the printer reports one (filament runout, user pause, nozzle clog, …) |
| `print_resumed` | State transitions from `PAUSE` to `RUNNING` |
| `critical_error` | `print_error` becomes non-zero (deduplicated per error code) |
| `filament_runout` | Error code `0x07008001` detected |
| `ams_change` | Active AMS tray changes between reports |
| `nozzle_temp_anomaly` | Nozzle temp deviates >20 °C from target while printing |
| `hms_alert` | New HMS error entries appear (clears when resolved, re-fires on recurrence) |

All events are deduplicated — repeated identical states do not
re-fire. Use a notifier's `events` list to subscribe to a subset.

## Standalone Use

The underlying daemon also runs outside Home Assistant (Docker or a
plain binary) with a TOML config — see
[`config.example.toml`](config.example.toml) and the upstream
[l33t0/bambu-notifier](https://github.com/l33t0/bambu-notifier) project
for that usage. RTSP camera snapshots additionally need `ffmpeg` on
the PATH.

## Development

```bash
go build -o bin/bambu-notifier ./cmd/bambu-notifier
go test -race ./...
golangci-lint run
```

Releases: bump `bambu-notifier/config.yaml` version + `CHANGELOG.md`,
tag `vX.Y.Z`, push — CI builds and publishes the per-arch add-on
images to GHCR.

## License

[MIT](LICENSE)
