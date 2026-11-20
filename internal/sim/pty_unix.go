//go:build darwin || linux

package sim

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// PTY represents a virtual serial port pair (master for simulator, slave for oml).
type PTY struct {
	master    *os.File
	slave     *os.File
	SlavePath string
	Symlink   string

	closeOnce sync.Once
}

// OpenPTY allocates a new pseudo-terminal pair in raw mode and optionally
// creates a convenience symlink (e.g. "/tmp/oml-sim") pointing to SlavePath.
func OpenPTY(symlink string) (*PTY, error) {
	masterFD, slavePath, err := openMasterPTY()
	if err != nil {
		return nil, err
	}

	// Open the slave in raw mode and keep it open so the simulator's master
	// Read does not return EIO while waiting for oml to connect or reconnect.
	slaveFD, err := unix.Open(slavePath, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(masterFD)
		return nil, fmt.Errorf("sim: open slave %q: %w", slavePath, err)
	}
	if err := setRawTermios(slaveFD); err != nil {
		_ = unix.Close(slaveFD)
		_ = unix.Close(masterFD)
		return nil, fmt.Errorf("sim: set raw mode on %q: %w", slavePath, err)
	}

	// Non-blocking masterFD lets Go's netpoller manage Read/Write so Close()
	// immediately unblocks any pending Read.
	if err := unix.SetNonblock(masterFD, true); err != nil {
		_ = unix.Close(slaveFD)
		_ = unix.Close(masterFD)
		return nil, fmt.Errorf("sim: set nonblock on master: %w", err)
	}

	p := &PTY{
		master:    os.NewFile(uintptr(masterFD), "/dev/ptmx"),
		slave:     os.NewFile(uintptr(slaveFD), slavePath),
		SlavePath: slavePath,
		Symlink:   symlink,
	}

	if symlink != "" {
		_ = os.Remove(symlink)
		if err := os.Symlink(slavePath, symlink); err != nil {
			_ = p.Close()
			return nil, fmt.Errorf("sim: create symlink %q -> %q: %w", symlink, slavePath, err)
		}
	}

	return p, nil
}

// Read reads bytes sent by the client (oml) from the master side.
func (p *PTY) Read(b []byte) (int, error) { return p.master.Read(b) }

// Write writes bytes from the simulated device to the master side.
func (p *PTY) Write(b []byte) (int, error) { return p.master.Write(b) }

// Close tears down the PTY and removes any created symlink.
func (p *PTY) Close() error {
	var firstErr error
	p.closeOnce.Do(func() {
		if p.Symlink != "" {
			_ = os.Remove(p.Symlink)
		}
		if p.master != nil {
			if err := p.master.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if p.slave != nil {
			if err := p.slave.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	return firstErr
}

func makeRaw(t *unix.Termios) {
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB
	t.Cflag |= unix.CS8
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}
