package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// LogFormat represents the log output format.
type LogFormat string

// Log output format options.
const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// LogLevel represents the log severity level.
type LogLevel string

// Log verbosity levels.
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Log holds logging configuration.
type Log struct {
	Level  LogLevel  `toml:"level" json:"level"`
	Format LogFormat `toml:"format" json:"format"`
}

// Discord holds Discord webhook notifier configuration.
type Discord struct {
	Name       string `toml:"name" json:"name"`
	WebhookURL string `toml:"webhook_url" json:"webhook_url"`
	Secret     string `toml:"secret" json:"secret"`
}

// Slack holds Slack webhook notifier configuration.
type Slack struct {
	Name       string `toml:"name" json:"name"`
	WebhookURL string `toml:"webhook_url" json:"webhook_url"`
	Secret     string `toml:"secret" json:"secret"`
}

// Telegram holds Telegram bot notifier configuration.
type Telegram struct {
	Name     string `toml:"name" json:"name"`
	BotToken string `toml:"bot_token" json:"bot_token"`
	ChatID   string `toml:"chat_id" json:"chat_id"`
}

// Notifiers holds the notifier configurations for a single printer.
type Notifiers struct {
	Discord  []Discord  `toml:"discord" json:"discord"`
	Slack    []Slack    `toml:"slack" json:"slack"`
	Telegram []Telegram `toml:"telegram" json:"telegram"`
}

// Printer holds the configuration for a single Bambu Lab printer.
type Printer struct {
	Name                  string    `toml:"name" json:"name"`
	Host                  string    `toml:"host" json:"host"`
	Port                  int       `toml:"port" json:"port"`
	SerialNumber          string    `toml:"serial_number" json:"serial_number"`
	AccessCode            string    `toml:"access_code" json:"access_code"`
	CameraPort            int       `toml:"camera_port" json:"camera_port"`
	TLSInsecureSkipVerify bool      `toml:"tls_insecure_skip_verify" json:"tls_insecure_skip_verify"`
	ReconnectDelaySeconds int       `toml:"reconnect_delay_seconds" json:"reconnect_delay_seconds"`
	Notifiers             Notifiers `toml:"notifiers" json:"notifiers"`

	// Flattened notifier lists as written by Home Assistant's
	// options.json — the Supervisor schema cannot nest a dict
	// inside printer list entries. Merged into Notifiers by Load.
	FlatDiscord  []Discord  `toml:"-" json:"discord"`
	FlatSlack    []Slack    `toml:"-" json:"slack"`
	FlatTelegram []Telegram `toml:"-" json:"telegram"`
}

// Discovery configures importing printers from the ha-bambulab Home
// Assistant integration's config entry storage. Discovered printers
// get the notifier lists and camera port configured here.
type Discovery struct {
	Enabled    bool      `toml:"enabled" json:"enabled"`
	Path       string    `toml:"path" json:"path"`
	CameraPort int       `toml:"camera_port" json:"camera_port"`
	Notifiers  Notifiers `toml:"notifiers" json:"notifiers"`

	// Flattened notifier lists for HA options.json (same reason as
	// Printer's Flat* fields). Merged into Notifiers by Load.
	FlatDiscord  []Discord  `toml:"-" json:"discord"`
	FlatSlack    []Slack    `toml:"-" json:"slack"`
	FlatTelegram []Telegram `toml:"-" json:"telegram"`
}

// Config is the top-level application configuration. The top-level
// Notifiers apply to every printer (manual and discovered) in addition
// to any per-printer lists; the HA add-on only exposes these global
// lists because the Supervisor schema cannot mark lists nested in
// printer entries as optional.
type Config struct {
	Log       Log       `toml:"log" json:"log"`
	Tail      bool      `toml:"tail" json:"tail"`
	Discovery Discovery `toml:"discovery" json:"discovery"`
	Notifiers Notifiers `toml:"notifiers" json:"notifiers"`
	Printers  []Printer `toml:"printer" json:"printer"`

	// Flattened global notifier lists for HA options.json.
	FlatDiscord  []Discord  `toml:"-" json:"discord"`
	FlatSlack    []Slack    `toml:"-" json:"slack"`
	FlatTelegram []Telegram `toml:"-" json:"telegram"`
}

// DiscoveredPrinter is a printer imported from an external source such
// as the ha-bambulab integration.
type DiscoveredPrinter struct {
	Name       string
	Host       string
	Serial     string
	AccessCode string
}

// MergeDiscovered appends discovered printers that are not already
// configured (matched by serial number), attaching the discovery
// notifier lists and camera port. TLS verification is disabled for
// discovered printers since Bambu printers use self-signed
// certificates. Returns the number of printers added.
func (cfg *Config) MergeDiscovered(ds []DiscoveredPrinter) int {
	known := make(map[string]bool, len(cfg.Printers))
	for _, p := range cfg.Printers {
		known[p.SerialNumber] = true
	}

	notifiers := combineNotifiers(
		cfg.Discovery.Notifiers, cfg.Notifiers,
	)

	added := 0
	for _, d := range ds {
		if known[d.Serial] {
			continue
		}
		known[d.Serial] = true

		cfg.Printers = append(cfg.Printers, Printer{
			Name:                  d.Name,
			Host:                  d.Host,
			SerialNumber:          d.Serial,
			AccessCode:            d.AccessCode,
			CameraPort:            cfg.Discovery.CameraPort,
			TLSInsecureSkipVerify: true,
			Notifiers:             notifiers,
		})
		added++
	}

	applyDefaults(cfg)

	return added
}

// Load reads a TOML or JSON configuration file from path (JSON when
// the path ends in .json, e.g. Home Assistant's /data/options.json),
// applies environment variable overrides, and validates required
// fields.
func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}

	if strings.HasSuffix(strings.ToLower(path), ".json") {
		err = json.Unmarshal(data, &cfg)
	} else {
		err = toml.Unmarshal(data, &cfg)
	}
	if err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}

	mergeFlatNotifiers(&cfg)
	applyGlobalNotifiers(&cfg)
	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := validate(cfg); err != nil {
		return cfg, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

func mergeFlatNotifiers(cfg *Config) {
	for i := range cfg.Printers {
		p := &cfg.Printers[i]
		p.Notifiers.Discord = append(
			p.Notifiers.Discord, p.FlatDiscord...,
		)
		p.Notifiers.Slack = append(
			p.Notifiers.Slack, p.FlatSlack...,
		)
		p.Notifiers.Telegram = append(
			p.Notifiers.Telegram, p.FlatTelegram...,
		)
		p.FlatDiscord = nil
		p.FlatSlack = nil
		p.FlatTelegram = nil
	}

	d := &cfg.Discovery
	d.Notifiers.Discord = append(
		d.Notifiers.Discord, d.FlatDiscord...,
	)
	d.Notifiers.Slack = append(
		d.Notifiers.Slack, d.FlatSlack...,
	)
	d.Notifiers.Telegram = append(
		d.Notifiers.Telegram, d.FlatTelegram...,
	)
	d.FlatDiscord = nil
	d.FlatSlack = nil
	d.FlatTelegram = nil

	cfg.Notifiers.Discord = append(
		cfg.Notifiers.Discord, cfg.FlatDiscord...,
	)
	cfg.Notifiers.Slack = append(
		cfg.Notifiers.Slack, cfg.FlatSlack...,
	)
	cfg.Notifiers.Telegram = append(
		cfg.Notifiers.Telegram, cfg.FlatTelegram...,
	)
	cfg.FlatDiscord = nil
	cfg.FlatSlack = nil
	cfg.FlatTelegram = nil
}

// applyGlobalNotifiers appends the top-level notifier lists to every
// configured printer. Discovered printers get them via
// MergeDiscovered.
func applyGlobalNotifiers(cfg *Config) {
	for i := range cfg.Printers {
		cfg.Printers[i].Notifiers = combineNotifiers(
			cfg.Printers[i].Notifiers, cfg.Notifiers,
		)
	}
}

// combineNotifiers returns a and b concatenated into freshly
// allocated lists.
func combineNotifiers(a, b Notifiers) Notifiers {
	return Notifiers{
		Discord: append(
			append([]Discord{}, a.Discord...), b.Discord...,
		),
		Slack: append(
			append([]Slack{}, a.Slack...), b.Slack...,
		),
		Telegram: append(
			append([]Telegram{}, a.Telegram...), b.Telegram...,
		),
	}
}

func applyDefaults(cfg *Config) {
	for i := range cfg.Printers {
		if cfg.Printers[i].Port == 0 {
			cfg.Printers[i].Port = 8883
		}
		if cfg.Printers[i].ReconnectDelaySeconds == 0 {
			cfg.Printers[i].ReconnectDelaySeconds = 30
		}
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = LogLevel(strings.ToLower(v))
	}

	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Log.Format = LogFormat(strings.ToLower(v))
	}
}

func validate(cfg Config) error {
	if err := validateLog(cfg.Log); err != nil {
		return err
	}

	if len(cfg.Printers) == 0 && !cfg.Discovery.Enabled {
		return fmt.Errorf("at least one [[printer]] is required")
	}

	for i, p := range cfg.Printers {
		if err := validatePrinter(i, p); err != nil {
			return err
		}
	}

	if err := validateNotifiers(
		"discovery.notifiers", cfg.Discovery.Notifiers,
	); err != nil {
		return err
	}

	return validateNotifiers("notifiers", cfg.Notifiers)
}

func validateLog(l Log) error {
	switch l.Level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, "":
	default:
		return fmt.Errorf("invalid log level %q", l.Level)
	}

	switch l.Format {
	case LogFormatJSON, LogFormatText, "":
	default:
		return fmt.Errorf("invalid log format %q", l.Format)
	}

	return nil
}

func validatePrinter(idx int, p Printer) error {
	if p.Name == "" {
		return fmt.Errorf("printer[%d]: name is required", idx)
	}

	if p.Host == "" {
		return fmt.Errorf("printer[%d]: host is required", idx)
	}

	if p.SerialNumber == "" {
		return fmt.Errorf(
			"printer[%d]: serial_number is required", idx,
		)
	}

	if p.AccessCode == "" {
		return fmt.Errorf(
			"printer[%d]: access_code is required", idx,
		)
	}

	return validateNotifiers(
		fmt.Sprintf("printer[%d].notifiers", idx), p.Notifiers,
	)
}

func validateNotifiers(prefix string, n Notifiers) error {
	for i, d := range n.Discord {
		if d.WebhookURL == "" {
			return fmt.Errorf(
				"%s.discord[%d]: webhook_url is required",
				prefix, i,
			)
		}
	}

	for i, s := range n.Slack {
		if s.WebhookURL == "" {
			return fmt.Errorf(
				"%s.slack[%d]: webhook_url is required",
				prefix, i,
			)
		}
	}

	for i, t := range n.Telegram {
		if t.BotToken == "" {
			return fmt.Errorf(
				"%s.telegram[%d]: bot_token is required",
				prefix, i,
			)
		}

		if t.ChatID == "" {
			return fmt.Errorf(
				"%s.telegram[%d]: chat_id is required",
				prefix, i,
			)
		}
	}

	return nil
}
