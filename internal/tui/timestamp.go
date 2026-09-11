package tui

import "strings"

// TimestampMode defines whether to render arrival clock time, relative delta (Δt), or both.
type TimestampMode int

const (
	TSModeClock TimestampMode = iota // clock arrival time (e.g. 15:04:05.000)
	TSModeDelta                      // relative elapsed time since previous log (e.g. +14.2ms)
	TSModeBoth                       // both clock and delta columns
	TSModeOff                        // hidden
)

func (m TimestampMode) String() string {
	switch m {
	case TSModeClock:
		return "clock"
	case TSModeDelta:
		return "delta"
	case TSModeBoth:
		return "both"
	case TSModeOff:
		return "off"
	default:
		return "clock"
	}
}

func (m TimestampMode) Next() TimestampMode {
	switch m {
	case TSModeClock:
		return TSModeDelta
	case TSModeDelta:
		return TSModeBoth
	case TSModeBoth:
		return TSModeOff
	case TSModeOff:
		return TSModeClock
	default:
		return TSModeClock
	}
}

func ParseTimestampMode(s string) TimestampMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "delta", "dt":
		return TSModeDelta
	case "both":
		return TSModeBoth
	case "off", "none", "false":
		return TSModeOff
	default:
		return TSModeClock
	}
}
