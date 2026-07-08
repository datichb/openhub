package deploy

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// DeployAgentConfig parses frontmatter of selected agents and writes per-agent
// configuration blocks into opencode.json: mode, permission, model (resolved via cascade).
//
// This phase runs AFTER DeployConfig (which writes global config + disables native agents)
// and adds per-hub-agent blocks to the "agent" section.
//
// Parameters:
//   - hubDir: source hub directory containing agents/
//   - selected: agent names to configure (empty = all found agents)
//   - projectOverrides: model overrides from the project DB (nil = no project overrides)
//   - hubOverrides: model overrides from hub.toml (nil = no hub overrides)
//   - provider: resolved provider for this project (for model normalization)
func DeployAgentConfig(hubDir string, selected []string, projectOverrides, hubOverrides *ModelOverrides, provider string) Phase {
	return Phase{
		Name: "Agent Configuration",
		Execute: func(ctx *Context) error {
			configPath := filepath.Join(ctx.Plan.ProjectPath, "opencode.json")

			// 1. Read existing opencode.json (should exist — DeployConfig ran before us)
			var config map[string]interface{}
			if data, err := os.ReadFile(configPath); err == nil {
				if err := json.Unmarshal(data, &config); err != nil {
					config = make(map[string]interface{})
				}
			} else {
				config = make(map[string]interface{})
			}

			// 2. Get or create the "agent" section
			agentCfg, ok := config["agent"].(map[string]interface{})
			if !ok {
				agentCfg = make(map[string]interface{})
			}

			// 3. Discover and parse selected agents
			srcDir := filepath.Join(hubDir, "agents")
			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				// No agents directory — nothing to configure
				return nil
			}

			// Build allow set for filtering
			allowSet := make(map[string]bool, len(selected))
			for _, a := range selected {
				allowSet[a] = true
			}

			// Walk agents directory and build config for each selected agent
			err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if filepath.Ext(path) != ".md" {
					return nil
				}

				// Filter by selected agents
				name := strings.TrimSuffix(d.Name(), ".md")
				if len(allowSet) > 0 && !allowSet[name] {
					return nil
				}

				// Parse frontmatter
				fm, err := ParseAgentFrontmatter(path)
				if err != nil {
					// Skip agents with malformed frontmatter — warn but don't fail deploy
					slog.Warn("skipping agent with malformed frontmatter",
						"agent", name, "error", err)
					return nil //nolint:nilerr // intentional: skip malformed agents
				}

				// Guard: agent must have an id
				if fm.ID == "" {
					slog.Warn("skipping agent with no id in frontmatter", "path", path)
					return nil
				}

				// Validate MCP server requirements
				if len(fm.MCPServers) > 0 && len(ctx.Plan.EnabledMCPServers) > 0 {
					validateAgentMCPServers(fm.ID, fm.MCPServers, ctx.Plan.EnabledMCPServers)
				}

				// Derive family from relative path
				rel, _ := filepath.Rel(srcDir, path)
				family := AgentFamily(rel)

				// Build agent config block
				agentBlock := buildAgentBlock(fm, family, projectOverrides, hubOverrides, provider)
				if agentBlock != nil {
					agentCfg[fm.ID] = agentBlock
				}

				return nil
			})
			if err != nil {
				return fmt.Errorf("walking agents directory: %w", err)
			}

			// 4. Write back
			config["agent"] = agentCfg

			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			tmpFile := configPath + ".tmp"
			if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
				return err
			}
			return os.Rename(tmpFile, configPath)
		},
	}
}

// buildAgentBlock constructs the per-agent configuration block for opencode.json.
// Returns nil if there's nothing meaningful to write (no mode, no permissions, no model).
func buildAgentBlock(fm *AgentFrontmatter, family string, projectOverrides, hubOverrides *ModelOverrides, provider string) map[string]interface{} {
	block := make(map[string]interface{})

	// Description: required by opencode for agent display and delegation
	if fm.Description != "" {
		block["description"] = fm.Description
	}

	// Mode: only write if subagent (primary is the default, no need to declare)
	if fm.Mode == "subagent" {
		block["mode"] = "subagent"
	}

	// Model: resolve via cascade
	model := ResolveAgentModel(fm.ID, family, projectOverrides, hubOverrides, fm.Model, provider)
	if model != "" {
		block["model"] = model
	}

	// Permission: serialize the structured permission map
	if len(fm.Permission) > 0 {
		block["permission"] = convertPermissionForJSON(fm.Permission)
	}

	// Only return block if it has content
	if len(block) == 0 {
		return nil
	}

	return block
}

// convertPermissionForJSON converts the parsed YAML permission structure to a format
// suitable for opencode.json. The structure is already normalized from the frontmatter parser,
// but we ensure proper JSON-compatible types.
func convertPermissionForJSON(perm map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(perm))
	for k, v := range perm {
		switch val := v.(type) {
		case map[string]interface{}:
			// Nested permission (bash, task): convert recursively
			result[k] = convertPermissionForJSON(val)
		case string:
			result[k] = val
		case bool:
			if val {
				result[k] = "allow"
			} else {
				result[k] = "deny"
			}
		default:
			result[k] = fmt.Sprintf("%v", val)
		}
	}
	return result
}

// validateAgentMCPServers checks if all MCP servers required by an agent are enabled.
// Emits slog warnings for each missing server — does not fail the deploy.
func validateAgentMCPServers(agentID string, required, enabled []string) {
	enabledSet := make(map[string]bool, len(enabled))
	for _, s := range enabled {
		enabledSet[s] = true
	}

	for _, req := range required {
		if !enabledSet[req] {
			slog.Warn("agent requires MCP server that is not enabled",
				"agent", agentID,
				"server", req,
				"hint", fmt.Sprintf("enable with: oh config set mcp.%s.enabled true", req),
			)
		}
	}
}
