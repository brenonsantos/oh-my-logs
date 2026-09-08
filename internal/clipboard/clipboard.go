package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
)

// Copy copies the given text to the system clipboard.
// It simultaneously emits an ANSI OSC 52 sequence (which functions across
// terminal emulators, tmux sessions, and SSH connections) and writes to the
// native OS clipboard utility if available.
func Copy(text string) error {
	// 1. ANSI OSC 52 sequence to standard error
	seq := osc52.New(text)
	_, _ = fmt.Fprint(os.Stderr, seq)

	// 2. Platform native clipboard commands
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()

	case "linux":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			cmd := exec.Command("wl-copy")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
		cmd2 := exec.Command("xsel", "--clipboard", "--input")
		cmd2.Stdin = strings.NewReader(text)
		_ = cmd2.Run()

	case "windows":
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
	}

	return nil
}

// Read retrieves text from the platform system clipboard if available.
func Read() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		if err == nil {
			return string(out), nil
		}
		return "", err

	case "linux":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			out, err := exec.Command("wl-paste", "--no-newline").Output()
			if err == nil {
				return string(out), nil
			}
		}
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		if err == nil {
			return string(out), nil
		}
		out2, err2 := exec.Command("xsel", "--clipboard", "--output").Output()
		if err2 == nil {
			return string(out2), nil
		}
		return "", fmt.Errorf("no clipboard provider found (tried wl-paste, xclip, xsel)")

	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
		if err == nil {
			return strings.TrimRight(string(out), "\r\n"), nil
		}
		return "", err
	}

	return "", fmt.Errorf("clipboard read not supported on %s", runtime.GOOS)
}
