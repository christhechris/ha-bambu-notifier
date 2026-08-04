package notifier

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/l33t0/bambu-notifier/internal/event"
)

func TestSummaryLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ev   event.Event
		want string
	}{
		{
			name: "all fields",
			ev: event.Event{
				Filename: "benchy.3mf",
				Progress: 42,
				ETA:      "1h35m",
			},
			want: "benchy.3mf · 42% · ETA 1h35m",
		},
		{
			name: "no ETA",
			ev: event.Event{
				Filename: "vase.gcode",
				Progress: 100,
			},
			want: "vase.gcode · 100%",
		},
		{
			name: "error appended",
			ev: event.Event{
				Filename: "part.gcode",
				Progress: 15,
				ErrorMsg: "nozzle clog detected",
			},
			want: "part.gcode · 15% · nozzle clog detected",
		},
		{
			name: "no filename",
			ev:   event.Event{Progress: 0},
			want: "0%",
		},
		{
			name: "pause reason appended",
			ev: event.Event{
				Filename: "benchy.3mf",
				Progress: 42,
				ETA:      "1h35m",
				Reason:   "filament runout",
			},
			want: "benchy.3mf · 42% · ETA 1h35m · filament runout",
		},
		{
			name: "reason not duplicated when equal to error",
			ev: event.Event{
				Filename: "part.gcode",
				Progress: 15,
				Reason:   "nozzle clog detected",
				ErrorMsg: "nozzle clog detected",
			},
			want: "part.gcode · 15% · nozzle clog detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, summaryLine(tt.ev))
		})
	}
}
