package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// S-06, S-08: the Beads workspace is written by bd only.
func TestBeadsDirGuard(t *testing.T) {
	own := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "bd *": "allow", "echo *": "allow", "cat *": "allow", "git *": "allow"}, "edit": "allow"})
	rules := append(own, BeadsDirGuard()...)
	eval := func(action, res string) sessionspec.Effect {
		var set []sessionspec.PermissionRule
		for _, r := range rules {
			if r.Action == action {
				set = append(set, r)
			}
		}
		e, _ := evaluate(set, res)
		return e
	}
	for _, cmd := range []string{
		"echo /other/.beads > .beads/redirect", "BD_GIT_HOOK=1 bd hooks run post-merge 0", "cat x > .beads/metadata.json",
		"echo '{}' >> .beads/issues.jsonl", "cp x .beads/config.yaml", "mkdir -p sub/.beads && echo x > sub/.beads/redirect",
	} {
		assert.Equal(t, sessionspec.EffectDeny, eval(sessionspec.ActionShell, cmd), cmd)
	}
	for _, cmd := range []string{"bd show pr-1", "bd list --json", "git status", "cat .beads/README.md"} {
		assert.Equal(t, sessionspec.EffectAllow, eval(sessionspec.ActionShell, cmd), cmd)
	}
	for _, p := range []string{".beads/redirect", ".beads/issues.jsonl", "sub/.beads/redirect", ".beads/config.yaml"} {
		assert.Equal(t, sessionspec.EffectDeny, eval(sessionspec.ActionEdit, p), p)
	}
	assert.Equal(t, sessionspec.EffectAllow, eval(sessionspec.ActionEdit, "docs/beads.md"))
}
