//go:build !windows

package gitutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asRoot makes OwnerOf see oh as root and every directory owned by uid.
func asRoot(t *testing.T, uid uint32) {
	t.Helper()
	euid, owner := geteuid, fileOwner
	geteuid = func() int { return 0 }
	fileOwner = func(string) (uint32, bool) { return uid, true }
	t.Cleanup(func() { geteuid, fileOwner = euid, owner; ResetOwners() })
}

// R3-remote-git: as root, git runs as the registered owner of the
// repository, with its folders and nothing of the configuration of oh.
func TestCommandRunsAsTheOwner(t *testing.T) {
	asRoot(t, 4242)
	dir := t.TempDir()

	cmd := Command(context.Background(), dir, "status")
	assert.Nil(t, cmd.SysProcAttr, "unregistered owner: git runs as oh (and refuses on its own)")

	RegisterOwner(Owner{UID: 4242, GID: 4343, Home: "/home/oh"})
	cmd = Command(context.Background(), dir, "status")
	require.NotNil(t, cmd.SysProcAttr)
	require.NotNil(t, cmd.SysProcAttr.Credential)
	assert.Equal(t, uint32(4242), cmd.SysProcAttr.Credential.Uid)
	assert.Equal(t, uint32(4343), cmd.SysProcAttr.Credential.Gid)
	assert.Empty(t, cmd.SysProcAttr.Credential.Groups, "no supplementary group of oh")
	assert.Equal(t, dir, cmd.Dir)
	assert.Contains(t, cmd.Env, "HOME=/home/oh")
	assert.Contains(t, cmd.Env, "GIT_TERMINAL_PROMPT=0", "still the safe git of oh")
	for _, kv := range cmd.Env {
		assert.False(t, strings.HasPrefix(kv, "HOME=") && kv != "HOME=/home/oh", kv)
	}
	args := strings.Join(cmd.Args, " ")
	assert.Contains(t, args, "core.fsmonitor=false")
	assert.Contains(t, args, "core.hooksPath="+os.DevNull)
	assert.NotContains(t, args, "safe.directory", "the refusal of git is never lifted")
	assert.Equal(t, 1, strings.Count(args, "credential.helper="), "no credential helper of oh")

	// A repository of root, or oh not root: git runs as oh.
	fileOwner = func(string) (uint32, bool) { return 0, true }
	assert.Nil(t, Command(context.Background(), dir, "status").SysProcAttr)
	fileOwner = func(string) (uint32, bool) { return 4242, true }
	geteuid = func() int { return 1000 }
	assert.Nil(t, Command(context.Background(), dir, "status").SysProcAttr)
	_, ok := OwnerOf("")
	assert.False(t, ok)
}

func TestLooksLikeRepository(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	assert.False(t, LooksLikeRepository(sub))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: x"), 0o644))
	assert.True(t, LooksLikeRepository(sub), "a .git above, file or directory")
	assert.False(t, LooksLikeRepository(""))
}

// A FIFO an agent puts in place of a hook never blocks the configuration
// check of oh.
func TestFingerprintDoesNotBlockOnFIFO(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	require.NoError(t, syscall.Mkfifo(filepath.Join(repo, ".git", "hooks", "pre-commit"), 0o644))
	done := make(chan error, 1)
	go func() { _, _, err := Fingerprint(context.Background(), repo); done <- err }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the fingerprint blocked on a FIFO")
	}
}
