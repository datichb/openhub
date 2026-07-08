package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
)

func TestGenerateProjectID_Normal(t *testing.T) {
	id := generateProjectID("my-project")
	// Should be "my-project-<8char uuid>"
	assert.True(t, strings.HasPrefix(id, "my-project-"), "got: %s", id)
	parts := strings.Split(id, "-")
	lastPart := parts[len(parts)-1]
	assert.Len(t, lastPart, 8, "UUID suffix should be 8 chars, got: %s", lastPart)
}

func TestGenerateProjectID_Unicode(t *testing.T) {
	id := generateProjectID("日本語プロジェクト")
	// All non-ascii chars are stripped → slug becomes empty → fallback to "project"
	assert.True(t, strings.HasPrefix(id, "project-"), "got: %s", id)
	parts := strings.Split(id, "-")
	lastPart := parts[len(parts)-1]
	assert.Len(t, lastPart, 8)
}

func TestGenerateProjectID_Long(t *testing.T) {
	longName := strings.Repeat("a", 50)
	id := generateProjectID(longName)
	// Slug is truncated at 32 chars, plus "-" plus 8-char UUID
	// Total max: 32 + 1 + 8 = 41
	assert.LessOrEqual(t, len(id), 41)
	slug := id[:strings.LastIndex(id, "-")]
	assert.LessOrEqual(t, len(slug), 32)
}

func TestGenerateProjectID_WithDashes(t *testing.T) {
	id := generateProjectID("my-cool-project")
	assert.True(t, strings.HasPrefix(id, "my-cool-project-"), "got: %s", id)
}

func TestGenerateProjectID_Spaces(t *testing.T) {
	id := generateProjectID("my cool project")
	// Spaces are converted to dashes
	assert.True(t, strings.HasPrefix(id, "my-cool-project-"), "got: %s", id)
}

func TestGenerateProjectID_SpecialChars(t *testing.T) {
	id := generateProjectID("hello@world!")
	// Special chars are stripped
	assert.True(t, strings.HasPrefix(id, "helloworld-"), "got: %s", id)
}

func TestGenerateProjectID_LeadingTrailingSpaces(t *testing.T) {
	id := generateProjectID("  spaced  ")
	// Should trim and handle properly
	assert.True(t, strings.HasPrefix(id, "spaced-"), "got: %s", id)
}

func TestBuildDeployPlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	cfg, err := config.Load()
	require.NoError(t, err)

	a := &app.App{
		Config: cfg,
		IO:     app.DefaultIOStreams(),
	}

	plan := buildDeployPlan(a, "/tmp/project", "test-id", "/tmp/hub", "anthropic", "claude-3", []string{"coder", "reviewer"}, nil)
	require.NotNil(t, plan)
	assert.Equal(t, "/tmp/project", plan.ProjectPath)
	assert.Equal(t, "test-id", plan.ProjectID)
	assert.Equal(t, "/tmp/hub", plan.HubDir)
	assert.Equal(t, "anthropic", plan.Provider)
	assert.Equal(t, "claude-3", plan.Model)
	assert.Len(t, plan.Phases, 5, "deploy plan should have 5 phases (agents, skills, config, agent-config, mcp)")
}

func TestCmdI18nKey(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *cobra.Command
		expected string
	}{
		{
			name: "root command (no parent)",
			setup: func() *cobra.Command {
				return &cobra.Command{Use: "oh"}
			},
			expected: "cmd.root",
		},
		{
			name: "direct child",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "oh"}
				child := &cobra.Command{Use: "start"}
				root.AddCommand(child)
				return child
			},
			expected: "cmd.start",
		},
		{
			name: "nested child (project list)",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "oh"}
				parent := &cobra.Command{Use: "project"}
				child := &cobra.Command{Use: "list"}
				root.AddCommand(parent)
				parent.AddCommand(child)
				return child
			},
			expected: "cmd.project.list",
		},
		{
			name: "deeply nested (config set)",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "oh"}
				l1 := &cobra.Command{Use: "config"}
				l2 := &cobra.Command{Use: "set"}
				root.AddCommand(l1)
				l1.AddCommand(l2)
				return l2
			},
			expected: "cmd.config.set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setup()
			result := cmdI18nKey(cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateProjectID_Uniqueness(t *testing.T) {
	// Same name should produce different IDs (UUID suffix differs)
	id1 := generateProjectID("test-project")
	id2 := generateProjectID("test-project")
	assert.NotEqual(t, id1, id2, "IDs should be unique due to UUID suffix")
}

func TestGenerateProjectID_EmptyString(t *testing.T) {
	id := generateProjectID("")
	// Empty name → slug becomes empty → fallback to "project"
	assert.True(t, strings.HasPrefix(id, "project-"), "got: %s", id)
}
