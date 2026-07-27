package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
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

	plan := buildDeployPlan(a, "/tmp/project", "test-id", "/tmp/hub", "anthropic", "claude-3", []string{"coder", "reviewer"}, nil, nil, nil)
	require.NotNil(t, plan)
	assert.Equal(t, "/tmp/project", plan.ProjectPath)
	assert.Equal(t, "test-id", plan.ProjectID)
	assert.Equal(t, "/tmp/hub", plan.HubDir)
	assert.Equal(t, "anthropic", plan.Provider)
	assert.Equal(t, "claude-3", plan.Model)
	assert.Len(t, plan.Phases, 6, "deploy plan should have 6 phases (agents, skills, config, agent-config, mcp, team)")
}

func TestBuildDeployPlan_TeamDisabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	// Hub has team enabled
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Team.Enabled = true
	cfg.Team.StateRepo = "git@gitlab.com:acme/team.git"
	cfg.Team.MemberID = "alice"

	a := &app.App{Config: cfg, IO: app.DefaultIOStreams()}

	// Project explicitly opts out
	projectTeamCfg := &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
	plan := buildDeployPlan(a, "/tmp/project", "test-id", "/tmp/hub", "", "", nil, nil, nil, projectTeamCfg)
	require.NotNil(t, plan)

	// Team MCP server should NOT appear in EnabledMCPServers
	for _, name := range plan.EnabledMCPServers {
		assert.NotEqual(t, "team", name, "team MCP must not be enabled for a disabled project")
	}
}

func TestBuildDeployPlan_TeamCustom(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	// Hub has no team — but project has custom config
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Team.Enabled = false

	a := &app.App{Config: cfg, IO: app.DefaultIOStreams()}

	projectTeamCfg := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@github.com:beta/other-team.git",
		MemberID:  "bob",
	}
	plan := buildDeployPlan(a, "/tmp/project", "test-id", "/tmp/hub", "", "", nil, nil, nil, projectTeamCfg)
	require.NotNil(t, plan)

	// Team MCP server SHOULD appear in EnabledMCPServers
	found := false
	for _, name := range plan.EnabledMCPServers {
		if name == "team" {
			found = true
		}
	}
	assert.True(t, found, "team MCP must be enabled for a custom-mode project")
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

func TestBuildMCPServersForProject_NilConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	a := &app.App{Config: cfg}

	// nil MCPConfig → inherit hub defaults (all disabled since fresh config)
	servers := buildMCPServersForProject(a, nil, config.ResolvedTeamConfig{})
	for _, s := range servers {
		assert.False(t, s.Enabled, "server %s should be disabled with nil MCPConfig", s.Name)
	}
}

func TestBuildMCPServersForProject_ProjectOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	a := &app.App{Config: cfg}

	// Project explicitly enables figma and gitlab (with token override)
	mcpCfg := &domain.ProjectMCPConfig{
		Services: []domain.ProjectMCPService{
			{Name: "figma", Enabled: boolPtr(true)},
			{Name: "gitlab", Enabled: boolPtr(true), TokenKey: "gitlab-token-myproject"},
		},
	}

	servers := buildMCPServersForProject(a, mcpCfg, config.ResolvedTeamConfig{})

	enabledNames := map[string]bool{}
	for _, s := range servers {
		if s.Enabled {
			enabledNames[s.Name] = true
		}
	}
	assert.True(t, enabledNames["figma"], "figma should be enabled")
	assert.True(t, enabledNames["gitlab"], "gitlab should be enabled")
	assert.False(t, enabledNames["gslides"], "gslides should not be enabled")
	assert.False(t, enabledNames["team"], "team should not be enabled")

	// Check token key override for gitlab
	for _, s := range servers {
		if s.Name == "gitlab" {
			assert.Equal(t, "gitlab-token-myproject", s.TokenKey)
		}
	}
}

func TestBuildMCPServersForProject_EnabledOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	// Hub has figma disabled (default fresh config)
	a := &app.App{Config: cfg}

	// Project force-enables figma
	mcpCfg := &domain.ProjectMCPConfig{
		Services: []domain.ProjectMCPService{
			{Name: "figma", Enabled: boolPtr(true)},
		},
	}

	servers := buildMCPServersForProject(a, mcpCfg, config.ResolvedTeamConfig{})
	for _, s := range servers {
		if s.Name == "figma" {
			assert.True(t, s.Enabled, "figma should be force-enabled by project override")
		}
	}
}

func TestBuildMCPServersForProject_DisabledOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	// Manually enable figma at hub level
	cfg.MCP.Figma.Enabled = true
	a := &app.App{Config: cfg}

	// Project force-disables figma
	mcpCfg := &domain.ProjectMCPConfig{
		Services: []domain.ProjectMCPService{
			{Name: "figma", Enabled: boolPtr(false)},
		},
	}

	servers := buildMCPServersForProject(a, mcpCfg, config.ResolvedTeamConfig{})
	for _, s := range servers {
		if s.Name == "figma" {
			assert.False(t, s.Enabled, "figma should be force-disabled by project override")
		}
	}
}

func TestBuildMCPServersForProject_NilEnabledInheritsHub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	// Hub has gitlab enabled
	cfg.MCP.Gitlab.Enabled = true
	a := &app.App{Config: cfg}

	// Project lists gitlab with Enabled=nil (only overrides token key)
	mcpCfg := &domain.ProjectMCPConfig{
		Services: []domain.ProjectMCPService{
			{Name: "gitlab", TokenKey: "gitlab-token-custom"},
		},
	}

	servers := buildMCPServersForProject(a, mcpCfg, config.ResolvedTeamConfig{})
	for _, s := range servers {
		if s.Name == "gitlab" {
			assert.True(t, s.Enabled, "gitlab should inherit hub enabled=true when project Enabled is nil")
			assert.Equal(t, "gitlab-token-custom", s.TokenKey)
		}
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

func TestBuildMCPServersForProject_TeamEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	// Hub has team disabled
	cfg.Team.Enabled = false
	a := &app.App{Config: cfg}

	// Resolved team is enabled (e.g. project has custom mode)
	resolvedTeam := config.ResolvedTeamConfig{
		Enabled:   true,
		StateRepo: "git@github.com:acme/team.git",
		StatePath: "/tmp/team",
		MemberID:  "alice",
	}

	servers := buildMCPServersForProject(a, nil, resolvedTeam)
	var teamServer *deploy.MCPServerDef
	for i := range servers {
		if servers[i].Name == "team" {
			teamServer = &servers[i]
		}
	}
	require.NotNil(t, teamServer, "team server must appear in the list")
	assert.True(t, teamServer.Enabled, "team server must be enabled when resolvedTeam.Enabled=true")
}

func TestBuildMCPServersForProject_TeamDisabledViaResolvedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()
	cfg, _ := config.Load()
	// Hub has team enabled
	cfg.Team.Enabled = true
	cfg.Team.StateRepo = "git@gitlab.com:acme/team.git"
	a := &app.App{Config: cfg}

	// Resolved team is disabled (project used disabled mode)
	resolvedTeam := config.ResolvedTeamConfig{Enabled: false}

	servers := buildMCPServersForProject(a, nil, resolvedTeam)
	for _, s := range servers {
		if s.Name == "team" {
			assert.False(t, s.Enabled, "team server must be disabled when resolvedTeam.Enabled=false")
		}
	}
}
