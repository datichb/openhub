package bundle

import "github.com/datichb/openhub/cli/internal/sessionspec"

// HookGuard refuses, to every agent, the ways around the git hooks of the
// project (A19): `--no-verify` (`-n` for commit), another hooks directory
// (`-c core.hooksPath=…`, `git config core.hooksPath`, `GIT_CONFIG_*`,
// config includes), and writing to .git or to the Beads hooks. The tool
// matches each command of a line on its own, environment prefixes included
// (verified on opencode 2.0.20). Added after the agent's own rules (the last
// matching rule wins).
func HookGuard() []sessionspec.PermissionRule {
	shell := []string{
		"*git*--no-verify*", "*git*commit* -n*", "*git*commit* -an*",
		"*.?ooks?ath*", "*GIT_CONFIG*", "*git*config*include*",
		"*.git/config*", "*.git/hooks*", "*.beads/hooks/*",
	}
	edit := []string{".git/*", "*/.git/*", ".beads/hooks/*", "*/.beads/hooks/*"}
	rules := make([]sessionspec.PermissionRule, 0, len(shell)+len(edit))
	for _, p := range shell {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	for _, p := range edit {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: p, Effect: sessionspec.EffectDeny})
	}
	return rules
}
