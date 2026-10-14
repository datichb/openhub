package bundle

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Documentation written by a planning agent (R4, E8: `agents.<id>.writes`).
// The agent keeps its own rules (edit refused, shell restricted) and gets,
// after them and before the guards, the edit of the declared paths and
// their commit, nothing else:
//
//	edit       docs/wiki/specs/*                          allow
//	shell      git add [--] docs/wiki/specs/*             allow
//	shell      git commit -m "*" -- docs/wiki/specs/*     allow  (also -m '*': only this path is committed)
//	shell      amend, -a/--all, --include, add -A/./-u      deny
//	shell      a path that is not one declared path         deny
//
// Only declared paths are committed (p4-fixes): the message is quoted and
// alone before `--` (a third quote of its kind is refused, so no pathspec
// hides between the message and `--`: `-m x src/a.go -- …`, `-m "x" a.go ""
// -- …`, `-p`, `--pathspec-from-file`, `:(glob)…`), and after `--` comes a
// single path under the declared one: no space, `..`, `/.`, quote, `$`,
// backquote or redirection after its folder. Several declared files are
// committed one by one, or by their folder. Not covered: a message holding
// as many quotes of its kind (refused, like an escaped `\"`).
//
// The commit stays locked until the checkpoint that unlocks it (session
// rules), and the guards of the bundle (git, hooks, Beads data, scratch
// folders, global guard) come after and win. A guard rail like the others
// (globalguard.go): the tool matches the raw text of a command.

// writesDeny are the forms of git add and git commit that would take in
// changes outside the declared paths (the work of the user).
var writesDeny = []string{
	"*git*commit*--amend*", "*git*commit* -a*", "*git*commit*--all*", "*git*commit*--include*", "*git*commit* -i *",
	"*git*commit*--only*", "*git*commit* -o *", "*git*commit*-C *", "*git*commit*--reuse-message*",
	"*git*add* -A*", "*git*add*--all*", "*git*add* -u*", "*git*add*--update*", "*git*add* .", "*git*add* . *", "*git*add* -f*", "*git*add*--force*",
}

// WritesPattern is the tool pattern of a `writes:` path: a folder
// (docs/x/**) becomes docs/x/*, the tool's * spanning folders.
func WritesPattern(p string) string {
	if dir, ok := strings.CutSuffix(p, workflow.WritesAll); ok {
		return dir + "/*"
	}
	return p
}

// WritesRules returns the rules of an agent given the documentation paths
// paths (nil without paths).
func WritesRules(paths []string) []sessionspec.PermissionRule {
	if len(paths) == 0 {
		return nil
	}
	var out []sessionspec.PermissionRule
	add := func(action, resource string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: action, Resource: resource, Effect: eff})
	}
	for _, p := range paths {
		add(sessionspec.ActionEdit, WritesPattern(p), sessionspec.EffectAllow)
	}
	for _, p := range paths {
		t := WritesPattern(p)
		for _, form := range writesForms {
			add(sessionspec.ActionShell, form+t, sessionspec.EffectAllow)
		}
	}
	for _, p := range writesDeny {
		add(sessionspec.ActionShell, p, sessionspec.EffectDeny)
	}
	for _, p := range writesPathDeny(paths) {
		add(sessionspec.ActionShell, p, sessionspec.EffectDeny)
	}
	return out
}

// writesForms are the commands allowed on a declared path, followed by it.
var writesForms = []string{"git add ", "git add -- ", `git commit -m "*" -- `, "git commit -m '*' -- "}

// writesMessageDeny refuses a third quote of the kind that opens the
// message: the only way to put an argument between the message and `--`.
var writesMessageDeny = []string{`git commit -m "*"*"*`, "git commit -m '*'*'*"}

// writesAfterDir is what may not follow the folder of a declared path: a
// second argument, a way out of the folder, a quote, an expansion, a
// redirection. " ?*" and not " *": a pattern ending in " *" also matches
// nothing after the space (toolMatch), so it would refuse every path.
var writesAfterDir = []string{" ?*", "\t*", "\n*", "..*", "/.*", "'*", `"*`, "$*", "`*", ">*", "<*"}

// writesPathDeny returns the refusals of a path that is not one declared
// path, after each allowed form.
func writesPathDeny(paths []string) []string {
	out := append([]string(nil), writesMessageDeny...)
	for _, p := range paths {
		dir, ok := strings.CutSuffix(p, workflow.WritesAll)
		if !ok {
			continue // a file: the allowed forms end with it
		}
		for _, form := range writesForms {
			for _, bad := range writesAfterDir {
				out = append(out, form+dir+"*"+bad)
			}
		}
	}
	return out
}

// writesOf returns the `writes:` paths of agent id in the workflow of req.
func (req Request) writesOf(id string) []string {
	if req.Spec == nil {
		return nil
	}
	return req.Spec.WritesOf(id)
}
