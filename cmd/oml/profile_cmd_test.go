package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

const testProfileYAML = `
name: CmdTest
parser:
  type: raw
columns:
  - field: message
    title: Message
    width: 0
decoders:
  - match: "^test:"
    format: "decoded: {message}"
`

func setupTestAppConfig(t *testing.T) *config.AppConfig {
	t.Helper()
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	decodersDir := filepath.Join(tmpDir, "decoders")
	logsDir := filepath.Join(tmpDir, "logs")

	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	if err := os.MkdirAll(decodersDir, 0o755); err != nil {
		t.Fatalf("failed to create decoders dir: %v", err)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	return &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		DecodersDir: decodersDir,
		LogsDir:     logsDir,
	}
}

func TestCmdProfileSubcommands(t *testing.T) {
	appCfg := setupTestAppConfig(t)

	// 1. Create a source profile
	srcFile := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(srcFile, []byte(testProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write source profile: %v", err)
	}

	// 2. Test install subcommand
	if code := runProfileCommand(appCfg, []string{"install", srcFile}); code != 0 {
		t.Fatalf("expected install exit code 0, got %d", code)
	}

	// 3. Test list subcommand
	if code := runProfileCommand(appCfg, []string{"list"}); code != 0 {
		t.Fatalf("expected list exit code 0, got %d", code)
	}

	// 4. Test show by name
	if code := runProfileCommand(appCfg, []string{"show", "CmdTest"}); code != 0 {
		t.Fatalf("expected show by name exit code 0, got %d", code)
	}

	// 5. Test show by ID "#1"
	if code := runProfileCommand(appCfg, []string{"show", "#1"}); code != 0 {
		t.Fatalf("expected show by ID exit code 0, got %d", code)
	}

	// 6. Test export to file
	exportFile := filepath.Join(t.TempDir(), "exported.yaml")
	if code := runProfileCommand(appCfg, []string{"export", "1", "--out", exportFile}); code != 0 {
		t.Fatalf("expected export exit code 0, got %d", code)
	}
	if _, err := os.Stat(exportFile); err != nil {
		t.Fatalf("expected exported file to exist: %v", err)
	}

	// 7. Test update subcommand with --check
	if code := runProfileCommand(appCfg, []string{"update", "--check"}); code != 0 {
		t.Fatalf("expected update --check exit code 0, got %d", code)
	}

	// 8. Test path subcommand
	if code := runProfileCommand(appCfg, []string{"path"}); code != 0 {
		t.Fatalf("expected path exit code 0, got %d", code)
	}

	// 9. Test uninstall by ID "1"
	if code := runProfileCommand(appCfg, []string{"uninstall", "1"}); code != 0 {
		t.Fatalf("expected uninstall exit code 0, got %d", code)
	}

	// 9. Verify profile was uninstalled
	list := config.ListProfiles(appCfg)
	for _, p := range list {
		if p.Name == "CmdTest" && p.IsGlobal {
			t.Errorf("expected CmdTest to be uninstalled, but still found in list")
		}
	}
}

func TestCmdProfileHelp(t *testing.T) {
	appCfg := setupTestAppConfig(t)
	if code := runProfileCommand(appCfg, []string{"help"}); code != 0 {
		t.Errorf("expected help exit code 0, got %d", code)
	}
}
