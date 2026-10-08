//go:build integration && e2e

package opencodev2

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A19 (piste T, T2): the ways around the git hooks are refused by the tool
// to an agent allowed to commit (bundle.HookGuard after its own rules).
func TestE2EHookBypassRefused(t *testing.T) {
	b := e2eBundle(t)
	b.Agents[0].Body = "You are a test harness. Do each step the user gives, exactly as written, one tool call per step, in order. A refused step is not an error: mark it DENIED and go on with the next step, without retrying it. Do every step. After all of them, list each step with OK or DENIED."
	allow := func(p string) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectAllow}
	}
	b.Agents[0].Permissions = append([]sessionspec.PermissionRule{
		{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectDeny},
		allow("git *"), allow("echo *"), allow("env *"),
		{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectAllow},
	}, bundle.HookGuard()...)
	r := startE2E(t, b, false)
	git := func(args ...string) string {
		out, _ := exec.Command("git", append([]string{"-C", r.project}, args...)...).CombinedOutput()
		return string(out)
	}
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	hooks := filepath.Join(r.project, ".beads", "hooks")
	require.NoError(t, os.MkdirAll(hooks, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\necho hook-refuses >&2\nexit 1\n"), 0o755))
	git("config", "core.hooksPath", ".beads/hooks")
	before, _ := os.ReadFile(filepath.Join(r.project, ".git", "config"))

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{SessionID: id, Title: "hooks", EntryAgent: "lead", Location: r.project}))
	steps := []string{
		"shell: git commit --allow-empty --no-verify -m p1",
		"shell: git commit --allow-empty -n -m p2",
		"shell: git -c core.hooksPath=/dev/null commit --allow-empty -m p3",
		"shell: GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null git commit --allow-empty -m p4",
		"shell: git config core.hooksPath /dev/null",
		"shell: echo '[core]' >> .git/config",
		"edit tool: overwrite the file .git/config with the single line [core]",
		"edit tool: overwrite the file .beads/hooks/pre-commit with the line exit 0",
		"shell: git commit --allow-empty -m p5",
	}
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, "Steps:\n"+strings.Join(steps, "\n")))
	require.NoError(t, NewClient(r.handle.URL, r.handle.Password).Wait(ctx, id))
	reply := r.text(t, ctx, id)
	assert.GreaterOrEqual(t, strings.Count(reply, "DENIED"), 6, "the agent tried every step: %s", reply)
	log := git("log", "--format=%s")
	for _, m := range []string{"p1", "p2", "p3", "p4", "p5"} {
		assert.NotContains(t, log, m, "no commit bypassing the hook: %s\n%s", log, reply)
	}
	after, _ := os.ReadFile(filepath.Join(r.project, ".git", "config"))
	assert.Equal(t, string(before), string(after), ".git/config unchanged")
	hook, _ := os.ReadFile(filepath.Join(hooks, "pre-commit"))
	assert.Contains(t, string(hook), "hook-refuses", "the hook is unchanged")
}
