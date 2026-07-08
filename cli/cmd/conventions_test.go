package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTicketFromBranch(t *testing.T) {
	tests := []struct {
		branch   string
		expected string
	}{
		{"feat/SRU-142-user-auth", "SRU-142"},
		{"fix/PROJ-99-hotfix", "PROJ-99"},
		{"chore/ABC-1-cleanup", "ABC-1"},
		{"main", ""},
		{"develop", ""},
		{"feat/no-ticket-ref", ""},
		{"SRU-142-direct", "SRU-142"},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := extractTicketFromBranch(tt.branch)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFindCommitPatternConventional(t *testing.T) {
	dir := t.TempDir()
	wikiDir := filepath.Join(dir, "docs", "wiki", "technical")
	os.MkdirAll(wikiDir, 0o755)
	os.WriteFile(filepath.Join(wikiDir, "conventions.md"), []byte(`
# Conventions

## Commits

We use Conventional Commits format.
`), 0o644)

	pattern := findCommitPattern(dir)
	assert.NotEmpty(t, pattern)
	assert.Contains(t, pattern, "feat|fix")
}

func TestFindCommitPatternExplicit(t *testing.T) {
	dir := t.TempDir()
	wikiDir := filepath.Join(dir, "docs", "wiki", "technical")
	os.MkdirAll(wikiDir, 0o755)
	os.WriteFile(filepath.Join(wikiDir, "conventions.md"), []byte("# Conventions\n\ncommit_pattern = `^\\[.+\\]`\n"), 0o644)

	pattern := findCommitPattern(dir)
	assert.Equal(t, `^\[.+\]`, pattern)
}

func TestFindBranchPattern(t *testing.T) {
	dir := t.TempDir()
	wikiDir := filepath.Join(dir, "docs", "wiki", "technical")
	os.MkdirAll(wikiDir, 0o755)
	os.WriteFile(filepath.Join(wikiDir, "conventions.md"), []byte("# Conventions\n\nbranch_pattern = `^(feat|fix|chore)/[A-Z]+-\\d+`\n"), 0o644)

	pattern := findBranchPattern(dir)
	assert.Equal(t, `^(feat|fix|chore)/[A-Z]+-\d+`, pattern)
}

func TestFindBranchPatternNone(t *testing.T) {
	dir := t.TempDir()
	// No conventions file
	pattern := findBranchPattern(dir)
	assert.Empty(t, pattern)
}

func TestReadConventionsFilePriority(t *testing.T) {
	dir := t.TempDir()
	// Create both paths — should prefer docs/wiki/technical/
	techDir := filepath.Join(dir, "docs", "wiki", "technical")
	os.MkdirAll(techDir, 0o755)
	os.WriteFile(filepath.Join(techDir, "conventions.md"), []byte("technical conventions"), 0o644)
	os.WriteFile(filepath.Join(dir, "CONVENTIONS.md"), []byte("root conventions"), 0o644)

	content := readConventionsFile(dir)
	assert.Equal(t, "technical conventions", content)
}

func TestGetCurrentBranch(t *testing.T) {
	// This test works in any git repo
	cwd, _ := os.Getwd()
	branch := getCurrentBranch(cwd)
	// Should return something (we're in a git repo)
	assert.NotEmpty(t, branch)
}
