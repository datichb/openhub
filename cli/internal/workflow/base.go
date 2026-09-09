package workflow

// BaseWorkflow returns the default workflow definition.
//
// This is the canonical Go transcription of the workflow described in
// skills/shared/hub-workflow-reference.md and
// skills/orchestrator/orchestrator-workflow-modes.md.
//
// It serves as the starting point for all overrides (hub → team → project).
func BaseWorkflow() WorkflowDefinition {
	return WorkflowDefinition{
		Version:     "1.0",
		Checkpoints: baseCheckpoints(),
		Agents:      baseAgents(),
		Modes: ModesConfig{
			Available: []string{"manuel", "semi-auto", "auto"},
			Default:   "manuel",
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxConsecutiveInvocations: 12,
		},
	}
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

func baseCheckpoints() []Checkpoint {
	return []Checkpoint{
		{
			ID:          "cp-0",
			Label:       "Mode selection",
			Description: "Orchestrator presents ticket table, asks workflow mode, gets user confirmation.",
			Behavior: map[string]CheckpointBehavior{
				"manuel":    BehaviorPause,
				"semi-auto": BehaviorPause,
				"auto":      BehaviorPause,
			},
			Mandatory: true,
			Agents:    []string{"orchestrator"},
		},
		{
			ID:          "cp-routing",
			Label:       "Routing",
			Description: "Route to pathfinder (simple) or planner (complex) based on complexity scoring, then optionally to designer and/or auditor.",
			Behavior: map[string]CheckpointBehavior{
				"manuel":    BehaviorAuto,
				"semi-auto": BehaviorAuto,
				"auto":      BehaviorAuto,
			},
			Mandatory: true,
			Agents:    []string{"pathfinder", "planner", "designer", "auditor"},
		},
		{
			ID:          "cp-1",
			Label:       "Start implementation",
			Description: "Orchestrator-dev presents ticket details before delegating to developer agents.",
			Behavior: map[string]CheckpointBehavior{
				"manuel":    BehaviorPause,
				"semi-auto": BehaviorAuto,
				"auto":      BehaviorAuto,
			},
			Mandatory: false,
			Agents:    []string{"orchestrator-dev"},
		},
		{
			ID:          "cp-2",
			Label:       "Commit or correct",
			Description: "Orchestrator-dev presents reviewer report. User decides: commit, correct, or correct-security. ALWAYS pauses in ALL modes.",
			Behavior: map[string]CheckpointBehavior{
				"manuel":    BehaviorPause,
				"semi-auto": BehaviorPause,
				"auto":      BehaviorPause,
			},
			Mandatory: true,
			Agents:    []string{"orchestrator-dev", "reviewer"},
		},
		{
			ID:          "cp-3",
			Label:       "Next ticket",
			Description: "Post-commit confirmation, move to next ticket.",
			Behavior: map[string]CheckpointBehavior{
				"manuel":    BehaviorPause,
				"semi-auto": BehaviorAuto,
				"auto":      BehaviorAuto,
			},
			Mandatory: false,
			Agents:    []string{"orchestrator-dev"},
		},
	}
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func baseAgents() []AgentSlot {
	return []AgentSlot{
		// ---------------------------------------------------------------
		// Planning family — orchestrated chain
		// ---------------------------------------------------------------
		{
			AgentID:   "orchestrator",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-0"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{
					"pathfinder", "planner", "onboarder",
					"designer", "orchestrator-dev", "debugger",
					"documentarian",
				},
			},
		},
		{
			AgentID:   "orchestrator-dev",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{
					"developer", "developer-refactor", "developer-migrator",
					"reviewer", "documentarian",
				},
			},
		},
		{
			AgentID:   "planner",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "complex"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{"documentarian", "designer"},
			},
		},
		{
			AgentID:   "pathfinder",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "simple"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{"documentarian", "designer"},
			},
		},
		{
			AgentID:   "onboarder",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "unknown-project"},
		},
		// ---------------------------------------------------------------
		// Design family
		// ---------------------------------------------------------------
		{
			AgentID:   "designer",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "design"},
		},
		// ---------------------------------------------------------------
		// Auditor family
		// ---------------------------------------------------------------
		{
			AgentID:   "auditor",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "audit"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{"auditor-subagent", "documentarian"},
			},
		},
		{
			AgentID:   "auditor-subagent",
			Role:      RoleWorkflow,
			Mode:      ModeSubagent,
			Mandatory: false,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-routing", Branch: "audit"},
		},
		// ---------------------------------------------------------------
		// Quality family — workflow participants
		// ---------------------------------------------------------------
		{
			AgentID:   "reviewer",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-2"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{"documentarian", "reviewer"},
			},
		},
		{
			AgentID:   "debugger",
			Role:      RoleWorkflow,
			Mode:      ModePrimary,
			Mandatory: true,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-0", Branch: "bug"},
			TaskPermissions: &TaskPermOverride{
				CanInvoke: []string{"documentarian"},
			},
		},
		// ---------------------------------------------------------------
		// Developer family — subagents (invoked by orchestrator-dev)
		// ---------------------------------------------------------------
		{
			AgentID:   "developer",
			Role:      RoleWorkflow,
			Mode:      ModeSubagent,
			Mandatory: false,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-1"},
		},
		{
			AgentID:   "developer-refactor",
			Role:      RoleWorkflow,
			Mode:      ModeSubagent,
			Mandatory: false,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-1"},
		},
		{
			AgentID:   "developer-migrator",
			Role:      RoleWorkflow,
			Mode:      ModeSubagent,
			Mandatory: false,
			Position:  &WorkflowPosition{AfterCheckpoint: "cp-1"},
		},
		// ---------------------------------------------------------------
		// Independent agents — available on-demand, not in the chain
		// ---------------------------------------------------------------
		{
			AgentID:   "database",
			Role:      RoleIndependent,
			Mode:      ModePrimary,
			Mandatory: false,
			TaskPermissions: &TaskPermOverride{
				CanInvoke:      []string{"documentarian"},
				CanBeInvokedBy: []string{},
			},
		},
		{
			AgentID:   "infra",
			Role:      RoleIndependent,
			Mode:      ModePrimary,
			Mandatory: false,
			TaskPermissions: &TaskPermOverride{
				CanBeInvokedBy: []string{},
			},
		},
		{
			AgentID:   "test-generator",
			Role:      RoleIndependent,
			Mode:      ModePrimary,
			Mandatory: false,
			TaskPermissions: &TaskPermOverride{
				CanInvoke:      []string{"documentarian", "reviewer"},
				CanBeInvokedBy: []string{},
			},
		},
		{
			AgentID:   "benchmarker",
			Role:      RoleIndependent,
			Mode:      ModePrimary,
			Mandatory: false,
			TaskPermissions: &TaskPermOverride{
				CanInvoke:      []string{"documentarian"},
				CanBeInvokedBy: []string{},
			},
		},
		{
			AgentID:   "documentarian",
			Role:      RoleIndependent,
			Mode:      ModePrimary,
			Mandatory: true,
			TaskPermissions: &TaskPermOverride{
				CanBeInvokedBy: []string{
					"orchestrator", "orchestrator-dev", "planner",
					"pathfinder", "auditor", "debugger", "reviewer",
					"database", "test-generator", "benchmarker",
				},
			},
		},
		{
			AgentID:   "brief-enricher",
			Role:      RoleIndependent,
			Mode:      ModeSubagent,
			Mandatory: false,
		},
	}
}
