package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const contextManifestFile = "context-manifest.json"

// ContextManifest tracks the freshness of deployed context files.
// Written at deploy time to .opencode/context-manifest.json.
type ContextManifest struct {
	GeneratedAt string                 `json:"generated_at"`
	HubDir      string                 `json:"hub_dir"`
	Files       map[string]ManifestEntry `json:"files"` // relative path → entry
}

// ManifestEntry records the hash and deploy time for a single source file.
type ManifestEntry struct {
	SHA256     string `json:"sha256"`
	DeployedAt string `json:"deployed_at"`
}

// FreshnessReport contains the result of a context freshness check.
type FreshnessReport struct {
	Fresh   bool     // true if all files match
	Stale   []string // source files that changed since deploy
	Missing []string // source files that no longer exist
	New     []string // source files not in the manifest (added since deploy)
}

// WriteContextManifest scans the hub source directory and writes a manifest
// recording SHA-256 hashes for all agent/skill/context files.
func WriteContextManifest(hubDir, projectPath string) error {
	manifest := ContextManifest{
		GeneratedAt: time.Now().Format(time.RFC3339),
		HubDir:      hubDir,
		Files:       make(map[string]ManifestEntry),
	}

	now := time.Now().Format(time.RFC3339)
	dirs := []string{"agents", "skills"}
	for _, dir := range dirs {
		srcDir := filepath.Join(hubDir, dir)
		if _, err := os.Stat(srcDir); err != nil {
			continue
		}
		_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(hubDir, path)
			hash, hashErr := hashFile(path)
			if hashErr != nil {
				return nil // skip files we can't read
			}
			manifest.Files[rel] = ManifestEntry{
				SHA256:     hash,
				DeployedAt: now,
			}
			return nil
		})
	}

	// Write manifest
	manifestPath := filepath.Join(projectPath, ".opencode", contextManifestFile)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling context manifest: %w", err)
	}
	return os.WriteFile(manifestPath, data, 0o644)
}

// CheckContextFreshness reads the manifest and compares against current source files.
// Returns a report indicating which files are stale, missing, or new.
func CheckContextFreshness(hubDir, projectPath string) (*FreshnessReport, error) {
	manifestPath := filepath.Join(projectPath, ".opencode", contextManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no manifest = nothing to check (first deploy pending)
		}
		return nil, fmt.Errorf("reading context manifest: %w", err)
	}

	var manifest ContextManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing context manifest: %w", err)
	}

	report := &FreshnessReport{Fresh: true}

	// Check existing manifest entries against current source
	for relPath, entry := range manifest.Files {
		srcPath := filepath.Join(hubDir, relPath)
		currentHash, err := hashFile(srcPath)
		if err != nil {
			report.Missing = append(report.Missing, relPath)
			report.Fresh = false
			continue
		}
		if currentHash != entry.SHA256 {
			report.Stale = append(report.Stale, relPath)
			report.Fresh = false
		}
	}

	// Check for new files in source that aren't in the manifest
	dirs := []string{"agents", "skills"}
	for _, dir := range dirs {
		srcDir := filepath.Join(hubDir, dir)
		if _, err := os.Stat(srcDir); err != nil {
			continue
		}
		_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(hubDir, path)
			if _, exists := manifest.Files[rel]; !exists {
				report.New = append(report.New, rel)
				report.Fresh = false
			}
			return nil
		})
	}

	return report, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
