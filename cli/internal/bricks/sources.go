package bricks

import (
	"os"
	"path/filepath"
)

// MCPServerDef is an oh MCP server of a project, resolved from the MCP
// cascade (team-state, hub, project) before it becomes a bundle server.
type MCPServerDef struct {
	Name         string
	Enabled      bool
	TokenKey     string            // keychain key name
	TokenEnv     string            // fallback environment variable name
	WriteEnabled bool              // for servers that support opt-in write mode
	URL          string            // resolved base URL (empty = use built-in default)
	Environment  map[string]string // additional environment variables to inject
}

// checkMCPToken verifies that the token for an MCP server is accessible:
// the environment variable, or a configured keychain key (the secret itself
// is read when the server starts). Servers with neither (team, workflow)
// always pass.
func checkMCPToken(s MCPServerDef) bool {
	if s.TokenEnv == "" && s.TokenKey == "" {
		return true
	}
	if s.TokenEnv != "" && os.Getenv(s.TokenEnv) != "" {
		return true
	}
	return s.TokenKey != ""
}

// providerOpencodeName maps hub provider names to opencode provider identifiers.
func providerOpencodeName(provider string) string {
	switch provider {
	case "bedrock":
		return "amazon-bedrock"
	default:
		return provider
	}
}

// discoverInstructionFiles returns the project documentation files embedded
// as instructions in every agent body (built-in defaults plus the extra
// files of `[deploy] instruction_files`), as relative paths.
func discoverInstructionFiles(projectPath string, extra []string) []interface{} {
	candidates := []string{
		"ONBOARDING.md",
		"CONVENTIONS.md",
		".claude/CLAUDE.md",
	}
	candidates = append(candidates, extra...)

	var found []interface{}
	for _, name := range candidates {
		path := filepath.Join(projectPath, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, name)
		}
	}
	return found
}
