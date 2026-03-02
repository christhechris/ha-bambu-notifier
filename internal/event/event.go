package event

import "time"

// Type represents the kind of print lifecycle event.
type Type string

// Print lifecycle event types.
const (
	PrintStarted      Type = "print_started"
	PrintFinished     Type = "print_finished"
	PrintFailed       Type = "print_failed"
	PrintPaused       Type = "print_paused"
	PrintResumed      Type = "print_resumed"
	CriticalError     Type = "critical_error"
	FilamentRunout    Type = "filament_runout"
	AMSChange         Type = "ams_change"
	NozzleTempAnomaly Type = "nozzle_temp_anomaly"
	HMSAlert          Type = "hms_alert"
)

// AMSSlot holds info about one AMS filament slot.
type AMSSlot struct {
	SlotID   int    `json:"slot_id"`
	Material string `json:"material"`
	ColorHex string `json:"color_hex"`
	InUse    bool   `json:"in_use"`
}

// Thermals holds temperature readings.
type Thermals struct {
	NozzleActual float64 `json:"nozzle_actual"`
	NozzleTarget float64 `json:"nozzle_target"`
	BedActual    float64 `json:"bed_actual"`
	BedTarget    float64 `json:"bed_target"`
}

// Event is the domain event emitted when a print lifecycle transition
// is detected.
type Event struct {
	Type        Type      `json:"type"`
	PrinterName string    `json:"printer_name"`
	Filename    string    `json:"filename"`
	Progress    int       `json:"progress"`
	ETA         string    `json:"eta"`
	NozzleType  string    `json:"nozzle_type"`
	Source      string    `json:"source"`
	Thermals    Thermals  `json:"thermals"`
	AMSSlots    []AMSSlot `json:"ams_slots,omitempty"`
	ErrorCode   int       `json:"error_code,omitempty"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	HMSErrors   []string  `json:"hms_errors,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Snapshot    []byte    `json:"-"` // JPEG camera frame, omitted from JSON
}

// GcodeState mirrors Bambu's gcode_state field values.
type GcodeState string

// Bambu printer gcode states.
const (
	StateRunning GcodeState = "RUNNING"
	StateFinish  GcodeState = "FINISH"
	StateFailed  GcodeState = "FAILED"
	StatePause   GcodeState = "PAUSE"
	StateIdle    GcodeState = "IDLE"
	StatePrepare GcodeState = "PREPARE"
	StateUnknown GcodeState = ""
)
