package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

func TestHookTokenRoute(t *testing.T) {
	assert.Equal(t, beadswire.HookTokenPath, hookTokenRoute)
}

// S-06 (audit v5): the hook mode of the gateway needs a hook token that the
// daemon gives only to a fake bd that git runs from the hook of the
// session; BD_GIT_HOOK=1 set by an agent opens nothing.
func TestBeadsGitHookToken(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("process tree check: macOS and Linux")
	}
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	sqlitetest.Seed(t, st).Project("p1", project)
	sessions, servers := sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "x", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))

	git := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = project, append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	_, err = git(nil, "init", "-q")
	require.NoError(t, err)
	for _, kv := range [][2]string{{"user.name", "t"}, {"user.email", "t@t"}, {"core.hooksPath", filepath.Join(project, ".beads", "hooks")}} {
		_, err = git(nil, "config", kv[0], kv[1])
		require.NoError(t, err)
	}
	hooks, bin := filepath.Join(project, ".beads", "hooks"), t.TempDir()
	require.NoError(t, os.MkdirAll(hooks, 0o700))
	// The hook of bd 1.3 (simplified) and the bd of local sessions.
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexport BD_GIT_HOOK=1\nbd hooks run pre-commit \"$@\"\n"), 0o755))
	self, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "bd"), []byte("#!/bin/sh\nOH_DAEMON_TEST_FAKEBD=1 exec "+self+" \"$@\"\n"), 0o755))
	marker := filepath.Join(t.TempDir(), "ran")
	realBD := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(realBD, []byte(fakeWhere+"echo \"$* $BD_GIT_HOOK\" >> "+marker+"\n"), 0o755))

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Tick: time.Hour, IdleAfter: time.Hour, BeadsBinary: realBD})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	h, err := c.Health(ctx)
	require.NoError(t, err)
	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_a", GroupKey: "g1", ProjectID: "p1", Location: project, BeadsAllow: []string{"show"}})
	require.NoError(t, err)
	env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		beadswire.EnvURL + "=" + h.ProxyURL + "/" + beadswire.Prefix, beadswire.EnvToken + "=" + g.Token,
		beadswire.EnvDaemonSocket + "=" + p.Socket()}
	ran := func() string { data, _ := os.ReadFile(marker); return string(data) }

	out, err := git(env, "commit", "--allow-empty", "-qm", "x")
	require.NoError(t, err, out)
	assert.Contains(t, ran(), "hooks run -- pre-commit 1", "run by git: the hook ran, with BD_GIT_HOOK for the real bd")

	require.NoError(t, os.Remove(marker))
	cmd := exec.Command(filepath.Join(bin, "bd"), "hooks", "run", "pre-commit")
	cmd.Dir, cmd.Env = project, append(os.Environ(), append(env, "BD_GIT_HOOK=1")...)
	out2, err := cmd.CombinedOutput()
	assert.Error(t, err, "BD_GIT_HOOK set by the caller, not run by git")
	assert.Contains(t, string(out2), "hook token")
	assert.Empty(t, ran(), "nothing ran")

	// A hook run by git with arguments git does not pass is not a hook.
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexport BD_GIT_HOOK=1\nbd hooks run pre-commit --extra \"$@\"\n"), 0o755))
	out, err = git(env, "commit", "--allow-empty", "-qm", "y")
	assert.Error(t, err, out)
	assert.Empty(t, ran())
	assert.True(t, strings.Contains(out, "not allowed") || strings.Contains(out, "refused") || strings.Contains(out, "unknown"), out)
}
