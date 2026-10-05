package opencodev2

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

//go:embed plugin/index.ts
var pluginIndex []byte

//go:embed plugin/package.json
var pluginPackage []byte

//go:embed plugin/posture.md
var posture string

// PluginDirName is the plugin directory written in each server group data dir.
const PluginDirName = "oh-plugin"

// WritePlugin writes the embedded oh plugin into dir (a directory with
// package.json is required by opencode for local plugins).
func WritePlugin(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pluginPackage, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.ts"), pluginIndex, 0o644)
}

// installPlugin writes the plugin and the agent bodies it injects into dir.
func installPlugin(dir string, b sessionspec.BundleSpec) error {
	if err := WritePlugin(dir); err != nil {
		return err
	}
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return err
	}
	for _, a := range b.Agents {
		if err := os.WriteFile(filepath.Join(agentsDir, a.ID+".md"), []byte(a.Body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// withOhPlugin returns a copy of b that ships the oh plugin installed in dir.
func withOhPlugin(b sessionspec.BundleSpec, dir, traceFile string) sessionspec.BundleSpec {
	// agent bodies are written next to the plugin by installPlugin
	out := b
	out.Plugins = append([]sessionspec.PluginDef(nil), b.Plugins...)
	systemAgents := make([]string, 0, len(SystemAgents))
	for id := range SystemAgents {
		systemAgents = append(systemAgents, id)
	}
	opts := map[string]any{
		"agentsDir":    filepath.Join(dir, "agents"),
		"agents":       b.AgentIDs(),
		"skills":       b.SkillIDs(),
		"systemAgents": systemAgents,
	}
	if traceFile != "" {
		opts["traceFile"] = traceFile
	}
	out.Plugins = append(out.Plugins, sessionspec.PluginDef{ID: OhPluginID, Dir: dir, Options: opts})
	return out
}

// withoutOhPlugin returns a copy of b for the fallback mode: no oh plugin,
// agent bodies are rendered as `system` and prefixed with the tooling posture
// that compensates for the replaced opencode base prompt.
func withoutOhPlugin(b sessionspec.BundleSpec) sessionspec.BundleSpec {
	out := b
	out.Plugins = nil
	for _, p := range b.Plugins {
		if p.ID != OhPluginID {
			out.Plugins = append(out.Plugins, p)
		}
	}
	out.Agents = append([]sessionspec.AgentDef(nil), b.Agents...)
	for i := range out.Agents {
		out.Agents[i].Body = strings.TrimSpace(posture) + "\n\n---\n\n" + out.Agents[i].Body
	}
	return out
}

// Plugin is a plugin loaded by the server.
type Plugin struct {
	ID     string `json:"id"`
	Source struct {
		Type string `json:"type"`
		Path string `json:"path,omitempty"`
	} `json:"source"`
	State struct {
		Status string `json:"status"`
	} `json:"state"`
}

// Plugins lists the plugins active at a location.
func (c *Client) Plugins(ctx context.Context, dir string) ([]Plugin, error) {
	var out struct {
		Data []Plugin `json:"data"`
	}
	err := c.do(ctx, "GET", "/api/plugin", locQuery(dir), nil, &out)
	return out.Data, err
}

// waitPluginActive waits until plugin id is reported active (plugins load
// asynchronously after the server starts).
func waitPluginActive(ctx context.Context, c *Client, dir, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := "not listed"
	for {
		plugins, err := c.Plugins(ctx, dir)
		if err == nil {
			for _, p := range plugins {
				if p.ID == id {
					if p.State.Status == "active" {
						return nil
					}
					last = p.State.Status
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("plugin %q not active: %s", id, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}
