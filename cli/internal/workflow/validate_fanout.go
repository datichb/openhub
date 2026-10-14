package workflow

import (
	"fmt"
	"slices"
	"strings"
)

// Validation of the fan-out (R6, schema_fanout.go). Every rule of the
// control substitution (CP-R3) is checked here, on the resolved workflow:
//   - the fan-out input is a bool input, the tickets input a beads-ids one;
//   - the child workflow exists, is not itself a fan-out and holds every
//     delegated checkpoint;
//   - the merge checkpoint exists only in fan-out sessions, is mandatory,
//     unlocks the merge and is verified by oh on the integration branch;
//   - `delegates` is declared by the hub layer only, never by a team or
//     project layer nor by a draft (`fanout_delegates_layer`);
//   - the merge operation is unlocked by the merge checkpoint only, and
//     `verify.on: integration` is only used there;
//   - each input given to the children is a value input of the parent,
//     neither the fan-out nor the tickets input, that the child workflow
//     declares with the same type (and every value of an enum);
//   - the fix workflow exists, is not a fan-out, takes the instruction in
//     a text input and has a mandatory checkpoint that unlocks its commit
//     (validated by the user: a fix round is never delegated).
func (v *validator) fanout() {
	s := v.s
	f := s.Fanout
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		base := "checkpoints." + k
		if cp.Fanout != "" {
			v.enum(base+".fanout", string(cp.Fanout), cp.Fanout.Valid(), string(FanoutOnly), string(FanoutNever))
			if f == nil {
				v.err("checkpoint_fanout_without_fanout", base+".fanout", k)
			}
		}
		merge := f != nil && f.Merge == k
		for i, op := range cp.Unlocks {
			if op == UnlockMerge && !merge {
				v.err("unlock_merge_fanout_only", fmt.Sprintf("%s.unlocks[%d]", base, i), k)
			}
		}
		if cp.Verify != nil && cp.Verify.On != "" {
			v.enum(base+".verify.on", string(cp.Verify.On), cp.Verify.On == VerifyOnIntegration, string(VerifyOnIntegration))
			if !merge {
				v.err("verify_on_integration_merge_only", base+".verify.on", k)
			}
		}
	}
	if f == nil {
		return
	}
	v.fanoutInputs(f)
	if f.Parallel < 0 || f.Parallel > MaxFanoutParallel {
		v.err("fanout_parallel", "fanout.parallel", f.Parallel, MaxFanoutParallel)
	}
	v.fanoutMerge(f)
	v.fanoutChild(f)
	v.fanoutChildInputs(f)
	v.fanoutFix(f)
	v.fanoutDelegatesLayer(f)
	if f.Merger != "" && !v.knownWorkflow(f.Merger) {
		v.err("fanout_unknown_workflow", "fanout.merger", f.Merger)
	}
}

func (v *validator) fanoutInputs(f *Fanout) {
	s := v.s
	if f.Input == "" {
		v.err("field_required", "fanout.input", "input")
	} else if in, ok := s.Inputs.Get(f.Input); !ok || in.Type != InputBool {
		v.err("fanout_input", "fanout.input", f.Input)
	}
	if f.Tickets == "" {
		v.err("field_required", "fanout.tickets", "tickets")
	} else if in, ok := s.Inputs.Get(f.Tickets); !ok || in.Type != InputBeadsIDs {
		v.err("fanout_tickets", "fanout.tickets", f.Tickets)
	}
}

// fanoutMerge checks the merge checkpoint: it is what replaces the
// delegated checkpoints, so each missing guarantee is an error.
func (v *validator) fanoutMerge(f *Fanout) {
	if f.Merge == "" {
		v.err("field_required", "fanout.merge", "merge")
		return
	}
	cp, ok := v.s.Checkpoints.Get(f.Merge)
	if !ok || cp.Disabled {
		v.err("fanout_merge_unknown", "fanout.merge", f.Merge)
		return
	}
	base := "checkpoints." + f.Merge
	if cp.Fanout != FanoutOnly {
		v.err("fanout_merge_scope", base+".fanout", f.Merge)
	}
	if !cp.IsMandatory() {
		v.err("fanout_merge_mandatory", base+".mandatory", f.Merge)
	}
	if !containsOp(cp.Unlocks, UnlockMerge) {
		v.err("fanout_merge_unlocks", base+".unlocks", f.Merge)
	}
	vs := cp.Verify
	switch {
	case vs == nil || (vs.From == "" && len(vs.Cmd) == 0):
		v.err("fanout_merge_verify", base+".verify", f.Merge)
	case vs.On != VerifyOnIntegration:
		v.err("fanout_merge_verify_on", base+".verify.on", f.Merge)
	case vs.ExpectOrPass() != VerifyPass:
		v.err("fanout_merge_verify_expect", base+".verify.expect", f.Merge)
	}
}

func (v *validator) fanoutChild(f *Fanout) {
	if f.Child == "" {
		v.err("field_required", "fanout.child", "child")
		return
	}
	if f.Child == v.s.ID {
		v.err("fanout_child_self", "fanout.child", f.Child)
		return
	}
	if !v.knownWorkflow(f.Child) {
		v.err("fanout_unknown_workflow", "fanout.child", f.Child)
		return
	}
	child := v.specOf(f.Child)
	if child == nil {
		return // the environment cannot tell
	}
	if child.Fanout != nil {
		v.err("fanout_child_nested", "fanout.child", f.Child)
	}
	for i, d := range f.Delegates {
		at := fmt.Sprintf("fanout.delegates[%d]", i)
		cp, ok := child.Checkpoints.Get(d)
		if !ok || cp.Disabled {
			v.err("fanout_delegate_unknown", at, d, f.Child, strings.Join(child.Checkpoints.Keys(), ", "))
		}
	}
}

// fanoutDelegatesLayer refuses a substitution of control declared by
// another layer than the hub (CP-R3): every document that wrote
// `fanout.delegates` must be a published hub document.
func (v *validator) fanoutDelegatesLayer(f *Fanout) {
	if len(f.Delegates) == 0 {
		return
	}
	seen := map[string]bool{}
	for i, d := range f.Delegates {
		if seen[d] {
			v.err("duplicate_entry", fmt.Sprintf("fanout.delegates[%d]", i), d)
		}
		seen[d] = true
	}
	at := []string{"fanout.delegates"}
	for p := range v.r.Origins {
		if strings.HasPrefix(p, "fanout.delegates[") {
			at = append(at, p)
		}
	}
	slices.Sort(at)
	for _, p := range at {
		o, ok := v.r.Origins.Of(p)
		if !ok {
			// No document recorded (in-memory): the workflow itself.
			o = Origin{Layer: v.r.Ref.Layer, ID: v.r.Ref.ID}
		}
		if o.Layer != LayerHub || o.Draft {
			v.err("fanout_delegates_layer", "fanout.delegates", o.Ref().String())
			return
		}
	}
}

// fanoutChildInputs checks the inputs given to the children
// (`fanout.inputs`).
func (v *validator) fanoutChildInputs(f *Fanout) {
	var child *Spec
	if f.Child != "" && f.Child != v.s.ID {
		child = v.specOf(f.Child)
	}
	seen := map[string]bool{}
	for i, id := range f.Inputs {
		at := fmt.Sprintf("fanout.inputs[%d]", i)
		if seen[id] {
			v.err("duplicate_entry", at, id)
			continue
		}
		seen[id] = true
		in, ok := v.s.Inputs.Get(id)
		if !ok || id == f.Input || id == f.Tickets || !slices.Contains(FanoutInputTypes, in.Type) {
			v.err("fanout_inputs_parent", at, id)
			continue
		}
		if child == nil {
			continue // the environment cannot tell
		}
		cin, ok := child.Inputs.Get(id)
		if !ok {
			v.err("fanout_inputs_child", at, id, f.Child)
			continue
		}
		if cin.Type != in.Type {
			v.err("fanout_inputs_type", at, id, string(in.Type), f.Child, string(cin.Type))
			continue
		}
		for _, val := range in.Values {
			if !slices.Contains(cin.Values, val) {
				v.err("fanout_inputs_value", at, id, val, f.Child)
			}
		}
	}
}

// fanoutFix checks the fix round (`fanout.fix`).
func (v *validator) fanoutFix(f *Fanout) {
	fx := f.Fix
	if fx == nil {
		return
	}
	if fx.Workflow == "" {
		v.err("field_required", "fanout.fix.workflow", "workflow")
		return
	}
	if fx.Workflow == v.s.ID || !v.knownWorkflow(fx.Workflow) {
		v.err("fanout_unknown_workflow", "fanout.fix.workflow", fx.Workflow)
		return
	}
	if fx.Input == "" {
		v.err("field_required", "fanout.fix.input", "input")
	}
	w := v.specOf(fx.Workflow)
	if w == nil {
		return // the environment cannot tell
	}
	if w.Fanout != nil {
		v.err("fanout_child_nested", "fanout.fix.workflow", fx.Workflow)
	}
	if in, ok := w.Inputs.Get(fx.Input); fx.Input != "" && (!ok || (in.Type != InputText && in.Type != InputString)) {
		v.err("fanout_fix_input", "fanout.fix.input", fx.Input, fx.Workflow)
	}
	commit := false
	for _, k := range w.Checkpoints.Keys() {
		cp, _ := w.Checkpoints.Get(k)
		commit = commit || (!cp.Disabled && cp.IsMandatory() && containsOp(cp.Unlocks, UnlockCommit))
	}
	if !commit {
		v.err("fanout_fix_commit", "fanout.fix.workflow", fx.Workflow)
	}
}
