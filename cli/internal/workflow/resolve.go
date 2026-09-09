package workflow

import "fmt"

// Resolve applies a sequence of WorkflowOverride layers on top of a base
// WorkflowDefinition and returns the fully resolved result.
//
// Overrides are applied in order (hub → team → project).  If an override
// has Enforced=true, subsequent overrides are skipped.
//
// The returned definition is validated before return — callers can assume
// it is structurally sound.
func Resolve(base WorkflowDefinition, overrides ...WorkflowOverride) (WorkflowDefinition, error) {
	result := base.DeepCopy()

	for i, ov := range overrides {
		if ov.IsEmpty() && !ov.Enforced {
			continue
		}

		if err := applyOverride(&result, &ov); err != nil {
			return WorkflowDefinition{}, fmt.Errorf("override #%d: %w", i, err)
		}

		// If this override is enforced, stop processing further layers.
		if ov.Enforced {
			break
		}
	}

	if err := Validate(&result); err != nil {
		return WorkflowDefinition{}, fmt.Errorf("resolved workflow invalid: %w", err)
	}

	return result, nil
}

// applyOverride mutates wf by applying a single override.
func applyOverride(wf *WorkflowDefinition, ov *WorkflowOverride) error {
	// 1. Checkpoint overrides
	for _, co := range ov.CheckpointOverrides {
		if err := applyCheckpointOverride(wf, &co); err != nil {
			return fmt.Errorf("checkpoint %q: %w", co.ID, err)
		}
	}

	// 2. Agent overrides
	for _, ao := range ov.AgentOverrides {
		if err := applyAgentOverride(wf, &ao); err != nil {
			return fmt.Errorf("agent %q: %w", ao.AgentID, err)
		}
	}

	// 3. Mode overrides
	if ov.ModeOverrides != nil {
		applyModeOverride(wf, ov.ModeOverrides)
	}

	// 4. Circuit breaker
	if ov.CircuitBreakerOverride != nil {
		wf.CircuitBreaker.MaxConsecutiveInvocations = *ov.CircuitBreakerOverride
	}

	return nil
}

// ---------------------------------------------------------------------------
// Checkpoint overrides
// ---------------------------------------------------------------------------

func applyCheckpointOverride(wf *WorkflowDefinition, co *CheckpointOverride) error {
	switch co.Action {
	case ActionModify:
		return modifyCheckpoint(wf, co)
	case ActionAdd:
		return addCheckpoint(wf, co)
	case ActionRemove:
		return removeCheckpoint(wf, co)
	default:
		return fmt.Errorf("unknown action %q", co.Action)
	}
}

func modifyCheckpoint(wf *WorkflowDefinition, co *CheckpointOverride) error {
	cp := wf.FindCheckpoint(co.ID)
	if cp == nil {
		return fmt.Errorf("checkpoint not found")
	}

	if co.Label != nil {
		cp.Label = *co.Label
	}
	if co.Description != nil {
		cp.Description = *co.Description
	}
	if co.Condition != nil {
		cp.Condition = *co.Condition
	}
	if co.Agents != nil {
		cp.Agents = *co.Agents
	}

	// Merge behaviors: only override specified modes, keep the rest.
	for mode, behavior := range co.Behavior {
		cp.Behavior[mode] = behavior
	}

	return nil
}

func addCheckpoint(wf *WorkflowDefinition, co *CheckpointOverride) error {
	// Verify the ID doesn't already exist.
	if wf.FindCheckpoint(co.ID) != nil {
		return fmt.Errorf("checkpoint already exists")
	}

	newCP := Checkpoint{
		ID:        co.ID,
		Behavior:  make(map[string]CheckpointBehavior),
		Mandatory: false, // user-added checkpoints are never mandatory
	}

	if co.Label != nil {
		newCP.Label = *co.Label
	}
	if co.Description != nil {
		newCP.Description = *co.Description
	}
	if co.Condition != nil {
		newCP.Condition = *co.Condition
	}
	if co.Agents != nil {
		newCP.Agents = *co.Agents
	}

	// Copy specified behaviors; default to "pause" for unspecified modes.
	for _, mode := range wf.Modes.Available {
		if b, ok := co.Behavior[mode]; ok {
			newCP.Behavior[mode] = b
		} else {
			newCP.Behavior[mode] = BehaviorPause
		}
	}

	// Insert at the right position.
	insertIdx := len(wf.Checkpoints) // default: append at end
	if co.InsertAfter != nil {
		if *co.InsertAfter == "" {
			insertIdx = 0
		} else {
			found := false
			for i, cp := range wf.Checkpoints {
				if cp.ID == *co.InsertAfter {
					insertIdx = i + 1
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("insert_after %q: checkpoint not found", *co.InsertAfter)
			}
		}
	}

	// Splice in.
	wf.Checkpoints = append(wf.Checkpoints, Checkpoint{})
	copy(wf.Checkpoints[insertIdx+1:], wf.Checkpoints[insertIdx:])
	wf.Checkpoints[insertIdx] = newCP

	return nil
}

func removeCheckpoint(wf *WorkflowDefinition, co *CheckpointOverride) error {
	for i, cp := range wf.Checkpoints {
		if cp.ID == co.ID {
			if cp.Mandatory {
				return fmt.Errorf("cannot remove mandatory checkpoint")
			}
			wf.Checkpoints = append(wf.Checkpoints[:i], wf.Checkpoints[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("checkpoint not found")
}

// ---------------------------------------------------------------------------
// Agent overrides
// ---------------------------------------------------------------------------

func applyAgentOverride(wf *WorkflowDefinition, ao *AgentOverride) error {
	slot := wf.FindAgent(ao.AgentID)
	if slot == nil {
		return fmt.Errorf("agent not found")
	}

	// Check mandatory constraints before disabling.
	if ao.Disabled != nil && *ao.Disabled && slot.Mandatory {
		return fmt.Errorf("cannot disable mandatory agent")
	}

	if ao.Disabled != nil && *ao.Disabled {
		slot.Role = RoleDisabled
		return nil
	}

	if ao.Role != nil {
		if slot.Mandatory && *ao.Role == RoleDisabled {
			return fmt.Errorf("cannot disable mandatory agent via role change")
		}
		slot.Role = *ao.Role
	}

	if ao.Mode != nil {
		slot.Mode = *ao.Mode
	}

	if ao.Position != nil {
		slot.Position = ao.Position
	}

	if ao.TaskPermissions != nil {
		slot.TaskPermissions = ao.TaskPermissions
	}

	return nil
}

// ---------------------------------------------------------------------------
// Mode overrides
// ---------------------------------------------------------------------------

func applyModeOverride(wf *WorkflowDefinition, mo *ModesOverride) {
	if mo.Available != nil {
		wf.Modes.Available = *mo.Available

		// Update checkpoint behaviors: ensure all checkpoints have entries
		// for the new mode set.
		for i := range wf.Checkpoints {
			cp := &wf.Checkpoints[i]
			// Add missing modes with "pause" default.
			for _, mode := range wf.Modes.Available {
				if _, ok := cp.Behavior[mode]; !ok {
					cp.Behavior[mode] = BehaviorPause
				}
			}
			// Remove behaviors for modes that no longer exist.
			for mode := range cp.Behavior {
				if !containsStr(wf.Modes.Available, mode) {
					delete(cp.Behavior, mode)
				}
			}
		}
	}

	if mo.Default != nil {
		wf.Modes.Default = *mo.Default
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
