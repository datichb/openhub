package bundle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// releaseRuntime is a release workflow: cp-release unlocks commit, tag and mr.
func releaseRuntime() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{
		ID: "release", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-release", Mandatory: true, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"},
				Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR}},
		},
	}
}

func shellRules(rules []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, r := range rules {
		if r.Action == sessionspec.ActionShell {
			out = append(out, r)
		}
	}
	return out
}

func denies(rules []sessionspec.PermissionRule, action string) bool {
	for _, r := range rules {
		if r.Action == action && r.Effect == sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

var (
	tagForms = []string{
		"git tag v1.2.0", "git tag -a v1.2.0 -m 'Release v1.2.0'", "git tag -s v1", "git tag -f v1", "git tag -d v1",
		"env git tag v1", "X=1 git tag v1", "/usr/bin/git tag v1", "git -C . tag v1", "git --no-pager tag -a v1 -m x",
		"sh -c 'git tag v1'", "bash -c \"git tag -a v1 -m x\"", "git -c alias.t='!git tag' t v1", "git add . && git tag v1",
		"git push --tags", "git push origin --tags", "git push --follow-tags", "git push origin refs/tags/v1", "git push origin tag v1",
		"git update-ref refs/tags/v1 HEAD", "echo abc > .git/refs/tags/v1", "printf x >> .git/packed-refs",
		"gh release create v1", "glab release create v1",
		"git tag -l", "git tag --list 'v*'", // listing: git describe or for-each-ref
	}
	mrForms = []string{
		"glab mr create --fill", "glab mr create -t x -d y", "env glab mr create", "/opt/homebrew/bin/glab mr create",
		"glab -R grp/prj mr create", "glab mr new", "gh pr create --fill", "gh pr create --title x --body y", "gh -R o/r pr create",
		"gh pr new", "sh -c 'glab mr create'", "bash -c 'gh pr create'", "glab api -X POST projects/1/merge_requests",
		"gh api repos/o/r/pulls -f title=x", "hub pull-request", "git push -o merge_request.create origin x",
	}
	publishReads = []string{
		"git describe --tags --abbrev=0", "git for-each-ref --sort=-v:refname refs/tags", "git log --oneline v1.1.0..HEAD",
		"git log --tags --simplify-by-decoration --oneline", "git show v1.1.0", "git rev-parse v1.1.0", "git status", "git log --grep tag",
		"glab mr list", "gh pr list", "gh pr view 3", "glab mr view 3", "git diff --stat v1.1.0..HEAD", "grep -n tag CHANGELOG.md",
	}
)

// E9: tag and mr are refused in a workflow that declares no checkpoint
// unlocking them, whatever the agent's rules (like apply).
func TestPublishLockedByDefault(t *testing.T) {
	rules := shellRules(SessionRules(testRuntime(), "manuel", nil))
	for _, line := range append(append([]string(nil), tagForms...), mrForms...) {
		assert.True(t, lockedLine(rules, line), "must be refused: %s", line)
	}
	for _, line := range publishReads {
		assert.False(t, lockedLine(rules, line), "must stay allowed: %s", line)
	}
	assert.True(t, denies(SessionRules(testRuntime(), "manuel", nil), sessionspec.MCPToolAction("gitlab", "create_mr")), "MCP create_mr refused")
	assert.Empty(t, PublishLockedByDefault(releaseRuntime()), "declared: the checkpoint locks them")
}

// E9: before cp-release, tag and mr are refused; after it, the tag waits for
// the release commit (same checkpoint), the merge request is open at once.
func TestPublishLocks(t *testing.T) {
	wf := releaseRuntime()
	before := SessionRules(wf, "semi-auto", nil)
	for _, line := range append(append([]string{"git commit -m x"}, tagForms...), mrForms...) {
		assert.True(t, lockedLine(shellRules(before), line), "before cp-release: %s", line)
	}
	assert.True(t, denies(before, sessionspec.MCPToolAction("gitlab", "create_mr")))

	now := time.Now()
	open := func(ops ...string) *domain.CheckpointState {
		st := &domain.CheckpointState{Passed: map[string]time.Time{"cp-release": now}, Unlocks: map[string]domain.CheckpointUnlock{}}
		for _, op := range ops {
			st.Unlocks[op] = domain.CheckpointUnlock{Checkpoint: "cp-release", At: now}
		}
		return st
	}
	passed := open(sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR)
	assert.Equal(t, []string{sessionspec.UnlockTag}, LockedOps(wf, "semi-auto", passed), "tag waits for the commit")
	rules := SessionRules(wf, "semi-auto", passed)
	assert.False(t, lockedLine(shellRules(rules), "git commit -m 'chore(release): v1.3.0'"))
	assert.True(t, lockedLine(shellRules(rules), "git tag -a v1.3.0 -m x"), "no tag before the release commit")
	assert.False(t, lockedLine(shellRules(rules), "glab mr create --fill"))
	assert.False(t, denies(rules, sessionspec.MCPToolAction("gitlab", "create_mr")))

	committed := open(sessionspec.UnlockTag, sessionspec.UnlockMR) // the commit window closed
	assert.Equal(t, []string{sessionspec.UnlockCommit}, LockedOps(wf, "semi-auto", committed), "committed: the commit locks again, the tag opens")
	assert.False(t, lockedLine(shellRules(SessionRules(wf, "semi-auto", committed)), "git tag -a v1.3.0 -m 'Release v1.3.0'"))

	// A tag unlocked alone (no commit at this checkpoint) opens at once.
	alone := releaseRuntime()
	alone.Checkpoints[0].Unlocks = []string{sessionspec.UnlockTag}
	assert.Empty(t, LockedOps(alone, "semi-auto", open(sessionspec.UnlockTag)))
	// A commit window opened by another checkpoint does not hold the tag.
	other := open(sessionspec.UnlockTag)
	other.Unlocks[sessionspec.UnlockCommit] = domain.CheckpointUnlock{Checkpoint: "cp-commit", At: now}
	assert.False(t, tagWaitsForCommit(other))
	// Passed again on the commit that closed its previous commit window:
	// the tag does not wait; after a new commit window on another head, it does.
	again := open(sessionspec.UnlockCommit, sessionspec.UnlockTag)
	again.Unlocks[sessionspec.UnlockCommit] = domain.CheckpointUnlock{Checkpoint: "cp-release", At: now, Head: "c2"}
	again.Committed = map[string]string{"cp-release": "c2"}
	assert.False(t, tagWaitsForCommit(again))
	again.Committed["cp-release"] = "c1"
	assert.True(t, tagWaitsForCommit(again))
}

// The workflow map names the publishing operations and their order.
func TestUnlockTextPublish(t *testing.T) {
	text := unlockText("cp-release", []string{sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR})
	for _, want := range []string{"`git tag`", "`glab mr create`", "Le tag vient après le commit", "La création du tag et l'ouverture de la merge request se referment", "Le push (branche et tags) revient à l'utilisateur"} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "fermeture du ticket")
	tag := unlockText("cp-x", []string{sessionspec.UnlockTag})
	assert.Contains(t, tag, "crée le tag aussitôt")
	assert.NotContains(t, tag, "merge request")
	assert.NotContains(t, unlockText("cp-x", []string{sessionspec.UnlockCommit}), "tag")
}
