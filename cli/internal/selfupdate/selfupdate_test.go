package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"github release", "https://github.com/datichb/openhub/releases/download/v3.9.1/openhub_darwin_arm64.tar.gz", false},
		{"github release CDN", "https://objects.githubusercontent.com/github-production-release-asset-2e65be/123/abc", false},
		{"plain HTTP", "http://github.com/datichb/openhub/releases/download/v1.0.0/openhub_linux_amd64.tar.gz", true},
		{"unknown host", "https://evil.com/openhub_linux_amd64.tar.gz", true},
		{"look-alike host", "https://not-github.com/release.tar.gz", true},
		{"github as a subdomain of another host", "https://github.com.evil.com/release.tar.gz", true},
		{"empty", "", true},
		{"not a URL", "not-a-url", true},
		{"other scheme", "ftp://github.com/file.tar.gz", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDownloadURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err, "%q is refused", tt.url)
			} else {
				assert.NoError(t, err, "%q is accepted", tt.url)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("hello selfupdate")
	sum := sha256.Sum256(content)
	file := filepath.Join(t.TempDir(), "testfile")
	require.NoError(t, os.WriteFile(file, content, 0o644))

	tests := []struct {
		name    string
		path    string
		sum     string
		wantErr bool
	}{
		{"matching sum", file, hex.EncodeToString(sum[:]), false},
		{"other sum", file, strings.Repeat("0", 64), true},
		{"missing file", filepath.Join(t.TempDir(), "nonexistent"), "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyChecksum(tt.path, tt.sum)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAssetName_CurrentPlatform(t *testing.T) {
	name, err := AssetName()
	require.NoError(t, err, "the current platform has a release asset")
	assert.True(t, strings.HasPrefix(name, "openhub_"), "asset %q starts with openhub_", name)
	assert.True(t, strings.HasSuffix(name, ".tar.gz"), "asset %q is a tar.gz", name)
}

func TestExtractBinary(t *testing.T) {
	tests := []struct {
		name    string
		extract func(archive, dest string) error
		archive string // file name of the archive
		entry   string // name of the entry in the archive
		content string
		wantErr bool
	}{
		{"binary at the root", extractFromTarGz, "test.tar.gz", "oh", "#!/bin/sh\necho hello\n", false},
		{"nested binary: only the base name counts", extractFromTarGz, "test.tar.gz", "subdir/oh", "nested binary content", false},
		{"no oh in the archive", extractFromTarGz, "test.tar.gz", "not-oh", "wrong binary", true},
		// os.CreateTemp names the archive of the Update flow with no extension.
		{"archive with no extension", extractBinary, "oh-selfupdate-123456789", "oh", "#!/bin/sh\necho updated\n", false},
		// The destination is chosen by the caller, never taken from the entry.
		{"entry that climbs out of the folder", extractFromTarGz, "test.tar.gz", "../../etc/oh", "traversal content", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, tt.archive)
			dest := filepath.Join(dir, "oh-extracted")
			createTestTarGz(t, archive, tt.entry, []byte(tt.content))

			err := tt.extract(archive, dest)
			if tt.wantErr {
				assert.Error(t, err, "the binary is not extracted")
				return
			}
			require.NoError(t, err)
			got, err := os.ReadFile(dest)
			require.NoError(t, err, "the binary is written to the destination")
			assert.Equal(t, tt.content, string(got), "content of the binary")
			assert.NoFileExists(t, filepath.Join(dir, "..", "..", "etc", "oh"), "nothing is written outside the folder")
		})
	}
}

// createTestTarGz creates a .tar.gz archive with a single file entry.
func createTestTarGz(t *testing.T, archivePath, entryName string, content []byte) {
	t.Helper()

	f, err := os.Create(archivePath)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name:     entryName,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write(content)
	require.NoError(t, err)
}
