package timing

import (
	"fmt"
	"sync"
	"time"
)

// Level categorizes an inter-log delta based on its relation to expected cadence.
type Level int

const (
	LevelNormal Level = iota // Cadence: normal steady-state stream pace (0.3x - 2.5x baseline)
	LevelBurst               // Burst: rapid back-to-back logs (< 0.3x baseline)
	LevelHiccup              // Hiccup: noticeable slowdown (2.5x - 5.0x baseline)
	LevelAlert               // Anomaly: critical stall or timeout (> 5.0x baseline or fixed threshold)
)

// Config configures dynamic and fixed latency threshold parameters.
type Config struct {
	WarnRatio      float64       `yaml:"warn_ratio"`      // Dynamic threshold multiplier (default: 2.5)
	AlertRatio     float64       `yaml:"alert_ratio"`     // Dynamic threshold multiplier (default: 5.0)
	WarnThreshold  time.Duration `yaml:"warn_threshold"`  // Fixed threshold override (e.g. 50ms)
	AlertThreshold time.Duration `yaml:"alert_threshold"` // Fixed threshold override (e.g. 200ms)
	Alpha          float64       `yaml:"alpha"`           // EMA smoothing factor 0 < alpha <= 1 (default: 0.1)
}

// DefaultConfig returns baseline timing parameters for general serial streams.
func DefaultConfig() Config {
	return Config{
		WarnRatio:  2.5,
		AlertRatio: 5.0,
		Alpha:      0.1,
	}
}

// Tracker computes a rolling Exponential Moving Average (EMA) of inter-log deltas
// and dynamically identifies latency bursts, hiccups, and anomalies.
type Tracker struct {
	mu    sync.RWMutex
	cfg   Config
	ema   float64 // in nanoseconds
	count int
}

// NewTracker creates a new Tracker with the specified config.
func NewTracker(cfg Config) *Tracker {
	if cfg.WarnRatio <= 0 {
		cfg.WarnRatio = 2.5
	}
	if cfg.AlertRatio <= 0 {
		cfg.AlertRatio = 5.0
	}
	if cfg.Alpha <= 0 || cfg.Alpha > 1.0 {
		cfg.Alpha = 0.1
	}
	return &Tracker{cfg: cfg}
}

// Update records a new inter-log delta and updates the moving average.
func (t *Tracker) Update(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if d <= 0 {
		return
	}
	ns := float64(d.Nanoseconds())
	t.count++
	if t.count == 1 {
		t.ema = ns
		return
	}
	t.ema = t.cfg.Alpha*ns + (1.0-t.cfg.Alpha)*t.ema
}

// Classify classifies d into LevelNormal, LevelBurst, LevelHiccup, or LevelAlert.
func (t *Tracker) Classify(d time.Duration) Level {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if d <= 0 {
		return LevelNormal
	}

	// 1. Fixed threshold overrides take precedence if configured in profile
	if t.cfg.AlertThreshold > 0 && d >= t.cfg.AlertThreshold {
		return LevelAlert
	}
	if t.cfg.WarnThreshold > 0 && d >= t.cfg.WarnThreshold {
		return LevelHiccup
	}
	if t.cfg.AlertThreshold > 0 || t.cfg.WarnThreshold > 0 {
		return LevelNormal
	}

	// 2. Dynamic EMA classification
	if t.count < 3 || t.ema <= 0 {
		return LevelNormal
	}

	ratio := float64(d.Nanoseconds()) / t.ema
	if ratio >= t.cfg.AlertRatio {
		return LevelAlert
	}
	if ratio >= t.cfg.WarnRatio {
		return LevelHiccup
	}
	if ratio < 0.3 {
		return LevelBurst
	}
	return LevelNormal
}

// Average returns the current moving average baseline duration.
func (t *Tracker) Average() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return time.Duration(t.ema)
}

// FormatDelta formats d into a compact, human-readable elapsed duration with fixed prefixes.
func FormatDelta(d time.Duration) string {
	if d <= 0 {
		return "+0.0ms"
	}
	if d < time.Microsecond {
		return fmt.Sprintf("+%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("+%dµs", d.Microseconds())
	}
	if d < 10*time.Millisecond {
		return fmt.Sprintf("+%.2fms", float64(d.Microseconds())/1000.0)
	}
	if d < time.Second {
		return fmt.Sprintf("+%.1fms", float64(d.Microseconds())/1000.0)
	}
	if d < time.Minute {
		return fmt.Sprintf("+%.2fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("+%dm%02ds", mins, secs)
}
