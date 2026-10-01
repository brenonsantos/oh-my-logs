//go:build windows

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// removeBinary removes the binary file on Windows.
// On Windows, a currently executing executable cannot be deleted directly due to OS file locks.
// However, Windows allows renaming an open or executing file.
// We rename the running executable to a temporary file, and dispatch a detached background
// command to delete the temporary file after this process exits.
func removeBinary(path string) error {
	// First attempt a direct removal (succeeds if not currently executing)
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// If direct removal failed due to sharing violation or permission error,
	// attempt to rename the file away from its installed location.
	tempPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".oml_uninstall_%d.tmp", os.Getpid()))
	_ = os.Remove(tempPath)

	if renameErr := os.Rename(path, tempPath); renameErr != nil {
		// If rename also fails, it's genuinely a permission issue (e.g. system directory like Program Files)
		return err
	}

	// Binary has been moved away from path (so path no longer exists to the user).
	// Launch a detached background process to clean up the temporary file once the current process exits.
	cmd := exec.Command("cmd.exe", "/C", fmt.Sprintf("ping 127.0.0.1 -n 2 >nul & del /f /q %q", tempPath))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	}
	_ = cmd.Start()

	return nil
}
