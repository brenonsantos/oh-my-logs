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
			name:     "standalone dollar device prompt becomes empty",
			input:    "$device",
			expected: "",
		},
		{
			name:     "standalone dollar device with space becomes empty",
			input:    "$ device ",
			expected: "",
		},
		{
			name:     "dollar device prompt prefix stripped from log",
			input:    "$device [ 12.345] <inf> sensor: ready",
			expected: "[ 12.345] <inf> sensor: ready",
		},
		{
			name:     "bracketed device prompt stripped from log",
			input:    "[device]$ [ 12.345] <inf> sensor: ready",
			expected: "[ 12.345] <inf> sensor: ready",
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
		{
			name:     "subcommand tab indentation and column formatting preserved",
			input:    "  set_conv_group\t: Set conversation group",
			expected: "  set_conv_group        : Set conversation group",
		},
		{
			name:     "inline ANSI cursor movements and clear sequences stripped",
			input:    "enable \x1b[1D \x1b[J\x1b[1D \x1b[J: enable target",
			expected: "enable   : enable target",
		},
		{
			name:     "active input buffer cleared by carriage return and erase line before log",
			input:    "os_msg enable_logging\r\x1b[K[    368.331] <wrn> nxp_wifi: WLAN: Unknown Event: 5",
			expected: "[    368.331] <wrn> nxp_wifi: WLAN: Unknown Event: 5",
		},
		{
			name:     "prompt with active input buffer cleared by carriage return before log",
			input:    "uart:~$ os_msg enable_logging\r\x1b[K[    368.331] <inf> wpa_supp: BG scan stopped",
			expected: "[    368.331] <inf> wpa_supp: BG scan stopped",
		},
		{
			name:     "prompt with active input buffer cleared by ANSI cursor back and erase before log",
			input:    "uart:~$ os_msg enable_logging\x1b[32D\x1b[J[    368.331] <inf> wpa_supp: BG scan stopped",
			expected: "[    368.331] <inf> wpa_supp: BG scan stopped",
		},
		{
			name:     "active input buffer followed by prompt redraw",
			input:    "os_msg enable_logging\ruart:~$ os_msg enable_logging",
			expected: "os_msg enable_logging",
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

func TestExpandTabs(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		tabWidth int
		expected string
	}{
		{
			name:     "no tabs",
			input:    "hello world",
			tabWidth: 8,
			expected: "hello world",
		},
		{
			name:     "single tab at start",
			input:    "\thello",
			tabWidth: 8,
			expected: "        hello",
		},
		{
			name:     "tab stop alignment with 4 spaces",
			input:    "cmd\tdesc",
			tabWidth: 4,
			expected: "cmd desc",
		},
		{
			name:     "tab stop alignment with 8 spaces",
			input:    "a\tb\tc",
			tabWidth: 8,
			expected: "a       b       c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := serial.ExpandTabs(tc.input, tc.tabWidth)
			if actual != tc.expected {
				t.Errorf("ExpandTabs(%q, %d) = %q, want %q", tc.input, tc.tabWidth, actual, tc.expected)
			}
		})
	}
}

func TestExpandCursorForward(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no cursor forward",
			input:    "plain string",
			expected: "plain string",
		},
		{
			name:     "single default cursor forward without digit",
			input:    "col1\x1b[Ccol2",
			expected: "col1 col2",
		},
		{
			name:     "explicit forward 10 positions",
			input:    "os_ble\x1b[10Cos_comm",
			expected: "os_ble          os_comm",
		},
		{
			name:     "multiline autocompletion row",
			input:    "os_ble\x1b[10Cos_comm\x1b[10Cos_crypto\x1b[10Cos_dataset",
			expected: "os_ble          os_comm          os_crypto          os_dataset",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := serial.ExpandCursorForward(tc.input)
			if actual != tc.expected {
				t.Errorf("ExpandCursorForward(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestExtractPromptRemainder(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		wantPrompt    string
		wantRemainder string
		wantMatched   bool
	}{
		{
			name:          "pure prompt",
			input:         "board:~$ ",
			wantPrompt:    "board:~$",
			wantRemainder: "",
			wantMatched:   true,
		},
		{
			name:          "prompt with partial command buffer",
			input:         "board:~$ os_",
			wantPrompt:    "board:~$",
			wantRemainder: "os_",
			wantMatched:   true,
		},
		{
			name:          "prompt with completed command",
			input:         "uart:~$ os_msg",
			wantPrompt:    "uart:~$",
			wantRemainder: "os_msg",
			wantMatched:   true,
		},
		{
			name:          "prompt with leading carriage return",
			input:         "\ruart:~$ os_msg",
			wantPrompt:    "uart:~$",
			wantRemainder: "os_msg",
			wantMatched:   true,
		},
		{
			name:          "bracketed prompt with completion",
			input:         "[myboard]:~$ status",
			wantPrompt:    "[myboard]:~$",
			wantRemainder: "status",
			wantMatched:   true,
		},
		{
			name:          "normal log line does not match",
			input:         "[   12.345] <inf> sensor: ok",
			wantPrompt:    "",
			wantRemainder: "",
			wantMatched:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, rem, matched := serial.ExtractPromptRemainder(tc.input)
			if matched != tc.wantMatched || prompt != tc.wantPrompt || rem != tc.wantRemainder {
				t.Errorf("ExtractPromptRemainder(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.input, prompt, rem, matched, tc.wantPrompt, tc.wantRemainder, tc.wantMatched)
			}
		})
	}
}

func TestResolveCarriageReturns(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"plain string", "plain string"},
		{"trailing cr\r", "trailing cr"},
		{"old\rnew line", "new line"},
		{"first\rsecond\rthird", "third"},
		{"os_msg\ruart:~$ os_msg", "uart:~$ os_msg"},
	}

	for _, tc := range cases {
		actual := serial.ResolveCarriageReturns(tc.input)
		if actual != tc.expected {
			t.Errorf("ResolveCarriageReturns(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestIsIdleShellPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "bare uart prompt",
			input:    "uart:~$ ",
			expected: true,
		},
		{
			name:     "bare device prompt",
			input:    "device:~$ ",
			expected: true,
		},
		{
			name:     "prompt with active command input",
			input:    "device:~$ os_",
			expected: true,
		},
		{
			name:     "prompt with active command input no space",
			input:    "device:~$ help",
			expected: true,
		},
		{
			name:     "in-flight log line with prompt-erase prefix",
			input:    "device:~$ \x1b[9D\x1b[J[    262.937] <inf> [lib-mqtt]: HandlePublishToClo",
			expected: false,
		},
		{
			name:     "in-flight log line with carriage return prompt erase",
			input:    "device:~$ \r\x1b[K[   123.456] <wrn> message",
			expected: false,
		},
		{
			name:     "in-flight log line without prompt",
			input:    "[      6.246] <err> [0x10A]: OnCloudProvisioningStatusChanged: Connection is not ",
			expected: false,
		},
		{
			name:     "tail continuation fragment of severed log line",
			input:    "udOsWork(): Published message successfully on topic",
			expected: false,
		},
		{
			name:     "log level indicator without prompt",
			input:    "<inf> system booting",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "whitespace only",
			input:    "   \t  ",
			expected: false,
		},
		{
			name:     "random text",
			input:    "hello world",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := serial.IsIdleShellPrompt(tc.input)
			if actual != tc.expected {
				t.Errorf("IsIdleShellPrompt(%q) = %v, want %v", tc.input, actual, tc.expected)
			}
		})
	}
}



