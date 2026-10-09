//go:build linux

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// As root on Linux (job conditions): the processes of the agent account are
// killed, and a trap planted by the agent in a work tree it owns does not
// run during Finish. Skipped when not root (run it in a container:
// `go test -c` then the binary as root).
func TestFinishAsRootWithAgentAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	agent := &User{UID: 10001, GID: 10001}
	f := newGitFixture(t)
	require.NoError(t, os.Chmod(f.root, 0o755))
	ctx := context.Background()
	g := Git{Token: "project-secret", Scratch: filepath.Join(f.root, "private"), Agent: agent}
	work := filepath.Join(f.root, "work")
	base, err := g.Clone(ctx, f.origin, "main", "", "feat/bd-42", work)
	require.NoError(t, err)
	require.NoError(t, chownTree(work, agent))

	// A background process of the agent, and its traps.
	sleeper := exec.Command("sleep", "300")
	runAs(sleeper, agent)
	require.NoError(t, sleeper.Start())
	done := make(chan error, 1)
	go func() { done <- sleeper.Wait() }()
	marker := filepath.Join(f.root, "pwned")
	trap := filepath.Join(work, "trap.sh")
	require.NoError(t, os.WriteFile(trap, []byte("#!/bin/sh\nid > "+marker+"\n"), 0o755))
	f.git(work, "config", "core.fsmonitor", trap)
	require.NoError(t, os.WriteFile(filepath.Join(work, ".git", "hooks", "post-commit"), []byte("#!/bin/sh\nid > "+marker+"\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, "csv.go"), []byte("package x"), 0o644))
	// A hard link to a file of root (fs.protected_hardlinks off or not).
	secret := filepath.Join(f.root, "secret")
	require.NoError(t, os.WriteFile(secret, []byte("root only"), 0o600))
	require.NoError(t, chownTree(work, agent))
	require.NoError(t, os.Link(secret, filepath.Join(work, "leak.txt")))

	require.NoError(t, KillUser(agent))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("agent process still running")
	}
	assert.Empty(t, userPIDs("/proc", agent.UID))

	res, err := g.Finish(ctx, f.origin, work, base, "main", "feat/bd-42", "oh: ticket bd-42", "")
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "a trap of the agent ran as root")
	files := f.git(f.origin, "ls-tree", "-r", "--name-only", "feat/bd-42")
	assert.Contains(t, files, "csv.go")
	assert.NotContains(t, files, "leak.txt", "a file of root is never committed")
	assert.Len(t, res.Commit, 40)
	var st syscall.Stat_t
	require.NoError(t, syscall.Stat(work, &st))
	assert.Equal(t, agent.UID, st.Uid, "work tree untouched")
}
