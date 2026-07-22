// Package skillregistry provides the skill marketplace:
// install, list, and remove community skills from external sources.
package skillregistry

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
)

const (
	// DefaultIndexURL is the community skills index.
	DefaultIndexURL = "https://raw.githubusercontent.com/datichb/oh-skills-index/main/index.json"
	maxDownloadSize = 10 * 1024 * 1024 // 10 MB
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// SkillManifest describes a skill package (manifest.json in the package root).
type SkillManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	// SkillFile is the SKILL.md filename (default: SKILL.md)
	SkillFile string `json:"skill_file"`
	// Tags for filtering (e.g. "golang", "security", "documentation")
	Tags []string `json:"tags"`
}

// InstalledSkill represents a skill package in ~/.oh/skills/<name>/.
type InstalledSkill struct {
	Manifest SkillManifest
	Path     string
	Source   string // "builtin" | "community"
}

// Registry manages community skill packages in ~/.oh/skills/.
type Registry struct {
	dir string
}

// NewRegistry creates a Registry pointing to ~/.oh/skills/.
func NewRegistry() *Registry {
	return &Registry{dir: filepath.Join(config.HubDir(), "skills")}
}

// ListInstalled returns all community skills installed in ~/.oh/skills/.
func (r *Registry) ListInstalled() ([]InstalledSkill, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills dir: %w", err)
	}

	var skills []InstalledSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(r.dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m SkillManifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		skills = append(skills, InstalledSkill{
			Manifest: m,
			Path:     filepath.Join(r.dir, entry.Name()),
			Source:   "community",
		})
	}
	return skills, nil
}

// Install downloads and installs a skill package from a Git URL or index name.
// Supported sources:
//   - Git URL: https://github.com/user/oh-skill-example
//   - Index name: golang-idioms (looked up in the community index)
func (r *Registry) Install(source string) (*InstalledSkill, error) {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating skills dir: %w", err)
	}

	// Determine download URL
	downloadURL := source
	if !strings.HasPrefix(source, "http") {
		// Look up in community index
		indexURL, err := r.lookupIndex(source)
		if err != nil {
			return nil, fmt.Errorf("looking up %q in index: %w", source, err)
		}
		downloadURL = indexURL
	}

	// Download archive
	archivePath, err := r.download(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("downloading skill: %w", err)
	}
	defer os.Remove(archivePath)

	// Extract and read manifest
	manifest, skillDir, err := r.extract(archivePath)
	if err != nil {
		return nil, fmt.Errorf("extracting skill: %w", err)
	}

	return &InstalledSkill{
		Manifest: *manifest,
		Path:     skillDir,
		Source:   "community",
	}, nil
}

// Remove uninstalls a community skill by name.
func (r *Registry) Remove(name string) error {
	skillDir := filepath.Join(r.dir, name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill %q is not installed", name)
	}
	return os.RemoveAll(skillDir)
}

// SkillMDPath returns the path to the SKILL.md file for an installed skill.
func (r *Registry) SkillMDPath(name string) (string, error) {
	entries, err := r.ListInstalled()
	if err != nil {
		return "", err
	}
	for _, s := range entries {
		if s.Manifest.Name == name {
			fileName := s.Manifest.SkillFile
			if fileName == "" {
				fileName = "SKILL.md"
			}
			return filepath.Join(s.Path, fileName), nil
		}
	}
	return "", fmt.Errorf("skill %q not found", name)
}

// IndexEntry represents a single skill in the community index.
type IndexEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"` // archive download URL
	Tags        []string `json:"tags"`
}

// FetchIndex downloads and parses the community skills index.
func FetchIndex() ([]IndexEntry, error) {
	return fetchIndexFrom(DefaultIndexURL)
}

func fetchIndexFrom(url string) ([]IndexEntry, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index fetch returned %d", resp.StatusCode)
	}
	var entries []IndexEntry
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDownloadSize)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding index: %w", err)
	}
	return entries, nil
}

func (r *Registry) lookupIndex(name string) (string, error) {
	entries, err := FetchIndex()
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name == name {
			return e.URL, nil
		}
	}
	return "", fmt.Errorf("skill %q not found in community index", name)
}

func (r *Registry) download(url string) (string, error) {
	// Convert GitHub repo URL to archive download URL
	archiveURL := url
	if strings.Contains(url, "github.com") && !strings.HasSuffix(url, ".tar.gz") && !strings.HasSuffix(url, ".zip") {
		// Convert https://github.com/user/repo to tarball
		archiveURL = strings.TrimRight(url, "/") + "/archive/refs/heads/main.tar.gz"
	}

	resp, err := httpClient.Get(archiveURL)
	if err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "oh-skill-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, maxDownloadSize)); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("writing download: %w", err)
	}
	return tmp.Name(), nil
}

func (r *Registry) extract(archivePath string) (*SkillManifest, string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", fmt.Errorf("opening gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	// First pass: find manifest.json
	var manifest SkillManifest
	var skillFiles = make(map[string][]byte)
	var topDir string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("reading tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		parts := strings.SplitN(header.Name, "/", 3)
		if len(parts) < 2 {
			continue
		}
		if topDir == "" {
			topDir = parts[0]
		}

		name := parts[len(parts)-1]
		data, err := io.ReadAll(io.LimitReader(tr, maxDownloadSize))
		if err != nil {
			continue
		}
		skillFiles[header.Name] = data

		if name == "manifest.json" {
			_ = json.Unmarshal(data, &manifest)
		}
	}

	if manifest.Name == "" {
		return nil, "", fmt.Errorf("manifest.json not found or invalid in archive")
	}

	// Write files to ~/.oh/skills/<name>/
	skillDir := filepath.Join(r.dir, manifest.Name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating skill dir: %w", err)
	}

	for path, data := range skillFiles {
		parts := strings.SplitN(path, "/", 3)
		if len(parts) < 3 {
			continue
		}
		relPath := parts[2]
		destPath := filepath.Join(skillDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(destPath, data, 0o644)
	}

	return &manifest, skillDir, nil
}
