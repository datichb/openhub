//go:build unix

package fakebd

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/peercheck"
)

// S-28: the bd of a local session does not hand its gateway token to a
// socket that is not the user's daemon (a process that took its place).
func TestHookTokenRefusesAnUntrustedSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "bd-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	require.NoError(t, os.Chmod(dir, 0o700))
	sock := filepath.Join(dir, "ohd.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	accepted := make(chan struct{}, 1)
	go func() {
		if c, err := l.Accept(); err == nil {
			accepted <- struct{}{}
			c.Close()
		}
	}()
	require.NoError(t, os.Chmod(sock, 0o666)) // open to the others

	_, err = hookToken(envOf(map[string]string{beadswire.EnvDaemonSocket: sock, beadswire.EnvToken: "ohg_secret"}), []string{"hooks", "run", "post-merge"})
	var pe *peercheck.Error
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, peercheck.SocketMode, pe.Kind)
	select {
	case <-accepted:
		t.Fatal("connected to the untrusted socket")
	default:
	}
}
