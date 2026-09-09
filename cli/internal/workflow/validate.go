package workflow

import (
	"fmt"
	"strings"
)

// Validate checks structural integrity of a resolved WorkflowDefinition.
// It returns a combined error listing all violations found.
func Validate(wf *WorkflowDefinition) error {
	var errs []string

	errs = append(errs, validateCheckpoints(wf)...)
	errs = append(errs, validateAgents(wf)...)
	errs = append(errs, validateModes(wf)...)
	errs = append(errs, validateCircuitBreaker(wf)...)

	if len(errs) > 0 {
		return fmt.Errorf("workflow validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

func validateCheckpoints(wf *WorkflowDefinition) []string {
	var errs []string

	if len(wf.Checkpoints) == 0 {
		errs = append(errs, "workflow must have at least one checkpoint")
		return errs
	}

	seen := make(map[string]bool)
	for _, cp := range wf.Checkpoints {
		if cp.ID == "" {
			errs = append(errs, "checkpoint has empty ID")
			continue
		}
		if seen[cp.ID] {
			errs = append(errs, fmt.Sprintf("duplicate checkpoint ID %q", cp.ID))
		}
		seen[cp.ID] = true

		// Every checkpoint must have a behavior for each available mode.
		for _, mode := range wf.Modes.Available {
			if _, ok := cp.Behavior[mode]; !ok {
				errs = append(errs, fmt.Sprintf("checkpoint %q missing behavior for mode %q", cp.ID, mode))
			}
		}

		// Conditional behavior requires a non-empty Condition.
		for mode, b := range cp.Behavior {
			if b == BehaviorConditional && strings.TrimSpace(cp.Condition) == "" {
				errs = append(errs, fmt.Sprintf("checkpoint %q has conditional behavior for mode %q but no condition defined", cp.ID, mode))
			}
		}

		// Valid behavior values.
		for mode, b := range cp.Behavior {
			switch b {
			case BehaviorPause, BehaviorAuto, BehaviorSkip, BehaviorConditional:
				// ok
			default:
				errs = append(errs, fmt.Sprintf("checkpoint %q has invalid behavior %q for mode %q", cp.ID, b, mode))
			}
		}

		// Referenced agents must exist.
		for _, agentID := range cp.Agents {
			if wf.FindAgent(agentID) == nil {
				errs = append(errs, fmt.Sprintf("checkpoint %q references unknown agent %q", cp.ID, agentID))
			}
		}
	}

	return errs
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func validateAgents(wf *WorkflowDefinition) []string {
	var errs []string

	seen := make(map[string]bool)
	cpIDs := make(map[string]bool)
	for _, cp := range wf.Checkpoints {
		cpIDs[cp.ID] = true
	}

	for _, a := range wf.Agents {
		if a.AgentID == "" {
			errs = append(errs, "agent has empty ID")
			continue
		}
		if seen[a.AgentID] {
			errs = append(errs, fmt.Sprintf("duplicate agent ID %q", a.AgentID))
		}
		seen[a.AgentID] = true

		// Valid role.
		switch a.Role {
		case RoleWorkflow, RoleIndependent, RoleDisabled:
			// ok
		default:
			errs = append(errs, fmt.Sprintf("agent %q has invalid role %q", a.AgentID, a.Role))
		}

		// Valid mode.
		switch a.Mode {
		case ModePrimary, ModeSubagent:
			// ok
		default:
			errs = append(errs, fmt.Sprintf("agent %q has invalid mode %q", a.AgentID, a.Mode))
		}

		// Workflow agents should have a position referencing a valid checkpoint.
		if a.Role == RoleWorkflow && a.Position != nil {
			if a.Position.AfterCheckpoint != "" && !cpIDs[a.Position.AfterCheckpoint] {
				errs = append(errs, fmt.Sprintf("agent %q position references unknown checkpoint %q",
					a.AgentID, a.Position.AfterCheckpoint))
			}
		}

		// TaskPermissions references must exist.
		if a.TaskPermissions != nil {
			for _, target := range a.TaskPermissions.CanInvoke {
				if wf.FindAgent(target) == nil {
					errs = append(errs, fmt.Sprintf("agent %q can_invoke references unknown agent %q", a.AgentID, target))
				}
			}
			for _, source := range a.TaskPermissions.CanBeInvokedBy {
				if wf.FindAgent(source) == nil {
					errs = append(errs, fmt.Sprintf("agent %q can_be_invoked_by references unknown agent %q", a.AgentID, source))
				}
			}
		}
	}

	return errs
}

// ---------------------------------------------------------------------------
// Modes
// ---------------------------------------------------------------------------

func validateModes(wf *WorkflowDefinition) []string {
	var errs []string

	if len(wf.Modes.Available) == 0 {
		errs = append(errs, "at least one workflow mode must be available")
	}

	seen := make(map[string]bool)
	for _, m := range wf.Modes.Available {
		if m == "" {
			errs = append(errs, "empty mode name in available modes")
		}
		if seen[m] {
			errs = append(errs, fmt.Sprintf("duplicate mode %q", m))
		}
		seen[m] = true
	}

	if wf.Modes.Default == "" {
		errs = append(errs, "default mode must be set")
	} else if !containsStr(wf.Modes.Available, wf.Modes.Default) {
		errs = append(errs, fmt.Sprintf("default mode %q is not in available modes", wf.Modes.Default))
	}

	return errs
}

// ---------------------------------------------------------------------------
// Circuit breaker
// ---------------------------------------------------------------------------

func validateCircuitBreaker(wf *WorkflowDefinition) []string {
	var errs []string
	if wf.CircuitBreaker.MaxConsecutiveInvocations < 0 {
		errs = append(errs, "circuit breaker max_consecutive_invocations cannot be negative")
	}
	return errs
}
