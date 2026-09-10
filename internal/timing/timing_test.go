package timing

import (
	"testing"
	"time"
)

func TestFormatDelta(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "+0.0ms"},
		{-5 * time.Millisecond, "+0.0ms"},
		{500 * time.Nanosecond, "+500ns"},
		{125 * time.Microsecond, "+125µs"},
		{2450 * time.Microsecond, "+2.45ms"},
		{45200 * time.Microsecond, "+45.2ms"},
		{500 * time.Millisecond, "+500.0ms"},
		{1450 * time.Millisecond, "+1.45s"},
		{18250 * time.Millisecond, "+18.25s"},
		{65 * time.Second, "+1m05s"},
		{124 * time.Second, "+2m04s"},
	}

	for _, tt := range tests {
		got := FormatDelta(tt.d)
		if got != tt.expected {
			t.Errorf("FormatDelta(%v) = %q, expected %q", tt.d, got, tt.expected)
		}
	}
}

func TestDynamicEMATracker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Alpha = 0.5 // fast test smoothing
	tracker := NewTracker(cfg)

	// In the beginning with < 3 samples, everything is normal
	if tracker.Classify(10*time.Millisecond) != LevelNormal {
		t.Errorf("expected LevelNormal for initial samples")
	}

	// Feed a steady 10ms stream
	for i := 0; i < 20; i++ {
		tracker.Update(10 * time.Millisecond)
	}

	avg := tracker.Average()
	if avg < 9*time.Millisecond || avg > 11*time.Millisecond {
		t.Errorf("expected average around 10ms, got %v", avg)
	}

	// 10ms should be normal cadence
	if got := tracker.Classify(10 * time.Millisecond); got != LevelNormal {
		t.Errorf("expected 10ms to be LevelNormal, got %v", got)
	}

	// 1ms is a rapid burst (< 0.3x)
	if got := tracker.Classify(1 * time.Millisecond); got != LevelBurst {
		t.Errorf("expected 1ms to be LevelBurst, got %v", got)
	}

	// 30ms is a hiccup (3.0x >= 2.5x)
	if got := tracker.Classify(30 * time.Millisecond); got != LevelHiccup {
		t.Errorf("expected 30ms to be LevelHiccup, got %v", got)
	}

	// 60ms is an alert anomaly (6.0x >= 5.0x)
	if got := tracker.Classify(60 * time.Millisecond); got != LevelAlert {
		t.Errorf("expected 60ms to be LevelAlert, got %v", got)
	}
}

func TestFixedThresholdOverride(t *testing.T) {
	cfg := Config{
		WarnThreshold:  50 * time.Millisecond,
		AlertThreshold: 200 * time.Millisecond,
	}
	tracker := NewTracker(cfg)

	// Feed 1ms loop
	for i := 0; i < 20; i++ {
		tracker.Update(1 * time.Millisecond)
	}

	// 10ms is 10x the moving average, but below the fixed 50ms warning threshold -> LevelNormal
	if got := tracker.Classify(10 * time.Millisecond); got != LevelNormal {
		t.Errorf("expected LevelNormal below fixed warn threshold, got %v", got)
	}

	// 80ms is above fixed warn threshold (50ms) -> LevelHiccup
	if got := tracker.Classify(80 * time.Millisecond); got != LevelHiccup {
		t.Errorf("expected LevelHiccup above fixed warn threshold, got %v", got)
	}

	// 250ms is above fixed alert threshold (200ms) -> LevelAlert
	if got := tracker.Classify(250 * time.Millisecond); got != LevelAlert {
		t.Errorf("expected LevelAlert above fixed alert threshold, got %v", got)
	}
}
