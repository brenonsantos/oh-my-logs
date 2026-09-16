package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// InstallOptions configures the profile installer behavior.
type InstallOptions struct {
	Force   bool          // overwrite existing profiles without error
	Token   string        // explicit bearer/auth token for HTTP requests
	Timeout time.Duration // timeout for network downloads (default: 15s)
}

// InstallResult reports details of an installed profile.
type InstallResult struct {
	Name                string
	Source              string
	TargetPath          string
	CompanionDecoders   string
	DecodersInstalled   int
	Overwritten         bool
}

// InstallSource installs profiles from a local file, directory, HTTP/HTTPS URL, or Git URL.
func InstallSource(appCfg *AppConfig, source string, opts InstallOptions) ([]InstallResult, error) {
	if appCfg == nil || appCfg.ProfilesDir == "" {
		return nil, fmt.Errorf("global profiles directory is not configured")
	}

	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}

	// 1. Git repository URL (e.g., git@github... or https://...git)
	if isGitURL(trimmed) {
		return installFromGit(appCfg, trimmed, opts)
	}

	// 2. HTTP/HTTPS URL (raw YAML file)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return installFromHTTP(appCfg, trimmed, opts)
	}

	// 3. Local filesystem
	fi, err := os.Stat(trimmed)
	if err != nil {
		return nil, fmt.Errorf("cannot access source %q: %w", trimmed, err)
	}

	if fi.IsDir() {
		return installFromDirectory(appCfg, trimmed, opts)
	}

	return installFromFile(appCfg, trimmed, opts)
}

func isGitURL(src string) bool {
	if strings.HasPrefix(src, "git@") {
		return true
	}
	if strings.HasSuffix(src, ".git") {
		return true
	}
	// Support github.com / enterprise repos without explicit .git if not pointing to raw
	if (strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "http://")) &&
		strings.Contains(src, "github") && !strings.Contains(src, "/raw/") && !strings.HasSuffix(src, ".yaml") && !strings.HasSuffix(src, ".yml") {
		return true
	}
	return false
}

func installFromGit(appCfg *AppConfig, gitURL string, opts InstallOptions) ([]InstallResult, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "oml-git-clone-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone failed for %s: %s (%w)", gitURL, strings.TrimSpace(string(out)), err)
	}

	results, err := installFromDirectory(appCfg, tmpDir, opts)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Source = gitURL
	}
	return results, nil
}

func installFromHTTP(appCfg *AppConfig, rawURL string, opts InstallOptions) ([]InstallResult, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	req.Header.Set("User-Agent", "oh-my-logs/profile-installer")

	// Determine authentication token
	token := opts.Token
	if token == "" {
		for _, envKey := range []string{"GH_TOKEN", "GITHUB_TOKEN", "OML_TOKEN", "GHE_TOKEN"} {
			if val := os.Getenv(envKey); val != "" {
				token = val
				break
			}
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", rawURL, err)
	}

	prof, err := parser.ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("invalid profile syntax from %s: %w", rawURL, err)
	}
	if strings.TrimSpace(prof.Name) == "" {
		return nil, fmt.Errorf("profile from %s must specify a 'name'", rawURL)
	}
	if _, err := prof.BuildParser(); err != nil {
		return nil, fmt.Errorf("profile parser validation failed: %w", err)
	}

	cleanName := strings.ToLower(strings.ReplaceAll(prof.Name, " ", "-"))
	targetPath := filepath.Join(appCfg.ProfilesDir, cleanName+".yaml")

	overwritten := false
	if _, statErr := os.Stat(targetPath); statErr == nil {
		if !opts.Force {
			return nil, fmt.Errorf("profile %q already exists at %s (use --force to overwrite)", prof.Name, targetPath)
		}
		overwritten = true
	}

	if err := os.MkdirAll(appCfg.ProfilesDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create profiles directory: %w", err)
	}

	writeData := data
	if prof.Source == "" {
		writeData = []byte(fmt.Sprintf("source: %q\n%s", rawURL, string(data)))
	}

	if err := os.WriteFile(targetPath, writeData, 0o644); err != nil {
		return nil, fmt.Errorf("cannot write profile to %q: %w", targetPath, err)
	}

	return []InstallResult{{
		Name:        prof.Name,
		Source:      rawURL,
		TargetPath:  targetPath,
		Overwritten: overwritten,
	}}, nil
}

func installFromFile(appCfg *AppConfig, filePath string, opts InstallOptions) ([]InstallResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read profile file %q: %w", filePath, err)
	}

	prof, err := parser.ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("invalid profile syntax in %q: %w", filePath, err)
	}
	if strings.TrimSpace(prof.Name) == "" {
		return nil, fmt.Errorf("profile in %q must specify a 'name'", filePath)
	}
	if _, err := prof.BuildParser(); err != nil {
		return nil, fmt.Errorf("profile parser validation failed: %w", err)
	}

	cleanName := strings.ToLower(strings.ReplaceAll(prof.Name, " ", "-"))
	targetPath := filepath.Join(appCfg.ProfilesDir, cleanName+".yaml")

	overwritten := false
	if _, statErr := os.Stat(targetPath); statErr == nil {
		if !opts.Force {
			return nil, fmt.Errorf("profile %q already exists at %s (use --force to overwrite)", prof.Name, targetPath)
		}
		overwritten = true
	}

	if err := os.MkdirAll(appCfg.ProfilesDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create profiles directory: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("cannot write profile to %q: %w", targetPath, err)
	}

	// Check if adjacent companion decoders directory exists
	companionDir := ""
	decodersInstalled := 0
	if appCfg.DecodersDir != "" {
		decSrc := findCompanionDecoderDir(filePath, cleanName)
		if decSrc != "" {
			targetDecDir := filepath.Join(appCfg.DecodersDir, cleanName)
			count, copyErr := copyDirectory(decSrc, targetDecDir)
			if copyErr == nil && count > 0 {
				companionDir = targetDecDir
				decodersInstalled = count
			}
		}
	}

	return []InstallResult{{
		Name:              prof.Name,
		Source:            filePath,
		TargetPath:        targetPath,
		CompanionDecoders: companionDir,
		DecodersInstalled: decodersInstalled,
		Overwritten:       overwritten,
	}}, nil
}

func installFromDirectory(appCfg *AppConfig, dirPath string, opts InstallOptions) ([]InstallResult, error) {
	// Search for profile YAML files directly in dirPath and dirPath/profiles
	var candidates []string
	searchDirs := []string{dirPath, filepath.Join(dirPath, "profiles")}
	seen := make(map[string]bool)

	for _, d := range searchDirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				full := filepath.Join(d, name)
				abs, _ := filepath.Abs(full)
				if !seen[abs] {
					seen[abs] = true
					candidates = append(candidates, full)
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no profile YAML files found in directory %q", dirPath)
	}

	var results []InstallResult
	for _, cand := range candidates {
		res, err := installFromFile(appCfg, cand, opts)
		if err != nil {
			if !opts.Force && strings.Contains(err.Error(), "already exists") {
				return nil, err
			}
			continue
		}
		if len(res) > 0 {
			single := res[0]
			// If companion decoders weren't resolved by installFromFile, attempt bundle root search
			if single.CompanionDecoders == "" && appCfg.DecodersDir != "" {
				cleanName := strings.ToLower(strings.ReplaceAll(single.Name, " ", "-"))
				decSrc := findCompanionDecoderDir(dirPath, cleanName)
				if decSrc != "" {
					targetDecDir := filepath.Join(appCfg.DecodersDir, cleanName)
					count, copyErr := copyDirectory(decSrc, targetDecDir)
					if copyErr == nil && count > 0 {
						single.CompanionDecoders = targetDecDir
						single.DecodersInstalled = count
					}
				}
			}
			results = append(results, single)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to install any valid profiles from %q", dirPath)
	}

	return results, nil
}

// findCompanionDecoderDir searches for companion decoders matching cleanName.
// Priority:
// 1. decoders/<cleanName>/ (namespaced directory in repo or bundle)
// 2. <cleanName>/ (if search directory is already a decoders folder)
// 3. decoders/ (flat fallback ONLY if it contains no child subdirectories)
func findCompanionDecoderDir(contextPath, cleanName string) string {
	var searchBases []string

	fi, err := os.Stat(contextPath)
	if err == nil && fi.IsDir() {
		searchBases = append(searchBases, contextPath, filepath.Dir(contextPath))
	} else {
		parent := filepath.Dir(contextPath)
		grandParent := filepath.Dir(parent)
		searchBases = append(searchBases, parent, grandParent)
	}

	// 1. Check for decoders/<cleanName>
	for _, b := range searchBases {
		cand := filepath.Join(b, "decoders", cleanName)
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			return cand
		}
	}

	// 2. Check if a searchBase is already a "decoders" folder containing <cleanName>
	for _, b := range searchBases {
		if strings.EqualFold(filepath.Base(b), "decoders") {
			cand := filepath.Join(b, cleanName)
			if info, err := os.Stat(cand); err == nil && info.IsDir() {
				return cand
			}
		}
	}

	// 3. Fallback: flat decoders/ directory without child subdirectories
	for _, b := range searchBases {
		cand := filepath.Join(b, "decoders")
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			if !dirHasSubdirectories(cand) {
				return cand
			}
		}
	}

	return ""
}

func dirHasSubdirectories(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return true
		}
	}
	return false
}

func copyDirectory(src, dst string) (int, error) {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "__pycache__" || name == ".DS_Store" || strings.HasSuffix(name, ".pyc") {
			continue
		}
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if entry.IsDir() {
			subCount, err := copyDirectory(srcPath, dstPath)
			if err != nil {
				return count, err
			}
			count += subCount
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return count, err
		}

		perm := os.FileMode(0o644)
		info, err := entry.Info()
		if err == nil {
			// Retain executable bit if present
			if info.Mode()&0o111 != 0 || strings.HasSuffix(entry.Name(), ".sh") || strings.HasSuffix(entry.Name(), ".py") {
				perm = 0o755
			}
		}

		if err := os.WriteFile(dstPath, data, perm); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
