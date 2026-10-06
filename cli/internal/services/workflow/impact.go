package workflow

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Impact summary of a change (O12): shown before publishing, and used to
// refuse a draft launch that would loosen the published security.

// ImpactLevel says whether a change widens what the workflow may do.
type ImpactLevel string

const (
	ImpactInfo  ImpactLevel = "info"  // neutral change
	ImpactWiden ImpactLevel = "widen" // more rights, less control
)

// ImpactItem is one change. Message is localized from
// "teamstate.workflow.impact.<code>".
type ImpactItem struct {
	Code    string      `json:"code"`
	Level   ImpactLevel `json:"level"`
	Path    string      `json:"path,omitempty"`
	Message string      `json:"message"`
}

// ImpactReport compares two resolved versions of a workflow.
type ImpactReport struct {
	New   bool         `json:"new,omitempty"` // no previous version
	Risk  wf.Risk      `json:"risk,omitempty"`
	Items []ImpactItem `json:"items"`
	// NewBricks lists the team catalogue bricks the new version uses for the
	// first time ("agent:<id>", "skill:<ref>"; « nouvelle brique » badge).
	NewBricks []string `json:"new_bricks,omitempty"`
}

// Widenings returns the items that widen the workflow.
func (r *ImpactReport) Widenings() []ImpactItem {
	var out []ImpactItem
	for _, it := range r.Items {
		if it.Level == ImpactWiden {
			out = append(out, it)
		}
	}
	return out
}

func (r *ImpactReport) add(level ImpactLevel, code, path string, args ...any) {
	r.Items = append(r.Items, ImpactItem{Code: code, Level: level, Path: path,
		Message: i18n.Tf("teamstate.workflow.impact."+code, args...)})
}

// Impact compares the resolution from (nil: new workflow) with to.
func (s *Service) Impact(from, to *Resolution) ImpactReport {
	var agents wf.AgentCatalog
	if to != nil {
		agents = to.env.Agents
	}
	var f, t *wf.Spec
	if from != nil {
		f = from.Spec
	}
	if to != nil {
		t = to.Spec
	}
	return impactOf(f, t, agents)
}

func impactOf(f, t *wf.Spec, agents wf.AgentCatalog) ImpactReport {
	r := ImpactReport{}
	if t == nil {
		return r
	}
	r.Risk = t.Risk
	if f == nil {
		r.New = true
		r.add(ImpactInfo, "new_workflow", "", t.ID, t.Risk)
		for _, id := range t.Members() {
			if writes(agents, id) {
				r.add(ImpactInfo, "agent_writes", "agents."+id, id)
			}
		}
		if t.AllowsRuntime(wf.RuntimeRemote) {
			r.add(ImpactInfo, "remote_allowed", "runtime.allowed")
		}
		return r
	}

	switch fr, tr := f.Risk.Rank(), t.Risk.Rank(); {
	case tr > fr:
		r.add(ImpactWiden, "risk_raised", "risk", f.Risk, t.Risk)
	case tr < fr:
		r.add(ImpactInfo, "risk_lowered", "risk", f.Risk, t.Risk)
	}
	if f.Isolation == wf.IsolationStrict && t.Isolation != wf.IsolationStrict {
		r.add(ImpactWiden, "isolation_loosened", "isolation")
	}
	if f.EntryAgent() != t.EntryAgent() {
		r.add(ImpactInfo, "entry_changed", "entry.agent", f.EntryAgent(), t.EntryAgent())
	}

	// Agents.
	fm, tm := f.Members(), t.Members()
	for _, id := range tm {
		if slices.Contains(fm, id) {
			continue
		}
		if writes(agents, id) {
			r.add(ImpactWiden, "agent_added_writes", "agents."+id, id)
		} else {
			r.add(ImpactInfo, "agent_added", "agents."+id, id)
		}
	}
	for _, id := range fm {
		if !slices.Contains(tm, id) {
			r.add(ImpactInfo, "agent_removed", "agents."+id, id)
		}
	}

	// Runtimes and modes.
	for _, rt := range t.AllowedRuntimes() {
		if f.AllowsRuntime(rt) {
			continue
		}
		if rt == wf.RuntimeRemote {
			r.add(ImpactWiden, "remote_allowed", "runtime.allowed")
		} else {
			r.add(ImpactWiden, "runtime_added", "runtime.allowed", rt)
		}
	}
	for _, m := range t.AllowedModes() {
		if slices.Contains(f.AllowedModes(), m) {
			continue
		}
		level := ImpactInfo
		if m == wf.ModeAuto {
			level = ImpactWiden
		}
		r.add(level, "mode_added", "modes.allowed", m)
	}

	impactCheckpoints(&r, f, t)

	// Beads and limits.
	switch {
	case f.Beads != nil && t.Beads == nil:
		r.add(ImpactWiden, "beads_unrestricted", "beads")
	case f.Beads != nil && t.Beads != nil:
		if extra := missing(t.Beads.Allow, f.Beads.Allow); len(extra) > 0 {
			r.add(ImpactWiden, "beads_widened", "beads.allow", strings.Join(extra, ", "))
		}
	}
	if fb, tb := budget(f), budget(t); fb != nil && (tb == nil || *tb > *fb) {
		shown := "∞"
		if tb != nil {
			shown = fmt.Sprint(*tb)
		}
		r.add(ImpactWiden, "budget_raised", "limits.budget_usd", *fb, shown)
	}

	// Resources.
	if extra := missing(t.MCP, f.MCP); len(extra) > 0 {
		r.add(ImpactWiden, "mcp_added", "mcp", strings.Join(extra, ", "))
	}
	if extra := missing(pluginIDs(t), pluginIDs(f)); len(extra) > 0 {
		r.add(ImpactWiden, "plugins_added", "plugins", strings.Join(extra, ", "))
	}
	if codeMode(t) && !codeMode(f) {
		r.add(ImpactWiden, "code_mode_enabled", "code_mode")
	}
	if extra := missing(skillExtra(t), skillExtra(f)); len(extra) > 0 {
		r.add(ImpactInfo, "skills_added", "skills.extra", strings.Join(extra, ", "))
	}
	if gone := missing(skillDeny(f), skillDeny(t)); len(gone) > 0 {
		r.add(ImpactInfo, "skills_undenied", "skills.deny", strings.Join(gone, ", "))
	}

	// Inputs and prompt.
	if extra := missing(t.Inputs.Keys(), f.Inputs.Keys()); len(extra) > 0 {
		r.add(ImpactInfo, "inputs_added", "inputs", strings.Join(extra, ", "))
	}
	if gone := missing(f.Inputs.Keys(), t.Inputs.Keys()); len(gone) > 0 {
		r.add(ImpactInfo, "inputs_removed", "inputs", strings.Join(gone, ", "))
	}
	if !samePrompt(f.Prompt, t.Prompt) {
		r.add(ImpactInfo, "prompt_changed", "prompt")
	}
	return r
}

func impactCheckpoints(r *ImpactReport, f, t *wf.Spec) {
	for _, id := range f.Checkpoints.Keys() {
		fc, _ := f.Checkpoints.Get(id)
		tc, ok := t.Checkpoints.Get(id)
		base := "checkpoints." + id
		if !ok {
			r.add(ImpactWiden, "checkpoint_removed", base, id)
			continue
		}
		if fc.IsMandatory() && !tc.IsMandatory() {
			r.add(ImpactWiden, "checkpoint_not_mandatory", base+".mandatory", id)
		}
		if tc.RemotePolicy().Rank() < fc.RemotePolicy().Rank() {
			r.add(ImpactWiden, "checkpoint_remote_loosened", base+".remote", id, fc.RemotePolicy(), tc.RemotePolicy())
		}
		modes := append(slices.Clone(f.AllowedModes()), t.AllowedModes()...)
		sort.Strings(modes)
		for _, m := range slices.Compact(modes) {
			if fb, tb := fc.Behavior(m), tc.Behavior(m); tb.Strictness() < fb.Strictness() {
				r.add(ImpactWiden, "checkpoint_loosened", base+".mode."+m, id, m, fb, tb)
			}
		}
	}
	for _, id := range t.Checkpoints.Keys() {
		if _, ok := f.Checkpoints.Get(id); !ok {
			r.add(ImpactInfo, "checkpoint_added", "checkpoints."+id, id)
		}
	}
}

// writes reports an agent that may modify files or run an unrestricted
// shell (unknown agents count as writing).
func writes(cat wf.AgentCatalog, id string) bool {
	if cat == nil {
		return false
	}
	a, ok := cat.Agent(id)
	return !ok || a.Edits || a.Shell
}

func missing(s, from []string) []string {
	var out []string
	for _, v := range s {
		if !slices.Contains(from, v) {
			out = append(out, v)
		}
	}
	return out
}

func budget(s *wf.Spec) *float64 {
	if s.Limits == nil {
		return nil
	}
	return s.Limits.BudgetUSD
}

func codeMode(s *wf.Spec) bool { return s.CodeMode != nil && *s.CodeMode }

func pluginIDs(s *wf.Spec) []string {
	out := make([]string, len(s.Plugins))
	for i, p := range s.Plugins {
		out[i] = p.ID
	}
	return out
}

func skillExtra(s *wf.Spec) []string {
	if s.Skills == nil {
		return nil
	}
	return s.Skills.Extra
}

func skillDeny(s *wf.Spec) []string {
	if s.Skills == nil {
		return nil
	}
	return s.Skills.Deny
}

func samePrompt(a, b *wf.Prompt) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
