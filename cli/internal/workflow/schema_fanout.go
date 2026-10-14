package workflow

import (
	"fmt"
	"slices"
	"strings"
)

// Fan-out of a workflow (R6, E3, CP-R3), experimental: a session launched
// with the fan-out input runs one child session per ticket, each in its own
// worktree and branch; oh merges each child into an integration branch,
// asks the merger to resolve a conflict it cannot, and runs the
// verification of the merge checkpoint of the parent on that branch.
//
//	inputs:
//	  fanout: { type: bool, default: false }
//	fanout:
//	  input: fanout          # bool input that makes the session a fan-out
//	  tickets: tickets       # beads-ids input: one child per ticket
//	  child: ticket          # workflow of each child session
//	  parallel: 3            # children at the same time (1 to 3)
//	  merge: cp-merge        # mandatory checkpoint of the parent, verified by oh on the integration branch
//	  delegates: [cp-2]      # checkpoints of the child passed automatically (CP-R3, hub layer only)
//	  merger: merge-conflicts  # workflow launched by oh on a conflict ("" = none)
//	  inputs: [method, instructions]  # inputs of the parent given to each child (same name in the child)
//	checkpoints:
//	  cp-2:     { fanout: never, … }   # the children pass it
//	  cp-merge:
//	    fanout: only
//	    mandatory: true
//	    unlocks: [merge]
//	    verify: { from: stack, expect: pass, on: integration }
//
// Control substitution (CP-R3): the checkpoints listed in `delegates` stop
// pausing in the children (their verification still runs) because the
// parent carries a mandatory checkpoint whose verification oh runs on the
// integration branch. Only the hub layer declares it: a team or project
// layer, or a draft, never does (`fanout_delegates_layer`), and a patch
// may only harden the fan-out (lower `parallel`). At run time the daemon
// checks the substitution again against the bundle of the parent
// (services/checkpoint, fanout.go). The sessions launched without the
// fan-out input keep exactly the checkpoints of before (ForFanout).

// Fanout is the fan-out of a workflow.
type Fanout struct {
	// Input is the bool input that makes a session a fan-out.
	Input string `yaml:"input"`
	// Tickets is the beads-ids input whose tickets become the children.
	Tickets string `yaml:"tickets"`
	// Child is the workflow of each child session (one ticket each).
	Child string `yaml:"child"`
	// Parallel is the number of children at the same time (default 1, at
	// most MaxFanoutParallel); the configuration of oh may lower it.
	Parallel int `yaml:"parallel,omitempty"`
	// Merge is the checkpoint of the parent that validates the merges.
	Merge string `yaml:"merge"`
	// Delegates are checkpoints of the child workflow passed automatically
	// in the children (CP-R3).
	Delegates []string `yaml:"delegates,omitempty"`
	// Merger is the workflow oh launches in the integration worktree when
	// it cannot merge a child by itself ("" = the child is left unmerged).
	Merger string `yaml:"merger,omitempty"`
	// Inputs are inputs of the parent given, when the session gets them,
	// to each child under the same name (method, instructions); nothing
	// else of the parent reaches the children.
	Inputs []string `yaml:"inputs,omitempty"`
}

// FanoutInputTypes are the types of the inputs a parent may give to its
// children: a value, never a list of tickets (the child gets its own).
var FanoutInputTypes = []InputType{InputString, InputText, InputBool, InputInt, InputEnum, InputPath, InputBranch}

// MaxFanoutParallel caps the children of a fan-out run at the same time.
const MaxFanoutParallel = 3

// FanoutScope is where a checkpoint exists, given the fan-out of the
// session.
type FanoutScope string

const (
	// FanoutOnly: in the fan-out sessions only.
	FanoutOnly FanoutScope = "only"
	// FanoutNever: in the other sessions only (the children pass it).
	FanoutNever FanoutScope = "never"
)

// Valid reports whether f is known ("" = every session).
func (f FanoutScope) Valid() bool { return f == "" || f == FanoutOnly || f == FanoutNever }

// VerifyOn is where a verification runs, when not in the session location.
type VerifyOn string

// VerifyOnIntegration runs the verification on the integration branch of a
// fan-out (its worktree).
const VerifyOnIntegration VerifyOn = "integration"

// EffectiveParallel is Parallel bounded to [1, MaxFanoutParallel].
func (f *Fanout) EffectiveParallel() int {
	if f == nil || f.Parallel < 1 {
		return 1
	}
	return min(f.Parallel, MaxFanoutParallel)
}

func (f *Fanout) clone() *Fanout {
	if f == nil {
		return nil
	}
	c := *f
	c.Delegates = cloneSlice(f.Delegates)
	c.Inputs = cloneSlice(f.Inputs)
	return &c
}

// String is the fan-out as shown in a diagnostic.
func (f *Fanout) String() string {
	if f == nil {
		return "none"
	}
	return fmt.Sprintf("{input: %s, tickets: %s, child: %s, parallel: %d, merge: %s, delegates: [%s], merger: %s, inputs: [%s]}",
		f.Input, f.Tickets, f.Child, f.EffectiveParallel(), f.Merge, strings.Join(f.Delegates, ","), f.Merger, strings.Join(f.Inputs, ","))
}

// FanoutOf reports whether a session with these inputs is a fan-out: the
// workflow declares one and its input is true (given, else its default).
func (s *Spec) FanoutOf(inputs map[string]any) bool {
	if s == nil || s.Fanout == nil || s.Fanout.Input == "" {
		return false
	}
	in, ok := s.Inputs.Get(s.Fanout.Input)
	if !ok || in.Type != InputBool {
		return false
	}
	v, given := inputs[s.Fanout.Input]
	if !given {
		v = in.Default
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true") || b == "1" || strings.EqualFold(b, "yes")
	}
	return false
}

// ForFanout returns s as a session sees it, fan-out or not: the
// checkpoints of the other kind of session are dropped, and in a fan-out
// session the agents gated on a dropped checkpoint (the developer, after
// cp-1) are removed: the children do their work. A session that is not
// a fan-out loses the fan-out block (same bundle as before R6). s itself
// is never changed.
func (s *Spec) ForFanout(on bool) *Spec {
	if s == nil {
		return nil
	}
	var drop []string
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		if (on && cp.Fanout == FanoutNever) || (!on && cp.Fanout == FanoutOnly) {
			drop = append(drop, k)
		}
	}
	if len(drop) == 0 && (on || s.Fanout == nil) {
		return s
	}
	c := s.Clone()
	for _, k := range drop {
		c.Checkpoints.Delete(k)
	}
	if !on {
		c.Fanout = nil
		return c
	}
	for _, id := range c.Agents.Keys() {
		if a, _ := c.Agents.Get(id); a.After != "" && slices.Contains(drop, a.After) {
			c.Agents.Delete(id)
		}
	}
	return c
}

// FanoutMerge returns the merge checkpoint of the fan-out of s.
func (s *Spec) FanoutMerge() (CheckpointSpec, bool) {
	if s == nil || s.Fanout == nil || s.Fanout.Merge == "" {
		return CheckpointSpec{}, false
	}
	return s.Checkpoints.Get(s.Fanout.Merge)
}

// SubstitutesControl reports whether the merge checkpoint of the fan-out
// replaces the delegated checkpoints of the children (CP-R3): mandatory,
// only in fan-out sessions, verified by oh on the integration branch with
// an expected success, and unlocking the merge.
func (s *Spec) SubstitutesControl() bool {
	cp, ok := s.FanoutMerge()
	if !ok || cp.Disabled || !cp.IsMandatory() || cp.Fanout != FanoutOnly {
		return false
	}
	v := cp.Verify
	if v == nil || v.On != VerifyOnIntegration || v.ExpectOrPass() != VerifyPass || (v.From == "" && len(v.Cmd) == 0) {
		return false
	}
	return containsOp(cp.Unlocks, UnlockMerge)
}

// ChildInputs returns the inputs of the parent given to each child: the
// inputs listed in `fanout.inputs` that the session got (given, not
// defaults: the child keeps its own).
func (s *Spec) ChildInputs(given map[string]any) map[string]string {
	if s == nil || s.Fanout == nil {
		return nil
	}
	var out map[string]string
	for _, id := range s.Fanout.Inputs {
		v, ok := given[id]
		if !ok || v == nil {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[id] = fmt.Sprint(v)
	}
	return out
}
