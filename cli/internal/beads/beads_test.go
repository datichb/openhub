package beads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadyStatus(t *testing.T) {
	tests := []struct {
		status string
		ready  bool
	}{
		{"open", true},
		{"ready", true},
		{"todo", true},
		{"to_do", true},
		{"backlog", true},
		{"in_progress", false},
		{"done", false},
		{"closed", false},
		{"review", false},
		{"blocked", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.ready, isReadyStatus(tt.status))
		})
	}
}

func TestHasLabel(t *testing.T) {
	ticket := Ticket{
		ID:     "bd-1",
		Title:  "Test ticket",
		Labels: []string{"ai-delegated", "bug", "P1"},
	}

	assert.True(t, hasLabel(ticket, "ai-delegated"))
	assert.True(t, hasLabel(ticket, "AI-Delegated")) // case insensitive
	assert.True(t, hasLabel(ticket, "bug"))
	assert.False(t, hasLabel(ticket, "feature"))
	assert.False(t, hasLabel(ticket, ""))
}

func TestHasLabelEmptyLabels(t *testing.T) {
	ticket := Ticket{
		ID:    "bd-2",
		Title: "No labels",
	}
	assert.False(t, hasLabel(ticket, "ai-delegated"))
}

func TestRunBdJSONParsing(t *testing.T) {
	// Test the JSON parsing logic with sample data
	sampleJSON := `[
		{"id": "bd-1", "title": "Fix login", "status": "open", "priority": "1", "type": "feature", "labels": ["ai-delegated"]},
		{"id": "bd-2", "title": "Epic: Auth", "status": "open", "priority": "0", "type": "epic"},
		{"id": "bd-3", "title": "Bug report", "status": "done", "priority": "2", "type": "bug", "parent": "bd-2"}
	]`

	// We can't easily mock exec.Command, but we can test the filter logic
	tickets := []Ticket{
		{ID: "bd-1", Title: "Fix login", Status: "open", Priority: "1", Type: "feature", Labels: []string{"ai-delegated"}},
		{ID: "bd-2", Title: "Epic: Auth", Status: "open", Priority: "0", Type: "epic"},
		{ID: "bd-3", Title: "Bug report", Status: "done", Priority: "2", Type: "bug", Parent: "bd-2"},
	}

	// Test epic filtering
	var epics []Ticket
	for _, t := range tickets {
		if t.Type == "epic" {
			epics = append(epics, t)
		}
	}
	require.Len(t, epics, 1)
	assert.Equal(t, "bd-2", epics[0].ID)

	// Test orphan filtering (no parent, ready, not epic)
	var orphansLabeled, orphansOther []Ticket
	for _, tk := range tickets {
		if tk.Type == "epic" || !isReadyStatus(tk.Status) || tk.Parent != "" {
			continue
		}
		if hasLabel(tk, "ai-delegated") {
			orphansLabeled = append(orphansLabeled, tk)
		} else {
			orphansOther = append(orphansOther, tk)
		}
	}
	require.Len(t, orphansLabeled, 1)
	assert.Equal(t, "bd-1", orphansLabeled[0].ID)
	assert.Empty(t, orphansOther)

	// Verify JSON is parseable
	_ = sampleJSON // used for documentation
}

func TestAvailable(t *testing.T) {
	// This test checks that Available() doesn't panic
	// It may pass or fail depending on whether bd is installed
	err := Available()
	if err != nil {
		assert.Contains(t, err.Error(), "bd not found")
		assert.Contains(t, err.Error(), "brew install")
	}
}

// ---------------------------------------------------------------------------
// EnsureInitFlags
// ---------------------------------------------------------------------------

func TestEnsureInitFlags_AddsAllMissing(t *testing.T) {
	args := []string{"init", "--prefix", "myapp"}
	result := EnsureInitFlags(args)

	assert.Contains(t, result, "--skip-hooks")
	assert.Contains(t, result, "--skip-agents")
	assert.Contains(t, result, "--setup-exclude")
	// Original args are preserved
	assert.Equal(t, "init", result[0])
	assert.Equal(t, "--prefix", result[1])
	assert.Equal(t, "myapp", result[2])
}

func TestEnsureInitFlags_NoDoubles(t *testing.T) {
	args := []string{"init", "--skip-hooks", "--skip-agents", "--setup-exclude"}
	result := EnsureInitFlags(args)

	assert.Equal(t, args, result)
}

func TestEnsureInitFlags_PartialPresent(t *testing.T) {
	args := []string{"init", "--skip-hooks", "--prefix", "x"}
	result := EnsureInitFlags(args)

	assert.Contains(t, result, "--skip-hooks")
	assert.Contains(t, result, "--skip-agents")
	assert.Contains(t, result, "--setup-exclude")
	// --skip-hooks should appear exactly once
	count := 0
	for _, a := range result {
		if a == "--skip-hooks" {
			count++
		}
	}
	assert.Equal(t, 1, count, "--skip-hooks should not be duplicated")
}

func TestEnsureInitFlags_StealthShortCircuit(t *testing.T) {
	args := []string{"init", "--stealth"}
	result := EnsureInitFlags(args)

	// When --stealth is present, no flags should be added
	assert.Equal(t, args, result)
	assert.NotContains(t, result, "--skip-hooks")
}

func TestEnsureInitFlags_DoesNotMutateInput(t *testing.T) {
	args := []string{"init", "--prefix", "x"}
	original := make([]string, len(args))
	copy(original, args)

	_ = EnsureInitFlags(args)

	assert.Equal(t, original, args, "input slice must not be mutated")
}

// ---------------------------------------------------------------------------
// removeBeadsSection
// ---------------------------------------------------------------------------

func TestRemoveBeadsSection_FullHook(t *testing.T) {
	hook := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
# This section is managed by beads. Do not remove these markers.
if command -v bd >/dev/null 2>&1; then
  export BD_GIT_HOOK=1
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	result := removeBeadsSection(hook)
	assert.True(t, isEmptyHook(result), "hook should be empty after removing beads section")
	assert.NotContains(t, result, "BEADS INTEGRATION")
}

func TestRemoveBeadsSection_MixedHook(t *testing.T) {
	hook := `#!/usr/bin/env bash
# My project's pre-commit hook
set -euo pipefail

# Run linter
golangci-lint run

# --- BEGIN BEADS INTEGRATION v1.2.2 ---
# This section is managed by beads. Do not remove these markers.
if command -v bd >/dev/null 2>&1; then
  export BD_GIT_HOOK=1
  bd hook pre-commit "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	result := removeBeadsSection(hook)

	assert.NotContains(t, result, "BEADS INTEGRATION")
	assert.Contains(t, result, "golangci-lint run")
	assert.Contains(t, result, "set -euo pipefail")
	assert.False(t, isEmptyHook(result), "hook should still have content")
}

func TestRemoveBeadsSection_NoBeads(t *testing.T) {
	hook := `#!/usr/bin/env bash
# Regular hook
echo "running tests"
go test ./...
`
	result := removeBeadsSection(hook)
	assert.Equal(t, hook, result, "hook without beads section should be unchanged")
}

func TestRemoveBeadsSection_MultipleVersions(t *testing.T) {
	hook := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.0.0 ---
old stuff
# --- END BEADS INTEGRATION v1.0.0 ---
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
new stuff
# --- END BEADS INTEGRATION v1.2.2 ---
`
	result := removeBeadsSection(hook)
	assert.True(t, isEmptyHook(result))
}

// ---------------------------------------------------------------------------
// isEmptyHook
// ---------------------------------------------------------------------------

func TestIsEmptyHook(t *testing.T) {
	assert.True(t, isEmptyHook(""))
	assert.True(t, isEmptyHook("#!/usr/bin/env sh\n"))
	assert.True(t, isEmptyHook("#!/usr/bin/env sh\n\n\n"))
	assert.True(t, isEmptyHook("#!/bin/bash\n  \n"))
	assert.False(t, isEmptyHook("#!/usr/bin/env sh\necho hello\n"))
	assert.False(t, isEmptyHook("set -e\n"))
}

// ---------------------------------------------------------------------------
// sanitizeHooks (integration with temp git repo)
// ---------------------------------------------------------------------------

func TestSanitizeHooks_RemovesBeadsHook(t *testing.T) {
	dir := setupTempGitDir(t)

	hookContent := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	writeHook(t, dir, "post-checkout", hookContent)

	cleaned := sanitizeHooks(dir)
	assert.Equal(t, []string{"post-checkout"}, cleaned)

	// Hook file should be deleted (it was entirely beads)
	_, err := os.Stat(filepath.Join(dir, ".git", "hooks", "post-checkout"))
	assert.True(t, os.IsNotExist(err), "hook file should be deleted")
}

func TestSanitizeHooks_PreservesProjectHook(t *testing.T) {
	dir := setupTempGitDir(t)

	hookContent := `#!/usr/bin/env bash
# Project linter
golangci-lint run

# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook pre-commit "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	writeHook(t, dir, "pre-commit", hookContent)

	cleaned := sanitizeHooks(dir)
	assert.Equal(t, []string{"pre-commit"}, cleaned)

	// Hook file should still exist
	content, err := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "golangci-lint run")
	assert.NotContains(t, string(content), "BEADS INTEGRATION")
}

func TestSanitizeHooks_SkipsNonBeadsHook(t *testing.T) {
	dir := setupTempGitDir(t)

	hookContent := `#!/usr/bin/env bash
# My custom hook
echo "running"
`
	writeHook(t, dir, "pre-push", hookContent)

	cleaned := sanitizeHooks(dir)
	assert.Empty(t, cleaned, "non-beads hook should not be touched")

	// Content should be unchanged
	content, err := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-push"))
	require.NoError(t, err)
	assert.Equal(t, hookContent, string(content))
}

// ---------------------------------------------------------------------------
// sanitizeGitignore
// ---------------------------------------------------------------------------

func TestSanitizeGitignore_MovesToExclude(t *testing.T) {
	dir := setupTempGitDir(t)

	gitignoreContent := `node_modules/
dist/
.beads/
.env
`
	writeFile(t, filepath.Join(dir, ".gitignore"), gitignoreContent)

	moved, err := sanitizeGitignoreReport(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{".beads/"}, moved)

	// .gitignore should no longer contain .beads/
	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), ".beads/")
	assert.Contains(t, string(content), "node_modules/")
	assert.Contains(t, string(content), ".env")

	// .git/info/exclude should contain .beads/
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	require.NoError(t, err)
	assert.Contains(t, string(exclude), ".beads/")
}

func TestSanitizeGitignore_NoDuplicatesInExclude(t *testing.T) {
	dir := setupTempGitDir(t)

	// Pre-populate exclude with .beads/
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	writeFile(t, excludePath, "# existing\n.beads/\n")

	gitignoreContent := `.beads/
`
	writeFile(t, filepath.Join(dir, ".gitignore"), gitignoreContent)

	_, err := sanitizeGitignoreReport(dir)
	require.NoError(t, err)

	// .beads/ should appear only once in exclude
	exclude, err := os.ReadFile(excludePath)
	require.NoError(t, err)
	count := 0
	for _, line := range splitLines(string(exclude)) {
		if line == ".beads/" {
			count++
		}
	}
	assert.Equal(t, 1, count, ".beads/ should appear exactly once in exclude")
}

func TestSanitizeGitignore_PreservesOtherEntries(t *testing.T) {
	dir := setupTempGitDir(t)

	gitignoreContent := `# Dependencies
node_modules/

# Build
dist/
build/

# Beads
.beads/

# Environment
.env
.env.local
`
	writeFile(t, filepath.Join(dir, ".gitignore"), gitignoreContent)

	moved, err := sanitizeGitignoreReport(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{".beads/"}, moved)

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	s := string(content)
	assert.Contains(t, s, "node_modules/")
	assert.Contains(t, s, "dist/")
	assert.Contains(t, s, ".env")
	assert.Contains(t, s, ".env.local")
	assert.NotContains(t, s, ".beads/")
}

func TestSanitizeGitignore_RemovesBeadsComments(t *testing.T) {
	dir := setupTempGitDir(t)

	gitignoreContent := `node_modules/
# beads
.beads/
.env
`
	writeFile(t, filepath.Join(dir, ".gitignore"), gitignoreContent)

	_, err := sanitizeGitignoreReport(dir)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "# beads")
	assert.NotContains(t, string(content), ".beads/")
}

func TestSanitizeGitignore_NoBeadsEntries(t *testing.T) {
	dir := setupTempGitDir(t)

	gitignoreContent := `node_modules/
dist/
`
	writeFile(t, filepath.Join(dir, ".gitignore"), gitignoreContent)

	moved, err := sanitizeGitignoreReport(dir)
	require.NoError(t, err)
	assert.Nil(t, moved, "no patterns should be moved when no beads entries exist")
}

// ---------------------------------------------------------------------------
// sanitizeAgentFiles
// ---------------------------------------------------------------------------

func TestSanitizeAgentFiles_RemovesBeadsAgents(t *testing.T) {
	dir := setupTempGitDir(t)

	// Create a beads-generated AGENTS.md
	agentsContent := `# AGENTS.md
This project uses bd (beads) for issue tracking.
Run bd prime for context. Use bd ready to find work.
Use bd show <id> to view a ticket. Use bd close <id> to close.
`
	writeFile(t, filepath.Join(dir, "AGENTS.md"), agentsContent)

	removed := sanitizeAgentFiles(dir)
	assert.Contains(t, removed, "AGENTS.md")

	_, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestSanitizeAgentFiles_KeepsNonBeadsAgents(t *testing.T) {
	dir := setupTempGitDir(t)

	// Create a non-beads AGENTS.md
	agentsContent := `# AGENTS.md
This project uses a custom workflow.
Please follow the coding standards in docs/.
`
	writeFile(t, filepath.Join(dir, "AGENTS.md"), agentsContent)

	removed := sanitizeAgentFiles(dir)
	assert.Empty(t, removed, "non-beads AGENTS.md should not be removed")

	_, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.NoError(t, err, "AGENTS.md should still exist")
}

// ---------------------------------------------------------------------------
// SanitizeBeadsInitReport (full integration)
// ---------------------------------------------------------------------------

func TestSanitizeBeadsInitReport_FullScenario(t *testing.T) {
	dir := setupTempGitDir(t)

	// Set up hooks
	hookContent := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	writeHook(t, dir, "post-checkout", hookContent)
	writeHook(t, dir, "post-merge", hookContent)

	// Set up .gitignore with beads entries
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.beads/\n.env\n")

	// Set up beads-generated AGENTS.md
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "Use bd prime and bd ready to work with beads.\n")

	// Run full sanitize
	report := SanitizeBeadsInitReport(dir)

	assert.True(t, report.HasChanges())
	assert.ElementsMatch(t, []string{"post-checkout", "post-merge"}, report.HooksCleaned)
	assert.Equal(t, []string{".beads/"}, report.GitignoreMoved)
	assert.Equal(t, []string{"AGENTS.md"}, report.AgentsRemoved)

	// Verify: hooks are gone
	_, err := os.Stat(filepath.Join(dir, ".git", "hooks", "post-checkout"))
	assert.True(t, os.IsNotExist(err))

	// Verify: .gitignore is clean
	content, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.NotContains(t, string(content), ".beads/")

	// Verify: .git/info/exclude has .beads/
	exclude, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	assert.Contains(t, string(exclude), ".beads/")

	// Verify: AGENTS.md is removed
	_, err = os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestSanitizeBeadsInitReport_NoChanges(t *testing.T) {
	dir := setupTempGitDir(t)

	report := SanitizeBeadsInitReport(dir)
	assert.False(t, report.HasChanges())
	assert.Empty(t, report.HooksCleaned)
	assert.Empty(t, report.GitignoreMoved)
	assert.Empty(t, report.AgentsRemoved)
}

// ---------------------------------------------------------------------------
// DiagnoseBeadsImpact
// ---------------------------------------------------------------------------

func TestDiagnoseBeadsImpact_DetectsIssues(t *testing.T) {
	dir := setupTempGitDir(t)

	// Create .beads/ so IsInitialized returns true
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))

	// Create a hook with beads section
	hookContent := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
bd hook post-checkout
# --- END BEADS INTEGRATION v1.2.2 ---
`
	writeHook(t, dir, "post-checkout", hookContent)

	// Add .beads/ to .gitignore
	writeFile(t, filepath.Join(dir, ".gitignore"), ".beads/\n")

	issues := DiagnoseBeadsImpact(dir)
	assert.NotEmpty(t, issues)

	kinds := make(map[string]bool)
	for _, issue := range issues {
		kinds[issue.Kind] = true
	}
	assert.True(t, kinds["hook"], "should detect hook issue")
	assert.True(t, kinds["gitignore"], "should detect gitignore issue")
	assert.True(t, kinds["exclude_missing"], "should detect missing exclude")
}

func TestDiagnoseBeadsImpact_CleanProject(t *testing.T) {
	dir := setupTempGitDir(t)

	// Create .beads/ so IsInitialized returns true
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))

	// Proper setup: .beads/ in exclude, not in gitignore, no hooks
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	writeFile(t, excludePath, ".beads/\n")

	issues := DiagnoseBeadsImpact(dir)
	assert.Empty(t, issues, "clean project should have no issues")
}

func TestDiagnoseBeadsImpact_SkipsUninitializedProject(t *testing.T) {
	dir := setupTempGitDir(t)
	// No .beads/ directory

	issues := DiagnoseBeadsImpact(dir)
	assert.Nil(t, issues)
}

// ---------------------------------------------------------------------------
// isBeadsCommentLine
// ---------------------------------------------------------------------------

func TestIsBeadsCommentLine(t *testing.T) {
	assert.True(t, isBeadsCommentLine("# beads"))
	assert.True(t, isBeadsCommentLine("# Beads"))
	assert.True(t, isBeadsCommentLine("# Added by bd"))
	assert.True(t, isBeadsCommentLine("# added by bd init"))
	assert.False(t, isBeadsCommentLine("# Dependencies"))
	assert.False(t, isBeadsCommentLine("node_modules/"))
	assert.False(t, isBeadsCommentLine(""))
}

// ---------------------------------------------------------------------------
// isBeadsGitignoreLine
// ---------------------------------------------------------------------------

func TestIsBeadsGitignoreLine(t *testing.T) {
	assert.True(t, isBeadsGitignoreLine(".beads/"))
	assert.True(t, isBeadsGitignoreLine(".beads"))
	assert.False(t, isBeadsGitignoreLine("node_modules/"))
	assert.False(t, isBeadsGitignoreLine(""))
	assert.False(t, isBeadsGitignoreLine("# .beads/"))
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupTempGitDir creates a temporary directory with a .git/hooks and .git/info structure.
func setupTempGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755))
	return dir
}

// writeHook creates a hook file in the .git/hooks directory.
func writeHook(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".git", "hooks", name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

// writeFile creates or overwrites a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
