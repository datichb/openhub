package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/workflow"
	toml "github.com/pelletier/go-toml/v2"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board configuration
// ─────────────────────────────────────────────────────────────────────────────

// Column role constants — semantic roles that drive business logic.
const (
	// ColumnRoleInitial marks the entry-point column (new tickets land here).
	ColumnRoleInitial = "initial"
	// ColumnRoleActive marks columns where work is actively happening.
	ColumnRoleActive = "active"
	// ColumnRoleTerminal marks completion columns (cleanup/retention applies).
	ColumnRoleTerminal = "terminal"
	// ColumnRoleBlocked marks impediment/waiting columns.
	ColumnRoleBlocked = "blocked"
)

// BoardColumnConfig defines a single board column.
type BoardColumnConfig struct {
	// ID is the internal key used in claim status fields and column matching.
	// Must be unique, lowercase, no spaces (e.g. "todo", "testing", "preprod").
	ID string `toml:"id"`
	// Name is the display label shown in the board header (e.g. "TESTING").
	Name string `toml:"name"`
	// Color overrides the DS palette color for this column.
	// Accepted values: "orange", "blue", "gray", "cyan", "green", "red", "purple", "yellow".
	// Empty = auto-assigned from role-based DS palette.
	Color string `toml:"color,omitempty"`
	// Role defines the semantic role of this column.
	// Values: "initial", "active", "terminal", "blocked".
	// Empty = "active" (default for columns without explicit role).
	Role string `toml:"role,omitempty"`
}

// BoardConfig holds team-level board layout settings.
// When Columns is empty, the default 6-column layout is used.
type BoardConfig struct {
	// Columns defines the ordered list of board columns.
	// Each column has an ID (used as claim status) and a display Name.
	// If empty, the default 6 columns are used.
	Columns []BoardColumnConfig `toml:"columns,omitempty"`
}

// InitialStatus returns the ID of the first column with role "initial".
// Falls back to the first column, or "planned" if no columns are configured.
func (cfg BoardConfig) InitialStatus() string {
	for _, c := range cfg.Columns {
		if c.Role == ColumnRoleInitial {
			return c.ID
		}
	}
	if len(cfg.Columns) > 0 {
		return cfg.Columns[0].ID
	}
	return ClaimStatusPlanned
}

// DefaultWorkStatus returns the ID of the first column with role "active".
// This is the status assigned when a member starts working on a ticket.
// Falls back to the second column, or "in_progress" if unavailable.
func (cfg BoardConfig) DefaultWorkStatus() string {
	for _, c := range cfg.Columns {
		if c.Role == ColumnRoleActive {
			return c.ID
		}
	}
	if len(cfg.Columns) > 1 {
		return cfg.Columns[1].ID
	}
	return ClaimStatusInProgress
}

// TerminalStatuses returns the IDs of all columns with role "terminal".
// Used by CleanupDoneClaims to identify completed tickets.
// Falls back to ["done"] if no columns are configured.
func (cfg BoardConfig) TerminalStatuses() []string {
	var ids []string
	for _, c := range cfg.Columns {
		if c.Role == ColumnRoleTerminal {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return []string{ClaimStatusDone}
	}
	return ids
}

// ActiveStatuses returns the IDs of all columns with role "active".
// Used for badge counts and summary displays.
func (cfg BoardConfig) ActiveStatuses() []string {
	var ids []string
	for _, c := range cfg.Columns {
		if c.Role == ColumnRoleActive {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return []string{ClaimStatusInProgress, ClaimStatusReview}
	}
	return ids
}

// BlockedStatuses returns the IDs of all columns with role "blocked".
func (cfg BoardConfig) BlockedStatuses() []string {
	var ids []string
	for _, c := range cfg.Columns {
		if c.Role == ColumnRoleBlocked {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return []string{ClaimStatusBlocked}
	}
	return ids
}

// AllStatuses returns the IDs of all configured columns in order.
// Falls back to the default 6 statuses if no columns are configured.
func (cfg BoardConfig) AllStatuses() []string {
	if len(cfg.Columns) == 0 {
		return []string{ClaimStatusPlanned, ClaimStatusInProgress, ClaimStatusReview, ClaimStatusValidation, ClaimStatusBlocked, ClaimStatusDone}
	}
	ids := make([]string, len(cfg.Columns))
	for i, c := range cfg.Columns {
		ids[i] = c.ID
	}
	return ids
}

// IsValidStatus reports whether s is a valid status for this board config.
// If no columns are configured, falls back to the built-in 6 statuses.
func (cfg BoardConfig) IsValidStatus(s string) bool {
	for _, id := range cfg.AllStatuses() {
		if id == s {
			return true
		}
	}
	// Legacy compat: "planned" is always valid (mapped to initial column at runtime).
	if s == ClaimStatusPlanned {
		return true
	}
	return false
}

// ColumnByID returns the column config for the given ID, or nil if not found.
func (cfg BoardConfig) ColumnByID(id string) *BoardColumnConfig {
	for i := range cfg.Columns {
		if cfg.Columns[i].ID == id {
			return &cfg.Columns[i]
		}
	}
	return nil
}

// HasCustomColumns reports whether the board has user-configured columns
// (as opposed to the default 6-column layout).
func (cfg BoardConfig) HasCustomColumns() bool {
	return len(cfg.Columns) > 0
}

// Validate checks the board config for structural errors.
// Returns nil if valid or if no custom columns are configured.
func (cfg BoardConfig) Validate() error {
	if len(cfg.Columns) == 0 {
		return nil // defaults will be used
	}
	if len(cfg.Columns) < 2 {
		return fmt.Errorf("board: at least 2 columns required, got %d", len(cfg.Columns))
	}
	seen := make(map[string]bool, len(cfg.Columns))
	var initialCount, terminalCount int
	for _, c := range cfg.Columns {
		if c.ID == "" {
			return fmt.Errorf("board: column ID must not be empty")
		}
		if c.Name == "" {
			return fmt.Errorf("board: column %q must have a display name", c.ID)
		}
		lower := strings.ToLower(c.ID)
		if lower != c.ID || strings.ContainsAny(c.ID, " \t") {
			return fmt.Errorf("board: column ID %q must be lowercase without spaces", c.ID)
		}
		if strings.Contains(c.ID, "..") || strings.ContainsAny(c.ID, "/\\") {
			return fmt.Errorf("board: column ID %q contains invalid characters", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("board: duplicate column ID %q", c.ID)
		}
		seen[c.ID] = true
		switch c.Role {
		case ColumnRoleInitial:
			initialCount++
		case ColumnRoleTerminal:
			terminalCount++
		case ColumnRoleActive, ColumnRoleBlocked, "":
			// ok
		default:
			return fmt.Errorf("board: column %q has unknown role %q (valid: initial, active, terminal, blocked)", c.ID, c.Role)
		}
	}
	if initialCount > 1 {
		return fmt.Errorf("board: at most 1 column can have role %q, got %d", ColumnRoleInitial, initialCount)
	}
	if terminalCount == 0 {
		return fmt.Errorf("board: at least 1 column must have role %q", ColumnRoleTerminal)
	}
	return nil
}

// DefaultBoardConfig returns the built-in 6-column board layout.
// Used as fallback when no custom columns are configured and when the
// discovery wizard needs a starting point.
func DefaultBoardConfig() BoardConfig {
	return BoardConfig{
		Columns: []BoardColumnConfig{
			{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
			{ID: "in_progress", Name: "IN PROGRESS", Role: ColumnRoleActive},
			{ID: "review", Name: "REVIEW", Role: ColumnRoleActive},
			{ID: "validation", Name: "VALIDATION", Role: ColumnRoleActive},
			{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			{ID: "blocked", Name: "BLOCKED", Role: ColumnRoleBlocked},
		},
	}
}

// TeamConfig represents the team-state configuration (config.toml in the repo).
type TeamConfig struct {
	Notification NotificationConfig `toml:"notification"`
	Takeover     TakeoverConfig     `toml:"takeover"`
	Parallel     ParallelConfig     `toml:"parallel"`
	Claim        ClaimConfig        `toml:"claim"`
	Tracker      TrackerConfig      `toml:"tracker"`
	Board        BoardConfig        `toml:"board"`
	// MCP holds team-level recommendations/enforcements for MCP services.
	// Each key is a service name ("gitlab", "jira", "figma", "gslides").
	MCP map[string]SharedMCPConfig `toml:"mcp"`
	// Models holds team-level model recommendations (ADR-030).
	// These are always recommendations (overridable) — never enforced.
	// Resolution: Project > Hub > Team(recommended) > Agent Frontmatter.
	Models TeamModelsConfig `toml:"models"`
	// Workflow held team-level overrides of the former workflow. Read only
	// by the v38 migration (→ team workflow `feature`, then removed).
	Workflow *WorkflowTeamConfig `toml:"workflow,omitempty"`
	// Governance holds who may publish the team workflows (v5 phase 2).
	Governance GovernanceConfig `toml:"governance,omitempty"`
	// Limits holds the team session restrictions (I6): recommended values
	// and enforced ceilings ([limits.recommended], [limits.enforced]).
	Limits *limits.TeamLimits `toml:"limits,omitempty"`
}

// WorkflowTeamConfig holds team-level workflow customization.
type WorkflowTeamConfig struct {
	// Overrides are applied on top of the hub-level workflow (which itself
	// is applied on top of the base workflow).
	Overrides *workflow.WorkflowOverride `toml:"overrides,omitempty"`
	// Enforced locks the workflow for all projects in this team.
	// When true, project-level WorkflowOverrides are ignored.
	Enforced *bool `toml:"enforced,omitempty"`
}

// IsEnforced reports whether the team workflow is enforced.
func (w *WorkflowTeamConfig) IsEnforced() bool {
	return w != nil && w.Enforced != nil && *w.Enforced
}

// TeamModelsConfig holds team-level model recommendations.
// All fields are recommendations that hub and project can override.
type TeamModelsConfig struct {
	// Default is the team-recommended default model for all agents.
	Default string `toml:"default,omitempty"`
	// Families maps family names to recommended models.
	Families map[string]string `toml:"families,omitempty"`
	// Agents maps agent IDs to recommended models.
	Agents map[string]string `toml:"agents,omitempty"`
}

// SharedMCPConfig holds team-level recommendations for a single MCP service.
// These are NOT credentials — they express what the team uses collectively.
//
// Each field can optionally be enforced via a companion *Enforced bool:
//   - Enforced = nil or false → the field is a recommendation (hub/project can override)
//   - Enforced = true → the field is imposed, hub/project cannot override
type SharedMCPConfig struct {
	// Enabled is the team recommendation: does the team use this service?
	// nil = no recommendation (each member decides independently).
	Enabled *bool `toml:"enabled,omitempty"`
	// EnabledEnforced marks Enabled as a team enforcement (cannot be overridden).
	EnabledEnforced *bool `toml:"enabled_enforced,omitempty"`
	// URL is the service base URL when the team uses a self-hosted instance.
	// Example: "https://gitlab.example.com" or a self-hosted Jira URL.
	// Empty = use the per-member default (public SaaS instance).
	URL string `toml:"url,omitempty"`
	// URLEnforced marks URL as a team enforcement (cannot be overridden).
	URLEnforced *bool `toml:"url_enforced,omitempty"`
	// WriteRecommended signals that the team recommends enabling write operations
	// for this service (e.g. MR creation on GitLab, label push).
	// The actual write permission is always controlled by the member's
	// hub.toml [mcp.<service>].write_enabled — this is informational only.
	WriteRecommended bool `toml:"write_recommended,omitempty"`
}

// IsEnabledEnforced reports whether the Enabled field is enforced by the team.
func (s SharedMCPConfig) IsEnabledEnforced() bool {
	return s.EnabledEnforced != nil && *s.EnabledEnforced
}

// IsURLEnforced reports whether the URL field is enforced by the team.
func (s SharedMCPConfig) IsURLEnforced() bool {
	return s.URLEnforced != nil && *s.URLEnforced
}

// NotificationConfig holds notification dispatcher settings.
type NotificationConfig struct {
	// Legacy Mattermost (backward compatible)
	MattermostWebhook string `toml:"mattermost_webhook"` // Incoming webhook URL
	Channel           string `toml:"channel"`            // Channel name
	Enabled           bool   `toml:"enabled"`
	BotName           string `toml:"bot_name"` // Display name for the bot

	// Multi-channel support
	// Type selects the notifier: "mattermost" (default), "slack", "discord", "teams"
	Type string `toml:"type"` // "mattermost" | "slack" | "discord" | "teams"

	// Generic webhook URL — used by slack, discord, teams
	WebhookURL string `toml:"webhook_url"`

	// Optional: multiple destinations
	Destinations []NotificationDestination `toml:"destinations"`
}

// NotificationDestination represents a single notification target.
type NotificationDestination struct {
	Type       string `toml:"type"`        // "mattermost" | "slack" | "discord" | "teams"
	WebhookURL string `toml:"webhook_url"` // Webhook URL
	Channel    string `toml:"channel"`     // Channel (Mattermost only)
	BotName    string `toml:"bot_name"`    // Bot display name
}

// TakeoverConfig holds settings for the takeover brief system.
type TakeoverConfig struct {
	StaleDays int `toml:"stale_days"` // Days of inactivity before a claim is considered stale (default: 3)
}

// ParallelConfig holds settings for parallel session execution.
type ParallelConfig struct {
	MaxSessions            int  `toml:"max_sessions"`              // Max concurrent sessions (default: 5)
	MaxBudgetMinutes       int  `toml:"max_budget_minutes"`        // Max total estimated minutes (default: 180, 0 = disabled)
	DefaultTicketWeightMin int  `toml:"default_ticket_weight_min"` // Fallback weight when no estimate (default: 60)
	PortRangeStart         int  `toml:"port_range_start"`          // Starting port for opencode serve (default: 4100)
	AutoMergeBeads         bool `toml:"auto_merge_beads"`          // Propose auto merge for Beads tickets (default: true)
	MaxRetries             int  `toml:"max_retries"`               // Max recovery attempts per failed session (default: 2)
	RetryDelaySeconds      int  `toml:"retry_delay_seconds"`       // Seconds to wait before retry (default: 5)
}

// ClaimConfig holds settings for the claim lifecycle.
type ClaimConfig struct {
	// DoneRetentionDays is the number of days a claim stays in "done" status
	// before being automatically cleaned up by CleanupDoneClaims.
	// Default: 7.
	DoneRetentionDays int `toml:"done_retention_days"`
}

// TrackerConfig holds settings for the external tracker sync (GitLab / Jira).
// Connection credentials (token) are personal and never stored here — they are
// resolved at runtime from the keychain or environment variables.
// The tracker_url allows the team to point to a specific GitLab/Jira instance
// independently of the MCP service configuration.
type TrackerConfig struct {
	// Enabled turns the tracker sync on or off globally.
	Enabled bool `toml:"enabled"`
	// Type selects the tracker backend: "gitlab" or "jira".
	Type string `toml:"type"`
	// TypeEnforced marks Type as factual/enforced (cannot be overridden locally).
	TypeEnforced *bool `toml:"type_enforced,omitempty"`
	// TrackerURL is the base URL of the tracker instance (e.g. "https://gitlab.example.com").
	// When set, it overrides the MCP service URL for tracker operations.
	// This allows the tracker to point to a different instance than the MCP GitLab/Jira.
	TrackerURL string `toml:"tracker_url,omitempty"`
	// TrackerTokenKey is the keychain key used to store the tracker-specific token.
	// When empty, defaults to "openhub.tracker.<type>.token".
	// Falls back to the MCP service token if the tracker-specific token is not found.
	TrackerTokenKey string `toml:"tracker_token_key,omitempty"`
	// AutoSync triggers a sync automatically when team views are opened.
	AutoSync bool `toml:"auto_sync"`
	// SyncIntervalMinutes is the polling interval when the board is open.
	// 0 means no timer-based sync (rely on view open / manual only).
	SyncIntervalMinutes int `toml:"sync_interval_minutes"`
	// AutoPlanAssigned automatically creates "planned" claims for tracker issues
	// that are assigned to a team member but not yet claimed.
	AutoPlanAssigned bool `toml:"auto_plan_assigned"`
	// MaxAutoPlanPerMember limits the number of auto-planned claims per member
	// to avoid flooding the TODO column. Default: 5.
	MaxAutoPlanPerMember int `toml:"max_auto_plan_per_member"`
	// AutoPlanUnassigned fetches unassigned open issues from the tracker and
	// creates "pool" claims (ClaimedBy="") so they appear on the team board
	// as claimable tickets. Members can claim them with the 'c' key.
	AutoPlanUnassigned bool `toml:"auto_plan_unassigned"`
	// UnassignedLabels restricts which unassigned issues are fetched.
	// Only issues matching ALL of these labels are imported.
	// Empty = all unassigned open issues (no label filter).
	UnassignedLabels []string `toml:"unassigned_labels,omitempty"`
	// MaxUnassignedIssues caps the number of unassigned issues fetched per project.
	// Default: 20.
	MaxUnassignedIssues int `toml:"max_unassigned_issues"`
	// PushLabels enables syncing claim labels back to the tracker.
	// Requires write_enabled on the tracker config.
	// When enabled, claiming a pool ticket also assigns it on the tracker.
	PushLabels bool `toml:"push_labels"`
	// PushLabelsEnforced marks PushLabels as enforced by the team.
	PushLabelsEnforced *bool `toml:"push_labels_enforced,omitempty"`
	// WriteEnabled controls whether tracker write operations (push labels,
	// assign issues) are allowed. Independent of MCP write_enabled.
	WriteEnabled bool `toml:"write_enabled"`
	// TrackerProject is the default external tracker project identifier for the team.
	// For GitLab this is the numeric project ID or URL-encoded path (e.g. "group/project").
	// For Jira this is the project key (e.g. "SRU").
	// Individual hub projects can override this via their ProjectTrackerConfig.
	TrackerProject string `toml:"tracker_project,omitempty"`
	// TicketPattern is the default regex with one capture group for extracting
	// the external tracker IID from a ticket ID string.
	// Individual hub projects can override this via their ProjectTrackerConfig.
	// Empty means the ticket ID is used as-is (numeric IID).
	TicketPattern string `toml:"ticket_pattern,omitempty"`
	// TicketPatterns maps hub project IDs to a regex with one capture group
	// that extracts the external tracker IID from a ticket ID string.
	//
	// Deprecated: use TrackerProject + TicketPattern (simple fields) or
	// per-project ProjectTrackerConfig overrides. Kept for backward compat.
	TicketPatterns map[string]string `toml:"ticket_patterns,omitempty"`
	// Projects maps hub project IDs to external tracker project identifiers.
	//
	// Deprecated: use TrackerProject (simple field) or per-project
	// ProjectTrackerConfig overrides. Kept for backward compat.
	Projects map[string]string `toml:"projects,omitempty"`
	// StatusMapping maps tracker status names (case-insensitive keys) to claim
	// statuses (planned, in_progress, review, validation, blocked, done).
	// Example: {"In Progress" = "in_progress", "Code Review" = "review", "In QA" = "validation"}
	// When a tracker issue status matches a key, the claim is placed in the
	// corresponding board column. Unmatched statuses fall back to the category-
	// based mapping (Jira statusCategory / GitLab state).
	StatusMapping map[string]string `toml:"status_mapping,omitempty"`
	// LabelStatusMapping maps tracker labels (case-insensitive keys) to claim
	// statuses. This is especially useful for GitLab which only has "opened"/"closed"
	// states — the real workflow status is carried by labels.
	// Order matters: the FIRST matching label wins (priority by config order).
	// Valid statuses: planned, in_progress, review, validation, blocked, done
	// Example: {"Bloqué" = "blocked", "DEV DOING" = "in_progress", "TO REVIEW" = "review"}
	LabelStatusMapping map[string]string `toml:"label_status_mapping,omitempty"`
}

// IsTypeEnforced reports whether the tracker Type is enforced by the team.
func (t TrackerConfig) IsTypeEnforced() bool {
	return t.TypeEnforced != nil && *t.TypeEnforced
}

// IsPushLabelsEnforced reports whether PushLabels is enforced by the team.
func (t TrackerConfig) IsPushLabelsEnforced() bool {
	return t.PushLabelsEnforced != nil && *t.PushLabelsEnforced
}

// LoadConfig reads config.toml from the team-state repo.
func (r *Repo) LoadConfig() (*TeamConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loadConfig()
}

// loadConfig is LoadConfig without the lock.
func (r *Repo) loadConfig() (*TeamConfig, error) {
	path := filepath.Join(r.path, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config
			return &TeamConfig{
				Notification: NotificationConfig{
					Enabled: false,
					BotName: "OpenHub",
				},
			}, nil
		}
		return nil, fmt.Errorf("reading config.toml: %w", err)
	}
	var cfg TeamConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config.toml: %w", err)
	}
	if cfg.Notification.BotName == "" {
		cfg.Notification.BotName = "OpenHub"
	}
	if cfg.Takeover.StaleDays <= 0 {
		cfg.Takeover.StaleDays = 3
	}
	if cfg.Parallel.MaxSessions <= 0 {
		cfg.Parallel.MaxSessions = 5
	}
	if cfg.Parallel.MaxBudgetMinutes < 0 {
		cfg.Parallel.MaxBudgetMinutes = 0
	}
	if cfg.Parallel.DefaultTicketWeightMin <= 0 {
		cfg.Parallel.DefaultTicketWeightMin = 60
	}
	if cfg.Parallel.PortRangeStart <= 0 {
		cfg.Parallel.PortRangeStart = 4100
	}
	if cfg.Parallel.MaxRetries < 0 {
		cfg.Parallel.MaxRetries = 0
	}
	if cfg.Parallel.RetryDelaySeconds < 0 {
		cfg.Parallel.RetryDelaySeconds = 0
	}
	if cfg.Claim.DoneRetentionDays <= 0 {
		cfg.Claim.DoneRetentionDays = 7
	}
	if cfg.Tracker.MaxAutoPlanPerMember <= 0 {
		cfg.Tracker.MaxAutoPlanPerMember = 5
	}
	if cfg.Tracker.MaxUnassignedIssues <= 0 {
		cfg.Tracker.MaxUnassignedIssues = 20
	}
	if cfg.Tracker.SyncIntervalMinutes <= 0 && cfg.Tracker.AutoSync {
		cfg.Tracker.SyncIntervalMinutes = 5
	}
	return &cfg, nil
}

// SaveConfig writes config.toml to the team-state repo.
func (r *Repo) SaveConfig(ctx context.Context, cfg *TeamConfig) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		data, err := toml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshaling config.toml: %w", err)
		}
		path := filepath.Join(r.path, "config.toml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		return r.commitAndPush(ctx, "config: update team config", "config.toml")
	})
}
