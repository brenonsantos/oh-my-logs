//go:build darwin

package sim

import (
	"bytes"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func openMasterPTY() (int, string, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("sim: open /dev/ptmx: %w", err)
	}

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGRANT, 0); errno != 0 {
		_ = unix.Close(fd)
		return -1, "", fmt.Errorf("sim: TIOCPTYGRANT: %w", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCPTYUNLK, 0); errno != 0 {
		_ = unix.Close(fd)
		return -1, "", fmt.Errorf("sim: TIOCPTYUNLK: %w", errno)
	}

	var nameBuf [128]byte
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&nameBuf[0]))); errno != 0 {
		_ = unix.Close(fd)
		return -1, "", fmt.Errorf("sim: TIOCPTYGNAME: %w", errno)
	}
	if idx := bytes.IndexByte(nameBuf[:], 0); idx >= 0 {
		return fd, string(nameBuf[:idx]), nil
	}
	return fd, string(nameBuf[:]), nil
}

func setRawTermios(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	makeRaw(t)
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}
