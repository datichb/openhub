//go:build unix

package peercheck

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// socketIn listens on a socket in a fresh private directory (short path:
// Unix socket paths are limited to ~104 bytes) and returns the directory,
// the socket and the connections the listener accepted.
func socketIn(t *testing.T) (dir, sock string, accepted <-chan net.Conn) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pc-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	require.NoError(t, os.Chmod(dir, 0o700))
	sock = filepath.Join(dir, "s.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	require.NoError(t, os.Chmod(sock, 0o600))
	ch := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			ch <- c
		}
	}()
	return dir, sock, ch
}

func wantKind(t *testing.T, err error, kind Kind) {
	t.Helper()
	var pe *Error
	require.ErrorAs(t, err, &pe, "a refusal")
	assert.Equal(t, kind, pe.Kind, pe.Error())
}

func TestDialTrustedSocket(t *testing.T) {
	_, sock, accepted := socketIn(t)
	c, err := Dial(context.Background(), sock)
	require.NoError(t, err)
	defer c.Close()
	srv := <-accepted
	defer srv.Close()
	_, err = c.Write([]byte("hi"))
	require.NoError(t, err)
	buf := make([]byte, 2)
	_, err = srv.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hi", string(buf))
}

func TestDialMissingSocketFailsAsUsual(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "pc-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	require.NoError(t, os.Chmod(dir, 0o700))
	_, err = Dial(context.Background(), filepath.Join(dir, "none.sock"))
	var opErr *net.OpError
	require.ErrorAs(t, err, &opErr, "the callers read this as \"daemon not running\"")
	var pe *Error
	assert.False(t, errors.As(err, &pe))

	_, err = Dial(context.Background(), filepath.Join(dir, "no-such-dir", "none.sock"))
	require.ErrorAs(t, err, &opErr)
}

func TestDialRefusesADirectoryOpenToTheOthers(t *testing.T) {
	for _, mode := range []os.FileMode{0o770, 0o707, 0o777} {
		dir, sock, accepted := socketIn(t)
		require.NoError(t, os.Chmod(dir, mode))
		_, err := Dial(context.Background(), sock)
		wantKind(t, err, DirWritable)
		select {
		case <-accepted:
			t.Fatalf("mode %o: connected anyway", mode)
		default:
		}
		require.NoError(t, os.Chmod(dir, 0o700))
	}
}

// Reading the directory is no harm: only the writes allow to replace the socket.
func TestDialAcceptsAReadableDirectory(t *testing.T) {
	dir, sock, _ := socketIn(t)
	require.NoError(t, os.Chmod(dir, 0o755))
	c, err := Dial(context.Background(), sock)
	require.NoError(t, err)
	c.Close()
}

func TestDialRefusesASocketOpenToTheOthers(t *testing.T) {
	for _, mode := range []os.FileMode{0o660, 0o606, 0o666, 0o700 | 0o004} {
		_, sock, _ := socketIn(t)
		require.NoError(t, os.Chmod(sock, mode))
		_, err := Dial(context.Background(), sock)
		wantKind(t, err, SocketMode)
	}
}

func TestDialRefusesALinkOrAFileInsteadOfTheSocket(t *testing.T) {
	dir, sock, _ := socketIn(t)
	link := filepath.Join(dir, "link.sock")
	require.NoError(t, os.Symlink(sock, link))
	_, err := Dial(context.Background(), link)
	wantKind(t, err, NotSocket)

	file := filepath.Join(dir, "file.sock")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	_, err = Dial(context.Background(), file)
	wantKind(t, err, NotSocket)
}

func TestDialRefusesALinkedDirectory(t *testing.T) {
	dir, sock, _ := socketIn(t)
	parent, err := os.MkdirTemp("/tmp", "pc-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(parent) })
	link := filepath.Join(parent, "run")
	require.NoError(t, os.Symlink(dir, link))
	_, err = Dial(context.Background(), filepath.Join(link, filepath.Base(sock)))
	wantKind(t, err, DirNotDirectory)
}

// The owner checks, as if the user were someone else: the files below belong
// to this user, who is not uid+1.
func TestChecksRefuseAnotherOwner(t *testing.T) {
	uid := os.Getuid()
	dir, sock, _ := socketIn(t)

	wantKind(t, checkDir(dir, uid+1), DirOwner)
	assert.NoError(t, checkDir(dir, uid))
	wantKind(t, checkSocketFile(sock, uid+1), SocketOwner)
	assert.NoError(t, checkSocketFile(sock, uid))

	_, err := dialAs(context.Background(), sock, uid+1)
	wantKind(t, err, DirOwner)
}

// The process listening must run as the user, whatever the socket file says:
// this is what stops a socket created by someone else in a directory the user
// shares (or a file replaced between the check and the connection).
func TestPeerCheckRefusesAnotherListenerUser(t *testing.T) {
	_, sock, accepted := socketIn(t)
	var d net.Dialer
	c, err := d.DialContext(context.Background(), "unix", sock)
	require.NoError(t, err)
	defer c.Close()
	defer func() { (<-accepted).Close() }()

	if _, err := UID(c); errors.Is(err, ErrUnsupported) {
		t.Skip("peer credentials not supported here")
	}
	uid := os.Getuid()
	got, err := UID(c)
	require.NoError(t, err)
	assert.Equal(t, uid, got, "the listener is this process")
	assert.NoError(t, checkPeer(c, sock, uid))
	wantKind(t, checkPeer(c, sock, uid+1), PeerUID)
}
