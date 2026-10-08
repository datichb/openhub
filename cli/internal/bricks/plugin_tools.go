package bricks

import (
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PluginToolsFile lists the permissions provided by plugins (hub
// permissions/plugin-tools.yaml).
const PluginToolsFile = "plugin-tools.yaml"

// PluginTools are the permissions a plugin provides: tool keys and shell
// commands (globs).
type PluginTools struct {
	Tools []string `yaml:"tools"`
	Shell []string `yaml:"shell"`
}

// LoadPluginTools reads permissions/plugin-tools.yaml (empty when absent).
func LoadPluginTools(hubDir string) (map[string]PluginTools, error) {
	data, err := os.ReadFile(filepath.Join(hubDir, "permissions", PluginToolsFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out map[string]PluginTools
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DropPluginPermissions removes from an agent permission map the tools and
// shell commands of the plugins that are not loaded.
func DropPluginPermissions(perms map[string]any, tools map[string]PluginTools, loaded map[string]bool) map[string]any {
	var toolGlobs, shellGlobs []string
	for id, t := range tools {
		if !loaded[id] {
			toolGlobs = append(toolGlobs, t.Tools...)
			shellGlobs = append(shellGlobs, t.Shell...)
		}
	}
	if len(toolGlobs) == 0 && len(shellGlobs) == 0 {
		return perms
	}
	match := func(globs []string, key string) bool {
		for _, g := range globs {
			if ok, _ := path.Match(g, key); ok || g == key {
				return true
			}
		}
		return false
	}
	out := make(map[string]any, len(perms))
	for k, v := range perms {
		if match(toolGlobs, k) {
			continue
		}
		if k == "bash" {
			if m, ok := v.(map[string]any); ok {
				nm := make(map[string]any, len(m))
				for ck, cv := range m {
					if !match(shellGlobs, ck) {
						nm[ck] = cv
					}
				}
				v = nm
			}
		}
		out[k] = v
	}
	return out
}
