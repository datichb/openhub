package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/config"
)

// Plugin is the interface that all plugin implementations must satisfy.
type Plugin interface {
	// Name returns the unique plugin identifier.
	Name() string
	// Description returns a short human-readable description.
	Description() string
	// Install deploys the plugin to the opencode plugins directory for the given project path.
	Install(ctx context.Context, projectPath string) error
	// Uninstall removes the plugin from the opencode plugins directory.
	Uninstall(ctx context.Context, projectPath string) error
	// IsInstalled reports whether the plugin is currently deployed.
	IsInstalled(projectPath string) (bool, error)
	// Status returns the full PluginStatus for the plugin.
	Status() PluginStatus
}

// PluginManifest describes a community plugin stored in ~/.oh/plugins/<name>/manifest.json.
type PluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	// FileName is the plugin file to copy into the opencode plugins dir.
	FileName string `json:"file_name"`
}

// Registry holds all known plugins (built-in + community).
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates a Registry pre-loaded with all built-in plugins.
func NewRegistry() *Registry {
	r := &Registry{plugins: make(map[string]Plugin)}
	r.register(&RTKPlugin{})
	r.loadCommunityPlugins()
	return r
}

func (r *Registry) register(p Plugin) {
	r.plugins[p.Name()] = p
}

// Get returns a plugin by name, or nil if not found.
func (r *Registry) Get(name string) Plugin {
	return r.plugins[name]
}

// All returns all registered plugins, built-in first then community.
func (r *Registry) All() []Plugin {
	// Built-in plugins in deterministic order
	builtins := []string{"rtk"}
	seen := make(map[string]bool)
	var result []Plugin

	for _, name := range builtins {
		if p, ok := r.plugins[name]; ok {
			result = append(result, p)
			seen[name] = true
		}
	}
	// Community plugins
	for name, p := range r.plugins {
		if !seen[name] {
			result = append(result, p)
		}
	}
	return result
}

// CommunityPluginsDir returns ~/.oh/plugins/.
func CommunityPluginsDir() string {
	return filepath.Join(config.HubDir(), "plugins")
}

// loadCommunityPlugins discovers plugins in ~/.oh/plugins/<name>/manifest.json.
func (r *Registry) loadCommunityPlugins() {
	dir := CommunityPluginsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // directory doesn't exist yet — no community plugins
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest PluginManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		if manifest.Name == "" || manifest.FileName == "" {
			continue
		}
		pluginDir := filepath.Join(dir, entry.Name())
		r.register(&communityPlugin{
			manifest:  manifest,
			pluginDir: pluginDir,
		})
	}
}

// communityPlugin implements Plugin for user-installed community plugins.
type communityPlugin struct {
	manifest  PluginManifest
	pluginDir string
}

func (c *communityPlugin) Name() string        { return c.manifest.Name }
func (c *communityPlugin) Description() string { return c.manifest.Description }

func (c *communityPlugin) Install(_ context.Context, projectPath string) error {
	src := filepath.Join(c.pluginDir, c.manifest.FileName)
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading plugin file %s: %w", c.manifest.FileName, err)
	}

	dest := filepath.Join(opencodePluginsDir(), c.manifest.FileName)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating plugins dir: %w", err)
	}
	return os.WriteFile(dest, data, 0o644)
}

func (c *communityPlugin) Uninstall(_ context.Context, projectPath string) error {
	dest := filepath.Join(opencodePluginsDir(), c.manifest.FileName)
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plugin: %w", err)
	}
	return nil
}

func (c *communityPlugin) IsInstalled(projectPath string) (bool, error) {
	dest := filepath.Join(opencodePluginsDir(), c.manifest.FileName)
	_, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (c *communityPlugin) Status() PluginStatus {
	installed, _ := c.IsInstalled("")
	return PluginStatus{
		Installed: installed,
		Path:      filepath.Join(opencodePluginsDir(), c.manifest.FileName),
		Version:   c.manifest.Version,
	}
}
