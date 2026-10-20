//go:build unix

package daemon

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// squatter listens on the socket path of paths, as a process that took the
// place of a stopped daemon, and records what it receives.
type squatter struct {
	got   chan []byte
	conns chan struct{}
}

func squat(t *testing.T, paths Paths) *squatter {
	t.Helper()
	l, err := net.Listen("unix", paths.Socket())
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	s := &squatter{got: make(chan []byte, 8), conns: make(chan struct{}, 8)}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			s.conns <- struct{}{}
			go func() {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				b, _ := io.ReadAll(c)
				s.got <- b
			}()
		}
	}()
	return s
}

// S-28: the client sends nothing to a socket that is not the user's: a
// secret (capability, group token, grant) would be read by whoever took the
// place of the daemon.
func TestClientRefusesAnUntrustedSocket(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, p Paths)
		kind  string
	}{
		{"socket open to the others", func(t *testing.T, p Paths) {
			require.NoError(t, os.Chmod(p.Socket(), 0o666))
		}, "socket_mode"},
		{"folder writable by the others", func(t *testing.T, p Paths) {
			require.NoError(t, os.Chmod(p.Socket(), 0o600))
			require.NoError(t, os.Chmod(p.Dir, 0o777))
		}, "dir_writable"},
		{"link instead of the socket", func(t *testing.T, p Paths) {
			real := p.Socket() + ".real"
			require.NoError(t, os.Rename(p.Socket(), real))
			require.NoError(t, os.Symlink(real, p.Socket()))
		}, "not_socket"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := shortPaths(t)
			require.NoError(t, os.Chmod(p.Dir, 0o700))
			sq := squat(t, p)
			tc.setup(t, p)

			c := NewClient(p).WithCapability("the-capability").WithGroupToken("the-group-token")
			_, err := c.Health(context.Background())
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUntrustedSocket)
			assert.NotErrorIs(t, err, ErrNotRunning)
			var ue *UntrustedSocketError
			require.ErrorAs(t, err, &ue)
			assert.Contains(t, ue.Path, "ohd")
			assert.NotEmpty(t, err.Error())

			err = c.RevokeOwner(context.Background(), "g1")
			assert.ErrorIs(t, err, ErrUntrustedSocket, "every request is checked, not only the first")
			_, err = c.Stream(context.Background(), "")
			assert.ErrorIs(t, err, ErrUntrustedSocket)

			select {
			case <-sq.conns:
				t.Fatal("the client connected to the untrusted socket")
			case b := <-sq.got:
				t.Fatalf("the squatter received %q", b)
			default:
			}
		})
	}
}

// The checks do not break the usual case: a daemon of the user is reached.
func TestClientReachesTheDaemonOfTheUser(t *testing.T) {
	e := newEnv(t)
	c, _ := e.start(t, nil, time.Hour)
	_, err := c.Health(context.Background())
	require.NoError(t, err)
}

// A socket nobody listens on is still "not running".
func TestClientWithoutSocketIsNotRunning(t *testing.T) {
	p := shortPaths(t)
	_, err := NewClient(p).Health(context.Background())
	assert.ErrorIs(t, err, ErrNotRunning)
	assert.NotErrorIs(t, err, ErrUntrustedSocket)
}

// Ensure does not start a daemon over a socket it refuses: the error says
// why instead of a timeout.
func TestEnsureReportsAnUntrustedSocket(t *testing.T) {
	p := shortPaths(t)
	require.NoError(t, os.Chmod(p.Dir, 0o700))
	squat(t, p)
	require.NoError(t, os.Chmod(p.Socket(), 0o666))
	_, _, err := Ensure(context.Background(), p, EnsureOptions{Executable: "/nonexistent/oh", Timeout: 50 * time.Millisecond})
	assert.ErrorIs(t, err, ErrUntrustedSocket)
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
