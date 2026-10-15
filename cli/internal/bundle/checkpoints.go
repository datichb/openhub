package bundle

import (
	"slices"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
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
	// Reports (L4): filing and reading the reports of the session.
	actionReport    = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolReport)
	actionReportGet = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolReportGet)
	actionReports   = sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolReports)
)

// CheckpointAction is the neutral permission action of workflow_checkpoint.
func CheckpointAction() string { return actionCheckpoint }

// BaseCheckpointRules are the bundle rules of the workflow MCP server.
func BaseCheckpointRules() []sessionspec.PermissionRule {
	return []sessionspec.PermissionRule{
		{Action: actionStatus, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionOutputs, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionReport, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionReportGet, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: actionReports, Resource: "*", Effect: sessionspec.EffectAllow},
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
//   - the edit of the `writes:` paths held by `writes_after` asks (oh
//     refuses it with its message, order.go);
//   - the shell commands of the operations a checkpoint unlocks (commit,
//     push, close, create, apply, tag, mr) are denied until it is passed;
//     apply, tag and mr are always denied when no checkpoint unlocks them.
func SessionRules(wf *sessionspec.WorkflowRuntime, mode string, st *domain.CheckpointState) []sessionspec.PermissionRule {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	effect := sessionspec.EffectAllow
	for _, c := range wf.Checkpoints {
		if MayAsk(c, mode) {
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
	rules = append(rules, fixLockRules(wf, st)...)
	rules = append(rules, writesLockRules(wf, mode, st)...)
	for _, op := range LockedOps(wf, mode, st) {
		if op == sessionspec.UnlockMR {
			rules = append(rules, publishLockRules(op)...) // shell and MCP
			continue
		}
		for _, p := range lockPatterns[op] {
			rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
		}
	}
	rules = append(rules, ApplyLockedByDefault(wf)...)
	return append(rules, PublishLockedByDefault(wf)...)
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
	sessionspec.UnlockCommit: subcommandPatterns("git", "commit", "revert", "cherry-pick", "merge", "pull", "am", "rebase", "stash pop"),
	sessionspec.UnlockPush:   subcommandPatterns("git", "push"),
	sessionspec.UnlockClose:  append(subcommandPatterns("bd", "close", "done"), "bd update *closed*", "* bd update *closed*", "*/bd update *closed*"),
	sessionspec.UnlockCreate: append(subcommandPatterns("bd", "create", "duplicate", "supersede"), "bd q *", "* bd q *", "*/bd q *"),
	// r9b: every write of the gateway that neither creates nor closes
	// (gateway.updateOp); `comment` is a word of its own (`bd comments`
	// lists).
	sessionspec.UnlockUpdate: append(subcommandPatterns("bd", "update", "edit", "label add", "label remove", "label propagate",
		"dep add", "dep remove", "dep relate", "dep unrelate", "link", "comments add", "reopen", "defer", "undefer", "unclaim",
		"assign", "priority", "set-state", "rename", "promote", "tag", "note", "delete", "duplicate", "supersede"),
		"bd comment *", "* bd comment *", "*/bd comment *"),
	sessionspec.UnlockApply: ApplyLockPatterns,
	// R8 (publishguard.go)
	sessionspec.UnlockTag: tagLockPatterns,
	sessionspec.UnlockMR:  mrLockPatterns,
	// R6 (fanoutguard.go)
	sessionspec.UnlockMerge: mergeLockPatterns,
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

// LockedOps returns the operations (`unlocks:`) still locked: their
// checkpoint is not passed in the current window (an unlocking checkpoint is
// never skipped, CheckpointDef.Behavior), in declaration order. A tag
// unlocked with the commit waits for the release to be committed
// (TagWaitsForCommit).
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
			if _, known := lockPatterns[op]; !known || slices.Contains(out, op) {
				continue // operation of a newer oh: locks nothing here; the commit of two checkpoints (unlock_again)
			}
			if _, open := unlocked(st, op); !open || (op == sessionspec.UnlockTag && TagWaitsForCommit(st)) {
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

// ---------------------------------------------------------------------------
// R3: verification run by oh, conditions, fix rounds and pauses.
// ---------------------------------------------------------------------------

// applyCheckpointVerify copies into rt the R3 fields of the checkpoints of
// s (verify:, when:) and the cap of the fix rounds. In a read-only workflow
// (risk: read) a verification is asked: its checkpoint pauses.
func applyCheckpointVerify(rt *sessionspec.WorkflowRuntime, s *workflow.Spec) {
	if s.Limits != nil && s.Limits.MaxFixRounds != nil {
		rt.MaxFixRounds = *s.Limits.MaxFixRounds
	}
	for i := range rt.Checkpoints {
		def := &rt.Checkpoints[i]
		cp, ok := s.Checkpoints.Get(def.ID)
		if !ok {
			continue
		}
		if v := cp.Verify; v != nil {
			d := &sessionspec.VerifyDef{From: string(v.From), Cmd: append([]string(nil), v.Cmd...), Expect: string(v.ExpectOrPass()),
				In: v.In, TimeoutSec: int(v.TimeoutOrDefault().Seconds()), MustTouch: append([]string(nil), v.MustTouch...),
				Ask:            s.Risk == workflow.RiskRead,
				MustTouchTests: v.MustTouchTests, OutsideTests: string(v.OutsideTests), Since: v.Since, On: string(v.On)}
			if v.From != "" {
				for _, k := range v.EffectiveKinds() {
					d.Kinds = append(d.Kinds, string(k))
				}
			}
			if c := v.Spec; c != nil {
				d.Spec = &sessionspec.SpecCheckDef{Paths: append([]string(nil), c.Paths...), Breakdown: c.Breakdown}
			}
			def.Verify = d
		}
		if t := cp.Tasks; t != nil {
			def.Tasks = &sessionspec.TasksCheckDef{Max: t.EffectiveMax(), Domains: append([]string(nil), t.Domains...)}
		}
		if w := cp.When; !w.IsZero() {
			def.When = &sessionspec.WhenDef{Entry: append([]string(nil), w.Entry...), Labels: append([]string(nil), w.Labels...),
				Paths: append([]string(nil), w.Paths...), Outputs: append([]string(nil), w.Outputs...)}
		}
	}
	// R7: the commit of the location is recorded when a checkpoint a
	// verification counts the changes from (since:) is passed.
	for _, def := range rt.Checkpoints {
		if def.Verify == nil || def.Verify.Since == "" {
			continue
		}
		for i := range rt.Checkpoints {
			if rt.Checkpoints[i].ID == def.Verify.Since {
				rt.Checkpoints[i].Base = true
			}
		}
	}
}

// MayAsk reports whether a checkpoint may wait for the user in mode: it
// pauses, or a condition evaluated by oh (when:) can make it pause.
func MayAsk(c sessionspec.CheckpointDef, mode string) bool {
	b := c.Behavior(mode)
	return NeedsDecision(b) || (c.When != nil && b == sessionspec.CheckpointAuto)
}

// IsDeveloper and IsReviewer name the agents whose rounds are counted
// (developer → reviewer, limits.max_fix_rounds).
func IsDeveloper(agent string) bool { return strings.HasPrefix(agent, "developer") }

// IsReviewer reports whether agent reviews the work of the developer.
func IsReviewer(agent string) bool { return strings.HasPrefix(agent, "reviewer") }

// fixLockRules deny the developers while the fix rounds of a ticket
// reached the cap (until the user steps in at the commit checkpoint).
func fixLockRules(wf *sessionspec.WorkflowRuntime, st *domain.CheckpointState) []sessionspec.PermissionRule {
	if st == nil || st.Unresolved == "" || wf.MaxFixRounds <= 0 {
		return nil
	}
	return []sessionspec.PermissionRule{{Action: sessionspec.ActionSubagent, Resource: "developer*", Effect: sessionspec.EffectDeny}}
}

// PlannedPause is a checkpoint that waits for the user in a mode.
type PlannedPause struct {
	ID        string
	Mandatory bool
	// Conditional: it pauses only when its condition holds.
	Conditional bool
}

// PlannedPauses returns the checkpoints that pause in mode, in declaration
// order (CP-R10): shown when a session is launched. In auto, only the
// mandatory decisions remain.
func PlannedPauses(wf *sessionspec.WorkflowRuntime, mode string) []PlannedPause {
	if wf == nil {
		return nil
	}
	if mode == "" {
		mode = wf.DefaultMode
	}
	var out []PlannedPause
	for _, c := range wf.Checkpoints {
		if !MayAsk(c, mode) {
			continue
		}
		mandatory := c.Mandatory || len(c.Unlocks) > 0
		b := c.Behavior(mode)
		out = append(out, PlannedPause{ID: c.ID, Mandatory: mandatory,
			Conditional: !mandatory && (b == sessionspec.CheckpointConditional || (c.When != nil && b == sessionspec.CheckpointAuto))})
	}
	return out
}
