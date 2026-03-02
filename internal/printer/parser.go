package printer

import (
	"encoding/json"
	"fmt"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// bambuReport is the top-level JSON structure from a Bambu printer MQTT report.
type bambuReport struct {
	Print *printReport `json:"print"`
}

// printReport contains the print status fields from a Bambu report.
type printReport struct {
	GcodeState    string     `json:"gcode_state"`
	GcodeFile     string     `json:"gcode_file"`
	Subtask       string     `json:"subtask_name"`
	MCPercent     int        `json:"mc_percent"`
	MCRemainingT  int        `json:"mc_remaining_time"`
	NozzleTemper  float64    `json:"nozzle_temper"`
	NozzleTarget  float64    `json:"nozzle_target_temper"`
	BedTemper     float64    `json:"bed_temper"`
	BedTarget     float64    `json:"bed_target_temper"`
	NozzleType    string     `json:"nozzle_type"`
	PrintError    int        `json:"print_error"`
	PrintErrorMsg string     `json:"fail_reason"`
	HMSErrors     []hmsEntry `json:"hms"`
	AMS           *amsStatus `json:"ams"`
	AMSStatus     int        `json:"ams_status"`
	Command       string     `json:"command"`
}

// hmsEntry represents a single HMS (Health Management System) alert.
type hmsEntry struct {
	Attr     int    `json:"attr"`
	Code     int    `json:"code"`
	Module   string `json:"module"`
	Severity string `json:"severity"`
	Msg      string `json:"msg"`
}

// amsStatus represents the AMS (Automatic Material System) state.
type amsStatus struct {
	AMS      []amsUnit `json:"ams"`
	TrayNow  string    `json:"tray_now"`
	TrayPre  string    `json:"tray_pre"`
	TrayTar  string    `json:"tray_tar"`
	InsertID string    `json:"insert_flag"`
	Version  int       `json:"version"`
}

// amsUnit represents one AMS unit with 4 trays.
type amsUnit struct {
	ID   string    `json:"id"`
	Tray []amsTray `json:"tray"`
}

// amsTray represents a single filament tray in an AMS unit.
type amsTray struct {
	ID       string `json:"id"`
	TrayType string `json:"tray_type"`
	Color    string `json:"tray_color"`
	Material string `json:"tray_sub_brands"`
	Remain   int    `json:"remain"`
}

// parsedReport is the structured output from parsing a Bambu MQTT report.
type parsedReport struct {
	GcodeState event.GcodeState
	Filename   string
	Progress   int
	ETA        string
	NozzleType string
	Source     string
	Thermals   event.Thermals
	ErrorCode  int
	ErrorMsg   string
	HMSErrors  []string
	AMSSlots   []event.AMSSlot
	AMSTrayNow string
}

// parseReport parses raw MQTT payload JSON into a parsedReport.
func parseReport(data []byte) (*parsedReport, error) {
	var report bambuReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal bambu report: %w", err)
	}

	if report.Print == nil {
		return nil, fmt.Errorf("report has no print field")
	}

	p := report.Print
	r := &parsedReport{
		GcodeState: event.GcodeState(p.GcodeState),
		Filename:   filenameFromReport(p),
		Progress:   p.MCPercent,
		ETA:        formatETA(p.MCRemainingT),
		NozzleType: p.NozzleType,
		Source:     sourceFromCommand(p.Command),
		Thermals: event.Thermals{
			NozzleActual: p.NozzleTemper,
			NozzleTarget: p.NozzleTarget,
			BedActual:    p.BedTemper,
			BedTarget:    p.BedTarget,
		},
		ErrorCode: p.PrintError,
		ErrorMsg:  p.PrintErrorMsg,
	}

	// HMS errors
	for _, h := range p.HMSErrors {
		msg := h.Msg
		if msg == "" {
			msg = fmt.Sprintf("HMS code=%d module=%s", h.Code, h.Module)
		}
		r.HMSErrors = append(r.HMSErrors, msg)
	}

	// AMS slots
	if p.AMS != nil {
		r.AMSTrayNow = p.AMS.TrayNow
		for _, unit := range p.AMS.AMS {
			for _, tray := range unit.Tray {
				slot := event.AMSSlot{
					Material: tray.Material,
					ColorHex: tray.Color,
					InUse:    tray.ID == p.AMS.TrayNow,
				}
				if id, err := parseIntSafe(tray.ID); err == nil {
					slot.SlotID = id
				}
				r.AMSSlots = append(r.AMSSlots, slot)
			}
		}
	}

	return r, nil
}

// filenameFromReport extracts the best filename from the report.
func filenameFromReport(p *printReport) string {
	if p.Subtask != "" {
		return p.Subtask
	}
	return p.GcodeFile
}

// formatETA converts remaining minutes into a human-readable string.
func formatETA(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	h := minutes / 60
	m := minutes % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// sourceFromCommand derives the print source from the command field.
func sourceFromCommand(cmd string) string {
	switch cmd {
	case "project_file":
		return "local"
	case "gcode_file":
		return "local"
	case "":
		return ""
	default:
		return cmd
	}
}

// parseIntSafe parses a string to int, returning an error if invalid.
func parseIntSafe(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
