package serial

import (
	"strings"
	"unicode/utf8"
)

// LineDiscipline is a streaming, single-row VT100/ANSI line editor.
//
// It replaces timer- and regex-based chunk heuristics with an explicit model of
// what a terminal would display on its current row:
//
//   - Printable runes are written at the cursor, overwriting existing cells.
//   - '\r' moves the cursor to column 0 (it never discards text).
//   - '\b' and ESC[nD move left, ESC[nC / ESC[nG / ESC[n;mH move right/absolute.
//   - ESC[K / ESC[J (and the 1/2 variants), ESC[nP, ESC[n@ and ESC[nX edit the row.
//   - '\n' commits the row as a log record and starts a new, empty row.
//   - SGR colour sequences are preserved (attached to the next written cell).
//   - All other escape sequences (OSC, private modes, cursor rows, ...) are consumed.
//
// Because the device prompt ("uart:~$ ") is simply the content of the row that
// has not been newline-terminated yet, no timeout is required to tell an
// in-flight log line from an idle prompt: both stay in the row buffer. An
// asynchronous log that erases the prompt (ESC[9D ESC[J ...) edits the same row
// before it is committed, so the prompt text never reaches the log stream.
//
// Prompt state is surfaced as non-committing EventPrompt snapshots at the end
// of each Feed call, only when the pending row looks like an idle shell prompt.
//
// A LineDiscipline is not safe for concurrent use.
type LineDiscipline struct {
	state vtState
	cells []cell
	col   int

	// pendingSGR holds colour sequences seen since the last written cell.
	pendingSGR string

	// CSI parser state.
	params   [8]int
	nparams  int
	curParam int
	hasParam bool
	ignore   bool // private / unsupported-intermediate sequence: swallow
	csiRaw   []byte

	// UTF-8 reassembly across byte/chunk boundaries.
	utf    [utf8.UTFMax]byte
	utfLen int

	maxCells   int
	lastPrompt string
	out        func(Event)
}

// EventKind classifies an Event emitted by LineDiscipline.
type EventKind uint8

const (
	// EventLog is a committed (newline-terminated or flushed) log row.
	EventLog EventKind = iota
	// EventPrompt is a non-committing snapshot of the pending row that looks
	// like an idle interactive shell prompt (possibly with partial user input).
	EventPrompt
)

// Event is a unit of output produced by LineDiscipline.
type Event struct {
	Kind EventKind
	Text string
}

type vtState uint8

const (
	vtGround vtState = iota
	vtEscape
	vtCSI
	vtOSC
	vtOSCEsc
	vtEscInter
)

type cell struct {
	r   rune
	sgr string // SGR sequence(s) emitted immediately before this rune
}

const (
	// DefaultMaxLineCells bounds the row length; reaching it force-commits the
	// row (the equivalent of terminal auto-wrap) so corrupt streams cannot grow
	// memory without bound.
	DefaultMaxLineCells = 16384
	// promptSnapshotMaxCells bounds the cost of per-chunk prompt detection.
	promptSnapshotMaxCells = 256
	maxCSIParam            = 65535
	tabWidth               = 8
)

// NewLineDiscipline returns a ready-to-use LineDiscipline.
func NewLineDiscipline() *LineDiscipline {
	return &LineDiscipline{maxCells: DefaultMaxLineCells, cells: make([]cell, 0, 256)}
}

// Feed processes p and calls out for every committed row and, at the end of p,
// for an idle-prompt snapshot if one is pending and has changed.
func (d *LineDiscipline) Feed(p []byte, out func(Event)) {
	d.out = out
	for _, b := range p {
		d.feedByte(b)
	}
	d.snapshotPrompt()
	d.out = nil
}

// Flush commits any pending non-blank row (call on EOF / disconnect).
func (d *LineDiscipline) Flush(out func(Event)) {
	d.out = out
	d.state = vtGround
	d.utfLen = 0
	// A row already reported as an idle prompt is shell state, not a log record.
	if d.lastPrompt == "" || d.render() != d.lastPrompt {
		d.commit()
	}
	d.cells, d.col, d.pendingSGR, d.lastPrompt = d.cells[:0], 0, "", ""
	d.out = nil
}

// Pending renders the current uncommitted row.
func (d *LineDiscipline) Pending() string { return d.render() }

// Reset discards all state.
func (d *LineDiscipline) Reset() {
	d.state = vtGround
	d.cells = d.cells[:0]
	d.col = 0
	d.pendingSGR = ""
	d.utfLen = 0
	d.lastPrompt = ""
}

func (d *LineDiscipline) feedByte(b byte) {
	switch d.state {
	case vtGround:
		d.ground(b)
	case vtEscape:
		switch {
		case b == '[':
			d.state = vtCSI
			d.nparams, d.curParam, d.hasParam, d.ignore = 0, 0, false, false
			d.csiRaw = d.csiRaw[:0]
		case b == ']':
			d.state = vtOSC
		case b >= 0x20 && b <= 0x2F:
			d.state = vtEscInter
		case b == 0x1b:
			// stay in escape
		default:
			// Two-byte escape (ESC c, ESC 7, ESC 8, ESC M, ...): consumed.
			d.state = vtGround
		}
	case vtEscInter:
		if b < 0x20 || b > 0x2F {
			d.state = vtGround
		}
	case vtOSC:
		switch b {
		case 0x07:
			d.state = vtGround
		case 0x1b:
			d.state = vtOSCEsc
		}
	case vtOSCEsc:
		if b == '\\' {
			d.state = vtGround
		} else {
			d.state = vtOSC
		}
	case vtCSI:
		d.csi(b)
	}
}

func (d *LineDiscipline) ground(b byte) {
	if d.utfLen > 0 || b >= 0x80 {
		d.utf[d.utfLen] = b
		d.utfLen++
		if !utf8.FullRune(d.utf[:d.utfLen]) {
			return
		}
		r, size := utf8.DecodeRune(d.utf[:d.utfLen])
		if r == utf8.RuneError && size <= 1 {
			// Invalid sequence: emit one replacement, re-feed trailing bytes.
			rest := append([]byte(nil), d.utf[1:d.utfLen]...)
			d.utfLen = 0
			d.putRune(utf8.RuneError)
			for _, rb := range rest {
				d.ground(rb)
			}
			return
		}
		d.utfLen = 0
		d.putRune(r)
		return
	}
	switch {
	case b == 0x1b:
		d.state = vtEscape
	case b == '\n':
		d.commit()
	case b == '\r':
		d.col = 0
	case b == '\b':
		if d.col > 0 {
			d.col--
		}
	case b == '\t':
		d.col += tabWidth - d.col%tabWidth
		if d.col >= d.maxCells {
			d.col = d.maxCells - 1
		}
	case b < 0x20 || b == 0x7f:
		// BEL, DEL, other C0 controls: no visible effect.
	default:
		d.putRune(rune(b))
	}
}

func (d *LineDiscipline) csi(b byte) {
	switch {
	case b >= '0' && b <= '9':
		if d.curParam < maxCSIParam {
			d.curParam = d.curParam*10 + int(b-'0')
			if d.curParam > maxCSIParam {
				d.curParam = maxCSIParam
			}
		}
		d.hasParam = true
		d.csiRaw = append(d.csiRaw, b)
	case b == ';' || b == ':':
		d.pushParam()
		d.csiRaw = append(d.csiRaw, b)
	case b >= 0x3C && b <= 0x3F, b >= 0x20 && b <= 0x2F:
		d.ignore = true // private marker or intermediate byte
		d.csiRaw = append(d.csiRaw, b)
	case b >= 0x40 && b <= 0x7E:
		d.pushParam()
		d.state = vtGround
		if !d.ignore {
			d.dispatchCSI(b)
		}
	case b == 0x1b:
		d.state = vtEscape
	default:
		// C0 control inside CSI: abort the sequence.
		d.state = vtGround
	}
}

func (d *LineDiscipline) pushParam() {
	if d.nparams < len(d.params) {
		d.params[d.nparams] = d.curParam
		d.nparams++
	}
	d.curParam, d.hasParam = 0, false
}

// param returns parameter i, or def when absent or zero.
func (d *LineDiscipline) param(i, def int) int {
	if i < d.nparams && d.params[i] > 0 {
		return d.params[i]
	}
	return def
}

func (d *LineDiscipline) dispatchCSI(final byte) {
	switch final {
	case 'D':
		d.col -= d.param(0, 1)
		if d.col < 0 {
			d.col = 0
		}
	case 'C':
		d.setCol(d.col + d.param(0, 1))
	case 'G':
		d.setCol(d.param(0, 1) - 1)
	case 'H', 'f':
		d.setCol(d.param(1, 1) - 1) // row parameter ignored: single-row model
	case 'K', 'J':
		mode := 0
		if d.nparams > 0 {
			mode = d.params[0]
		}
		switch mode {
		case 0:
			d.eraseToEnd()
		case 1:
			d.blank(0, d.col+1)
		default: // 2 (line / screen), 3 (scrollback)
			d.cells = d.cells[:0]
		}
	case 'P':
		n := d.param(0, 1)
		if d.col < len(d.cells) {
			if d.col+n >= len(d.cells) {
				d.cells = d.cells[:d.col]
			} else {
				d.cells = append(d.cells[:d.col], d.cells[d.col+n:]...)
			}
		}
	case '@':
		if d.col < len(d.cells) {
			n := d.param(0, 1)
			if len(d.cells)+n > d.maxCells {
				n = d.maxCells - len(d.cells)
			}
			if n > 0 {
				ins := make([]cell, n)
				for i := range ins {
					ins[i].r = ' '
				}
				d.cells = append(d.cells[:d.col], append(ins, d.cells[d.col:]...)...)
			}
		}
	case 'X':
		d.blank(d.col, d.col+d.param(0, 1))
	case 'm':
		d.pendingSGR += "\x1b[" + string(d.csiRaw) + "m"
	}
}

func (d *LineDiscipline) setCol(c int) {
	if c < 0 {
		c = 0
	}
	if c >= d.maxCells {
		c = d.maxCells - 1
	}
	d.col = c
}

func (d *LineDiscipline) eraseToEnd() {
	if d.col < len(d.cells) {
		d.cells = d.cells[:d.col]
	}
}

// blank overwrites cells in [from,to) with spaces (existing cells only).
func (d *LineDiscipline) blank(from, to int) {
	if to > len(d.cells) {
		to = len(d.cells)
	}
	for i := from; i < to; i++ {
		d.cells[i] = cell{r: ' '}
	}
}

func (d *LineDiscipline) putRune(r rune) {
	if d.col >= d.maxCells {
		d.commit() // auto-wrap failsafe
	}
	for len(d.cells) < d.col {
		d.cells = append(d.cells, cell{r: ' '})
	}
	c := cell{r: r, sgr: d.pendingSGR}
	d.pendingSGR = ""
	if d.col < len(d.cells) {
		d.cells[d.col] = c
	} else {
		d.cells = append(d.cells, c)
	}
	d.col++
}

// render returns the row as a string, with SGR sequences re-inserted.
func (d *LineDiscipline) render() string {
	var sb strings.Builder
	sb.Grow(len(d.cells) + len(d.pendingSGR))
	for _, c := range d.cells {
		sb.WriteString(c.sgr)
		sb.WriteRune(c.r)
	}
	// Trailing SGR (e.g. reset code) belongs to the line, but only when the
	// row has visible content, so colour resets never create phantom rows.
	if len(d.cells) > 0 {
		sb.WriteString(d.pendingSGR)
	}
	return sb.String()
}

func (d *LineDiscipline) blankRow() bool {
	for _, c := range d.cells {
		if c.r != ' ' {
			return false
		}
	}
	return true
}

func (d *LineDiscipline) commit() {
	blank := d.blankRow()
	var text string
	if !blank {
		text = d.render()
	}
	d.cells = d.cells[:0]
	d.col = 0
	d.pendingSGR = ""
	d.lastPrompt = ""
	if !blank && d.out != nil {
		d.out(Event{Kind: EventLog, Text: text})
	}
}

// snapshotPrompt emits a non-committing EventPrompt when the pending row is an
// idle shell prompt that differs from the last one reported.
func (d *LineDiscipline) snapshotPrompt() {
	if d.state != vtGround || d.utfLen != 0 || len(d.cells) == 0 || len(d.cells) > promptSnapshotMaxCells || d.out == nil {
		return
	}
	text := d.render()
	if text == d.lastPrompt || !IsIdleShellPrompt(text) {
		return
	}
	d.lastPrompt = text
	d.out(Event{Kind: EventPrompt, Text: text})
}

// RenderVT applies VT100 line-editing semantics to a single raw row (which may
// contain \r, \b and ESC sequences) and returns what a terminal would show.
// Embedded '\n' bytes are ignored: this is a single-row API.
func RenderVT(s string) string {
	d := NewLineDiscipline()
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			d.feedByte(s[i])
		}
	}
	return d.render()
}

// PromptSnapshotPrefix marks a Source line as a non-committing prompt snapshot
// (shell state) rather than a log record. It cannot occur in terminal output:
// NUL bytes are discarded by LineDiscipline.
const PromptSnapshotPrefix = "\x00oml:prompt\x00"

// MarkPromptSnapshot tags text as a prompt snapshot for transport over a Source.
func MarkPromptSnapshot(text string) string { return PromptSnapshotPrefix + text }

// SplitPromptSnapshot reports whether line is a prompt snapshot and returns its text.
func SplitPromptSnapshot(line string) (string, bool) {
	return strings.CutPrefix(line, PromptSnapshotPrefix)
}
