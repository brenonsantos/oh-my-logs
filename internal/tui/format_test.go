package tui

import (
	"testing"
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
