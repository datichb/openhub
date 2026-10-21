package bricks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentFrontmatterFromBytes_Basic(t *testing.T) {
	t.Parallel()
	input := []byte(`---
id: orchestrator
label: Orchestrator
description: AI project manager
mode: primary
permission:
  question: allow
  bash: deny
  task:
    "*": deny
    "planner": allow
model: anthropic/claude-sonnet-4-6
skills: [skill-a, skill-b]
native_skills: [native-a]
---

# Orchestrator

Body content here.
`)

	fm, err := ParseAgentFrontmatterFromBytes(input)
	require.NoError(t, err)

	assert.Equal(t, "orchestrator", fm.ID)
	assert.Equal(t, "Orchestrator", fm.Label)
	assert.Equal(t, "AI project manager", fm.Description)
	assert.Equal(t, "primary", fm.Mode)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", fm.Model)
	assert.Equal(t, []string{"skill-a", "skill-b"}, fm.Skills)
	assert.Equal(t, []string{"native-a"}, fm.NativeSkills)

	// Check permissions
	assert.Equal(t, "allow", fm.Permission["question"])
	assert.Equal(t, "deny", fm.Permission["bash"])

	taskPerm, ok := fm.Permission["task"].(map[string]interface{})
	require.True(t, ok, "task permission should be a map")
	assert.Equal(t, "deny", taskPerm["*"])
	assert.Equal(t, "allow", taskPerm["planner"])
}

func TestParseAgentFrontmatterFromBytes_Subagent(t *testing.T) {
	t.Parallel()
	input := []byte(`---
id: developer
label: Developer
description: Development assistant
mode: subagent
permission:
  question: deny
  skill: allow
  bash:
    "*": deny
    "bd *": allow
    "git status*": allow
  read: allow
  edit: allow
  write: allow
  task:
    "*": deny
    "documentarian": allow
skills: [dev-standards]
---

# Developer
`)

	fm, err := ParseAgentFrontmatterFromBytes(input)
	require.NoError(t, err)

	assert.Equal(t, "developer", fm.ID)
	assert.Equal(t, "subagent", fm.Mode)
	assert.Equal(t, "", fm.Model) // no model declared

	// Bash permissions are a nested map
	bashPerm, ok := fm.Permission["bash"].(map[string]interface{})
	require.True(t, ok, "bash permission should be a map")
	assert.Equal(t, "deny", bashPerm["*"])
	assert.Equal(t, "allow", bashPerm["bd *"])
	assert.Equal(t, "allow", bashPerm["git status*"])

	// Simple permissions
	assert.Equal(t, "deny", fm.Permission["question"])
	assert.Equal(t, "allow", fm.Permission["read"])
	assert.Equal(t, "allow", fm.Permission["edit"])
}

// The mode defaults to "primary"; C1.6: `verify: true` grants the
// verification commands; unknown keys are ignored.
func TestParseAgentFrontmatterFromBytes_ModeAndVerify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		wantMode   string
		wantVerify bool
	}{
		{"mode defaults to primary, verify to false", "---\nid: reviewer\nlabel: Reviewer\ndescription: Code reviewer\npermission:\n  read: allow\n---\n\n# Reviewer\n", "primary", false},
		{"verify: true, unknown keys ignored", "---\nid: reviewer\nverify: true\nfuture_key: x\n---\n\n# Reviewer\n", "primary", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := ParseAgentFrontmatterFromBytes([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, fm.Mode, "mode")
			assert.Equal(t, tt.wantVerify, fm.Verify, "verify")
		})
	}
}

func TestParseAgentFrontmatterFromBytes_WithComments(t *testing.T) {
	t.Parallel()
	input := []byte(`---
id: developer
label: Developer
description: Dev agent
mode: subagent
permission:
  bash:
    "*": deny
    # Git commands
    "git status*": allow
    "git diff*": allow
    # Package managers
    "npm *": allow
  read: allow
skills: [dev-standards]
---

# Developer
`)

	fm, err := ParseAgentFrontmatterFromBytes(input)
	require.NoError(t, err)

	assert.Equal(t, "developer", fm.ID)

	bashPerm, ok := fm.Permission["bash"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "deny", bashPerm["*"])
	assert.Equal(t, "allow", bashPerm["git status*"])
	assert.Equal(t, "allow", bashPerm["git diff*"])
	assert.Equal(t, "allow", bashPerm["npm *"])
}

func TestParseAgentFrontmatterFromBytes_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"no frontmatter", "# Just a markdown file\n\nNo frontmatter here.\n", "does not start with ---"},
		{"empty frontmatter", "---\n---\n\n# Empty\n", "empty frontmatter"},
		{"unclosed frontmatter", "---\nid: broken\nlabel: Broken\n", "no closing ---"},
		{"empty file", "", "empty file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAgentFrontmatterFromBytes([]byte(tt.input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAgentFamily(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relPath  string
		expected string
	}{
		{"planning/orchestrator.md", "planning"},
		{"developer/developer.md", "developer"},
		{"quality/reviewer.md", "quality"},
		{"auditor/auditor-subagent.md", "auditor"},
		{"design/designer.md", "design"},
		{"documentation/documentarian.md", "documentation"},
		// Edge case: agent at root of agents/ (no family)
		{"standalone.md", ""},
	}

	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			assert.Equal(t, tt.expected, AgentFamily(tt.relPath))
		})
	}
}

func TestParseAgentFrontmatterFromBytes_ComplexPermissions(t *testing.T) {
	t.Parallel()
	// Real-world-like frontmatter with many permission types
	input := []byte(`---
id: pathfinder
label: Pathfinder
description: Reconnaissance agent
mode: primary
permission:
  question: allow
  skill: allow
  bash:
    "*": deny
    "bd list *": allow
    "bd ready": allow
    "ls *": allow
    "git log *": allow
  edit: deny
  write: deny
  websearch: allow
  webfetch: allow
  task:
    "*": deny
    "documentarian": allow
    "designer": allow
  ctx_search: allow
  ctx_stats: allow
  ctx_batch_execute: allow
model: anthropic/claude-sonnet-4-6
skills: [skill-a, skill-b, skill-c]
native_skills: [native-a, native-b]
---

# Pathfinder
`)

	fm, err := ParseAgentFrontmatterFromBytes(input)
	require.NoError(t, err)

	assert.Equal(t, "pathfinder", fm.ID)
	assert.Equal(t, "primary", fm.Mode)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", fm.Model)

	// Verify all permission types
	assert.Equal(t, "allow", fm.Permission["question"])
	assert.Equal(t, "allow", fm.Permission["skill"])
	assert.Equal(t, "deny", fm.Permission["edit"])
	assert.Equal(t, "deny", fm.Permission["write"])
	assert.Equal(t, "allow", fm.Permission["websearch"])
	assert.Equal(t, "allow", fm.Permission["webfetch"])
	assert.Equal(t, "allow", fm.Permission["ctx_search"])
	assert.Equal(t, "allow", fm.Permission["ctx_stats"])
	assert.Equal(t, "allow", fm.Permission["ctx_batch_execute"])

	bashPerm := fm.Permission["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bashPerm["*"])
	assert.Equal(t, "allow", bashPerm["bd list *"])
	assert.Equal(t, "allow", bashPerm["bd ready"])

	taskPerm := fm.Permission["task"].(map[string]interface{})
	assert.Equal(t, "deny", taskPerm["*"])
	assert.Equal(t, "allow", taskPerm["documentarian"])
	assert.Equal(t, "allow", taskPerm["designer"])
}
