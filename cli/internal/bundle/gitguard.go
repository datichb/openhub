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
	// a git alias hides the subcommand (`git config alias.c clean`, then
	// `git c -fd`); `git -c alias.…` is refused by the global guard
	"config *alias*",
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

// gitGuardPrefixes are the forms of `git` before a refused subcommand (v5
// critical N3: the tool matches the raw text of each command, probe U1).
// The subcommand follows `git` or its global options, never any text: a
// commit or tag message that holds "git" then "clean" (`git tag -a v5 -m
// "git hooks, cleanup"`) stays allowed (p4-fixes). Covered: `git sub`,
// after a prefix (`env git`, `X=1 git`, `xargs git`), by path (`/usr/bin/git`,
// `\git`), after global options (`git -C <dir> sub`, `git --no-pager sub`),
// in a shell string (`sh -c '…git sub'`, `bash -lc`, `eval`), a quoted
// `git` (`'git' sub`), several spaces, a tab or a line continuation between
// `git` and the subcommand. Not covered: a quoted or split subcommand
// (`git 'clean'`, `git cl""ean`, as for every rule of the tool).
var gitGuardPrefixes = []string{
	"git ", "* git ", "*/git ",
	"git -* ", "* git -* ", "*/git -* ",
	"*sh -*c *git* ", "*eval *git* ",
	"*git  *", "*git\t*", "*git \\\n*",
	"*git' ", "*git\" ",
}

// GitShellGuard returns the shell rules appended to an agent's rules (own
// rules: what the agent may already run) that refuse the destructive git
// commands (gitGuardPrefixes).
func GitShellGuard(own []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	var shell []sessionspec.PermissionRule
	for _, r := range own {
		if r.Action == sessionspec.ActionShell {
			shell = append(shell, r)
		}
	}
	var out []sessionspec.PermissionRule
	add := func(resource string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: resource, Effect: eff})
	}
	for _, c := range gitGuardDeny {
		for _, pre := range gitGuardPrefixes {
			add(pre+c, sessionspec.EffectDeny)
		}
	}
	for _, k := range gitGuardKeep {
		if eff, ok := evaluate(shell, "git "+k[1]); ok && eff != sessionspec.EffectDeny {
			add("*git* "+k[0], eff)
		}
	}
	return out
}
