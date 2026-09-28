package deploy

import "fmt"

// mcpIntegrationKeys maps (agentID, serverName) pairs to i18n keys.
// The actual translation is resolved at the call site in the cmd layer
// to keep the deploy package independent of the i18n system.
var mcpIntegrationKeys = map[string]map[string]string{
	"designer":   {"figma": "deploy.mcp.desc.designer.figma"},
	"planner":    {"gitlab": "deploy.mcp.desc.planner.gitlab"},
	"pathfinder": {"gitlab": "deploy.mcp.desc.pathfinder.gitlab"},
	"onboarder":  {"gitlab": "deploy.mcp.desc.onboarder.gitlab"},
}

// mcpServerKeys provides a generic i18n key fallback per server
// when no agent-specific key exists.
var mcpServerKeys = map[string]string{
	"figma":   "deploy.mcp.desc.figma",
	"gitlab":  "deploy.mcp.desc.gitlab",
	"jira":    "deploy.mcp.desc.jira",
	"gslides": "deploy.mcp.desc.gslides",
}

// describeMCPIntegration returns an i18n key for an agent/server pair.
// The caller is responsible for resolving it via i18n.T().
// Falls back to a generic server key, then to a formatted key for unknown servers.
func describeMCPIntegration(agentID, serverName string) string {
	if agentKeys, ok := mcpIntegrationKeys[agentID]; ok {
		if key, ok := agentKeys[serverName]; ok {
			return key
		}
	}
	if key, ok := mcpServerKeys[serverName]; ok {
		return key
	}
	return fmt.Sprintf("deploy.mcp.desc.%s", serverName)
}
