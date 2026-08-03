package printer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/l33t0/bambu-notifier/internal/event"
)

// Bambu firmware is inconsistent about JSON scalar types across
// printer generations (e.g. the H2D sends fields as bool or number
// where the X1/P1 send strings). The flex* types below accept any
// scalar representation instead of failing the whole report.

// flexString accepts a JSON string, number, bool, or null.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	if string(b) == "null" {
		*f = ""
		return nil
	}
	*f = flexString(b)
	return nil
}

// flexInt accepts a JSON number (int or float) or numeric string;
// anything else parses as 0.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if v, err := strconv.ParseFloat(
			strings.TrimSpace(s), 64,
		); err == nil {
			*f = flexInt(v)
			return nil
		}
	}

	*f = 0
	return nil
}

// flexFloat accepts a JSON number or numeric string; anything else
// parses as 0.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexFloat(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if v, err := strconv.ParseFloat(
			strings.TrimSpace(s), 64,
		); err == nil {
			*f = flexFloat(v)
			return nil
		}
	}

	*f = 0
	return nil
}

// bambuReport is the top-level JSON structure from a Bambu printer MQTT report.
type bambuReport struct {
	Print *printReport `json:"print"`
}

// printReport contains the print status fields from a Bambu report.
// Only fields this daemon consumes are decoded; unknown fields are
// ignored by encoding/json.
type printReport struct {
	GcodeState    string     `json:"gcode_state"`
	GcodeFile     string     `json:"gcode_file"`
	Subtask       string     `json:"subtask_name"`
	MCPercent     flexInt    `json:"mc_percent"`
	MCRemainingT  flexInt    `json:"mc_remaining_time"`
	NozzleTemper  flexFloat  `json:"nozzle_temper"`
	NozzleTarget  flexFloat  `json:"nozzle_target_temper"`
	BedTemper     flexFloat  `json:"bed_temper"`
	BedTarget     flexFloat  `json:"bed_target_temper"`
	NozzleType    string     `json:"nozzle_type"`
	PrintError    flexInt    `json:"print_error"`
	PrintErrorMsg string     `json:"fail_reason"`
	HMSErrors     []hmsEntry `json:"hms"`
	AMS           *amsStatus `json:"ams"`
	IPCam         *ipcam     `json:"ipcam"`
	Command       string     `json:"command"`
}

// ipcam holds the printer's camera advertisement. rtsp_url is set on
// RTSP-capable models (X1/X2/H2/P2 series) when liveview is enabled.
type ipcam struct {
	RTSPUrl flexString `json:"rtsp_url"`
}

// hmsEntry represents a single HMS (Health Management System) alert.
type hmsEntry struct {
	Attr     flexInt    `json:"attr"`
	Code     flexInt    `json:"code"`
	Module   flexString `json:"module"`
	Severity flexString `json:"severity"`
	Msg      flexString `json:"msg"`
}

// amsStatus represents the AMS (Automatic Material System) state.
type amsStatus struct {
	AMS     []amsUnit  `json:"ams"`
	TrayNow flexString `json:"tray_now"`
}

// amsUnit represents one AMS unit with 4 trays.
type amsUnit struct {
	ID   flexString `json:"id"`
	Tray []amsTray  `json:"tray"`
}

// amsTray represents a single filament tray in an AMS unit.
type amsTray struct {
	ID       flexString `json:"id"`
	TrayType flexString `json:"tray_type"`
	Color    flexString `json:"tray_color"`
	Material flexString `json:"tray_sub_brands"`
	Remain   flexInt    `json:"remain"`
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
	RTSPUrl    string
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
		Progress:   int(p.MCPercent),
		ETA:        formatETA(int(p.MCRemainingT)),
		NozzleType: p.NozzleType,
		Source:     sourceFromCommand(p.Command),
		Thermals: event.Thermals{
			NozzleActual: float64(p.NozzleTemper),
			NozzleTarget: float64(p.NozzleTarget),
			BedActual:    float64(p.BedTemper),
			BedTarget:    float64(p.BedTarget),
		},
		ErrorCode: int(p.PrintError),
		ErrorMsg:  p.PrintErrorMsg,
	}

	// HMS errors
	for _, h := range p.HMSErrors {
		msg := string(h.Msg)
		if msg == "" {
			msg = fmt.Sprintf("HMS code=%d module=%s", h.Code, h.Module)
		}
		r.HMSErrors = append(r.HMSErrors, msg)
	}

	if p.IPCam != nil {
		r.RTSPUrl = string(p.IPCam.RTSPUrl)
	}

	// AMS slots
	if p.AMS != nil {
		r.AMSTrayNow = string(p.AMS.TrayNow)
		for _, unit := range p.AMS.AMS {
			for _, tray := range unit.Tray {
				slot := event.AMSSlot{
					Material: string(tray.Material),
					ColorHex: string(tray.Color),
					InUse:    tray.ID == p.AMS.TrayNow,
				}
				if id, err := parseIntSafe(string(tray.ID)); err == nil {
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
