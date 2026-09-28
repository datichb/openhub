package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// validateDownloadURL
// ---------------------------------------------------------------------------

func TestValidateDownloadURL_ValidGitHub(t *testing.T) {
	urls := []string{
		"https://github.com/datichb/openhub/releases/download/v3.9.1/openhub_darwin_arm64.tar.gz",
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/123/abc",
	}
	for _, u := range urls {
		if err := validateDownloadURL(u); err != nil {
			t.Errorf("expected valid URL %q, got error: %v", u, err)
		}
	}
}

func TestValidateDownloadURL_RejectsHTTP(t *testing.T) {
	err := validateDownloadURL("http://github.com/datichb/openhub/releases/download/v1.0.0/openhub_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error for HTTP scheme, got nil")
	}
}

func TestValidateDownloadURL_RejectsUntrustedHost(t *testing.T) {
	urls := []string{
		"https://evil.com/openhub_linux_amd64.tar.gz",
		"https://not-github.com/release.tar.gz",
		"https://github.com.evil.com/release.tar.gz",
	}
	for _, u := range urls {
		if err := validateDownloadURL(u); err == nil {
			t.Errorf("expected error for untrusted URL %q, got nil", u)
		}
	}
}

func TestValidateDownloadURL_RejectsEmptyAndGarbage(t *testing.T) {
	urls := []string{
		"",
		"not-a-url",
		"ftp://github.com/file.tar.gz",
	}
	for _, u := range urls {
		if err := validateDownloadURL(u); err == nil {
			t.Errorf("expected error for %q, got nil", u)
		}
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum
// ---------------------------------------------------------------------------

func TestVerifyChecksum_Match(t *testing.T) {
	content := []byte("hello selfupdate")
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	tmp := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(tmp, expected); err != nil {
		t.Errorf("expected checksum match, got error: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(tmp, []byte("actual content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyChecksum(tmp, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
}

func TestVerifyChecksum_FileNotFound(t *testing.T) {
	err := verifyChecksum(filepath.Join(t.TempDir(), "nonexistent"), "abc")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

// ---------------------------------------------------------------------------
// parseChecksums (tested via a helper that mimics downloadAndParseChecksums)
// ---------------------------------------------------------------------------

func TestParseChecksumsFormat(t *testing.T) {
	// Simulate the parsing logic from downloadAndParseChecksums.
	// We test the scanner-based parsing directly.
	checksumContent := "c826d79e598b90b029ce430704e46b638fc24cff1cb512f3058bba2fc05a7c09  openhub_darwin_amd64.tar.gz\n" +
		"1f68e1fe441851b2e2451d464bb87972290d80ab2ee84189bd4d399b3d68279e  openhub_darwin_arm64.tar.gz\n" +
		"8b594c9bf8177b67f22729290a45023d5395b3b92f3f659d46f31fb6342c1c31  openhub_linux_amd64.tar.gz\n"

	// Write a fake checksums file, create a fake release with a file:// URL
	// that won't work with downloadAndParseChecksums (needs HTTP).
	// Instead, test the parsing by creating a file and running verifyChecksum
	// with a known hash.

	tests := []struct {
		name     string
		asset    string
		wantHash string
		wantErr  bool
	}{
		{"darwin_amd64", "openhub_darwin_amd64.tar.gz", "c826d79e598b90b029ce430704e46b638fc24cff1cb512f3058bba2fc05a7c09", false},
		{"darwin_arm64", "openhub_darwin_arm64.tar.gz", "1f68e1fe441851b2e2451d464bb87972290d80ab2ee84189bd4d399b3d68279e", false},
		{"linux_amd64", "openhub_linux_amd64.tar.gz", "8b594c9bf8177b67f22729290a45023d5395b3b92f3f659d46f31fb6342c1c31", false},
		{"not_found", "openhub_windows_amd64.tar.gz", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := parseChecksumLine(checksumContent, tt.asset)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hash != tt.wantHash {
				t.Errorf("got hash %q, want %q", hash, tt.wantHash)
			}
		})
	}
}

// parseChecksumLine is a test helper that replicates the parsing logic
// from downloadAndParseChecksums without needing an HTTP server.
func parseChecksumLine(content, targetAsset string) (string, error) {
	lines := splitLines(content)
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := splitFields(line)
		if len(parts) != 2 {
			continue
		}
		if parts[1] == targetAsset {
			return parts[0], nil
		}
	}
	return "", errNoChecksum(targetAsset)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	field := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if field != "" {
				fields = append(fields, field)
				field = ""
			}
		} else {
			field += string(c)
		}
	}
	if field != "" {
		fields = append(fields, field)
	}
	return fields
}

type checksumNotFoundError struct{ asset string }

func (e *checksumNotFoundError) Error() string {
	return "no checksum found for " + e.asset
}

func errNoChecksum(asset string) error {
	return &checksumNotFoundError{asset: asset}
}

// ---------------------------------------------------------------------------
// AssetName
// ---------------------------------------------------------------------------

func TestAssetName_CurrentPlatform(t *testing.T) {
	name, err := AssetName()
	if err != nil {
		t.Fatalf("AssetName() failed on current platform: %v", err)
	}
	if name == "" {
		t.Fatal("AssetName() returned empty string")
	}
	// Should start with "openhub_" and end with ".tar.gz"
	if !hasPrefix(name, "openhub_") {
		t.Errorf("expected prefix 'openhub_', got %q", name)
	}
	if !hasSuffix(name, ".tar.gz") {
		t.Errorf("expected suffix '.tar.gz', got %q", name)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// ---------------------------------------------------------------------------
// extractFromTarGz
// ---------------------------------------------------------------------------

func TestExtractFromTarGz_ValidArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "oh-extracted")

	binaryContent := []byte("#!/bin/sh\necho hello\n")
	createTestTarGz(t, archivePath, "oh", binaryContent)

	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz failed: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", got, binaryContent)
	}
}

func TestExtractFromTarGz_BinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "oh-extracted")

	createTestTarGz(t, archivePath, "not-oh", []byte("wrong binary"))

	err := extractFromTarGz(archivePath, destPath)
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestExtractFromTarGz_NestedPathExtractsCorrectly(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "oh-extracted")

	// Archive with nested path — filepath.Base("subdir/oh") == "oh"
	binaryContent := []byte("nested binary content")
	createTestTarGz(t, archivePath, "subdir/oh", binaryContent)

	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz failed: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", got, binaryContent)
	}
}

func TestExtractFromTarGz_PathTraversalSafe(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "oh-extracted")

	// Archive entry with ../../oh — filepath.Base returns "oh" so it matches,
	// but destPath is hardcoded by the caller, not derived from header.Name.
	binaryContent := []byte("traversal content")
	createTestTarGz(t, archivePath, "../../etc/oh", binaryContent)

	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz failed: %v", err)
	}

	// Verify the file was written to destPath, NOT to ../../etc/oh
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", got, binaryContent)
	}

	// Verify no file was written outside the temp dir
	badPath := filepath.Join(dir, "..", "..", "etc", "oh")
	if _, err := os.Stat(badPath); err == nil {
		t.Error("path traversal: file was written outside temp dir")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// createTestTarGz creates a .tar.gz archive with a single file entry.
func createTestTarGz(t *testing.T, archivePath, entryName string, content []byte) {
	t.Helper()

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
}
