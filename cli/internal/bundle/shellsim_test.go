package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestShellCommandsSplitsLikeTheTool(t *testing.T) {
	cases := map[string][]string{
		"git status":                         {"git status"},
		"git add . && git commit -m x":       {"git add .", "git commit -m x"},
		"a || b; c | d & e":                  {"a", "b", "c", "d", "e"},
		"X=1 git diff":                       {"X=1 git diff"},
		"echo x > out.txt":                   {"echo x > out.txt"},
		"go test ./... 2>&1 | tail -5":       {"go test ./... 2>&1", "tail -5"},
		"echo $(git push)":                   {"git push", "echo $(git push)"},
		"echo \"$(git push)\"":               {"git push", "echo \"$(git push)\""},
		"echo `git push`":                    {"git push", "echo `git push`"},
		"(cd x && git push)":                 {"git push"},
		"cd /tmp && ls":                      {"ls"},
		"export A=1 && git status":           {"git status"},
		"export A=$(git push)":               {"git push"},
		"A=1":                                {},
		"sh -c 'git commit -m x; git push'":  {"sh -c 'git commit -m x; git push'"},
		"if true; then git push; fi":         {"true", "git push"},
		"echo 'a && b'":                      {"echo 'a && b'"},
		"git  push":                          {"git  push"},
		"git pu''sh":                         {"git pu''sh"},
		"diff <(git show a) <(git show b)":   {"git show a", "git show b", "diff <(git show a) <(git show b)"},
		"while read l; do echo $l; done < f": {"read l", "echo $l"},
	}
	for line, want := range cases {
		got := ShellCommands(line)
		if len(want) == 0 {
			assert.Empty(t, got, line)
			continue
		}
		assert.Equal(t, want, got, line)
	}
}

func TestToolMatch(t *testing.T) {
	assert.True(t, toolMatch("git status", "git status *"), "trailing ' *' matches the bare command")
	assert.True(t, toolMatch("git status -s", "git status *"))
	assert.False(t, toolMatch("git statusx", "git status *"))
	assert.True(t, toolMatch("a\nb", "a*"), "newlines")
	assert.False(t, toolMatch("GIT push", "git*"), "case sensitive")
	assert.True(t, toolMatch("git log -1", "git log?-1"))
	assert.True(t, toolMatch("x.y", "x.y"), "dots are literal")
	assert.False(t, toolMatch("xzy", "x.y"))
}

func TestShellEffectLineRefusedWhenOneCommandIs(t *testing.T) {
	allow := func(p string) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectAllow}
	}
	deny := func(p string) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny}
	}
	ask := func(p string) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectAsk}
	}
	rules := []sessionspec.PermissionRule{deny("*"), allow("git *"), allow("ls*"), ask("go test*"), deny("*git*push*")}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "git status && ls"))
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, "git status && git push"))
	assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, "ls && go test ./..."))
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, "go test ./... && git push"), "deny wins over ask")
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, "X=1 git status"), "the environment prefix is part of the text (U1)")
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "git pu''sh"), "quotes are not normalized (U2): a pattern is not a barrier")
	assert.Equal(t, sessionspec.EffectAsk, ShellEffect(nil, "whoami"), "nothing matching asks")
	// Session rules come after the agent rules: the last one wins (U5).
	session := append(append([]sessionspec.PermissionRule(nil), rules...), allow("git push*"))
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(session, "git push"))
}
