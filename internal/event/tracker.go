package event

import (
	"math"
	"time"
)

// Filament runout error code reported by Bambu printers.
const filamentRunoutCode = 0x07008001

// nozzleTempThreshold is the maximum acceptable deviation between actual and
// target nozzle temperature while printing.
const nozzleTempThreshold = 20.0

// Tracker detects gcode_state transitions and emits domain events.
type Tracker interface {
	Process(report Report) []Event
}

// Report is the parsed status payload received from a printer over MQTT.
type Report struct {
	GcodeState GcodeState
	Filename   string
	Progress   int
	ETA        string
	NozzleType string
	Source     string
	Thermals   Thermals
	AMSSlots   []AMSSlot
	PrintError int
	HMSErrors  []string
}

var _ Tracker = (*tracker)(nil)

// tracker holds per-printer state needed to detect transitions.
type tracker struct {
	printerName string
	prevState   GcodeState
	prevError   int
	prevHMS     map[string]struct{}
	prevAMSSlot int // slot_id of the previously active AMS slot, -1 if none
	now         func() time.Time
}

// NewTracker returns a Tracker for the given printer name.
func NewTracker(printerName string) Tracker {
	return &tracker{
		printerName: printerName,
		prevState:   StateUnknown,
		prevError:   0,
		prevHMS:     make(map[string]struct{}),
		prevAMSSlot: -1,
		now:         time.Now,
	}
}

// Process evaluates a Report against the tracker's internal state and returns
// any events that should be fired. It updates internal state after evaluation.
func (t *tracker) Process(report Report) []Event {
	var events []Event

	events = t.detectStateTransition(report, events)
	events = t.detectCriticalError(report, events)
	events = t.detectFilamentRunout(report, events)
	events = t.detectAMSChange(report, events)
	events = t.detectNozzleTempAnomaly(report, events)
	events = t.detectHMSAlerts(report, events)

	t.prevState = report.GcodeState
	t.prevError = report.PrintError

	return events
}

func (t *tracker) detectStateTransition(r Report, events []Event) []Event {
	if r.GcodeState == t.prevState {
		return events
	}

	var typ Type
	switch r.GcodeState {
	case StateRunning:
		if t.prevState == StatePause {
			typ = PrintResumed
		} else {
			typ = PrintStarted
		}
	case StateFinish:
		typ = PrintFinished
	case StateFailed:
		typ = PrintFailed
	case StatePause:
		typ = PrintPaused
	default:
		return events
	}

	return append(events, t.makeEvent(typ, r))
}

func (t *tracker) detectCriticalError(r Report, events []Event) []Event {
	if r.PrintError == 0 || r.PrintError == t.prevError {
		return events
	}

	ev := t.makeEvent(CriticalError, r)
	ev.ErrorCode = r.PrintError

	return append(events, ev)
}

func (t *tracker) detectFilamentRunout(r Report, events []Event) []Event {
	if r.PrintError != filamentRunoutCode {
		return events
	}
	if t.prevError == filamentRunoutCode {
		return events
	}
	if r.GcodeState != StateRunning && r.GcodeState != StatePause {
		return events
	}

	ev := t.makeEvent(FilamentRunout, r)
	ev.ErrorCode = filamentRunoutCode

	return append(events, ev)
}

func (t *tracker) detectAMSChange(r Report, events []Event) []Event {
	current := activeAMSSlot(r.AMSSlots)
	defer func() { t.prevAMSSlot = current }()

	if current == -1 || t.prevAMSSlot == -1 {
		return events
	}
	if current == t.prevAMSSlot {
		return events
	}

	return append(events, t.makeEvent(AMSChange, r))
}

func (t *tracker) detectNozzleTempAnomaly(r Report, events []Event) []Event {
	if r.GcodeState != StateRunning {
		return events
	}
	if r.Thermals.NozzleTarget == 0 {
		return events
	}

	diff := math.Abs(r.Thermals.NozzleActual - r.Thermals.NozzleTarget)
	if diff <= nozzleTempThreshold {
		return events
	}

	return append(events, t.makeEvent(NozzleTempAnomaly, r))
}

func (t *tracker) detectHMSAlerts(r Report, events []Event) []Event {
	var newErrors []string
	for _, e := range r.HMSErrors {
		if _, seen := t.prevHMS[e]; !seen {
			newErrors = append(newErrors, e)
		}
	}

	if len(newErrors) == 0 {
		return events
	}

	// Rebuild the set from the current report so cleared errors can re-fire.
	t.prevHMS = make(map[string]struct{}, len(r.HMSErrors))
	for _, e := range r.HMSErrors {
		t.prevHMS[e] = struct{}{}
	}

	ev := t.makeEvent(HMSAlert, r)
	ev.HMSErrors = newErrors

	return append(events, ev)
}

func (t *tracker) makeEvent(typ Type, r Report) Event {
	return Event{
		Type:        typ,
		PrinterName: t.printerName,
		Filename:    r.Filename,
		Progress:    r.Progress,
		ETA:         r.ETA,
		NozzleType:  r.NozzleType,
		Source:      r.Source,
		Thermals:    r.Thermals,
		AMSSlots:    r.AMSSlots,
		Timestamp:   t.now(),
	}
}

func activeAMSSlot(slots []AMSSlot) int {
	for _, s := range slots {
		if s.InUse {
			return s.SlotID
		}
	}
	return -1
}
