package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// ProfileUpdateCheck holds the comparison between local and remote versions.
type ProfileUpdateCheck struct {
	Name           string
	Path           string
	Source         string
	CurrentVersion string
	RemoteVersion  string
	HasUpdate      bool
	Error          error
}

// ProfileUpdateResult reports the outcome of updating a profile.
type ProfileUpdateResult struct {
	Name             string
	Source           string
	PreviousVersion  string
	NewVersion       string
	Updated          bool
	DecodersUpdated  int
	Error            error
}

// IsNewerVersion compares current and candidate versions.
// Returns true if candidate is strictly newer than current.
func IsNewerVersion(current, candidate string) bool {
	cur := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(current, "v"), "V"))
	cand := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(candidate, "v"), "V"))

	if cand == "" {
		return false
	}
	if cur == "" {
		return true // Any version is newer than no version
	}

	curParts := strings.Split(cur, ".")
	candParts := strings.Split(cand, ".")

	maxLen := len(curParts)
	if len(candParts) > maxLen {
		maxLen = len(candParts)
	}

	for i := 0; i < maxLen; i++ {
		var curNum, candNum int
		if i < len(curParts) {
			curNum, _ = strconv.Atoi(strings.TrimRight(curParts[i], "-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		}
		if i < len(candParts) {
			candNum, _ = strconv.Atoi(strings.TrimRight(candParts[i], "-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		}

		if candNum > curNum {
			return true
		}
		if candNum < curNum {
			return false
		}
	}

	return false
}

// CheckProfileUpdate checks if a newer version of the specified profile exists remotely.
func CheckProfileUpdate(appCfg *AppConfig, idOrName string, opts InstallOptions) (*ProfileUpdateCheck, error) {
	info, err := ResolveProfile(appCfg, idOrName)
	if err != nil {
		return nil, err
	}

	if !info.IsGlobal {
		return nil, fmt.Errorf("profile %q is a %s profile; only global profiles can be updated", info.Name, info.Scope)
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read profile file %q: %w", info.Path, err)
	}

	prof, err := parser.ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse profile file %q: %w", info.Path, err)
	}

	source := prof.Source
	if source == "" {
		return &ProfileUpdateCheck{
			Name:           prof.Name,
			Path:           info.Path,
			CurrentVersion: prof.Version,
			Error:          fmt.Errorf("no remote source URL recorded for profile %q", prof.Name),
		}, nil
	}

	// Fetch remote profile metadata without installing
	remoteResults, err := InstallSource(appCfg, source, InstallOptions{
		Force:   true,
		Token:   opts.Token,
		Timeout: opts.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote source %s: %w", source, err)
	}

	if len(remoteResults) == 0 {
		return nil, fmt.Errorf("no valid profile found at %s", source)
	}

	remoteData, _ := os.ReadFile(remoteResults[0].TargetPath)
	remoteProf, _ := parser.ParseProfile(remoteData)
	remoteVer := ""
	if remoteProf != nil {
		remoteVer = remoteProf.Version
	}

	hasUpdate := IsNewerVersion(prof.Version, remoteVer)

	return &ProfileUpdateCheck{
		Name:           prof.Name,
		Path:           info.Path,
		Source:         source,
		CurrentVersion: prof.Version,
		RemoteVersion:  remoteVer,
		HasUpdate:      hasUpdate,
	}, nil
}

// UpdateProfile updates a single profile from its source if a newer version exists (or if Force is set).
func UpdateProfile(appCfg *AppConfig, idOrName string, opts InstallOptions) (*ProfileUpdateResult, error) {
	info, err := ResolveProfile(appCfg, idOrName)
	if err != nil {
		return nil, err
	}

	if !info.IsGlobal {
		return nil, fmt.Errorf("cannot update profile %q: it is a %s profile (%s)", info.Name, info.Scope, info.Path)
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read profile file %q: %w", info.Path, err)
	}

	prof, err := parser.ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse profile %q: %w", info.Path, err)
	}

	source := prof.Source
	if source == "" {
		return nil, fmt.Errorf("profile %q has no remote source URL configured", prof.Name)
	}

	prevVersion := prof.Version

	installOpts := opts
	installOpts.Force = true // Overwrite current profile
	results, err := InstallSource(appCfg, source, installOpts)
	if err != nil {
		return nil, fmt.Errorf("update failed for %s: %w", source, err)
	}

	newVersion := prevVersion
	decodersCount := 0
	if len(results) > 0 {
		newData, _ := os.ReadFile(results[0].TargetPath)
		if newProf, _ := parser.ParseProfile(newData); newProf != nil && newProf.Version != "" {
			newVersion = newProf.Version
		}
		decodersCount = results[0].DecodersInstalled
	}

	return &ProfileUpdateResult{
		Name:            prof.Name,
		Source:          source,
		PreviousVersion: prevVersion,
		NewVersion:      newVersion,
		Updated:         true,
		DecodersUpdated: decodersCount,
	}, nil
}

// UpdateAllProfiles checks and updates all global profiles that have a remote source configured.
func UpdateAllProfiles(appCfg *AppConfig, opts InstallOptions) ([]ProfileUpdateResult, error) {
	if appCfg == nil || appCfg.ProfilesDir == "" {
		return nil, fmt.Errorf("global profiles directory not configured")
	}

	profiles := ListProfiles(appCfg)
	var results []ProfileUpdateResult

	for _, p := range profiles {
		if !p.IsGlobal || p.Source == "" {
			continue
		}
		res, err := UpdateProfile(appCfg, p.Name, opts)
		if err != nil {
			results = append(results, ProfileUpdateResult{
				Name:   p.Name,
				Source: p.Source,
				Error:  err,
			})
			continue
		}
		results = append(results, *res)
	}

	return results, nil
}
