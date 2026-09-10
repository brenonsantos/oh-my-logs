package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDisplayFormatCycleAndParse(t *testing.T) {
	f := FormatParsed
	if f.String() != "parsed" || f.Label() != "Parsed Columns" || f.Tag() != "PARSED" {
		t.Fatalf("unexpected properties for FormatParsed: %s, %s, %s", f.String(), f.Label(), f.Tag())
	}

	f = f.Next()
	if f != FormatRaw || f.String() != "raw" || f.Tag() != "RAW" {
		t.Fatalf("expected FormatRaw, got %v", f)
	}

	f = f.Next()
	if f != FormatHex || f.String() != "hex" || f.Tag() != "HEX" {
		t.Fatalf("expected FormatHex, got %v", f)
	}

	f = f.Next()
	if f != FormatBinary || f.String() != "binary" || f.Tag() != "BIN" {
		t.Fatalf("expected FormatBinary, got %v", f)
	}

	f = f.Next()
	if f != FormatParsed {
		t.Fatalf("expected FormatParsed after cycling, got %v", f)
	}

	// Parsing
	if ParseDisplayFormat("raw") != FormatRaw || ParseDisplayFormat("RAW") != FormatRaw {
		t.Errorf("failed parsing raw")
	}
	if ParseDisplayFormat("hex") != FormatHex || ParseDisplayFormat("HEXDUMP") != FormatHex {
		t.Errorf("failed parsing hex")
	}
	if ParseDisplayFormat("binary") != FormatBinary || ParseDisplayFormat("bits") != FormatBinary {
		t.Errorf("failed parsing binary")
	}
	if ParseDisplayFormat("unknown") != FormatParsed {
		t.Errorf("failed fallback parsing")
	}
}

func TestFormatHexBytes(t *testing.T) {
	if got := FormatHexBytes(""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}

	input := "Hello World!"
	// H=48, e=65, l=6c, l=6c, o=6f, ' '=20, W=57, o=6f -> 8 bytes
	// r=72, l=6c, d=64, !=21
	expected := "48 65 6c 6c 6f 20 57 6f  72 6c 64 21"
	got := FormatHexBytes(input)
	if got != expected {
		t.Errorf("expected hex %q, got %q", expected, got)
	}
}

func TestFormatBinaryBits(t *testing.T) {
	if got := FormatBinaryBits(""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}

	input := "AB"
	// A=65=01000001, B=66=01000010
	expected := "01000001 01000010"
	got := FormatBinaryBits(input)
	if got != expected {
		t.Errorf("expected binary %q, got %q", expected, got)
	}
}

func TestFormatASCII(t *testing.T) {
	if got := FormatASCII(""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}

	input := "Hello\x00\x0a\x1bWorld\x7f!"
	expected := "Hello...World.!"
	got := FormatASCII(input)
	if got != expected {
		t.Errorf("expected ASCII %q, got %q", expected, got)
	}
}

func TestFormatByteLen(t *testing.T) {
	if got := FormatByteLen(0); got != "0B" {
		t.Errorf("expected '0B', got %s", got)
	}
	if got := FormatByteLen(512); got != "512B" {
		t.Errorf("expected '512B', got %s", got)
	}
	if got := FormatByteLen(2048); got != "2.0KB" {
		t.Errorf("expected '2.0KB', got %s", got)
	}
}

func TestFormatCanonicalHexLines(t *testing.T) {
	if got := FormatCanonicalHexLines(""); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}

	// 20 bytes: should produce 2 lines (16 bytes on line 0, 4 bytes on line 1)
	input := "1234567890abcdefghij"
	lines := FormatCanonicalHexLines(input)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Line 0
	if lines[0].Offset != "0000: " {
		t.Errorf("expected offset '0000: ', got %q", lines[0].Offset)
	}
	if len(lines[0].Hex) != 48 {
		t.Errorf("expected padded hex string length 48, got %d (%q)", len(lines[0].Hex), lines[0].Hex)
	}
	if lines[0].ASCII != "1234567890abcdef" {
		t.Errorf("expected ASCII '1234567890abcdef', got %q", lines[0].ASCII)
	}

	// Line 1: 4 bytes (ghij), offset 0010:
	if lines[1].Offset != "0010: " {
		t.Errorf("expected offset '0010: ', got %q", lines[1].Offset)
	}
	if len(lines[1].Hex) != 48 {
		t.Errorf("expected padded hex string length 48, got %d (%q)", len(lines[1].Hex), lines[1].Hex)
	}
	if lines[1].ASCII != "ghij" {
		t.Errorf("expected ASCII 'ghij', got %q", lines[1].ASCII)
	}
}

func TestFormatCanonicalBinaryLines(t *testing.T) {
	if got := FormatCanonicalBinaryLines(""); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}

	// 10 bytes: should produce 2 lines (8 bytes on line 0, 2 bytes on line 1)
	input := "0123456789"
	lines := FormatCanonicalBinaryLines(input)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if lines[0].Offset != "0000: " {
		t.Errorf("expected offset '0000: ', got %q", lines[0].Offset)
	}
	if len(lines[0].Binary) != 72 {
		t.Errorf("expected padded binary string length 72, got %d (%q)", len(lines[0].Binary), lines[0].Binary)
	}
	if lines[0].ASCII != "01234567" {
		t.Errorf("expected ASCII '01234567', got %q", lines[0].ASCII)
	}

	if lines[1].Offset != "0008: " {
		t.Errorf("expected offset '0008: ', got %q", lines[1].Offset)
	}
	if len(lines[1].Binary) != 72 {
		t.Errorf("expected padded binary string length 72, got %d (%q)", len(lines[1].Binary), lines[1].Binary)
	}
	if lines[1].ASCII != "89" {
		t.Errorf("expected ASCII '89', got %q", lines[1].ASCII)
	}
}

func TestAnsiCutBehavior(t *testing.T) {
	styled := theme.Accent.Render("0123456789ABCDEF")
	cut := ansiCut(styled, 4, 8)
	t.Logf("original=%q cut=%q width=%d", styled, cut, lipgloss.Width(cut))
	if lipgloss.Width(cut) != 8 {
		t.Errorf("expected width 8, got %d", lipgloss.Width(cut))
	}
}
