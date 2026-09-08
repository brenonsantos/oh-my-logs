package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultInstallDir returns the recommended binary installation directory for the current OS.
func DefaultInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "Programs", "oh-my-logs"), nil
	case "darwin":
		// On macOS, try /usr/local/bin if writable, else ~/.local/bin
		if isWritableDir("/usr/local/bin") {
			return "/usr/local/bin", nil
		}
		return filepath.Join(home, ".local", "bin"), nil
	default: // linux / unix
		if isWritableDir("/usr/local/bin") {
			return "/usr/local/bin", nil
		}
		return filepath.Join(home, ".local", "bin"), nil
	}
}

func isWritableDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	// Try creating a temp file to verify write permissions
	testFile := filepath.Join(dir, fmt.Sprintf(".oml_write_test_%d", os.Getpid()))
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}

// InstallBinary copies the currently running binary to targetDir and ensures example profiles exist.
func InstallBinary(appCfg *AppConfig, targetDir string) (string, error) {
	selfPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find current executable: %w", err)
	}
	// Resolve symlinks
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlink for executable: %w", err)
	}

	if targetDir == "" {
		targetDir, err = DefaultInstallDir()
		if err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create target directory %q: %w", targetDir, err)
	}

	binaryName := "oml"
	if runtime.GOOS == "windows" {
		binaryName = "oml.exe"
	}
	destPath := filepath.Join(targetDir, binaryName)

	// Don't overwrite if installing to the exact same file
	absSrc, _ := filepath.Abs(selfPath)
	absDest, _ := filepath.Abs(destPath)
	if absSrc != absDest {
		srcFile, err := os.Open(selfPath)
		if err != nil {
			return "", fmt.Errorf("cannot read executable: %w", err)
		}
		defer srcFile.Close()

		// Write to temporary file first then rename to handle running binary replacement
		tempDest := destPath + ".tmp"
		destFile, err := os.OpenFile(tempDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("cannot write to %q: %w", destPath, err)
		}

		if _, err := io.Copy(destFile, srcFile); err != nil {
			destFile.Close()
			_ = os.Remove(tempDest)
			return "", fmt.Errorf("failed copying binary: %w", err)
		}
		destFile.Close()

		if err := os.Rename(tempDest, destPath); err != nil {
			// On Windows, rename might fail if destination exists; try removing first
			_ = os.Remove(destPath)
			if err := os.Rename(tempDest, destPath); err != nil {
				return "", fmt.Errorf("cannot install binary to %q: %w", destPath, err)
			}
		}
		_ = os.Chmod(destPath, 0o755)
	}

	// Copy example profiles to global directory if present and not yet installed
	if appCfg != nil && appCfg.ProfilesDir != "" {
		CopyDefaultProfiles(appCfg.ProfilesDir)
	}

	return destPath, nil
}

// CopyDefaultProfiles copies any files in local profiles/examples into targetDir without overwriting.
func CopyDefaultProfiles(targetDir string) int {
	_ = os.MkdirAll(targetDir, 0o755)
	localExamples := filepath.Join("profiles", "examples")
	entries, err := os.ReadDir(localExamples)
	if err != nil {
		return 0
	}

	copied := 0
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		dest := filepath.Join(targetDir, e.Name())
		if _, err := os.Stat(dest); err == nil {
			continue // Don't overwrite existing user profiles
		}
		src := filepath.Join(localExamples, e.Name())
		data, err := os.ReadFile(src)
		if err == nil {
			if err := os.WriteFile(dest, data, 0o644); err == nil {
				copied++
			}
		}
	}
	return copied
}

// UninstallBinary removes the installed binary from targetDir.
func UninstallBinary(targetDir string) (string, error) {
	if targetDir == "" {
		var err error
		targetDir, err = DefaultInstallDir()
		if err != nil {
			return "", err
		}
	}

	binaryName := "oml"
	if runtime.GOOS == "windows" {
		binaryName = "oml.exe"
	}
	destPath := filepath.Join(targetDir, binaryName)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return "", fmt.Errorf("binary not found at %q", destPath)
	}

	if err := os.Remove(destPath); err != nil {
		return "", fmt.Errorf("cannot remove %q: %w", destPath, err)
	}

	return destPath, nil
}
