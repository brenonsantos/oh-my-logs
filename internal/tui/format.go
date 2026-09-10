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

// CanonicalHexLine represents one formatted line in a canonical 16-byte hex dump.
type CanonicalHexLine struct {
	Offset string // e.g. "0000: "
	Hex    string // e.g. "5b 20 20 20 20 20 20 36  2e 32 34 31 5d 20 3c 69" (padded to 48 chars)
	ASCII  string // e.g. "[      6.241] <i"
}

// FormatCanonicalHexLines splits raw into 16-byte chunks and formats them as canonical lines.
func FormatCanonicalHexLines(raw string) []CanonicalHexLine {
	if len(raw) == 0 {
		return nil
	}
	b := []byte(raw)
	var lines []CanonicalHexLine
	for i := 0; i < len(b); i += 16 {
		chunkEnd := i + 16
		if chunkEnd > len(b) {
			chunkEnd = len(b)
		}
		chunk := b[i:chunkEnd]

		offsetStr := fmt.Sprintf("%04x: ", i)

		var hexSb strings.Builder
		for j := 0; j < len(chunk); j++ {
			if j > 0 {
				if j == 8 {
					hexSb.WriteString("  ")
				} else {
					hexSb.WriteByte(' ')
				}
			}
			fmt.Fprintf(&hexSb, "%02x", chunk[j])
		}
		hexStr := hexSb.String()
		if len(chunk) < 16 {
			padLen := 48 - len(hexStr)
			if padLen > 0 {
				hexStr += strings.Repeat(" ", padLen)
			}
		}

		asciiStr := FormatASCII(string(chunk))

		lines = append(lines, CanonicalHexLine{
			Offset: offsetStr,
			Hex:    hexStr,
			ASCII:  asciiStr,
		})
	}
	return lines
}

// CanonicalBinaryLine represents one formatted line in a canonical binary dump.
type CanonicalBinaryLine struct {
	Offset string // e.g. "0000: "
	Binary string // e.g. "01011011 00100000 00100000 00100000  00100000 00100000 00100000 00110110" (8 bytes)
	ASCII  string // e.g. "[      6" (8 chars)
}

// FormatCanonicalBinaryLines splits raw into 8-byte chunks and formats them as binary lines.
func FormatCanonicalBinaryLines(raw string) []CanonicalBinaryLine {
	if len(raw) == 0 {
		return nil
	}
	b := []byte(raw)
	var lines []CanonicalBinaryLine
	for i := 0; i < len(b); i += 8 {
		chunkEnd := i + 8
		if chunkEnd > len(b) {
			chunkEnd = len(b)
		}
		chunk := b[i:chunkEnd]

		offsetStr := fmt.Sprintf("%04x: ", i)

		var binSb strings.Builder
		for j := 0; j < len(chunk); j++ {
			if j > 0 {
				if j == 4 {
					binSb.WriteString("  ")
				} else {
					binSb.WriteByte(' ')
				}
			}
			fmt.Fprintf(&binSb, "%08b", chunk[j])
		}
		binStr := binSb.String()
		if len(chunk) < 8 {
			padLen := 72 - len(binStr)
			if padLen > 0 {
				binStr += strings.Repeat(" ", padLen)
			}
		}

		asciiStr := FormatASCII(string(chunk))

		lines = append(lines, CanonicalBinaryLine{
			Offset: offsetStr,
			Binary: binStr,
			ASCII:  asciiStr,
		})
	}
	return lines
}

