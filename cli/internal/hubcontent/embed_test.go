package hubcontent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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
	if string(versionData) != marker() {
		t.Errorf(".version = %q, want %q", versionData, marker())
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
	if string(data) != marker() {
		t.Errorf(".version = %q, want %q", data, marker())
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

// The hub lives in OH_HOME when it is set (a relocated hub), else in ~/.oh.
func TestHubContentDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	t.Setenv("OH_HOME", "")
	if got, want := HubContentDir(), filepath.Join(home, ".oh", "hub"); got != want {
		t.Errorf("HubContentDir() = %q, want %q", got, want)
	}

	relocated := t.TempDir()
	t.Setenv("OH_HOME", relocated)
	if got, want := HubContentDir(), filepath.Join(relocated, "hub"); got != want {
		t.Errorf("HubContentDir() with OH_HOME = %q, want %q", got, want)
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

// A development build ("dev") or a build of a modified tree ("-dirty") keeps
// its version while the embedded content changes: the marker adds a
// fingerprint, a release is told by its version alone.
func TestMarker(t *testing.T) {
	prev := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = prev })
	for _, v := range []string{"dev", "5.1.0-dirty", "v5.1.0-3-gabc1234-dirty"} {
		buildinfo.Version = v
		got := marker()
		if !strings.HasPrefix(got, v+"+") || len(got) != len(v)+1+12 {
			t.Errorf("marker() for %q = %q, want <version>+<12 hex>", v, got)
		}
	}
	for _, v := range []string{"5.1.0", "5.1.0-rc.1", "v5.1.0"} {
		buildinfo.Version = v
		if got := marker(); got != v {
			t.Errorf("marker() for %q = %q, want the version alone", v, got)
		}
	}
}

func TestFingerprintFollowsTheContent(t *testing.T) {
	a := fstest.MapFS{"hub/agents/a.md": {Data: []byte("one")}, "hub/skills/.gitkeep": {Data: nil}}
	same := fstest.MapFS{"hub/agents/a.md": {Data: []byte("one")}}
	changed := fstest.MapFS{"hub/agents/a.md": {Data: []byte("two")}}
	renamed := fstest.MapFS{"hub/agents/b.md": {Data: []byte("one")}}
	if fingerprint(a) != fingerprint(same) {
		t.Error("a placeholder .gitkeep changes nothing")
	}
	if fingerprint(a) == fingerprint(changed) || fingerprint(a) == fingerprint(renamed) {
		t.Error("a changed or renamed file changes the fingerprint")
	}
}

// Content embedded differently under the same version: a dev build extracts
// again (the hub kept the content of the previous build); a release does not.
func TestNeedsExtractFollowsTheContentOfADevBuild(t *testing.T) {
	prev := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = prev })
	t.Setenv("OH_HOME", t.TempDir())
	dest := HubContentDir()
	versionFile := filepath.Join(dest, ".version")

	buildinfo.Version = "dev"
	if err := Extract(dest); err != nil {
		t.Fatal(err)
	}
	if NeedsExtract() {
		t.Fatal("just extracted: nothing to extract")
	}
	for name, old := range map[string]string{"marker of an older dev build": "dev", "other content": "dev+000000000000"} {
		if err := os.WriteFile(versionFile, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		if !NeedsExtract() {
			t.Errorf("%s (%q): the embedded content may have changed, extract again", name, old)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, "sentinel.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Extract(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sentinel.txt")); err == nil {
		t.Error("Extract() kept the previous content of a dev build")
	}
	if NeedsExtract() {
		t.Error("extracted again: up to date")
	}

	buildinfo.Version = "5.1.0"
	if err := os.WriteFile(versionFile, []byte("5.1.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	if NeedsExtract() {
		t.Error("a release is told by its version alone")
	}
}
