package workflow

import "slices"

// Order of the checkpoints and of the writes (r5-fixes):
//
//	agents:
//	  planner: { role: workflow, writes: ["docs/wiki/specs/**"], writes_after: cp-understanding }
//	checkpoints:
//	  cp-converge: { after: cp-understanding, … }
//
// `writes_after` opens the `writes:` paths of an agent only once a
// checkpoint is passed; `after` on a checkpoint refuses it until an earlier
// one is passed. Both name a checkpoint of the workflow that exists in every
// session (not one of a method only); the `after` of a checkpoint names one
// declared before it, so that the order has no cycle. A layer may add them
// or move them to a later checkpoint, never remove them (patcher.order).

// order checks `writes_after` and the `after` of the checkpoints.
func (v *validator) order() {
	s := v.s
	keys := s.Checkpoints.Keys()
	known := func(id string) (CheckpointSpec, int, bool) {
		cp, ok := s.Checkpoints.Get(id)
		if !ok || cp.Disabled {
			return cp, -1, false
		}
		return cp, slices.Index(keys, id), true
	}
	for _, id := range s.Agents.Keys() {
		a, _ := s.Agents.Get(id)
		if a.WritesAfter == "" || a.Role == RoleDisabled {
			continue
		}
		at := "agents." + id + ".writes_after"
		if len(a.Writes) == 0 {
			v.err("writes_after_without_writes", at, id)
		}
		if cp, _, ok := known(a.WritesAfter); !ok {
			v.err("writes_after_unknown", at, a.WritesAfter)
		} else if cp.Method != "" {
			v.err("order_method_after", at, id, a.WritesAfter)
		}
	}
	for i, k := range keys {
		cp, _ := s.Checkpoints.Get(k)
		if cp.After == "" || cp.Disabled {
			continue
		}
		at := "checkpoints." + k + ".after"
		target, j, ok := known(cp.After)
		switch {
		case !ok:
			v.err("checkpoint_after_unknown", at, cp.After)
		case j >= i:
			v.err("checkpoint_after_order", at, k, cp.After)
		case target.Method != "" && target.Method != cp.Method:
			v.err("order_method_after", at, k, cp.After)
		}
	}
}

// orderAfter keeps an inherited order (`writes_after`, the `after` of a
// checkpoint): it cannot be removed, nor moved to an earlier checkpoint;
// moving it to a later checkpoint hardens it.
func (p *patcher) orderAfter(path string, old *string, after string) {
	if *old == "" || after == *old {
		*old = after
		return
	}
	cps := p.dst.Checkpoints.Keys()
	if from, to := slices.Index(cps, *old), slices.Index(cps, after); from >= 0 && to > from {
		*old = after
		return
	}
	shown := after
	if shown == "" {
		shown = `""`
	}
	p.loosening(path, shown, *old)
}
