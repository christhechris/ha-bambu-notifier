// Package discovery imports printer configurations from the
// ha-bambulab Home Assistant integration's config entry storage.
package discovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/l33t0/bambu-notifier/internal/config"
)

const bambuLabDomain = "bambu_lab"

// DefaultPaths are the locations of Home Assistant's config entry
// storage: as mounted into an add-on container (homeassistant_config
// map) and as seen from Home Assistant Core itself.
var DefaultPaths = []string{
	"/homeassistant/.storage/core.config_entries",
	"/config/.storage/core.config_entries",
}

// storageFile mirrors the shape of core.config_entries. Only the
// fields written by ha-bambulab's config flow are decoded.
type storageFile struct {
	Data struct {
		Entries []storageEntry `json:"entries"`
	} `json:"data"`
}

type storageEntry struct {
	Domain string `json:"domain"`
	Title  string `json:"title"`
	Data   struct {
		Serial     string `json:"serial"`
		DeviceType string `json:"device_type"`
	} `json:"data"`
	Options struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		AccessCode string `json:"access_code"`
	} `json:"options"`
}

// Printers reads Home Assistant's core.config_entries storage at path
// (or the first DefaultPaths candidate that exists when path is empty)
// and returns the printers configured in the ha-bambulab integration.
// Entries without a host or access code (cloud-only setups) are
// skipped with a warning: this daemon only speaks the printers' local
// MQTT.
func Printers(
	path string,
	logger *slog.Logger,
) ([]config.DiscoveredPrinter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("reading HA storage: %w", err)
	}

	var storage storageFile
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("parsing HA storage: %w", err)
	}

	var printers []config.DiscoveredPrinter
	for _, e := range storage.Data.Entries {
		if e.Domain != bambuLabDomain {
			continue
		}

		serial := e.Data.Serial
		if serial == "" {
			serial = e.Title
		}

		if e.Options.Host == "" || e.Options.AccessCode == "" {
			logger.Warn(
				"skipping ha-bambulab printer without local "+
					"host/access code (cloud-only setup)",
				"serial", serial,
			)
			continue
		}

		name := e.Options.Name
		if name == "" {
			name = serial
		}

		printers = append(printers, config.DiscoveredPrinter{
			Name:       name,
			Host:       e.Options.Host,
			Serial:     serial,
			AccessCode: e.Options.AccessCode,
			Model:      e.Data.DeviceType,
		})
	}

	return printers, nil
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	for _, p := range DefaultPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf(
		"HA config entry storage not found (tried %v); is the "+
			"homeassistant_config mount available?", DefaultPaths,
	)
}
