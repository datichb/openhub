package workflow

// WorkflowOverride describes modifications to apply on top of a base (or
// already-resolved) WorkflowDefinition.  Each configuration level (hub, team,
// project) stores at most one WorkflowOverride.
//
// Only non-nil / non-zero fields take effect — the rest is inherited.
type WorkflowOverride struct {
	// CheckpointOverrides modifies, adds, or removes checkpoints.
	CheckpointOverrides []CheckpointOverride `json:"checkpoint_overrides,omitempty" toml:"checkpoint_overrides,omitempty"`

	// AgentOverrides modifies agent slots (role, mode, permissions, enabled state).
	AgentOverrides []AgentOverride `json:"agent_overrides,omitempty" toml:"agent_overrides,omitempty"`

	// ModeOverrides changes available modes and/or the default mode.
	ModeOverrides *ModesOverride `json:"mode_overrides,omitempty" toml:"mode_overrides,omitempty"`

	// CircuitBreakerOverride replaces the max consecutive invocations.
	// nil means inherited.
	CircuitBreakerOverride *int `json:"circuit_breaker_override,omitempty" toml:"circuit_breaker_override,omitempty"`

	// Enforced prevents lower-level overrides from modifying the workflow.
	// When true at team level, projects in that team cannot add their own overrides.
	Enforced bool `json:"enforced,omitempty" toml:"enforced,omitempty"`
}

// IsEmpty returns true if the override has no modifications.
func (o *WorkflowOverride) IsEmpty() bool {
	if o == nil {
		return true
	}
	return len(o.CheckpointOverrides) == 0 &&
		len(o.AgentOverrides) == 0 &&
		o.ModeOverrides == nil &&
		o.CircuitBreakerOverride == nil
}

// CheckpointOverride describes a single modification to a checkpoint.
type CheckpointOverride struct {
	// ID is the target checkpoint ID (required for all actions).
	ID string `json:"id" toml:"id"`

	// Action is what to do: modify an existing checkpoint, add a new one, or remove one.
	Action OverrideAction `json:"action" toml:"action"`

	// --- Fields below are used by ActionModify and ActionAdd ---

	// Label overrides the checkpoint label (nil = keep existing).
	Label *string `json:"label,omitempty" toml:"label,omitempty"`

	// Description overrides the checkpoint description (nil = keep existing).
	Description *string `json:"description,omitempty" toml:"description,omitempty"`

	// Behavior overrides per-mode behaviors.  Only specified modes are changed;
	// unspecified modes keep their existing behavior.
	Behavior map[string]CheckpointBehavior `json:"behavior,omitempty" toml:"behavior,omitempty"`

	// Condition overrides the natural-language condition (nil = keep existing).
	Condition *string `json:"condition,omitempty" toml:"condition,omitempty"`

	// Agents replaces the agent list for this checkpoint (nil = keep existing).
	Agents *[]string `json:"agents,omitempty" toml:"agents,omitempty"`

	// --- Fields below are used only by ActionAdd ---

	// InsertAfter is the ID of the checkpoint after which to insert the new one.
	// Empty string means insert at the beginning.
	InsertAfter *string `json:"insert_after,omitempty" toml:"insert_after,omitempty"`
}

// AgentOverride describes a modification to a single agent slot.
type AgentOverride struct {
	// AgentID is the target agent (required).
	AgentID string `json:"agent_id" toml:"agent_id"`

	// Role changes the agent's workflow role (nil = keep existing).
	Role *AgentRole `json:"role,omitempty" toml:"role,omitempty"`

	// Mode changes the agent mode of the tool (nil = keep existing).
	Mode *AgentMode `json:"mode,omitempty" toml:"mode,omitempty"`

	// Disabled removes the agent from the workflow entirely.
	// This is equivalent to setting Role to RoleDisabled but is more
	// explicit and works as a simple toggle.
	Disabled *bool `json:"disabled,omitempty" toml:"disabled,omitempty"`

	// Position changes where the agent sits in the workflow graph (nil = keep).
	// Only meaningful when Role is RoleWorkflow.
	Position *WorkflowPosition `json:"position,omitempty" toml:"position,omitempty"`

	// TaskPermissions overrides the agent's invocation rights (nil = keep).
	TaskPermissions *TaskPermOverride `json:"task_permissions,omitempty" toml:"task_permissions,omitempty"`
}

// ModesOverride changes the available workflow modes and/or the default.
type ModesOverride struct {
	// Available replaces the list of available modes (nil = keep existing).
	Available *[]string `json:"available,omitempty" toml:"available,omitempty"`

	// Default changes the default mode (nil = keep existing).
	Default *string `json:"default,omitempty" toml:"default,omitempty"`
}
