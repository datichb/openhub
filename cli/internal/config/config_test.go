package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Reset cached config
	Reset()

	// Point to a temp dir with no config file
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "en", cfg.CLI.Language)
	assert.Empty(t, cfg.Opencode.DefaultProvider)
}

func TestLoad_FromFile(t *testing.T) {
	Reset()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	tomlContent := `
[cli]
language = "fr"

[opencode]
version = "1.17.2"
channel = "beta"
auto_update = true
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(tomlContent), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "fr", cfg.CLI.Language)
	// keys of the former managed install (removed in v5) are ignored
	assert.Empty(t, cfg.Opencode.DefaultProvider)
}

// TestSaveAndLoad_Roundtrip verifies that saving a Config and reloading it
// produces the same values. This catches tag mismatches between mapstructure
// (used by Load/Viper) and toml (used by Save/go-toml).
func TestSaveAndLoad_Roundtrip(t *testing.T) {
	Reset()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	// Build a config with representative values in all fields
	original := &Config{
		Name: "test-hub",
		CLI:  CLIConfig{Language: "fr"},
		Opencode: OpencodeConfig{
			DefaultProvider: "bedrock",
		},
		Provider: ProviderConfigs{
			Bedrock: ProviderConfig{
				AWSProfile: "prod",
				AWSRegion:  "eu-west-1",
				AuthMode:   "profile",
			},
		},
		MCP: MCPConfig{
			Gitlab: MCPServerConfig{
				Enabled:      true,
				Token:        "gitlab-token",
				WriteEnabled: true,
				URL:          "https://gitlab.example.com",
			},
			Jira: MCPServerConfig{
				Enabled: false,
				Token:   "jira-token",
			},
		},
		Worktree: WorktreeConfig{
			AutoCleanup:   true,
			BaseBranch:    "main",
			BranchPattern: "feat/%s",
		},
		Teams: []TeamConfig{
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
		Models: ModelsConfig{
			Default:  "claude-sonnet-4-20250514",
			Families: map[string]string{"quality": "claude-opus-4-20250514"},
			Agents:   map[string]string{"reviewer": "claude-opus-4-20250514"},
		},
		Websearch: WebsearchConfig{Enabled: true},
	}

	// Save
	err := Save(original)
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
	Reset()
	loaded, err := Load()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Verify all fields survived the roundtrip
	assert.Equal(t, "fr", loaded.CLI.Language)
	assert.Equal(t, "bedrock", loaded.Opencode.DefaultProvider)

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

func TestTeamConfig_Validate_Valid(t *testing.T) {
	tc := TeamConfig{
		ID:        "my-team",
		Enabled:   true,
		StateRepo: "git@gitlab.com:team/ts.git",
		MemberID:  "alice",
	}
	assert.NoError(t, tc.Validate())
}

func TestTeamConfig_Validate_DisabledNoRepo(t *testing.T) {
	tc := TeamConfig{
		ID:      "my-team",
		Enabled: false, // disabled — repo and memberID not required
	}
	assert.NoError(t, tc.Validate())
}

func TestTeamConfig_Validate_EmptyID(t *testing.T) {
	tc := TeamConfig{ID: ""}
	err := tc.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestTeamConfig_Validate_InvalidSlug(t *testing.T) {
	cases := []string{"My Team!", "UPPER", "has space", "slash/bad", "under_score"}
	for _, id := range cases {
		tc := TeamConfig{ID: id}
		err := tc.Validate()
		assert.Error(t, err, "ID=%q should fail validation", id)
		assert.Contains(t, err.Error(), "slug")
	}
}

func TestTeamConfig_Validate_MissingRepo(t *testing.T) {
	tc := TeamConfig{
		ID:        "my-team",
		Enabled:   true,
		StateRepo: "",
		MemberID:  "alice",
	}
	err := tc.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state_repo")
}

func TestTeamConfig_Validate_MissingMemberID(t *testing.T) {
	tc := TeamConfig{
		ID:        "my-team",
		Enabled:   true,
		StateRepo: "git@gitlab.com:team/ts.git",
		MemberID:  "",
	}
	err := tc.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "member_id")
}

func TestTeamConfig_Validate_InvalidMemberID(t *testing.T) {
	cases := []string{"foo/bar", "has space", "tab\there", "new\nline", "back\\slash"}
	for _, mid := range cases {
		tc := TeamConfig{
			ID:        "my-team",
			Enabled:   true,
			StateRepo: "git@gitlab.com:team/ts.git",
			MemberID:  mid,
		}
		err := tc.Validate()
		assert.Error(t, err, "memberID=%q should fail", mid)
		assert.Contains(t, err.Error(), "invalid characters")
	}
}

func TestValidateTeams_Unique(t *testing.T) {
	teams := []TeamConfig{
		{ID: "alpha", Enabled: false},
		{ID: "beta", Enabled: false},
	}
	assert.NoError(t, ValidateTeams(teams))
}

func TestValidateTeams_Duplicates(t *testing.T) {
	teams := []TeamConfig{
		{ID: "alpha", Enabled: false},
		{ID: "alpha", Enabled: false},
	}
	err := ValidateTeams(teams)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// ─── Update tests ────────────────────────────────────────────────────────────

func TestUpdate_Basic(t *testing.T) {
	Reset()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	initial := `[cli]
language = "en"
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(initial), 0o644))

	// Update language
	err := Update(func(c *Config) error {
		c.CLI.Language = "fr"
		return nil
	})
	require.NoError(t, err)

	// Reload and verify
	Reset()
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "fr", cfg.CLI.Language)
}

func TestUpdate_ChainedNoExternalModificationError(t *testing.T) {
	Reset()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	initial := `[cli]
language = "en"
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(initial), 0o644))

	// Simulate wizard: multiple sequential Update calls must not trigger
	// ErrExternalModification — this is the core bug fix verification.
	err := Update(func(c *Config) error {
		c.CLI.Language = "fr"
		return nil
	})
	require.NoError(t, err)

	err = Update(func(c *Config) error {
		c.Opencode.DefaultProvider = "bedrock"
		return nil
	})
	require.NoError(t, err)

	err = Update(func(c *Config) error {
		c.MCP.Figma.Enabled = true
		return nil
	})
	require.NoError(t, err)

	err = Update(func(c *Config) error {
		c.Websearch.Enabled = true
		return nil
	})
	require.NoError(t, err)

	// All four updates should succeed and all mutations should be preserved
	Reset()
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "fr", cfg.CLI.Language)
	assert.Equal(t, "bedrock", cfg.Opencode.DefaultProvider)
	assert.Equal(t, true, cfg.MCP.Figma.Enabled)
	assert.Equal(t, true, cfg.Websearch.Enabled)
}

// ─── MCPServer tests ─────────────────────────────────────────────────────────

func TestMCPServer_Dispatch(t *testing.T) {
	c := &Config{}

	assert.NotNil(t, c.MCPServer("figma"))
	assert.NotNil(t, c.MCPServer("gitlab"))
	assert.NotNil(t, c.MCPServer("jira"))
	assert.NotNil(t, c.MCPServer("gslides"))
	assert.NotNil(t, c.MCPServer("Figma")) // case-insensitive
	assert.Nil(t, c.MCPServer("unknown"))

	// Verify pointer identity — mutation through MCPServer must affect the config
	c.MCPServer("gitlab").Enabled = true
	c.MCPServer("gitlab").WriteEnabled = true
	assert.Equal(t, true, c.MCP.Gitlab.Enabled)
	assert.Equal(t, true, c.MCP.Gitlab.WriteEnabled)
}

// ─── ToMap tests ─────────────────────────────────────────────────────────────

func TestToMap_AllKeys(t *testing.T) {
	c := &Config{
		CLI:      CLIConfig{Language: "fr"},
		Opencode: OpencodeConfig{DefaultProvider: "bedrock"},
		MCP: MCPConfig{
			Gitlab: MCPServerConfig{Enabled: true, Token: "gitlab-token"},
		},
		Websearch: WebsearchConfig{Enabled: true},
	}

	m := c.ToMap()
	require.NotNil(t, m)

	assert.Equal(t, "fr", m["cli.language"])
	assert.Equal(t, "bedrock", m["opencode.default_provider"])
	assert.Equal(t, true, m["mcp.gitlab.enabled"])
	assert.Equal(t, "gitlab-token", m["mcp.gitlab.token_key"])
	assert.Equal(t, true, m["websearch.enabled"])
}
