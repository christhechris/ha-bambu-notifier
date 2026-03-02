package printer

import (
	"testing"

	"github.com/l33t0/bambu-notifier/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReport(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    *parsedReport
		wantErr bool
	}{
		{
			name: "full running report",
			payload: `{
				"print": {
					"gcode_state": "RUNNING",
					"subtask_name": "benchy.gcode",
					"gcode_file": "benchy_full.gcode",
					"mc_percent": 42,
					"mc_remaining_time": 95,
					"nozzle_temper": 215.5,
					"nozzle_target_temper": 220.0,
					"bed_temper": 59.8,
					"bed_target_temper": 60.0,
					"nozzle_type": "hardened_steel",
					"print_error": 0
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StateRunning,
				Filename:   "benchy.gcode",
				Progress:   42,
				ETA:        "1h35m",
				NozzleType: "hardened_steel",
				Thermals: event.Thermals{
					NozzleActual: 215.5,
					NozzleTarget: 220.0,
					BedActual:    59.8,
					BedTarget:    60.0,
				},
			},
		},
		{
			name: "finished report",
			payload: `{
				"print": {
					"gcode_state": "FINISH",
					"subtask_name": "vase.gcode",
					"mc_percent": 100,
					"mc_remaining_time": 0,
					"nozzle_temper": 25.0,
					"nozzle_target_temper": 0,
					"bed_temper": 30.0,
					"bed_target_temper": 0
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StateFinish,
				Filename:   "vase.gcode",
				Progress:   100,
				ETA:        "",
				Thermals: event.Thermals{
					NozzleActual: 25.0,
					BedActual:    30.0,
				},
			},
		},
		{
			name: "failed report with error",
			payload: `{
				"print": {
					"gcode_state": "FAILED",
					"subtask_name": "part.gcode",
					"mc_percent": 15,
					"mc_remaining_time": 0,
					"nozzle_temper": 200.0,
					"nozzle_target_temper": 220.0,
					"bed_temper": 60.0,
					"bed_target_temper": 60.0,
					"print_error": 318767105,
					"fail_reason": "first layer inspection failed"
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StateFailed,
				Filename:   "part.gcode",
				Progress:   15,
				ErrorCode:  318767105,
				ErrorMsg:   "first layer inspection failed",
				Thermals: event.Thermals{
					NozzleActual: 200.0,
					NozzleTarget: 220.0,
					BedActual:    60.0,
					BedTarget:    60.0,
				},
			},
		},
		{
			name: "report with HMS errors",
			payload: `{
				"print": {
					"gcode_state": "RUNNING",
					"subtask_name": "test.gcode",
					"mc_percent": 50,
					"mc_remaining_time": 30,
					"nozzle_temper": 210.0,
					"nozzle_target_temper": 210.0,
					"bed_temper": 60.0,
					"bed_target_temper": 60.0,
					"hms": [
						{"attr": 1, "code": 12345, "module": "nozzle", "severity": "warning", "msg": "Nozzle clog detected"},
						{"attr": 2, "code": 67890, "module": "ams", "severity": "critical", "msg": ""}
					]
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StateRunning,
				Filename:   "test.gcode",
				Progress:   50,
				ETA:        "30m",
				Thermals: event.Thermals{
					NozzleActual: 210.0,
					NozzleTarget: 210.0,
					BedActual:    60.0,
					BedTarget:    60.0,
				},
				HMSErrors: []string{
					"Nozzle clog detected",
					"HMS code=67890 module=ams",
				},
			},
		},
		{
			name: "report with AMS data",
			payload: `{
				"print": {
					"gcode_state": "RUNNING",
					"subtask_name": "multicolor.gcode",
					"mc_percent": 20,
					"mc_remaining_time": 120,
					"nozzle_temper": 220.0,
					"nozzle_target_temper": 220.0,
					"bed_temper": 60.0,
					"bed_target_temper": 60.0,
					"ams": {
						"ams": [
							{
								"id": "0",
								"tray": [
									{"id": "0", "tray_type": "PLA", "tray_color": "FF0000", "tray_sub_brands": "Bambu PLA Basic", "remain": 80},
									{"id": "1", "tray_type": "PLA", "tray_color": "00FF00", "tray_sub_brands": "Bambu PLA Basic", "remain": 50}
								]
							}
						],
						"tray_now": "0",
						"tray_pre": "255",
						"tray_tar": "0"
					}
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StateRunning,
				Filename:   "multicolor.gcode",
				Progress:   20,
				ETA:        "2h00m",
				Thermals: event.Thermals{
					NozzleActual: 220.0,
					NozzleTarget: 220.0,
					BedActual:    60.0,
					BedTarget:    60.0,
				},
				AMSTrayNow: "0",
				AMSSlots: []event.AMSSlot{
					{SlotID: 0, Material: "Bambu PLA Basic", ColorHex: "FF0000", InUse: true},
					{SlotID: 1, Material: "Bambu PLA Basic", ColorHex: "00FF00"},
				},
			},
		},
		{
			name: "fallback to gcode_file when subtask_name empty",
			payload: `{
				"print": {
					"gcode_state": "PREPARE",
					"gcode_file": "/data/Metadata/fallback.gcode",
					"subtask_name": "",
					"mc_percent": 0,
					"mc_remaining_time": 0,
					"nozzle_temper": 25.0,
					"nozzle_target_temper": 0,
					"bed_temper": 25.0,
					"bed_target_temper": 0
				}
			}`,
			want: &parsedReport{
				GcodeState: event.StatePrepare,
				Filename:   "/data/Metadata/fallback.gcode",
				Thermals: event.Thermals{
					NozzleActual: 25.0,
					BedActual:    25.0,
				},
			},
		},
		{
			name:    "invalid JSON",
			payload: `{invalid`,
			wantErr: true,
		},
		{
			name:    "missing print field",
			payload: `{"info": {}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReport([]byte(tt.payload))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		want    string
	}{
		{name: "zero", minutes: 0, want: ""},
		{name: "negative", minutes: -5, want: ""},
		{name: "minutes only", minutes: 45, want: "45m"},
		{name: "one hour", minutes: 60, want: "1h00m"},
		{name: "hours and minutes", minutes: 125, want: "2h05m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatETA(tt.minutes))
		})
	}
}

func TestParseIntSafe(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "zero", input: "0", want: 0},
		{name: "single digit", input: "5", want: 5},
		{name: "multi digit", input: "123", want: 123},
		{name: "non-numeric", input: "abc", wantErr: true},
		{name: "mixed", input: "12a3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIntSafe(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
