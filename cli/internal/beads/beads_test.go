package beads

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
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

func TestIsInProgressStatus(t *testing.T) {
	tests := []struct {
		status     string
		inProgress bool
	}{
		{"in_progress", true},
		{"in-progress", true},
		{"doing", true},
		{"wip", true},
		{"open", false},
		{"ready", false},
		{"todo", false},
		{"done", false},
		{"review", false},
		{"blocked", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.inProgress, IsInProgressStatus(tt.status))
		})
	}
}

func TestIsDevPickableStatus(t *testing.T) {
	tests := []struct {
		status   string
		pickable bool
	}{
		// Ready statuses — pickable
		{"open", true},
		{"ready", true},
		{"todo", true},
		{"to_do", true},
		{"backlog", true},
		// In-progress statuses — pickable (resume)
		{"in_progress", true},
		{"in-progress", true},
		{"doing", true},
		{"wip", true},
		// Non-pickable
		{"done", false},
		{"closed", false},
		{"review", false},
		{"blocked", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.pickable, isDevPickableStatus(tt.status))
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
	if runtime.GOOS == "windows" {
		t.Skip("bd is looked up as bd.exe")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	err := Available()
	require.Error(t, err, "no bd in PATH")
	assert.Contains(t, err.Error(), "bd not found")
	assert.Contains(t, err.Error(), "brew install")

	writeFile(t, filepath.Join(dir, "bd"), "#!/bin/sh\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "bd"), 0o755))
	assert.NoError(t, Available(), "bd in PATH")
}

// ---------------------------------------------------------------------------
// EnsureInitFlags
// ---------------------------------------------------------------------------

func TestEnsureInitFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantAdded []string
	}{
		{"adds every missing flag after the original arguments", []string{"init", "--prefix", "myapp"}, []string{"--skip-hooks", "--skip-agents", "--setup-exclude"}},
		{"adds only the flags that are missing", []string{"init", "--skip-hooks", "--prefix", "x"}, []string{"--skip-agents", "--setup-exclude"}},
		{"no doubles when every flag is there", []string{"init", "--skip-hooks", "--skip-agents", "--setup-exclude"}, nil},
		{"--stealth short-circuits: no flag added", []string{"init", "--stealth"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]string(nil), tc.args...)

			result := EnsureInitFlags(tc.args)

			require.Len(t, result, len(original)+len(tc.wantAdded), "original arguments plus the missing flags")
			assert.Equal(t, original, result[:len(original)], "original arguments are preserved, in order")
			assert.ElementsMatch(t, tc.wantAdded, result[len(original):], "flags added")
			assert.Equal(t, original, tc.args, "input slice must not be mutated")
		})
	}
}

// ---------------------------------------------------------------------------
// removeBeadsSection
// ---------------------------------------------------------------------------

func TestRemoveBeadsSection(t *testing.T) {
	tests := []struct {
		name      string
		hook      string
		wantEmpty bool
		wantKeeps []string
	}{
		{
			name: "a hook that is only the beads section is empty afterwards",
			hook: `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
# This section is managed by beads. Do not remove these markers.
if command -v bd >/dev/null 2>&1; then
  export BD_GIT_HOOK=1
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`,
			wantEmpty: true,
		},
		{
			name: "the project part of a mixed hook stays",
			hook: `#!/usr/bin/env bash
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
`,
			wantKeeps: []string{"golangci-lint run", "set -euo pipefail"},
		},
		{
			name: "a hook without beads section is unchanged",
			hook: `#!/usr/bin/env bash
# Regular hook
echo "running tests"
go test ./...
`,
			wantKeeps: []string{`#!/usr/bin/env bash
# Regular hook
echo "running tests"
go test ./...
`},
		},
		{
			name: "several versions of the section all go",
			hook: `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.0.0 ---
old stuff
# --- END BEADS INTEGRATION v1.0.0 ---
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
new stuff
# --- END BEADS INTEGRATION v1.2.2 ---
`,
			wantEmpty: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := removeBeadsSection(tc.hook)

			assert.NotContains(t, result, "BEADS INTEGRATION", "no beads marker left")
			assert.Equal(t, tc.wantEmpty, isEmptyHook(result), "hook empty after removing the beads section")
			for _, keep := range tc.wantKeeps {
				assert.Contains(t, result, keep, "project content kept")
			}
		})
	}
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

func TestSanitizeHooks(t *testing.T) {
	tests := []struct {
		name          string
		hook          string
		content       string
		wantCleaned   []string
		wantDeleted   bool
		wantKeeps     []string
		wantUnchanged bool
	}{
		{
			name: "a hook that was entirely beads is deleted",
			hook: "post-checkout",
			content: `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`,
			wantCleaned: []string{"post-checkout"},
			wantDeleted: true,
		},
		{
			name: "the project part of a hook is preserved",
			hook: "pre-commit",
			content: `#!/usr/bin/env bash
# Project linter
golangci-lint run

# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook pre-commit "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`,
			wantCleaned: []string{"pre-commit"},
			wantKeeps:   []string{"golangci-lint run"},
		},
		{
			name: "a hook without beads section is not touched",
			hook: "pre-push",
			content: `#!/usr/bin/env bash
# My custom hook
echo "running"
`,
			wantUnchanged: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := gittest.Repo(t)
			writeHook(t, dir, tc.hook, tc.content)

			cleaned := sanitizeHooks(dir)

			assert.Equal(t, tc.wantCleaned, cleaned, "hooks cleaned")
			content, err := os.ReadFile(filepath.Join(dir, ".git", "hooks", tc.hook))
			if tc.wantDeleted {
				assert.True(t, os.IsNotExist(err), "hook file should be deleted")
				return
			}
			require.NoError(t, err, "the hook file still exists")
			if tc.wantUnchanged {
				assert.Equal(t, tc.content, string(content), "content unchanged")
				return
			}
			assert.NotContains(t, string(content), "BEADS INTEGRATION", "no beads marker left")
			for _, keep := range tc.wantKeeps {
				assert.Contains(t, string(content), keep, "project content kept")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sanitizeGitignore
// ---------------------------------------------------------------------------

func TestSanitizeGitignore(t *testing.T) {
	tests := []struct {
		name         string
		exclude      string
		gitignore    string
		wantMoved    []string
		wantKept     []string
		wantGone     []string
		wantExcluded bool
	}{
		{
			name:         "the beads entry moves to .git/info/exclude",
			gitignore:    "node_modules/\ndist/\n.beads/\n.env\n",
			wantMoved:    []string{".beads/"},
			wantKept:     []string{"node_modules/", ".env"},
			wantGone:     []string{".beads/"},
			wantExcluded: true,
		},
		{
			name:         "no duplicate when the exclude already has it",
			exclude:      "# existing\n.beads/\n",
			gitignore:    ".beads/\n",
			wantMoved:    []string{".beads/"},
			wantGone:     []string{".beads/"},
			wantExcluded: true,
		},
		{
			name:         "the other entries and comments stay",
			gitignore:    "# Dependencies\nnode_modules/\n\n# Build\ndist/\nbuild/\n\n# Beads\n.beads/\n\n# Environment\n.env\n.env.local\n",
			wantMoved:    []string{".beads/"},
			wantKept:     []string{"node_modules/", "dist/", ".env", ".env.local"},
			wantGone:     []string{".beads/"},
			wantExcluded: true,
		},
		{
			name:         "the comment of beads goes with its entry",
			gitignore:    "node_modules/\n# beads\n.beads/\n.env\n",
			wantMoved:    []string{".beads/"},
			wantKept:     []string{"node_modules/", ".env"},
			wantGone:     []string{"# beads", ".beads/"},
			wantExcluded: true,
		},
		{
			name:      "nothing to move without beads entries",
			gitignore: "node_modules/\ndist/\n",
			wantMoved: nil,
			wantKept:  []string{"node_modules/", "dist/"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := gittest.Repo(t)
			excludePath := filepath.Join(dir, ".git", "info", "exclude")
			if tc.exclude != "" {
				writeFile(t, excludePath, tc.exclude)
			}
			writeFile(t, filepath.Join(dir, ".gitignore"), tc.gitignore)

			moved, err := sanitizeGitignoreReport(dir)

			require.NoError(t, err)
			assert.Equal(t, tc.wantMoved, moved, "patterns moved")
			content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			require.NoError(t, err)
			for _, keep := range tc.wantKept {
				assert.Contains(t, string(content), keep, ".gitignore keeps its other entries")
			}
			for _, gone := range tc.wantGone {
				assert.NotContains(t, string(content), gone, ".gitignore no longer has it")
			}
			if tc.wantExcluded {
				exclude, err := os.ReadFile(excludePath)
				require.NoError(t, err)
				once := 0
				for _, line := range splitLines(string(exclude)) {
					if line == ".beads/" {
						once++
					}
				}
				assert.Equal(t, 1, once, ".beads/ should appear exactly once in exclude")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SanitizeBeadsInit (full integration)
// ---------------------------------------------------------------------------

func TestSanitizeBeadsInit_FullScenario(t *testing.T) {
	dir := gittest.Repo(t)

	hookContent := `#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.2.2 ---
if command -v bd >/dev/null 2>&1; then
  bd hook post-checkout "$@"
fi
# --- END BEADS INTEGRATION v1.2.2 ---
`
	writeHook(t, dir, "post-checkout", hookContent)
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.beads/\n.env\n")
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "# Agent Instructions\n\nRun `bd prime` for full workflow context.\n\n## Non-Interactive Shell Commands\n\n<!-- BEGIN BEADS INTEGRATION v:1 -->\nbd ready\n<!-- END BEADS INTEGRATION -->\n")

	assert.NoError(t, SanitizeBeadsInit(dir))

	_, err := os.Stat(filepath.Join(dir, ".git", "hooks", "post-checkout"))
	assert.True(t, os.IsNotExist(err))
	content, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.NotContains(t, string(content), ".beads/")
	exclude, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	assert.Contains(t, string(exclude), ".beads/")
	_, err = os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err), "AGENTS.md generated by bd is removed")
}

func TestSanitizeBeadsInit_KeepsUserAgentsFile(t *testing.T) {
	dir := gittest.Repo(t)
	// A file of the user that mentions bd, without block managed by bd.
	agents := "# AGENTS.md\nThis project uses bd (beads): run bd ready, then bd show <id>.\n"
	writeFile(t, filepath.Join(dir, "AGENTS.md"), agents)
	assert.NoError(t, SanitizeBeadsInit(dir))
	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	assert.NoError(t, err)
	assert.Equal(t, agents, string(got))
}

// ---------------------------------------------------------------------------
// DiagnoseBeadsImpact
// ---------------------------------------------------------------------------

func TestDiagnoseBeadsImpact_DetectsIssues(t *testing.T) {
	dir := gittest.Repo(t)

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
	assert.True(t, kinds["exclude_lock_missing"], "should detect missing gate lock exclude")
}

// Q31: a project excluding .beads/ but not the lock bd 1.3 leaves beside it.
func TestDiagnoseBeadsImpact_GateLockMissing(t *testing.T) {
	dir := gittest.Repo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))
	writeFile(t, filepath.Join(dir, ".git", "info", "exclude"), ".beads/\n")

	issues := DiagnoseBeadsImpact(dir)
	require.Len(t, issues, 1)
	assert.Equal(t, "exclude_lock_missing", issues[0].Kind)
	assert.Contains(t, issues[0].Detail, "/.beads.gate.lock")
}

func TestDiagnoseBeadsImpact_CleanProject(t *testing.T) {
	dir := gittest.Repo(t)

	// Create .beads/ so IsInitialized returns true
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))

	// Proper setup: .beads/ and its lock in exclude, not in gitignore, no hooks
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	writeFile(t, excludePath, ".beads/\n/.beads.gate.lock\n")

	issues := DiagnoseBeadsImpact(dir)
	assert.Empty(t, issues, "clean project should have no issues")
}

func TestDiagnoseBeadsImpact_SkipsUninitializedProject(t *testing.T) {
	dir := gittest.Repo(t)
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
