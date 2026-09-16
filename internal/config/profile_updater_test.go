package config_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current   string
		candidate string
		expected  bool
	}{
		{"1.0.0", "1.0.1", true},
		{"v1.0.0", "v1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.2.0", "1.2.0", false},
		{"1.2.1", "1.2.0", false},
		{"v2.0.0", "v1.9.9", false},
		{"", "1.0.0", true},
		{"1.0.0", "", false},
	}

	for _, tc := range tests {
		got := config.IsNewerVersion(tc.current, tc.candidate)
		if got != tc.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tc.current, tc.candidate, got, tc.expected)
		}
	}
}

func TestUpdateProfile_Success(t *testing.T) {
	// Remote server serving version 1.1.0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
name: UpdatableTest
version: "1.1.0"
parser:
  type: raw
columns:
  - field: message
    title: Updated Message
    width: 0
`))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	_ = os.MkdirAll(profilesDir, 0o755)

	// Local installed version 1.0.0 with source pointing to ts.URL
	initialYAML := fmt.Sprintf(`
name: UpdatableTest
version: "1.0.0"
source: %q
parser:
  type: raw
columns:
  - field: message
    title: Old Message
    width: 0
`, ts.URL)

	profFile := filepath.Join(profilesDir, "updatabletest.yaml")
	if err := os.WriteFile(profFile, []byte(initialYAML), 0o644); err != nil {
		t.Fatalf("failed to write initial profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		DecodersDir: filepath.Join(tmpDir, "decoders"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	// Run update
	res, err := config.UpdateProfile(appCfg, "UpdatableTest", config.InstallOptions{})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if res.PreviousVersion != "1.0.0" {
		t.Errorf("expected PreviousVersion 1.0.0, got %s", res.PreviousVersion)
	}
	if res.NewVersion != "1.1.0" {
		t.Errorf("expected NewVersion 1.1.0, got %s", res.NewVersion)
	}

	// Verify profile file on disk was updated to 1.1.0
	updatedProf, err := parser.LoadProfile(profFile)
	if err != nil {
		t.Fatalf("failed to load updated profile: %v", err)
	}
	if updatedProf.Version != "1.1.0" {
		t.Errorf("expected loaded profile version 1.1.0, got %s", updatedProf.Version)
	}
}
