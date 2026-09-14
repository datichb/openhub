package deploy

import "fmt"

// mcpIntegrationDescriptions maps (agentID, serverName) pairs to a
// human-readable description of what the integration provides.
// Used by CollectMissingMCPIntegrations for the deploy summary.
var mcpIntegrationDescriptions = map[string]map[string]string{
	"designer":   {"figma": "accès aux designs et composants via Figma"},
	"planner":    {"gitlab": "accès aux issues, merge requests et milestones via GitLab"},
	"pathfinder": {"gitlab": "exploration du code et des pipelines via GitLab"},
	"onboarder":  {"gitlab": "analyse de l'historique projet via GitLab"},
}

// mcpServerDescriptions provides a generic fallback description per server
// when no agent-specific description exists.
var mcpServerDescriptions = map[string]string{
	"figma":   "intégration Figma",
	"gitlab":  "intégration GitLab",
	"gslides": "intégration Google Slides",
}

// describeMCPIntegration returns a human-readable description for an
// agent/server pair. Falls back to a generic server description, then
// to a minimal "intégration <server>" string.
func describeMCPIntegration(agentID, serverName string) string {
	if agentDescs, ok := mcpIntegrationDescriptions[agentID]; ok {
		if desc, ok := agentDescs[serverName]; ok {
			return desc
		}
	}
	if desc, ok := mcpServerDescriptions[serverName]; ok {
		return desc
	}
	return fmt.Sprintf("intégration %s", serverName)
}
