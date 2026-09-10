package tui

import (
	"fmt"
	"strings"
)

// DisplayFormat defines how serial records are visually presented in the table.
type DisplayFormat int

const (
	// FormatParsed renders structured profile columns (timestamp, level, module, message).
	FormatParsed DisplayFormat = iota
	// FormatRaw renders the exact unparsed serial text.
	FormatRaw
	// FormatHex renders canonical two-character hex byte pairs with byte length and ASCII column.
	FormatHex
	// FormatBinary renders 8-bit binary strings with byte length and ASCII column.
	FormatBinary
)

func (f DisplayFormat) String() string {
	switch f {
	case FormatParsed:
		return "parsed"
	case FormatRaw:
		return "raw"
	case FormatHex:
		return "hex"
	case FormatBinary:
		return "binary"
	default:
		return "parsed"
	}
}

// Label returns a human-readable title for UI indicators and settings.
func (f DisplayFormat) Label() string {
	switch f {
	case FormatParsed:
		return "Parsed Columns"
	case FormatRaw:
		return "Raw Text"
	case FormatHex:
		return "Hex Dump"
	case FormatBinary:
		return "Binary Bits"
	default:
		return "Parsed Columns"
	}
}

// Tag returns a compact badge label for status bars and tabs.
func (f DisplayFormat) Tag() string {
	switch f {
	case FormatParsed:
		return "PARSED"
	case FormatRaw:
		return "RAW"
	case FormatHex:
		return "HEX"
	case FormatBinary:
		return "BIN"
	default:
		return "PARSED"
	}
}

// Next cycles sequentially to the next display format.
func (f DisplayFormat) Next() DisplayFormat {
	switch f {
	case FormatParsed:
		return FormatRaw
	case FormatRaw:
		return FormatHex
	case FormatHex:
		return FormatBinary
	case FormatBinary:
		return FormatParsed
	default:
		return FormatParsed
	}
}

// ParseDisplayFormat parses a string into a DisplayFormat.
func ParseDisplayFormat(s string) DisplayFormat {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "raw", "unparsed":
		return FormatRaw
	case "hex", "hexdump":
		return FormatHex
	case "binary", "bin", "bits":
		return FormatBinary
	default:
		return FormatParsed
	}
}

// FormatHexBytes converts raw string bytes to canonical two-digit lowercase hex representations.
// Groups into 8-byte blocks separated by double spaces for optimal readability.
func FormatHexBytes(raw string) string {
	if len(raw) == 0 {
		return ""
	}
	var sb strings.Builder
	// Allocate approximate capacity: 3 chars per byte + extra group spaces
	sb.Grow(len(raw)*3 + len(raw)/8)

	for i := 0; i < len(raw); i++ {
		if i > 0 {
			if i%8 == 0 {
				sb.WriteString("  ")
			} else {
				sb.WriteByte(' ')
			}
		}
		fmt.Fprintf(&sb, "%02x", raw[i])
	}
	return sb.String()
}

// FormatBinaryBits converts raw string bytes to 8-bit binary representation.
func FormatBinaryBits(raw string) string {
	if len(raw) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(raw) * 9)

	for i := 0; i < len(raw); i++ {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%08b", raw[i])
	}
	return sb.String()
}

// FormatASCII produces a terminal-safe printable ASCII representation where non-printable
// characters (< 32 or >= 127) are replaced by a dot ('.').
func FormatASCII(raw string) string {
	if len(raw) == 0 {
		return ""
	}
	b := []byte(raw)
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 32 && c < 127 {
			out[i] = c
		} else {
			out[i] = '.'
		}
	}
	return string(out)
}

// FormatByteLen formats byte length cleanly (e.g. "12B", "1.4KB").
func FormatByteLen(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1fKB", float64(n)/1024.0)
}
