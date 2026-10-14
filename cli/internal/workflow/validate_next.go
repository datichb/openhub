package workflow

import (
	"fmt"
	"sort"
	"strings"
)

// Validation of the composition fields (R5, schema_next.go): `next:`,
// `aliases:`, `outputs[].record` and the session sources of `from:`.

// composition checks `next:` and `aliases:`.
func (v *validator) composition() {
	v.next()
	v.aliases()
}

// specOf returns the spec of another workflow when the environment can
// give it (nil otherwise: the checks that need it are skipped).
func (v *validator) specOf(id string) *Spec {
	specs, ok := v.env.Workflows.(WorkflowSpecs)
	if !ok {
		return nil
	}
	sp, _ := specs.WorkflowSpec(id)
	return sp
}

// knownWorkflow reports whether id is a workflow (or an alias) of the
// catalogue; true when the environment cannot tell.
func (v *validator) knownWorkflow(id string) bool {
	if v.env.Workflows == nil || v.env.Workflows.HasWorkflow(id) {
		return true
	}
	return v.specOf(id) != nil
}

func (v *validator) next() {
	s := v.s
	seen := map[string]bool{}
	for i, n := range s.Next {
		base := fmt.Sprintf("next[%d]", i)
		switch {
		case n.Workflow == "":
			v.err("field_required", base+".workflow", "workflow")
			continue
		case n.Workflow == s.ID:
			v.err("next_self", base+".workflow", n.Workflow)
			continue
		case seen[n.Workflow]:
			v.err("duplicate_entry", base+".workflow", n.Workflow)
		case !v.knownWorkflow(n.Workflow):
			v.err("next_unknown_workflow", base+".workflow", n.Workflow)
			continue
		}
		seen[n.Workflow] = true
		if n.When != nil {
			for j, o := range n.When.Outputs {
				if _, ok := s.Output(o); !ok {
					v.err("next_unknown_output", fmt.Sprintf("%s.when.outputs[%d]", base, j), o, outputIDs(s))
				}
			}
		}
		target := v.specOf(n.Workflow)
		keys := make([]string, 0, len(n.Inputs))
		for k := range n.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			at := base + ".inputs." + k
			f, ok := ParseSessionFrom(n.Inputs[k])
			if !ok || f.Source != SourceSessionOutput || f.Workflow != "" {
				v.err("next_input_invalid", at, n.Inputs[k])
				continue
			}
			out, ok := s.Output(f.Output)
			if !ok {
				v.err("next_unknown_output", at, f.Output, outputIDs(s))
				continue
			}
			if target == nil {
				continue
			}
			in, ok := target.Inputs.Get(k)
			if !ok {
				v.err("next_unknown_input", at, k, n.Workflow)
				continue
			}
			if !OutputFits(out.Type, in.Type) {
				v.err("next_input_type", at, f.Output, string(out.Type), k, string(in.Type))
			}
		}
	}
}

func (v *validator) aliases() {
	s := v.s
	seen := map[string]bool{}
	for i, a := range s.Aliases {
		at := fmt.Sprintf("aliases[%d]", i)
		switch {
		case !reKebabID.MatchString(a.ID):
			v.err("id_invalid", at+".id", a.ID)
		case a.ID == s.ID:
			v.err("alias_self", at+".id", a.ID)
		case seen[a.ID]:
			v.err("duplicate_entry", at+".id", a.ID)
		case v.env.Workflows != nil && v.env.Workflows.HasWorkflow(a.ID):
			// A document with that id wins: the alias never applies.
			v.warn("alias_shadowed", at+".id", a.ID)
		}
		seen[a.ID] = true
		if a.With != "" {
			if _, ok := s.Inputs.Get(a.With); !ok {
				v.err("alias_with_unknown_input", at+".with", a.With)
			}
		}
	}
}

// outputRecord checks the record kind of an output and its type.
func (v *validator) outputRecord(base string, o Output) {
	if o.Record == "" {
		return
	}
	if !o.Record.Valid() {
		names := make([]string, len(OutputRecords))
		for i, r := range OutputRecords {
			names[i] = string(r)
		}
		v.err("enum_invalid", base+".record", string(o.Record), strings.Join(names, ", "))
		return
	}
	if want := o.Record.OutputType(); o.Type != "" && o.Type != want {
		v.err("output_record_type", base+".record", string(o.Record), string(want), string(o.Type))
	}
	if o.Record == RecordSpec && !v.s.hasSpecCheck() {
		v.warn("output_record_spec_unchecked", base+".record")
	}
	if o.Record == RecordTasks && v.s.TasksCheckpoint() == "" {
		v.err("output_record_tasks_unchecked", base+".record")
	}
	if o.Record == RecordInput {
		if _, ok := v.s.Inputs.Get(o.ID); !ok {
			v.err("output_record_input_unknown", base+".record", o.ID)
		}
	}
}

// hasSpecCheck reports whether a checkpoint checks the specs (verify.spec).
func (s *Spec) hasSpecCheck() bool {
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		if cp.Verify != nil && cp.Verify.Spec != nil {
			return true
		}
	}
	return false
}

// sessionFrom checks a `from:` reading a previous session.
func (v *validator) sessionFrom(at string, in Input) {
	f, ok := ParseSessionFrom(in.From)
	if !ok {
		v.err("input_from_invalid", at, in.From)
		return
	}
	switch f.Source {
	case SourceSessionTrace:
		if in.Type != InputText {
			v.err("input_from_type", at, in.From, string(in.Type), string(InputText))
		}
		if f.Workflow != "" && !v.knownWorkflow(f.Workflow) {
			v.err("input_from_unknown_workflow", at, f.Workflow)
		}
	case SourceSessionOutput:
		if f.Workflow == "" {
			v.err("input_from_session_workflow", at, in.From)
			return
		}
		if !v.knownWorkflow(f.Workflow) {
			v.err("input_from_unknown_workflow", at, f.Workflow)
			return
		}
		sp := v.specOf(f.Workflow)
		if sp == nil {
			return
		}
		out, ok := sp.Output(f.Output)
		if !ok {
			v.err("input_from_unknown_output", at, f.Output, f.Workflow, outputIDs(sp))
			return
		}
		if !OutputFits(out.Type, in.Type) {
			v.err("input_from_type", at, in.From, string(in.Type), string(out.Type))
		}
	}
}

func outputIDs(s *Spec) string {
	ids := make([]string, 0, len(s.Outputs))
	for _, o := range s.Outputs {
		ids = append(ids, o.ID)
	}
	if len(ids) == 0 {
		return "—"
	}
	return strings.Join(ids, ", ")
}
