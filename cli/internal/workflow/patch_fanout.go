package workflow

import (
	"fmt"
	"slices"
)

// Patch rules of the fan-out (R6, schema_fanout.go): the fan-out is a
// security field. A layer other than the hub may only lower `parallel`;
// changing the input, the tickets, the child workflow, the merge
// checkpoint, the merger or the delegated checkpoints (CP-R3) is a
// loosening, and the parent value is kept. Giving or changing the scope
// (`fanout:`) of an inherited checkpoint is a loosening too: it would drop
// a checkpoint from a kind of session.
func (p *patcher) fanout() {
	if p.blocked["fanout"] || !p.doc.Has("fanout") {
		return
	}
	v, old := p.doc.Spec.Fanout, p.dst.Fanout
	if p.widens() {
		p.dst.Fanout = p.hubFanout(v, old)
		return
	}
	if old == nil {
		if v != nil {
			p.loosening("fanout", v.String(), "none")
		}
		return
	}
	if v == nil {
		p.loosening("fanout", "null", old.String())
		return
	}
	c := old.clone()
	str := func(path, nv, ov string) {
		if p.has(path) && nv != ov {
			p.loosening(path, nv, ov)
		}
	}
	str("fanout.input", v.Input, old.Input)
	str("fanout.tickets", v.Tickets, old.Tickets)
	str("fanout.child", v.Child, old.Child)
	str("fanout.merge", v.Merge, old.Merge)
	str("fanout.merger", v.Merger, old.Merger)
	if p.has("fanout.delegates") && !slices.Equal(v.Delegates, old.Delegates) {
		p.loosening("fanout.delegates", fmt.Sprint(v.Delegates), fmt.Sprint(old.Delegates))
	}
	if p.has("fanout.parallel") {
		if v.Parallel < 1 || v.EffectiveParallel() > old.EffectiveParallel() {
			p.loosening("fanout.parallel", v.Parallel, old.EffectiveParallel())
		} else {
			c.Parallel = v.Parallel
		}
	}
	p.dst.Fanout = c
}

// hubFanout merges a hub patch: every written field replaces the parent's.
func (p *patcher) hubFanout(v, old *Fanout) *Fanout {
	if v == nil {
		return nil
	}
	if old == nil {
		return v.clone()
	}
	c := old.clone()
	if p.has("fanout.input") {
		c.Input = v.Input
	}
	if p.has("fanout.tickets") {
		c.Tickets = v.Tickets
	}
	if p.has("fanout.child") {
		c.Child = v.Child
	}
	if p.has("fanout.parallel") {
		c.Parallel = v.Parallel
	}
	if p.has("fanout.merge") {
		c.Merge = v.Merge
	}
	if p.has("fanout.delegates") {
		c.Delegates = cloneSlice(v.Delegates)
	}
	if p.has("fanout.merger") {
		c.Merger = v.Merger
	}
	return c
}

// checkpointFanout merges `fanout:` of a checkpoint patch.
func (p *patcher) checkpointFanout(base string, old *CheckpointSpec, v CheckpointSpec) {
	if !p.has(base+".fanout") || v.Fanout == old.Fanout {
		return
	}
	if p.widens() {
		old.Fanout = v.Fanout
		return
	}
	p.loosening(base+".fanout", fanoutScopeText(v.Fanout), fanoutScopeText(old.Fanout))
}

func fanoutScopeText(f FanoutScope) string {
	if f == "" {
		return "all"
	}
	return string(f)
}
