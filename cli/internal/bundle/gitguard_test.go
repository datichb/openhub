package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A18: whatever the permission base allows, an agent cannot throw away
// changes it did not make; the harmless forms stay as allowed as before.
func TestGitShellGuard(t *testing.T) {
	own := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{
		"*": "deny", "git status*": "allow", "git checkout*": "allow", "git switch*": "allow",
		"git stash*": "allow", "git worktree*": "allow", "rm *": "allow",
	}})
	rules := append(append(own, GitShellGuard(own)...), BeadsShellGuard()...)
	effect := func(cmd string) sessionspec.Effect {
		eff, _ := evaluate(rules, cmd)
		return eff
	}
	for _, cmd := range []string{
		"git checkout app/store.py", "git checkout -- app/store.py", "git checkout HEAD -- .", "git checkout .",
		"git restore app/store.py", "git restore --staged --worktree x", "git reset --hard", "git reset HEAD~1 --hard",
		"git clean -fd", "git stash", "git stash drop", "git stash clear", "git stash push -m x",
		"git switch --discard-changes main", "git switch -f main", "git worktree remove ../x", "git branch -D feat/x",
		"git rm app/store.py", "git -C /tmp/p checkout app/store.py", "git -C /tmp/p reset --hard",
	} {
		assert.Equal(t, sessionspec.EffectDeny, effect(cmd), cmd)
	}
	for _, cmd := range []string{
		"git status", "git switch main", "git switch -c feat/x", "git checkout -b feat/x", "git stash list",
		"git worktree list", "rm build.log",
	} {
		assert.Equal(t, sessionspec.EffectAllow, effect(cmd), cmd)
	}

	// The guard never widens an agent: no `git checkout -b` for a reviewer.
	reviewer := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "git diff*": "allow"}})
	rules = append(reviewer, GitShellGuard(reviewer)...)
	eff, _ := evaluate(rules, "git checkout -b feat/x")
	assert.Equal(t, sessionspec.EffectDeny, eff)
	eff, _ = evaluate(rules, "git diff")
	assert.Equal(t, sessionspec.EffectAllow, eff)
}
