//go:build !darwin && !linux

package sim

import "fmt"

// PTY is not supported on this platform.
type PTY struct {
	SlavePath string
	Symlink   string
}

// OpenPTY returns an error on platforms without POSIX PTY support.
func OpenPTY(string) (*PTY, error) {
	return nil, fmt.Errorf("sim: PTY virtual serial ports require macOS or Linux")
}

func (p *PTY) Read([]byte) (int, error)  { return 0, fmt.Errorf("sim: unsupported platform") }
func (p *PTY) Write([]byte) (int, error) { return 0, fmt.Errorf("sim: unsupported platform") }
func (p *PTY) Close() error              { return nil }
