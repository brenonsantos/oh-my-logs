package serial

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// LineEnding defines the line termination suffix to append to TX payloads.
type LineEnding int

const (
	EndingCRLF LineEnding = iota
	EndingLF
	EndingCR
	EndingNone
)

// String returns the user-visible acronym for the line ending.
func (e LineEnding) String() string {
	switch e {
	case EndingCRLF:
		return "CRLF"
	case EndingLF:
		return "LF"
	case EndingCR:
		return "CR"
	case EndingNone:
		return "None"
	default:
		return "CRLF"
	}
}

// Suffix returns the raw byte sequence for the line ending.
func (e LineEnding) Suffix() string {
	switch e {
	case EndingCRLF:
		return "\r\n"
	case EndingLF:
		return "\n"
	case EndingCR:
		return "\r"
	case EndingNone:
		return ""
	default:
		return "\r\n"
	}
}

// Next cycles to the next line ending option (CRLF -> LF -> CR -> None -> CRLF).
func (e LineEnding) Next() LineEnding {
	switch e {
	case EndingCRLF:
		return EndingLF
	case EndingLF:
		return EndingCR
	case EndingCR:
		return EndingNone
	case EndingNone:
		return EndingCRLF
	default:
		return EndingCRLF
	}
}

// ParseLineEnding parses a string into a LineEnding, defaulting to EndingCRLF.
func ParseLineEnding(s string) LineEnding {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "LF":
		return EndingLF
	case "CR":
		return EndingCR
	case "NONE", "RAW":
		return EndingNone
	default:
		return EndingCRLF
	}
}

// FormatTXPayload parses escape sequences (e.g. \x02, \r, \n, \t, \\) in raw input
// and appends the selected line ending suffix.
func FormatTXPayload(raw string, ending LineEnding) ([]byte, error) {
	var buf []byte
	n := len(raw)
	for i := 0; i < n; i++ {
		if raw[i] == '\\' {
			if i+1 >= n {
				return nil, fmt.Errorf("incomplete escape sequence '\\' at end of input")
			}
			next := raw[i+1]
			switch next {
			case 'r':
				buf = append(buf, '\r')
				i++
			case 'n':
				buf = append(buf, '\n')
				i++
			case 't':
				buf = append(buf, '\t')
				i++
			case '\\':
				buf = append(buf, '\\')
				i++
			case 'x', 'X':
				if i+3 >= n {
					return nil, fmt.Errorf("incomplete hex escape %q at position %d", raw[i:], i)
				}
				hexStr := raw[i+2 : i+4]
				b, err := hex.DecodeString(hexStr)
				if err != nil {
					return nil, fmt.Errorf("invalid hex escape \"\\x%s\": %w", hexStr, err)
				}
				buf = append(buf, b[0])
				i += 3
			default:
				// Literal backslash + character if not recognized as escape
				buf = append(buf, '\\', next)
				i++
			}
		} else {
			buf = append(buf, raw[i])
		}
	}

	buf = append(buf, []byte(ending.Suffix())...)
	return buf, nil
}
