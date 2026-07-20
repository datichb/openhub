package views

import (
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
)

// hubViper creates a viper instance configured to read hub.toml.
// Used by multiple views for reading/writing hub configuration.
func hubViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("hub")
	v.SetConfigType("toml")
	v.AddConfigPath(config.HubDir())
	v.AddConfigPath(".")
	v.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
	_ = v.ReadInConfig()
	return v
}
