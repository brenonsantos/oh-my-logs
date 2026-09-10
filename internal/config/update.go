package config

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultLatestReleaseURL is the GitHub API endpoint for the latest release.
var DefaultLatestReleaseURL = "https://api.github.com/repos/brenonsantos/oh-my-logs/releases/latest"

// ReleaseAsset represents an artifact asset attached to a GitHub release.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ReleaseInfo holds the metadata of a release returned by GitHub API.
type ReleaseInfo struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Assets  []ReleaseAsset `json:"assets"`
	HTMLURL string         `json:"html_url"`
}

// FindMatchingAsset finds the asset corresponding to the provided operating system and architecture.
func FindMatchingAsset(assets []ReleaseAsset, goos, goarch string) (*ReleaseAsset, error) {
	suffix := fmt.Sprintf("_%s_%s.tar.gz", goos, goarch)
	if goos == "windows" {
		suffix = fmt.Sprintf("_%s_%s.zip", goos, goarch)
	}

	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("no release asset found matching *_%s_%s", goos, goarch)
}

// FetchLatestRelease queries the GitHub API for latest release metadata.
func FetchLatestRelease(apiURL, userAgent string) (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d (%s)", resp.StatusCode, resp.Status)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode release JSON: %w", err)
	}

	return &info, nil
}

// ExtractBinaryFromArchive extracts the 'oml' or 'oml.exe' binary bytes from archive data.
func ExtractBinaryFromArchive(archiveData []byte, goos string) ([]byte, error) {
	binaryName := "oml"
	if goos == "windows" {
		binaryName = "oml.exe"
	}

	if goos == "windows" {
		zipReader, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
		if err != nil {
			return nil, fmt.Errorf("failed opening zip archive: %w", err)
		}
		for _, f := range zipReader.File {
			if filepath.Base(f.Name) == binaryName {
				rc, err := f.Open()
				if err != nil {
					return nil, fmt.Errorf("failed opening %s inside zip: %w", f.Name, err)
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("binary %q not found in zip archive", binaryName)
	}

	// Unix / macOS (.tar.gz)
	gzReader, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return nil, fmt.Errorf("failed opening gzip stream: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binaryName {
			return io.ReadAll(tarReader)
		}
	}

	return nil, fmt.Errorf("binary %q not found in tar.gz archive", binaryName)
}

// UpdateBinary checks for a newer release on GitHub and in-place updates the current executable.
func UpdateBinary(appCfg *AppConfig, currentVersion string) (string, error) {
	userAgent := fmt.Sprintf("oh-my-logs/%s (%s; %s)", currentVersion, runtime.GOOS, runtime.GOARCH)
	release, err := FetchLatestRelease(DefaultLatestReleaseURL, userAgent)
	if err != nil {
		return "", fmt.Errorf("failed checking for updates: %w", err)
	}

	cleanCurrent := strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
	cleanLatest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")

	if cleanCurrent == cleanLatest && cleanCurrent != "" {
		return fmt.Sprintf("oml is already up to date (%s)", release.TagName), nil
	}

	asset, err := FindMatchingAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", fmt.Errorf("release %s has no matching artifact: %w", release.TagName, err)
	}

	// Download archive
	client := &http.Client{Timeout: 60 * time.Second}
	dlReq, err := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed creating download request: %w", err)
	}
	dlReq.Header.Set("User-Agent", userAgent)

	dlResp, err := client.Do(dlReq)
	if err != nil {
		return "", fmt.Errorf("failed downloading %s: %w", asset.Name, err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d (%s)", dlResp.StatusCode, dlResp.Status)
	}

	archiveBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed reading downloaded asset: %w", err)
	}

	binaryBytes, err := ExtractBinaryFromArchive(archiveBytes, runtime.GOOS)
	if err != nil {
		return "", fmt.Errorf("failed extracting archive: %w", err)
	}

	// Determine destination path to replace
	destPath, err := os.Executable()
	if err != nil {
		defDir, defErr := DefaultInstallDir()
		if defErr != nil {
			return "", fmt.Errorf("cannot determine executable or install path: %w", defErr)
		}
		binaryName := "oml"
		if runtime.GOOS == "windows" {
			binaryName = "oml.exe"
		}
		destPath = filepath.Join(defDir, binaryName)
	} else {
		if realPath, err := filepath.EvalSymlinks(destPath); err == nil {
			destPath = realPath
		}
	}

	// Atomic replacement: write to temp file then rename
	tempDest := destPath + fmt.Sprintf(".tmp_%d", os.Getpid())
	if err := os.WriteFile(tempDest, binaryBytes, 0o755); err != nil {
		if os.IsPermission(err) {
			if runtime.GOOS == "windows" {
				return "", fmt.Errorf("permission denied writing to %s (please run PowerShell as Administrator)", destPath)
			}
			return "", fmt.Errorf("permission denied writing to %s (please run: sudo oml --update)", destPath)
		}
		return "", fmt.Errorf("cannot write updated binary to temporary file %s: %w", tempDest, err)
	}

	// Windows locked binary handling
	if runtime.GOOS == "windows" {
		oldDest := destPath + ".old"
		_ = os.Remove(oldDest)
		_ = os.Rename(destPath, oldDest)
	}

	if err := os.Rename(tempDest, destPath); err != nil {
		_ = os.Remove(tempDest)
		if os.IsPermission(err) {
			if runtime.GOOS == "windows" {
				return "", fmt.Errorf("permission denied updating %s (please run PowerShell as Administrator)", destPath)
			}
			return "", fmt.Errorf("permission denied updating %s (please run: sudo oml --update)", destPath)
		}
		return "", fmt.Errorf("cannot replace binary at %s: %w", destPath, err)
	}

	_ = os.Chmod(destPath, 0o755)

	// Update example profiles if present
	if appCfg != nil && appCfg.ProfilesDir != "" {
		CopyDefaultProfiles(appCfg.ProfilesDir)
	}

	return fmt.Sprintf("✓ Successfully updated oml from v%s to %s at:\n  %s", cleanCurrent, release.TagName, destPath), nil
}
