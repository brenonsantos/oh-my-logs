package serial

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
)

func TestLineEndingHelpers(t *testing.T) {
	tests := []struct {
		ending LineEnding
		name   string
		suffix string
		next   LineEnding
	}{
		{EndingCRLF, "CRLF", "\r\n", EndingLF},
		{EndingLF, "LF", "\n", EndingCR},
		{EndingCR, "CR", "\r", EndingNone},
		{EndingNone, "None", "", EndingCRLF},
	}

	for _, tc := range tests {
		if tc.ending.String() != tc.name {
			t.Errorf("expected %s, got %s", tc.name, tc.ending.String())
		}
		if tc.ending.Suffix() != tc.suffix {
			t.Errorf("expected %q, got %q", tc.suffix, tc.ending.Suffix())
		}
		if tc.ending.Next() != tc.next {
			t.Errorf("expected next %v, got %v", tc.next, tc.ending.Next())
		}
	}

	if ParseLineEnding("lf") != EndingLF {
		t.Errorf("expected EndingLF")
	}
	if ParseLineEnding("CR") != EndingCR {
		t.Errorf("expected EndingCR")
	}
	if ParseLineEnding("none") != EndingNone {
		t.Errorf("expected EndingNone")
	}
	if ParseLineEnding("raw") != EndingNone {
		t.Errorf("expected EndingNone for raw")
	}
	if ParseLineEnding("unknown") != EndingCRLF {
		t.Errorf("expected EndingCRLF fallback")
	}
}

func TestFormatTXPayload(t *testing.T) {
	// 1. Basic text with CRLF
	payload, err := FormatTXPayload("reboot", EndingCRLF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload) != "reboot\r\n" {
		t.Errorf("expected reboot\\r\\n, got %q", string(payload))
	}

	// 2. LF ending
	payload, err = FormatTXPayload("AT+GMR", EndingLF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload) != "AT+GMR\n" {
		t.Errorf("expected AT+GMR\\n, got %q", string(payload))
	}

	// 3. None / Raw
	payload, err = FormatTXPayload("RAW_PAYLOAD", EndingNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload) != "RAW_PAYLOAD" {
		t.Errorf("expected RAW_PAYLOAD, got %q", string(payload))
	}

	// 4. Hex escape sequences (\x02 \x55 \xAA \x03)
	payload, err = FormatTXPayload(`\x02PING\x03`, EndingNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{0x02, 'P', 'I', 'N', 'G', 0x03}
	if !bytes.Equal(payload, expected) {
		t.Errorf("expected %v, got %v", expected, payload)
	}

	// 5. Common escapes: \r, \n, \t, \\
	payload, err = FormatTXPayload(`line1\nline2\t\r\\`, EndingNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedEsc := []byte("line1\nline2\t\r\\")
	if !bytes.Equal(payload, expectedEsc) {
		t.Errorf("expected %q, got %q", expectedEsc, payload)
	}

	// 6. Invalid hex escape
	_, err = FormatTXPayload(`\xGG`, EndingNone)
	if err == nil {
		t.Errorf("expected error for invalid hex escape \\xGG")
	}

	// 7. Incomplete hex escape
	_, err = FormatTXPayload(`\xA`, EndingNone)
	if err == nil {
		t.Errorf("expected error for incomplete hex escape \\xA")
	}

	// 8. Trailing slash
	_, err = FormatTXPayload(`test\`, EndingNone)
	if err == nil {
		t.Errorf("expected error for trailing backslash")
	}
}

func TestFileSourceWriteReadOnly(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "dummy.log")
	src, err := NewFileSource(logFile)
	if err == nil {
		src.Stop()
	}

	// FileSource returns ErrReadOnlySource
	fs := &FileSource{}
	_, err = fs.Write([]byte("test"))
	if err == nil || !errors.Is(err, ErrReadOnlySource) {
		t.Errorf("expected ErrReadOnlySource, got %v", err)
	}
}
