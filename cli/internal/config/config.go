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

	"github.com/datichb/openhub/cli/internal/fsutil"
	"github.com/datichb/openhub/cli/internal/limits"
	toml "github.com/pelletier/go-toml/v2"
)

// ErrExternalModification is returned by Save when hub.toml changed on disk
// since the Config being saved was read from it (by a CLI command, another TUI
// instance or another part of this process). The protection follows the
// Config that is saved, not the process: a copy read before the change is
// refused even if hub.toml was read again since. Callers should not retry the
// same copy: they change only their own fields with Update, which re-reads
// hub.toml under the lock.
var ErrExternalModification = errors.New("hub.toml modified externally")

// Config represents the hub configuration.
type Config struct {
	Name     string          `toml:"name,omitempty"` // Project/hub display name (shown in TUI title)
	CLI      CLIConfig       `toml:"cli"`
	Deploy   DeployConfig    `toml:"deploy,omitempty"`
	LLM      LLMConfig       `toml:"llm"`
	Provider ProviderConfigs `toml:"provider"`
	MCP      MCPConfig       `toml:"mcp"`
	Worktree WorktreeConfig  `toml:"worktree"`
	// Team is the legacy single-team field. Retained for backward-compat reading
	// of hub.toml files that still use the [team] section. On Load, if Team is
	// populated and Teams is empty, it is auto-migrated into Teams[0].
	// New code should use Teams exclusively.
	Team TeamConfig `toml:"team,omitempty"`
	// Teams holds the list of teams the user belongs to (ADR-029).
	// Each project references a team by its ID (Project.TeamID).
	Teams  []TeamConfig `toml:"teams,omitempty"`
	Models ModelsConfig `toml:"models"`
	// Tracker holds the member's local overrides for the tracker sync feature.
	// Any field left at its zero value means "inherit from the team-state config".
	Tracker TrackerLocalConfig `toml:"tracker,omitempty"`
	// Websearch holds web search permission settings (Exa AI).
	Websearch WebsearchConfig `toml:"websearch,omitempty"`
	// Session holds v5 session settings (how sessions are opened, idle sleep).
	Session SessionConfig `toml:"session,omitempty"`
	// Execution holds the v5 execution settings (default runtime, container
	// engine, image cache, pinned tool version, strict isolation).
	Execution ExecutionConfig `toml:"execution,omitempty"`
	// Remote holds the GitLab CI targets of the remote runtime (v5 phase 5).
	Remote RemoteConfig `toml:"remote,omitempty"`
	// Fanout holds the fan-out settings (R6, experimental, off by default).
	Fanout FanoutConfig `toml:"fanout,omitempty"`
	// Limits holds the optional session restrictions of the hub level (I6,
	// off by default; cascade in internal/limits).
	Limits limits.Limits `toml:"limits,omitempty"`
	// Beads holds the Beads settings of the hub (zero impact clean-up).
	Beads BeadsConfig `toml:"beads,omitempty"`
	// GC holds the clean-up settings (oh gc and the daemon, N16).
	GC GCConfig `toml:"gc,omitempty"`

	// src is the state of hub.toml this Config was read from (or last saved
	// to): Save compares it with the file to refuse a stale copy. It is copied
	// along with the struct, never written to the file.
	src fileStamp
}

// ExecutionConfig configures where v5 sessions run.
type ExecutionConfig struct {
	// Runtime is the preferred runtime (local | container), used when the
	// workflow allows it and the project sets none ("" = workflow default).
	Runtime string `toml:"runtime,omitempty"`
	// Engine selects the container engine: auto (default) | colima | podman | docker.
	Engine string `toml:"engine,omitempty"`
	// ColimaProfile is the Colima profile used by the colima engine
	// ("" = $COLIMA_PROFILE, else "default").
	ColimaProfile string `toml:"colima_profile,omitempty"`
	// KeepImages is the number of container images kept per project and
	// role (base, dev) — default 2.
	KeepImages int `toml:"keep_images,omitempty"`
	// ToolVersion pins the tool version of the container images: when set
	// and different from the machine client, container launches are refused
	// ("" = the machine client version).
	ToolVersion string `toml:"tool_version,omitempty"`
	// StrictIsolation hides the user tool configuration (XDG_CONFIG_HOME) of
	// local servers too (containers never see it).
	StrictIsolation bool `toml:"strict_isolation,omitempty"`
	// ProxyFirstByteTimeoutMinutes is how long the credential proxy waits
	// for a model provider to start answering before it gives up on the
	// request (default 5). A stream, once started, has no time limit.
	ProxyFirstByteTimeoutMinutes int `toml:"proxy_first_byte_timeout_minutes,omitempty"`
	// EnvAllow adds variables of the user's environment to those the tool
	// server of a local session inherits (and so the shells of the agents).
	// Only a short base list passes (PATH, HOME, LANG, proxies…): names, or
	// PREFIX_* patterns. Containers and remote jobs never inherit.
	EnvAllow []string `toml:"env_allow,omitempty"`
}

// DefaultKeepImages is the default number of container images kept.
const DefaultKeepImages = 2

// DefaultProxyFirstByteTimeoutMinutes is the default wait of the proxy for
// the start of a provider answer: slow models take minutes before the first
// byte.
const DefaultProxyFirstByteTimeoutMinutes = 5

// ProxyFirstByteTimeout returns how long the proxy waits for a provider to
// start answering (default 5 minutes).
func (e ExecutionConfig) ProxyFirstByteTimeout() time.Duration {
	if e.ProxyFirstByteTimeoutMinutes <= 0 {
		return DefaultProxyFirstByteTimeoutMinutes * time.Minute
	}
	return time.Duration(e.ProxyFirstByteTimeoutMinutes) * time.Minute
}

// Images returns the number of container images kept (default 2).
func (e ExecutionConfig) Images() int {
	if e.KeepImages <= 0 {
		return DefaultKeepImages
	}
	return e.KeepImages
}

// SessionConfig configures v5 agentic sessions.
type SessionConfig struct {
	// Attach selects how a session client is opened:
	// auto (default) | iterm | terminal | tmux | browser | suspend.
	Attach string `toml:"attach,omitempty"`
	// ITermStyle selects tab (default) | split | window when iTerm2 is used.
	ITermStyle string `toml:"iterm_style,omitempty"`
	// IdleSleepMinutes stops an idle tool server after N minutes (default 5).
	IdleSleepMinutes int `toml:"idle_sleep_minutes,omitempty"`
	// Notify controls system notifications for sessions (decisions waiting,
	// end of a turn): "" or "on" (default) | "off".
	Notify string `toml:"notify,omitempty"`
}

// NotifyEnabled reports whether system notifications are on.
func (s SessionConfig) NotifyEnabled() bool { return s.Notify != "off" }

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

// UpsertTeam saves t in Teams without touching any other entry: it replaces
// the entry with the same ID or, when t has a StateRepo, the same StateRepo
// (keeping its Name and its GitLabURL if t has none), and appends t otherwise.
func (c *Config) UpsertTeam(t TeamConfig) {
	for i := range c.Teams {
		cur := c.Teams[i]
		if cur.ID != t.ID && (t.StateRepo == "" || cur.StateRepo != t.StateRepo) {
			continue
		}
		if t.Name == "" {
			t.Name = cur.Name
		}
		if t.GitLabURL == "" && t.StateRepo == cur.StateRepo {
			t.GitLabURL = cur.GitLabURL
		}
		c.Teams[i] = t
		return
	}
	c.Teams = append(c.Teams, t)
}

// MCPServer returns a pointer to the MCPServerConfig for the given service name
// (e.g. "figma", "gitlab", "jira", "gslides", "github", "linear"). Returns nil
// for a service without hub-level section: "team" (derived from the team
// config) and the custom servers, which are configured per project only.
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
	case "github":
		return &c.MCP.Github
	case "linear":
		return &c.MCP.Linear
	default:
		return nil
	}
}

// DefaultTeam returns the first enabled team, or nil if no teams are
// configured. Solo teams are ignored.
func (c *Config) DefaultTeam() *TeamConfig {
	for i := range c.Teams {
		if c.Teams[i].Enabled && !c.Teams[i].Solo {
			return &c.Teams[i]
		}
	}
	for i := range c.Teams {
		if !c.Teams[i].Solo {
			return &c.Teams[i]
		}
	}
	return nil
}

// ModelsConfig holds the model resolution cascade at the hub level.
// Corresponds to [models], [models.families], [models.agents] in hub.toml.
type ModelsConfig struct {
	Default  string            `toml:"default,omitempty"`  // hub-level global default model
	Families map[string]string `toml:"families,omitempty"` // family name → model (e.g., "quality" = "claude-opus-4")
	Agents   map[string]string `toml:"agents,omitempty"`   // agent-id → model (e.g., "reviewer" = "claude-opus-4")
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
	ID string `toml:"id"`
	// Name is a human-readable display name (e.g. "Equipe ACME").
	// Optional — if empty, ID is used for display.
	Name      string `toml:"name,omitempty"`
	Enabled   bool   `toml:"enabled"`
	StateRepo string `toml:"state_repo"`           // Git remote URL for the team-state repo
	StatePath string `toml:"state_path,omitempty"` // Local clone path (default: ~/.oh/team-state)
	MemberID  string `toml:"member_id"`            // Current user's member ID
	// GitLabURL is the GitLab server that hosts the team-state repository,
	// declared by the member when state_repo reaches it through an SSH alias
	// (git@mon-alias:acme/team.git): the alias is a name of ~/.ssh/config, not
	// the server, so the token of the team would otherwise be attached to
	// "mon-alias" and never match a GitLab URL. Personal (the alias is local to
	// the machine): it lives in hub.toml. Empty = the host of state_repo.
	GitLabURL string `toml:"gitlab_url,omitempty"`
	// Solo marks a local team-state without remote (v5 phase 2, `oh team
	// init --solo`): it only holds workflows; team features (board, claims,
	// notifications) stay off and ActiveTeam ignores it.
	Solo bool `toml:"solo,omitempty"`
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
		if t.Solo && t.StatePath == "" {
			return fmt.Errorf("team %q: state_path is required for a solo team", t.ID)
		}
		if t.StateRepo == "" && !t.Solo {
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
	AutoCleanup   bool   `toml:"auto_cleanup"`
	BaseBranch    string `toml:"base_branch,omitempty"`    // empty = auto-detect (main/master)
	BranchPattern string `toml:"branch_pattern,omitempty"` // e.g. "feat/%s"; empty = auto-detect from conventions or heuristic
}

// CLIConfig holds CLI-specific settings.
type CLIConfig struct {
	Language  string `toml:"language"`
	SetupDone bool   `toml:"setup_done,omitempty"`
	// Compact is the compact layout of the TUI: auto (under 32 rows, the
	// default), always or never.
	Compact string `toml:"compact,omitempty"`
}

// DeployConfig holds the `[deploy]` section. Only the project instruction
// files remain since v5 (the closed world of the session bundles replaced
// `disable_native_agents`, D13).
type DeployConfig struct {
	// InstructionFiles lists additional project files embedded as
	// instructions in every agent body of a session bundle.
	// These are merged with the built-in defaults (ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md).
	InstructionFiles []string `toml:"instruction_files,omitempty"`
}

// LLMConfig holds the LLM settings of the hub ([llm]; read from its former
// section too, legacy_keys.go).
type LLMConfig struct {
	DefaultProvider string `toml:"default_provider,omitempty"`
}

// ProviderConfigs holds per-provider non-secret configuration.
// Corresponds to [provider.bedrock], [provider.anthropic], etc. in hub.toml.
type ProviderConfigs struct {
	Bedrock    ProviderConfig `toml:"bedrock"`
	Anthropic  ProviderConfig `toml:"anthropic"`
	OpenRouter ProviderConfig `toml:"openrouter"`
}

// ProviderConfig holds non-secret configuration for a single provider.
type ProviderConfig struct {
	AWSProfile string `toml:"aws_profile,omitempty"` // AWS profile name (bedrock only)
	AWSRegion  string `toml:"aws_region,omitempty"`  // AWS region (bedrock only)
	AuthMode   string `toml:"auth_mode,omitempty"`   // "bearer" | "profile" | "env" (bedrock only)
}

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Figma   MCPServerConfig `toml:"figma"`
	Gitlab  MCPServerConfig `toml:"gitlab"`
	Jira    MCPServerConfig `toml:"jira"`
	Gslides MCPServerConfig `toml:"gslides"`
	Github  MCPServerConfig `toml:"github"`
	Linear  MCPServerConfig `toml:"linear"`
}

// MCPServerConfig holds individual MCP server settings.
type MCPServerConfig struct {
	Enabled      bool   `toml:"enabled"`
	Token        string `toml:"token_key,omitempty"` // keychain key name, not the secret itself
	WriteEnabled bool   `toml:"write_enabled"`       // opt-in for write operations (e.g. GitLab MR creation)
	// URL is an optional per-member override for the service base URL.
	// Useful when the member needs to point to a different instance than
	// the team recommendation stored in team-state config.toml.
	// When empty, the team-state shared URL (if any) or the built-in default is used.
	URL string `toml:"url,omitempty"`
	// TrustedURL is the URL the member approved for this service when the
	// team imposes one (`oh mcp trust`, audit v5 S-31). The URL imposed by
	// the team is used only when it equals this one (mcpresolve). It lives
	// in the personal hub.toml, never in the team-state.
	TrustedURL string `toml:"trusted_url,omitempty"`
}

// TrackerLocalConfig holds the member's personal overrides for tracker sync settings.
// All pointer fields use nil-means-inherit semantics: a nil value means the member
// has no preference and the corresponding setting from team-state config.toml is used.
// Non-nil values override the team recommendation for this member only.
type TrackerLocalConfig struct {
	// Enabled overrides whether tracker sync is active for this member.
	// nil = inherit team-state. false = disable sync locally (e.g. no network access).
	Enabled *bool `toml:"enabled,omitempty"`
	// AutoSync overrides whether sync runs automatically on team view open.
	AutoSync *bool `toml:"auto_sync,omitempty"`
	// PushLabels overrides whether hub labels are pushed back to the tracker.
	// nil = inherit team recommendation. The effective value is also gated by
	// [tracker].write_enabled — push never happens without write permission.
	PushLabels *bool `toml:"push_labels,omitempty"`
	// AutoPlanAssigned overrides the auto-plan-from-tracker-assignee behaviour.
	AutoPlanAssigned *bool `toml:"auto_plan_assigned,omitempty"`
	// MaxAutoPlanPerMember overrides the per-member auto-plan limit.
	MaxAutoPlanPerMember *int `toml:"max_auto_plan_per_member,omitempty"`
	// TrackerURL is the base URL of the tracker instance for this member.
	// When set, overrides the team-state TrackerURL.
	// Allows standalone tracker usage without a team or MCP config.
	TrackerURL string `toml:"tracker_url,omitempty"`
	// TrackerTokenKey is the keychain key for the tracker token.
	// When set, overrides the team-state TrackerTokenKey.
	// Allows standalone tracker usage without MCP config.
	TrackerTokenKey string `toml:"tracker_token_key,omitempty"`
	// TrackerTokenURL is the server the token under TrackerTokenKey is valid
	// for (e.g. https://gitlab.example.com). A GitLab token is only used for
	// the server it is declared for; a key without it is never sent to a
	// server. It is the server of the key set here: it is not inherited by a
	// key of another level.
	TrackerTokenURL string `toml:"tracker_token_url,omitempty"`
	// WriteEnabled controls whether the tracker can perform write operations
	// (push labels, assign issues). nil = inherit team-state.
	// Independent of MCP write_enabled — the tracker has its own permission.
	WriteEnabled *bool `toml:"write_enabled,omitempty"`
}

// TokenDecl returns the key of the tracker token and the server it is
// declared for ("" when none): the pair that is read and written together.
func (t TrackerLocalConfig) TokenDecl() (key, server string) {
	return t.TrackerTokenKey, t.TrackerTokenURL
}

// SetTokenDecl sets the key of the tracker token and the server it is valid
// for. An empty key clears both: a server without a key means nothing.
func (t *TrackerLocalConfig) SetTokenDecl(key, server string) {
	if key == "" {
		server = ""
	}
	t.TrackerTokenKey, t.TrackerTokenURL = key, server
}

// WebsearchConfig holds web search/fetch permission settings.
// When Enabled is true, agents receive websearch and webfetch permissions via Exa AI.
type WebsearchConfig struct {
	Enabled bool `toml:"enabled"`
}

var (
	cfg       *Config
	cfgLoaded bool
	cfgErr    error
	cfgMu     sync.Mutex
)

// fileStamp is the state of hub.toml at the moment a Config was read from it
// or saved to it. The zero value means the Config does not come from the file
// (built by hand): nothing to compare it with.
type fileStamp struct {
	known  bool // the Config has a provenance
	exists bool // hub.toml existed then
	mtime  time.Time
	size   int64
}

func statStamp() fileStamp {
	st := fileStamp{known: true}
	if info, err := os.Stat(ConfigPath()); err == nil {
		st.exists, st.mtime, st.size = true, info.ModTime(), info.Size()
	}
	return st
}

// changedSince reports whether hub.toml is no longer in the state st describes.
// A file removed since is not a modification: the next save recreates it.
func (st fileStamp) changedSince() (bool, error) {
	if !st.known {
		return false, nil
	}
	info, err := os.Stat(ConfigPath())
	switch {
	case err == nil:
		if !st.exists {
			return true, nil
		}
		return !info.ModTime().Equal(st.mtime) || info.Size() != st.size, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("stat hub.toml: %w", err)
	}
}

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

// ServiceTokenKeys lists the keychain keys that can hold the token of an MCP
// service at hub level: the key configured in hub.toml, the current
// convention (openhub.mcp.<service>.token) and the legacy name
// (<service>-token), without duplicates.
func ServiceTokenKeys(c *Config, service string) []string {
	var keys []string
	add := func(k string) {
		if k == "" {
			return
		}
		for _, existing := range keys {
			if existing == k {
				return
			}
		}
		keys = append(keys, k)
	}
	if c != nil {
		if s := c.MCPServer(service); s != nil {
			add(s.Token)
		}
	}
	add(DefaultTokenKeyForService(service))
	add(service + "-token")
	return keys
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
// Solo teams (local workflow spaces) are never the active team.
//
// This method bridges the transition from single-team to multi-team. New code
// should use FindTeam(id) with a project's TeamID instead.
func (c *Config) ActiveTeam() TeamConfig {
	if t := c.DefaultTeam(); t != nil {
		return *t
	}
	if len(c.Teams) > 0 {
		return TeamConfig{}
	}
	// Legacy fallback
	if c.Team.StateRepo != "" {
		return c.Team
	}
	return TeamConfig{}
}

// readDisk reads hub.toml and returns a new Config stamped with the state of
// the file it was read from. The stamp is taken before the read: a file
// replaced in between makes the Config look older than it is, which Save then
// refuses (never the opposite). Must be called under cfgMu.
func readDisk() (*Config, error) {
	path := ConfigPath()
	stamp := statStamp()
	data, err := os.ReadFile(path)
	var c *Config
	switch {
	case err == nil:
		var notes []Notice
		c, notes, err = parseHub(path, data)
		for _, n := range notes {
			notify(n)
		}
	case os.IsNotExist(err):
		// No hub.toml is fine: the defaults apply.
		c, err = defaultConfig(), nil
	default:
		return nil, fmt.Errorf("reading hub.toml: %w", err)
	}
	if err != nil {
		return nil, err
	}
	// Post-load cleanup: if Teams is populated (either from [[teams]] in
	// the file or from RunMigrationIfNeeded), clear the legacy Team field
	// to ensure omitempty suppresses it on next Save: the file may still
	// hold a residual [team] section.
	if len(c.Teams) > 0 {
		c.Team = TeamConfig{}
	}
	c.src = stamp
	return c, nil
}

// Load reads the hub.toml configuration. It is safe to call from any goroutine.
// The first call reads from disk; subsequent calls return the cached result
// until Reset or Save invalidates it. The cached Config is shared: code that
// changes hub.toml uses Update, not a Load, a change and a Save.
func Load() (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	if cfgLoaded {
		return cfg, cfgErr
	}
	cfg, cfgErr = readDisk()
	cfgLoaded = true
	return cfg, cfgErr
}

// Read reads hub.toml again and returns it as a new Config, leaving the cache
// alone: the Config others share is not replaced, nobody sees a different
// pointer. Use it to look at the file as it is now (refresh a view, check
// what another process wrote); to change the file, use Update.
func Read() (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return readDisk()
}

// Reset drops the cached config so the next Load re-reads from disk. It does
// not weaken the protection of Save, which belongs to each Config (see
// ErrExternalModification). Tests use it to start from the file; code that
// only wants to read the file again calls Read. Safe to call concurrently
// with Load: both are serialized by cfgMu.
func Reset() {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgLoaded = false
	cfg = nil
	cfgErr = nil
}

// Save writes c to hub.toml with a full TOML marshal (comments not preserved).
// It is for complete writes that are legitimate on their own: the first
// hub.toml, a migration of the whole file. To change some settings, use
// Update, which only changes what its function changes, on the file as it is.
//
// The write is atomic and durable (unique tmp file, fsync, rename, fsync of
// the directory) to prevent corruption on crash or power loss. After saving,
// the in-memory cache is invalidated so the next Load re-reads from disk.
//
// If hub.toml changed since c was read from it, ErrExternalModification is
// returned and nothing is written. A Config built by hand is not compared.
func Save(c *Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if err := saveLocked(c); err != nil {
		return err
	}
	// Invalidate the cache so the next Load() reflects the new state.
	cfgLoaded = false
	cfg = nil
	cfgErr = nil
	return nil
}

// saveLocked validates and writes c, then stamps it with the file it wrote.
// Must be called under cfgMu.
func saveLocked(c *Config) error {
	// Validate teams before persisting
	if len(c.Teams) > 0 {
		if err := ValidateTeams(c.Teams); err != nil {
			return fmt.Errorf("invalid team config: %w", err)
		}
	}

	changed, err := c.src.changedSince()
	if err != nil {
		return err
	}
	if changed {
		return ErrExternalModification
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

	// Atomic write: a unique temp file in the same directory, synced, then
	// renamed. This prevents a half-written hub.toml on crash or power loss.
	backupConfig(path, data)
	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}
	c.src = statStamp()
	return nil
}

// updateAttempts bounds the re-reads of Update when another process writes
// hub.toml between its read and its write.
const updateAttempts = 5

// Update changes hub.toml: under the lock it reads the file as it is on disk
// (never a copy kept by the caller or the cache), applies fn to that Config and
// writes it atomically. fn changes only the settings of its caller, so what
// the others wrote in the meantime is kept. If another process writes the
// file between the read and the write, Update reads again and calls fn again:
// fn must only depend on the Config it receives.
//
// The Config shared through Load, when it is loaded, receives the result in
// place: whoever holds it sees the new values.
func Update(fn func(*Config) error) error {
	_, err := UpdateAndGet(fn)
	return err
}

// UpdateAndGet is Update, and returns the Config that was written (a new
// value, which the caller may copy into the Config it shares).
func UpdateAndGet(fn func(*Config) error) (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	var lastErr error
	for range updateAttempts {
		c, err := readDisk()
		if err != nil {
			return nil, fmt.Errorf("loading config: %w", err)
		}
		if err := fn(c); err != nil {
			return nil, err
		}
		if err := saveLocked(c); err != nil {
			if errors.Is(err, ErrExternalModification) {
				lastErr = err
				continue
			}
			return nil, err
		}
		if cfgLoaded && cfg != nil && cfgErr == nil {
			*cfg = *c
		} else {
			cfgLoaded, cfg, cfgErr = false, nil, nil
		}
		return c, nil
	}
	return nil, lastErr
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
func flattenMap(prefix string, src, dst map[string]interface{}) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMap(key, val, dst)
		case []interface{}:
			// Keep arrays as-is (e.g. [[teams]], deploy.instruction_files)
			dst[key] = val
		default:
			dst[key] = val
		}
	}
}
