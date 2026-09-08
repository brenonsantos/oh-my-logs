package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// ProfileInfo holds metadata about a discovered profile.
type ProfileInfo struct {
	Name       string
	Path       string
	IsGlobal   bool
	ParserType string
}

// ListProfiles returns metadata for all discovered profiles.
func ListProfiles(appCfg *AppConfig) []ProfileInfo {
	paths := FindAllProfilePaths(appCfg)
	var list []ProfileInfo
	for _, p := range paths {
		prof, err := parser.LoadProfile(p)
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		parserType := "custom"
		if err == nil {
			if prof.Name != "" {
				name = prof.Name
			}
			if prof.Parser.Type != "" {
				parserType = prof.Parser.Type
			}
		}

		isGlobal := false
		if appCfg != nil && appCfg.ProfilesDir != "" {
			absP, _ := filepath.Abs(p)
			absGlobal, _ := filepath.Abs(appCfg.ProfilesDir)
			if strings.HasPrefix(absP, absGlobal) {
				isGlobal = true
			}
		}

		list = append(list, ProfileInfo{
			Name:       name,
			Path:       p,
			IsGlobal:   isGlobal,
			ParserType: parserType,
		})
	}
	return list
}

// ImportProfile reads, validates, and installs a profile YAML into the user's OS profiles directory.
func ImportProfile(appCfg *AppConfig, sourcePath string) (targetPath string, profName string, err error) {
	if appCfg == nil || appCfg.ProfilesDir == "" {
		return "", "", fmt.Errorf("user profiles directory is not configured")
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("cannot read profile file: %w", err)
	}

	// Validate profile structure and parser build
	prof, err := parser.LoadProfile(sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("invalid profile syntax: %w", err)
	}
	if strings.TrimSpace(prof.Name) == "" {
		return "", "", fmt.Errorf("profile must specify a 'name'")
	}
	if _, err := prof.BuildParser(); err != nil {
		return "", "", fmt.Errorf("profile parser validation failed: %w", err)
	}

	if err := os.MkdirAll(appCfg.ProfilesDir, 0o755); err != nil {
		return "", "", fmt.Errorf("cannot create profiles directory: %w", err)
	}

	cleanName := strings.ToLower(strings.ReplaceAll(prof.Name, " ", "-"))
	targetPath = filepath.Join(appCfg.ProfilesDir, cleanName+".yaml")

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("cannot write profile to %q: %w", targetPath, err)
	}

	return targetPath, prof.Name, nil
}

// ExportProfile looks up a profile and returns its raw YAML contents and resolved path.
func ExportProfile(appCfg *AppConfig, nameOrPath string) (content []byte, sourcePath string, err error) {
	resolved := ResolveProfilePath(appCfg, nameOrPath)
	if resolved == "" {
		// Fallback check for raw or example files
		if strings.EqualFold(nameOrPath, "raw") {
			candidate := filepath.Join("profiles", "examples", "raw.yaml")
			if _, statErr := os.Stat(candidate); statErr == nil {
				resolved = candidate
			}
		}
	}
	if resolved == "" {
		return nil, "", fmt.Errorf("profile %q not found", nameOrPath)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read profile file %q: %w", resolved, err)
	}

	return data, resolved, nil
}
