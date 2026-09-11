package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
)

// OrderedRecordFields returns key-value pairs sorted with prominent fields first
// (level, timestamp, module, caller, thread), followed by remaining fields alphabetically.
// Excludes "message" and "msg" which are rendered in the primary payload view.
func OrderedRecordFields(fields map[string]string) [][2]string {
	if len(fields) == 0 {
		return nil
	}

	priorityOrder := []string{
		"level",
		"timestamp", "time", "uptime", "_ts",
		"module", "tag", "service", "app",
		"caller", "func", "function", "file", "line",
		"thread", "pid", "tid",
	}

	priorityMap := make(map[string]int)
	for i, p := range priorityOrder {
		priorityMap[p] = i + 1
	}

	var pairs [][2]string
	for k, v := range fields {
		lowerK := strings.ToLower(k)
		if lowerK == "message" || lowerK == "msg" {
			continue
		}
		pairs = append(pairs, [2]string{k, v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		pI := priorityMap[strings.ToLower(pairs[i][0])]
		pJ := priorityMap[strings.ToLower(pairs[j][0])]

		if pI > 0 && pJ > 0 {
			if pI != pJ {
				return pI < pJ
			}
		} else if pI > 0 {
			return true
		} else if pJ > 0 {
			return false
		}

		return strings.ToLower(pairs[i][0]) < strings.ToLower(pairs[j][0])
	})

	return pairs
}

// FormatHexPreview formats raw bytes into canonical 16-byte hex dump lines.
func FormatHexPreview(raw []byte, maxBytes int) []string {
	if len(raw) == 0 {
		return nil
	}

	limit := len(raw)
	if maxBytes > 0 && limit > maxBytes {
		limit = maxBytes
	}

	var lines []string
	for i := 0; i < limit; i += 16 {
		chunkEnd := i + 16
		if chunkEnd > limit {
			chunkEnd = limit
		}
		chunk := raw[i:chunkEnd]

		// Hex bytes
		var hexParts []string
		for _, b := range chunk {
			hexParts = append(hexParts, fmt.Sprintf("%02x", b))
		}
		hexStr := strings.Join(hexParts, " ")
		if len(chunk) < 16 {
			// Pad remaining space
			hexStr += strings.Repeat("   ", 16-len(chunk))
		}

		// Printable ASCII
		var asciiParts []rune
		for _, b := range chunk {
			if b >= 32 && b <= 126 {
				asciiParts = append(asciiParts, rune(b))
			} else {
				asciiParts = append(asciiParts, '.')
			}
		}

		lines = append(lines, fmt.Sprintf("%04x  %-48s  |%s|", i, hexStr, string(asciiParts)))
	}

	if limit < len(raw) {
		lines = append(lines, fmt.Sprintf("... (%d more bytes)", len(raw)-limit))
	}

	return lines
}

// FormattedRecordDetail builds a clean plaintext multi-line representation of
// a Record suitable for copying to the OS clipboard or exporting.
func FormattedRecordDetail(r record.Record, tsField string, bookmarked bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Record ID: #%d\n", r.ID))

	tsStr := ""
	if !r.Timestamp.IsZero() {
		tsStr = r.Timestamp.Format("2006-01-02 15:04:05.000000")
	} else if r.Get(tsField) != "" {
		tsStr = r.Get(tsField)
	}
	if tsStr != "" {
		sb.WriteString(fmt.Sprintf("Timestamp: %s\n", tsStr))
	}

	if r.Delta > 0 {
		sb.WriteString(fmt.Sprintf("Delta Time: +%s\n", timing.FormatDelta(r.Delta)))
	}

	if lvl := r.Get("level"); lvl != "" {
		sb.WriteString(fmt.Sprintf("Level: %s\n", strings.ToUpper(lvl)))
	}

	if bookmarked {
		sb.WriteString("Pinned: Yes\n")
	}

	fields := OrderedRecordFields(r.Fields)
	if len(fields) > 0 {
		sb.WriteString("\nFields:\n")
		for _, f := range fields {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", f[0], f[1]))
		}
	}

	msg := r.Get("message")
	if msg == "" {
		msg = r.Get("msg")
	}

	if msg != "" {
		sb.WriteString("\nMessage:\n")
		det := DetectAndFormatPayload(msg, Palette{})
		if det.Type != PayloadNone {
			if det.Prefix != "" {
				sb.WriteString(det.Prefix + "\n")
			}
			sb.WriteString(det.FormattedText + "\n")
			if det.Suffix != "" {
				sb.WriteString(det.Suffix + "\n")
			}
		} else {
			sb.WriteString(msg + "\n")
		}
	}

	if r.Raw != "" && (len(r.Fields) > 0 || r.Raw != msg) {
		sb.WriteString("\nRaw Log:\n")
		sb.WriteString(r.Raw + "\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}
