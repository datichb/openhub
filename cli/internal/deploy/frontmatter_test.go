package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentFrontmatterFromBytes_Basic(t *testing.T) {
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
mcpServers: [gitlab]
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
	assert.Equal(t, []string{"gitlab"}, fm.MCPServers)

	// Check permissions
	assert.Equal(t, "allow", fm.Permission["question"])
	assert.Equal(t, "deny", fm.Permission["bash"])

	taskPerm, ok := fm.Permission["task"].(map[string]interface{})
	require.True(t, ok, "task permission should be a map")
	assert.Equal(t, "deny", taskPerm["*"])
	assert.Equal(t, "allow", taskPerm["planner"])
}

func TestParseAgentFrontmatterFromBytes_Subagent(t *testing.T) {
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

func TestParseAgentFrontmatterFromBytes_DefaultMode(t *testing.T) {
	input := []byte(`---
id: reviewer
label: Reviewer
description: Code reviewer
permission:
  read: allow
---

# Reviewer
`)

	fm, err := ParseAgentFrontmatterFromBytes(input)
	require.NoError(t, err)

	// Mode defaults to "primary" when not specified
	assert.Equal(t, "primary", fm.Mode)
}

func TestParseAgentFrontmatterFromBytes_WithComments(t *testing.T) {
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

func TestParseAgentFrontmatterFromBytes_NoFrontmatter(t *testing.T) {
	input := []byte(`# Just a markdown file

No frontmatter here.
`)

	_, err := ParseAgentFrontmatterFromBytes(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not start with ---")
}

func TestParseAgentFrontmatterFromBytes_EmptyFrontmatter(t *testing.T) {
	input := []byte(`---
---

# Empty
`)

	_, err := ParseAgentFrontmatterFromBytes(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty frontmatter")
}

func TestParseAgentFrontmatterFromBytes_UnclosedFrontmatter(t *testing.T) {
	input := []byte(`---
id: broken
label: Broken
`)

	_, err := ParseAgentFrontmatterFromBytes(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no closing ---")
}

func TestParseAgentFrontmatterFromBytes_EmptyFile(t *testing.T) {
	_, err := ParseAgentFrontmatterFromBytes([]byte{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty file")
}

func TestAgentFamily(t *testing.T) {
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
mcpServers: [gitlab]
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
