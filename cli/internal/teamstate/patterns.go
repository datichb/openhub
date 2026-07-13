package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Pattern represents a reusable decomposition pattern.
type Pattern struct {
	Name       string   `toml:"name"`
	Tags       []string `toml:"tags"`
	Complexity string   `toml:"complexity"` // low | medium | high
	Source     string   `toml:"source"`     // planner | pathfinder | manual
	Project    string   `toml:"project"`    // originating project
	Validated  bool     `toml:"validated"`
	CreatedAt  string   `toml:"created_at"`
}

// patternsIndex is the TOML structure of patterns/index.toml.
type patternsIndex struct {
	Patterns []Pattern `toml:"patterns"`
}

// ListPatterns returns all patterns from the index.
// If tags is non-empty, filters by matching at least minMatchTags tags.
func (r *Repo) ListPatterns(tags []string, minMatchTags int) ([]Pattern, error) {
	index, err := r.loadPatternsIndex()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(tags) == 0 {
		return index.Patterns, nil
	}

	// Filter by tag matching
	var filtered []Pattern
	for _, p := range index.Patterns {
		matches := countTagMatches(p.Tags, tags)
		if matches >= minMatchTags {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// ReadPattern reads the full content of a pattern file.
func (r *Repo) ReadPattern(name string) (string, error) {
	patternPath := filepath.Join(r.path, "patterns", name+".md")
	data, err := os.ReadFile(patternPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("pattern %q not found", name)
		}
		return "", err
	}
	return string(data), nil
}

// CreatePattern adds a new pattern to the index and creates the .md file.
func (r *Repo) CreatePattern(ctx interface{}, p Pattern, content string) error {
	dir := filepath.Join(r.path, "patterns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating patterns dir: %w", err)
	}

	// Set creation date if not set
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format("2006-01-02")
	}

	// Add to index
	index, err := r.loadPatternsIndex()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading patterns index: %w", err)
	}
	if index == nil {
		index = &patternsIndex{}
	}

	// Check for duplicate
	for _, existing := range index.Patterns {
		if existing.Name == p.Name {
			return fmt.Errorf("pattern %q already exists", p.Name)
		}
	}

	index.Patterns = append(index.Patterns, p)

	// Write index
	if err := r.savePatternsIndex(index); err != nil {
		return err
	}

	// Write pattern content file
	mdPath := filepath.Join(dir, p.Name+".md")
	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing pattern file: %w", err)
	}

	return nil
}

// ValidatePattern marks a pattern as validated.
func (r *Repo) ValidatePattern(name string) error {
	index, err := r.loadPatternsIndex()
	if err != nil {
		return err
	}

	found := false
	for i := range index.Patterns {
		if index.Patterns[i].Name == name {
			index.Patterns[i].Validated = true
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("pattern %q not found", name)
	}

	return r.savePatternsIndex(index)
}

// RemovePattern removes a pattern from the index and deletes its file.
func (r *Repo) RemovePattern(name string) error {
	index, err := r.loadPatternsIndex()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]Pattern, 0, len(index.Patterns))
	for _, p := range index.Patterns {
		if p.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("pattern %q not found", name)
	}

	index.Patterns = filtered
	if err := r.savePatternsIndex(index); err != nil {
		return err
	}

	// Remove .md file (best-effort)
	mdPath := filepath.Join(r.path, "patterns", name+".md")
	_ = os.Remove(mdPath)

	return nil
}

// --- internal helpers ---

func (r *Repo) loadPatternsIndex() (*patternsIndex, error) {
	path := filepath.Join(r.path, "patterns", "index.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx patternsIndex
	if err := toml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing patterns index: %w", err)
	}
	return &idx, nil
}

func (r *Repo) savePatternsIndex(index *patternsIndex) error {
	dir := filepath.Join(r.path, "patterns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := toml.Marshal(index)
	if err != nil {
		return fmt.Errorf("marshaling patterns index: %w", err)
	}
	path := filepath.Join(dir, "index.toml")
	return os.WriteFile(path, data, 0o644)
}

func countTagMatches(patternTags, queryTags []string) int {
	tagSet := make(map[string]bool, len(patternTags))
	for _, t := range patternTags {
		tagSet[strings.ToLower(t)] = true
	}
	count := 0
	for _, t := range queryTags {
		if tagSet[strings.ToLower(t)] {
			count++
		}
	}
	return count
}
