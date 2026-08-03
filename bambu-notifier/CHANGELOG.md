# Changelog

## 1.2.0

- Notifiers accept an optional `events` list to choose which event
  types they receive (e.g. `events: [print_started, print_finished,
  print_failed]`). Omitted = all events, as before.

## 1.1.1

- Fix: H2D (and other newer-firmware) MQTT reports were dropped because
  some fields changed JSON type between printer generations (e.g.
  `insert_flag` sent as bool instead of string). The parser now accepts
  string/number/bool variants for all scalar fields instead of
  discarding the whole report.
- When a printer accepts the connection but drops it immediately
  without sending reports, the log now explains the likely cause
  (X1 series firmware requiring LAN Mode Developer Mode for
  third-party access) instead of a bare EOF.

## 1.1.0

- Notifiers are now configured once at the top level (`discord`, `slack`,
  `telegram`) and apply to all printers. Any combination works and unused
  ones can be left empty — this fixes "Missing option 'telegram'" errors
  when saving the configuration.
- New: automatic printer import from the
  [ha-bambulab](https://github.com/greghesp/ha-bambulab) integration via
  `discovery.enabled: true` — no manual printer entries needed.

**Breaking**: notifier lists inside `printer` entries are no longer
accepted by the add-on configuration — move them to the top level.

## 1.0.0

- Initial release of the Home Assistant add-on.
- Multi-printer support over local MQTT/TLS with automatic reconnection.
- Discord, Slack, and Telegram notifiers with optional HMAC payload signing.
- Camera snapshots on start/finish/failed/paused events.
