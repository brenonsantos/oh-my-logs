package sim

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Scenario controls what background traffic patterns the simulated device emits.
type Scenario string

const (
	ScenarioNormal   Scenario = "normal"
	ScenarioVTStress Scenario = "vt-stress"
	ScenarioFlood    Scenario = "flood"
	ScenarioPayloads Scenario = "payloads"
	ScenarioAll      Scenario = "all"
)

// ParseScenario validates a scenario name. Empty defaults to ScenarioAll.
func ParseScenario(name string) (Scenario, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "all":
		return ScenarioAll, nil
	case "normal":
		return ScenarioNormal, nil
	case "vt-stress", "vt":
		return ScenarioVTStress, nil
	case "flood", "burst":
		return ScenarioFlood, nil
	case "payloads", "payload":
		return ScenarioPayloads, nil
	default:
		return "", fmt.Errorf("sim: unknown scenario %q (supported: normal, vt-stress, flood, payloads, all)", name)
	}
}

// Config configures a simulated embedded UART device.
type Config struct {
	Profile      Profile
	Scenario     Scenario
	RateHz       float64       // background async logs per second (0 = quiet)
	MidLinePause time.Duration // pause injected mid-line during VT stress (0 = disabled)
	StartupBurst int           // optional burst emitted right after boot banner
	Seed         int64         // deterministic RNG seed (0 = time-based)
}

// DefaultConfig returns a ready-to-use Zephyr configuration covering all scenarios.
func DefaultConfig() Config {
	p, _ := ParseProfile("zephyr")
	return Config{
		Profile:      p,
		Scenario:     ScenarioAll,
		RateHz:       1.5,
		MidLinePause: 50 * time.Millisecond,
	}
}

// Device simulates an embedded MCU sharing a single UART between an interactive
// VT100 shell and asynchronous debug log output.
type Device struct {
	cfg Config
	rw  io.ReadWriter

	mu          sync.Mutex
	shell       *shellState
	rng         *rand.Rand
	startTime   time.Time
	seq         uint64
	logRateHz   float64
	logPaused   bool
	lastRXWasCR bool
}

// NewDevice creates a simulated device attached to rw (a PTY, pipe, or socket).
func NewDevice(rw io.ReadWriter, cfg Config) *Device {
	if cfg.Profile.Prompt == "" {
		cfg.Profile, _ = ParseProfile("zephyr")
	}
	if cfg.Scenario == "" {
		cfg.Scenario = ScenarioAll
	}
	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Device{
		cfg:       cfg,
		rw:        rw,
		shell:     newShellState(),
		rng:       rand.New(rand.NewSource(seed)),
		startTime: time.Now(),
		logRateHz: cfg.RateHz,
	}
}

// Run starts the simulated boot sequence, interactive RX shell loop, and
// background async log generator. It blocks until ctx is cancelled or rw closes.
func (d *Device) Run(ctx context.Context) error {
	d.mu.Lock()
	d.emitBootBannerLocked()
	if d.cfg.StartupBurst > 0 {
		d.emitBurstLocked(d.cfg.StartupBurst)
	}
	d.writeRaw(d.cfg.Profile.Prompt)
	d.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.rxLoop(ctx)
	}()

	go d.txLoop(ctx)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (d *Device) rxLoop(ctx context.Context) error {
	buf := make([]byte, 256)
	for {
		n, err := d.rw.Read(buf)
		if n > 0 {
			d.mu.Lock()
			for _, b := range buf[:n] {
				d.feedInputByte(b)
			}
			d.mu.Unlock()
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func (d *Device) txLoop(ctx context.Context) {
	for {
		d.mu.Lock()
		paused := d.logPaused
		rate := d.logRateHz
		d.mu.Unlock()

		var wait time.Duration
		if paused || rate <= 0 {
			wait = 100 * time.Millisecond
		} else {
			wait = time.Duration(float64(time.Second) / rate)
			if wait < time.Millisecond {
				wait = time.Millisecond
			}
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if paused || rate <= 0 {
			continue
		}

		d.emitPeriodicEvent(ctx)
	}
}

func (d *Device) emitPeriodicEvent(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seq++
	seq := d.seq

	switch d.cfg.Scenario {
	case ScenarioFlood:
		d.emitAsyncBatchLocked(50)
		return
	case ScenarioPayloads:
		d.emitAsyncPayloadLocked(seq)
		return
	case ScenarioVTStress:
		d.emitAsyncVTStressStepLocked(ctx, seq)
		return
	case ScenarioNormal:
		d.emitAsyncLogLocked(ctx, d.nextNormalLogLocked(), false)
		return
	default: // ScenarioAll
		switch seq % 6 {
		case 0:
			d.emitAsyncPayloadLocked(seq)
		case 3:
			d.emitAsyncVTStressStepLocked(ctx, seq)
		default:
			d.emitAsyncLogLocked(ctx, d.nextNormalLogLocked(), seq%2 == 0)
		}
	}
}

// emitAsyncLogLocked erases the active prompt + any typed user input, prints
// msg (optionally pausing mid-line to test framing), and redraws the prompt
// with the user's uncommitted input preserved.
func (d *Device) emitAsyncLogLocked(ctx context.Context, msg string, pauseMidLine bool) {
	erase := d.cfg.Profile.EraseSequence(d.shell.promptLineWidth(d.cfg.Profile.Prompt))
	if pauseMidLine && d.cfg.MidLinePause > 0 && len(msg) > 16 {
		split := len(msg) / 2
		d.writeRaw(erase + msg[:split])
		// Release lock? Keep lock held briefly so user keystrokes don't race mid-line,
		// matching how an RTOS UART TX FIFO stalls during a workqueue delay.
		t := time.NewTimer(d.cfg.MidLinePause)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		d.writeRaw(msg[split:] + "\r\n")
	} else {
		d.writeRaw(erase + msg + "\r\n")
	}
	d.redrawPromptAfterAsyncLocked()
}

func (d *Device) redrawPromptAfterAsyncLocked() {
	d.writeRaw(d.cfg.Profile.Prompt + d.shell.currentText())
	if back := len(d.shell.buf) - d.shell.cursor; back > 0 {
		d.writeRaw(fmt.Sprintf("\x1b[%dD", back))
	}
}

func (d *Device) emitAsyncBatchLocked(count int) {
	erase := d.cfg.Profile.EraseSequence(d.shell.promptLineWidth(d.cfg.Profile.Prompt))
	var sb strings.Builder
	sb.Grow(count * 80)
	sb.WriteString(erase)
	for i := 0; i < count; i++ {
		d.seq++
		sb.WriteString(d.nextNormalLogLocked())
		sb.WriteString("\r\n")
	}
	d.writeRaw(sb.String())
	d.redrawPromptAfterAsyncLocked()
}

func (d *Device) emitAsyncPayloadLocked(seq uint64) {
	erase := d.cfg.Profile.EraseSequence(d.shell.promptLineWidth(d.cfg.Profile.Prompt))
	d.writeRaw(erase)
	switch seq % 3 {
	case 0:
		d.emitPayloadLocked("json")
	case 1:
		d.emitPayloadLocked("hex")
	default:
		d.emitPayloadLocked("telemetry")
	}
	d.redrawPromptAfterAsyncLocked()
}

func (d *Device) emitAsyncVTStressStepLocked(ctx context.Context, seq uint64) {
	erase := d.cfg.Profile.EraseSequence(d.shell.promptLineWidth(d.cfg.Profile.Prompt))
	switch seq % 3 {
	case 0:
		// Progress bar overwrite + short OK confirmation.
		d.writeRaw(erase)
		d.emitProgressBarLocked()
		d.writeRaw("OK\r\n")
		d.redrawPromptAfterAsyncLocked()
	case 1:
		// Mid-line pause across a long log line.
		msg := d.cfg.Profile.FormatLog(d.uptime(), LevelInfo, "mqtt",
			fmt.Sprintf("HandlePublishToBrokerWork(): published packet seq=%d payload_len=128", seq))
		d.emitAsyncLogLocked(ctx, msg, true)
	default:
		// Inline backspace correction + OSC title + UTF-8.
		msg := d.cfg.Profile.FormatLog(d.uptime(), LevelWarn, "sensor",
			"calibraton\b\bion: temp=24.6°C \x1b]0;sim-title\x07status=nominal")
		d.emitAsyncLogLocked(ctx, msg, false)
	}
}

func (d *Device) nextNormalLogLocked() string {
	mods := d.cfg.Profile.Modules
	mod := mods[d.rng.Intn(len(mods))]

	roll := d.rng.Intn(100)
	var lvl Level
	var msg string
	switch {
	case roll < 10:
		lvl = LevelError
		msg = fmt.Sprintf("I/O timeout on bus=%d retries=%d err=-116", d.rng.Intn(3), 1+d.rng.Intn(3))
	case roll < 25:
		lvl = LevelWarn
		msg = fmt.Sprintf("RSSI low (-%d dBm), channel=%d seq=%d", 75+d.rng.Intn(20), 1+d.rng.Intn(11), d.seq)
	case roll < 45:
		lvl = LevelDebug
		msg = fmt.Sprintf("RX frame len=%d crc=0x%04x seq=%d", 16+d.rng.Intn(240), d.rng.Intn(0xffff), d.seq)
	default:
		lvl = LevelInfo
		msg = fmt.Sprintf("Heartbeat ok uptime_ms=%d heap_free=%d seq=%d", d.uptime().Milliseconds(), 32768+d.rng.Intn(16384), d.seq)
	}
	return d.cfg.Profile.FormatLog(d.uptime(), lvl, mod, msg)
}

func (d *Device) emitBootBannerLocked() {
	for _, line := range d.cfg.Profile.Banner {
		d.writeRaw(line + "\r\n")
	}
}

func (d *Device) emitProgressBarLocked() {
	steps := []string{
		"flashing [##        ]  20%",
		"flashing [#####     ]  50%",
		"flashing [########  ]  80%",
		"flashing [##########] 100%",
	}
	for _, st := range steps {
		d.writeRaw("\r" + st)
		if d.cfg.MidLinePause > 0 {
			time.Sleep(d.cfg.MidLinePause / 2)
		}
	}
	d.writeRaw("\r\n")
}

func (d *Device) emitBurstLocked(count int) {
	t0 := time.Now()
	var sb strings.Builder
	// Write in 64-line chunks so PTY buffers flow smoothly at high throughput.
	const chunkLines = 64
	sb.Grow(chunkLines * 96)
	for i := 1; i <= count; i++ {
		d.seq++
		lvl := LevelInfo
		if i%50 == 0 {
			lvl = LevelWarn
		} else if i%200 == 0 {
			lvl = LevelError
		}
		sb.WriteString(d.cfg.Profile.FormatLog(d.uptime(), lvl, "burst",
			fmt.Sprintf("flood line %d/%d seq=%d", i, count, d.seq)))
		sb.WriteString("\r\n")
		if i%chunkLines == 0 || i == count {
			d.writeRaw(sb.String())
			sb.Reset()
		}
	}
	elapsed := time.Since(t0)
	summary := d.cfg.Profile.FormatLog(d.uptime(), LevelInfo, "burst",
		fmt.Sprintf("completed %d lines in %v", count, elapsed.Round(time.Microsecond)))
	d.writeRaw(summary + "\r\n")
}

func (d *Device) emitPayloadLocked(kind string) {
	u := d.uptime()
	if kind == "all" || kind == "json" {
		temp := 22.0 + float64(d.rng.Intn(80))/10.0
		mv := 3250 + d.rng.Intn(120)
		j := fmt.Sprintf(`{"device":"node-01","seq":%d,"temp_c":%.1f,"vbat_mv":%d,"state":"active"}`, d.seq, temp, mv)
		d.writeRaw(d.cfg.Profile.FormatLog(u, LevelInfo, "mqtt", "TX JSON "+j) + "\r\n")
	}
	if kind == "all" || kind == "hex" {
		hexFrame := fmt.Sprintf("AA 55 08 %02X %02X 00 1C 4F 9A 0D 0A", byte(d.seq>>8), byte(d.seq))
		d.writeRaw(d.cfg.Profile.FormatLog(u, LevelDebug, "ble", "RX PDU [11 bytes]: "+hexFrame) + "\r\n")
	}
	if kind == "all" || kind == "telemetry" {
		temp := 21.5 + float64(d.rng.Intn(90))/10.0
		curr := 95 + d.rng.Intn(85)
		rssi := -(50 + d.rng.Intn(35))
		t := fmt.Sprintf("temp=%.1f current_mA=%d rssi=%d seq=%d", temp, curr, rssi, d.seq)
		d.writeRaw(d.cfg.Profile.FormatLog(u, LevelInfo, "sensor", t) + "\r\n")
	}
}

func (d *Device) emitVTStressLocked() {
	u := d.uptime()
	// 1. Carriage-return overwrite progress bar.
	d.emitProgressBarLocked()
	// 2. Carriage-return + erase-to-end.
	d.writeRaw("stale status text 99%\r\x1b[K" + d.cfg.Profile.FormatLog(u, LevelInfo, "vt", "cleared via CR+ESC[K") + "\r\n")
	// 3. Inline backspace edit.
	d.writeRaw(d.cfg.Profile.FormatLog(u, LevelInfo, "vt", "helo\b\bllo backspace ok") + "\r\n")
	// 4. Cursor-forward spacing & tab stops.
	d.writeRaw("col1\x1b[4Ccol2\tcol3\r\n")
	// 5. Private mode + OSC + SGR color + UTF-8.
	d.writeRaw("\x1b[?25l\x1b]0;oml-sim\x07\x1b[32mUTF-8 café 24.5°C ✓\x1b[0m\x1b[?25h\r\n")
	// 6. Short legitimate lines.
	d.writeRaw("OK\r\nACK\r\n")
}

func (d *Device) uptime() time.Duration {
	return time.Since(d.startTime)
}

func (d *Device) writeRaw(s string) {
	if s == "" {
		return
	}
	_, _ = io.WriteString(d.rw, s)
}
