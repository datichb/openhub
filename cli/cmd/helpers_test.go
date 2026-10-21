package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestGenerateProjectID(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantPrefix string
	}{
		{"a name", "my-project", "my-project-"},
		{"dashes are kept", "my-cool-project", "my-cool-project-"},
		{"spaces become dashes", "my cool project", "my-cool-project-"},
		{"special characters are stripped", "hello@world!", "helloworld-"},
		{"leading and trailing spaces are trimmed", "  spaced  ", "spaced-"},
		{"non-ASCII only falls back to project", "日本語プロジェクト", "project-"},
		{"empty falls back to project", "", "project-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := generateProjectID(tt.in)
			assert.True(t, strings.HasPrefix(id, tt.wantPrefix), "got: %s", id)
			assert.Len(t, strings.TrimPrefix(id, tt.wantPrefix), 8, "8-char UUID suffix in %s", id)
		})
	}

	t.Run("a long name is cut at 32 characters", func(t *testing.T) {
		id := generateProjectID(strings.Repeat("a", 50))
		assert.LessOrEqual(t, len(id), 41, "32 + dash + 8")
		assert.LessOrEqual(t, len(id[:strings.LastIndex(id, "-")]), 32, "slug of %s", id)
	})

	t.Run("the same name gives different ids", func(t *testing.T) {
		assert.NotEqual(t, generateProjectID("test-project"), generateProjectID("test-project"))
	})
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

func TestBuildMCPServersForProject(t *testing.T) {
	configtest.Home(t)
	tests := []struct {
		name    string
		hub     func(*config.Config)
		project *domain.ProjectMCPConfig
		team    config.ResolvedTeamConfig
		check   func(t *testing.T, servers map[string]bricks.MCPServerDef)
	}{
		{
			name: "no project config: the hub defaults, all disabled on a fresh config",
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				for name, s := range servers {
					assert.False(t, s.Enabled, "server %s", name)
				}
			},
		},
		{
			name: "the project enables services and overrides a token key",
			project: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{
				{Name: "figma", Enabled: boolPtr(true)},
				{Name: "gitlab", Enabled: boolPtr(true), TokenKey: "gitlab-token-myproject"},
			}},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				assert.True(t, servers["figma"].Enabled, "figma")
				assert.True(t, servers["gitlab"].Enabled, "gitlab")
				assert.False(t, servers["gslides"].Enabled, "gslides")
				assert.False(t, servers["team"].Enabled, "team")
				assert.Equal(t, "gitlab-token-myproject", servers["gitlab"].TokenKey)
			},
		},
		{
			name:    "the project disables what the hub enables",
			hub:     func(c *config.Config) { c.MCP.Figma.Enabled = true },
			project: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{{Name: "figma", Enabled: boolPtr(false)}}},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				assert.False(t, servers["figma"].Enabled)
			},
		},
		{
			name:    "an unset enabled inherits the hub and keeps the token override",
			hub:     func(c *config.Config) { c.MCP.Gitlab.Enabled = true },
			project: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{{Name: "gitlab", TokenKey: "gitlab-token-custom"}}},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				assert.True(t, servers["gitlab"].Enabled)
				assert.Equal(t, "gitlab-token-custom", servers["gitlab"].TokenKey)
			},
		},
		{
			name: "github and linear",
			hub: func(c *config.Config) {
				c.MCP.Github = config.MCPServerConfig{Enabled: true, Token: "openhub.mcp.github.token"}
				c.MCP.Linear.Enabled = true
			},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				assert.True(t, servers["github"].Enabled, "github")
				assert.Equal(t, "openhub.mcp.github.token", servers["github"].TokenKey)
				assert.Equal(t, "GITHUB_TOKEN", servers["github"].TokenEnv)
				assert.True(t, servers["linear"].Enabled, "linear")
				assert.Equal(t, "LINEAR_API_KEY", servers["linear"].TokenEnv)
			},
		},
		{
			name: "the team server follows the resolved team: enabled for a project team",
			hub:  func(c *config.Config) { c.Team.Enabled = false },
			team: config.ResolvedTeamConfig{Enabled: true, StateRepo: "git@github.com:acme/team.git", StatePath: "/tmp/team", MemberID: "alice"},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				team, ok := servers["team"]
				require.True(t, ok, "the team server appears in the list")
				assert.True(t, team.Enabled)
			},
		},
		{
			name: "the team server follows the resolved team: disabled by the project",
			hub: func(c *config.Config) {
				c.Team.Enabled = true
				c.Team.StateRepo = "git@gitlab.com:acme/team.git"
			},
			team: config.ResolvedTeamConfig{Enabled: false},
			check: func(t *testing.T, servers map[string]bricks.MCPServerDef) {
				assert.False(t, servers["team"].Enabled)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Reset()
			cfg, err := config.Load()
			require.NoError(t, err)
			if tt.hub != nil {
				tt.hub(cfg)
			}
			byName := map[string]bricks.MCPServerDef{}
			for _, s := range buildMCPServersForProject(&app.App{Config: cfg}, tt.project, tt.team) {
				byName[s.Name] = s
			}
			tt.check(t, byName)
		})
	}
}

func TestBuildProjectMCPConfig(t *testing.T) {
	// Empty list returns nil
	assert.Nil(t, buildProjectMCPConfig(nil))
	assert.Nil(t, buildProjectMCPConfig([]string{}))

	// Non-empty list builds config
	cfg := buildProjectMCPConfig([]string{"figma", "gitlab"})
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Services, 2)
	assert.Equal(t, "figma", cfg.Services[0].Name)
	assert.Equal(t, "gitlab", cfg.Services[1].Name)
	assert.Empty(t, cfg.Services[0].TokenKey) // no override
}
