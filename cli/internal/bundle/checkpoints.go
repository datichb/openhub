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
//   - every delegation is denied while the circuit breaker holds;
//   - the shell commands of the operations a checkpoint unlocks (commit,
//     push, close) are denied until it is passed.
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
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "*", Effect: sessionspec.EffectDeny})
	} else {
		locked := LockedAgents(wf, mode, st)
		sort.Strings(locked)
		for _, a := range locked {
			rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: a, Effect: sessionspec.EffectDeny})
		}
	}
	for _, op := range LockedOps(wf, mode, st) {
		for _, p := range lockPatterns[op] {
			rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
		}
	}
	return rules
}

// lockPatterns are the shell commands refused while an operation is locked.
// The tool matches each command of a compound line on its own (`a && b`,
// pipes, subshells; verified on opencode 2.0.20). The patterns are anchored
// on the subcommand (N10), so that `git log --grep commit`, `git diff
// src/pushX.ts` or `bd list --status closed` stay allowed, and cover the
// usual detours: a prefix (`env git commit`, `X=1 git commit`), global
// options (`git -c k=v commit`, `git -C dir commit`, `bd --json close`), a
// path (`/usr/bin/git commit`), a shell string (`sh -c 'git commit'`), a git
// alias defined on the command line. Commits made by other commands (revert,
// cherry-pick, merge, pull, am) are locked with commit. Closing and creating
// tickets are also refused by the Beads gateway, whatever the shell.
var lockPatterns = map[string][]string{
	sessionspec.UnlockCommit: subcommandPatterns("git", "commit", "revert", "cherry-pick", "merge", "pull", "am"),
	sessionspec.UnlockPush:   subcommandPatterns("git", "push"),
	sessionspec.UnlockClose:  append(subcommandPatterns("bd", "close", "done"), "bd update *closed*", "* bd update *closed*", "*/bd update *closed*"),
	sessionspec.UnlockCreate: append(subcommandPatterns("bd", "create", "duplicate", "supersede"), "bd q *", "* bd q *", "*/bd q *"),
	sessionspec.UnlockApply:  ApplyLockPatterns,
}

// wordOnly are the subcommands that prefix harmless ones (git merge-base).
var wordOnly = map[string]bool{"merge": true}

// subcommandPatterns returns the patterns that refuse `<tool> <sub>` for
// each sub, whatever comes before the tool or between the tool and sub.
func subcommandPatterns(tool string, subs ...string) []string {
	var out []string
	for _, sub := range subs {
		word := sub + "*"
		if wordOnly[sub] {
			word = sub + " *" // git merge, not git merge-base
		}
		out = append(out,
			tool+" "+word,               // git commit -m x
			"* "+tool+" "+word,          // env git commit, X=1 git commit
			"*/"+tool+" "+word,          // /usr/bin/git commit
			"*sh -c *"+tool+"*"+sub+"*", // sh -c 'git commit', bash -c "…"
		)
		if wordOnly[sub] {
			out = append(out, tool+" "+sub, "* "+tool+" "+sub, "*/"+tool+" "+sub)
		}
		// Global options first (git -c k=v commit, git -C dir commit, bd
		// --json close): the subcommand is a word of its own, so that
		// `bd --json list --status closed` stays allowed.
		for _, pre := range []string{"", "* ", "*/"} {
			out = append(out, pre+tool+" -* "+sub+" *", pre+tool+" -* "+sub)
		}
	}
	if tool == "git" {
		// git -c alias.x='!git commit' x: an alias hides the subcommand.
		out = append(out, "git -*alias*", "* git -*alias*", "*/git -*alias*")
	}
	return out
}

// ApplyLockPatterns are the infrastructure changes refused while `apply` is
// locked (leading `*`: global flags such as --context, --profile, -chdir may
// come first). The infra guard of the agents refuses them outside a workflow.
var ApplyLockPatterns = []string{
	"*terraform* apply*", "*terraform* destroy*", "*terraform* import*", "*terraform* refresh*",
	"*terraform* taint*", "*terraform* untaint*", "*terraform* state rm*", "*terraform* state mv*",
	"*terraform* state push*", "*terraform* force-unlock*",
	"*tofu* apply*", "*tofu* destroy*", "*tofu* import*", "*tofu* state rm*", "*tofu* state mv*",
	"*kubectl* apply*", "*kubectl* create*", "*kubectl* delete*", "*kubectl* replace*", "*kubectl* patch*",
	"*kubectl* edit*", "*kubectl* scale*", "*kubectl* rollout *", "*kubectl* drain*", "*kubectl* cordon*",
	"*kubectl* taint*", "*kubectl* label*", "*kubectl* annotate*", "*kubectl* set *",
	"*helm* install*", "*helm* upgrade*", "*helm* uninstall*", "*helm* delete*", "*helm* rollback*",
	"*pulumi* up*", "*pulumi* destroy*", "*pulumi* refresh*", "*pulumi* import*",
	"*aws* create-*", "*aws* delete-*", "*aws* put-*", "*aws* update-*", "*aws* modify-*", "*aws* terminate-*",
	"*aws* s3 cp*", "*aws* s3 mv*", "*aws* s3 rm*", "*aws* s3 sync*",
	"*gcloud* create*", "*gcloud* delete*", "*gcloud* update*", "*gcloud* deploy*",
	"*az* create*", "*az* delete*", "*az* update*", "*az* deploy*",
}

// LockedOps returns the operations (`unlocks:`) still locked: their
// checkpoint is not passed in the current window (an unlocking checkpoint is
// never skipped, CheckpointDef.Behavior), in declaration order.
func LockedOps(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []string {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	var out []string
	for _, c := range wf.Checkpoints {
		if c.Behavior(mode) == sessionspec.CheckpointSkip {
			continue
		}
		for _, op := range c.Unlocks {
			if _, known := lockPatterns[op]; !known {
				continue // operation of a newer oh: locks nothing here
			}
			if _, open := unlocked(st, op); !open {
				out = append(out, op)
			}
		}
	}
	return out
}

// UnlockingCheckpoint returns the checkpoint that unlocks op ("" = none).
func UnlockingCheckpoint(wf *sessionspec.WorkflowRuntime, op string) (sessionspec.CheckpointDef, bool) {
	if wf == nil {
		return sessionspec.CheckpointDef{}, false
	}
	for _, c := range wf.Checkpoints {
		for _, o := range c.Unlocks {
			if o == op {
				return c, true
			}
		}
	}
	return sessionspec.CheckpointDef{}, false
}

func unlocked(st *domain.CheckpointState, op string) (domain.CheckpointUnlock, bool) {
	if st == nil {
		return domain.CheckpointUnlock{}, false
	}
	u, ok := st.Unlocks[op]
	return u, ok
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
