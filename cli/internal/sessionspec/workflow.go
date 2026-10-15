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

// Tools of the workflow MCP server. Tools reach the model as
// <server>_<tool> (the tool prefixes MCP tools), i.e. workflow_checkpoint: the name the agents
// and the generated skills use.
const (
	WorkflowToolStatus     = "status"
	WorkflowToolCheckpoint = "checkpoint"
	WorkflowToolOutputs    = "outputs"
	// Reports (modernisation L4): workflow_report, workflow_report_get,
	// workflow_reports.
	WorkflowToolReport    = "report"
	WorkflowToolReportGet = "report_get"
	WorkflowToolReports   = "reports"
)

// WorkflowTools are the tools of the workflow MCP server.
var WorkflowTools = []string{WorkflowToolStatus, WorkflowToolCheckpoint, WorkflowToolOutputs,
	WorkflowToolReport, WorkflowToolReportGet, WorkflowToolReports}

// LegacyWorkflowTools maps the tool names served before v5 finalisation
// (Q3-2: the model saw workflow_workflow_checkpoint) to the current ones, so
// that the events of a session started before the rename are still read.
var LegacyWorkflowTools = map[string]string{
	"workflow_status":     WorkflowToolStatus,
	"workflow_checkpoint": WorkflowToolCheckpoint,
	"workflow_outputs":    WorkflowToolOutputs,
}

// OhBinVar stands for the oh executable in MCP server commands. Adapters
// expand it when they start a server (WithOhBinary); the bundle hash covers
// the unexpanded command, so it does not depend on where oh is installed.
const OhBinVar = "{{oh.bin}}"

// mcpActionPrefix marks a neutral permission action on an MCP tool.
const mcpActionPrefix = "mcp:"

// MCPToolAction is the neutral permission action of an MCP tool. Adapters
// translate it to the tool's own name (e.g. `<server>_<tool>`).
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

// EnvGroupToken holds, in the environment of the tool server (and so of the
// workflow MCP server it runs), the token of its server group: the workflow
// MCP server sends it to the oh daemon, which refuses the workflow routes
// without it (v5 critical N1). Never in the shell of a session (the adapter
// removes it).
const EnvGroupToken = "OH_GROUP_TOKEN"

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
	// Scratch are the session scratch folders of the workflow (SCR).
	Scratch []ScratchDef `json:"scratch,omitempty"`

	// R3 (workflow_verify.go): developer → reviewer rounds per ticket
	// (`limits.max_fix_rounds`, 0 = no cap).
	MaxFixRounds int `json:"max_fix_rounds,omitempty"`

	// R7: Method is the method of the session ("" = standard): the
	// bundle holds only the checkpoints of this method.
	Method string `json:"method,omitempty"`

	// R6 (fanout.go): Fanout is the fan-out of a fan-out session (nil for
	// the others).
	Fanout *FanoutDef `json:"fanout,omitempty"`
	// r5-fixes: WritesGates hold the `writes:` paths of an agent until a
	// checkpoint is passed (`writes_after`).
	WritesGates []WritesGate `json:"writes_gates,omitempty"`
	// p6-debts: Writes are the `writes:` paths of each agent: oh checks the
	// git add and git commit lines of these paths the rules cannot read
	// (several paths, several -m).
	Writes map[string][]string `json:"writes,omitempty"`
}

// WritesGate holds the `writes:` paths of Agent until the checkpoint After
// is passed.
type WritesGate struct {
	Agent string   `json:"agent"`
	Paths []string `json:"paths"`
	After string   `json:"after"`
}

// ScratchDef is the scratch folder of an agent for a session: written by
// Owner, read by Readers, in that session only.
type ScratchDef struct {
	Owner   string   `json:"owner"`
	Readers []string `json:"readers,omitempty"`
}

// CheckpointDef is a checkpoint and its behavior in each workflow mode.
type CheckpointDef struct {
	ID        string            `json:"id"`
	Label     map[string]string `json:"label,omitempty"` // language → text ("" = any language)
	Behaviors map[string]string `json:"behaviors"`       // mode → pause | auto | skip | conditional
	Condition string            `json:"condition,omitempty"`
	Mandatory bool              `json:"mandatory,omitempty"`
	// Unlocks lists the operations locked until this checkpoint is passed.
	Unlocks []string `json:"unlocks,omitempty"`

	// R3 (workflow_verify.go): verification run by oh before the checkpoint
	// passes, and conditions that make it pause.
	Verify *VerifyDef `json:"verify,omitempty"`
	When   *WhenDef   `json:"when,omitempty"`

	// L3: the agents that pass the checkpoint (workflow `by:`, or deduced
	// from the delegation graph); oh warns when another agent passes it.
	By []string `json:"by,omitempty"`

	// LD-D: Wiki "sync" makes the checkpoint synchronize the wiki
	// (doc-sync): its card shows the whole queue of the project, and when it
	// is passed oh applies the accepted proposals and records the
	// synchronization.
	Wiki string `json:"wiki,omitempty"`

	// R7: Base: oh records the commit of the location when the checkpoint
	// is passed (a later verification counts the changes from there,
	// VerifyDef.Since).
	Base bool `json:"base,omitempty"`

	// r5-fixes: After is the checkpoint that must be passed first (a
	// checkpoint skipped in the mode, or absent from the session, counts as
	// passed): oh refuses this one until then.
	After string `json:"after,omitempty"`

	// p5-fixes: CommitNext is the later checkpoint that unlocks the commit
	// again (`unlock_again`): the commit window of this one holds one
	// commit, and opens only when there is something to commit.
	CommitNext string `json:"commit_next,omitempty"`

	// sweep-fanout: Tasks makes oh read, check and record the breakdown
	// given with the call (nil = none).
	Tasks *TasksCheckDef `json:"tasks,omitempty"`
}

// TasksCheckDef is the check of the breakdown given at a checkpoint
// (copied from the oh/v1 schema): at most Max tasks, each of a domain of
// Domains (empty: any).
type TasksCheckDef struct {
	Max     int      `json:"max"`
	Domains []string `json:"domains,omitempty"`
}

// WikiSync is the `wiki:` of a checkpoint that synchronizes the wiki.
const WikiSync = "sync"

// Operations a checkpoint unlocks (copied from the oh/v1 schema).
const (
	UnlockCommit = "commit"
	UnlockPush   = "push"
	UnlockClose  = "close"
	UnlockCreate = "create"
	UnlockApply  = "apply"
	// R8: publishing (risk: publish).
	UnlockTag = "tag"
	UnlockMR  = "mr"
	// R6: merging (fan-out: done by oh at the merge checkpoint).
	UnlockMerge = "merge"
)

// DefaultBeadsAllow is the `bd` allow-list of a workflow that declares no
// `beads:` block (read-only commands), applied by the Beads gateway and used
// as the parent list when a workflow layer patches such a workflow.
var DefaultBeadsAllow = []string{"show", "list", "ready", "search", "query", "children", "comments", "count", "status", "graph", "history"}

// Behavior returns the behavior of the checkpoint in mode (unset = pause; a
// mandatory checkpoint, or one that unlocks an operation, is never skipped).
func (c CheckpointDef) Behavior(mode string) string {
	b := c.Behaviors[mode]
	if b == "" || ((c.Mandatory || len(c.Unlocks) > 0) && b == CheckpointSkip) {
		return CheckpointPause
	}
	if c.Verify != nil && c.Verify.Ask && b != CheckpointSkip {
		return CheckpointPause // risk: read: the commands run once validated
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
	// Record is how oh records the output itself (R5: created, closed,
	// spec, report); such an output is never declared by an agent.
	Record string `json:"record,omitempty"`
	// B1: with record "written", the files read (Paths) and the regular
	// expression whose first group is recorded (Match, "" = the paths).
	Paths []string `json:"paths,omitempty"`
	Match string   `json:"match,omitempty"`
}

// Recorded returns the output recorded with kind record (first one).
func (w *WorkflowRuntime) Recorded(record string) (OutputDef, bool) {
	if w == nil {
		return OutputDef{}, false
	}
	for _, o := range w.Outputs {
		if o.Record == record {
			return o, true
		}
	}
	return OutputDef{}, false
}

// Checkpoint returns the checkpoint with id (case-insensitive: agent
// prompts often write "CP-1" for "cp-1").
func (w *WorkflowRuntime) Checkpoint(id string) (CheckpointDef, bool) {
	if w == nil {
		return CheckpointDef{}, false
	}
	id = strings.TrimSpace(id)
	for _, c := range w.Checkpoints {
		if strings.EqualFold(c.ID, id) {
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
