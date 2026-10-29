package mcp

import (
	"math"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func TestParseTimeQuery(t *testing.T) {
	tests := []struct {
		input       string
		wantKind    TimeKind
		wantSeconds float64
	}{
		{"250", TimeKindUptimeSeconds, 250.0},
		{"250.", TimeKindUptimeSeconds, 250.0},
		{"250.123456", TimeKindUptimeSeconds, 250.123456},
		{"0.5", TimeKindUptimeSeconds, 0.5},
		{"120s", TimeKindUptimeSeconds, 120.0},
		{"2m", TimeKindUptimeSeconds, 120.0},
		{"14:15:30", TimeKindTimeOfDay, 0},
		{"14:15:30.500", TimeKindTimeOfDay, 0},
		{"2026-10-29T14:15:30Z", TimeKindDateTime, 0},
	}

	for _, tc := range tests {
		pt, err := ParseTimeQuery(tc.input)
		if err != nil {
			t.Fatalf("ParseTimeQuery(%q) error: %v", tc.input, err)
		}
		if pt.Kind != tc.wantKind {
			t.Errorf("ParseTimeQuery(%q) kind = %v, want %v", tc.input, pt.Kind, tc.wantKind)
		}
		if tc.wantKind == TimeKindUptimeSeconds && math.Abs(pt.Seconds-tc.wantSeconds) > 1e-6 {
			t.Errorf("ParseTimeQuery(%q) seconds = %v, want %v", tc.input, pt.Seconds, tc.wantSeconds)
		}
	}
}

func TestTimeRangeMatching_Uptime(t *testing.T) {
	from, err := ParseTimeQuery("250.0")
	if err != nil {
		t.Fatal(err)
	}
	to, err := ParseTimeQuery("255.0")
	if err != nil {
		t.Fatal(err)
	}

	recIn := record.Record{
		Fields: map[string]string{"uptime": "252.450"},
		Raw:    "[  252.450] <inf> sensor: ok",
	}
	recBelow := record.Record{
		Fields: map[string]string{"uptime": "249.999"},
		Raw:    "[  249.999] <inf> sensor: boot",
	}
	recAbove := record.Record{
		Fields: map[string]string{"uptime": "255.001"},
		Raw:    "[  255.001] <inf> sensor: shutdown",
	}
	recRawOnly := record.Record{
		Raw: "[  251.000] <inf> sensor: raw parsed",
	}

	if !IsRecordInTimeRange(recIn, from, to) {
		t.Errorf("expected recIn to match range 250..255")
	}
	if IsRecordInTimeRange(recBelow, from, to) {
		t.Errorf("expected recBelow not to match range 250..255")
	}
	if IsRecordInTimeRange(recAbove, from, to) {
		t.Errorf("expected recAbove not to match range 250..255")
	}
	if !IsRecordInTimeRange(recRawOnly, from, to) {
		t.Errorf("expected recRawOnly to extract uptime from raw brackets and match range 250..255")
	}
}

func TestRecordDistanceToTime_Closest(t *testing.T) {
	target, err := ParseTimeQuery("250.")
	if err != nil {
		t.Fatal(err)
	}

	records := []record.Record{
		{ID: 1, Fields: map[string]string{"uptime": "100.0"}, Raw: "[ 100.0] line 1"},
		{ID: 2, Fields: map[string]string{"uptime": "249.8"}, Raw: "[ 249.8] line 2"},
		{ID: 3, Fields: map[string]string{"uptime": "250.1"}, Raw: "[ 250.1] line 3"},
		{ID: 4, Fields: map[string]string{"uptime": "300.0"}, Raw: "[ 300.0] line 4"},
	}

	bestIdx := -1
	bestDist := math.MaxFloat64
	for i, r := range records {
		dist, ok := RecordDistanceToTime(r, target)
		if ok && dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	if bestIdx != 2 {
		t.Fatalf("expected closest record to 250. to be ID 3 (index 2, 250.1), got index %d", bestIdx)
	}
}

func TestTimeRangeMatching_DateTime(t *testing.T) {
	t1 := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 29, 10, 5, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 29, 10, 10, 0, 0, time.UTC)

	from, _ := ParseTimeQuery("2026-10-29T10:02:00Z")
	to, _ := ParseTimeQuery("2026-10-29T10:08:00Z")

	rec1 := record.Record{ID: 1, Timestamp: t1}
	rec2 := record.Record{ID: 2, Timestamp: t2}
	rec3 := record.Record{ID: 3, Timestamp: t3}

	if IsRecordInTimeRange(rec1, from, to) {
		t.Errorf("rec1 should be outside range")
	}
	if !IsRecordInTimeRange(rec2, from, to) {
		t.Errorf("rec2 should be inside range")
	}
	if IsRecordInTimeRange(rec3, from, to) {
		t.Errorf("rec3 should be outside range")
	}
}
