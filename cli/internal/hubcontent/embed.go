// Package hubcontent provides embedded hub content (agents and skills) and
// extraction logic to deploy them to ~/.oh/hub/.
//
// The hub/ subdirectory is populated at build time (goreleaser copies agents/
// and skills/ from the repo root). In development, run `make embed-sync` or
// the equivalent copy commands to populate it.
package hubcontent

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/buildinfo"
)

//go:embed all:hub/agents all:hub/skills
var embedded embed.FS

// HubContentDir returns the path where hub content is extracted (~/.oh/hub/).
func HubContentDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oh", "hub")
}

// IsInstalled returns true if hub content has been extracted at least once.
func IsInstalled() bool {
	dir := HubContentDir()
	_, err := os.Stat(filepath.Join(dir, "agents"))
	return err == nil
}

// NeedsExtract returns true if the hub content needs to be extracted
// (not present or version mismatch with current binary).
func NeedsExtract() bool {
	versionFile := filepath.Join(HubContentDir(), ".version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return true
	}
	return string(data) != buildinfo.Version
}

// Extract writes the embedded hub content to destDir.
// Idempotent — skips if already at current version.
func Extract(destDir string) error {
	versionFile := filepath.Join(destDir, ".version")
	if data, err := os.ReadFile(versionFile); err == nil && string(data) == buildinfo.Version {
		return nil // already up to date
	}

	// Clean and recreate
	os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating hub content dir: %w", err)
	}

	// Walk embedded FS and write files
	err := fs.WalkDir(embedded, "hub", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root "hub" directory itself
		if path == "hub" {
			return nil
		}

		// Strip "hub/" prefix to get relative path
		relPath := path[len("hub/"):]

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		// Skip .gitkeep placeholder files
		if d.Name() == ".gitkeep" {
			return nil
		}

		data, err := fs.ReadFile(embedded, path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}
		return os.WriteFile(destPath, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("extracting hub content: %w", err)
	}

	// Write version marker
	return os.WriteFile(versionFile, []byte(buildinfo.Version), 0o644)
}
