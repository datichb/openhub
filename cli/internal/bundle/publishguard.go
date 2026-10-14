package bundle

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Publishing guard (R8, E9): creating a tag and opening a merge request are
// operations a checkpoint unlocks (`unlocks: [tag, mr]`, risk: publish
// only). Until it is passed, oh refuses them to every agent of the session;
// in a workflow that declares no checkpoint unlocking them, they are always
// refused (PublishLockedByDefault), whatever the rules of the agent allow.
//
// Like the other locks, these are guard rails on the raw text of a command
// (shellsim.go): the real protection is the credentials given to the
// session. Listing tags stays possible with the commands that only read
// (`git describe --tags`, `git for-each-ref refs/tags`, `git log --tags`):
// every `git tag` is refused while the tag is locked, its listing forms
// included, so that no option combination creates a tag.

// tagLockPatterns refuse what creates, moves or publishes a tag: `git tag`
// (and its detours, subcommandPatterns), pushing tags (`--tags`,
// `--follow-tags`, `refs/tags/…`, `git push origin tag v1`), writing the
// reference by hand, and the releases of the forges (they create the tag on
// the remote).
var tagLockPatterns = append(subcommandPatterns("git", "tag"),
	"*git*push*--tags*", "*git*push*--follow-tags*", "*git*push*refs/tags*", "*git*push* tag *",
	"*git*update-ref*refs/tags*", "*.git/refs/tags*", "*packed-refs*",
	"*gh *release*create*", "*gh *release*upload*", "*glab *release*create*", "*glab *release*upload*",
)

// mrLockPatterns refuse what opens a merge request: the CLIs of the forges
// (and their aliases), their API, and the GitLab push options.
var mrLockPatterns = []string{
	"glab mr create*", "* glab mr create*", "*/glab mr create*", "glab -* mr create*", "*glab* mr new*",
	"gh pr create*", "* gh pr create*", "*/gh pr create*", "gh -* pr create*", "*gh* pr new*",
	"*sh -c *glab*mr*create*", "*sh -c *gh*pr*create*",
	"*glab*api*merge_requests*", "*gh*api*/pulls*", "*hub *pull-request*",
	"*merge_request.create*",
}

// PublishLockPatterns returns the shell patterns of tag and mr (refused by
// default, PublishLockedByDefault).
func PublishLockPatterns() []string {
	return append(append([]string(nil), tagLockPatterns...), mrLockPatterns...)
}

// mrMCPActions are the MCP tools that open a merge request.
var mrMCPActions = []string{sessionspec.MCPToolAction("gitlab", "create_mr")}

// publishOps are the publishing operations locked by default.
var publishOps = []string{sessionspec.UnlockTag, sessionspec.UnlockMR}

// publishLockRules returns the rules that refuse op (tag or mr).
func publishLockRules(op string) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, p := range lockPatterns[op] {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	if op == sessionspec.UnlockMR {
		for _, a := range mrMCPActions {
			out = append(out, sessionspec.PermissionRule{Action: a, Resource: "*", Effect: sessionspec.EffectDeny})
		}
	}
	return out
}

// PublishLockedByDefault returns the session rules that refuse tag and mr
// in a workflow that declares no checkpoint unlocking them (nothing would
// ever open them).
func PublishLockedByDefault(wf *sessionspec.WorkflowRuntime) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, op := range publishOps {
		if _, ok := UnlockingCheckpoint(wf, op); !ok {
			out = append(out, publishLockRules(op)...)
		}
	}
	return out
}

// TagWaitsForCommit reports whether the tag stays locked although its
// checkpoint was passed: the same checkpoint unlocked the commit and the
// release is not committed yet (its window is open). The tag then always
// marks the release commit, never the commit before it. A checkpoint
// passed again right on the commit that closed its previous commit window
// (the release is committed, nothing new) does not hold the tag back.
func TagWaitsForCommit(st *domain.CheckpointState) bool {
	tag, tagOpen := unlocked(st, sessionspec.UnlockTag)
	commit, commitOpen := unlocked(st, sessionspec.UnlockCommit)
	if !tagOpen || !commitOpen || !strings.EqualFold(tag.Checkpoint, commit.Checkpoint) {
		return false
	}
	done, ok := st.Committed[commit.Checkpoint]
	return !ok || done == "" || done != commit.Head
}
