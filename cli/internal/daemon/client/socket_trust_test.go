//go:build unix

package client

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
)

// shortPaths returns runtime paths in a short folder (Unix socket paths are
// limited to ~104 bytes).
func shortPaths(t *testing.T) api.Paths {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ohd-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return api.Paths{Dir: dir}
}

// squatter listens on the socket path of paths, as a process that took the
// place of a stopped daemon, and records what it receives.
type squatter struct {
	got   chan []byte
	conns chan struct{}
}

func squat(t *testing.T, paths api.Paths) *squatter {
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
		setup func(t *testing.T, p api.Paths)
		kind  string
	}{
		{"socket open to the others", func(t *testing.T, p api.Paths) {
			require.NoError(t, os.Chmod(p.Socket(), 0o666))
		}, "socket_mode"},
		{"folder writable by the others", func(t *testing.T, p api.Paths) {
			require.NoError(t, os.Chmod(p.Socket(), 0o600))
			require.NoError(t, os.Chmod(p.Dir, 0o777))
		}, "dir_writable"},
		{"link instead of the socket", func(t *testing.T, p api.Paths) {
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
			assert.ErrorIs(t, err, api.ErrUntrustedSocket)
			assert.NotErrorIs(t, err, api.ErrNotRunning)
			var ue *api.UntrustedSocketError
			require.ErrorAs(t, err, &ue)
			assert.Contains(t, ue.Path, "ohd")
			assert.NotEmpty(t, err.Error())

			err = c.RevokeOwner(context.Background(), "g1")
			assert.ErrorIs(t, err, api.ErrUntrustedSocket, "every request is checked, not only the first")
			_, err = c.Stream(context.Background(), "")
			assert.ErrorIs(t, err, api.ErrUntrustedSocket)

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

// A socket nobody listens on is still "not running".
func TestClientWithoutSocketIsNotRunning(t *testing.T) {
	p := shortPaths(t)
	_, err := NewClient(p).Health(context.Background())
	assert.ErrorIs(t, err, api.ErrNotRunning)
	assert.NotErrorIs(t, err, api.ErrUntrustedSocket)
}

// Ensure does not start a daemon over a socket it refuses: the error says
// why instead of a timeout.
func TestEnsureReportsAnUntrustedSocket(t *testing.T) {
	p := shortPaths(t)
	require.NoError(t, os.Chmod(p.Dir, 0o700))
	squat(t, p)
	require.NoError(t, os.Chmod(p.Socket(), 0o666))
	_, _, err := Ensure(context.Background(), p, EnsureOptions{Executable: "/nonexistent/oh", Timeout: 50 * time.Millisecond})
	assert.ErrorIs(t, err, api.ErrUntrustedSocket)
}
