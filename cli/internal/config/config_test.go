package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Reset cached config
	configtest.Home(t)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "en", cfg.CLI.Language)
	assert.Empty(t, cfg.LLM.DefaultProvider)
}

// TestSaveAndLoad_Roundtrip verifies that saving a Config and reloading it
// produces the same values. This catches tag mismatches between mapstructure
// (used by Load/Viper) and toml (used by Save/go-toml).
func TestSaveAndLoad_Roundtrip(t *testing.T) {
	tmpDir := configtest.Home(t)

	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	// Build a config with representative values in all fields
	original := &config.Config{
		Name: "test-hub",
		CLI:  config.CLIConfig{Language: "fr"},
		LLM: config.LLMConfig{
			DefaultProvider: "bedrock",
		},
		Provider: config.ProviderConfigs{
			Bedrock: config.ProviderConfig{
				AWSProfile: "prod",
				AWSRegion:  "eu-west-1",
				AuthMode:   "profile",
			},
		},
		MCP: config.MCPConfig{
			Gitlab: config.MCPServerConfig{
				Enabled:      true,
				Token:        "gitlab-token",
				WriteEnabled: true,
				URL:          "https://gitlab.example.com",
			},
			Jira: config.MCPServerConfig{
				Enabled: false,
				Token:   "jira-token",
			},
		},
		Worktree: config.WorktreeConfig{
			AutoCleanup:   true,
			BaseBranch:    "main",
			BranchPattern: "feat/%s",
		},
		Teams: []config.TeamConfig{
			{
				ID:        "acme",
				Name:      "Equipe ACME",
				Enabled:   true,
				StateRepo: "git@gitlab.com:acme/team-state.git",
				StatePath: "/tmp/team-states/acme",
				MemberID:  "alice",
			},
			{
				ID:        "beta",
				Enabled:   true,
				StateRepo: "git@github.com:beta/ts.git",
				MemberID:  "bob",
			},
		},
		Models: config.ModelsConfig{
			Default:  "claude-sonnet-4-20250514",
			Families: map[string]string{"quality": "claude-opus-4-20250514"},
			Agents:   map[string]string{"reviewer": "claude-opus-4-20250514"},
		},
		Websearch: config.WebsearchConfig{Enabled: true},
	}

	// Save
	err := config.Save(original)
	require.NoError(t, err)

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(ohDir, "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "language")
	assert.Contains(t, string(content), "acme")
	assert.Contains(t, string(content), "gitlab-token")
	// Should NOT contain Go field names (capitalized)
	assert.NotContains(t, string(content), "AutoUpdate")
	assert.NotContains(t, string(content), "StateRepo")
	assert.NotContains(t, string(content), "MemberID")
	assert.NotContains(t, string(content), "DefaultProvider")

	// Reload
	config.Reset()
	loaded, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Verify all fields survived the roundtrip
	assert.Equal(t, "fr", loaded.CLI.Language)
	assert.Equal(t, "bedrock", loaded.LLM.DefaultProvider)

	assert.Equal(t, "prod", loaded.Provider.Bedrock.AWSProfile)
	assert.Equal(t, "eu-west-1", loaded.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "profile", loaded.Provider.Bedrock.AuthMode)

	assert.Equal(t, true, loaded.MCP.Gitlab.Enabled)
	assert.Equal(t, "gitlab-token", loaded.MCP.Gitlab.Token)
	assert.Equal(t, true, loaded.MCP.Gitlab.WriteEnabled)
	assert.Equal(t, "https://gitlab.example.com", loaded.MCP.Gitlab.URL)
	assert.Equal(t, false, loaded.MCP.Jira.Enabled)
	assert.Equal(t, "jira-token", loaded.MCP.Jira.Token)

	assert.Equal(t, true, loaded.Worktree.AutoCleanup)
	assert.Equal(t, "main", loaded.Worktree.BaseBranch)
	assert.Equal(t, "feat/%s", loaded.Worktree.BranchPattern)

	// Teams (multi-team roundtrip)
	require.Len(t, loaded.Teams, 2)
	assert.Equal(t, "acme", loaded.Teams[0].ID)
	assert.Equal(t, "Equipe ACME", loaded.Teams[0].Name)
	assert.Equal(t, true, loaded.Teams[0].Enabled)
	assert.Equal(t, "git@gitlab.com:acme/team-state.git", loaded.Teams[0].StateRepo)
	assert.Equal(t, "/tmp/team-states/acme", loaded.Teams[0].StatePath)
	assert.Equal(t, "alice", loaded.Teams[0].MemberID)
	assert.Equal(t, "beta", loaded.Teams[1].ID)
	assert.Equal(t, "bob", loaded.Teams[1].MemberID)

	// Legacy Team field should be empty (omitempty suppresses it)
	assert.Equal(t, "", loaded.Team.StateRepo)

	// Models
	assert.Equal(t, "claude-sonnet-4-20250514", loaded.Models.Default)
	assert.Equal(t, "claude-opus-4-20250514", loaded.Models.Families["quality"])
	assert.Equal(t, "claude-opus-4-20250514", loaded.Models.Agents["reviewer"])

	// Websearch
	assert.Equal(t, true, loaded.Websearch.Enabled)
}

// ─── TeamConfig.Validate + ValidateTeams tests ───────────────────────────────

func TestTeamConfigValidate(t *testing.T) {
	valid := config.TeamConfig{ID: "my-team", Enabled: true, StateRepo: "git@gitlab.com:team/ts.git", MemberID: "alice"}
	with := func(mod func(*config.TeamConfig)) config.TeamConfig {
		tc := valid
		mod(&tc)
		return tc
	}
	tests := []struct {
		name    string
		team    config.TeamConfig
		wantErr string // "" : valid
	}{
		{"valid", valid, ""},
		{"disabled: the repo and the member are not required", config.TeamConfig{ID: "my-team"}, ""},
		{"empty id", config.TeamConfig{}, "required"},
		{"slug: spaces and punctuation", config.TeamConfig{ID: "My Team!"}, "slug"},
		{"slug: upper case", config.TeamConfig{ID: "UPPER"}, "slug"},
		{"slug: space", config.TeamConfig{ID: "has space"}, "slug"},
		{"slug: slash", config.TeamConfig{ID: "slash/bad"}, "slug"},
		{"slug: underscore", config.TeamConfig{ID: "under_score"}, "slug"},
		{"enabled without a repo", with(func(tc *config.TeamConfig) { tc.StateRepo = "" }), "state_repo"},
		{"enabled without a member", with(func(tc *config.TeamConfig) { tc.MemberID = "" }), "member_id"},
		{"member with a slash", with(func(tc *config.TeamConfig) { tc.MemberID = "foo/bar" }), "invalid characters"},
		{"member with a space", with(func(tc *config.TeamConfig) { tc.MemberID = "has space" }), "invalid characters"},
		{"member with a tab", with(func(tc *config.TeamConfig) { tc.MemberID = "tab\there" }), "invalid characters"},
		{"member with a newline", with(func(tc *config.TeamConfig) { tc.MemberID = "new\nline" }), "invalid characters"},
		{"member with a backslash", with(func(tc *config.TeamConfig) { tc.MemberID = "back\\slash" }), "invalid characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.team.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("ValidateTeams accepts distinct ids", func(t *testing.T) {
		assert.NoError(t, config.ValidateTeams([]config.TeamConfig{{ID: "alpha"}, {ID: "beta"}}))
	})
	t.Run("ValidateTeams refuses duplicates", func(t *testing.T) {
		err := config.ValidateTeams([]config.TeamConfig{{ID: "alpha"}, {ID: "alpha"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
	})
}

// ─── Update tests ────────────────────────────────────────────────────────────

// Sequential updates (the wizard) do not trigger ErrExternalModification and
// every mutation is kept.
func TestUpdate_Chained(t *testing.T) {
	tmpDir := configtest.Home(t)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	initial := `[cli]
language = "en"
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(initial), 0o644))

	// Simulate wizard: multiple sequential Update calls must not trigger
	// ErrExternalModification — this is the core bug fix verification.
	err := config.Update(func(c *config.Config) error {
		c.CLI.Language = "fr"
		return nil
	})
	require.NoError(t, err)

	err = config.Update(func(c *config.Config) error {
		c.LLM.DefaultProvider = "bedrock"
		return nil
	})
	require.NoError(t, err)

	err = config.Update(func(c *config.Config) error {
		c.MCP.Figma.Enabled = true
		return nil
	})
	require.NoError(t, err)

	err = config.Update(func(c *config.Config) error {
		c.Websearch.Enabled = true
		return nil
	})
	require.NoError(t, err)

	// All four updates should succeed and all mutations should be preserved
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "fr", cfg.CLI.Language)
	assert.Equal(t, "bedrock", cfg.LLM.DefaultProvider)
	assert.Equal(t, true, cfg.MCP.Figma.Enabled)
	assert.Equal(t, true, cfg.Websearch.Enabled)
}

// ─── MCPServer tests ─────────────────────────────────────────────────────────

func TestMCPServer_Dispatch(t *testing.T) {
	c := &config.Config{}

	assert.NotNil(t, c.MCPServer("figma"))
	assert.NotNil(t, c.MCPServer("gitlab"))
	assert.NotNil(t, c.MCPServer("jira"))
	assert.NotNil(t, c.MCPServer("gslides"))
	assert.NotNil(t, c.MCPServer("Figma")) // case-insensitive
	assert.Nil(t, c.MCPServer("unknown"))
	assert.Nil(t, c.MCPServer("team"), "team has no hub-level section")

	// github and linear are hub-level services too: enabling them must write.
	c.MCPServer("github").Enabled = true
	c.MCPServer("linear").Token = "openhub.mcp.linear.token"
	assert.True(t, c.MCP.Github.Enabled)
	assert.Equal(t, "openhub.mcp.linear.token", c.MCP.Linear.Token)

	// Verify pointer identity — mutation through MCPServer must affect the config
	c.MCPServer("gitlab").Enabled = true
	c.MCPServer("gitlab").WriteEnabled = true
	assert.Equal(t, true, c.MCP.Gitlab.Enabled)
	assert.Equal(t, true, c.MCP.Gitlab.WriteEnabled)
}

// ─── ToMap tests ─────────────────────────────────────────────────────────────

func TestToMap_AllKeys(t *testing.T) {
	c := &config.Config{
		CLI: config.CLIConfig{Language: "fr"},
		LLM: config.LLMConfig{DefaultProvider: "bedrock"},
		MCP: config.MCPConfig{
			Gitlab: config.MCPServerConfig{Enabled: true, Token: "gitlab-token"},
		},
		Websearch: config.WebsearchConfig{Enabled: true},
	}

	m := c.ToMap()
	require.NotNil(t, m)

	assert.Equal(t, "fr", m["cli.language"])
	assert.Equal(t, "bedrock", m["llm.default_provider"])
	assert.Equal(t, true, m["mcp.gitlab.enabled"])
	assert.Equal(t, "gitlab-token", m["mcp.gitlab.token_key"])
	assert.Equal(t, true, m["websearch.enabled"])
}

func TestServiceTokenKeys(t *testing.T) {
	c := &config.Config{}
	assert.Equal(t, []string{"openhub.mcp.gitlab.token", "gitlab-token"}, config.ServiceTokenKeys(c, "gitlab"))

	c.MCP.Gitlab.Token = "custom-gitlab"
	assert.Equal(t, []string{"custom-gitlab", "openhub.mcp.gitlab.token", "gitlab-token"}, config.ServiceTokenKeys(c, "gitlab"))

	c.MCP.Gitlab.Token = "gitlab-token" // legacy name still configured: listed once
	assert.Equal(t, []string{"gitlab-token", "openhub.mcp.gitlab.token"}, config.ServiceTokenKeys(c, "gitlab"))

	assert.Equal(t, []string{"openhub.mcp.github.token", "github-token"}, config.ServiceTokenKeys(nil, "github"))
}
