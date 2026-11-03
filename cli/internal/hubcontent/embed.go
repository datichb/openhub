// Package hubcontent provides embedded hub content (agents, skills,
// permissions and workflows) and extraction logic to deploy them to ~/.oh/hub/.
//
// The hub/ subdirectory is populated at build time (goreleaser copies agents/,
// skills/, permissions/ and workflows/ from the repo root). In development, run
// `make embed-sync` or the equivalent copy commands to populate it.
package hubcontent

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/buildinfo"
)

//go:embed all:hub/agents all:hub/skills all:hub/permissions all:hub/workflows
var embedded embed.FS

// marker is what the .version file of an extracted hub holds. A release is
// told by its version; a development build (version "dev") or a build of a
// modified tree ("-dirty") keeps the same version while its content changes,
// so the marker adds a fingerprint of the embedded content.
func marker() string {
	v := buildinfo.Version
	if v == "dev" || strings.HasSuffix(v, "-dirty") {
		return v + "+" + embeddedFingerprint()
	}
	return v
}

var embeddedFingerprint = sync.OnceValue(func() string { return fingerprint(embedded) })

// fingerprint is a short digest of the files of the hub/ tree of fsys (names
// and content, without the .gitkeep placeholders).
func fingerprint(fsys fs.FS) string {
	h := sha256.New()
	err := fs.WalkDir(fsys, "hub", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == ".gitkeep" {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(data))
		h.Write(data)
		return nil
	})
	if err != nil { // an unreadable tree never matches a marker written before
		fmt.Fprintf(h, "error\x00%v", err)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// HubContentDir returns the path where hub content is extracted (~/.oh/hub/).
func HubContentDir() string {
	if dir := os.Getenv("OH_HOME"); dir != "" { // relocated hub (see config.HubDir)
		return filepath.Join(dir, "hub")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oh", "hub")
}

// NeedsExtract returns true if the hub content needs to be extracted (not
// present, or a version mismatch with the current binary; for a development
// or modified build, a mismatch of the embedded content too).
func NeedsExtract() bool {
	versionFile := filepath.Join(HubContentDir(), ".version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return true
	}
	return string(data) != marker()
}

// Extract writes the embedded hub content to destDir.
// Idempotent — skips if already at current version.
func Extract(destDir string) error {
	versionFile := filepath.Join(destDir, ".version")
	if data, err := os.ReadFile(versionFile); err == nil && string(data) == marker() {
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
	return os.WriteFile(versionFile, []byte(marker()), 0o644)
}

// Install copies a prepared hub (agents/, skills/, permissions/, workflows/
// of srcDir) to destDir and marks it as the content of this binary, so that
// the startup leaves it in place instead of extracting the embedded hub.
// Evaluation only: oh eval runs a pass with a variant of the hub this way,
// in a temporary OH_HOME.
func Install(srcDir, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating hub content dir: %w", err)
	}
	for _, part := range []string{"agents", "skills", "permissions", "workflows"} {
		src := filepath.Join(srcDir, part)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if err := os.CopyFS(filepath.Join(destDir, part), os.DirFS(src)); err != nil {
			return fmt.Errorf("installing hub content: %w", err)
		}
	}
	return os.WriteFile(filepath.Join(destDir, ".version"), []byte(marker()), 0o644)
}
