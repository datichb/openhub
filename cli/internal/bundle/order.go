package bundle

import (
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Order of the checkpoints and of the writes (r5-fixes, workflow
// validate_order.go).
//
//   - `writes_after`: the `writes:` paths of an agent stay closed until the
//     checkpoint is passed. The session rules turn their edit into "ask"
//     for every agent of the session; the oh plugin relays the request and
//     oh refuses it with a message that tells the agent to wait for the
//     checkpoint (CheckpointService.Evaluate). A rule that denies would be
//     refused by the tool before the plugin is asked, with its generic
//     message. Without an answer of oh, the request asks the user.
//   - `after` on a checkpoint: oh refuses the checkpoint while the earlier
//     one has not been passed (CheckpointService.Reached).

// LockedWrites returns the writes gates whose checkpoint is not passed (a
// checkpoint skipped in the mode opens them).
func LockedWrites(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []sessionspec.WritesGate {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	var out []sessionspec.WritesGate
	for _, g := range wf.WritesGates {
		if !gateOpen(wf, mode, st, g.After) {
			out = append(out, g)
		}
	}
	return out
}

// writesLockRules make the edit of the locked writes paths ask, so that oh
// refuses it with its message.
func writesLockRules(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, g := range LockedWrites(wf, mode, st) {
		for _, p := range g.Paths {
			out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: WritesPattern(p), Effect: sessionspec.EffectAsk})
		}
	}
	return out
}

// LockedWrite returns the gate that holds the edit of resource (an edit
// permission resource), if any.
func LockedWrite(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState, resource string) (sessionspec.WritesGate, bool) {
	for _, g := range LockedWrites(wf, mode, st) {
		for _, p := range g.Paths {
			if toolMatch(resource, WritesPattern(p)) {
				return g, true
			}
		}
	}
	return sessionspec.WritesGate{}, false
}

// CheckpointWaitsFor returns the checkpoint c waits for (`after:`), when it
// is not passed yet: "" when c may be passed. A checkpoint skipped in the
// mode, or absent from the session (another method), does not hold it.
func CheckpointWaitsFor(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState, c sessionspec.CheckpointDef) string {
	if wf == nil || c.After == "" {
		return ""
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	first, ok := wf.Checkpoint(c.After)
	if !ok || st.HasPassed(c.After) || first.Behavior(mode) == sessionspec.CheckpointSkip {
		return ""
	}
	return c.After
}
