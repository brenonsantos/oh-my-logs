package sim

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// shellState implements a VT100 interactive line editor with cursor movement,
// command history, Tab completion, and built-in diagnostic commands.
// All methods are called while holding Device.mu.
type shellState struct {
	buf      []rune
	cursor   int
	history  []string
	histIdx  int // len(history) means editing fresh line
	savedBuf []rune

	// Escape sequence parser for incoming keystrokes (arrows, Home/End/Delete).
	escState uint8
	csiBuf   []byte

	// UTF-8 byte reassembly for incoming runes.
	utf    [utf8.UTFMax]byte
	utfLen int
}

const (
	escNone uint8 = iota
	escStart
	escCSI
)

var topCommands = []string{
	"burst",
	"help",
	"kernel",
	"log",
	"payload",
	"progress",
	"reboot",
	"status",
	"stress",
}

var subCommands = map[string][]string{
	"kernel":  {"threads", "uptime", "version"},
	"log":     {"pause", "rate", "resume"},
	"payload": {"hex", "json", "telemetry"},
	"stress":  {"vt"},
}

func newShellState() *shellState {
	return &shellState{}
}

func (s *shellState) currentText() string {
	return string(s.buf)
}

func (s *shellState) promptLineWidth(prompt string) int {
	return utf8.RuneCountInString(prompt) + len(s.buf)
}

// feedByte processes one byte received from the host terminal (oml).
func (d *Device) feedInputByte(b byte) {
	s := d.shell
	switch s.escState {
	case escStart:
		if b == '[' {
			s.escState = escCSI
			s.csiBuf = s.csiBuf[:0]
			return
		}
		// Alt+<key> shortcuts (e.g. Alt+b, Alt+f).
		s.escState = escNone
		switch b {
		case 'b', 'B':
			d.shellWordLeft()
		case 'f', 'F':
			d.shellWordRight()
		}
		return
	case escCSI:
		if b >= 0x40 && b <= 0x7e {
			seq := string(s.csiBuf) + string(b)
			s.escState = escNone
			d.handleCSIKey(seq)
			return
		}
		if len(s.csiBuf) < 16 {
			s.csiBuf = append(s.csiBuf, b)
		} else {
			s.escState = escNone
		}
		return
	}

	if s.utfLen > 0 || b >= 0x80 {
		s.utf[s.utfLen] = b
		s.utfLen++
		if !utf8.FullRune(s.utf[:s.utfLen]) {
			return
		}
		r, _ := utf8.DecodeRune(s.utf[:s.utfLen])
		s.utfLen = 0
		if r != utf8.RuneError {
			d.shellInsertRune(r)
		}
		return
	}

	switch b {
	case 0x1b:
		s.escState = escStart
	case '\r', '\n':
		// Ignore trailing \n immediately after \r so CRLF doesn't double-submit.
		if b == '\n' && d.lastRXWasCR {
			d.lastRXWasCR = false
			return
		}
		d.lastRXWasCR = (b == '\r')
		d.shellSubmit()
	case '\t':
		d.lastRXWasCR = false
		d.shellTabComplete()
	case 0x7f, '\b':
		d.lastRXWasCR = false
		d.shellBackspace()
	case 0x01: // Ctrl+A (Home)
		d.shellMoveCursor(0)
	case 0x03: // Ctrl+C (Interrupt)
		d.writeRaw("^C\r\n")
		s.buf = s.buf[:0]
		s.cursor = 0
		s.histIdx = len(s.history)
		d.writeRaw(d.cfg.Profile.Prompt)
	case 0x04: // Ctrl+D (EOF / delete char)
		if len(s.buf) > 0 {
			d.shellDelete()
		} else {
			d.writeRaw("\r\n(use 'reboot' or Ctrl+C; simulated UART remains open)\r\n" + d.cfg.Profile.Prompt)
		}
	case 0x05: // Ctrl+E (End)
		d.shellMoveCursor(len(s.buf))
	case 0x0b: // Ctrl+K (Kill to end)
		if s.cursor < len(s.buf) {
			s.buf = s.buf[:s.cursor]
			d.writeRaw("\x1b[K")
		}
	case 0x0c: // Ctrl+L (Redraw prompt line)
		d.redrawPromptLine()
	case 0x15: // Ctrl+U (Kill entire line)
		s.buf = s.buf[:0]
		s.cursor = 0
		d.redrawPromptLine()
	case 0x17: // Ctrl+W (Kill previous word)
		d.shellKillWord()
	default:
		d.lastRXWasCR = false
		if b >= 0x20 && b < 0x7f {
			d.shellInsertRune(rune(b))
		}
	}
}

func (d *Device) handleCSIKey(seq string) {
	switch seq {
	case "A": // Up
		d.shellHistoryPrev()
	case "B": // Down
		d.shellHistoryNext()
	case "C": // Right
		if d.shell.cursor < len(d.shell.buf) {
			d.shell.cursor++
			d.writeRaw("\x1b[C")
		}
	case "D": // Left
		if d.shell.cursor > 0 {
			d.shell.cursor--
			d.writeRaw("\x1b[D")
		}
	case "H", "1~": // Home
		d.shellMoveCursor(0)
	case "F", "4~": // End
		d.shellMoveCursor(len(d.shell.buf))
	case "3~": // Delete
		d.shellDelete()
	}
}

func (d *Device) shellInsertRune(r rune) {
	s := d.shell
	if s.cursor == len(s.buf) {
		s.buf = append(s.buf, r)
		s.cursor++
		d.writeRaw(string(r))
		return
	}
	s.buf = append(s.buf, 0)
	copy(s.buf[s.cursor+1:], s.buf[s.cursor:])
	s.buf[s.cursor] = r
	s.cursor++
	d.redrawPromptLine()
}

func (d *Device) shellBackspace() {
	s := d.shell
	if s.cursor == 0 {
		return
	}
	if s.cursor == len(s.buf) {
		s.buf = s.buf[:len(s.buf)-1]
		s.cursor--
		d.writeRaw("\b \b")
		return
	}
	s.buf = append(s.buf[:s.cursor-1], s.buf[s.cursor:]...)
	s.cursor--
	d.redrawPromptLine()
}

func (d *Device) shellDelete() {
	s := d.shell
	if s.cursor >= len(s.buf) {
		return
	}
	s.buf = append(s.buf[:s.cursor], s.buf[s.cursor+1:]...)
	d.redrawPromptLine()
}

func (d *Device) shellMoveCursor(target int) {
	s := d.shell
	if target < 0 {
		target = 0
	}
	if target > len(s.buf) {
		target = len(s.buf)
	}
	diff := target - s.cursor
	s.cursor = target
	if diff > 0 {
		d.writeRaw(fmt.Sprintf("\x1b[%dC", diff))
	} else if diff < 0 {
		d.writeRaw(fmt.Sprintf("\x1b[%dD", -diff))
	}
}

func (d *Device) shellWordLeft() {
	s := d.shell
	pos := s.cursor
	for pos > 0 && s.buf[pos-1] == ' ' {
		pos--
	}
	for pos > 0 && s.buf[pos-1] != ' ' {
		pos--
	}
	d.shellMoveCursor(pos)
}

func (d *Device) shellWordRight() {
	s := d.shell
	pos := s.cursor
	for pos < len(s.buf) && s.buf[pos] != ' ' {
		pos++
	}
	for pos < len(s.buf) && s.buf[pos] == ' ' {
		pos++
	}
	d.shellMoveCursor(pos)
}

func (d *Device) shellKillWord() {
	s := d.shell
	if s.cursor == 0 {
		return
	}
	start := s.cursor
	for start > 0 && s.buf[start-1] == ' ' {
		start--
	}
	for start > 0 && s.buf[start-1] != ' ' {
		start--
	}
	s.buf = append(s.buf[:start], s.buf[s.cursor:]...)
	s.cursor = start
	d.redrawPromptLine()
}

func (d *Device) shellHistoryPrev() {
	s := d.shell
	if len(s.history) == 0 || s.histIdx == 0 {
		return
	}
	if s.histIdx == len(s.history) {
		s.savedBuf = append([]rune(nil), s.buf...)
	}
	s.histIdx--
	s.buf = []rune(s.history[s.histIdx])
	s.cursor = len(s.buf)
	d.redrawPromptLine()
}

func (d *Device) shellHistoryNext() {
	s := d.shell
	if s.histIdx >= len(s.history) {
		return
	}
	s.histIdx++
	if s.histIdx == len(s.history) {
		s.buf = append([]rune(nil), s.savedBuf...)
	} else {
		s.buf = []rune(s.history[s.histIdx])
	}
	s.cursor = len(s.buf)
	d.redrawPromptLine()
}

func (d *Device) redrawPromptLine() {
	s := d.shell
	d.writeRaw("\r\x1b[K" + d.cfg.Profile.Prompt + string(s.buf))
	if back := len(s.buf) - s.cursor; back > 0 {
		d.writeRaw(fmt.Sprintf("\x1b[%dD", back))
	}
}

func (d *Device) shellTabComplete() {
	s := d.shell
	line := string(s.buf[:s.cursor])
	trailingSpace := strings.HasSuffix(line, " ")
	fields := strings.Fields(line)

	var candidates []string
	var prefix string

	switch {
	case len(fields) == 0:
		candidates = topCommands
	case len(fields) == 1 && !trailingSpace:
		prefix = fields[0]
		for _, c := range topCommands {
			if strings.HasPrefix(c, prefix) {
				candidates = append(candidates, c)
			}
		}
	case len(fields) == 1 && trailingSpace:
		candidates = subCommands[fields[0]]
	case len(fields) == 2 && !trailingSpace:
		prefix = fields[1]
		for _, sub := range subCommands[fields[0]] {
			if strings.HasPrefix(sub, prefix) {
				candidates = append(candidates, sub)
			}
		}
	}

	if len(candidates) == 0 {
		d.writeRaw("\x07") // BEL
		return
	}

	sort.Strings(candidates)
	if len(candidates) == 1 {
		suffix := candidates[0][len(prefix):]
		if len(fields) <= 1 && !trailingSpace {
			if _, hasSub := subCommands[candidates[0]]; hasSub {
				suffix += " "
			}
		}
		for _, r := range suffix {
			d.shellInsertRune(r)
		}
		return
	}

	// Complete longest common prefix if it extends beyond prefix.
	lcp := longestCommonPrefix(candidates)
	if len(lcp) > len(prefix) {
		for _, r := range lcp[len(prefix):] {
			d.shellInsertRune(r)
		}
		return
	}

	// Ambiguous: display multi-column candidate list and redraw prompt + buffer.
	d.writeRaw("\r\n")
	var row strings.Builder
	for i, c := range candidates {
		if i > 0 {
			row.WriteString("\x1b[4C")
		}
		row.WriteString(c)
	}
	d.writeRaw(row.String() + "\r\n")
	d.redrawPromptLine()
}

func longestCommonPrefix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	p := items[0]
	for _, s := range items[1:] {
		for !strings.HasPrefix(s, p) {
			if p == "" {
				return ""
			}
			p = p[:len(p)-1]
		}
	}
	return p
}

func (d *Device) shellSubmit() {
	s := d.shell
	cmdLine := strings.TrimSpace(string(s.buf))
	d.writeRaw("\r\n")
	s.buf = s.buf[:0]
	s.cursor = 0

	if cmdLine != "" {
		if len(s.history) == 0 || s.history[len(s.history)-1] != cmdLine {
			s.history = append(s.history, cmdLine)
		}
	}
	s.histIdx = len(s.history)
	s.savedBuf = nil

	if cmdLine != "" {
		d.execCommand(cmdLine)
	}
	d.writeRaw(d.cfg.Profile.Prompt)
}

func (d *Device) execCommand(cmdLine string) {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return
	}

	switch fields[0] {
	case "help":
		d.writeRaw(strings.Join([]string{
			"Available commands:",
			"  help                      : Show this command reference",
			"  kernel <subcmd>           : version | uptime | threads",
			"  status                    : Print short OK/ACK status lines",
			"  burst [n]                 : Emit n log lines at max throughput (default 1000)",
			"  progress                  : Simulate carriage-return progress bar",
			"  payload <kind>            : json | hex | telemetry",
			"  stress vt                 : Run VT100 framing & escape gauntlet",
			"  log <rate Hz|pause|resume>: Control background async log stream",
			"  reboot                    : Emit simulated boot banner & sequence",
		}, "\r\n") + "\r\n")

	case "status":
		d.writeRaw("OK\r\nACK\r\n")

	case "kernel":
		sub := "version"
		if len(fields) > 1 {
			sub = fields[1]
		}
		switch sub {
		case "version":
			d.writeRaw(fmt.Sprintf("Simulated %s firmware v1.0.0 (ARM Cortex-M4)\r\n", d.cfg.Profile.Kind))
		case "uptime":
			d.writeRaw(fmt.Sprintf("uptime: %d ms\r\n", d.uptime().Milliseconds()))
		case "threads":
			d.writeRaw(strings.Join([]string{
				"Scheduler threads:",
				"  0x20001040  main         prio=0   state=running   stack=512/2048",
				"  0x20001880  shell_uart   prio=14  state=ready     stack=768/2048",
				"  0x20002100  net_rx       prio=7   state=pending   stack=640/1536",
				"  0x20002900  sensor_wq    prio=10  state=sleeping  stack=384/1024",
			}, "\r\n") + "\r\n")
		default:
			d.writeRaw(fmt.Sprintf("kernel: unknown subcommand %q (try: version, uptime, threads)\r\n", sub))
		}

	case "progress":
		d.emitProgressBarLocked()

	case "burst":
		count := 1000
		if len(fields) > 1 {
			if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 {
				if n > 100000 {
					n = 100000
				}
				count = n
			}
		}
		d.emitBurstLocked(count)

	case "payload":
		kind := "all"
		if len(fields) > 1 {
			kind = fields[1]
		}
		d.emitPayloadLocked(kind)

	case "stress":
		d.emitVTStressLocked()

	case "log":
		if len(fields) < 2 {
			d.writeRaw(fmt.Sprintf("log: rate=%.2f Hz, paused=%v\r\n", d.logRateHz, d.logPaused))
			return
		}
		switch fields[1] {
		case "pause":
			d.logPaused = true
			d.writeRaw("Background logging paused\r\n")
		case "resume":
			d.logPaused = false
			d.writeRaw("Background logging resumed\r\n")
		case "rate":
			if len(fields) < 3 {
				d.writeRaw("Usage: log rate <hz>\r\n")
				return
			}
			hz, err := strconv.ParseFloat(fields[2], 64)
			if err != nil || hz < 0 || hz > 50000 {
				d.writeRaw("Invalid rate (expected 0..50000)\r\n")
				return
			}
			d.logRateHz = hz
			d.logPaused = (hz == 0)
			d.writeRaw(fmt.Sprintf("Background log rate set to %.2f Hz\r\n", hz))
		default:
			d.writeRaw(fmt.Sprintf("log: unknown subcommand %q (try: pause, resume, rate)\r\n", fields[1]))
		}

	case "reboot":
		d.emitBootBannerLocked()

	default:
		d.writeRaw(fmt.Sprintf("%s: command not found\r\n", fields[0]))
	}
}
