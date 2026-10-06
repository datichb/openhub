package workflow

import (
	"gopkg.in/yaml.v3"
)

// This file defines the declarative workflow document, schema `oh/v1`.
//
// A Spec is what a YAML file contains (hub, team, project or draft layer).
// The same shape is used for a full definition and for a patch on top of
// another workflow (`extends`): when `extends` is set, every field that is
// set in the document overrides the parent, the rest is inherited.
//
// Patch rules (applied by the resolver):
//   - scalars and pointers: override when set;
//   - lists: replace the parent list as a whole;
//   - ordered maps (inputs, agents, checkpoints): merged by key; new keys are
//     appended; an agent is removed with `role: disabled`, a checkpoint with
//     `disabled: true` (refused if the parent marks it mandatory); inputs
//     cannot be removed;
//   - security fields (risk, isolation, runtime.allowed, mandatory
//     checkpoints, beads.allow, remote policies) may only be hardened.
//
// The legacy WorkflowDefinition (types.go) stays readable during the
// migration; it is not an oh/v1 document.

// APIVersionV1 and KindWorkflow identify an oh/v1 workflow document.
const (
	APIVersionV1 = "oh/v1"
	KindWorkflow = "Workflow"
)

// Category groups workflows in the launcher.
type Category string

const (
	CategoryDevelop   Category = "develop"
	CategoryFrame     Category = "frame"
	CategoryQuality   Category = "quality"
	CategoryKnowledge Category = "knowledge"
	CategoryOther     Category = "other"
)

// Risk is the highest effect a workflow may have on the project.
type Risk string

const (
	RiskRead    Risk = "read"    // no file edits, no Beads writes
	RiskPlan    Risk = "plan"    // no file edits, Beads writes limited to beads.allow
	RiskWrite   Risk = "write"   // edits files and Beads
	RiskPublish Risk = "publish" // may push branches or open merge requests
)

// Isolation is the required closed-world guarantee.
type Isolation string

const (
	IsolationStrict   Isolation = "strict"   // requires an adapter with full isolation
	IsolationStandard Isolation = "standard" // partial isolation accepted (warning)
)

// RemotePolicy says how a checkpoint is handled when the session runs on CI.
type RemotePolicy string

const (
	RemoteAuto   RemotePolicy = "auto"   // approved automatically by the policy responder
	RemoteDefer  RemotePolicy = "defer"  // stop cleanly and wait for the user locally
	RemoteForbid RemotePolicy = "forbid" // the workflow cannot run remotely
)

// InputType is the type of a launch input.
type InputType string

const (
	InputString   InputType = "string"
	InputText     InputType = "text"
	InputBool     InputType = "bool"
	InputInt      InputType = "int"
	InputEnum     InputType = "enum"
	InputPath     InputType = "path"
	InputBranch   InputType = "branch"
	InputBeadsID  InputType = "beads-id"
	InputBeadsIDs InputType = "beads-ids"
)

// OutputType is the type of a value declared by a session (O7).
type OutputType string

const (
	OutputBranch       OutputType = "branch"
	OutputMergeRequest OutputType = "merge_request"
	OutputBeadsIDs     OutputType = "beads-ids"
	OutputPath         OutputType = "path"
)

// Runtime names an execution environment.
type Runtime string

const (
	RuntimeLocal     Runtime = "local"
	RuntimeContainer Runtime = "container"
	RuntimeRemote    Runtime = "remote"
)

// Workflow modes shipped by the hub. Custom modes are allowed.
const (
	ModeManual   = "manuel"
	ModeSemiAuto = "semi-auto"
	ModeAuto     = "auto"
)

// Spec is an oh/v1 workflow document.
type Spec struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	// ID is kebab-case and unique within its layer.
	ID string `yaml:"id"`
	// Version is managed by publication; never edited by hand.
	Version int `yaml:"version,omitempty"`

	Category    Category      `yaml:"category,omitempty"`
	Label       LocalizedText `yaml:"label,omitempty"`
	Description LocalizedText `yaml:"description,omitempty"`

	// Extends references the parent workflow: "hub:<id>", "team:<id>" or
	// "project:<id>". Empty for a full definition.
	Extends string `yaml:"extends,omitempty"`

	Risk      Risk      `yaml:"risk,omitempty"`
	Isolation Isolation `yaml:"isolation,omitempty"`
	// CodeMode enables the tool's code execution mode (off by default).
	CodeMode *bool `yaml:"code_mode,omitempty"`

	Entry  *Entry               `yaml:"entry,omitempty"`
	Inputs OrderedMap[Input]    `yaml:"inputs,omitempty"`
	Prompt *Prompt              `yaml:"prompt,omitempty"`
	Agents OrderedMap[AgentRef] `yaml:"agents,omitempty"`
	// Checkpoints are passed in declaration order.
	Checkpoints    OrderedMap[CheckpointSpec] `yaml:"checkpoints,omitempty"`
	Modes          *Modes                     `yaml:"modes,omitempty"`
	CircuitBreaker *CircuitBreaker            `yaml:"circuit_breaker,omitempty"`
	Models         *Models                    `yaml:"models,omitempty"`
	Skills         *SkillSelection            `yaml:"skills,omitempty"`
	Plugins        []PluginRef                `yaml:"plugins,omitempty"`
	// MCP lists oh MCP server ids (gitlab, figma, jira, team…).
	MCP     []string     `yaml:"mcp,omitempty"`
	Beads   *BeadsAccess `yaml:"beads,omitempty"`
	Runtime *RuntimeSpec `yaml:"runtime,omitempty"`
	Outputs []Output     `yaml:"outputs,omitempty"`
	Limits  *Limits      `yaml:"limits,omitempty"`
	// Preconditions are checked before launch, in declaration order.
	Preconditions OrderedMap[Precondition] `yaml:"preconditions,omitempty"`
}

// Entry selects the agent the session starts on.
type Entry struct {
	// Agent defaults to the generic "conductor" when empty.
	Agent string `yaml:"agent,omitempty"`
}

// Input is a launch input; its key in Spec.Inputs is the variable name used
// by the prompt template.
type Input struct {
	Type     InputType     `yaml:"type"`
	Required bool          `yaml:"required,omitempty"`
	Default  any           `yaml:"default,omitempty"` // may use {{ .other_input }}
	Label    LocalizedText `yaml:"label,omitempty"`
	Help     LocalizedText `yaml:"help,omitempty"`
	// Values lists the choices of an enum.
	Values []string `yaml:"values,omitempty"`
	// Picker configures the Beads picker (beads-id, beads-ids).
	Picker *Picker `yaml:"picker,omitempty"`
	// MaxLength truncates the injected value (O11). 0 = adapter default.
	MaxLength int `yaml:"max_length,omitempty"`
}

// Picker configures ticket selection.
type Picker struct {
	Filter string `yaml:"filter,omitempty"` // e.g. ai-delegated
	Epic   string `yaml:"epic,omitempty"`   // restrict to an epic
	// Multi allows selecting several tickets: one session per ticket.
	Multi bool `yaml:"multi,omitempty"`
}

// Prompt is the initial prompt. Exactly one of Template or Text is set.
type Prompt struct {
	// Template is a Go text/template path, relative to the layer's prompts
	// directory (e.g. "prompts/ticket.md.tmpl").
	Template string `yaml:"template,omitempty"`
	// Text is an inline template.
	Text string `yaml:"text,omitempty"`
}

// AgentRef places an agent from the brick catalogue in the workflow.
type AgentRef struct {
	Role AgentRole `yaml:"role"`
	// Mode overrides the agent's own mode (primary / subagent).
	Mode AgentMode `yaml:"mode,omitempty"`
	// After gates the agent: it can only be called once this checkpoint has
	// been passed, or once this agent has run.
	After string `yaml:"after,omitempty"`
	// Calls lists the agents this agent may delegate to. Omitted: derived
	// from the agent's own task permissions, restricted to the workflow.
	Calls []string `yaml:"calls,omitempty"`
}

// CheckpointSpec is a gate of the workflow.
type CheckpointSpec struct {
	Label       LocalizedText `yaml:"label,omitempty"`
	Description LocalizedText `yaml:"description,omitempty"`
	// Mode maps each workflow mode to its behavior.
	Mode map[string]CheckpointBehavior `yaml:"mode,omitempty"`
	// Condition is evaluated when the behavior is "conditional".
	Condition string       `yaml:"condition,omitempty"`
	Remote    RemotePolicy `yaml:"remote,omitempty"`
	Mandatory *bool        `yaml:"mandatory,omitempty"`
	// Disabled removes an inherited checkpoint (patch only).
	Disabled bool `yaml:"disabled,omitempty"`
}

// Modes lists the allowed workflow modes.
type Modes struct {
	Default string   `yaml:"default,omitempty"`
	Allowed []string `yaml:"allowed,omitempty"`
}

// CircuitBreaker stops runaway delegation loops.
type CircuitBreaker struct {
	// MaxConsecutiveSubagents pauses after N consecutive subagent calls
	// without user interaction. 0 disables the breaker.
	MaxConsecutiveSubagents *int `yaml:"max_consecutive_subagents,omitempty"`
}

// Models is the workflow level of the model cascade (O9).
type Models struct {
	Default string            `yaml:"default,omitempty"` // provider/model[#variant]
	Agents  map[string]string `yaml:"agents,omitempty"`
}

// SkillSelection adjusts the skill closure computed from the agents.
type SkillSelection struct {
	Extra []string `yaml:"extra,omitempty"`
	Deny  []string `yaml:"deny,omitempty"`
}

// PluginRef declares a tool plugin (O5). A plain string is shorthand for
// `{ id: <string> }`.
type PluginRef struct {
	ID      string         `yaml:"id"`
	Options map[string]any `yaml:"options,omitempty"`
}

// UnmarshalYAML accepts the scalar shorthand.
func (p *PluginRef) UnmarshalYAML(unmarshal func(any) error) error {
	node, err := captureNode(unmarshal)
	if err != nil {
		return err
	}
	if node.Kind == yaml.ScalarNode {
		*p = PluginRef{ID: node.Value}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nodeError(node, "expected_plugin")
	}
	type plain PluginRef
	var v plain
	if err := unmarshal(&v); err != nil {
		return err
	}
	*p = PluginRef(v)
	return nil
}

// BeadsAccess is the allow-list of `bd` subcommands available to agents.
type BeadsAccess struct {
	Allow []string `yaml:"allow,omitempty"`
}

// RuntimeSpec lists where the workflow may run.
type RuntimeSpec struct {
	Default Runtime   `yaml:"default,omitempty"`
	Allowed []Runtime `yaml:"allowed,omitempty"`
}

// Output is a typed value the session may declare (workflow_outputs).
type Output struct {
	ID    string        `yaml:"id"`
	Type  OutputType    `yaml:"type"`
	Label LocalizedText `yaml:"label,omitempty"`
}

// Precondition is a check made before launch (in the session location).
// When it fails, oh shows Label and suggests another workflow, or refuses
// the launch (OnFail: block).
type Precondition struct {
	Label  LocalizedText      `yaml:"label,omitempty"`
	Check  PreconditionCheck  `yaml:"check"`
	OnFail PreconditionOnFail `yaml:"on_fail,omitempty"`
	// Suggest is the workflow offered instead (optional).
	Suggest *PreconditionSuggest `yaml:"suggest,omitempty"`
	// Disabled removes an inherited precondition (patch only).
	Disabled bool `yaml:"disabled,omitempty"`
}

// PreconditionCheck holds exactly one test.
type PreconditionCheck struct {
	// PathExists is satisfied when one of the paths (relative to the
	// session location) exists.
	PathExists []string `yaml:"path_exists,omitempty"`
}

// PreconditionOnFail is what a failed precondition does.
type PreconditionOnFail string

// Failed precondition behaviors.
const (
	OnFailSuggest PreconditionOnFail = "suggest" // message + suggested workflow; the user may go on
	OnFailBlock   PreconditionOnFail = "block"   // the launch is refused
)

// PreconditionSuggest is the workflow offered when a precondition fails.
type PreconditionSuggest struct {
	Workflow string `yaml:"workflow"`
	// Resume offers to relaunch this workflow, with the same inputs, once
	// the suggested one has finished (O7).
	Resume bool `yaml:"resume,omitempty"`
}

// Limits are optional per-session caps (I6).
type Limits struct {
	BudgetUSD *float64 `yaml:"budget_usd,omitempty"`
}

// ---------------------------------------------------------------------------
// Enum checks
// ---------------------------------------------------------------------------

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	switch c {
	case CategoryDevelop, CategoryFrame, CategoryQuality, CategoryKnowledge, CategoryOther:
		return true
	}
	return false
}

// Valid reports whether r is a known risk level.
func (r Risk) Valid() bool {
	switch r {
	case RiskRead, RiskPlan, RiskWrite, RiskPublish:
		return true
	}
	return false
}

// Rank orders risk levels (read < plan < write < publish); 0 for unknown
// values.
func (r Risk) Rank() int {
	switch r {
	case RiskRead:
		return 1
	case RiskPlan:
		return 2
	case RiskWrite:
		return 3
	case RiskPublish:
		return 4
	}
	return 0
}

// Valid reports whether i is a known isolation level.
func (i Isolation) Valid() bool {
	return i == IsolationStrict || i == IsolationStandard
}

// Valid reports whether p is a known remote policy.
func (p RemotePolicy) Valid() bool {
	switch p {
	case RemoteAuto, RemoteDefer, RemoteForbid:
		return true
	}
	return false
}

// Valid reports whether o is a known failed precondition behavior.
func (o PreconditionOnFail) Valid() bool {
	return o == OnFailSuggest || o == OnFailBlock
}

// Valid reports whether t is a known input type.
func (t InputType) Valid() bool {
	switch t {
	case InputString, InputText, InputBool, InputInt, InputEnum,
		InputPath, InputBranch, InputBeadsID, InputBeadsIDs:
		return true
	}
	return false
}

// Valid reports whether t is a known output type.
func (t OutputType) Valid() bool {
	switch t {
	case OutputBranch, OutputMergeRequest, OutputBeadsIDs, OutputPath:
		return true
	}
	return false
}

// Valid reports whether r is a known runtime.
func (r Runtime) Valid() bool {
	switch r {
	case RuntimeLocal, RuntimeContainer, RuntimeRemote:
		return true
	}
	return false
}

// EntryAgent returns the entry agent, defaulting to the conductor.
func (s *Spec) EntryAgent() string {
	if s.Entry != nil && s.Entry.Agent != "" {
		return s.Entry.Agent
	}
	return DefaultEntryAgent
}

// DefaultEntryAgent is the generic orchestrator used when entry.agent is empty.
const DefaultEntryAgent = "conductor"
