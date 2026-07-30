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
	assert.Equal(t, "stable", cfg.Opencode.Channel)
	assert.Equal(t, false, cfg.Opencode.AutoUpdate)
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
	assert.Equal(t, "1.17.2", cfg.Opencode.Version)
	assert.Equal(t, "beta", cfg.Opencode.Channel)
	assert.Equal(t, true, cfg.Opencode.AutoUpdate)
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
			Version:         "2.0.0",
			Channel:         "beta",
			AutoUpdate:      true,
			InstallDir:      "/usr/local/bin",
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
	assert.Equal(t, "2.0.0", loaded.Opencode.Version)
	assert.Equal(t, "beta", loaded.Opencode.Channel)
	assert.Equal(t, true, loaded.Opencode.AutoUpdate)
	assert.Equal(t, "/usr/local/bin", loaded.Opencode.InstallDir)
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
}
