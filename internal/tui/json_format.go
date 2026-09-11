package tui

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// JSONDetection contains the results of inspecting a string for JSON payloads.
type JSONDetection struct {
	HasJSON      bool
	Prefix       string
	RawJSON      string
	Suffix       string
	IndentedJSON string
}

// DetectAndFormatJSON inspects a string to see if it is a JSON object/array
// or contains an embedded JSON object/array. If valid JSON is found, it formats
// and indents it with 2 spaces.
func DetectAndFormatJSON(s string) JSONDetection {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 {
		return JSONDetection{HasJSON: false}
	}

	// 1. Direct JSON (object or array)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		if json.Valid([]byte(trimmed)) {
			var out bytes.Buffer
			if err := json.Indent(&out, []byte(trimmed), "", "  "); err == nil {
				return JSONDetection{
					HasJSON:      true,
					RawJSON:      trimmed,
					IndentedJSON: out.String(),
				}
			}
		}
	}

	// 2. Embedded JSON in a prefixed log line (e.g. "12:00:00 [INF] payload: {\"status\":\"ok\"}")
	// Search for any candidate '{' or '['
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '{' && ch != '[' {
			continue
		}
		closeChar := byte('}')
		if ch == '[' {
			closeChar = ']'
		}
		// Try from the last occurrence of closeChar backwards down to i
		lastIdx := strings.LastIndexByte(s, closeChar)
		for lastIdx > i {
			candidate := s[i : lastIdx+1]
			if json.Valid([]byte(candidate)) {
				var out bytes.Buffer
				if err := json.Indent(&out, []byte(candidate), "", "  "); err == nil {
					return JSONDetection{
						HasJSON:      true,
						Prefix:       strings.TrimSpace(s[:i]),
						RawJSON:      candidate,
						Suffix:       strings.TrimSpace(s[lastIdx+1:]),
						IndentedJSON: out.String(),
					}
				}
			}
			lastIdx = strings.LastIndexByte(s[:lastIdx], closeChar)
		}
	}

	return JSONDetection{HasJSON: false}
}

// ColorizeJSON applies theme colors to an indented JSON string.
// Keys -> Cyan/Accent, Strings -> Green, Numbers -> Yellow, Booleans/Null -> Purple, Delimiters -> Muted.
func ColorizeJSON(indentedJSON string, p Palette) string {
	styleKey := lipgloss.NewStyle().Foreground(p.Cyan).Bold(true)
	styleString := lipgloss.NewStyle().Foreground(p.Green)
	styleNumber := lipgloss.NewStyle().Foreground(p.Yellow)
	styleBoolNull := lipgloss.NewStyle().Foreground(p.Purple)
	styleBrace := lipgloss.NewStyle().Foreground(p.Accent)
	stylePunct := lipgloss.NewStyle().Foreground(p.Muted)

	var sb strings.Builder
	runes := []rune(indentedJSON)
	n := len(runes)
	i := 0

	for i < n {
		ch := runes[i]

		// Whitespace: preserve directly
		if unicode.IsSpace(ch) {
			sb.WriteRune(ch)
			i++
			continue
		}

		// String token
		if ch == '"' {
			start := i
			i++ // skip opening quote
			for i < n {
				if runes[i] == '\\' && i+1 < n {
					i += 2 // skip escaped character
					continue
				}
				if runes[i] == '"' {
					i++ // include closing quote
					break
				}
				i++
			}
			strLiteral := string(runes[start:i])

			// Lookahead for colon (is this an object key?)
			lookahead := i
			for lookahead < n && unicode.IsSpace(runes[lookahead]) {
				lookahead++
			}
			if lookahead < n && runes[lookahead] == ':' {
				sb.WriteString(styleKey.Render(strLiteral))
			} else {
				sb.WriteString(styleString.Render(strLiteral))
			}
			continue
		}

		// Braces and Brackets
		if ch == '{' || ch == '}' || ch == '[' || ch == ']' {
			sb.WriteString(styleBrace.Render(string(ch)))
			i++
			continue
		}

		// Punctuation (colons, commas)
		if ch == ':' || ch == ',' {
			sb.WriteString(stylePunct.Render(string(ch)))
			i++
			continue
		}

		// Numbers (including negative and exponential)
		if unicode.IsDigit(ch) || ch == '-' {
			start := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.' || runes[i] == 'e' || runes[i] == 'E' || runes[i] == '+' || runes[i] == '-') {
				i++
			}
			numStr := string(runes[start:i])
			sb.WriteString(styleNumber.Render(numStr))
			continue
		}

		// Identifiers: true, false, null
		if unicode.IsLetter(ch) {
			start := i
			for i < n && unicode.IsLetter(runes[i]) {
				i++
			}
			ident := string(runes[start:i])
			if ident == "true" || ident == "false" || ident == "null" {
				sb.WriteString(styleBoolNull.Render(ident))
			} else {
				sb.WriteString(ident)
			}
			continue
		}

		// Fallback for any other characters
		sb.WriteRune(ch)
		i++
	}

	return sb.String()
}

// wrapTextLines breaks multi-line text cleanly across maxWidth boundaries,
// respecting words when possible.
func wrapTextLines(text string, maxWidth int) []string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	var result []string
	rawLines := strings.Split(text, "\n")

	for _, line := range rawLines {
		line = strings.TrimRight(line, "\r")
		if len([]rune(line)) <= maxWidth {
			result = append(result, line)
			continue
		}

		remaining := line
		for len([]rune(remaining)) > maxWidth {
			runes := []rune(remaining)
			// Look for the last space within maxWidth
			breakIdx := -1
			for j := maxWidth; j >= 0; j-- {
				if j < len(runes) && unicode.IsSpace(runes[j]) {
					breakIdx = j
					break
				}
			}

			if breakIdx <= 0 {
				// No space found; hard break at maxWidth
				breakIdx = maxWidth
			}

			result = append(result, string(runes[:breakIdx]))
			remaining = strings.TrimLeft(string(runes[breakIdx:]), " ")
		}
		if len(remaining) > 0 {
			result = append(result, remaining)
		}
	}
	return result
}
