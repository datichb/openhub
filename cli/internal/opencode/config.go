package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ProjectConfig holds relevant fields read from the project's opencode.json.
type ProjectConfig struct {
	Model      string
	Plugins    []string
	Compaction *CompactionConfig
}

// CompactionConfig mirrors the compaction section of opencode.json.
type CompactionConfig struct {
	Auto     bool `json:"auto"`
	Prune    bool `json:"prune"`
	Reserved int  `json:"reserved"`
}

// ReadProjectConfig reads the opencode.json at projectPath and extracts
// model, plugin, and compaction fields. Returns zero-value fields on error.
func ReadProjectConfig(projectPath string) ProjectConfig {
	var cfg ProjectConfig

	data, err := os.ReadFile(filepath.Join(projectPath, "opencode.json"))
	if err != nil {
		return cfg
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg
	}

	// Model
	if modelRaw, ok := raw["model"]; ok {
		var model string
		if json.Unmarshal(modelRaw, &model) == nil {
			cfg.Model = model
		}
	}

	// Plugins
	if pluginRaw, ok := raw["plugin"]; ok {
		var plugins []string
		if json.Unmarshal(pluginRaw, &plugins) == nil {
			cfg.Plugins = plugins
		}
	}

	// Compaction
	if compRaw, ok := raw["compaction"]; ok {
		var comp CompactionConfig
		if json.Unmarshal(compRaw, &comp) == nil {
			cfg.Compaction = &comp
		}
	}

	return cfg
}
