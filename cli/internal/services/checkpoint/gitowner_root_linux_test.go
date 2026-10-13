//go:build linux

package checkpoint

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

	"github.com/datichb/openhub/cli/internal/gitutil"
)

// As root in a clone of the agent account (remote job), the commit window,
// the closing guard and the fingerprint of the work read git as that
// account; unregistered, git refuses and every one of them fails closed.
func TestGitReadsAsTheAgentAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root on Linux (remote job): go test -c, then the binary as root")
	}
	t.Cleanup(gitutil.ResetOwners)
	root := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(root), 0o755)) // t.TempDir: 0700
	require.NoError(t, os.Chmod(root, 0o755))
	repo, home := filepath.Join(root, "work"), filepath.Join(root, "home")
	require.NoError(t, os.MkdirAll(repo, 0o755))
	require.NoError(t, os.MkdirAll(home, 0o755))
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "safe.directory=" + repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o644))
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a // x\n"), 0o644))
	git("commit", "-qam", "work")
	for _, d := range []string{repo, home} {
		require.NoError(t, filepath.Walk(d, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Lchown(p, 65534, 65534)
		}))
	}
	ctx := context.Background()

	_, _, err := GitHead(ctx, repo)
	require.Error(t, err, "unregistered: git refuses, the commit window closes")
	_, err = GitWork(ctx, repo, base, time.Now())
	require.Error(t, err, "unregistered: the closing guard refuses")
	assert.True(t, unreadable(gitFingerprint(ctx, repo, nil)), "unregistered: no verification tied to it")

	acct := &Account{UID: 65534, GID: 65534, Home: home}
	gitutil.RegisterOwner(acct.Owner())
	head, dirty, err := GitHead(ctx, repo)
	require.NoError(t, err)
	assert.Len(t, head, 40)
	assert.False(t, dirty)
	w, err := GitWork(ctx, repo, base, time.Now())
	require.NoError(t, err)
	assert.True(t, w.Committed, "the closing guard sees the commit")
	assert.False(t, unreadable(gitFingerprint(ctx, repo, nil)))
	paths, err := gitChanged(ctx, repo, base, acct)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go"}, paths)
}
