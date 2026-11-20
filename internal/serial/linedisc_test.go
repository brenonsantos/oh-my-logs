package serial_test

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func feedAll(chunks ...string) (logs, prompts []string, ld *serial.LineDiscipline) {
	ld = serial.NewLineDiscipline()
	emit := func(e serial.Event) {
		if e.Kind == serial.EventLog {
			logs = append(logs, e.Text)
		} else {
			prompts = append(prompts, e.Text)
		}
	}
	for _, c := range chunks {
		ld.Feed([]byte(c), emit)
	}
	return
}

func TestLineDiscipline_Logs(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{"simple crlf", []string{"hello\r\nworld\r\n"}, []string{"hello", "world"}},
		{"lf only", []string{"a\nb\n"}, []string{"a", "b"}},
		{"split across chunks", []string{"[  1.000] <inf> mod: Hel", "lo wor", "ld\r\n"}, []string{"[  1.000] <inf> mod: Hello world"}},
		{"short legit lines kept", []string{"OK\r\nACK\r\n"}, []string{"OK", "ACK"}},
		{"empty lines dropped", []string{"\r\n\r\nx\r\n\r\n"}, []string{"x"}},
		{"progress bar overwrite", []string{"10%\r20%\r30%\r\n"}, []string{"30%"}},
		{"cr overwrite keeps tail", []string{"longtext\rab\r\n"}, []string{"abngtext"}},
		{"cr then erase", []string{"progress 50%\r\x1b[Kdone\r\n"}, []string{"done"}},
		{"backspace edit", []string{"helo\b\bllo!\r\n"}, []string{"hello!"}},
		{"del ignored", []string{"ab\x7fc\r\n"}, []string{"abc"}},
		{"prompt erase then log", []string{"device:~$ \x1b[10D\x1b[J[  2.5] <inf> net: up\r\ndevice:~$ "}, []string{"[  2.5] <inf> net: up"}},
		{"prompt erase with typed input", []string{"uart:~$ os_msg\x1b[14D\x1b[J[  3.0] <wrn> x: y\r\n"}, []string{"[  3.0] <wrn> x: y"}},
		{"prompt erase via cr+K", []string{"uart:~$ ab\r\x1b[K[  3.1] <err> x: z\r\n"}, []string{"[  3.1] <err> x: z"}},
		{"erase split mid-escape", []string{"uart:~$ \x1b[", "8D\x1b[", "J[ 1] <inf> a: b\r\n"}, []string{"[ 1] <inf> a: b"}},
		{"sgr preserved", []string{"\x1b[32mgreen\x1b[0m\r\n"}, []string{"\x1b[32mgreen\x1b[0m"}},
		{"private mode and osc swallowed", []string{"\x1b[?25l\x1b]0;title\x07text\x1b[?25h\r\n"}, []string{"text"}},
		{"cursor forward pads", []string{"a\x1b[3Cb\r\n"}, []string{"a   b"}},
		{"tab stops", []string{"ab\tc\r\n"}, []string{"ab      c"}},
		{"erase to start", []string{"abcdef\x1b[3D\x1b[1Kxy\r\n"}, []string{"   xyf"[0:0] + "   xyf"}},
		{"delete chars", []string{"abcdef\x1b[4D\x1b[2P\r\n"}, []string{"abef"}},
		{"utf8 split across chunks", []string{"caf\xc3", "\xa9\r\n"}, []string{"café"}},
		{"invalid utf8", []string{"a\xffb\r\n"}, []string{"a\ufffdb"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs, _, _ := feedAll(tc.chunks...)
			if strings.Join(logs, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("logs = %q, want %q", logs, tc.want)
			}
		})
	}
}

func TestLineDiscipline_PromptSnapshots(t *testing.T) {
	logs, prompts, ld := feedAll("device:~$ ")
	if len(logs) != 0 || len(prompts) != 1 || prompts[0] != "device:~$ " {
		t.Fatalf("logs=%q prompts=%q", logs, prompts)
	}
	// Typing echo updates the snapshot but never commits a log row.
	_, prompts, _ = feedAll("uart:~$ ", "h", "e", "l")
	if len(prompts) != 4 || prompts[3] != "uart:~$ hel" {
		t.Fatalf("prompts=%q", prompts)
	}
	// Same prompt is not re-reported.
	var n int
	ld.Feed(nil, func(serial.Event) { n++ })
	if n != 0 {
		t.Fatalf("duplicate snapshot emitted")
	}
	// An in-flight log line is never reported as a prompt.
	_, prompts, _ = feedAll("[  1.0] <inf> mod: partial mess")
	if len(prompts) != 0 {
		t.Fatalf("in-flight log reported as prompt: %q", prompts)
	}
}

func TestLineDiscipline_FlushAndBound(t *testing.T) {
	logs, _, ld := feedAll("no newline at eof")
	if len(logs) != 0 {
		t.Fatal("must not commit without newline")
	}
	ld.Flush(func(e serial.Event) { logs = append(logs, e.Text) })
	if len(logs) != 1 || logs[0] != "no newline at eof" {
		t.Fatalf("flush: %q", logs)
	}

	big := strings.Repeat("x", serial.DefaultMaxLineCells*2+5)
	logs, _, _ = feedAll(big)
	if len(logs) != 2 || len(logs[0]) != serial.DefaultMaxLineCells {
		t.Fatalf("bound: got %d rows", len(logs))
	}
}

func TestRenderVT(t *testing.T) {
	if got := serial.RenderVT("old\rnew"); got != "new" {
		t.Fatalf("got %q", got)
	}
	if got := serial.RenderVT("longtext\rab"); got != "abngtext" {
		t.Fatalf("got %q", got)
	}
}

func TestLineDiscipline_PromptSnapshotTracksCursorCol(t *testing.T) {
	ld := serial.NewLineDiscipline()
	var events []serial.Event
	emit := func(e serial.Event) {
		if e.Kind == serial.EventPrompt {
			events = append(events, e)
		}
	}

	ld.Feed([]byte("uart:~$ help"), emit)
	ld.Feed([]byte("\x1b[2D"), emit) // cursor moves left 2 (col 12 -> col 10) without changing text
	if len(events) != 2 {
		t.Fatalf("expected 2 prompt snapshots (initial + cursor move), got %d: %+v", len(events), events)
	}
	if events[0].Col != 12 || events[1].Col != 10 {
		t.Fatalf("unexpected cols: %+v", events)
	}

	tagged := serial.MarkPromptSnapshotWithCol(events[1].Text, events[1].Col)
	text, col, ok := serial.SplitPromptSnapshotWithCol(tagged)
	if !ok || text != "uart:~$ help" || col != 10 {
		t.Fatalf("SplitPromptSnapshotWithCol = (%q, %d, %v)", text, col, ok)
	}
	if plain, ok := serial.SplitPromptSnapshot(tagged); !ok || plain != "uart:~$ help" {
		t.Fatalf("SplitPromptSnapshot = (%q, %v)", plain, ok)
	}
}

func BenchmarkLineDiscipline(b *testing.B) {
	data := []byte(strings.Repeat("[  262.937] <inf> lib-mqtt: Connected to broker\r\n", 200))
	ld := serial.NewLineDiscipline()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ld.Feed(data, func(serial.Event) {})
	}
}
