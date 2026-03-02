package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedTime = time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)

func newTestTracker() *tracker {
	t := NewTracker("test-printer").(*tracker)
	t.now = func() time.Time { return fixedTime }
	return t
}

func baseReport(state GcodeState) Report {
	return Report{
		GcodeState: state,
		Filename:   "benchy.gcode",
		Progress:   50,
		ETA:        "01:30",
		NozzleType: "0.4mm",
		Source:      "lan",
		Thermals: Thermals{
			NozzleActual: 210,
			NozzleTarget: 210,
			BedActual:    60,
			BedTarget:    60,
		},
	}
}

func TestTrackerStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		prev      GcodeState
		current   GcodeState
		wantType  Type
		wantEmpty bool
	}{
		{
			name:     "IDLE to RUNNING emits PrintStarted",
			prev:     StateIdle,
			current:  StateRunning,
			wantType: PrintStarted,
		},
		{
			name:     "PREPARE to RUNNING emits PrintStarted",
			prev:     StatePrepare,
			current:  StateRunning,
			wantType: PrintStarted,
		},
		{
			name:     "RUNNING to FINISH emits PrintFinished",
			prev:     StateRunning,
			current:  StateFinish,
			wantType: PrintFinished,
		},
		{
			name:     "RUNNING to FAILED emits PrintFailed",
			prev:     StateRunning,
			current:  StateFailed,
			wantType: PrintFailed,
		},
		{
			name:     "RUNNING to PAUSE emits PrintPaused",
			prev:     StateRunning,
			current:  StatePause,
			wantType: PrintPaused,
		},
		{
			name:     "PAUSE to RUNNING emits PrintResumed not PrintStarted",
			prev:     StatePause,
			current:  StateRunning,
			wantType: PrintResumed,
		},
		{
			name:      "RUNNING to RUNNING emits nothing",
			prev:      StateRunning,
			current:   StateRunning,
			wantEmpty: true,
		},
		{
			name:      "FINISH to FINISH emits nothing",
			prev:      StateFinish,
			current:   StateFinish,
			wantEmpty: true,
		},
		{
			name:      "IDLE to IDLE emits nothing",
			prev:      StateIdle,
			current:   StateIdle,
			wantEmpty: true,
		},
		{
			name:      "RUNNING to IDLE emits nothing",
			prev:      StateRunning,
			current:   StateIdle,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = tt.prev

			events := tr.Process(baseReport(tt.current))

			if tt.wantEmpty {
				assert.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			assert.Equal(t, tt.wantType, events[0].Type)
			assert.Equal(t, "test-printer", events[0].PrinterName)
			assert.Equal(t, fixedTime, events[0].Timestamp)
		})
	}
}

func TestTrackerCriticalError(t *testing.T) {
	tests := []struct {
		name      string
		prevError int
		curError  int
		wantEmpty bool
	}{
		{
			name:     "new error fires CriticalError",
			curError: 0x0500C001,
		},
		{
			name:      "same error repeated does not fire",
			prevError: 0x0500C001,
			curError:  0x0500C001,
			wantEmpty: true,
		},
		{
			name:      "no error does not fire",
			curError:  0,
			wantEmpty: true,
		},
		{
			name:     "different error fires again",
			prevError: 0x0500C001,
			curError:  0x0500C002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = StateRunning
			tr.prevError = tt.prevError

			r := baseReport(StateRunning)
			r.PrintError = tt.curError
			events := tr.Process(r)

			if tt.wantEmpty {
				assert.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			assert.Equal(t, CriticalError, events[0].Type)
			assert.Equal(t, tt.curError, events[0].ErrorCode)
		})
	}
}

func TestTrackerFilamentRunout(t *testing.T) {
	tests := []struct {
		name      string
		state     GcodeState
		prevError int
		curError  int
		wantType  Type
		wantCount int
	}{
		{
			name:      "filament runout while RUNNING",
			state:     StateRunning,
			curError:  filamentRunoutCode,
			wantCount: 2, // CriticalError + FilamentRunout
		},
		{
			name:      "filament runout while PAUSE",
			state:     StatePause,
			prevError: 0,
			curError:  filamentRunoutCode,
			wantCount: 2, // CriticalError + FilamentRunout (no state transition since prevState == PAUSE)
		},
		{
			name:      "filament runout repeated does not fire",
			state:     StateRunning,
			prevError: filamentRunoutCode,
			curError:  filamentRunoutCode,
			wantCount: 0,
		},
		{
			name:      "filament runout while IDLE does not fire runout",
			state:     StateIdle,
			curError:  filamentRunoutCode,
			wantCount: 1, // only CriticalError, not FilamentRunout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = tt.state // same state to avoid state transition events
			tr.prevError = tt.prevError

			r := baseReport(tt.state)
			r.PrintError = tt.curError
			events := tr.Process(r)

			assert.Len(t, events, tt.wantCount)
		})
	}
}

func TestTrackerFilamentRunoutEmitsFilamentRunoutType(t *testing.T) {
	tr := newTestTracker()
	tr.prevState = StateRunning

	r := baseReport(StateRunning)
	r.PrintError = filamentRunoutCode
	events := tr.Process(r)

	require.Len(t, events, 2)
	assert.Equal(t, CriticalError, events[0].Type)
	assert.Equal(t, FilamentRunout, events[1].Type)
	assert.Equal(t, filamentRunoutCode, events[1].ErrorCode)
}

func TestTrackerAMSChange(t *testing.T) {
	tests := []struct {
		name      string
		prevSlot  int
		curSlots  []AMSSlot
		wantEmpty bool
	}{
		{
			name:     "slot changes from 0 to 1",
			prevSlot: 0,
			curSlots: []AMSSlot{
				{SlotID: 0, InUse: false},
				{SlotID: 1, InUse: true},
			},
		},
		{
			name:      "slot stays the same",
			prevSlot:  1,
			curSlots:  []AMSSlot{{SlotID: 1, InUse: true}},
			wantEmpty: true,
		},
		{
			name:      "no active slot and no previous",
			prevSlot:  -1,
			curSlots:  []AMSSlot{{SlotID: 0, InUse: false}},
			wantEmpty: true,
		},
		{
			name:      "first report with active slot does not fire",
			prevSlot:  -1,
			curSlots:  []AMSSlot{{SlotID: 0, InUse: true}},
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = StateRunning
			tr.prevAMSSlot = tt.prevSlot

			r := baseReport(StateRunning)
			r.AMSSlots = tt.curSlots
			events := tr.Process(r)

			if tt.wantEmpty {
				assert.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			assert.Equal(t, AMSChange, events[0].Type)
		})
	}
}

func TestTrackerNozzleTempAnomaly(t *testing.T) {
	tests := []struct {
		name      string
		state     GcodeState
		actual    float64
		target    float64
		wantEmpty bool
	}{
		{
			name:   "deviation over threshold while RUNNING",
			state:  StateRunning,
			actual: 240,
			target: 210,
		},
		{
			name:      "deviation exactly at threshold",
			state:     StateRunning,
			actual:    230,
			target:    210,
			wantEmpty: true,
		},
		{
			name:      "small deviation while RUNNING",
			state:     StateRunning,
			actual:    215,
			target:    210,
			wantEmpty: true,
		},
		{
			name:      "deviation over threshold while IDLE",
			state:     StateIdle,
			actual:    240,
			target:    210,
			wantEmpty: true,
		},
		{
			name:      "target is zero",
			state:     StateRunning,
			actual:    25,
			target:    0,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = tt.state

			r := baseReport(tt.state)
			r.Thermals.NozzleActual = tt.actual
			r.Thermals.NozzleTarget = tt.target
			events := tr.Process(r)

			if tt.wantEmpty {
				assert.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			assert.Equal(t, NozzleTempAnomaly, events[0].Type)
		})
	}
}

func TestTrackerHMSAlerts(t *testing.T) {
	tests := []struct {
		name      string
		prevHMS   map[string]struct{}
		curHMS    []string
		wantEmpty bool
		wantHMS   []string
	}{
		{
			name:    "new HMS entries fire alert",
			prevHMS: map[string]struct{}{},
			curHMS:  []string{"0x0700_0001", "0x0700_0002"},
			wantHMS: []string{"0x0700_0001", "0x0700_0002"},
		},
		{
			name:    "only new entries reported",
			prevHMS: map[string]struct{}{"0x0700_0001": {}},
			curHMS:  []string{"0x0700_0001", "0x0700_0003"},
			wantHMS: []string{"0x0700_0003"},
		},
		{
			name:      "same entries repeated does not fire",
			prevHMS:   map[string]struct{}{"0x0700_0001": {}},
			curHMS:    []string{"0x0700_0001"},
			wantEmpty: true,
		},
		{
			name:      "empty HMS does not fire",
			prevHMS:   map[string]struct{}{},
			curHMS:    nil,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTracker()
			tr.prevState = StateRunning
			tr.prevHMS = tt.prevHMS

			r := baseReport(StateRunning)
			r.HMSErrors = tt.curHMS
			events := tr.Process(r)

			if tt.wantEmpty {
				assert.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			assert.Equal(t, HMSAlert, events[0].Type)
			assert.Equal(t, tt.wantHMS, events[0].HMSErrors)
		})
	}
}

func TestTrackerMultipleEventsInSingleReport(t *testing.T) {
	tr := newTestTracker()
	tr.prevState = StateIdle

	r := baseReport(StateRunning)
	r.PrintError = 0x0500C001
	r.HMSErrors = []string{"0x0700_0001"}
	r.Thermals.NozzleActual = 250
	r.Thermals.NozzleTarget = 210

	events := tr.Process(r)

	types := make([]Type, len(events))
	for i, e := range events {
		types[i] = e.Type
	}

	assert.Contains(t, types, PrintStarted)
	assert.Contains(t, types, CriticalError)
	assert.Contains(t, types, NozzleTempAnomaly)
	assert.Contains(t, types, HMSAlert)
}

func TestTrackerStatePersistsAcrossCalls(t *testing.T) {
	tr := newTestTracker()

	// First call: IDLE -> RUNNING
	events := tr.Process(baseReport(StateRunning))
	require.Len(t, events, 1)
	assert.Equal(t, PrintStarted, events[0].Type)

	// Second call: RUNNING -> RUNNING (no event)
	events = tr.Process(baseReport(StateRunning))
	assert.Empty(t, events)

	// Third call: RUNNING -> PAUSE
	events = tr.Process(baseReport(StatePause))
	require.Len(t, events, 1)
	assert.Equal(t, PrintPaused, events[0].Type)

	// Fourth call: PAUSE -> RUNNING (resume, not start)
	events = tr.Process(baseReport(StateRunning))
	require.Len(t, events, 1)
	assert.Equal(t, PrintResumed, events[0].Type)
}
