# Changelog

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
