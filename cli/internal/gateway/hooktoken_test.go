package gateway

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/procinfo"
)

func TestGitHookName(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"hooks", "run", "pre-commit"}, "pre-commit"},
		{[]string{"hooks", "run", "prepare-commit-msg", ".git/COMMIT_EDITMSG"}, "prepare-commit-msg"},
		{[]string{"hooks", "run", "prepare-commit-msg", ".git/COMMIT_EDITMSG", "message"}, "prepare-commit-msg"},
		{[]string{"hooks", "run", "prepare-commit-msg", ".git/COMMIT_EDITMSG", "commit", sha}, "prepare-commit-msg"},
		{[]string{"hooks", "run", "post-merge", "0"}, "post-merge"},
		{[]string{"hooks", "run", "post-checkout", sha, sha, "1"}, "post-checkout"},
		{[]string{"hooks", "run", "pre-push", "origin", "git@host:x.git"}, "pre-push"},
		// S-06: anything git would not pass is not a hook.
		{[]string{"hooks", "run", "pre-commit", "extra"}, ""},
		{[]string{"hooks", "run", "post-merge", "0", "--db", "/x"}, ""},
		{[]string{"hooks", "run", "post-merge"}, ""},
		{[]string{"hooks", "run", "post-checkout", "x", sha, "1"}, ""},
		{[]string{"hooks", "run", "prepare-commit-msg", "-f", "message"}, ""},
		{[]string{"--db", "/x", "hooks", "run", "pre-commit"}, ""},
		{[]string{"hooks", "run", "post-rewrite"}, ""},
		{[]string{"hooks", "install"}, ""},
	} {
		assert.Equal(t, tc.want, GitHookName(tc.argv), tc.argv)
	}
}

func TestHookTokensOneTime(t *testing.T) {
	st, _ := OpenStore("")
	g := Grant{SessionID: "s", GroupKey: "g", Location: "/p"}
	tok := st.IssueHookToken(g, "pre-commit")
	assert.False(t, st.consumeHookToken(tok, g, "post-merge"), "another hook")
	tok = st.IssueHookToken(g, "pre-commit")
	other := g
	other.Holder = "child"
	assert.False(t, st.consumeHookToken(tok, other, "pre-commit"), "another holder")
	tok = st.IssueHookToken(g, "pre-commit")
	assert.True(t, st.consumeHookToken(tok, g, "pre-commit"))
	assert.False(t, st.consumeHookToken(tok, g, "pre-commit"), "used once")
	assert.False(t, st.consumeHookToken("", g, "pre-commit"))
}

// The hook mode needs a hook token; a container skips hooks without one.
func TestBeadsGitHookNeedsToken(t *testing.T) {
	f := newFixture(t, []string{"show"})
	call := func(tok string, argv ...string) (int, beadswire.ExecResponse) {
		return f.call(t, f.token, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj", GitHook: true, HookToken: tok})
	}
	code, resp := call("", "hooks", "run", "pre-commit")
	assert.Equal(t, http.StatusOK, code, "container: skipped, nothing runs on the machine")
	assert.Empty(t, resp.Stdout)

	f.b.View = nil
	code, resp = call("", "hooks", "run", "post-merge", "0")
	assert.Equal(t, http.StatusForbidden, code, "BD_GIT_HOOK alone opens nothing")
	assert.NotEmpty(t, resp.Error)
	code, _ = call("ohh_forged", "hooks", "run", "post-merge", "0")
	assert.Equal(t, http.StatusForbidden, code)
	tok := f.b.Store.IssueHookToken(f.grant, "post-merge")
	code, resp = call(tok, "hooks", "run", "post-merge", "0")
	require.Equal(t, http.StatusOK, code, resp.Error)
	assert.Contains(t, string(resp.Stdout), "argv:hooks run -- post-merge 0")
	code, _ = call(tok, "hooks", "run", "post-merge", "0")
	assert.Equal(t, http.StatusForbidden, code, "one time")
}

func TestCheckHookCaller(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	project := realDir(t)
	require.NoError(t, exec.Command("git", "-C", project, "init", "-q").Run())
	require.NoError(t, exec.Command("git", "-C", project, "config", "core.hooksPath", ".beads/hooks").Run())
	hook := filepath.Join(project, ".beads", "hooks", "pre-commit")
	tree := map[int]procinfo.Process{
		10: {PID: 10, PPID: 11, Args: []string{"/oh", "__bd", "hooks", "run", "pre-commit"}},
		11: {PID: 11, PPID: 12, Args: []string{"sh", ".beads/hooks/pre-commit"}},
		12: {PID: 12, PPID: 13, Args: []string{"/usr/bin/git", "commit"}},
		13: {PID: 13, PPID: 1, Args: []string{"zsh"}},
		20: {PID: 20, PPID: 21, Args: []string{"/oh", "__bd", "hooks", "run", "pre-commit"}},
		21: {PID: 21, PPID: 13, Args: []string{"sh", hook}},    // the agent runs the hook itself
		30: {PID: 30, PPID: 12, Args: []string{"/oh", "__bd"}}, // git, but not from the hook
	}
	read := func(pid int) (procinfo.Process, error) {
		if p, ok := tree[pid]; ok {
			return p, nil
		}
		return procinfo.Process{}, errors.New("no process " + strconv.Itoa(pid))
	}
	ctx := context.Background()
	assert.NoError(t, CheckHookCaller(ctx, 10, project, "pre-commit", read))
	assert.ErrorIs(t, CheckHookCaller(ctx, 10, project, "post-merge", read), ErrNotFromGit, "another hook")
	assert.ErrorIs(t, CheckHookCaller(ctx, 20, project, "pre-commit", read), ErrNotFromGit)
	assert.ErrorIs(t, CheckHookCaller(ctx, 30, project, "pre-commit", read), ErrNotFromGit)
	assert.ErrorIs(t, CheckHookCaller(ctx, 99, project, "pre-commit", read), ErrNotFromGit)
	unsupported := func(int) (procinfo.Process, error) { return procinfo.Process{}, procinfo.ErrUnsupported }
	assert.ErrorIs(t, CheckHookCaller(ctx, 10, project, "pre-commit", unsupported), procinfo.ErrUnsupported)
}

// The real process tree of a hook run by git.
func TestCheckHookCallerRealGit(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("procinfo")
	}
	project := realDir(t)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", project, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	run("config", "core.hooksPath", filepath.Join(project, ".beads", "hooks"))
	hooks := filepath.Join(project, ".beads", "hooks")
	require.NoError(t, os.MkdirAll(hooks, 0o700))
	pidFile := filepath.Join(t.TempDir(), "pid")
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"),
		[]byte("#!/bin/sh\nsh -c 'echo $$ > "+pidFile+"; sleep 2'\n"), 0o755))
	cmd := exec.Command("git", "-C", project, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-qm", "x")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Wait() })
	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil
	}, 10*time.Second, 20*time.Millisecond)
	assert.NoError(t, CheckHookCaller(context.Background(), pid, project, "pre-commit", procinfo.Get))
	assert.ErrorIs(t, CheckHookCaller(context.Background(), os.Getpid(), project, "pre-commit", procinfo.Get), ErrNotFromGit)
}
