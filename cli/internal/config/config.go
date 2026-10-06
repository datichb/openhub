// Package config handles reading and writing the hub.toml configuration file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/workflow"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

// ErrExternalModification is returned by Save when the hub.toml file has been
// modified externally (e.g. by a CLI command or another TUI instance) since the
// last save or load performed by this process. Callers should reload from disk
// before retrying.
var ErrExternalModification = errors.New("hub.toml modified externally")

// Config represents the hub configuration.
type Config struct {
	Name     string          `mapstructure:"name" toml:"name,omitempty"` // Project/hub display name (shown in TUI title)
	CLI      CLIConfig       `mapstructure:"cli" toml:"cli"`
	Deploy   DeployConfig    `mapstructure:"deploy" toml:"deploy,omitempty"`
	Opencode OpencodeConfig  `mapstructure:"opencode" toml:"opencode"`
	Provider ProviderConfigs `mapstructure:"provider" toml:"provider"`
	MCP      MCPConfig       `mapstructure:"mcp" toml:"mcp"`
	Worktree WorktreeConfig  `mapstructure:"worktree" toml:"worktree"`
	// Team is the legacy single-team field. Retained for backward-compat reading
	// of hub.toml files that still use the [team] section. On Load, if Team is
	// populated and Teams is empty, it is auto-migrated into Teams[0].
	// New code should use Teams exclusively.
	Team TeamConfig `mapstructure:"team" toml:"team,omitempty"`
	// Teams holds the list of teams the user belongs to (ADR-029).
	// Each project references a team by its ID (Project.TeamID).
	Teams  []TeamConfig `mapstructure:"teams" toml:"teams,omitempty"`
	Models ModelsConfig `mapstructure:"models" toml:"models"`
	// Tracker holds the member's local overrides for the tracker sync feature.
	// Any field left at its zero value means "inherit from the team-state config".
	Tracker TrackerLocalConfig `mapstructure:"tracker" toml:"tracker,omitempty"`
	// Websearch holds web search permission settings (Exa AI).
	Websearch WebsearchConfig `mapstructure:"websearch" toml:"websearch,omitempty"`
	// Workflow holds hub-level workflow overrides (applied on top of the base workflow).
	Workflow *WorkflowHubConfig `mapstructure:"workflow" toml:"workflow,omitempty"`
	// Session holds v5 session settings (how sessions are opened, idle sleep).
	Session SessionConfig `mapstructure:"session" toml:"session,omitempty"`
}

// SessionConfig configures v5 agentic sessions.
type SessionConfig struct {
	// Attach selects how a session client is opened:
	// auto (default) | iterm | terminal | tmux | browser | suspend.
	Attach string `mapstructure:"attach" toml:"attach,omitempty"`
	// ITermStyle selects tab (default) | split | window when iTerm2 is used.
	ITermStyle string `mapstructure:"iterm_style" toml:"iterm_style,omitempty"`
	// IdleSleepMinutes stops an idle tool server after N minutes (default 5).
	IdleSleepMinutes int `mapstructure:"idle_sleep_minutes" toml:"idle_sleep_minutes,omitempty"`
	// Notify controls system notifications for sessions (decisions waiting,
	// end of a turn): "" or "on" (default) | "off".
	Notify string `mapstructure:"notify" toml:"notify,omitempty"`
}

// NotifyEnabled reports whether system notifications are on.
func (s SessionConfig) NotifyEnabled() bool { return s.Notify != "off" }

// WorkflowHubConfig holds workflow customization at the hub level.
type WorkflowHubConfig struct {
	Overrides *workflow.WorkflowOverride `mapstructure:"overrides" toml:"overrides,omitempty"`
}

// FindTeam looks up a team by ID. Returns nil if not found.
func (c *Config) FindTeam(id string) *TeamConfig {
	for i := range c.Teams {
		if c.Teams[i].ID == id {
			return &c.Teams[i]
		}
	}
	return nil
}

// FindTeamByRepo looks up a team by its StateRepo URL. Returns nil if not found.
func (c *Config) FindTeamByRepo(repo string) *TeamConfig {
	for i := range c.Teams {
		if c.Teams[i].StateRepo == repo {
			return &c.Teams[i]
		}
	}
	return nil
}

// MCPServer returns a pointer to the MCPServerConfig for the given service name
// (e.g. "figma", "gitlab", "jira", "gslides"). Returns nil if the name is unknown.
// This allows dynamic dispatch by service name instead of hardcoding field access.
func (c *Config) MCPServer(name string) *MCPServerConfig {
	switch strings.ToLower(name) {
	case "figma":
		return &c.MCP.Figma
	case "gitlab":
		return &c.MCP.Gitlab
	case "jira":
		return &c.MCP.Jira
	case "gslides":
		return &c.MCP.Gslides
	default:
		return nil
	}
}

// DefaultTeam returns the first enabled team, or nil if no teams are configured.
func (c *Config) DefaultTeam() *TeamConfig {
	for i := range c.Teams {
		if c.Teams[i].Enabled {
			return &c.Teams[i]
		}
	}
	if len(c.Teams) > 0 {
		return &c.Teams[0]
	}
	return nil
}

// ModelsConfig holds the model resolution cascade at the hub level.
// Corresponds to [models], [models.families], [models.agents] in hub.toml.
type ModelsConfig struct {
	Default  string            `mapstructure:"default" toml:"default,omitempty"`   // hub-level global default model
	Families map[string]string `mapstructure:"families" toml:"families,omitempty"` // family name → model (e.g., "quality" = "claude-opus-4")
	Agents   map[string]string `mapstructure:"agents" toml:"agents,omitempty"`     // agent-id → model (e.g., "reviewer" = "claude-opus-4")
}

// TeamConfig holds team collaboration settings.
// Each entry represents a team the user belongs to; the hub may reference
// multiple teams via the [[teams]] TOML array (or the legacy [team] section
// for backward-compat single-team setups).
type TeamConfig struct {
	// ID is a short local identifier for referencing this team (e.g. "acme").
	// Used by projects to declare their team affiliation (Project.TeamID).
	// When migrating from the legacy [team] section, ID is auto-derived from
	// the StateRepo URL (last path segment, stripped of ".git").
	ID string `mapstructure:"id" toml:"id"`
	// Name is a human-readable display name (e.g. "Equipe ACME").
	// Optional — if empty, ID is used for display.
	Name      string `mapstructure:"name" toml:"name,omitempty"`
	Enabled   bool   `mapstructure:"enabled" toml:"enabled"`
	StateRepo string `mapstructure:"state_repo" toml:"state_repo"`           // Git remote URL for the team-state repo
	StatePath string `mapstructure:"state_path" toml:"state_path,omitempty"` // Local clone path (default: ~/.oh/team-state)
	MemberID  string `mapstructure:"member_id" toml:"member_id"`             // Current user's member ID
}

// DisplayName returns Name if set, otherwise falls back to ID.
func (t TeamConfig) DisplayName() string {
	if t.Name != "" {
		return t.Name
	}
	return t.ID
}

// validTeamID matches a slug-safe identifier: lowercase letters, digits, hyphens.
var validTeamID = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*$`)

// Validate checks that a TeamConfig is semantically valid.
// Returns an error describing the first invalid field found.
func (t TeamConfig) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("team ID is required")
	}
	if !validTeamID.MatchString(t.ID) {
		return fmt.Errorf("team ID %q must be a lowercase slug (letters, digits, hyphens)", t.ID)
	}
	if t.Enabled {
		if t.StateRepo == "" {
			return fmt.Errorf("team %q: state_repo is required when enabled", t.ID)
		}
		if t.MemberID == "" {
			return fmt.Errorf("team %q: member_id is required when enabled", t.ID)
		}
		if strings.ContainsAny(t.MemberID, " \t\n/\\") {
			return fmt.Errorf("team %q: member_id %q contains invalid characters", t.ID, t.MemberID)
		}
	}
	return nil
}

// ValidateTeams checks a slice of TeamConfigs for individual validity and uniqueness.
func ValidateTeams(teams []TeamConfig) error {
	seen := make(map[string]bool, len(teams))
	for _, t := range teams {
		if err := t.Validate(); err != nil {
			return err
		}
		if seen[t.ID] {
			return fmt.Errorf("duplicate team ID %q", t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}

// WorktreeConfig holds git worktree management settings.
type WorktreeConfig struct {
	AutoCleanup   bool   `mapstructure:"auto_cleanup" toml:"auto_cleanup"`
	BaseBranch    string `mapstructure:"base_branch" toml:"base_branch,omitempty"`       // empty = auto-detect (main/master)
	BranchPattern string `mapstructure:"branch_pattern" toml:"branch_pattern,omitempty"` // e.g. "feat/%s"; empty = auto-detect from conventions or heuristic
}

// CLIConfig holds CLI-specific settings.
type CLIConfig struct {
	Language  string `mapstructure:"language" toml:"language"`
	SetupDone bool   `mapstructure:"setup_done" toml:"setup_done,omitempty"`
}

// DeployConfig holds deployment behavior overrides.
type DeployConfig struct {
	// DisableNativeAgents overrides the default list of opencode native agents to disable.
	// If empty/nil, the built-in default list is used (build, plan, general, explore, scout).
	// Set to an explicit list to control which native agents are disabled on deploy.
	DisableNativeAgents []string `mapstructure:"disable_native_agents" toml:"disable_native_agents,omitempty"`
	// InstructionFiles lists additional project files to include as opencode instructions.
	// These are merged with the built-in defaults (ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md).
	InstructionFiles []string `mapstructure:"instruction_files" toml:"instruction_files,omitempty"`
}

// OpencodeConfig holds opencode dependency settings.
type OpencodeConfig struct {
	Version         string `mapstructure:"version" toml:"version"`
	Channel         string `mapstructure:"channel" toml:"channel"`
	AutoUpdate      bool   `mapstructure:"auto_update" toml:"auto_update"`
	InstallDir      string `mapstructure:"install_dir" toml:"install_dir,omitempty"`
	DefaultProvider string `mapstructure:"default_provider" toml:"default_provider,omitempty"`
}

// ProviderConfigs holds per-provider non-secret configuration.
// Corresponds to [provider.bedrock], [provider.anthropic], etc. in hub.toml.
type ProviderConfigs struct {
	Bedrock    ProviderConfig `mapstructure:"bedrock" toml:"bedrock"`
	Anthropic  ProviderConfig `mapstructure:"anthropic" toml:"anthropic"`
	OpenRouter ProviderConfig `mapstructure:"openrouter" toml:"openrouter"`
}

// ProviderConfig holds non-secret configuration for a single provider.
type ProviderConfig struct {
	AWSProfile string `mapstructure:"aws_profile" toml:"aws_profile,omitempty"` // AWS profile name (bedrock only)
	AWSRegion  string `mapstructure:"aws_region" toml:"aws_region,omitempty"`   // AWS region (bedrock only)
	AuthMode   string `mapstructure:"auth_mode" toml:"auth_mode,omitempty"`     // "bearer" | "profile" | "env" (bedrock only)
}

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Figma   MCPServerConfig `mapstructure:"figma" toml:"figma"`
	Gitlab  MCPServerConfig `mapstructure:"gitlab" toml:"gitlab"`
	Jira    MCPServerConfig `mapstructure:"jira" toml:"jira"`
	Gslides MCPServerConfig `mapstructure:"gslides" toml:"gslides"`
}

// MCPServerConfig holds individual MCP server settings.
type MCPServerConfig struct {
	Enabled      bool   `mapstructure:"enabled" toml:"enabled"`
	Token        string `mapstructure:"token_key" toml:"token_key,omitempty"` // keychain key name, not the secret itself
	WriteEnabled bool   `mapstructure:"write_enabled" toml:"write_enabled"`   // opt-in for write operations (e.g. GitLab MR creation)
	// URL is an optional per-member override for the service base URL.
	// Useful when the member needs to point to a different instance than
	// the team recommendation stored in team-state config.toml.
	// When empty, the team-state shared URL (if any) or the built-in default is used.
	URL string `mapstructure:"url,omitempty" toml:"url,omitempty"`
}

// TrackerLocalConfig holds the member's personal overrides for tracker sync settings.
// All pointer fields use nil-means-inherit semantics: a nil value means the member
// has no preference and the corresponding setting from team-state config.toml is used.
// Non-nil values override the team recommendation for this member only.
type TrackerLocalConfig struct {
	// Enabled overrides whether tracker sync is active for this member.
	// nil = inherit team-state. false = disable sync locally (e.g. no network access).
	Enabled *bool `mapstructure:"enabled" toml:"enabled,omitempty"`
	// AutoSync overrides whether sync runs automatically on team view open.
	AutoSync *bool `mapstructure:"auto_sync" toml:"auto_sync,omitempty"`
	// PushLabels overrides whether hub labels are pushed back to the tracker.
	// nil = inherit team recommendation. The effective value is also gated by
	// [tracker].write_enabled — push never happens without write permission.
	PushLabels *bool `mapstructure:"push_labels" toml:"push_labels,omitempty"`
	// AutoPlanAssigned overrides the auto-plan-from-tracker-assignee behaviour.
	AutoPlanAssigned *bool `mapstructure:"auto_plan_assigned" toml:"auto_plan_assigned,omitempty"`
	// MaxAutoPlanPerMember overrides the per-member auto-plan limit.
	MaxAutoPlanPerMember *int `mapstructure:"max_auto_plan_per_member" toml:"max_auto_plan_per_member,omitempty"`
	// TrackerURL is the base URL of the tracker instance for this member.
	// When set, overrides the team-state TrackerURL.
	// Allows standalone tracker usage without a team or MCP config.
	TrackerURL string `mapstructure:"tracker_url" toml:"tracker_url,omitempty"`
	// TrackerTokenKey is the keychain key for the tracker token.
	// When set, overrides the team-state TrackerTokenKey.
	// Allows standalone tracker usage without MCP config.
	TrackerTokenKey string `mapstructure:"tracker_token_key" toml:"tracker_token_key,omitempty"`
	// WriteEnabled controls whether the tracker can perform write operations
	// (push labels, assign issues). nil = inherit team-state.
	// Independent of MCP write_enabled — the tracker has its own permission.
	WriteEnabled *bool `mapstructure:"write_enabled" toml:"write_enabled,omitempty"`
}

// WebsearchConfig holds web search/fetch permission settings.
// When Enabled is true, agents receive websearch and webfetch permissions via Exa AI.
type WebsearchConfig struct {
	Enabled bool `mapstructure:"enabled" toml:"enabled"`
}

var (
	cfg       *Config
	cfgLoaded bool
	cfgErr    error
	cfgMu     sync.Mutex
	// lastSaveMtime tracks the mtime of hub.toml after the last Save or Load
	// by this process. Used to detect external modifications before auto-save.
	lastSaveMtime time.Time
)

// Default keychain key names for MCP service tokens (ADR-030 convention).
// Pattern: openhub.mcp.<service>.token
// Per-project override pattern: openhub.mcp.<service>.token.<projectID>
const (
	DefaultGitLabTokenKey  = "openhub.mcp.gitlab.token"
	DefaultJiraTokenKey    = "openhub.mcp.jira.token"
	DefaultFigmaTokenKey   = "openhub.mcp.figma.token"
	DefaultGslidesTokenKey = "openhub.mcp.gslides.token"
)

// DefaultTokenKeyForService returns the conventional keychain key name for a service.
func DefaultTokenKeyForService(service string) string {
	switch service {
	case "gitlab":
		return DefaultGitLabTokenKey
	case "jira":
		return DefaultJiraTokenKey
	case "figma":
		return DefaultFigmaTokenKey
	case "gslides":
		return DefaultGslidesTokenKey
	default:
		return "openhub.mcp." + service + ".token"
	}
}

// TeamGitLabTokenKey returns the keychain key for a team-specific GitLab token.
// Pattern: openhub.team.<teamID>.gitlab.token
// This is separate from the MCP and tracker token keys so that teams pointing
// to different GitLab instances do not overwrite each other's tokens.
func TeamGitLabTokenKey(teamID string) string {
	return "openhub.team." + teamID + ".gitlab.token"
}

// HubDir returns the path to the .oh configuration directory.
func HubDir() string {
	// OH_HOME relocates the hub directory (tests, isolated environments).
	if dir := os.Getenv("OH_HOME"); dir != "" {
		return dir
	}
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

// ActiveTeam returns the effective "current team" for backward-compat with code
// that accessed cfg.Team directly. Resolution:
//  1. If Teams has entries → return the first enabled team (or first team if none enabled)
//  2. If Teams is empty but legacy Team has a StateRepo → return legacy Team
//  3. Otherwise → return a zero TeamConfig (disabled)
//
// This method bridges the transition from single-team to multi-team. New code
// should use FindTeam(id) with a project's TeamID instead.
func (c *Config) ActiveTeam() TeamConfig {
	if len(c.Teams) > 0 {
		for _, t := range c.Teams {
			if t.Enabled {
				return t
			}
		}
		return c.Teams[0]
	}
	// Legacy fallback
	if c.Team.StateRepo != "" {
		return c.Team
	}
	return TeamConfig{}
}

// Load reads the hub.toml configuration. It is safe to call from any goroutine.
// The first call reads from disk; subsequent calls return the cached result
// until Reset or Save invalidates it.
func Load() (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	if cfgLoaded {
		return cfg, cfgErr
	}

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
	v.SetDefault("websearch.enabled", false)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			cfgErr = err
			cfgLoaded = true
			return cfg, cfgErr
		}
		// Config not found is OK — use defaults
	}

	cfg = &Config{}
	cfgErr = v.Unmarshal(cfg)
	// Post-load cleanup: if Teams is populated (either from [[teams]] in file
	// or from RunMigrationIfNeeded), clear the legacy Team field to ensure
	// omitempty suppresses it on next Save. Viper may have populated Team
	// from a residual [team] section in the file.
	if cfgErr == nil && len(cfg.Teams) > 0 {
		cfg.Team = TeamConfig{}
	}
	// Record mtime for external modification detection.
	if cfgErr == nil {
		recordMtime()
	}
	cfgLoaded = true
	return cfg, cfgErr
}

// Reset clears the cached config so the next Load re-reads from disk.
// Safe to call concurrently with Load — both are serialized by cfgMu.
func Reset() {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgLoaded = false
	cfg = nil
	cfgErr = nil
	lastSaveMtime = time.Time{}
}

// Save writes cfg to hub.toml using a full TOML marshal (comments not preserved).
// The write is atomic (tmp file + rename) to prevent corruption on crash.
// After saving, the in-memory cache is invalidated so the next Load re-reads from disk.
//
// If the file was modified externally since the last Save/Load by this process,
// ErrExternalModification is returned. The caller should reload before retrying.
func Save(c *Config) error {
	// Validate teams before persisting
	if len(c.Teams) > 0 {
		if err := ValidateTeams(c.Teams); err != nil {
			return fmt.Errorf("invalid team config: %w", err)
		}
	}

	cfgMu.Lock()
	defer cfgMu.Unlock()

	// Check for external modifications before writing.
	if err := checkExternalModification(); err != nil {
		return err
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling hub.toml: %w", err)
	}
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	// Atomic write: write to a temp file in the same directory, then rename.
	// This prevents a half-written hub.toml on crash or power loss.
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("writing hub.toml tmp: %w", err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile) // best-effort cleanup
		return fmt.Errorf("renaming hub.toml tmp: %w", err)
	}

	// Record the new mtime after successful write.
	recordMtime()

	// Invalidate the cache so the next Load() reflects the new state.
	cfgLoaded = false
	cfg = nil
	cfgErr = nil
	return nil
}

// Update loads the current configuration, applies fn to mutate it, and saves the
// result atomically. This is the recommended way to modify hub.toml for simple
// mutations — it ensures the Load-Mutate-Save cycle is performed correctly.
//
// For complex flows that need to inspect the config before deciding what to change,
// use Load() + direct mutation + Save() manually.
func Update(fn func(*Config) error) error {
	c, err := Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := fn(c); err != nil {
		return err
	}
	return Save(c)
}

// ToMap serializes the config to a flat dotted-key map suitable for display
// (e.g. "oh config list", "oh config get <key>"). The map keys use the TOML
// dotted notation (e.g. "cli.language", "mcp.gitlab.enabled").
func (c *Config) ToMap() map[string]interface{} {
	data, err := toml.Marshal(c)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil
	}
	flat := make(map[string]interface{})
	flattenMap("", m, flat)
	return flat
}

// flattenMap recursively flattens a nested map into dotted keys.
func flattenMap(prefix string, src map[string]interface{}, dst map[string]interface{}) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMap(key, val, dst)
		case []interface{}:
			// Keep arrays as-is (e.g. [[teams]], deploy.disable_native_agents)
			dst[key] = val
		default:
			dst[key] = val
		}
	}
}

// checkExternalModification detects whether hub.toml was modified by another
// process since we last read or wrote it. Must be called under cfgMu.
func checkExternalModification() error {
	if lastSaveMtime.IsZero() {
		return nil // first save, nothing to compare against
	}
	path := ConfigPath()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file deleted externally — we'll recreate it
		}
		return fmt.Errorf("stat hub.toml: %w", err)
	}
	if info.ModTime().After(lastSaveMtime.Add(time.Millisecond)) {
		return ErrExternalModification
	}
	return nil
}

// recordMtime records the current mtime of hub.toml. Must be called under cfgMu.
func recordMtime() {
	path := ConfigPath()
	if info, err := os.Stat(path); err == nil {
		lastSaveMtime = info.ModTime()
	}
}
