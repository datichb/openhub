// Package workflow defines the declarative oh/v1 workflows (schema.go):
// parsing, resolution by layers, validation, delegation graph, prompt
// rendering. The types below are shared by the schema (behaviors, roles,
// modes) and by the former overrides read once by the v38 migration
// (override.go, legacy.go); the former hard-coded workflow model was removed
// in v5 (P2-T18).
package workflow

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// CheckpointBehavior describes what happens at a checkpoint for a given mode.
type CheckpointBehavior string

const (
	BehaviorPause       CheckpointBehavior = "pause"       // always ask the user
	BehaviorAuto        CheckpointBehavior = "auto"        // proceed automatically
	BehaviorSkip        CheckpointBehavior = "skip"        // skip entirely
	BehaviorConditional CheckpointBehavior = "conditional" // evaluate Condition field
)

// AgentRole describes how an agent participates in the workflow.
type AgentRole string

const (
	RoleWorkflow    AgentRole = "workflow"    // part of the orchestrated chain
	RoleIndependent AgentRole = "independent" // available on-demand, not in the chain
	RoleDisabled    AgentRole = "disabled"    // not available at all
)

// AgentMode maps to the opencode agent mode.
type AgentMode string

const (
	ModePrimary  AgentMode = "primary"  // user can invoke directly
	ModeSubagent AgentMode = "subagent" // invocable only via task
)

// OverrideAction describes what an override does to a checkpoint.
type OverrideAction string

const (
	ActionModify OverrideAction = "modify"
	ActionAdd    OverrideAction = "add"
	ActionRemove OverrideAction = "remove"
)

// WorkflowPosition locates an agent relative to a checkpoint.
type WorkflowPosition struct {
	// AfterCheckpoint is the ID of the checkpoint this agent sits after.
	AfterCheckpoint string `json:"after_checkpoint" toml:"after_checkpoint"`

	// Branch is the fork branch name (empty for the main path).
	Branch string `json:"branch,omitempty" toml:"branch,omitempty"`
}

// TaskPermOverride explicitly configures inter-agent invocation.
type TaskPermOverride struct {
	// CanInvoke lists agent IDs this agent can call via task.
	// For workflow agents this extends the implicit graph-derived permissions.
	CanInvoke []string `json:"can_invoke,omitempty" toml:"can_invoke,omitempty"`

	// CanBeInvokedBy lists agent IDs that can call this agent via task.
	CanBeInvokedBy []string `json:"can_be_invoked_by,omitempty" toml:"can_be_invoked_by,omitempty"`
}

