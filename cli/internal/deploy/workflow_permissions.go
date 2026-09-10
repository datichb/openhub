package deploy

import (
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ApplyWorkflowPermissions updates agent configurations with task permissions
// derived from the resolved workflow, and applies agent mode overrides.
//
// It modifies the agentConfigs map in-place:
//   - For each active agent in the workflow, the "task" permission is replaced
//     with the workflow-derived permission (implicit graph + explicit overrides).
//   - The agent "mode" is updated if the workflow overrides it.
//   - Disabled agents are marked for exclusion.
//
// The agentConfigs keys are agent IDs matching the workflow AgentSlot.AgentID.
func ApplyWorkflowPermissions(
	wf *workflow.WorkflowDefinition,
	agentConfigs map[string]map[string]interface{},
) {
	// Derive the full task permission map from the workflow graph.
	perms := workflow.DeriveTaskPermissions(wf)

	for _, agent := range wf.Agents {
		cfg, exists := agentConfigs[agent.AgentID]
		if !exists {
			continue
		}

		// Skip disabled agents — they'll be excluded from deploy.
		if agent.Role == workflow.RoleDisabled {
			continue
		}

		// Apply task permissions.
		if allowed, ok := perms[agent.AgentID]; ok {
			taskPerm := workflow.BuildTaskPermissionMap(allowed)
			// Convert to map[string]interface{} for opencode.json compatibility.
			taskPermIface := make(map[string]interface{}, len(taskPerm))
			for k, v := range taskPerm {
				taskPermIface[k] = v
			}

			// Get or create the permission map.
			permMap, _ := cfg["permission"].(map[string]interface{})
			if permMap == nil {
				permMap = make(map[string]interface{})
				cfg["permission"] = permMap
			}
			permMap["task"] = taskPermIface
		}

		// Apply mode override.
		if agent.Mode == workflow.ModeSubagent {
			cfg["mode"] = "subagent"
		} else {
			// Primary is the default — remove explicit mode to keep config clean.
			delete(cfg, "mode")
		}
	}
}

// DisabledAgentIDs returns the list of agent IDs that are disabled in the
// workflow.  These agents should be excluded from the deploy.
func DisabledAgentIDs(wf *workflow.WorkflowDefinition) []string {
	var disabled []string
	for _, a := range wf.Agents {
		if a.Role == workflow.RoleDisabled {
			disabled = append(disabled, a.AgentID)
		}
	}
	return disabled
}
