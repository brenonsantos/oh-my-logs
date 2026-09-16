package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// ProfileInfo holds metadata about a discovered profile.
type ProfileInfo struct {
	ID            int
	Name          string
	Version       string
	Source        string
	Path          string
	IsGlobal      bool
	Scope         string // "global", "local", "builtin"
	ParserType    string
	DecodersCount int
	HasCompanion  bool // true if companion decoders exist in DecodersDir
}

// ListProfiles returns metadata for all discovered profiles, numbered 1..N.
func ListProfiles(appCfg *AppConfig) []ProfileInfo {
	paths := FindAllProfilePaths(appCfg)
	var list []ProfileInfo
	for _, p := range paths {
		prof, err := parser.LoadProfile(p)
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		parserType := "custom"
		decodersCount := 0
		version := ""
		source := ""
		if err == nil {
			if prof.Name != "" {
				name = prof.Name
			}
			if prof.Version != "" {
				version = prof.Version
			}
			if prof.Source != "" {
				source = prof.Source
			}
			if prof.Parser.Type != "" {
				parserType = prof.Parser.Type
			}
			decodersCount = len(prof.Decoders)
		}

		isGlobal := false
		scope := "local"
		if appCfg != nil && appCfg.ProfilesDir != "" {
			absP, _ := filepath.Abs(p)
			absGlobal, _ := filepath.Abs(appCfg.ProfilesDir)
			if strings.HasPrefix(absP, absGlobal) {
				isGlobal = true
				scope = "global"
			}
		}

		if !isGlobal {
			lowerP := strings.ToLower(filepath.ToSlash(p))
			if strings.Contains(lowerP, "profiles/examples/") || strings.Contains(lowerP, "examples/profiles/") {
				scope = "builtin"
			}
		}

		hasCompanion := false
		if appCfg != nil && appCfg.DecodersDir != "" {
			cleanName := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			decDir := filepath.Join(appCfg.DecodersDir, cleanName)
			if fi, statErr := os.Stat(decDir); statErr == nil && fi.IsDir() {
				hasCompanion = true
			}
		}

		list = append(list, ProfileInfo{
			ID:            len(list) + 1,
			Name:          name,
			Version:       version,
			Source:        source,
			Path:          p,
			IsGlobal:      isGlobal,
			Scope:         scope,
			ParserType:    parserType,
			DecodersCount: decodersCount,
			HasCompanion:  hasCompanion,
		})
	}
	return list
}

// ResolveProfile resolves a profile by numeric ID (#1, 1), name, or filename.
func ResolveProfile(appCfg *AppConfig, idOrName string) (*ProfileInfo, error) {
	trimmed := strings.TrimSpace(idOrName)
	if trimmed == "" {
		return nil, fmt.Errorf("profile identifier cannot be empty")
	}

	profiles := ListProfiles(appCfg)

	// 1. Check numeric ID (e.g. "1", "#1")
	cleanID := strings.TrimPrefix(trimmed, "#")
	if num, err := strconv.Atoi(cleanID); err == nil && num >= 1 && num <= len(profiles) {
		p := profiles[num-1]
		return &p, nil
	}

	// 2. Exact match on profile Name (case-insensitive)
	for _, p := range profiles {
		if strings.EqualFold(p.Name, trimmed) {
			return &p, nil
		}
	}

	// 3. Match on filename or slug without extension
	for _, p := range profiles {
		base := strings.TrimSuffix(filepath.Base(p.Path), filepath.Ext(p.Path))
		if strings.EqualFold(base, trimmed) {
			return &p, nil
		}
	}

	// 4. Direct existing file path match
	if fi, err := os.Stat(trimmed); err == nil && !fi.IsDir() {
		for _, p := range profiles {
			absP, _ := filepath.Abs(p.Path)
			absTrimmed, _ := filepath.Abs(trimmed)
			if absP == absTrimmed {
				return &p, nil
			}
		}
		// If not in standard list but valid file on disk
		prof, err := parser.LoadProfile(trimmed)
		name := trimmed
		parserType := "custom"
		decodersCount := 0
		if err == nil {
			if prof.Name != "" {
				name = prof.Name
			}
			if prof.Parser.Type != "" {
				parserType = prof.Parser.Type
			}
			decodersCount = len(prof.Decoders)
		}
		return &ProfileInfo{
			ID:            0,
			Name:          name,
			Path:          trimmed,
			IsGlobal:      false,
			Scope:         "file",
			ParserType:    parserType,
			DecodersCount: decodersCount,
		}, nil
	}

	return nil, fmt.Errorf("profile %q not found (run 'oml profile list' to view available profiles)", trimmed)
}

// UninstallProfile removes a globally installed profile and its companion decoders.
func UninstallProfile(appCfg *AppConfig, idOrName string, removeDecoders bool) (*ProfileInfo, error) {
	info, err := ResolveProfile(appCfg, idOrName)
	if err != nil {
		return nil, err
	}

	if !info.IsGlobal {
		return nil, fmt.Errorf("cannot uninstall profile %q: it is a %s profile (%s)", info.Name, info.Scope, info.Path)
	}

	if err := os.Remove(info.Path); err != nil {
		return nil, fmt.Errorf("failed to remove profile file %q: %w", info.Path, err)
	}

	if removeDecoders && appCfg != nil && appCfg.DecodersDir != "" {
		cleanName := strings.ToLower(strings.ReplaceAll(info.Name, " ", "-"))
		decDir := filepath.Join(appCfg.DecodersDir, cleanName)
		if fi, statErr := os.Stat(decDir); statErr == nil && fi.IsDir() {
			_ = os.RemoveAll(decDir)
		}
	}

	return info, nil
}

// ProfileInspection holds structured inspection details for a profile.
type ProfileInspection struct {
	Info           ProfileInfo
	Profile        *parser.Profile
	RawYAML        string
	FilePath       string
	CompanionDir   string
	CompanionFiles []string
}

// InspectProfile loads and returns complete profile details for display.
func InspectProfile(appCfg *AppConfig, idOrName string) (*ProfileInspection, error) {
	info, err := ResolveProfile(appCfg, idOrName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read profile file %q: %w", info.Path, err)
	}

	prof, err := parser.ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse profile %q: %w", info.Path, err)
	}

	companionDir := ""
	var companionFiles []string
	if appCfg != nil && appCfg.DecodersDir != "" {
		cleanName := strings.ToLower(strings.ReplaceAll(info.Name, " ", "-"))
		decDir := filepath.Join(appCfg.DecodersDir, cleanName)
		if fi, statErr := os.Stat(decDir); statErr == nil && fi.IsDir() {
			companionDir = decDir
			if entries, err := os.ReadDir(decDir); err == nil {
				for _, e := range entries {
					companionFiles = append(companionFiles, e.Name())
				}
			}
		}
	}

	return &ProfileInspection{
		Info:           *info,
		Profile:        prof,
		RawYAML:        string(data),
		FilePath:       info.Path,
		CompanionDir:   companionDir,
		CompanionFiles: companionFiles,
	}, nil
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
func ExportProfile(appCfg *AppConfig, idOrName string) (content []byte, sourcePath string, err error) {
	info, err := ResolveProfile(appCfg, idOrName)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read profile file %q: %w", info.Path, err)
	}

	return data, info.Path, nil
}
