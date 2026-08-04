# Changelog

## 1.4.0

- Paused notifications now include the reason when the printer reports
  one — e.g. "filament runout", "paused by user", "nozzle clog
  detected", "first layer defect detected" — derived from the
  printer's reported stage, with active HMS alerts or the raw error
  code as fallbacks. The webhook JSON gains an optional `reason`
  field.

## 1.3.1

- Notification messages are now compact — two lines: event and printer
  on the first, then filename · progress · ETA (and the error message
  for failure events). Thermals, nozzle type, source, and AMS details
  are no longer included. Snapshots attach as before.

## 1.3.0

- Camera snapshots now work on X1/X2/H2/P2-series printers via their
  RTSPS live stream (ffmpeg frame grab; ffmpeg is bundled in the
  add-on image). With discovery, `discovery.camera_port` now enables
  snapshots on every printer: P1/A1 models use the JPEG chamber-image
  port as before, RTSP models grab a frame from their stream. Manual
  printers use the new `camera_rtsp: true` option. Requires LAN Mode
  Liveview enabled on X1-series printers.

## 1.2.2

- Camera snapshots: the port-6000 JPEG protocol only exists on P1/A1
  series printers — X1/H2 series stream RTSPS instead (not supported).
  Discovery now applies `discovery.camera_port` only to P1/A1-series
  models, and a printer that rejects the camera TLS handshake disables
  snapshots for that printer with one explanatory message instead of
  warning on every event.

## 1.2.1

- Fix: X1-series printers dropped the connection ~2 seconds after
  subscribe, in a loop. The daemon used the printer serial as its MQTT
  client id, colliding with the printer firmware's own internal MQTT
  client and causing a session-takeover fight. A unique random client
  id is now used per connection.

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
