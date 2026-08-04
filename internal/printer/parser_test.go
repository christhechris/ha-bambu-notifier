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

func TestParseReport_flexibleTypes(t *testing.T) {
	t.Run("H2D report with bool insert_flag and numeric ids", func(t *testing.T) {
		// The H2D sends insert_flag as bool and ids as numbers
		// where X1/P1 firmware sends strings; one odd field must
		// not drop the whole report.
		payload := `{
			"print": {
				"gcode_state": "RUNNING",
				"subtask_name": "bracket.gcode",
				"mc_percent": 61,
				"mc_remaining_time": 30,
				"nozzle_temper": 220.1,
				"nozzle_target_temper": 220.0,
				"bed_temper": 55.0,
				"bed_target_temper": 55.0,
				"ams": {
					"insert_flag": false,
					"power_on_flag": true,
					"tray_now": 0,
					"ams": [
						{
							"id": 0,
							"tray": [
								{"id": 0, "tray_type": "PLA", "tray_color": "FF0000FF", "tray_sub_brands": "PLA Basic", "remain": 80}
							]
						}
					]
				}
			}
		}`

		r, err := parseReport([]byte(payload))
		require.NoError(t, err)

		assert.Equal(t, event.StateRunning, r.GcodeState)
		assert.Equal(t, 61, r.Progress)
		assert.Equal(t, "0", r.AMSTrayNow)
		require.Len(t, r.AMSSlots, 1)
		assert.Equal(t, "PLA Basic", r.AMSSlots[0].Material)
		assert.True(t, r.AMSSlots[0].InUse)
	})

	t.Run("parses stg_cur as number or string", func(t *testing.T) {
		numeric := `{"print": {"gcode_state": "PAUSE", "subtask_name": "x.gcode", "stg_cur": 6}}`
		r, err := parseReport([]byte(numeric))
		require.NoError(t, err)
		assert.Equal(t, 6, r.StageID)

		stringy := `{"print": {"gcode_state": "PAUSE", "subtask_name": "x.gcode", "stg_cur": "16"}}`
		r, err = parseReport([]byte(stringy))
		require.NoError(t, err)
		assert.Equal(t, 16, r.StageID)
	})

	t.Run("captures advertised rtsp_url", func(t *testing.T) {
		payload := `{
			"print": {
				"gcode_state": "RUNNING",
				"subtask_name": "x.gcode",
				"ipcam": {
					"ipcam_dev": "1",
					"rtsp_url": "rtsps://192.168.1.50/streaming/live/1"
				}
			}
		}`

		r, err := parseReport([]byte(payload))
		require.NoError(t, err)

		assert.Equal(t,
			"rtsps://192.168.1.50/streaming/live/1", r.RTSPUrl,
		)
	})

	t.Run("numeric fields sent as strings", func(t *testing.T) {
		payload := `{
			"print": {
				"gcode_state": "RUNNING",
				"subtask_name": "x.gcode",
				"mc_percent": "42",
				"mc_remaining_time": "95",
				"nozzle_temper": "215.5",
				"print_error": "0"
			}
		}`

		r, err := parseReport([]byte(payload))
		require.NoError(t, err)

		assert.Equal(t, 42, r.Progress)
		assert.Equal(t, "1h35m", r.ETA)
		assert.InDelta(t, 215.5, r.Thermals.NozzleActual, 0.001)
		assert.Equal(t, 0, r.ErrorCode)
	})

	t.Run("garbage scalar types parse as zero values", func(t *testing.T) {
		payload := `{
			"print": {
				"gcode_state": "RUNNING",
				"subtask_name": "x.gcode",
				"mc_percent": true,
				"nozzle_temper": null,
				"print_error": {"weird": 1}
			}
		}`

		r, err := parseReport([]byte(payload))
		require.NoError(t, err)

		assert.Equal(t, 0, r.Progress)
		assert.Zero(t, r.Thermals.NozzleActual)
		assert.Equal(t, 0, r.ErrorCode)
	})
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
