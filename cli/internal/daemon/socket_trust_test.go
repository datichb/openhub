//go:build unix

package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The checks do not break the usual case: a daemon of the user is reached.
func TestClientReachesTheDaemonOfTheUser(t *testing.T) {
	e := newEnv(t)
	c, _ := e.start(t, nil, time.Hour)
	_, err := c.Health(context.Background())
	require.NoError(t, err)
}

// S-28: the daemon works in a folder only the user can enter before it
// creates the socket in it.
func TestSecurePrivateDir(t *testing.T) {
	base := t.TempDir()
	open := filepath.Join(base, "open")
	require.NoError(t, os.Mkdir(open, 0o777))
	require.NoError(t, os.Chmod(open, 0o777))
	require.NoError(t, securePrivateDir(open))
	fi, err := os.Stat(open)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm(), "closed before the socket exists")

	private := filepath.Join(base, "private")
	require.NoError(t, os.Mkdir(private, 0o700))
	require.NoError(t, securePrivateDir(private))

	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(private, link))
	assert.Error(t, securePrivateDir(link), "a link is not followed")

	file := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	assert.Error(t, securePrivateDir(file))

	assert.True(t, errors.Is(securePrivateDir(filepath.Join(base, "missing")), os.ErrNotExist))
}

// S-28: whatever the folder was, the running daemon has a private folder and
// a socket reachable by the user only.
func TestDaemonSocketIsPrivate(t *testing.T) {
	e := newEnv(t)
	require.NoError(t, os.Chmod(e.paths.Dir, 0o777))
	e.start(t, nil, time.Hour)

	di, err := os.Stat(e.paths.Dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())
	si, err := os.Stat(e.paths.Socket())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), si.Mode().Perm())
	assert.NotZero(t, si.Mode()&os.ModeSocket)
}
