package serial

import (
	"strings"
	"testing"
	"time"
)

func TestPipeSource_ReadLines(t *testing.T) {
	input := "first line\r\nsecond line\nthird line"
	r := strings.NewReader(input)
	src := NewPipeSource(r)
	defer src.Stop()

	var lines []string
	timeout := time.After(2 * time.Second)
	done := false

	for !done {
		select {
		case line, ok := <-src.Lines():
			if !ok {
				done = true
				break
			}
			lines = append(lines, line)
		case err, ok := <-src.Errors():
			if ok && err != nil {
				t.Fatalf("unexpected error from pipe source: %v", err)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for pipe lines, got: %v", lines)
		}
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "first line" || lines[1] != "second line" || lines[2] != "third line" {
		t.Errorf("unexpected lines content: %v", lines)
	}
}

func TestPipeSource_WriteReturnsErrReadOnly(t *testing.T) {
	r := strings.NewReader("sample\n")
	src := NewPipeSource(r)
	defer src.Stop()

	_, err := src.Write([]byte("foo"))
	if err == nil {
		t.Fatalf("expected ErrReadOnlySource, got nil")
	}
}

func TestPipeSource_ResolvesVT100Sequences(t *testing.T) {
	input := "uart:~$ \x1b[8D\x1b[J[  1.000] <inf> net: up\r\nprogress 50%\r\x1b[Kdone\r\nhelo\b\bllo!\n"
	src := NewPipeSource(strings.NewReader(input))
	defer src.Stop()

	var lines []string
	for l := range src.Lines() {
		lines = append(lines, l)
	}
	want := []string{"[  1.000] <inf> net: up", "done", "hello!"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", lines, want)
	}
}
