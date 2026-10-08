package bundle

import "github.com/datichb/openhub/cli/internal/sessionspec"

// Git commands that throw away work an agent did not do (v5 corrections,
// A18: a developer ran `git checkout app/store.py` and erased the user's
// uncommitted change). They are refused to every agent, last of its rules
// (last match wins), whatever its permission base allows. `git checkout
// <path>` cannot be told from `git checkout <branch>` by a pattern: the whole
// command is refused, branch changes go through `git switch` (and `git
// checkout -b` stays allowed to the agents that had `git checkout`).
var gitGuardDeny = []string{
	"checkout *",
	"restore *",
	"reset --hard*", "reset * --hard*",
	"reset --merge*", "reset * --merge*",
	"reset --keep*", "reset * --keep*",
	"clean*",
	"stash*",
	"switch --discard-changes*", "switch * --discard-changes*",
	"switch -f*", "switch * -f*",
	"switch --force*", "switch * --force*",
	"worktree remove*",
	"branch -D*", "branch * -D*",
	"rm *",
}

// gitGuardKeep are the harmless forms of the refused commands, allowed again
// to an agent whose own rules allowed them before the guard: the pattern and
// the command its own rules are evaluated on.
var gitGuardKeep = [][2]string{
	{"checkout -b *", "checkout -b oh/x"},
	{"stash list*", "stash list"},
	{"stash show*", "stash show"},
	{"rm --cached *", "rm --cached x"},
}

// GitShellGuard returns the shell rules appended to an agent's rules (own
// rules: what the agent may already run) that refuse the destructive git
// commands, also in their `git -C <dir> …` form.
func GitShellGuard(own []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	var shell []sessionspec.PermissionRule
	for _, r := range own {
		if r.Action == sessionspec.ActionShell {
			shell = append(shell, r)
		}
	}
	var out []sessionspec.PermissionRule
	add := func(cmd string, eff sessionspec.Effect) {
		for _, prefix := range []string{"git ", "git -C * "} {
			out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: prefix + cmd, Effect: eff})
		}
	}
	for _, c := range gitGuardDeny {
		add(c, sessionspec.EffectDeny)
	}
	for _, k := range gitGuardKeep {
		if eff, ok := evaluate(shell, "git "+k[1]); ok && eff != sessionspec.EffectDeny {
			add(k[0], eff)
		}
	}
	return out
}
