package bundle

import (
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Sessions whose session rules are not posted yet (p6-debts, closed by
// default). A session oh creates gets its session rules in the creation
// request; a subagent session, created by the tool's delegation, gets them
// only when the daemon handles its creation event (or resyncs after a lost
// event stream). Meanwhile only the rules of its agent apply, and the locks
// of the checkpoints (`unlocks:`, `writes_after`, gated agents) exist in the
// session rules only: `git commit*: allow` of a developer would commit
// before the checkpoint that unlocks it.
//
// PendingGuard are the requests the oh plugin never lets through on the
// word of the rendered rules alone: it relays them to oh, which answers
// from the session rules of the session's oh session, adopting a subagent
// session that is not linked yet. Without an answer (daemon unreachable,
// session unknown, timeout) the plugin refuses them. They are the session
// rules of a session that passed nothing in a mode where every checkpoint
// pauses (the most locks a session of the workflow can have), plus the
// rules of every fan-out role, all as "deny": the plugin only uses them to
// recognise the requests, never to loosen a decision.
//
// Requests outside these rules are unchanged.

// pendingMode is a mode no workflow declares: every checkpoint pauses in
// it, so that nothing is skipped and every lock holds.
const pendingMode = "\x00oh-pending"

// PendingGuard returns the requests held until oh confirms the session
// rules of the session (nil for a bundle without workflow).
func PendingGuard(wf *sessionspec.WorkflowRuntime) []sessionspec.PermissionRule {
	if wf == nil {
		return nil
	}
	rules := append(SessionRules(wf, pendingMode, nil), FanoutRoleRules(FanoutParent)...)
	seen := map[sessionspec.PermissionRule]bool{}
	var out []sessionspec.PermissionRule
	for _, r := range rules {
		if r.Action == actionCheckpoint || r.Effect == sessionspec.EffectAllow {
			continue // workflow_checkpoint is always relayed (gated)
		}
		r.Effect = sessionspec.EffectDeny
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// SessionRulesEffect returns the effect of the session rules for the
// resources of a request: the last matching rule of each resource (the
// tool's order), the most restrictive across resources; ok is false when no
// rule matches any resource.
func SessionRulesEffect(rules []sessionspec.PermissionRule, action string, resources []string) (effect sessionspec.Effect, ok bool) {
	for _, res := range resources {
		var eff sessionspec.Effect
		for _, r := range rules {
			if toolMatch(action, r.Action) && toolMatch(res, r.Resource) {
				eff = r.Effect
			}
		}
		switch {
		case eff == sessionspec.EffectDeny:
			return eff, true
		case eff == sessionspec.EffectAsk || (eff != "" && effect == ""):
			effect, ok = eff, true
		}
	}
	return effect, ok
}
