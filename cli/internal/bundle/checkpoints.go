package bundle

import (
	"sort"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Checkpoint rules (P3-T02, P3-T04, P3-T06). The bundle is shared by every
// session of a server group whatever its mode, so it only carries the safe
// base: workflow_checkpoint asks. Mode and progress dependent rules are
// session rules, set when the session is created and replaced by the
// CheckpointService as the session moves on.
//
// A tool permission request does not say which checkpoint is reached (the
// id is only in the tool call input): when a mode mixes paused and automatic
// checkpoints, the rule stays "ask" and oh lets the automatic ones through.

var (
	actionCheckpoint = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolCheckpoint)
	actionStatus     = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolStatus)
	actionOutputs    = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolOutputs)
)

// CheckpointAction is the neutral permission action of workflow_checkpoint.
func CheckpointAction() string { return actionCheckpoint }

// BaseCheckpointRules are the bundle rules of the workflow MCP server.
func BaseCheckpointRules() []sessionspec.PermissionRule {
	return []sessionspec.PermissionRule{
		{Action: actionStatus, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionOutputs, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionCheckpoint, Resource: "*", Effect: sessionspec.EffectAsk},
	}
}

// NeedsDecision reports whether a checkpoint behavior waits for the user
// (conditions are not evaluated by oh: a conditional checkpoint asks).
func NeedsDecision(behavior string) bool {
	return behavior == sessionspec.CheckpointPause || behavior == sessionspec.CheckpointConditional
}

// SessionRules returns the session rules of a workflow session in mode,
// given its progress (nil = just started):
//   - workflow_checkpoint asks when a checkpoint of the mode waits for the
//     user, is allowed otherwise;
//   - each gated agent (`after:`) is denied until its lock is released;
//   - every delegation is denied while the circuit breaker holds.
func SessionRules(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []sessionspec.PermissionRule {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	effect := sessionspec.EffectAllow
	for _, c := range wf.Checkpoints {
		if NeedsDecision(c.Behavior(mode)) {
			effect = sessionspec.EffectAsk
			break
		}
	}
	rules := []sessionspec.PermissionRule{{Action: actionCheckpoint, Resource: "*", Effect: effect}}
	if st != nil && st.Breaker {
		return append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "*", Effect: sessionspec.EffectDeny})
	}
	locked := LockedAgents(wf, mode, st)
	sort.Strings(locked)
	for _, a := range locked {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: a, Effect: sessionspec.EffectDeny})
	}
	return rules
}

// LockedAgents returns the agents whose `after:` lock is not released: the
// checkpoint is not passed (a checkpoint skipped in the mode releases it),
// or the agent has not run yet.
func LockedAgents(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []string {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	var out []string
	for _, g := range wf.Gates {
		if !gateOpen(wf, mode, st, g.After) {
			out = append(out, g.Agent)
		}
	}
	return out
}

func gateOpen(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState, after string) bool {
	if c, ok := wf.Checkpoint(after); ok {
		return st.HasPassed(after) || c.Behavior(mode) == sessionspec.CheckpointSkip
	}
	return st.HasRun(after)
}
