package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TeamConfigFile is the filename written into .opencode/ for team config.
// The team MCP server reads this file instead of hub.toml so that per-project
// team settings are honoured without relying on environment variables.
const TeamConfigFile = "team.json"

// DeployedTeamConfig is the structure written to .opencode/team.json.
// It carries the fully-resolved (effective) team configuration for this project.
type DeployedTeamConfig struct {
	Enabled   bool   `json:"enabled"`
	StateRepo string `json:"state_repo"`
	StatePath string `json:"state_path"`
	MemberID  string `json:"member_id"`
}

// MCPServerDef describes an MCP server to potentially deploy.
type MCPServerDef struct {
	Name         string
	Enabled      bool
	TokenKey     string            // keychain key name
	TokenEnv     string            // fallback environment variable name
	WriteEnabled bool              // for servers that support opt-in write mode
	URL          string            // resolved base URL (empty = use built-in default)
	Environment  map[string]string // additional environment variables to inject
}

// DeployMCP creates a Phase that injects mcpServers into opencode.json.
// Only MCP servers that are both enabled AND have a valid token are deployed.
// Servers that fail validation are skipped with a warning (returned in results).
func DeployMCP(servers []MCPServerDef, binaryName string) Phase {
	return Phase{
		Name: "MCP Servers",
		Execute: func(ctx *Context) error {
			// Determine which MCP servers are functional
			var deployed []MCPServerDef
			for _, s := range servers {
				if !s.Enabled {
					continue
				}
				if !checkMCPToken(s) {
					// Warning stored in context but not a fatal error
					ctx.Results = append(ctx.Results, PhaseResult{
						Name:    fmt.Sprintf("MCP/%s", s.Name),
						Success: true, // not a failure, just a skip
						Message: fmt.Sprintf("skipped — token non trouvé (env: %s, keychain: %s)", s.TokenEnv, s.TokenKey),
					})
					continue
				}
				deployed = append(deployed, s)
			}

			if len(deployed) == 0 {
				return nil // nothing to deploy
			}

			// Read existing opencode.json
			configPath := filepath.Join(ctx.Plan.ProjectPath, "opencode.json")
			var config map[string]interface{}
			if data, err := os.ReadFile(configPath); err == nil {
				if err := json.Unmarshal(data, &config); err != nil {
					config = make(map[string]interface{})
				}
			} else {
				config = make(map[string]interface{})
			}

			// Merge mcp section (preserve existing entries)
			mcpServers, ok := config["mcp"].(map[string]interface{})
			if !ok {
				mcpServers = make(map[string]interface{})
			}

			for _, s := range deployed {
				command := []string{binaryName, "mcp", "serve", s.Name}
				// Inject --token-key so the serve command can resolve the token from keychain
				if s.TokenKey != "" {
					command = append(command, "--token-key", s.TokenKey)
				}
				entry := map[string]interface{}{
					"type":    "local",
					"command": command,
					"enabled": true,
				}
			// Inject environment variables
			env := make(map[string]string)
			if s.WriteEnabled {
				env["GITLAB_WRITE_ENABLED"] = "true"
			}
			for k, v := range s.Environment {
				env[k] = v
			}
			if len(env) > 0 {
				entry["environment"] = env
			}
				mcpServers[s.Name] = entry
			}
			config["mcp"] = mcpServers

			// Write atomically
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

// checkMCPToken verifies that the token for an MCP server is accessible.
// It checks the environment variable first, then falls back to checking
// if the keychain key name is non-empty (actual keychain lookup happens at runtime).
// Servers with neither TokenEnv nor TokenKey are considered "tokenless" (e.g., team)
// and always pass the check.
func checkMCPToken(s MCPServerDef) bool {
	// Tokenless servers (e.g., team) are always OK when enabled
	if s.TokenEnv == "" && s.TokenKey == "" {
		return true
	}
	// Check environment variable
	if s.TokenEnv != "" && os.Getenv(s.TokenEnv) != "" {
		return true
	}
	// Check if keychain key is configured (actual secret checked at serve time)
	if s.TokenKey != "" {
		return true
	}
	return false
}

// DefaultMCPServers returns the standard MCP server definitions used by oh.
func DefaultMCPServers(enabled map[string]bool, tokenKeys map[string]string, writeEnabled map[string]bool) []MCPServerDef {
	return []MCPServerDef{
		{
			Name:         "figma",
			Enabled:      enabled["figma"],
			TokenKey:     tokenKeys["figma"],
			TokenEnv:     "FIGMA_TOKEN",
			WriteEnabled: false,
		},
		{
			Name:         "gitlab",
			Enabled:      enabled["gitlab"],
			TokenKey:     tokenKeys["gitlab"],
			TokenEnv:     "GITLAB_TOKEN",
			WriteEnabled: writeEnabled["gitlab"],
		},
		{
			Name:         "gslides",
			Enabled:      enabled["gslides"],
			TokenKey:     tokenKeys["gslides"],
			TokenEnv:     "GOOGLE_ACCESS_TOKEN",
			WriteEnabled: false,
		},
		{
			Name:    "team",
			Enabled: enabled["team"],
			// No token needed — reads .opencode/team.json written by DeployTeamConfig
		},
	}
}

// DeployTeamConfig creates a Phase that writes (or removes) .opencode/team.json.
//
// When teamCfg.Enabled is true the file is written with the resolved config.
// When teamCfg.Enabled is false the file is removed if it exists, ensuring that
// a project with Mode=="disabled" never has a stale team.json from a previous deploy.
func DeployTeamConfig(teamCfg DeployedTeamConfig) Phase {
	return Phase{
		Name: "Team Config",
		Execute: func(ctx *Context) error {
			teamConfigPath := filepath.Join(ctx.Plan.ProjectPath, ".opencode", TeamConfigFile)

			if !teamCfg.Enabled {
				// Remove any existing file so the MCP server cannot be accidentally activated
				if err := os.Remove(teamConfigPath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("removing team config: %w", err)
				}
				return nil
			}

			// Ensure .opencode/ exists
			if err := os.MkdirAll(filepath.Dir(teamConfigPath), 0o755); err != nil {
				return fmt.Errorf("creating .opencode dir: %w", err)
			}

			data, err := json.MarshalIndent(teamCfg, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling team config: %w", err)
			}

			tmpFile := teamConfigPath + ".tmp"
			if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
				return fmt.Errorf("writing team config: %w", err)
			}
			return os.Rename(tmpFile, teamConfigPath)
		},
	}
}
