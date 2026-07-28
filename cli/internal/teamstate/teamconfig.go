package teamstate

import (
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
	// MCP holds team-level recommendations for MCP services.
	// Each key is a service name ("gitlab", "jira", "figma", "gslides").
	// These are optional defaults — individual hub.toml settings always override.
	MCP map[string]SharedMCPConfig `toml:"mcp"`
}

// SharedMCPConfig holds team-level recommendations for a single MCP service.
// These are NOT credentials — they express what the team uses collectively.
// Individual members can override any field in their own hub.toml.
type SharedMCPConfig struct {
	// Enabled is the team recommendation: does the team use this service?
	// nil = no recommendation (each member decides independently).
	Enabled *bool `toml:"enabled,omitempty"`
	// URL is the service base URL when the team uses a self-hosted instance.
	// Example: "https://gitlab.example.com" or a self-hosted Jira URL.
	// Empty = use the per-member default (public SaaS instance).
	URL string `toml:"url,omitempty"`
	// WriteRecommended signals that the team recommends enabling write operations
	// for this service (e.g. MR creation on GitLab, label push).
	// The actual write permission is always controlled by the member's
	// hub.toml [mcp.<service>].write_enabled — this is informational only.
	WriteRecommended bool `toml:"write_recommended,omitempty"`
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
// The connection credentials (token, base URL) are NOT stored here — they are
// reused from the hub's MCP configuration ([mcp.gitlab] / [mcp.jira] in hub.toml)
// so there is no duplication of secrets.
type TrackerConfig struct {
	// Enabled turns the tracker sync on or off globally.
	Enabled bool `toml:"enabled"`
	// Type selects the tracker backend: "gitlab" or "jira".
	Type string `toml:"type"`
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
func (r *Repo) SaveConfig(cfg *TeamConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config.toml: %w", err)
	}
	path := filepath.Join(r.path, "config.toml")
	return os.WriteFile(path, data, 0o644)
}
