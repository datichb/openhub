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
//	edit       docs/wiki/specs/*                      allow
//	shell      git add [--] docs/wiki/specs/*         allow
//	shell      git commit -m * -- docs/wiki/specs/*   allow  (only these paths are committed)
//	shell      amend, -a/--all, --include, add -A/./-u  deny
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
		add(sessionspec.ActionShell, "git add "+t, sessionspec.EffectAllow)
		add(sessionspec.ActionShell, "git add -- "+t, sessionspec.EffectAllow)
		add(sessionspec.ActionShell, "git commit -m * -- "+t, sessionspec.EffectAllow)
	}
	for _, p := range writesDeny {
		add(sessionspec.ActionShell, p, sessionspec.EffectDeny)
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
