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

// p4-fixes: the guard aims at the subcommand of git, not at any word of the
// command: a message that holds "git" then "clean" (a release tag, a commit)
// runs, every form of a real `git clean` is still refused.
func TestGitShellGuardSubcommandOnly(t *testing.T) {
	own := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "allow"}})
	rules := append(own, GitShellGuard(own)...)
	for _, cmd := range []string{
		"git clean -fd", "git clean", "git clean -n", "git clean -xdf -- build/", "/usr/bin/git clean -fd", `\git clean -fd`,
		"env git clean -fd", "X=1 git clean -fd", "xargs git clean -fd", "command git clean -fd", "nice -n 5 git clean -f",
		"git -C /tmp/p clean -fd", "git -C . clean -fd", "git --no-pager clean -f", "git --git-dir=.git --work-tree=. clean -f",
		"git -C a -C b clean -f", "git -C /tmp/p  clean -fd", "git  clean -fd", "git\tclean -fd", "git \\\nclean -fd",
		"sh -c 'git clean -fd'", `bash -c "git clean -fd"`, "bash -lc 'cd /tmp/p && git clean -fd'", "sh -c 'git -C x clean -f'",
		`eval "git clean -fd"`, "'git' clean -fd", `"git" clean -fd`, "true && git clean -fd", "echo $(git clean -fd)",
		"git config alias.c clean", "git config --global alias.wipe '!git clean -fd'",
		// the other refused subcommands keep the same forms
		"git -C /tmp/p checkout app/store.py", "env git stash", "sh -c 'git reset --hard'", "git --no-pager rm x",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{
		`git tag -a v5.0.0 -m "Release 5.0.0: git hooks, cleanup of the guard"`,
		`git commit -m "chore: clean the git guard"`, `git commit -m "fix(git): cleanup after rm and checkout"`,
		`git commit -m "docs: stash, clean and checkout of git are refused"`, "git log --grep cleanup",
		"git log --oneline -- cleanup.go", "git diff main -- internal/clean.go", "git status && make clean",
		"git branch -d feat/cleanup", `git commit -m "feat: add cleaner" -- docs/cleanup.md`, "git config user.name x",
		"git show HEAD:clean.go", "git checkout -b feat/clean", "git stash list", "git rm --cached clean.log",
	} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, cmd), cmd)
	}
	// Accepted false positives: the command itself in a message, a word of
	// the message after a global option of git.
	for _, cmd := range []string{`git commit -m "docs: git clean is refused"`, `git -C /tmp/p commit -m "chore: cleanup"`} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
}
