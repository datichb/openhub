package workflow

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Composition of workflows (R5, E4): what follows a session, the outputs oh
// records itself, the inputs computed from a previous session, and the
// other ids a workflow answers to.
//
//	aliases: [cadrage]                        # oh run cadrage → this workflow
//	outputs:
//	  - { id: tickets, type: beads-ids, record: created }   # recorded by oh
//	  - { id: spec, type: path, record: spec }
//	next:
//	  - workflow: implement-spec
//	    when: { outputs: [tickets] }
//	    inputs: { tickets: session.output(tickets) }
//
//	inputs:                                   # in implement-spec
//	  tickets: { type: beads-ids, from: session.output(spec.tickets) }
//
// `next:` is a proposal: at the end of a session oh offers the workflows
// whose conditions hold, with their inputs prefilled, and never launches
// one by itself. Lists are replaced as a whole by a patch.

// NextStep is a workflow proposed once a session of this one ends.
type NextStep struct {
	Workflow string        `yaml:"workflow"`
	Label    LocalizedText `yaml:"label,omitempty"`
	// When: proposed only when the session recorded every output listed.
	When *NextWhen `yaml:"when,omitempty"`
	// Inputs prefill inputs of the next workflow, by input name, with an
	// output of the session that ends: "session.output(<output id>)".
	Inputs map[string]string `yaml:"inputs,omitempty"`
}

// NextWhen is the condition of a NextStep.
type NextWhen struct {
	Outputs []string `yaml:"outputs,omitempty"`
	// Equals (B1): proposed only when each output listed was recorded with
	// this value (case ignored): `equals: { verdict: go }`.
	Equals map[string]string `yaml:"equals,omitempty"`
}

// Holds reports whether the condition holds for the outputs of a session
// (a nil condition always holds).
func (w *NextWhen) Holds(outputs map[string]any) bool {
	if w == nil {
		return true
	}
	for _, id := range w.Outputs {
		if outputValue(outputs[id]) == "" {
			return false
		}
	}
	for id, want := range w.Equals {
		if !strings.EqualFold(outputValue(outputs[id]), strings.TrimSpace(want)) {
			return false
		}
	}
	return true
}

// outputValue is a recorded output as text (lists comma-joined).
func outputValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(x, ",")
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, fmt.Sprint(e))
		}
		return strings.Join(parts, ",")
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// Alias is another id that launches the workflow (`oh run <alias>`). A
// document with that id, in any layer, wins over the alias.
type Alias struct {
	ID string `yaml:"id"`
	// With routes the alias to this workflow only when this input is given
	// at launch (`--tickets` fills the ticket input); otherwise the alias
	// goes to the workflow that declares it without `with`.
	With string `yaml:"with,omitempty"`
}

// UnmarshalYAML accepts the scalar shorthand (`aliases: [cadrage]`).
func (a *Alias) UnmarshalYAML(unmarshal func(any) error) error {
	node, err := captureNode(unmarshal)
	if err != nil {
		return err
	}
	if node.Kind == yaml.ScalarNode {
		*a = Alias{ID: node.Value}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nodeError(node, "expected_alias")
	}
	type plain Alias
	var v plain
	if err := unmarshal(&v); err != nil {
		return err
	}
	*a = Alias(v)
	return nil
}

// OutputRecord says how oh records an output itself, from what it saw the
// session do. A recorded output cannot be declared by an agent
// (workflow_outputs refuses it).
type OutputRecord string

const (
	// RecordCreated: the tickets created through the Beads gateway
	// (beads-ids).
	RecordCreated OutputRecord = "created"
	// RecordClosed: the tickets closed through the Beads gateway
	// (beads-ids).
	RecordClosed OutputRecord = "closed"
	// RecordUpdated: the tickets changed through the Beads gateway, without
	// being created or closed (priority, labels, dependencies…; beads-ids,
	// r9b).
	RecordUpdated OutputRecord = "updated"
	// RecordSpec: the specs checked by oh at a checkpoint with
	// `verify.spec` (path).
	RecordSpec OutputRecord = "spec"
	// RecordReport: the last report deposited with workflow_report
	// (report).
	RecordReport OutputRecord = "report"
	// RecordInput: the value of the input of the session with the same id,
	// recorded at launch (text), so that `next:` passes it on (the method
	// of spec to implement-spec, feature-migration).
	RecordInput OutputRecord = "input"
	// RecordWritten: the files the session changed under the output's
	// `paths:`, read each time a checkpoint passes (path; several joined by
	// commas), or with `match:` the first group of that regular expression
	// in those files (text): the postmortem of incident, the decision of
	// spike and its verdict (B1).
	RecordWritten OutputRecord = "written"
	// RecordTasks: the breakdown oh read and checked at the checkpoint with
	// `tasks:` (text, one task per line; sweep-fanout).
	RecordTasks OutputRecord = "tasks"
)

// OutputRecords lists the valid record kinds.
var OutputRecords = []OutputRecord{RecordCreated, RecordClosed, RecordUpdated, RecordSpec, RecordReport, RecordInput, RecordWritten, RecordTasks}

// Valid reports whether r is known ("" = declared by the session).
func (r OutputRecord) Valid() bool { return r == "" || slices.Contains(OutputRecords, r) }

// OutputType is the type of the outputs recorded this way.
func (r OutputRecord) OutputType() OutputType {
	switch r {
	case RecordCreated, RecordClosed, RecordUpdated:
		return OutputBeadsIDs
	case RecordSpec:
		return OutputPath
	case RecordReport:
		return OutputReport
	case RecordInput, RecordTasks:
		return OutputText
	case RecordWritten:
		return OutputPath
	}
	return ""
}

// RecordType is the type of output o recorded by oh (`record: written` with
// `match:` records text).
func (o Output) RecordType() OutputType {
	if o.Record == RecordWritten && o.Match != "" {
		return OutputText
	}
	return o.Record.OutputType()
}

// RecordedInputs returns the outputs recorded from the inputs of a session
// (`record: input`): the value given at launch, else the default.
func (s *Spec) RecordedInputs(inputs map[string]any) map[string]any {
	out := map[string]any{}
	for _, o := range s.Outputs {
		if o.Record != RecordInput {
			continue
		}
		in, ok := s.Inputs.Get(o.ID)
		if !ok {
			continue
		}
		v, given := inputs[o.ID]
		if !given || v == nil || fmt.Sprint(v) == "" {
			v = in.Default
		}
		if v != nil && fmt.Sprint(v) != "" {
			out[o.ID] = fmt.Sprint(v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RecordedOutput returns the output recorded with kind r (first one).
func (s *Spec) RecordedOutput(r OutputRecord) (Output, bool) {
	for _, o := range s.Outputs {
		if o.Record == r {
			return o, true
		}
	}
	return Output{}, false
}

// Output returns the output id.
func (s *Spec) Output(id string) (Output, bool) {
	for _, o := range s.Outputs {
		if o.ID == id {
			return o, true
		}
	}
	return Output{}, false
}

// Sources computed from a previous session (`from:`).
const (
	// SourceSessionOutput: an output of a session. In `from:`,
	// session.output(<workflow>.<output>): the output of the session the
	// launch is chained from (--parent) when it is a session of that
	// workflow, else of the last session of the project of that workflow
	// that recorded it. In `next.inputs`, session.output(<output>): the
	// session that ends.
	SourceSessionOutput = "session.output"
	// SourceSessionTrace: a summary of the trace of a session (text),
	// session.trace() or session.trace(<workflow>): the parent session, else
	// the last session of the project (of that workflow).
	SourceSessionTrace = "session.trace"
)

// SessionSources are the sources read from a previous session.
var SessionSources = []string{SourceSessionOutput, SourceSessionTrace}

var reSessionFrom = regexp.MustCompile(`^(session\.output|session\.trace)\(\s*(?:([a-z0-9]+(?:-[a-z0-9]+)*)(?:\.([a-z][a-z0-9_-]*))?)?\s*\)$`)

// SessionFrom is a parsed session source.
type SessionFrom struct {
	Source   string // SourceSessionOutput or SourceSessionTrace
	Workflow string // workflow of the session ("" = the session that ends, or any)
	Output   string // output id (session.output)
}

// ParseSessionFrom parses a session source: session.output(<workflow>.<id>),
// session.output(<id>) (Workflow empty), session.trace([<workflow>]).
func ParseSessionFrom(from string) (SessionFrom, bool) {
	m := reSessionFrom.FindStringSubmatch(from)
	if m == nil {
		return SessionFrom{}, false
	}
	f := SessionFrom{Source: m[1]}
	switch {
	case f.Source == SourceSessionTrace:
		if m[3] != "" {
			return SessionFrom{}, false
		}
		f.Workflow = m[2]
	case m[3] != "":
		f.Workflow, f.Output = m[2], m[3]
	case m[2] != "":
		f.Output = m[2]
	default:
		return SessionFrom{}, false
	}
	return f, true
}

// IsSessionSource reports whether a `from:` value reads a previous session
// (valid or not).
func IsSessionSource(from string) bool { return strings.HasPrefix(strings.TrimSpace(from), "session.") }

// OutputFits reports whether an output of type t may fill an input of type
// in (next.inputs, session.output).
func OutputFits(t OutputType, in InputType) bool {
	switch t {
	case OutputBeadsIDs:
		return in == InputBeadsIDs || in == InputBeadsID
	case OutputBranch:
		return in == InputBranch || in == InputString
	case OutputPath:
		return in == InputPath || in == InputString || in == InputText
	case OutputReport, OutputMergeRequest:
		return in == InputString || in == InputText
	case OutputText:
		return in == InputString || in == InputText || in == InputEnum
	}
	return false
}
