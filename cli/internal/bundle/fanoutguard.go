package bundle

import (
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Fan-out guard (R6, E3, CP-R3). oh merges the children itself: no agent
// of a fan-out (parent, child or merger) pushes, merges, closes a ticket
// or moves the branches of the fan-out, whatever its checkpoints unlock.
// These session rules are added by the CheckpointService for the sessions
// oh recorded as part of a fan-out (domain.SessionFanout); like the other
// locks they hold the raw text of a command (shellsim.go), and oh checks
// the result too: the integration branch must be where oh left it, and the
// Beads gateway refuses the closings (checkpoint.GuardClose).

// Roles of a session in a fan-out (copied from domain.FanoutRole).
const (
	FanoutParent = "parent"
	FanoutChild  = "child"
	FanoutMerger = "merger"
	FanoutFixer  = "fixer"
)

// mergeLockPatterns refuse what merges a branch: git merge and git pull
// (merge-base stays allowed), and the merges of the forges.
var mergeLockPatterns = append(subcommandPatterns("git", "merge", "pull"),
	"*glab *mr*merge*", "*gh *pr*merge*", "*glab*api*merge_requests*/merge*", "*gh*api*/pulls/*/merge*",
)

// fanoutBranchPatterns refuse what moves or removes a branch or a worktree
// of the fan-out by hand (only oh does).
var fanoutBranchPatterns = append(subcommandPatterns("git", "update-ref", "worktree"),
	"*git*branch*oh/fanout/*", "*git*checkout*oh/fanout/*", "*git*switch*oh/fanout/*", "*git*reset*oh/fanout/*",
	"*refs/heads/oh/fanout/*", "*.git/refs/heads/oh*",
)

// mrMergeMCPActions are the MCP tools that merge a merge request.
var mrMergeMCPActions = []string{sessionspec.MCPToolAction("gitlab", "merge_mr"), sessionspec.MCPToolAction("gitlab", "accept_mr")}

// FanoutRoleRules returns the session rules of a session of a fan-out:
// push, merge, ticket closing and the branches of the fan-out refused to
// every role; the commit too for the parent (oh merges, the children
// commit their own work, the merger concludes the merge, the fixer
// commits its fix on the integration branch after its commit checkpoint).
func FanoutRoleRules(role string) []sessionspec.PermissionRule {
	if role != FanoutParent && role != FanoutChild && role != FanoutMerger && role != FanoutFixer {
		return nil
	}
	var patterns []string
	patterns = append(patterns, lockPatterns[sessionspec.UnlockPush]...)
	patterns = append(patterns, mergeLockPatterns...)
	patterns = append(patterns, lockPatterns[sessionspec.UnlockClose]...)
	patterns = append(patterns, fanoutBranchPatterns...)
	patterns = append(patterns, tagLockPatterns...)
	if role == FanoutParent {
		patterns = append(patterns, lockPatterns[sessionspec.UnlockCommit]...)
	}
	out := make([]sessionspec.PermissionRule, 0, len(patterns)+4)
	for _, p := range patterns {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	for _, a := range append(append([]string(nil), mrMergeMCPActions...), mrMCPActions...) {
		out = append(out, sessionspec.PermissionRule{Action: a, Resource: "*", Effect: sessionspec.EffectDeny})
	}
	return out
}

// FanoutLockedOps are the operations a session of a fan-out never runs
// itself (workflow_status, LockedOps).
func FanoutLockedOps(role string) []string {
	switch role {
	case FanoutParent:
		return []string{sessionspec.UnlockCommit, sessionspec.UnlockPush, sessionspec.UnlockMerge, sessionspec.UnlockClose}
	case FanoutChild, FanoutMerger, FanoutFixer:
		return []string{sessionspec.UnlockPush, sessionspec.UnlockMerge, sessionspec.UnlockClose}
	}
	return nil
}

// fanoutDef copies the fan-out of a fan-out session (nil otherwise: the
// spec of another session has none, workflow.Spec.ForFanout).
func fanoutDef(s *workflow.Spec) *sessionspec.FanoutDef {
	f := s.Fanout
	if f == nil {
		return nil
	}
	d := &sessionspec.FanoutDef{Child: f.Child, Tickets: f.Tickets, Parallel: f.EffectiveParallel(), Merge: f.Merge,
		Delegates: append([]string(nil), f.Delegates...), Merger: f.Merger}
	if f.Fix != nil {
		d.Fix, d.FixInput = f.Fix.Workflow, f.Fix.Input
	}
	return d
}
