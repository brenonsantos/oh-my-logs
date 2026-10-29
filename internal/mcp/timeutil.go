package mcp

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// TimeKind identifies how a parsed time query should be compared.
type TimeKind int

const (
	TimeKindUnknown TimeKind = iota
	TimeKindUptimeSeconds
	TimeKindTimeOfDay
	TimeKindDateTime
)

// ParsedTime represents a parsed user time input (uptime, clock time, or absolute datetime).
type ParsedTime struct {
	Kind        TimeKind
	Seconds     float64   // Used when Kind == TimeKindUptimeSeconds
	Nanoseconds int64     // Used when Kind == TimeKindTimeOfDay (nanoseconds since midnight)
	DateTime    time.Time // Used when Kind == TimeKindDateTime
	Raw         string
}

// regexUptime matches uptime within brackets or at start of raw line like "[  250.123]" or "[250.123]"
var regexUptime = regexp.MustCompile(`\[\s*(\d+(?:\.\d+)?)\s*\]`)

// ParseTimeQuery parses a user-supplied time string into a ParsedTime object.
// Supported formats:
// - Uptime float / int: "250", "250.", "250.123456"
// - Duration: "250s", "4m10s", "500ms"
// - Time-of-day: "14:15:30", "14:15:30.500", "14:15:30,500", "00:00:05.200"
// - Datetime: "2026-10-29T14:15:30Z", "2026-10-29 14:15:30"
func ParseTimeQuery(s string) (ParsedTime, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ParsedTime{}, fmt.Errorf("empty time string")
	}

	res := ParsedTime{Raw: trimmed}

	// 1. Check if it's an uptime number ending in '.' (e.g. "250.")
	numCandidate := trimmed
	if strings.HasSuffix(numCandidate, ".") {
		numCandidate = strings.TrimSuffix(numCandidate, ".")
	}

	if sec, err := strconv.ParseFloat(numCandidate, 64); err == nil {
		res.Kind = TimeKindUptimeSeconds
		res.Seconds = sec
		return res, nil
	}

	// 2. Check if it's a duration (e.g. "250s", "2m", "100ms")
	if dur, err := time.ParseDuration(trimmed); err == nil {
		res.Kind = TimeKindUptimeSeconds
		res.Seconds = dur.Seconds()
		return res, nil
	}

	// 3. Check ISO8601 / RFC3339 datetime
	dateTimeLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	}
	for _, layout := range dateTimeLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			res.Kind = TimeKindDateTime
			res.DateTime = t
			return res, nil
		}
	}

	// 4. Check time-of-day clock layouts (e.g. "15:04:05.000", "15:04:05")
	normalizedClock := strings.ReplaceAll(trimmed, ",", ".")
	clockLayouts := []string{
		"15:04:05.000000",
		"15:04:05.000",
		"15:04:05",
		"04:05.000",
		"04:05",
	}
	for _, layout := range clockLayouts {
		if t, err := time.Parse(layout, normalizedClock); err == nil {
			res.Kind = TimeKindTimeOfDay
			ns := int64(t.Hour())*3600*1e9 + int64(t.Minute())*60*1e9 + int64(t.Second())*1e9 + int64(t.Nanosecond())
			res.Nanoseconds = ns
			return res, nil
		}
	}

	return ParsedTime{}, fmt.Errorf("unrecognized time or uptime format: %q", s)
}

// ExtractRecordTime inspects a record and extracts available time representations:
// - uptime: parsed float seconds from fields or raw line brackets
// - wallTime: record wall-clock ingestion timestamp if not zero
// - timeOfDayNs: nanoseconds since midnight if wallTime or time field exists
func ExtractRecordTime(r record.Record) (uptime *float64, wallTime *time.Time, timeOfDayNs *int64) {
	// Check fields for explicit uptime
	upStr := r.Get("uptime")
	if upStr != "" {
		if sec, err := strconv.ParseFloat(strings.TrimSpace(upStr), 64); err == nil {
			uptime = &sec
		}
	}

	// If no explicit uptime field, inspect raw text for "[ <float> ]"
	if uptime == nil && r.Raw != "" {
		if matches := regexUptime.FindStringSubmatch(r.Raw); len(matches) > 1 {
			if sec, err := strconv.ParseFloat(matches[1], 64); err == nil {
				uptime = &sec
			}
		}
	}

	// Check wall-clock timestamp
	if !r.Timestamp.IsZero() {
		wt := r.Timestamp
		wallTime = &wt
		ns := int64(wt.Hour())*3600*1e9 + int64(wt.Minute())*60*1e9 + int64(wt.Second())*1e9 + int64(wt.Nanosecond())
		timeOfDayNs = &ns
	}

	// If wall-clock timestamp is zero, check parsed time field
	if timeOfDayNs == nil {
		tStr := r.Get("time")
		if tStr == "" {
			tStr = r.Get("timestamp")
		}
		if tStr != "" {
			normalized := strings.ReplaceAll(tStr, ",", ".")
			for _, layout := range []string{"15:04:05.000000", "15:04:05.000", "15:04:05"} {
				if t, err := time.Parse(layout, normalized); err == nil {
					ns := int64(t.Hour())*3600*1e9 + int64(t.Minute())*60*1e9 + int64(t.Second())*1e9 + int64(t.Nanosecond())
					timeOfDayNs = &ns
					break
				}
			}
		}
	}

	return uptime, wallTime, timeOfDayNs
}

// IsRecordInTimeRange returns true if the record falls between from and to.
func IsRecordInTimeRange(r record.Record, from, to ParsedTime) bool {
	uptime, wallTime, timeOfDayNs := ExtractRecordTime(r)

	// If comparing by uptime seconds
	if from.Kind == TimeKindUptimeSeconds || to.Kind == TimeKindUptimeSeconds {
		if uptime == nil {
			return false
		}
		return *uptime >= from.Seconds && *uptime <= to.Seconds
	}

	// If comparing by absolute datetime
	if from.Kind == TimeKindDateTime || to.Kind == TimeKindDateTime {
		if wallTime == nil {
			return false
		}
		if !from.DateTime.IsZero() && wallTime.Before(from.DateTime) {
			return false
		}
		if !to.DateTime.IsZero() && wallTime.After(to.DateTime) {
			return false
		}
		return true
	}

	// If comparing by time of day
	if from.Kind == TimeKindTimeOfDay || to.Kind == TimeKindTimeOfDay {
		if timeOfDayNs == nil {
			return false
		}
		return *timeOfDayNs >= from.Nanoseconds && *timeOfDayNs <= to.Nanoseconds
	}

	return false
}

// RecordDistanceToTime computes the absolute distance in seconds between the record and the target time.
// Returns (distance, true) if comparable, or (0, false) if no compatible time could be extracted.
func RecordDistanceToTime(r record.Record, target ParsedTime) (float64, bool) {
	uptime, wallTime, timeOfDayNs := ExtractRecordTime(r)

	switch target.Kind {
	case TimeKindUptimeSeconds:
		if uptime == nil {
			return 0, false
		}
		return math.Abs(*uptime - target.Seconds), true

	case TimeKindDateTime:
		if wallTime == nil {
			return 0, false
		}
		return math.Abs(wallTime.Sub(target.DateTime).Seconds()), true

	case TimeKindTimeOfDay:
		if timeOfDayNs == nil {
			return 0, false
		}
		diffNs := float64(*timeOfDayNs - target.Nanoseconds)
		return math.Abs(diffNs / 1e9), true

	default:
		return 0, false
	}
}
