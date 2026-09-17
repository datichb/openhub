package workflow

// DeriveTaskPermissions computes the full task permission map for every
// active agent in the workflow.
//
// The returned map is agentID → list of agent IDs it can invoke via task.
//
// For workflow agents, permissions are derived from the graph structure:
//   - An agent at a checkpoint can invoke agents at directly downstream
//     checkpoints (next in the sequence).
//   - The orchestrator (first workflow agent) can invoke all workflow agents.
//   - Explicit TaskPermOverride.CanInvoke extends the implicit permissions.
//
// For independent agents, only explicit TaskPermOverride.CanInvoke is used.
//
// Disabled agents are excluded from all results.
func DeriveTaskPermissions(wf *WorkflowDefinition) map[string][]string {
	result := make(map[string][]string)

	active := wf.ActiveAgents()

	// Build a set of active agent IDs for filtering.
	activeSet := make(map[string]bool, len(active))
	for _, a := range active {
		activeSet[a.AgentID] = true
	}

	// Build checkpoint ordering for downstream analysis.
	cpOrder := make(map[string]int, len(wf.Checkpoints))
	for i, cp := range wf.Checkpoints {
		cpOrder[cp.ID] = i
	}

	// Map checkpoint → agents positioned after it.
	cpAgents := make(map[string][]string)
	for _, a := range active {
		if a.Role == RoleWorkflow && a.Position != nil {
			cpAgents[a.Position.AfterCheckpoint] = append(
				cpAgents[a.Position.AfterCheckpoint], a.AgentID,
			)
		}
	}

	for _, agent := range active {
		perms := make(map[string]bool)

		if agent.Role == RoleWorkflow && agent.Position != nil {
			// Find the checkpoint this agent sits at.
			cpIdx, ok := cpOrder[agent.Position.AfterCheckpoint]
			if ok {
				// Can invoke agents at the next checkpoint (one step downstream).
				if nextIdx := cpIdx + 1; nextIdx < len(wf.Checkpoints) {
					nextCP := wf.Checkpoints[nextIdx]
					for _, downstream := range cpAgents[nextCP.ID] {
						if activeSet[downstream] {
							perms[downstream] = true
						}
					}
					// Also include agents listed in the checkpoint itself.
					for _, cpAgent := range nextCP.Agents {
						if activeSet[cpAgent] && cpAgent != agent.AgentID {
							perms[cpAgent] = true
						}
					}
				}

				// Agents at the same checkpoint are not automatically invocable
				// unless explicitly configured — no action needed here.
			}
		}

		// Add explicit CanInvoke.
		if agent.TaskPermissions != nil {
			for _, target := range agent.TaskPermissions.CanInvoke {
				if activeSet[target] {
					perms[target] = true
				}
			}
		}

		// Also honor CanBeInvokedBy from other agents.
		for _, other := range active {
			if other.TaskPermissions != nil {
				for _, invoker := range other.TaskPermissions.CanBeInvokedBy {
					if invoker == agent.AgentID && activeSet[other.AgentID] {
						perms[other.AgentID] = true
					}
				}
			}
		}

		// Remove self-reference.
		delete(perms, agent.AgentID)

		// Convert to sorted slice.
		if len(perms) > 0 {
			list := make([]string, 0, len(perms))
			for id := range perms {
				list = append(list, id)
			}
			sortStrings(list)
			result[agent.AgentID] = list
		}
	}

	return result
}

// BuildTaskPermissionMap produces the opencode-compatible permission.task map
// for a single agent.  The output format is:
//
//	{"*": "deny", "agent-a": "allow", "agent-b": "allow", ...}
//
// If the agent has no task permissions, it returns {"*": "deny"}.
func BuildTaskPermissionMap(allowedAgents []string) map[string]string {
	m := map[string]string{"*": "deny"}
	for _, a := range allowedAgents {
		m[a] = "allow"
	}
	return m
}

// sortStrings sorts a string slice in-place (insertion sort, no import needed).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		key := ss[i]
		j := i - 1
		for j >= 0 && ss[j] > key {
			ss[j+1] = ss[j]
			j--
		}
		ss[j+1] = key
	}
}
