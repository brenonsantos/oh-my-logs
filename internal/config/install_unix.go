//go:build !windows

package config

import "os"

// removeBinary removes the binary file on Unix-like operating systems.
// On Unix, unlinking an open or currently executing binary is supported by the OS kernel.
func removeBinary(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
