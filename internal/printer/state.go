package printer

import (
	"math"
	"sync"
	"time"

	"github.com/l33t0/bambu-notifier/internal/event"
)

const (
	nozzleTempDeviationThreshold = 20.0
	filamentRunoutCode           = 0x07008001
)

// stateTracker tracks the gcode_state of a single printer and detects
// lifecycle transitions that should emit events.
type stateTracker struct {
	printerName string

	mu             sync.Mutex
	prevState      event.GcodeState
	prevErrorCode  int
	prevAMSTray    string
	prevHMS        map[string]bool
	anomalyEmitted bool
}

func newStateTracker(printerName string) *stateTracker {
	return &stateTracker{
		printerName: printerName,
		prevState:   event.StateUnknown,
		prevHMS:     make(map[string]bool),
	}
}

// update processes a parsed report and returns any events detected.
func (st *stateTracker) update(r *parsedReport) []event.Event {
	st.mu.Lock()
	defer st.mu.Unlock()

	var events []event.Event
	now := time.Now()

	base := event.Event{
		PrinterName: st.printerName,
		Filename:    r.Filename,
		Progress:    r.Progress,
		ETA:         r.ETA,
		NozzleType:  r.NozzleType,
		Source:      r.Source,
		Thermals:    r.Thermals,
		AMSSlots:    r.AMSSlots,
		Timestamp:   now,
	}

	// Detect gcode_state transitions.
	if r.GcodeState != st.prevState &&
		r.GcodeState != event.StateUnknown {
		if evt, ok := st.transitionEvent(
			st.prevState, r.GcodeState, base,
		); ok {
			if evt.Type == event.PrintPaused {
				evt.Reason = pauseReason(r)
			}
			events = append(events, evt)
		}
		st.prevState = r.GcodeState
		st.anomalyEmitted = false
	}

	// Detect filament runout (specific error code mid-print).
	if r.ErrorCode == filamentRunoutCode &&
		st.prevErrorCode != filamentRunoutCode &&
		(r.GcodeState == event.StateRunning ||
			r.GcodeState == event.StatePause) {
		e := base
		e.Type = event.FilamentRunout
		e.ErrorCode = filamentRunoutCode
		events = append(events, e)
	}

	// Detect critical error (deduplicated by error code).
	if r.ErrorCode != 0 && r.ErrorCode != st.prevErrorCode {
		e := base
		e.Type = event.CriticalError
		e.ErrorCode = r.ErrorCode
		e.ErrorMsg = r.ErrorMsg
		events = append(events, e)
	}
	st.prevErrorCode = r.ErrorCode

	// Detect new HMS alerts (rebuild set each cycle so cleared
	// alerts can re-fire if they return).
	newHMS := make(map[string]bool, len(r.HMSErrors))
	for _, msg := range r.HMSErrors {
		newHMS[msg] = true
		if !st.prevHMS[msg] {
			e := base
			e.Type = event.HMSAlert
			e.HMSErrors = []string{msg}
			events = append(events, e)
		}
	}
	st.prevHMS = newHMS

	// Detect AMS tray change.
	if r.AMSTrayNow != "" && r.AMSTrayNow != st.prevAMSTray &&
		st.prevAMSTray != "" {
		e := base
		e.Type = event.AMSChange
		events = append(events, e)
	}
	if r.AMSTrayNow != "" {
		st.prevAMSTray = r.AMSTrayNow
	}

	// Detect nozzle temperature anomaly while printing.
	if r.GcodeState == event.StateRunning &&
		r.Thermals.NozzleTarget > 0 {
		delta := math.Abs(
			r.Thermals.NozzleActual - r.Thermals.NozzleTarget,
		)
		if delta > nozzleTempDeviationThreshold &&
			!st.anomalyEmitted {
			st.anomalyEmitted = true
			e := base
			e.Type = event.NozzleTempAnomaly
			events = append(events, e)
		} else if delta <= nozzleTempDeviationThreshold {
			st.anomalyEmitted = false
		}
	}

	return events
}

func (st *stateTracker) transitionEvent(
	from, to event.GcodeState,
	base event.Event,
) (event.Event, bool) {
	e := base

	switch to {
	case event.StateRunning:
		if from == event.StatePause {
			e.Type = event.PrintResumed
		} else {
			e.Type = event.PrintStarted
		}
	case event.StateFinish:
		e.Type = event.PrintFinished
	case event.StateFailed:
		e.Type = event.PrintFailed
	case event.StatePause:
		e.Type = event.PrintPaused
	case event.StatePrepare:
		if from == event.StateIdle ||
			from == event.StateUnknown ||
			from == event.StateFinish {
			e.Type = event.PrintStarted
		} else {
			return event.Event{}, false
		}
	case event.StateIdle:
		return event.Event{}, false
	default:
		return event.Event{}, false
	}

	return e, true
}

func (st *stateTracker) reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prevState = event.StateUnknown
	st.prevErrorCode = 0
	st.prevAMSTray = ""
	st.prevHMS = make(map[string]bool)
	st.anomalyEmitted = false
}
