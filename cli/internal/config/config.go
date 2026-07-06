// Package config handles reading and writing the hub.toml configuration file.
package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
)

// Config represents the hub configuration.
type Config struct {
	CLI      CLIConfig      `mapstructure:"cli"`
	Opencode OpencodeConfig `mapstructure:"opencode"`
	MCP      MCPConfig      `mapstructure:"mcp"`
	Worktree WorktreeConfig `mapstructure:"worktree"`
}

// WorktreeConfig holds git worktree management settings.
type WorktreeConfig struct {
	AutoCleanup bool   `mapstructure:"auto_cleanup"`
	BaseBranch  string `mapstructure:"base_branch"` // empty = auto-detect (main/master)
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

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Figma   MCPServerConfig `mapstructure:"figma"`
	Gitlab  MCPServerConfig `mapstructure:"gitlab"`
	Gslides MCPServerConfig `mapstructure:"gslides"`
}

// MCPServerConfig holds individual MCP server settings.
type MCPServerConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token_key"` // keychain key name, not the secret itself
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

// Load reads the hub.toml configuration. It is safe to call multiple times.
func Load() (*Config, error) {
	cfgOnce.Do(func() {
		v := viper.New()
		v.SetConfigName("hub")
		v.SetConfigType("toml")
		v.AddConfigPath(HubDir())
		v.AddConfigPath(".")

		// Defaults
		v.SetDefault("cli.language", "en")
		v.SetDefault("opencode.channel", "stable")
		v.SetDefault("opencode.auto_update", false)
		v.SetDefault("opencode.install_dir", filepath.Join(HubDir(), "bin"))
		v.SetDefault("worktree.auto_cleanup", true)
		v.SetDefault("worktree.base_branch", "")

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
