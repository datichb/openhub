// Package selfupdate handles in-place updates of the oh binary itself.
package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/retry"
	"github.com/datichb/openhub/cli/internal/semver"
)

// GitHub API URLs of the releases (variables for the tests).
var (
	ohReleasesAPI   = "https://api.github.com/repos/datichb/openhub/releases/latest"
	ohReleaseTagAPI = "https://api.github.com/repos/datichb/openhub/releases/tags/v%s"
)

const (
	ohAPITimeout      = 15 * time.Second
	ohDownloadTimeout = 5 * time.Minute
	ohMaxRetries      = 3

	// maxDownloadSize is the upper bound for a downloaded archive (500 MB).
	maxDownloadSize = 500 * 1024 * 1024

	// maxAPIResponseSize is the upper bound for a GitHub API response (1 MB).
	maxAPIResponseSize = 1 * 1024 * 1024

	// maxChecksumFileSize is the upper bound for checksums.txt (10 KB).
	maxChecksumFileSize = 10 * 1024

	// checksumAssetName is the name of the checksums file in each release.
	checksumAssetName = "checksums.txt"

	// signatureBundleSuffix is the suffix appended to checksums.txt for the cosign bundle.
	signatureBundleSuffix = ".sigstore.json"
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

func fetchRelease(apiURL string) (*Release, error) {
	client := newHTTPClient(ohAPITimeout)
	req, err := http.NewRequest("GET", apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "oh-cli-selfupdate")

	var release *Release
	retryErr := retry.Do(context.Background(), retry.Config{
		MaxAttempts: ohMaxRetries,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}, func(attempt int) (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			return !errors.Is(err, ErrRedirect), fmt.Errorf("fetching release (attempt %d/%d): %w", attempt, ohMaxRetries, err)
		}
		if retry.IsTransientHTTP(resp.StatusCode) || resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			return true, fmt.Errorf("GitHub API returned %d (attempt %d/%d)", resp.StatusCode, attempt, ohMaxRetries)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return false, fmt.Errorf("release not found")
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
		}
		var r Release
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxAPIResponseSize)).Decode(&r); err != nil {
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

// Option changes the behaviour of Update.
type Option func(*updateOptions)

type updateOptions struct{ allowDowngrade bool }

// AllowDowngrade lets Update install a version older than (or equal to) the
// running one: the caller got an explicit request for it (S-39).
func AllowDowngrade() Option { return func(o *updateOptions) { o.allowDowngrade = true } }

// currentVersion is the running version (variable for the tests).
var currentVersion = func() string { return buildinfo.Version }

// ErrDowngrade is returned when a release is not newer than the running oh
// and nothing explicit asked for it.
var ErrDowngrade = errors.New("not a newer version")

// checkVersionChange refuses an update that would not go forward (S-39):
// a release not newer than current, which is what a replayed or forged API
// answer would offer; and, when a version was requested, a release that is not
// that version. A development build ("dev", unparsable) has nothing to
// compare with.
func checkVersionChange(current string, release *Release, requested string, opts updateOptions) error {
	requested = strings.TrimPrefix(requested, "v")
	if requested != "" && requested != "latest" && release.Version() != requested {
		return fmt.Errorf("release %s is not the requested version %s", release.TagName, requested)
	}
	if opts.allowDowngrade || current == "" || current == "dev" {
		return nil
	}
	cmp, ok := semver.Compare(current, release.Version())
	if !ok {
		if strings.TrimPrefix(current, "v") == release.Version() {
			cmp, ok = 0, true
		} else {
			return nil
		}
	}
	if cmp >= 0 {
		return fmt.Errorf("%w: release %s is not newer than the running version %s", ErrDowngrade, release.TagName, current)
	}
	return nil
}

// Update downloads the specified version of oh and replaces the running binary.
// If version is empty or "latest", fetches the latest release. A release that
// is not newer than the running version is refused, unless AllowDowngrade is
// given. Returns the new binary path on success.
func Update(version string, progress ProgressFunc, opts ...Option) (string, error) {
	var o updateOptions
	for _, opt := range opts {
		opt(&o)
	}
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
	if err := checkVersionChange(currentVersion(), release, version, o); err != nil {
		return "", err
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

	// Validate download URL points to a trusted host
	if err := validateDownloadURL(asset.BrowserDownloadURL); err != nil {
		return "", fmt.Errorf("untrusted download URL: %w", err)
	}

	// Find checksums asset for integrity verification
	var checksumAsset *Asset
	for i := range release.Assets {
		if release.Assets[i].Name == checksumAssetName {
			checksumAsset = &release.Assets[i]
			break
		}
	}
	if checksumAsset == nil {
		return "", fmt.Errorf("checksums file %q not found in release %s — refusing to install unverified binary", checksumAssetName, release.TagName)
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

	// Download the checksums, verify their signature, then the archive
	sums, err := fetchChecksums(checksumAsset)
	if err != nil {
		return "", fmt.Errorf("fetching checksums: %w", err)
	}
	var sigBundle *Asset
	for i := range release.Assets {
		if release.Assets[i].Name == checksumAssetName+signatureBundleSuffix {
			sigBundle = &release.Assets[i]
			break
		}
	}
	if err := verifySignature(release, sums, sigBundle); err != nil {
		return "", err
	}
	expectedHash, err := parseChecksums(sums, assetName)
	if err != nil {
		return "", err
	}
	if err := verifyChecksum(tmpPath, expectedHash); err != nil {
		return "", err
	}

	// Extract binary from archive
	extractedPath := tmpPath + "-bin"
	if err := extractBinary(tmpPath, extractedPath); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}
	defer os.Remove(extractedPath)

	// Find current binary path
	currentBin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding current binary: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}

	// Atomic replace: stage the new binary in the same directory as
	// currentBin so that os.Rename is a same-filesystem rename (atomic
	// on POSIX). A crash between the backup rename and the final rename
	// previously left no binary at currentBin; this approach keeps the
	// current binary in place until the single atomic rename succeeds.
	binDir := filepath.Dir(currentBin)
	staged, err := os.CreateTemp(binDir, ".oh-update-*")
	if err != nil {
		return "", fmt.Errorf("creating staged binary: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath) // cleanup on any failure path

	src, err := os.Open(extractedPath)
	if err != nil {
		staged.Close()
		return "", fmt.Errorf("opening extracted binary: %w", err)
	}
	if _, err := io.Copy(staged, src); err != nil {
		src.Close()
		staged.Close()
		return "", fmt.Errorf("staging new binary: %w", err)
	}
	src.Close()
	staged.Close()

	if err := os.Chmod(stagedPath, 0o755); err != nil {
		return "", fmt.Errorf("setting staged binary permissions: %w", err)
	}

	// Best-effort backup of current binary for manual rollback.
	oldPath := currentBin + ".old"
	_ = os.Remove(oldPath)
	_ = os.Rename(currentBin, oldPath)

	// Single atomic rename (same filesystem guaranteed).
	if err := os.Rename(stagedPath, currentBin); err != nil {
		// Attempt to restore the old binary
		if rbErr := os.Rename(oldPath, currentBin); rbErr != nil {
			return "", fmt.Errorf("installing new binary: %w (rollback also failed: %v — your previous binary is at %s)", err, rbErr, oldPath)
		}
		return "", fmt.Errorf("installing new binary: %w", err)
	}

	// Remove old backup
	_ = os.Remove(oldPath)

	return currentBin, nil
}

func downloadAsset(asset *Asset, dest *os.File, progress ProgressFunc) error {
	return retry.Do(context.Background(), retry.Config{
		MaxAttempts: ohMaxRetries,
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

		client := newHTTPClient(ohDownloadTimeout)
		resp, err := client.Get(asset.BrowserDownloadURL)
		if err != nil {
			return !errors.Is(err, ErrRedirect), fmt.Errorf("downloading %s (attempt %d): %w", asset.Name, attempt, err)
		}
		defer resp.Body.Close()

		if retry.IsTransientHTTP(resp.StatusCode) {
			return true, fmt.Errorf("download %s: HTTP %d (attempt %d)", asset.Name, resp.StatusCode, attempt)
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		reader := io.LimitReader(resp.Body, maxDownloadSize)
		if progress != nil {
			reader = &progressReader{
				reader:   reader,
				total:    asset.Size,
				progress: progress,
			}
		}

		if _, err := io.Copy(dest, reader); err != nil {
			return true, fmt.Errorf("writing download (attempt %d): %w", attempt, err)
		}
		return false, nil
	})
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
	// oh releases are always tar.gz archives (see AssetName).
	// The archivePath may be a temp file without a .tar.gz extension
	// (e.g. from os.CreateTemp), so we extract directly instead of
	// relying on the filename suffix.
	return extractFromTarGz(archivePath, destPath)
}

// ErrRedirect is returned when a redirect leaves the hosts oh trusts (S-39).
var ErrRedirect = errors.New("redirect refused")

// maxRedirects bounds the redirects of one request.
const maxRedirects = 5

// allowedRedirectHosts are the hosts a request may be redirected to: GitHub
// and the CDN serving its release assets (github.com answers with a redirect
// to one of them; GitHub moved the assets to release-assets.* in 2025).
var allowedRedirectHosts = []string{
	"github.com",
	"api.github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
	"github-releases.githubusercontent.com",
}

// checkRedirect lets a request follow a redirect to HTTPS (default port) on
// an allowed host only.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: more than %d redirects", ErrRedirect, maxRedirects)
	}
	u := req.URL
	if u.Scheme != "https" {
		return fmt.Errorf("%w: %q is not HTTPS", ErrRedirect, u.Redacted())
	}
	if p := u.Port(); p != "" && p != "443" {
		return fmt.Errorf("%w: port %s of %q", ErrRedirect, p, u.Hostname())
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range allowedRedirectHosts {
		if host == h {
			return nil
		}
	}
	return fmt.Errorf("%w: untrusted host %q", ErrRedirect, u.Hostname())
}

// httpTransport is the transport of the update requests (nil = default;
// variable for the tests).
var httpTransport http.RoundTripper

// newHTTPClient is the logged client of the update requests, which follows
// redirects to the allowed hosts only.
func newHTTPClient(timeout time.Duration) *http.Client {
	return httplog.Wrap(&http.Client{Timeout: timeout, Transport: httpTransport, CheckRedirect: checkRedirect}, "selfupdate")
}

// allowedDownloadHosts is the set of hosts trusted for binary downloads.
var allowedDownloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
}

// validateDownloadURL checks that a download URL uses HTTPS and points to a trusted host.
func validateDownloadURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("insecure scheme %q — only HTTPS is allowed", u.Scheme)
	}
	for _, h := range allowedDownloadHosts {
		if u.Host == h || strings.HasSuffix(u.Host, "."+h) {
			return nil
		}
	}
	return fmt.Errorf("untrusted host %q — expected github.com or objects.githubusercontent.com", u.Host)
}

// fetchChecksums downloads the checksums file of a release.
func fetchChecksums(checksumAsset *Asset) ([]byte, error) {
	if err := validateDownloadURL(checksumAsset.BrowserDownloadURL); err != nil {
		return nil, fmt.Errorf("untrusted checksums URL: %w", err)
	}

	var body []byte
	retryErr := retry.Do(context.Background(), retry.Config{
		MaxAttempts: ohMaxRetries,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}, func(attempt int) (bool, error) {
		client := newHTTPClient(ohAPITimeout)
		resp, err := client.Get(checksumAsset.BrowserDownloadURL)
		if err != nil {
			return !errors.Is(err, ErrRedirect), fmt.Errorf("downloading checksums (attempt %d): %w", attempt, err)
		}
		defer resp.Body.Close()

		if retry.IsTransientHTTP(resp.StatusCode) {
			return true, fmt.Errorf("checksums download: HTTP %d (attempt %d)", resp.StatusCode, attempt)
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("checksums download failed: HTTP %d", resp.StatusCode)
		}

		body, err = io.ReadAll(io.LimitReader(resp.Body, maxChecksumFileSize))
		if err != nil {
			return true, fmt.Errorf("reading checksums (attempt %d): %w", attempt, err)
		}
		return false, nil
	})
	if retryErr != nil {
		return nil, retryErr
	}
	return body, nil
}

// parseChecksums extracts the expected SHA256 (hex) of the named asset from a
// checksums file.
func parseChecksums(body []byte, targetAssetName string) (string, error) {
	// Parse checksums.txt format: "<sha256>  <filename>\n"
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "hash  filename" (two spaces) or "hash filename" (one space)
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		if parts[1] == targetAssetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("no checksum found for %q in checksums file — refusing to install unverified binary", targetAssetName)
}

// verifyChecksum computes the SHA256 of the file at filePath and compares it
// to the expected hex-encoded hash. Returns an error on mismatch.
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
		return fmt.Errorf("checksum mismatch: expected %s, got %s — the downloaded file may be corrupted or tampered with", expectedSHA256, actual)
	}
	return nil
}
