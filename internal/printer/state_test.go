package printer

import (
	"testing"

	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseReport(state event.GcodeState) *parsedReport {
	return &parsedReport{
		GcodeState: state,
		Filename:   "test.gcode",
		Progress:   50,
		ETA:        "30m",
		Thermals: event.Thermals{
			NozzleActual: 210.0,
			NozzleTarget: 210.0,
			BedActual:    60.0,
			BedTarget:    60.0,
		},
	}
}

func TestStateTracker_Transitions(t *testing.T) {
	tests := []struct {
		name      string
		states    []event.GcodeState
		wantTypes []event.Type
	}{
		{
			name:      "idle to running emits started",
			states:    []event.GcodeState{event.StateIdle, event.StateRunning},
			wantTypes: []event.Type{event.PrintStarted},
		},
		{
			name:      "unknown to running emits started",
			states:    []event.GcodeState{event.StateRunning},
			wantTypes: []event.Type{event.PrintStarted},
		},
		{
			name:      "running to finish emits finished",
			states:    []event.GcodeState{event.StateRunning, event.StateFinish},
			wantTypes: []event.Type{event.PrintStarted, event.PrintFinished},
		},
		{
			name:      "running to failed emits failed",
			states:    []event.GcodeState{event.StateRunning, event.StateFailed},
			wantTypes: []event.Type{event.PrintStarted, event.PrintFailed},
		},
		{
			name:      "running to pause emits paused",
			states:    []event.GcodeState{event.StateRunning, event.StatePause},
			wantTypes: []event.Type{event.PrintStarted, event.PrintPaused},
		},
		{
			name:      "pause to running emits resumed",
			states:    []event.GcodeState{event.StateRunning, event.StatePause, event.StateRunning},
			wantTypes: []event.Type{event.PrintStarted, event.PrintPaused, event.PrintResumed},
		},
		{
			name:      "prepare from idle emits started",
			states:    []event.GcodeState{event.StateIdle, event.StatePrepare},
			wantTypes: []event.Type{event.PrintStarted},
		},
		{
			name:      "finish to prepare emits started (new print)",
			states:    []event.GcodeState{event.StateRunning, event.StateFinish, event.StatePrepare},
			wantTypes: []event.Type{event.PrintStarted, event.PrintFinished, event.PrintStarted},
		},
		{
			name:      "same state repeated no duplicate",
			states:    []event.GcodeState{event.StateRunning, event.StateRunning, event.StateRunning},
			wantTypes: []event.Type{event.PrintStarted},
		},
		{
			name:      "idle to idle no event",
			states:    []event.GcodeState{event.StateIdle, event.StateIdle},
			wantTypes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newStateTracker("test-printer")
			var gotTypes []event.Type

			for _, state := range tt.states {
				r := baseReport(state)
				events := st.update(r)
				for _, e := range events {
					gotTypes = append(gotTypes, e.Type)
				}
			}

			assert.Equal(t, tt.wantTypes, gotTypes)
		})
	}
}

func TestStateTracker_NoDuplicateEventsForSameState(t *testing.T) {
	st := newStateTracker("test-printer")

	r := baseReport(event.StateRunning)

	events1 := st.update(r)
	require.Len(t, events1, 1)
	assert.Equal(t, event.PrintStarted, events1[0].Type)

	// Same report again should not emit duplicate event
	events2 := st.update(r)
	assert.Empty(t, events2)

	// And again
	events3 := st.update(r)
	assert.Empty(t, events3)
}

func TestStateTracker_CriticalError(t *testing.T) {
	st := newStateTracker("test-printer")

	r := baseReport(event.StateRunning)
	r.ErrorCode = 318767105
	r.ErrorMsg = "first layer inspection failed"

	events := st.update(r)
	require.Len(t, events, 2) // PrintStarted + CriticalError

	types := make([]event.Type, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}
	assert.Contains(t, types, event.PrintStarted)
	assert.Contains(t, types, event.CriticalError)

	// Find the error event and check fields
	for _, e := range events {
		if e.Type == event.CriticalError {
			assert.Equal(t, 318767105, e.ErrorCode)
			assert.Equal(t, "first layer inspection failed", e.ErrorMsg)
		}
	}
}

func TestStateTracker_CriticalErrorDedup(t *testing.T) {
	st := newStateTracker("test-printer")

	r := baseReport(event.StateRunning)
	r.ErrorCode = 318767105
	st.update(r)

	// Same error code repeated → no duplicate
	r2 := baseReport(event.StateRunning)
	r2.ErrorCode = 318767105
	events := st.update(r2)
	assert.Empty(t, events)

	// Different error code → new event
	r3 := baseReport(event.StateRunning)
	r3.ErrorCode = 999
	events2 := st.update(r3)
	require.Len(t, events2, 1)
	assert.Equal(t, event.CriticalError, events2[0].Type)
}

func TestStateTracker_FilamentRunout(t *testing.T) {
	st := newStateTracker("test-printer")

	r := baseReport(event.StateRunning)
	r.ErrorCode = 0x07008001
	events := st.update(r)

	types := make([]event.Type, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}
	assert.Contains(t, types, event.FilamentRunout)
	assert.Contains(t, types, event.CriticalError)
}

func TestStateTracker_HMSAlertClearedAndRefires(t *testing.T) {
	st := newStateTracker("test-printer")

	// First report with HMS error
	r1 := baseReport(event.StateRunning)
	r1.HMSErrors = []string{"Nozzle clog"}
	st.update(r1)

	// HMS error cleared
	r2 := baseReport(event.StateRunning)
	r2.HMSErrors = nil
	st.update(r2)

	// Same HMS error returns → should re-fire
	r3 := baseReport(event.StateRunning)
	r3.HMSErrors = []string{"Nozzle clog"}
	events := st.update(r3)
	require.Len(t, events, 1)
	assert.Equal(t, event.HMSAlert, events[0].Type)
}

func TestStateTracker_HMSAlerts(t *testing.T) {
	st := newStateTracker("test-printer")

	r := baseReport(event.StateRunning)
	r.HMSErrors = []string{"Nozzle clog detected", "AMS error"}

	events := st.update(r)
	// PrintStarted + 2 HMS alerts
	require.Len(t, events, 3)

	// Same HMS errors again → no duplicates
	r2 := baseReport(event.StateRunning)
	r2.HMSErrors = []string{"Nozzle clog detected", "AMS error"}
	events2 := st.update(r2)
	assert.Empty(t, events2)

	// New HMS error → emitted
	r3 := baseReport(event.StateRunning)
	r3.HMSErrors = []string{"Nozzle clog detected", "AMS error", "New error"}
	events3 := st.update(r3)
	require.Len(t, events3, 1)
	assert.Equal(t, event.HMSAlert, events3[0].Type)
	assert.Equal(t, []string{"New error"}, events3[0].HMSErrors)
}

func TestStateTracker_AMSChange(t *testing.T) {
	st := newStateTracker("test-printer")

	// First report with tray 0
	r1 := baseReport(event.StateRunning)
	r1.AMSTrayNow = "0"
	events1 := st.update(r1)
	// Only PrintStarted, no AMS change on first report
	var amsEvents1 []event.Event
	for _, e := range events1 {
		if e.Type == event.AMSChange {
			amsEvents1 = append(amsEvents1, e)
		}
	}
	assert.Empty(t, amsEvents1)

	// Same tray → no event
	r2 := baseReport(event.StateRunning)
	r2.AMSTrayNow = "0"
	events2 := st.update(r2)
	assert.Empty(t, events2)

	// Different tray → AMS change
	r3 := baseReport(event.StateRunning)
	r3.AMSTrayNow = "1"
	events3 := st.update(r3)
	require.Len(t, events3, 1)
	assert.Equal(t, event.AMSChange, events3[0].Type)
}

func TestStateTracker_NozzleTempAnomaly(t *testing.T) {
	st := newStateTracker("test-printer")

	// Normal temperature while running
	r1 := baseReport(event.StateRunning)
	r1.Thermals.NozzleActual = 210.0
	r1.Thermals.NozzleTarget = 210.0
	events1 := st.update(r1)
	var anomalyEvents1 []event.Event
	for _, e := range events1 {
		if e.Type == event.NozzleTempAnomaly {
			anomalyEvents1 = append(anomalyEvents1, e)
		}
	}
	assert.Empty(t, anomalyEvents1)

	// Large deviation → anomaly
	r2 := baseReport(event.StateRunning)
	r2.Thermals.NozzleActual = 185.0
	r2.Thermals.NozzleTarget = 210.0
	events2 := st.update(r2)
	require.Len(t, events2, 1)
	assert.Equal(t, event.NozzleTempAnomaly, events2[0].Type)

	// Same deviation again → no duplicate
	r3 := baseReport(event.StateRunning)
	r3.Thermals.NozzleActual = 183.0
	r3.Thermals.NozzleTarget = 210.0
	events3 := st.update(r3)
	assert.Empty(t, events3)

	// Temperature recovers → reset
	r4 := baseReport(event.StateRunning)
	r4.Thermals.NozzleActual = 209.0
	r4.Thermals.NozzleTarget = 210.0
	events4 := st.update(r4)
	assert.Empty(t, events4)

	// Deviation again → new anomaly event
	r5 := baseReport(event.StateRunning)
	r5.Thermals.NozzleActual = 185.0
	r5.Thermals.NozzleTarget = 210.0
	events5 := st.update(r5)
	require.Len(t, events5, 1)
	assert.Equal(t, event.NozzleTempAnomaly, events5[0].Type)
}

func TestStateTracker_NoAnomalyWhenNotRunning(t *testing.T) {
	st := newStateTracker("test-printer")

	// Large deviation but state is IDLE → no anomaly
	r := baseReport(event.StateIdle)
	r.Thermals.NozzleActual = 25.0
	r.Thermals.NozzleTarget = 210.0
	events := st.update(r)
	for _, e := range events {
		assert.NotEqual(t, event.NozzleTempAnomaly, e.Type)
	}
}

func TestStateTracker_Reset(t *testing.T) {
	st := newStateTracker("test-printer")

	// Build up some state
	st.update(baseReport(event.StateRunning))
	r := baseReport(event.StateRunning)
	r.HMSErrors = []string{"some error"}
	r.AMSTrayNow = "0"
	st.update(r)

	// Reset
	st.reset()

	// After reset, the same transition should fire again
	events := st.update(baseReport(event.StateRunning))
	require.Len(t, events, 1)
	assert.Equal(t, event.PrintStarted, events[0].Type)
}

func TestStateTracker_EventFieldsPopulated(t *testing.T) {
	st := newStateTracker("my-printer")

	r := baseReport(event.StateRunning)
	r.Filename = "cool_model.gcode"
	r.Progress = 75
	r.ETA = "1h00m"
	r.NozzleType = "hardened_steel"
	r.AMSSlots = []event.AMSSlot{
		{SlotID: 0, Material: "PLA", ColorHex: "FF0000"},
	}

	events := st.update(r)
	require.Len(t, events, 1)

	e := events[0]
	assert.Equal(t, event.PrintStarted, e.Type)
	assert.Equal(t, "my-printer", e.PrinterName)
	assert.Equal(t, "cool_model.gcode", e.Filename)
	assert.Equal(t, 75, e.Progress)
	assert.Equal(t, "1h00m", e.ETA)
	assert.Equal(t, "hardened_steel", e.NozzleType)
	assert.Equal(t, 210.0, e.Thermals.NozzleActual)
	assert.Len(t, e.AMSSlots, 1)
	assert.False(t, e.Timestamp.IsZero())
}
