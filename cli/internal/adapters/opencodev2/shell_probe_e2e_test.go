//go:build integration && e2e

package opencodev2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Lot 0 of the v5 critical fixes (17-critiques, probes U1–U5): how the tool
// matches shell permission rules, observed through the files each command
// creates when it runs. The simulator of internal/bundle (ShellEffect)
// follows these results; a change of the tool breaks this test first.
func TestE2EShellMatcherSemantics(t *testing.T) {
	b := e2eBundle(t)
	b.Agents[0].Body = "You are a test harness. Do each step the user gives, exactly as written (same characters, same quotes, same spaces), one shell tool call per step, in order. A refused step is not an error: mark it DENIED and go on with the next step, without retrying it or rewriting it. Do every step. After all of them, list each step with OK or DENIED."
	rule := func(action, p string, e sessionspec.Effect) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: action, Resource: p, Effect: e}
	}
	sh := sessionspec.ActionShell
	b.Agents[0].Permissions = []sessionspec.PermissionRule{
		rule(sh, "*", sessionspec.EffectDeny),
		rule(sh, "touch *", sessionspec.EffectAllow),
		rule(sh, "echo *", sessionspec.EffectAllow),
		rule(sh, "sh *", sessionspec.EffectAllow),
		rule(sh, "printenv *", sessionspec.EffectAllow),
		rule(sh, "touch secret*", sessionspec.EffectDeny),
		rule(sh, "touch u5d*", sessionspec.EffectDeny),
		rule(sh, "*U6=*", sessionspec.EffectDeny),
		rule(ActionExternalDir, "*", sessionspec.EffectDeny),
		rule(sessionspec.ActionEdit, "*", sessionspec.EffectAllow),
	}
	r := startE2E(t, b, false)
	outside := filepath.Dir(r.project)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	run := func(env map[string]string, session []sessionspec.PermissionRule, steps []string) string {
		id := sessionspec.NewSessionID()
		require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{
			SessionID: id, Title: "probe", EntryAgent: "lead", Location: r.project, SessionEnv: env, SessionRules: session,
		}))
		require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, "Steps:\n"+strings.Join(steps, "\n")))
		require.NoError(t, NewClient(r.handle.URL, r.handle.Password).Wait(ctx, id))
		return r.text(t, ctx, id)
	}
	exists := func(p string) bool {
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.project, p)
		}
		_, err := os.Stat(p)
		return err == nil
	}

	// Production-like session: an environment is set (the shell does not
	// inherit the server process environment, F30).
	env := map[string]string{"PATH": os.Getenv("PATH"), "HOME": os.Getenv("HOME"), "OH_SESSION_ID": "probe"}
	reply := run(env, []sessionspec.PermissionRule{
		rule(sh, "touch u5s*", sessionspec.EffectDeny),
		rule(sh, "touch u5d*", sessionspec.EffectAllow),
	}, []string{
		"1. shell: U1=1 touch u1_env",
		"2. shell: touch sec''ret_u2q",
		"3. shell: touch  secret_u2s",
		"4. shell: echo x > " + filepath.Join(outside, "outside_u4r"),
		"5. shell: touch " + filepath.Join(outside, "outside_u4a"),
		"6. shell: cd " + outside + " && touch outside_u4c",
		"7. shell: touch u5a_ok && touch secret_u5",
		"8. shell: touch u5s_session",
		"9. shell: touch u5d_session",
		"10. shell: export U6=1 && touch u6_export",
		"11. shell: sh -c 'touch secret_u7'",
		"12. shell: printenv OPENCODE_SERVER_PASSWORD > u3_pw_env.txt",
		"13. shell: printenv OPENCODE_CONFIG_CONTENT > u3_cfg_env.txt",
	})
	t.Logf("reply: %s", reply)

	// U1: the environment prefix is part of the text compared.
	assert.False(t, exists("u1_env"), "U1: `X=1 touch` does not match `touch *`")
	// U2: quotes and spaces are not normalized: a pattern is not a barrier.
	assert.True(t, exists("secret_u2q"), "U2: `touch sec''ret` escapes `touch secret*`")
	assert.True(t, exists("secret_u2s"), "U2: `touch  secret` (two spaces) escapes `touch secret*`")
	// U4: redirections and arguments are not checked against the external
	// directory rules, `cd` is.
	assert.True(t, exists(filepath.Join(outside, "outside_u4r")), "U4: a redirection writes outside the project")
	assert.True(t, exists(filepath.Join(outside, "outside_u4a")), "U4: an argument writes outside the project")
	assert.False(t, exists(filepath.Join(outside, "outside_u4c")), "U4: `cd` outside the project is checked")
	// U5: a line is refused when one of its commands is; session rules come
	// after the agent rules (the last matching rule wins, in both ways).
	assert.False(t, exists("u5a_ok"), "U5: the whole line is refused")
	assert.False(t, exists("u5s_session"), "U5: a session deny wins over an agent allow")
	assert.True(t, exists("u5d_session"), "U5: a session allow wins over an agent deny")
	// Declarations and interpreter strings are not checked as commands.
	assert.True(t, exists("u6_export"), "`export X=…` is not a command for the matcher")
	assert.True(t, exists("secret_u7"), "`sh -c '…'` is not parsed")

	pw, _ := os.ReadFile(filepath.Join(r.project, "u3_pw_env.txt"))
	cfg, _ := os.ReadFile(filepath.Join(r.project, "u3_cfg_env.txt"))
	t.Logf("U3 with a session environment: password visible=%v, config visible=%v", strings.TrimSpace(string(pw)) != "", strings.TrimSpace(string(cfg)) != "")

	// Session without an environment (a sub-session before the daemon
	// adopts it, a session opened from the tool's own interface).
	reply = run(nil, nil, []string{
		"1. shell: printenv OPENCODE_SERVER_PASSWORD > u3_pw_noenv.txt",
		"2. shell: printenv OPENCODE_CONFIG_CONTENT > u3_cfg_noenv.txt",
	})
	t.Logf("reply: %s", reply)
	pw, _ = os.ReadFile(filepath.Join(r.project, "u3_pw_noenv.txt"))
	cfg, _ = os.ReadFile(filepath.Join(r.project, "u3_cfg_noenv.txt"))
	t.Logf("U3 without a session environment: password visible=%v, config visible=%v", strings.TrimSpace(string(pw)) != "", strings.TrimSpace(string(cfg)) != "")
}

// N3 (v5 critical): the global guard of oh, applied by the oh plugin to an
// agent allowed to run anything: git options that run a program or write a
// file, the git pointer file, the oh data and the git environment are
// refused; an ordinary command still runs.
func TestE2EGitOutputAndUploadPack(t *testing.T) {
	b := e2eBundle(t)
	b.Agents[0].Body = "You are a test harness. Do each step the user gives, exactly as written, one tool call per step, in order. A refused step is not an error: mark it DENIED and go on with the next step, without retrying it. Do every step."
	b.Agents[0].Permissions = []sessionspec.PermissionRule{
		{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectAllow},
	}
	b.AgentGuard = append(bundle.GlobalShellGuard(""), bundle.GlobalEditGuard()...)
	r := startE2E(t, b, false)
	require.NoError(t, os.MkdirAll(filepath.Join(r.project, "sub"), 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{SessionID: id, Title: "guard", EntryAgent: "lead", Location: r.project,
		SessionEnv: map[string]string{"PATH": os.Getenv("PATH"), "HOME": os.Getenv("HOME")}}))
	steps := []string{
		"1. shell: git log --output=out_log.txt",
		"2. shell: git fetch --upload-pack='touch pwned_up; git-upload-pack' .",
		"3. edit tool: create the file sub/.git with the single line gitdir: /tmp/elsewhere",
		"4. shell: ls ~/.oh/run > leak.txt",
		"5. shell: GIT_DIR=/tmp git status > gitdir.txt",
		"6. shell: echo fine > ok.txt",
	}
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, "Steps:\n"+strings.Join(steps, "\n")))
	require.NoError(t, NewClient(r.handle.URL, r.handle.Password).Wait(ctx, id))
	t.Logf("reply: %s", r.text(t, ctx, id))
	for _, f := range []string{"out_log.txt", "pwned_up", "sub/.git", "leak.txt", "gitdir.txt"} {
		_, err := os.Stat(filepath.Join(r.project, f))
		assert.True(t, os.IsNotExist(err), "%s: refused by the guard", f)
	}
	_, err := os.Stat(filepath.Join(r.project, "ok.txt"))
	assert.NoError(t, err, "an ordinary command still runs (the agent tried the steps)")
}
