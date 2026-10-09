package bundle

import "github.com/datichb/openhub/cli/internal/sessionspec"

// BeadsDirGuard refuses, to every agent, writing the Beads workspace
// itself (audit v5, S-06, S-08): bd writes `.beads/`, agents go through bd.
// A `.beads/redirect`, `metadata.json` or `config.yaml` written by an
// agent would point bd at another database, and `BD_GIT_HOOK` set by an
// agent would pose as a git hook. Guardrails only: the gateway fixes the
// workspace of the session and refuses bd when those files changed.
func BeadsDirGuard() []sessionspec.PermissionRule {
	shell := []string{
		"*BD_GIT_HOOK*", "*.beads/redirect*", "*.beads/metadata.json*", "*.beads/config.yaml*",
		"*>*.beads/*", "*.beads/embeddeddolt*", "*.beads/dolt/*",
	}
	edit := []string{".beads/*", "*/.beads/*"}
	rules := make([]sessionspec.PermissionRule, 0, len(shell)+len(edit))
	for _, p := range shell {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	for _, p := range edit {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: p, Effect: sessionspec.EffectDeny})
	}
	return rules
}
