package opencode

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/retry"
)

const (
	// githubReleasesAPI is the endpoint for fetching the latest opencode release.
	githubReleasesAPI = "https://api.github.com/repos/anomalyco/opencode/releases/latest"

	// githubReleaseByTagAPI is the endpoint template for fetching a specific release.
	githubReleaseByTagAPI = "https://api.github.com/repos/anomalyco/opencode/releases/tags/v%s"

	// downloadTimeout is the maximum time allowed for downloading the binary.
	downloadTimeout = 5 * time.Minute

	// apiTimeout is the maximum time allowed for API calls.
	apiTimeout = 15 * time.Second

	// apiMaxRetries is the number of retry attempts for GitHub API calls.
	apiMaxRetries = 3

	// maxBinarySize is the upper bound for an extracted binary (200 MB).
	maxBinarySize = 200 * 1024 * 1024

	// maxDownloadSize is the upper bound for a downloaded archive (500 MB).
	maxDownloadSize = 500 * 1024 * 1024
)

// Release holds metadata from a GitHub release.
type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// Version returns the release version without the "v" prefix.
func (r *Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// ReleaseAsset holds metadata for a single release asset.
type ReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"` // "sha256:xxxx"
}

// SHA256 extracts the hex-encoded SHA256 from the digest field.
func (a *ReleaseAsset) SHA256() string {
	if strings.HasPrefix(a.Digest, "sha256:") {
		return strings.TrimPrefix(a.Digest, "sha256:")
	}
	return ""
}

// ProgressFunc is called during download with bytes downloaded and total size.
type ProgressFunc func(downloaded, total int64)

// LatestRelease fetches the latest opencode release metadata from GitHub.
func LatestRelease() (*Release, error) {
	return fetchRelease(githubReleasesAPI)
}

// ReleaseByVersion fetches a specific opencode release metadata from GitHub.
func ReleaseByVersion(version string) (*Release, error) {
	url := fmt.Sprintf(githubReleaseByTagAPI, version)
	return fetchRelease(url)
}

func fetchRelease(url string) (*Release, error) {
	client := &http.Client{Timeout: apiTimeout}
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "oh-cli")

	var release *Release
	retryErr := retry.Do(context.Background(), retry.Config{
		MaxAttempts: apiMaxRetries,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}, func(attempt int) (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			return true, fmt.Errorf("fetching release (attempt %d/%d): %w", attempt, apiMaxRetries, err)
		}

		// Rate limiting — respect Retry-After
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return true, fmt.Errorf("GitHub API rate limited (attempt %d/%d)", attempt, apiMaxRetries)
		}

		// Transient server errors
		if retry.IsTransientHTTP(resp.StatusCode) {
			resp.Body.Close()
			return true, fmt.Errorf("GitHub API returned %d (attempt %d/%d)", resp.StatusCode, attempt, apiMaxRetries)
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return false, fmt.Errorf("release not found")
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}

		var r Release
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return false, fmt.Errorf("decoding release: %w", err)
		}
		release = &r
		return false, nil
	})

	if retryErr != nil {
		return nil, retryErr
	}
	return release, nil
}

// Download downloads and installs a specific version of opencode.
// It downloads the archive, verifies the SHA256 checksum, extracts the binary,
// and installs it to the configured install directory.
// The progress function is called periodically with download progress.
func Download(version, installDir string, progress ProgressFunc) (string, error) {
	// Resolve release
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

	// Find the correct asset for our platform
	assetName, archiveFormat, err := AssetName()
	if err != nil {
		return "", err
	}

	var asset *ReleaseAsset
	for i := range release.Assets {
		if release.Assets[i].Name == assetName {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil {
		return "", fmt.Errorf("asset %q not found in release %s", assetName, release.TagName)
	}

	// Prepare install directory
	installDir = expandHome(installDir)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", fmt.Errorf("creating install directory: %w", err)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "opencode-download-*")
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

	// Verify checksum — mandatory for security
	expectedSHA := asset.SHA256()
	if expectedSHA == "" {
		os.Remove(tmpPath)
		return "", fmt.Errorf("no checksum available for %s — refusing to install unverified binary", asset.Name)
	}
	if err := verifyChecksum(tmpPath, expectedSHA); err != nil {
		return "", err
	}

	// Extract binary
	resolvedVersion := release.Version()
	binaryPath := filepath.Join(installDir, BinaryName+"-"+resolvedVersion)

	switch archiveFormat {
	case archiveFormatZip:
		err = extractFromZip(tmpPath, binaryPath)
	case archiveFormatTarGz:
		err = extractFromTarGz(tmpPath, binaryPath)
	default:
		err = fmt.Errorf("unsupported archive format: %s", archiveFormat)
	}
	if err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return "", fmt.Errorf("setting permissions: %w", err)
	}

	// Create/update symlink (or copy on Windows)
	symlinkPath := filepath.Join(installDir, BinaryName)
	_ = os.Remove(symlinkPath)
	if err := symlinkOrCopy(binaryPath, symlinkPath); err != nil {
		return "", fmt.Errorf("creating symlink: %w", err)
	}

	return binaryPath, nil
}

// InstalledVersion returns the version of the managed opencode binary, or empty string.
func InstalledVersion(installDir string) string {
	installDir = expandHome(installDir)
	symlinkPath := filepath.Join(installDir, BinaryName)

	target, err := readSymlink(symlinkPath)
	if err != nil {
		return ""
	}

	// Extract version from "opencode-1.17.13"
	base := filepath.Base(target)
	if strings.HasPrefix(base, BinaryName+"-") {
		return strings.TrimPrefix(base, BinaryName+"-")
	}
	return ""
}

func downloadAsset(asset *ReleaseAsset, dest *os.File, progress ProgressFunc) error {
	if asset.Size > maxDownloadSize {
		return fmt.Errorf("asset too large: %d bytes (max %d)", asset.Size, maxDownloadSize)
	}

	return retry.Do(context.Background(), retry.Config{
		MaxAttempts: apiMaxRetries,
		BaseDelay:   2 * time.Second,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}, func(attempt int) (bool, error) {
		// Reset file for retry
		if attempt > 1 {
			if _, err := dest.Seek(0, io.SeekStart); err != nil {
				return false, fmt.Errorf("resetting download file: %w", err)
			}
			if err := dest.Truncate(0); err != nil {
				return false, fmt.Errorf("truncating download file: %w", err)
			}
		}

		client := &http.Client{Timeout: downloadTimeout}
		resp, err := client.Get(asset.BrowserDownloadURL)
		if err != nil {
			return true, fmt.Errorf("downloading %s (attempt %d): %w", asset.Name, attempt, err)
		}
		defer resp.Body.Close()

		if retry.IsTransientHTTP(resp.StatusCode) {
			return true, fmt.Errorf("download %s: HTTP %d (attempt %d)", asset.Name, resp.StatusCode, attempt)
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		var reader io.Reader = io.LimitReader(resp.Body, maxDownloadSize)
		if progress != nil {
			reader = &progressReader{
				reader:   reader,
				total:    asset.Size,
				progress: progress,
			}
		}

		if _, err := io.Copy(dest, reader); err != nil {
			// Network errors during copy are transient
			return true, fmt.Errorf("writing download (attempt %d): %w", attempt, err)
		}
		return false, nil
	})
}

func verifyChecksum(filePath, expectedSHA256 string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s — le fichier est peut-être corrompu", expectedSHA256, actual)
	}
	return nil
}

func extractFromZip(archivePath, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	// Find the opencode binary (should be the only file, or named "opencode")
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Name != BinaryName && len(r.File) != 1 {
			continue
		}
		return extractZipEntry(f, destPath)
	}
	return fmt.Errorf("binary %q not found in zip archive", BinaryName)
}

func extractZipEntry(f *zip.File, destPath string) error {
	if f.UncompressedSize64 > maxBinarySize {
		return fmt.Errorf("zip entry too large: %d bytes (max %d)", f.UncompressedSize64, maxBinarySize)
	}

	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, io.LimitReader(src, maxBinarySize)); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}
	return nil
}

func extractFromTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening tar.gz: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Skip non-regular files (directories, symlinks, etc.)
		if header.Typeflag != tar.TypeReg {
			continue
		}
		// Match binary name (handle archives with directory prefix)
		if filepath.Base(header.Name) != BinaryName {
			continue
		}
		return extractTarEntry(tr, destPath)
	}
	return fmt.Errorf("binary %q not found in tar archive", BinaryName)
}

func extractTarEntry(tr *tar.Reader, destPath string) error {
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, io.LimitReader(tr, maxBinarySize)); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}
	return nil
}

// progressReader wraps an io.Reader and calls a progress function.
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

// EnsureInstalled checks if the opencode binary is available.
// If not found, it either prompts the user (interactive mode) or returns an error.
// Returns the path to the binary.
func EnsureInstalled(interactive bool) (string, error) {
	// Try to find existing binary
	path, err := FindBinary()
	if err == nil {
		return path, nil
	}

	if !interactive {
		return "", fmt.Errorf("opencode non trouvé. Installez-le avec:\n  brew install anomalyco/tap/opencode\n  ou: oh upgrade opencode")
	}

	// In interactive mode, we return the error — the caller (cmd/start.go)
	// will handle the prompt using huh (to avoid importing TUI libs here).
	return "", err
}
