package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// QB1: a bd named by a path bypasses the fake bd of the session (and the
// Beads gateway): the guard, last of every agent, refuses it even when the
// agent allows any shell command.
func TestBeadsShellGuard(t *testing.T) {
	rules := append(ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "allow", "bd *": "allow"}}), BeadsShellGuard()...)
	effect := func(cmd string) sessionspec.Effect {
		var eff sessionspec.Effect
		for _, r := range rules {
			if r.Action == sessionspec.ActionShell && wildcardMatch(r.Resource, cmd) {
				eff = r.Effect
			}
		}
		return eff
	}
	for _, cmd := range []string{"bd close bd-1", "bd", "git status", "ls ./bdx"} {
		assert.Equal(t, sessionspec.EffectAllow, effect(cmd), cmd)
	}
	for _, cmd := range []string{"/opt/homebrew/bin/bd close bd-1", "~/go/bin/bd", "sh -c '/usr/local/bin/bd delete x'", "./bd list"} {
		assert.Equal(t, sessionspec.EffectDeny, effect(cmd), cmd)
	}
}
