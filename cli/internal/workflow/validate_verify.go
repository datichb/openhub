package workflow

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"time"
)

// Validation of the R3 checkpoint fields (verify, when) and of the pause
// budget (CP-R2).

// Pause budget (CP-R2, report 2026-10 §9.4): in semi-auto, a workflow pauses
// at most for BudgetDecisionPauses human decisions (scope, understanding,
// arbitration) plus one per checkpoint that unlocks an irreversible action
// (commit, close, create, apply, push). A conditional checkpoint, or one
// with `when:`, counts as a pause (the worst case).
const BudgetDecisionPauses = 2

// PauseCount is the pause count of a workflow in its budget mode.
type PauseCount struct {
	// Mode is semi-auto, or the default mode when semi-auto is not allowed.
	Mode string
	// Pauses are the checkpoints that may pause in Mode.
	Pauses []string
	// Irreversible are the pauses that unlock an operation.
	Irreversible []string
	// Budget is BudgetDecisionPauses + len(Irreversible).
	Budget int
	// MethodPauses are the pauses of the checkpoints of a method (R7),
	// by method: a session of that method may pause MethodPauseAllowance
	// more times (report 2026-10 §9.2: TDD adds cp-seams).
	MethodPauses map[Method][]string
}

// MethodPauseAllowance is the decision a method may add to the budget: the
// method is chosen at launch, for that session only.
const MethodPauseAllowance = 1

// Exceeded reports whether the workflow pauses more than its budget, in a
// session without method or in a session of each method.
func (c PauseCount) Exceeded() bool {
	base := len(c.Pauses)
	for _, ps := range c.MethodPauses {
		base -= len(ps)
	}
	if base > c.Budget {
		return true
	}
	for _, ps := range c.MethodPauses {
		if base+len(ps) > c.Budget+MethodPauseAllowance {
			return true
		}
	}
	return false
}

// PauseBudget counts the pauses of the workflow against its budget.
func (s *Spec) PauseBudget() PauseCount {
	mode := ModeSemiAuto
	if !slices.Contains(s.AllowedModes(), mode) {
		mode = s.DefaultMode()
	}
	c := PauseCount{Mode: mode}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		// In a read-only workflow, a verified checkpoint asks (bundle).
		asked := cp.Verify != nil && s.Risk == RiskRead && cp.Behavior(mode) != BehaviorSkip
		if cp.Disabled || (!cp.MayPause(mode) && !asked) {
			continue
		}
		c.Pauses = append(c.Pauses, k)
		if cp.Method != "" {
			if c.MethodPauses == nil {
				c.MethodPauses = map[Method][]string{}
			}
			c.MethodPauses[cp.Method] = append(c.MethodPauses[cp.Method], k)
		}
		if len(cp.Unlocks) > 0 {
			c.Irreversible = append(c.Irreversible, k)
		}
	}
	c.Budget = BudgetDecisionPauses + len(c.Irreversible)
	return c
}

// MayPause reports whether the checkpoint may wait for the user in mode:
// pause (a mandatory checkpoint is never skipped), conditional, or a
// behavior that `when:` can turn into a pause.
func (c CheckpointSpec) MayPause(mode string) bool {
	b := c.Behavior(mode)
	if b == BehaviorSkip && c.IsMandatory() {
		b = BehaviorPause
	}
	switch b {
	case BehaviorPause, BehaviorConditional:
		return true
	case BehaviorAuto:
		return !c.When.IsZero()
	}
	return false
}

func (v *validator) checkpointsVerify() {
	s := v.s
	agents := map[string]bool{s.EntryAgent(): true}
	for _, id := range s.Agents.Keys() {
		agents[id] = true
	}
	outputs := map[string]bool{}
	for _, o := range s.Outputs {
		outputs[o.ID] = true
		outputs[string(o.Type)] = true
	}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		base := "checkpoints." + k
		if cp.Verify != nil {
			v.verify(base+".verify", cp.Verify)
			v.verifyTests(base+".verify", k, cp)
		}
		if cp.When != nil {
			v.when(base+".when", k, cp, agents, outputs)
		}
	}
	if c := s.PauseBudget(); c.Exceeded() {
		v.warn("pause_budget_exceeded", "checkpoints", len(c.Pauses), c.Mode, c.Budget, strings.Join(c.Pauses, ", "))
	}
}

func (v *validator) verify(base string, vs *VerifySpec) {
	if vs.From == "" && len(vs.Cmd) == 0 {
		v.err("verify_empty", base)
	}
	if vs.From != "" {
		v.enum(base+".from", string(vs.From), vs.From == VerifyFromStack, string(VerifyFromStack))
	} else if len(vs.Kinds) > 0 {
		v.warn("field_ignored", base+".kinds", "kinds", "cmd")
	}
	names := make([]string, len(VerifyKinds))
	for i, k := range VerifyKinds {
		names[i] = string(k)
	}
	for i, k := range vs.Kinds {
		v.enum(fmt.Sprintf("%s.kinds[%d]", base, i), string(k), k.Valid(), names...)
	}
	if vs.Expect != "" {
		v.enum(base+".expect", string(vs.Expect), vs.Expect == VerifyPass || vs.Expect == VerifyFail, string(VerifyPass), string(VerifyFail))
	}
	for i, c := range vs.Cmd {
		at := fmt.Sprintf("%s.cmd[%d]", base, i)
		switch {
		case strings.TrimSpace(c) == "":
			v.err("field_required", at, at)
		case strings.Contains(c, "{{"):
			v.err("verify_cmd_template", at, c)
		}
	}
	if vs.In != "" {
		clean := path.Clean(vs.In)
		if path.IsAbs(vs.In) || clean == ".." || strings.HasPrefix(clean, "../") {
			v.err("verify_in_invalid", base+".in", vs.In)
		}
	}
	if vs.Timeout != "" {
		d, err := time.ParseDuration(vs.Timeout)
		if err != nil || d <= 0 || d > MaxVerifyTimeout {
			v.err("verify_timeout_invalid", base+".timeout", vs.Timeout, MaxVerifyTimeout.String())
		}
	}
	for i, p := range vs.MustTouch {
		if strings.TrimSpace(p) == "" {
			at := fmt.Sprintf("%s.must_touch[%d]", base, i)
			v.err("field_required", at, at)
		}
	}
}

func (v *validator) when(base, id string, cp CheckpointSpec, agents, outputs map[string]bool) {
	w := cp.When
	if w.IsZero() {
		v.err("when_empty", base)
		return
	}
	if cp.IsMandatory() {
		// A mandatory checkpoint always pauses: when: cannot remove it.
		v.warn("when_mandatory", base, id)
	}
	for i, a := range w.Entry {
		if !agents[a] {
			v.warn("when_unknown_agent", fmt.Sprintf("%s.entry[%d]", base, i), a)
		}
	}
	for i, o := range w.Outputs {
		if len(outputs) > 0 && !outputs[o] {
			v.warn("when_unknown_output", fmt.Sprintf("%s.outputs[%d]", base, i), o)
		}
	}
	for _, f := range []struct {
		name string
		list []string
	}{{"labels", w.Labels}, {"paths", w.Paths}} {
		for i, s := range f.list {
			if strings.TrimSpace(s) == "" {
				at := fmt.Sprintf("%s.%s[%d]", base, f.name, i)
				v.err("field_required", at, at)
			}
		}
	}
}
