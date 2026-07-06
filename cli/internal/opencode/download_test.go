package opencode

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRelease creates a mock GitHub release JSON response.
func mockRelease(version string, assets []ReleaseAsset) []byte {
	release := Release{
		TagName: "v" + version,
		Assets:  assets,
	}
	data, _ := json.Marshal(release)
	return data
}

// createTestZip creates a zip archive containing a single file named "opencode"
// with the given content. Returns the path to the zip file.
func createTestZip(t *testing.T, content []byte) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	require.NoError(t, err)

	w := zip.NewWriter(tmpFile)
	f, err := w.Create("opencode")
	require.NoError(t, err)
	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

// createTestTarGz creates a tar.gz archive containing a single file named "opencode"
// with the given content. Returns the path to the archive.
func createTestTarGz(t *testing.T, content []byte) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.tar.gz")
	require.NoError(t, err)

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "opencode",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write(content)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestLatestReleaseSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assets := []ReleaseAsset{
			{
				Name:               "opencode-darwin-arm64.zip",
				Size:               40000000,
				BrowserDownloadURL: "https://example.com/opencode-darwin-arm64.zip",
				Digest:             "sha256:abcdef1234567890",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockRelease("1.17.13", assets))
	}))
	defer server.Close()

	// Override the API URL for testing
	origURL := githubReleasesAPI
	defer func() { /* can't reassign const, test fetchRelease directly */ }()
	_ = origURL

	// Test fetchRelease directly
	release, err := fetchRelease(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "v1.17.13", release.TagName)
	assert.Equal(t, "1.17.13", release.Version())
	assert.Len(t, release.Assets, 1)
	assert.Equal(t, "abcdef1234567890", release.Assets[0].SHA256())
}

func TestLatestReleaseNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchRelease(server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLatestReleaseRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := fetchRelease(server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestReleaseAssetSHA256(t *testing.T) {
	tests := []struct {
		digest   string
		expected string
	}{
		{"sha256:abcdef", "abcdef"},
		{"sha256:dd016d3e26b347d675ab26c45d1e287545912d5c4c49fa0770b622d4a1367e23", "dd016d3e26b347d675ab26c45d1e287545912d5c4c49fa0770b622d4a1367e23"},
		{"", ""},
		{"md5:abc", ""},
	}

	for _, tt := range tests {
		a := ReleaseAsset{Digest: tt.digest}
		assert.Equal(t, tt.expected, a.SHA256(), "digest=%q", tt.digest)
	}
}

func TestVerifyChecksumSuccess(t *testing.T) {
	content := []byte("hello opencode binary")
	tmpFile := filepath.Join(t.TempDir(), "testfile")
	require.NoError(t, os.WriteFile(tmpFile, content, 0o644))

	expected := sha256Hex(content)
	err := verifyChecksum(tmpFile, expected)
	assert.NoError(t, err)
}

func TestVerifyChecksumMismatch(t *testing.T) {
	content := []byte("hello opencode binary")
	tmpFile := filepath.Join(t.TempDir(), "testfile")
	require.NoError(t, os.WriteFile(tmpFile, content, 0o644))

	err := verifyChecksum(tmpFile, "0000000000000000000000000000000000000000000000000000000000000000")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestExtractFromZip(t *testing.T) {
	content := []byte("#!/bin/sh\necho opencode\n")
	zipPath := createTestZip(t, content)
	destPath := filepath.Join(t.TempDir(), "opencode-extracted")

	err := extractFromZip(zipPath, destPath)
	require.NoError(t, err)

	extracted, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, content, extracted)
}

func TestExtractFromTarGz(t *testing.T) {
	content := []byte("#!/bin/sh\necho opencode\n")
	tarPath := createTestTarGz(t, content)
	destPath := filepath.Join(t.TempDir(), "opencode-extracted")

	err := extractFromTarGz(tarPath, destPath)
	require.NoError(t, err)

	extracted, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, content, extracted)
}

func TestDownloadFullFlow(t *testing.T) {
	// Create a fake binary content
	binaryContent := []byte("fake-opencode-binary-content-for-testing")

	// Create a zip archive with that content
	zipPath := createTestZip(t, binaryContent)
	zipData, err := os.ReadFile(zipPath)
	require.NoError(t, err)
	zipSHA := sha256Hex(zipData)

	// Mock server serving both API and download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/release":
			assets := []ReleaseAsset{
				{
					Name:               "opencode-darwin-arm64.zip",
					Size:               int64(len(zipData)),
					BrowserDownloadURL: fmt.Sprintf("http://%s/download/opencode-darwin-arm64.zip", r.Host),
					Digest:             "sha256:" + zipSHA,
				},
				{
					Name:               "opencode-linux-x64.tar.gz",
					Size:               1000,
					BrowserDownloadURL: fmt.Sprintf("http://%s/download/opencode-linux-x64.tar.gz", r.Host),
					Digest:             "sha256:fakedigest",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(mockRelease("1.17.13", assets))
		case "/download/opencode-darwin-arm64.zip":
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))
			w.Write(zipData)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Verify fetchRelease works
	release, err := fetchRelease(server.URL + "/api/release")
	require.NoError(t, err)
	assert.Equal(t, "1.17.13", release.Version())

	// Find the darwin-arm64 asset
	var asset *ReleaseAsset
	for i := range release.Assets {
		if release.Assets[i].Name == "opencode-darwin-arm64.zip" {
			asset = &release.Assets[i]
			break
		}
	}
	require.NotNil(t, asset)

	// Download to temp
	installDir := t.TempDir()
	tmpFile, err := os.CreateTemp("", "dl-test-*")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	var progressCalls int
	err = downloadAsset(asset, tmpFile, func(downloaded, total int64) {
		progressCalls++
	})
	require.NoError(t, err)
	tmpFile.Close()
	assert.Greater(t, progressCalls, 0)

	// Verify checksum
	err = verifyChecksum(tmpFile.Name(), zipSHA)
	require.NoError(t, err)

	// Extract
	binaryPath := filepath.Join(installDir, "opencode-1.17.13")
	err = extractFromZip(tmpFile.Name(), binaryPath)
	require.NoError(t, err)

	// Verify content
	extracted, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, extracted)
}

func TestInstalledVersion(t *testing.T) {
	installDir := t.TempDir()

	// No symlink — empty version
	assert.Equal(t, "", InstalledVersion(installDir))

	// Create a fake versioned binary and symlink
	binaryPath := filepath.Join(installDir, "opencode-1.17.13")
	require.NoError(t, os.WriteFile(binaryPath, []byte("fake"), 0o755))
	symlinkPath := filepath.Join(installDir, "opencode")
	require.NoError(t, os.Symlink(binaryPath, symlinkPath))

	assert.Equal(t, "1.17.13", InstalledVersion(installDir))
}

func TestSymlinkUpdate(t *testing.T) {
	installDir := t.TempDir()

	// Create first version
	v1Path := filepath.Join(installDir, "opencode-1.17.12")
	require.NoError(t, os.WriteFile(v1Path, []byte("v1"), 0o755))
	symlinkPath := filepath.Join(installDir, "opencode")
	require.NoError(t, os.Symlink(v1Path, symlinkPath))

	assert.Equal(t, "1.17.12", InstalledVersion(installDir))

	// Simulate upgrade: remove old symlink, create new
	os.Remove(symlinkPath)
	v2Path := filepath.Join(installDir, "opencode-1.17.13")
	require.NoError(t, os.WriteFile(v2Path, []byte("v2"), 0o755))
	require.NoError(t, os.Symlink(v2Path, symlinkPath))

	assert.Equal(t, "1.17.13", InstalledVersion(installDir))
}

func TestEnsureInstalledNonInteractive(t *testing.T) {
	// When opencode is already in PATH (which it likely is in the test env),
	// EnsureInstalled should succeed.
	// If not in PATH, it should return an error with instructions.
	_, err := EnsureInstalled(false)
	if err != nil {
		assert.Contains(t, err.Error(), "brew install")
	}
}

func TestProgressReader(t *testing.T) {
	content := []byte("hello world this is some content")
	var lastDownloaded, lastTotal int64
	pr := &progressReader{
		reader: &mockReader{data: content},
		total:  int64(len(content)),
		progress: func(downloaded, total int64) {
			lastDownloaded = downloaded
			lastTotal = total
		},
	}

	buf := make([]byte, 10)
	n, _ := pr.Read(buf)
	assert.Equal(t, 10, n)
	assert.Equal(t, int64(10), lastDownloaded)
	assert.Equal(t, int64(len(content)), lastTotal)
}

type mockReader struct {
	data   []byte
	offset int
}

func (r *mockReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
