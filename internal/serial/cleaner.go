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
	promptEraseRe = regexp.MustCompile(`^(?:.*?\x1b\[\d*D\s*(?:\x1b\[[0-2]?[JK]\s*)+|.*?\r\s*(?:\x1b\[[0-2]?[JK]\s*)*)`)

	// shellPromptRe matches leftover interactive shell prompts at the start of a line
	// that were not followed by ANSI erasure codes.
	// Examples: "device:~$ ", "uart:~$ ", "myboard:~$ ", "shell> ", "root@host# ".
	shellPromptRe = regexp.MustCompile(`^(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.-]+(?::~?|~)?[$#>]\s*`)

	// residualCursorRe matches leading ANSI cursor movement or erase commands
	// without removing SGR color codes (which end in 'm').
	residualCursorRe = regexp.MustCompile(`^(?:\x1b\[\d*[A-HJK]\s*)+`)
)

// CleanTerminalLine strips interactive shell prompts and ANSI cursor-repositioning /
// erasure sequences that embedded shells use to redraw or erase prompts before logging.
// SGR color escape codes (e.g. \x1b[32m) and valid log content are preserved intact.
func CleanTerminalLine(line string) string {
	if line == "" {
		return ""
	}

	// 1. Remove prompt erasure sequences (e.g. "device:~$ \x1b[10D\x1b[J")
	cleaned := promptEraseRe.ReplaceAllLiteralString(line, "")

	// 2. Remove residual cursor movement / line erasure codes at line start
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	// 3. Remove leftover shell prompts (e.g. "device:~$ ")
	cleaned = shellPromptRe.ReplaceAllLiteralString(cleaned, "")

	// 4. Remove any newly exposed leading cursor controls
	cleaned = residualCursorRe.ReplaceAllLiteralString(cleaned, "")

	return strings.TrimSpace(cleaned)
}
