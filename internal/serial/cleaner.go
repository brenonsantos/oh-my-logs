package serial

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// promptEraseRe matches an interactive prompt (optionally followed by active user input
	// in the shell's buffer) and the ANSI cursor-back, carriage-return, and line-erasure
	// escape sequences sent by embedded shells (e.g. Zephyr CONFIG_SHELL=y, FreeRTOS)
	// when clearing the active prompt to output an asynchronous log message.
	// Example: "device:~$ \x1b[10D\x1b[J", "device:~$ os_msg\x1b[16D\x1b[J", "uart:~$ os_msg\r\x1b[K".
	promptEraseRe = regexp.MustCompile(`^(?:(?:(?:[$#]\s*[a-zA-Z0-9_.-]+[:>]?\s*|\[[a-zA-Z0-9_.-]+\](?::~?|~)?\s*[$#>]\s*|(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.-]+(?::~?|~)?[$#>]\s*)[^\x1b\r\n]*)\s*(?:\x1b\[\d*D\s*(?:\x1b\[[0-2]?[JK]\s*)+|\r\s*(?:\x1b\[[0-2]?[JK]\s*)*)|\r\s*(?:\x1b\[[0-2]?[JK]\s*)*)`)

	// shellPromptRe matches leftover interactive shell prompts at the start of a line
	// that were not followed by ANSI erasure codes.
	// Examples: "device:~$ ", "uart:~$ ", "$device", "$ device", "[device]:~$ ", "shell> ", "root@host# ".
	shellPromptRe = regexp.MustCompile(`^(?:[$#]\s*[a-zA-Z0-9_.-]+[:>]?\s*|\[[a-zA-Z0-9_.-]+\](?::~?|~)?\s*[$#>]\s*|(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.-]+(?::~?|~)?[$#>]\s*)`)

	// residualCursorRe matches leading ANSI cursor movement or erase commands
	// without removing SGR color codes (which end in 'm').
	residualCursorRe = regexp.MustCompile(`^(?:\x1b\[\d*[A-HJK]\s*)+`)

	// inlineCursorEraseRe matches cursor movement or line-clear sequences (e.g. \x1b[1D\x1b[J)
	// that can appear anywhere in the stream when MCU shells erase/redraw characters.
	inlineCursorEraseRe = regexp.MustCompile(`\x1b\[\d*[A-HJK]`)

	// cursorForwardRe matches ANSI cursor-forward sequences (\x1b[<n>C) used by VT100
	// terminals and embedded shells (such as Zephyr) to space out autocompletion columns.
	cursorForwardRe = regexp.MustCompile(`\x1b\[(\d*)C`)

	// logHeaderIndicatorRe identifies log headers (timestamps, levels, brackets) that indicate
	// the buffer contains an active in-flight log line rather than an idle interactive shell prompt.
	logHeaderIndicatorRe = regexp.MustCompile(`(?i)(?:\[\s*\d+(?:\.\d+)?\s*\]|<(?:err|wrn|inf|dbg|trace|fatal|crit)>|\[(?:err|wrn|inf|dbg|error|warn|info|debug)\])`)
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

// ExpandCursorForward replaces ANSI cursor-forward jumps (\x1b[<n>C) with n spaces
// (defaulting to 1 space when n is omitted or 0), preserving column formatting in
// MCU autocompletion tables and help listings.
func ExpandCursorForward(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	return cursorForwardRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := cursorForwardRe.FindStringSubmatch(match)
		if len(sub) < 2 || sub[1] == "" {
			return " "
		}
		n, err := strconv.Atoi(sub[1])
		if err != nil || n <= 0 {
			return " "
		}
		if n > 120 {
			n = 120
		}
		return strings.Repeat(" ", n)
	})
}

// CleanTerminalLine strips interactive shell prompts and ANSI cursor-repositioning /
// erasure sequences that embedded shells use to redraw or erase prompts before logging.
// SGR color escape codes (e.g. \x1b[32m), tabs (expanded to tab stops), cursor jumps
// (expanded to spaces), and indented subcommand layouts are preserved intact without flattening.
func CleanTerminalLine(line string) string {
	if line == "" {
		return ""
	}

	// 0. Resolve in-band carriage returns so overwritten lines or prompts cleared via \r reflect terminal display
	cleaned := ResolveCarriageReturns(line)

	// 1. Expand tabs so tabular shell layouts (help menus, subcommands) retain their column alignment
	cleaned = ExpandTabs(cleaned, 8)

	// 2. Expand ANSI cursor-forward sequences so multiline completion columns (e.g. Zephyr shell) do not merge
	cleaned = ExpandCursorForward(cleaned)

	// 3. Remove prompt erasure sequences (e.g. "device:~$ \x1b[10D\x1b[J")
	cleaned = promptEraseRe.ReplaceAllLiteralString(cleaned, "")

	// 4. Remove residual cursor movement / line erasure codes at line start
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	// 5. Remove leftover shell prompts (e.g. "device:~$ ")
	cleaned = shellPromptRe.ReplaceAllLiteralString(cleaned, "")

	// 6. Remove any newly exposed leading cursor controls
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	// 7. Strip any leftover inline cursor-movement / clear sequences (e.g. \x1b[1D \x1b[J)
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

// IsIdleShellPrompt returns true if line represents a genuine idle interactive shell prompt
// waiting for engineer input (e.g. "device:~$ " or "uart:~$ os_") without a trailing newline.
// It returns false if the line is an in-flight log message (e.g. prefixed by prompt-erasure
// escape codes, or containing log timestamps/levels), preventing premature line splitting.
func IsIdleShellPrompt(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// 1. If it contains prompt-erasure sequences, the shell was erasing the prompt
	// to output an asynchronous log message.
	if promptEraseRe.MatchString(line) {
		return false
	}

	// 2. If it contains log level or timestamp signatures, it is an in-flight log line.
	if logHeaderIndicatorRe.MatchString(line) {
		return false
	}

	// 3. Must match an active shell prompt at the current cursor position.
	_, remainder, ok := ExtractPromptRemainder(line)
	if !ok {
		return false
	}

	// 4. If the remainder itself contains log signatures, do not treat as idle prompt.
	if logHeaderIndicatorRe.MatchString(remainder) {
		return false
	}

	return true
}

// ResolveCarriageReturns handles in-band carriage returns (\r) in serial streams,
// taking the text after the last carriage return to reflect what a terminal displays.
func ResolveCarriageReturns(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	trimmed := strings.TrimRight(s, "\r")
	if !strings.ContainsRune(trimmed, '\r') {
		return trimmed
	}
	lastCR := strings.LastIndexByte(trimmed, '\r')
	return trimmed[lastCR+1:]
}

// ExtractPromptRemainder inspects line to determine if it begins with an interactive shell prompt.
// If matched, it returns the detected prompt and any trailing command line / autocomplete buffer.
func ExtractPromptRemainder(line string) (prompt string, remainder string, matched bool) {
	s := ResolveCarriageReturns(line)
	s = ExpandCursorForward(s)
	s = promptEraseRe.ReplaceAllLiteralString(s, "")
	s = residualCursorRe.ReplaceAllLiteralString(s, "")
	s = strings.TrimLeft(s, "\r\n ")
	s = residualCursorRe.ReplaceAllLiteralString(s, "")
	loc := shellPromptRe.FindStringIndex(s)
	if loc == nil {
		return "", "", false
	}
	prompt = strings.TrimSpace(s[loc[0]:loc[1]])
	rem := strings.TrimRight(s[loc[1]:], "\r\n")
	return prompt, rem, true
}
