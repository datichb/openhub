package sessionspec

import (
	"os"
	"strings"
)

// Workflow runtime of a session bundle (phase 3, checkpoints): the part of
// the oh/v1 workflow oh needs while a session runs (checkpoints, gates,
// circuit breaker, outputs), and the oh MCP server `workflow` that agents
// call at each checkpoint.

// WorkflowMCPServer is the name of the oh MCP server injected in every
// bundle built from a workflow (`oh mcp serve workflow`).
const WorkflowMCPServer = "workflow"

// Tools of the workflow MCP server.
const (
	WorkflowToolStatus     = "workflow_status"
	WorkflowToolCheckpoint = "workflow_checkpoint"
	WorkflowToolOutputs    = "workflow_outputs"
)

// OhBinVar stands for the oh executable in MCP server commands. Adapters
// expand it when they start a server (WithOhBinary); the bundle hash covers
// the unexpanded command, so it does not depend on where oh is installed.
const OhBinVar = "{{oh.bin}}"

// mcpActionPrefix marks a neutral permission action on an MCP tool.
const mcpActionPrefix = "mcp:"

// MCPToolAction is the neutral permission action of an MCP tool. Adapters
// translate it to the tool's own name (opencode V2: `<server>_<tool>`).
func MCPToolAction(server, tool string) string {
	return mcpActionPrefix + server + "/" + tool
}

// ParseMCPToolAction splits a neutral MCP tool action.
func ParseMCPToolAction(action string) (server, tool string, ok bool) {
	rest, found := strings.CutPrefix(action, mcpActionPrefix)
	if !found {
		return "", "", false
	}
	server, tool, ok = strings.Cut(rest, "/")
	return server, tool, ok && server != "" && tool != ""
}

// WorkflowMCPDef is the declaration of the workflow MCP server.
func WorkflowMCPDef() MCPServerDef {
	return MCPServerDef{Name: WorkflowMCPServer, Type: "local", Command: []string{OhBinVar, "mcp", "serve", WorkflowMCPServer}}
}

// Checkpoint behaviors (copied from the oh/v1 schema).
const (
	CheckpointPause       = "pause"
	CheckpointAuto        = "auto"
	CheckpointSkip        = "skip"
	CheckpointConditional = "conditional"
)

// WorkflowRuntime is what a running session needs from its workflow.
type WorkflowRuntime struct {
	ID string `json:"id"`
	// DefaultMode is the mode of a session started without one.
	DefaultMode string `json:"default_mode,omitempty"`
	// Checkpoints in declaration order (the order they are passed).
	Checkpoints []CheckpointDef `json:"checkpoints,omitempty"`
	// Gates lock agents until a checkpoint is passed or another agent ran.
	Gates []AgentGate `json:"gates,omitempty"`
	// MaxConsecutiveSubagents trips the circuit breaker (0 = disabled).
	MaxConsecutiveSubagents int         `json:"max_consecutive_subagents,omitempty"`
	Outputs                 []OutputDef `json:"outputs,omitempty"`
}

// CheckpointDef is a checkpoint and its behavior in each workflow mode.
type CheckpointDef struct {
	ID        string            `json:"id"`
	Label     map[string]string `json:"label,omitempty"` // language → text ("" = any language)
	Behaviors map[string]string `json:"behaviors"`       // mode → pause | auto | skip | conditional
	Condition string            `json:"condition,omitempty"`
	Mandatory bool              `json:"mandatory,omitempty"`
}

// Behavior returns the behavior of the checkpoint in mode (unset = pause; a
// mandatory checkpoint is never skipped).
func (c CheckpointDef) Behavior(mode string) string {
	b := c.Behaviors[mode]
	if b == "" || (c.Mandatory && b == CheckpointSkip) {
		return CheckpointPause
	}
	return b
}

// LabelFor returns the label in lang (fallback: any language, then the id).
func (c CheckpointDef) LabelFor(lang string) string {
	if s := c.Label[lang]; s != "" {
		return s
	}
	for _, l := range []string{"", "en", "fr"} {
		if s := c.Label[l]; s != "" {
			return s
		}
	}
	for _, s := range c.Label {
		if s != "" {
			return s
		}
	}
	return c.ID
}

// AgentGate locks Agent until After (a checkpoint or another agent) is done.
type AgentGate struct {
	Agent string `json:"agent"`
	After string `json:"after"`
}

// OutputDef is a typed output the session may declare.
type OutputDef struct {
	ID    string            `json:"id"`
	Type  string            `json:"type"`
	Label map[string]string `json:"label,omitempty"`
}

// Checkpoint returns the checkpoint with id.
func (w *WorkflowRuntime) Checkpoint(id string) (CheckpointDef, bool) {
	if w == nil {
		return CheckpointDef{}, false
	}
	for _, c := range w.Checkpoints {
		if c.ID == id {
			return c, true
		}
	}
	return CheckpointDef{}, false
}

// WithOhBinary returns a copy of b whose MCP commands run bin instead of
// OhBinVar (empty = the running executable). Idempotent.
func (b BundleSpec) WithOhBinary(bin string) BundleSpec {
	if bin == "" {
		bin = OhExecutable()
	}
	out := b
	if len(b.MCP) == 0 {
		return out
	}
	out.MCP = make([]MCPServerDef, len(b.MCP))
	for i, m := range b.MCP {
		out.MCP[i] = m
		if len(m.Command) == 0 {
			continue
		}
		out.MCP[i].Command = make([]string, len(m.Command))
		for j, arg := range m.Command {
			out.MCP[i].Command[j] = strings.ReplaceAll(arg, OhBinVar, bin)
		}
	}
	return out
}

// OhExecutable returns the path of the running oh executable ("oh" when unknown).
func OhExecutable() string {
	if p, err := os.Executable(); err == nil && p != "" {
		return p
	}
	return "oh"
}
