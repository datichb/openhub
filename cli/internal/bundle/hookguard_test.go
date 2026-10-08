package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A19: the ways around the git hooks are refused to every agent, after its
// own rules (developer-rw allows git commit*, git *-like commands).
func TestHookGuard(t *testing.T) {
	own := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "git commit*": "allow", "git config*": "allow", "git *": "allow", "cat *": "allow", "echo *": "allow"}, "edit": "allow"})
	rules := append(own, HookGuard()...)
	shell := func(cmd string) sessionspec.Effect {
		var set []sessionspec.PermissionRule
		for _, r := range rules {
			if r.Action == sessionspec.ActionShell {
				set = append(set, r)
			}
		}
		e, _ := evaluate(set, cmd)
		return e
	}
	edit := func(path string) sessionspec.Effect {
		var set []sessionspec.PermissionRule
		for _, r := range rules {
			if r.Action == sessionspec.ActionEdit {
				set = append(set, r)
			}
		}
		e, _ := evaluate(set, path)
		return e
	}
	for _, cmd := range []string{
		"git commit --no-verify -m x", "git commit -n -m x", "git commit -nm x", "git commit -anm x", "git push --no-verify",
		"git -c core.hooksPath=/dev/null commit -m x", "git -c core.hookspath=/dev/null commit -m x", "git config core.hooksPath /dev/null",
		"git config --unset core.hooksPath", "env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath git commit -m x",
		"GIT_CONFIG_PARAMETERS='core.hooksPath=/x' git commit -m x", "git config --local include.path ../x",
		"echo '[core]' >> .git/config", "cat .git/hooks/pre-commit", "echo exit 0 > .beads/hooks/pre-commit",
	} {
		assert.Equal(t, sessionspec.EffectDeny, shell(cmd), cmd)
	}
	for _, cmd := range []string{"git commit -m 'feat: x'", "git status", "git log --oneline", "git config user.name"} {
		assert.Equal(t, sessionspec.EffectAllow, shell(cmd), cmd)
	}
	for _, p := range []string{".git/config", ".git/hooks/pre-commit", "sub/.git/config", ".beads/hooks/pre-commit"} {
		assert.Equal(t, sessionspec.EffectDeny, edit(p), p)
	}
	assert.Equal(t, sessionspec.EffectAllow, edit("app/main.py"))
}
