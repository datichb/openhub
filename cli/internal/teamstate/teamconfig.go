package teamstate

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// TeamConfig represents the team-state configuration (config.toml in the repo).
type TeamConfig struct {
	Notification NotificationConfig `toml:"notification"`
	Takeover     TakeoverConfig     `toml:"takeover"`
	Parallel     ParallelConfig     `toml:"parallel"`
}

// NotificationConfig holds notification dispatcher settings.
type NotificationConfig struct {
	MattermostWebhook string `toml:"mattermost_webhook"` // Incoming webhook URL
	Channel           string `toml:"channel"`            // Channel name (single channel for all)
	Enabled           bool   `toml:"enabled"`
	BotName           string `toml:"bot_name"` // Display name for the bot
}

// TakeoverConfig holds settings for the takeover brief system.
type TakeoverConfig struct {
	StaleDays int `toml:"stale_days"` // Days of inactivity before a claim is considered stale (default: 3)
}

// ParallelConfig holds settings for parallel session execution.
type ParallelConfig struct {
	MaxSessions    int  `toml:"max_sessions"`       // Max concurrent sessions (default: 3)
	PortRangeStart int  `toml:"port_range_start"`   // Starting port for opencode serve (default: 4100)
	AutoMergeBeads bool `toml:"auto_merge_beads"`   // Propose auto merge for Beads tickets (default: true)
}

// LoadConfig reads config.toml from the team-state repo.
func (r *Repo) LoadConfig() (*TeamConfig, error) {
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
	return &cfg, nil
}

// SaveConfig writes config.toml to the team-state repo.
func (r *Repo) SaveConfig(cfg *TeamConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config.toml: %w", err)
	}
	path := filepath.Join(r.path, "config.toml")
	return os.WriteFile(path, data, 0o644)
}
