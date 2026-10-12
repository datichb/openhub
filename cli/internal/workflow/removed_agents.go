package workflow

import "github.com/datichb/openhub/cli/internal/i18n"

// removedAgents are the agent ids the hub no longer ships (consolidation C1),
// with the i18n key of what replaces them: a workflow of the user or team
// layer that still names one gets the replacement instead of a bare
// "unknown agent".
var removedAgents = map[string]string{
	"developer-refactor": "workflow.removed_agent.developer-refactor",
	"developer-migrator": "workflow.removed_agent.developer-migrator",
	"test-generator":     "workflow.removed_agent.test-generator",
	"infra":              "workflow.removed_agent.infra",
	"database":           "workflow.removed_agent.database",
}

// unknownAgentDiag reports an agent id missing from the catalogue (code), or
// a removed agent with its replacement.
func unknownAgentDiag(code, path, id string) Diagnostic {
	if key, ok := removedAgents[id]; ok {
		return errDiag("agent_removed", path, id, i18n.T(key))
	}
	return errDiag(code, path, id)
}
