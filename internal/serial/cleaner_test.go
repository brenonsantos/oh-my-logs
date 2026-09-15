package serial_test

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestCleanTerminalLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty line",
			input:    "",
			expected: "",
		},
		{
			name:     "plain log line untouched",
			input:    "[   5206.361] <inf> os_msg: CS:Vd/BzS6(JJ)CGvXa=MQAAAA,CFph3=MgUAAA",
			expected: "[   5206.361] <inf> os_msg: CS:Vd/BzS6(JJ)CGvXa=MQAAAA,CFph3=MgUAAA",
		},
		{
			name:     "board prompt with ANSI cursor back and erase",
			input:    "board:~$ \x1b[9D\x1b[J[   5208.360] <inf> os_msg: CS:Vd/BzS6(JL)CGvXa=MQAAAA,CFph3=LQUAAA",
			expected: "[   5208.360] <inf> os_msg: CS:Vd/BzS6(JL)CGvXa=MQAAAA,CFph3=LQUAAA",
		},
		{
			name:     "prompt with double space before ANSI erase",
			input:    "board:~$  \x1b[9D \x1b[J[   5210.360] <inf> os_msg: hello",
			expected: "[   5210.360] <inf> os_msg: hello",
		},
		{
			name:     "uart shell prompt with carriage return and erase line",
			input:    "uart:~$ \r\x1b[K[   12.345] <dbg> app: init ok",
			expected: "[   12.345] <dbg> app: init ok",
		},
		{
			name:     "prompt without ANSI erase sequence",
			input:    "myboard:~$ [      6.122] <inf> app: boot ok",
			expected: "[      6.122] <inf> app: boot ok",
		},
		{
			name:     "custom shell angle bracket prompt",
			input:    "shell> [ 1.000] system ready",
			expected: "[ 1.000] system ready",
		},
		{
			name:     "root hash prompt",
			input:    "root@device# [ 0.001] kernel loaded",
			expected: "[ 0.001] kernel loaded",
		},
		{
			name:     "standalone prompt becomes empty",
			input:    "board:~$ ",
			expected: "",
		},
		{
			name:     "colored log preserved",
			input:    "\x1b[32m[   5208.360]\x1b[0m <inf> os_msg: ok",
			expected: "\x1b[32m[   5208.360]\x1b[0m <inf> os_msg: ok",
		},
		{
			name:     "message containing dollar sign and angle brackets untouched",
			input:    "[ 1.234] <inf> transaction total is $50.00 for user <john>",
			expected: "[ 1.234] <inf> transaction total is $50.00 for user <john>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := serial.CleanTerminalLine(tc.input)
			if actual != tc.expected {
				t.Errorf("CleanTerminalLine(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}
