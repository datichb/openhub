// Package workflow defines the configurable workflow model for the hub.
//
// A WorkflowDefinition describes the full agent orchestration graph:
// checkpoints (ordered gates), agent slots (catalogue of available agents),
// workflow modes (manuel / semi-auto / auto), and a circuit-breaker.
//
// Definitions are built by resolving a hardcoded base workflow with zero or
// more WorkflowOverride layers (hub → team → project).  The resolved
// definition is consumed by the deploy pipeline to generate skill files and
// by the TUI to render an interactive graph.
package workflow

import "encoding/json"

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

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// WorkflowDefinition is the fully resolved workflow graph.
type WorkflowDefinition struct {
	// Version tracks schema evolution (semver-ish, e.g. "1.0").
	Version string `json:"version" toml:"version"`

	// Checkpoints is the ordered sequence of gates in the workflow.
	Checkpoints []Checkpoint `json:"checkpoints" toml:"checkpoints"`

	// Agents is the full catalogue of agent slots.
	Agents []AgentSlot `json:"agents" toml:"agents"`

	// Modes configures which workflow modes are available.
	Modes ModesConfig `json:"modes" toml:"modes"`

	// CircuitBreaker limits consecutive task invocations without user input.
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" toml:"circuit_breaker"`
}

// Checkpoint is a named gate in the workflow where the orchestrator may
// pause, auto-continue, skip, or evaluate a condition depending on the
// active workflow mode.
type Checkpoint struct {
	// ID is a unique slug (e.g. "cp-0", "cp-review-gate").
	ID string `json:"id" toml:"id"`

	// Label is a short human-readable name.
	Label string `json:"label" toml:"label"`

	// Description is an optional longer explanation.
	Description string `json:"description,omitempty" toml:"description,omitempty"`

	// Behavior maps each mode name to a CheckpointBehavior.
	// Keys are mode slugs: "manuel", "semi-auto", "auto", plus any custom modes.
	Behavior map[string]CheckpointBehavior `json:"behavior" toml:"behavior"`

	// Condition is a natural-language rule evaluated by the LLM when
	// the behavior for the current mode is BehaviorConditional.
	Condition string `json:"condition,omitempty" toml:"condition,omitempty"`

	// Mandatory checkpoints cannot be removed by lower-level overrides.
	Mandatory bool `json:"mandatory,omitempty" toml:"mandatory,omitempty"`

	// Agents lists agent IDs involved at this checkpoint.
	Agents []string `json:"agents,omitempty" toml:"agents,omitempty"`
}

// AgentSlot describes one agent's place in the workflow.
type AgentSlot struct {
	// AgentID matches the agent frontmatter "id" field.
	AgentID string `json:"agent_id" toml:"agent_id"`

	// Role determines how the agent participates.
	Role AgentRole `json:"role" toml:"role"`

	// Mode is the opencode agent mode (primary or subagent).
	Mode AgentMode `json:"mode" toml:"mode"`

	// Mandatory agents cannot be disabled by lower-level overrides.
	Mandatory bool `json:"mandatory,omitempty" toml:"mandatory,omitempty"`

	// Position places the agent in the workflow graph (only for RoleWorkflow).
	Position *WorkflowPosition `json:"position,omitempty" toml:"position,omitempty"`

	// TaskPermissions configures inter-agent invocation rights.
	// For workflow agents this is optional (derived from graph when nil).
	// For independent agents this is the only source of invocation config.
	TaskPermissions *TaskPermOverride `json:"task_permissions,omitempty" toml:"task_permissions,omitempty"`
}

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

// ModesConfig describes which workflow modes are available and the default.
type ModesConfig struct {
	// Available lists all available mode slugs.
	Available []string `json:"available" toml:"available"`

	// Default is the mode used when the user does not choose.
	Default string `json:"default" toml:"default"`
}

// CircuitBreakerConfig limits runaway autonomous loops.
type CircuitBreakerConfig struct {
	// MaxConsecutiveInvocations triggers a mandatory pause after N consecutive
	// task invocations without user interaction.  0 means disabled.
	MaxConsecutiveInvocations int `json:"max_consecutive_invocations" toml:"max_consecutive_invocations"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// FindCheckpoint returns the checkpoint with the given ID, or nil.
func (w *WorkflowDefinition) FindCheckpoint(id string) *Checkpoint {
	for i := range w.Checkpoints {
		if w.Checkpoints[i].ID == id {
			return &w.Checkpoints[i]
		}
	}
	return nil
}

// FindAgent returns the agent slot with the given ID, or nil.
func (w *WorkflowDefinition) FindAgent(id string) *AgentSlot {
	for i := range w.Agents {
		if w.Agents[i].AgentID == id {
			return &w.Agents[i]
		}
	}
	return nil
}

// ActiveAgents returns only non-disabled agent slots.
func (w *WorkflowDefinition) ActiveAgents() []AgentSlot {
	var out []AgentSlot
	for _, a := range w.Agents {
		if a.Role != RoleDisabled {
			out = append(out, a)
		}
	}
	return out
}

// WorkflowAgents returns agents with RoleWorkflow.
func (w *WorkflowDefinition) WorkflowAgents() []AgentSlot {
	var out []AgentSlot
	for _, a := range w.Agents {
		if a.Role == RoleWorkflow {
			out = append(out, a)
		}
	}
	return out
}

// IndependentAgents returns agents with RoleIndependent.
func (w *WorkflowDefinition) IndependentAgents() []AgentSlot {
	var out []AgentSlot
	for _, a := range w.Agents {
		if a.Role == RoleIndependent {
			out = append(out, a)
		}
	}
	return out
}

// DeepCopy returns an independent deep copy of the workflow definition.
func (w *WorkflowDefinition) DeepCopy() WorkflowDefinition {
	data, _ := json.Marshal(w)
	var clone WorkflowDefinition
	_ = json.Unmarshal(data, &clone)
	return clone
}
