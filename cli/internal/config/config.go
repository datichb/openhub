// Package config handles reading and writing the hub.toml configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

// Config represents the hub configuration.
type Config struct {
	Name     string          `mapstructure:"name"` // Project/hub display name (shown in TUI title)
	CLI      CLIConfig       `mapstructure:"cli"`
	Opencode OpencodeConfig  `mapstructure:"opencode"`
	Provider ProviderConfigs `mapstructure:"provider"`
	MCP      MCPConfig       `mapstructure:"mcp"`
	Worktree WorktreeConfig  `mapstructure:"worktree"`
	Team     TeamConfig      `mapstructure:"team"`
	Models   ModelsConfig    `mapstructure:"models"`
	// Tracker holds the member's local overrides for the tracker sync feature.
	// Any field left at its zero value means "inherit from the team-state config".
	Tracker TrackerLocalConfig `mapstructure:"tracker"`
}

// ModelsConfig holds the model resolution cascade at the hub level.
// Corresponds to [models], [models.families], [models.agents] in hub.toml.
type ModelsConfig struct {
	Default  string            `mapstructure:"default"`  // hub-level global default model
	Families map[string]string `mapstructure:"families"` // family name → model (e.g., "quality" = "claude-opus-4")
	Agents   map[string]string `mapstructure:"agents"`   // agent-id → model (e.g., "reviewer" = "claude-opus-4")
}

// TeamConfig holds team collaboration settings.
type TeamConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	StateRepo string `mapstructure:"state_repo"` // Git remote URL for the team-state repo
	StatePath string `mapstructure:"state_path"` // Local clone path (default: ~/.oh/team-state)
	MemberID  string `mapstructure:"member_id"`  // Current user's member ID
}

// WorktreeConfig holds git worktree management settings.
type WorktreeConfig struct {
	AutoCleanup   bool   `mapstructure:"auto_cleanup"`
	BaseBranch    string `mapstructure:"base_branch"`    // empty = auto-detect (main/master)
	BranchPattern string `mapstructure:"branch_pattern"` // e.g. "feat/%s"; empty = auto-detect from conventions or heuristic
}

// CLIConfig holds CLI-specific settings.
type CLIConfig struct {
	Language string `mapstructure:"language"`
}

// OpencodeConfig holds opencode dependency settings.
type OpencodeConfig struct {
	Version         string `mapstructure:"version"`
	Channel         string `mapstructure:"channel"`
	AutoUpdate      bool   `mapstructure:"auto_update"`
	InstallDir      string `mapstructure:"install_dir"`
	DefaultProvider string `mapstructure:"default_provider"`
}

// ProviderConfigs holds per-provider non-secret configuration.
// Corresponds to [provider.bedrock], [provider.anthropic], etc. in hub.toml.
type ProviderConfigs struct {
	Bedrock    ProviderConfig `mapstructure:"bedrock"`
	Anthropic  ProviderConfig `mapstructure:"anthropic"`
	OpenRouter ProviderConfig `mapstructure:"openrouter"`
}

// ProviderConfig holds non-secret configuration for a single provider.
type ProviderConfig struct {
	AWSProfile string `mapstructure:"aws_profile"` // AWS profile name (bedrock only)
	AWSRegion  string `mapstructure:"aws_region"`  // AWS region (bedrock only)
	AuthMode   string `mapstructure:"auth_mode"`   // "bearer" | "profile" | "env" (bedrock only)
}

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Figma   MCPServerConfig `mapstructure:"figma"`
	Gitlab  MCPServerConfig `mapstructure:"gitlab"`
	Jira    MCPServerConfig `mapstructure:"jira"`
	Gslides MCPServerConfig `mapstructure:"gslides"`
}

// MCPServerConfig holds individual MCP server settings.
type MCPServerConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Token        string `mapstructure:"token_key"`     // keychain key name, not the secret itself
	WriteEnabled bool   `mapstructure:"write_enabled"` // opt-in for write operations (e.g. GitLab MR creation)
	// URL is an optional per-member override for the service base URL.
	// Useful when the member needs to point to a different instance than
	// the team recommendation stored in team-state config.toml.
	// When empty, the team-state shared URL (if any) or the built-in default is used.
	URL string `mapstructure:"url,omitempty"`
}

// TrackerLocalConfig holds the member's personal overrides for tracker sync settings.
// All pointer fields use nil-means-inherit semantics: a nil value means the member
// has no preference and the corresponding setting from team-state config.toml is used.
// Non-nil values override the team recommendation for this member only.
type TrackerLocalConfig struct {
	// Enabled overrides whether tracker sync is active for this member.
	// nil = inherit team-state. false = disable sync locally (e.g. no network access).
	Enabled *bool `mapstructure:"enabled"`
	// AutoSync overrides whether sync runs automatically on team view open.
	AutoSync *bool `mapstructure:"auto_sync"`
	// PushLabels overrides whether hub labels are pushed back to the tracker.
	// nil = inherit team recommendation. The effective value is also gated by
	// [mcp.<type>].write_enabled — push never happens without write permission.
	PushLabels *bool `mapstructure:"push_labels"`
	// AutoPlanAssigned overrides the auto-plan-from-tracker-assignee behaviour.
	AutoPlanAssigned *bool `mapstructure:"auto_plan_assigned"`
	// MaxAutoPlanPerMember overrides the per-member auto-plan limit.
	MaxAutoPlanPerMember *int `mapstructure:"max_auto_plan_per_member"`
}

var (
	cfg     *Config
	cfgOnce sync.Once
	cfgErr  error
	cfgMu   sync.Mutex
)

// HubDir returns the path to the .oh configuration directory.
func HubDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".oh"
	}
	return filepath.Join(home, ".oh")
}

// ConfigPath returns the full path to hub.toml.
func ConfigPath() string {
	return filepath.Join(HubDir(), "hub.toml")
}

// DefaultTeamStatePath returns the default local path for the team-state repo.
func DefaultTeamStatePath() string {
	return filepath.Join(HubDir(), "team-state")
}

// Load reads the hub.toml configuration. It is safe to call multiple times.
func Load() (*Config, error) {
	cfgOnce.Do(func() {
		v := viper.New()
		v.SetConfigName("hub")
		v.SetConfigType("toml")
		v.AddConfigPath(HubDir())
		v.AddConfigPath(".")

		// Defaults
		v.SetDefault("name", "OpenHub")
		v.SetDefault("cli.language", "en")
		v.SetDefault("opencode.channel", "stable")
		v.SetDefault("opencode.auto_update", false)
		v.SetDefault("opencode.install_dir", filepath.Join(HubDir(), "bin"))
		v.SetDefault("worktree.auto_cleanup", true)
		v.SetDefault("worktree.base_branch", "")
		v.SetDefault("team.enabled", false)
		v.SetDefault("team.state_path", filepath.Join(HubDir(), "team-state"))

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				cfgErr = err
				return
			}
			// Config not found is OK — use defaults
		}

		cfg = &Config{}
		cfgErr = v.Unmarshal(cfg)
	})
	return cfg, cfgErr
}

// Reset clears the cached config (useful for tests).
// Must not be called concurrently with Load.
func Reset() {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgOnce = sync.Once{}
	cfg = nil
	cfgErr = nil
}

// Save writes cfg to hub.toml using a full TOML marshal (comments not preserved).
// After saving, the in-memory cache is invalidated so the next Load re-reads from disk.
func Save(c *Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling hub.toml: %w", err)
	}
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}
	// Invalidate the cache so the next Load() reflects the new state.
	cfgOnce = sync.Once{}
	cfg = nil
	cfgErr = nil
	return nil
}
