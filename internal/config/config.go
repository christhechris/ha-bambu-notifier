package config

import (
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
	Level  LogLevel  `toml:"level"`
	Format LogFormat `toml:"format"`
}

// Discord holds Discord webhook notifier configuration.
type Discord struct {
	Name       string `toml:"name"`
	WebhookURL string `toml:"webhook_url"`
	Secret     string `toml:"secret"`
}

// Slack holds Slack webhook notifier configuration.
type Slack struct {
	Name       string `toml:"name"`
	WebhookURL string `toml:"webhook_url"`
	Secret     string `toml:"secret"`
}

// Telegram holds Telegram bot notifier configuration.
type Telegram struct {
	Name     string `toml:"name"`
	BotToken string `toml:"bot_token"`
	ChatID   string `toml:"chat_id"`
}

// Notifiers holds the notifier configurations for a single printer.
type Notifiers struct {
	Discord  []Discord  `toml:"discord"`
	Slack    []Slack    `toml:"slack"`
	Telegram []Telegram `toml:"telegram"`
}

// Printer holds the configuration for a single Bambu Lab printer.
type Printer struct {
	Name                  string    `toml:"name"`
	Host                  string    `toml:"host"`
	Port                  int       `toml:"port"`
	SerialNumber          string    `toml:"serial_number"`
	AccessCode            string    `toml:"access_code"`
	CameraPort            int       `toml:"camera_port"`
	TLSInsecureSkipVerify bool      `toml:"tls_insecure_skip_verify"`
	ReconnectDelaySeconds int       `toml:"reconnect_delay_seconds"`
	Notifiers             Notifiers `toml:"notifiers"`
}

// Config is the top-level application configuration.
type Config struct {
	Log      Log       `toml:"log"`
	Printers []Printer `toml:"printer"`
}

// Load reads a TOML configuration file from path, applies environment
// variable overrides, and validates required fields.
func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := validate(cfg); err != nil {
		return cfg, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
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

	if len(cfg.Printers) == 0 {
		return fmt.Errorf("at least one [[printer]] is required")
	}

	for i, p := range cfg.Printers {
		if err := validatePrinter(i, p); err != nil {
			return err
		}
	}

	return nil
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

	return validateNotifiers(idx, p.Notifiers)
}

func validateNotifiers(idx int, n Notifiers) error {
	for i, d := range n.Discord {
		if d.WebhookURL == "" {
			return fmt.Errorf(
				"printer[%d].notifiers.discord[%d]: "+
					"webhook_url is required",
				idx, i,
			)
		}
	}

	for i, s := range n.Slack {
		if s.WebhookURL == "" {
			return fmt.Errorf(
				"printer[%d].notifiers.slack[%d]: "+
					"webhook_url is required",
				idx, i,
			)
		}
	}

	for i, t := range n.Telegram {
		if t.BotToken == "" {
			return fmt.Errorf(
				"printer[%d].notifiers.telegram[%d]: "+
					"bot_token is required",
				idx, i,
			)
		}

		if t.ChatID == "" {
			return fmt.Errorf(
				"printer[%d].notifiers.telegram[%d]: "+
					"chat_id is required",
				idx, i,
			)
		}
	}

	return nil
}
