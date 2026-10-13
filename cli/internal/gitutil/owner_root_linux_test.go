//go:build linux

package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentUID is the account of the agents in the root tests (nobody).
const agentUID = 65534

// agentRepo returns, as root, a repository given to the agent account whose
// configuration runs a program git cannot be told not to run (a clean
// filter named by an attribute): it appends the uid it runs as to marker.
func agentRepo(t *testing.T) (repo, marker string, o Owner) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(root), 0o755)) // t.TempDir: 0700
	require.NoError(t, os.Chmod(root, 0o755))
	repo = filepath.Join(root, "work")
	home := filepath.Join(root, "home")
	shared := filepath.Join(root, "shared")
	for _, d := range []string{repo, home, shared} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.Chmod(shared, 0o777))
	marker = filepath.Join(shared, "ran-as")
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644))
	git("add", "a.txt")
	git("commit", "-qm", "init")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.txt filter=trap\n"), 0o644))
	git("config", "filter.trap.clean", "sh -c 'id -u >> "+marker+"; cat'")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("two\n"), 0o644))
	for _, d := range []string{repo, home} {
		require.NoError(t, filepath.Walk(d, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Lchown(p, agentUID, agentUID)
		}))
	}
	return repo, marker, Owner{UID: agentUID, GID: agentUID, Home: home}
}

// As root in a repository of the agent account (remote job): git refuses
// it as is, oh never lifts the refusal, and once the account is registered
// git runs as it — what the repository makes git run never runs as root.
func TestGitAsTheOwnerOfTheRepository(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root on Linux (remote job): go test -c, then the binary as root")
	}
	t.Cleanup(ResetOwners)
	repo, marker, o := agentRepo(t)
	ctx := context.Background()

	out, err := Command(ctx, repo, "status", "--porcelain").CombinedOutput()
	require.Error(t, err, "root in a repository of another owner: git refuses")
	assert.Contains(t, string(out), "dubious ownership")

	RegisterOwner(o)
	out, err = Command(ctx, repo, "status", "--porcelain").CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "a.txt")
	ran, err := os.ReadFile(marker)
	require.NoError(t, err, "the filter of the repository ran")
	for _, uid := range strings.Fields(string(ran)) {
		assert.Equal(t, "65534", uid, "never as root")
	}
	_, _, err = Fingerprint(ctx, repo)
	require.NoError(t, err, "the configuration check reads it too")
}
