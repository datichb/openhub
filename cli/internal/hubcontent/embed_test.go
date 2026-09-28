package hubcontent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/buildinfo"
)

func TestExtract_CreatesFilesAndVersionMarker(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "hub")

	if err := Extract(destDir); err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	// Version marker must exist with current version
	versionData, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("reading .version: %v", err)
	}
	if string(versionData) != buildinfo.Version {
		t.Errorf(".version = %q, want %q", versionData, buildinfo.Version)
	}

	// Core directories must exist
	for _, dir := range []string{"agents", "skills", "permissions"} {
		info, err := os.Stat(filepath.Join(destDir, dir))
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", dir)
		}
	}
}

func TestExtract_Idempotent(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "hub")

	// First extraction
	if err := Extract(destDir); err != nil {
		t.Fatalf("first Extract() failed: %v", err)
	}

	// Write a sentinel file to detect if destDir is wiped on second call
	sentinel := filepath.Join(destDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second extraction — same version, should be a no-op
	if err := Extract(destDir); err != nil {
		t.Fatalf("second Extract() failed: %v", err)
	}

	// Sentinel should still exist (directory was NOT wiped)
	if _, err := os.Stat(sentinel); err != nil {
		t.Error("idempotent Extract() wiped the directory — sentinel file is gone")
	}
}

func TestExtract_ReExtractsOnVersionChange(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "hub")

	// First extraction
	if err := Extract(destDir); err != nil {
		t.Fatalf("first Extract() failed: %v", err)
	}

	// Write a sentinel
	sentinel := filepath.Join(destDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fake a version mismatch by overwriting the .version file
	versionFile := filepath.Join(destDir, ".version")
	if err := os.WriteFile(versionFile, []byte("old-version"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-extract — should wipe and recreate because version differs
	if err := Extract(destDir); err != nil {
		t.Fatalf("re-Extract() failed: %v", err)
	}

	// Sentinel should be gone (directory was wiped)
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("Extract() with version mismatch should have wiped the directory, but sentinel still exists")
	}

	// Version marker should be updated
	data, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatalf("reading .version after re-extract: %v", err)
	}
	if string(data) != buildinfo.Version {
		t.Errorf(".version = %q, want %q", data, buildinfo.Version)
	}
}

func TestExtract_SkipsGitkeepFiles(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "hub")

	if err := Extract(destDir); err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	// Walk the extracted directory and check no .gitkeep files exist
	err := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == ".gitkeep" {
			t.Errorf("found .gitkeep file that should have been skipped: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking destDir: %v", err)
	}
}

func TestExtract_FilesAreReadable(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "hub")

	if err := Extract(destDir); err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	// Verify at least one file was extracted and is readable
	fileCount := 0
	err := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() == ".version" {
			return nil
		}
		fileCount++
		// Verify file is readable
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("cannot read extracted file %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Errorf("extracted file %s is empty", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking destDir: %v", err)
	}
	if fileCount == 0 {
		t.Error("no files were extracted")
	}
}

func TestHubContentDir_ReturnsNonEmpty(t *testing.T) {
	dir := HubContentDir()
	if dir == "" {
		t.Fatal("HubContentDir() returned empty string")
	}
	// Should end with .oh/hub
	if filepath.Base(dir) != "hub" {
		t.Errorf("HubContentDir() = %q, expected to end with 'hub'", dir)
	}
}
