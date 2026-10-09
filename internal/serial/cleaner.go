package serial

import (
	"regexp"
	"strings"
)

var (
	// promptEraseRe matches an interactive prompt and the ANSI cursor-back and line-erasure
	// escape sequences sent by embedded shells (e.g. Zephyr CONFIG_SHELL=y, FreeRTOS)
	// when clearing the active prompt to output an asynchronous log message.
	// Example: "device:~$ \x1b[10D\x1b[J" or "uart:~$ \x1b[8D\x1b[K" or "prompt\r\x1b[K".
	promptEraseRe = regexp.MustCompile(`^(?:(?:[$#]\s*[a-zA-Z0-9_.-]+[:>]?\s*|\[[a-zA-Z0-9_.-]+\]\s*[$#>]\s*|(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.-]+(?::~?|~)?[$#>]\s*)\s*(?:\x1b\[\d*D\s*(?:\x1b\[[0-2]?[JK]\s*)+|\r\s*(?:\x1b\[[0-2]?[JK]\s*)*)|\r\s*(?:\x1b\[[0-2]?[JK]\s*)*)`)

	// shellPromptRe matches leftover interactive shell prompts at the start of a line
	// that were not followed by ANSI erasure codes.
	// Examples: "device:~$ ", "uart:~$ ", "$device", "$ device", "[device]$ ", "shell> ", "root@host# ".
	shellPromptRe = regexp.MustCompile(`^(?:[$#]\s*[a-zA-Z0-9_.-]+[:>]?\s*|\[[a-zA-Z0-9_.-]+\]\s*[$#>]\s*|(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.-]+(?::~?|~)?[$#>]\s*)`)

	// residualCursorRe matches leading ANSI cursor movement or erase commands
	// without removing SGR color codes (which end in 'm').
	residualCursorRe = regexp.MustCompile(`^(?:\x1b\[\d*[A-HJK]\s*)+`)

	// inlineCursorEraseRe matches cursor movement or line-clear sequences (e.g. \x1b[1D\x1b[J)
	// that can appear anywhere in the stream when MCU shells erase/redraw characters.
	inlineCursorEraseRe = regexp.MustCompile(`\x1b\[\d*[A-HJK]`)
)

// ExpandTabs replaces tab characters with spaces aligned to tabWidth stops (default 8).
func ExpandTabs(s string, tabWidth int) string {
	if tabWidth <= 0 {
		tabWidth = 8
	}
	if !strings.ContainsRune(s, '\t') {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + 8)
	col := 0

	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			if r == '\n' || r == '\r' {
				col = 0
			} else {
				col++
			}
		}
	}
	return b.String()
}

// CleanTerminalLine strips interactive shell prompts and ANSI cursor-repositioning /
// erasure sequences that embedded shells use to redraw or erase prompts before logging.
// SGR color escape codes (e.g. \x1b[32m), tabs (expanded to tab stops), and indented
// subcommand layouts are preserved intact without flattening.
func CleanTerminalLine(line string) string {
	if line == "" {
		return ""
	}

	// 1. Expand tabs so tabular shell layouts (help menus, subcommands) retain their column alignment
	cleaned := ExpandTabs(line, 8)

	// 2. Remove prompt erasure sequences (e.g. "device:~$ \x1b[10D\x1b[J")
	cleaned = promptEraseRe.ReplaceAllLiteralString(cleaned, "")

	// 3. Remove residual cursor movement / line erasure codes at line start
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	// 4. Remove leftover shell prompts (e.g. "device:~$ ")
	cleaned = shellPromptRe.ReplaceAllLiteralString(cleaned, "")

	// 5. Remove any newly exposed leading cursor controls
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	// 6. Strip any leftover inline cursor-movement / clear sequences (e.g. \x1b[1D \x1b[J)
	cleaned = inlineCursorEraseRe.ReplaceAllLiteralString(cleaned, "")

	// Strip trailing carriage returns and newlines, preserving leading indentation for subcommands
	cleaned = strings.TrimRight(cleaned, "\r\n")

	// If the entire remaining line is only whitespace, return empty string so blank prompt artifacts vanish
	if strings.TrimSpace(cleaned) == "" {
		return ""
	}

	return cleaned
}

// DetectShellPrompt inspects line and returns the prompt string if line matches
// a standalone or prefixed shell prompt (e.g. "device:~$ ", "$device", "uart:~$ ").
func DetectShellPrompt(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	m := shellPromptRe.FindString(trimmed)
	return strings.TrimSpace(m)
}
