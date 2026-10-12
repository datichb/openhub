package bundle

import (
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// verifyFixDeny are the options that make a verification command change the
// project (lint fixes, formatters, snapshot and golden updates): refused
// after the verification commands, an agent that verifies never fixes.
var verifyFixDeny = []string{"*--fix*", "*--write*", "*-update*", "* -- -u*", "* test -u*"}

// VerifyRules returns the shell rules of an agent marked `verify: true`: the
// build, test, lint and type check commands of the project (commands, from
// bricks.VerifyCommands), then verifyFixDeny. They run the project's code:
// allowed where the code is the team's own, asked in a read-only workflow
// (risk: read, the review of a merge request often written by someone
// else, whose tests would run unseen). Nothing without commands (no
// project path or no known stack).
func VerifyRules(commands []string, risk workflow.Risk) []sessionspec.PermissionRule {
	if len(commands) == 0 {
		return nil
	}
	eff := sessionspec.EffectAllow
	if risk == workflow.RiskRead {
		eff = sessionspec.EffectAsk
	}
	out := make([]sessionspec.PermissionRule, 0, len(commands)+len(verifyFixDeny))
	for _, c := range commands {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: c, Effect: eff})
	}
	for _, p := range verifyFixDeny {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	return out
}

// frontendFilter leaves the frontend domain skills out of keep in a project
// without web frontend, from the signals Build detected once (rootFilter is
// given no project path so that it does not walk the project again).
func frontendFilter(keep func(string) bool, projectPath string, signals bricks.Signals) func(string) bool {
	if bricks.FrontendIn(projectPath, signals) {
		return keep
	}
	frontend := map[string]bool{}
	for _, ref := range bricks.FrontendSkills {
		frontend[ref] = true
	}
	return func(ref string) bool { return !frontend[ref] && keep(ref) }
}
