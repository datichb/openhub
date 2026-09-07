package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// TeamConfig represents the team-state configuration (config.toml in the repo).
type TeamConfig struct {
	Notification NotificationConfig        `toml:"notification"`
	Takeover     TakeoverConfig            `toml:"takeover"`
	Parallel     ParallelConfig            `toml:"parallel"`
	Claim        ClaimConfig               `toml:"claim"`
	Tracker      TrackerConfig             `toml:"tracker"`
	// MCP holds team-level recommendations/enforcements for MCP services.
	// Each key is a service name ("gitlab", "jira", "figma", "gslides").
	MCP map[string]SharedMCPConfig `toml:"mcp"`
	// Models holds team-level model recommendations (ADR-030).
	// These are always recommendations (overridable) — never enforced.
	// Resolution: Project > Hub > Team(recommended) > Agent Frontmatter.
	Models TeamModelsConfig `toml:"models"`
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
	MaxSessions    int  `toml:"max_sessions"`     // Max concurrent sessions (default: 3)
	PortRangeStart int  `toml:"port_range_start"` // Starting port for opencode serve (default: 4100)
	AutoMergeBeads bool `toml:"auto_merge_beads"` // Propose auto merge for Beads tickets (default: true)
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
	// PushLabels enables syncing claim labels back to the tracker (requires
	// write_enabled on the MCP gitlab/jira server).
	PushLabels bool `toml:"push_labels"`
	// PushLabelsEnforced marks PushLabels as enforced by the team.
	PushLabelsEnforced *bool `toml:"push_labels_enforced,omitempty"`
	// TicketPatterns maps hub project IDs to a regex with one capture group
	// that extracts the external tracker IID from a ticket ID string.
	// Example: {"T-SRU": "SRU-(\\d+)", "T-FRONT": "FRONT-(\\d+)"}
	TicketPatterns map[string]string `toml:"ticket_patterns"`
	// Projects maps hub project IDs to external tracker project identifiers.
	// For GitLab this is the numeric project ID or URL-encoded path.
	// For Jira this is the project key (e.g. "SRU").
	// Example: {"T-SRU": "42", "T-FRONT": "my-group/frontend"}
	Projects map[string]string `toml:"projects"`
}

// LoadConfig reads config.toml from the team-state repo.
func (r *Repo) LoadConfig() (*TeamConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
		cfg.Parallel.MaxSessions = 3
	}
	if cfg.Parallel.PortRangeStart <= 0 {
		cfg.Parallel.PortRangeStart = 4100
	}
	if cfg.Claim.DoneRetentionDays <= 0 {
		cfg.Claim.DoneRetentionDays = 7
	}
	if cfg.Tracker.MaxAutoPlanPerMember <= 0 {
		cfg.Tracker.MaxAutoPlanPerMember = 5
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
