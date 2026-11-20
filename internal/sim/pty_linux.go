//go:build linux

package sim

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func openMasterPTY() (int, string, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("sim: open /dev/ptmx: %w", err)
	}

	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		_ = unix.Close(fd)
		return -1, "", fmt.Errorf("sim: TIOCSPTLCK: %w", err)
	}

	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		_ = unix.Close(fd)
		return -1, "", fmt.Errorf("sim: TIOCGPTN: %w", err)
	}
	return fd, fmt.Sprintf("/dev/pts/%d", n), nil
}

func setRawTermios(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	makeRaw(t)
	return unix.IoctlSetTermios(fd, unix.TCSETS, t)
}
