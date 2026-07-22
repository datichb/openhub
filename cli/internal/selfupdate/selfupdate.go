// Package selfupdate handles in-place updates of the oh binary itself.
package selfupdate

import (
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

const (
	ohReleasesAPI    = "https://api.github.com/repos/datichb/openhub/releases/latest"
	ohReleaseTagAPI  = "https://api.github.com/repos/datichb/openhub/releases/tags/v%s"
	ohAPITimeout     = 15 * time.Second
	ohDownloadTimeout = 5 * time.Minute
	ohMaxRetries     = 3
)

// Release holds metadata from a GitHub release of oh.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Version returns the release version without the "v" prefix.
func (r *Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// Asset holds metadata for a single release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// LatestRelease fetches the latest oh release metadata from GitHub.
func LatestRelease() (*Release, error) {
	return fetchRelease(ohReleasesAPI)
}

// ReleaseByVersion fetches a specific oh release by version string.
func ReleaseByVersion(version string) (*Release, error) {
	return fetchRelease(fmt.Sprintf(ohReleaseTagAPI, strings.TrimPrefix(version, "v")))
}

func fetchRelease(url string) (*Release, error) {
	client := &http.Client{Timeout: ohAPITimeout}
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "oh-cli-selfupdate")

	var lastErr error
	for attempt := 1; attempt <= ohMaxRetries; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetching release (attempt %d/%d): %w", attempt, ohMaxRetries, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("GitHub API returned %d (attempt %d/%d)", resp.StatusCode, attempt, ohMaxRetries)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("release not found")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
		}
		var r Release
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding release: %w", err)
		}
		return &r, nil
	}
	return nil, lastErr
}

// AssetName returns the expected archive name for the current platform.
func AssetName() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
	}
	arch, ok := archMap[goarch]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	switch goos {
	case "darwin", "linux":
		return fmt.Sprintf("openhub_%s_%s.tar.gz", goos, arch), nil
	default:
		return "", fmt.Errorf("unsupported OS for self-update: %s (use Homebrew or install.sh)", goos)
	}
}

// ProgressFunc is called during download with bytes downloaded and total.
type ProgressFunc func(downloaded, total int64)

// Update downloads the specified version of oh and replaces the running binary.
// If version is empty or "latest", fetches the latest release.
// Returns the new binary path on success.
func Update(version string, progress ProgressFunc) (string, error) {
	var release *Release
	var err error
	if version == "" || version == "latest" {
		release, err = LatestRelease()
	} else {
		release, err = ReleaseByVersion(version)
	}
	if err != nil {
		return "", fmt.Errorf("fetching release info: %w", err)
	}

	assetName, err := AssetName()
	if err != nil {
		return "", err
	}

	var asset *Asset
	for i := range release.Assets {
		if release.Assets[i].Name == assetName {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil {
		return "", fmt.Errorf("asset %q not found in release %s", assetName, release.TagName)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "oh-selfupdate-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := downloadAsset(asset, tmpFile, progress); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Extract binary from archive
	extractedPath := tmpPath + "-bin"
	if err := extractBinary(tmpPath, extractedPath); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}
	defer os.Remove(extractedPath)

	if err := os.Chmod(extractedPath, 0o755); err != nil {
		return "", fmt.Errorf("setting permissions: %w", err)
	}

	// Find current binary path
	currentBin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding current binary: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}

	// Atomic replace: rename current → .old, new → current
	oldPath := currentBin + ".old"
	if err := os.Rename(currentBin, oldPath); err != nil {
		return "", fmt.Errorf("backing up current binary: %w", err)
	}

	if err := os.Rename(extractedPath, currentBin); err != nil {
		// Attempt to restore
		_ = os.Rename(oldPath, currentBin)
		return "", fmt.Errorf("installing new binary: %w", err)
	}

	// Remove old backup
	_ = os.Remove(oldPath)

	return currentBin, nil
}

func downloadAsset(asset *Asset, dest *os.File, progress ProgressFunc) error {
	client := &http.Client{Timeout: ohDownloadTimeout}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    asset.Size,
			progress: progress,
		}
	}

	if _, err := io.Copy(dest, reader); err != nil {
		return fmt.Errorf("writing download: %w", err)
	}
	return nil
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progress   ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.progress != nil {
		pr.progress(pr.downloaded, pr.total)
	}
	return n, err
}

func extractBinary(archivePath, destPath string) error {
	// Determine format from extension
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractFromTarGz(archivePath, destPath)
	}
	return fmt.Errorf("unsupported archive format for self-update")
}
