package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveBinary_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test_binary")
	if err := os.WriteFile(f, []byte("test content"), 0o755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	if err := removeBinary(f); err != nil {
		t.Fatalf("removeBinary failed: %v", err)
	}

	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Errorf("file still exists after removeBinary")
	}
}

func TestRemoveBinary_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "does_not_exist")
	if err := removeBinary(f); err != nil {
		t.Errorf("removeBinary on non-existent file returned error: %v", err)
	}
}
