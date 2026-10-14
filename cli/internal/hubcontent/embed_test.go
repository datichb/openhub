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
	for _, dir := range []string{"agents", "skills", "permissions", "workflows"} {
		info, err := os.Stat(filepath.Join(destDir, dir))
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", dir)
		}
	}

	// Shipped workflows and their prompt templates (<hub>/workflows, read by
	// hubcat.LoadWorkflows).
	for _, f := range []string{"workflows/ticket.yaml", "workflows/spec.yaml", "workflows/quick.yaml", "workflows/prompts/ticket.md.tmpl"} {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			t.Errorf("expected %s to be extracted: %v", f, err)
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

// Install puts a variant of the hub in an oh home (oh eval --variant): the
// startup keeps it instead of extracting the embedded hub.
func TestInstall_KeepsAVariantInPlace(t *testing.T) {
	src := t.TempDir()
	for _, f := range []string{"agents/a/x.md", "skills/c/s.md", "workflows/w.yaml"} {
		p := filepath.Join(src, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	dest := HubContentDir()
	if err := os.MkdirAll(filepath.Join(dest, "skills", "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(src, dest); err != nil {
		t.Fatalf("Install() failed: %v", err)
	}
	if NeedsExtract() {
		t.Fatal("an installed variant needs no extraction")
	}
	if err := Extract(dest); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "skills", "c", "s.md")); err != nil || string(data) != "skills/c/s.md" {
		t.Errorf("variant replaced: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "old")); !os.IsNotExist(err) {
		t.Errorf("previous content left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "permissions")); !os.IsNotExist(err) {
		t.Errorf("no permissions/ in the source: none installed (%v)", err)
	}
}
