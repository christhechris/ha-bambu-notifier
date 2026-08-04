package printer

import "fmt"

// pauseStageReasons maps Bambu stg_cur stage ids to human-readable
// pause reasons. Only the paused_* stages (plus the M400 G-code
// pause) are listed; ids follow ha-bambulab's CURRENT_STAGE_IDS.
var pauseStageReasons = map[int]string{
	5:  "paused by G-code (M400)",
	6:  "filament runout",
	16: "paused by user",
	17: "front cover fell off",
	20: "nozzle temperature malfunction",
	21: "heat bed temperature malfunction",
	23: "skipped step detected",
	26: "AMS lost",
	27: "low heat-break fan speed",
	28: "chamber temperature control error",
	30: "paused by user G-code",
	32: "filament detected covering nozzle",
	33: "cutter error",
	34: "first layer defect detected",
	35: "nozzle clog detected",
}

// pauseReason derives why the printer paused: the reported stage if
// it is a known pause stage, otherwise the first active HMS alert,
// otherwise the raw error code. Empty when nothing points at a cause
// (e.g. a plain user pause on firmware that doesn't report stages).
func pauseReason(r *parsedReport) string {
	if reason, ok := pauseStageReasons[r.StageID]; ok {
		return reason
	}
	if len(r.HMSErrors) > 0 {
		return r.HMSErrors[0]
	}
	if r.ErrorCode != 0 {
		return fmt.Sprintf("error code 0x%08X", r.ErrorCode)
	}
	return ""
}
