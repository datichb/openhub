// Package peercheck connects to a Unix socket only after checking who is
// behind it (audit v5, S-28). The daemon refuses the connections of other
// users; this is the same check the other way round, for the clients that
// send it secrets (tokens, credentials): if the daemon is stopped, any
// process of the user could create the socket in its place and read what is
// sent to it.
//
// Before connecting, the directory of the socket and the socket itself must
// belong to the user and be closed to the others. After connecting, the
// process that listens must run as the user (SO_PEERCRED on Linux,
// LOCAL_PEERCRED on macOS). Where the system cannot tell (Windows, whose
// daemon runs inside oh), only what can be checked is.
//
// The package is small and free of the rest of oh: the fake bd of the
// container images uses it too.
package peercheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// Kind tells which check refused a socket.
type Kind string

const (
	DirNotDirectory Kind = "dir_not_directory" // the directory is a link or not a directory
	DirOwner        Kind = "dir_owner"         // the directory belongs to another user
	DirWritable     Kind = "dir_writable"      // the directory is writable by its group or the others
	NotSocket       Kind = "not_socket"        // the socket path is a link or not a socket
	SocketOwner     Kind = "socket_owner"      // the socket belongs to another user
	SocketMode      Kind = "socket_mode"       // the socket is open to its group or the others
	PeerUID         Kind = "peer_uid"          // the process listening runs as another user
)

// Error is the refusal of a socket that cannot be trusted.
type Error struct {
	Kind Kind
	Path string
	// Detail is a technical complement (owner found, mode…), English.
	Detail string
}

func (e *Error) Error() string {
	s := fmt.Sprintf("socket %s not trusted (%s)", e.Path, e.Kind)
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	return s
}

// ErrUnsupported: the system cannot tell which user runs the other end of a
// socket.
var ErrUnsupported = errors.New("peer credentials not supported")

// Dial connects to the Unix socket at path as the current user expects it:
// see the package documentation. A missing directory or socket is not a
// refusal: the connection fails as usual (nobody is listening).
func Dial(ctx context.Context, path string) (net.Conn, error) {
	return dialAs(ctx, path, os.Getuid())
}

func dialAs(ctx context.Context, path string, uid int) (net.Conn, error) {
	if uid >= 0 && runtime.GOOS != "windows" {
		if err := checkDir(filepath.Dir(path), uid); err != nil {
			return nil, err
		}
		if err := checkSocketFile(path, uid); err != nil {
			return nil, err
		}
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	if uid >= 0 && runtime.GOOS != "windows" {
		if err := checkPeer(c, path, uid); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

// checkDir: the directory of the socket is a real directory of the user,
// closed to the writes of the others (they could replace the socket).
func checkDir(dir string, uid int) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil // nobody is listening: the dial fails as usual
	}
	if !fi.IsDir() {
		return &Error{Kind: DirNotDirectory, Path: dir, Detail: fi.Mode().String()}
	}
	if owner, ok := FileOwner(fi); ok && owner != uid {
		return &Error{Kind: DirOwner, Path: dir, Detail: fmt.Sprintf("owner uid %d, want %d", owner, uid)}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return &Error{Kind: DirWritable, Path: dir, Detail: fmt.Sprintf("mode %04o", fi.Mode().Perm())}
	}
	return nil
}

// checkSocketFile: the path is a socket (not a link, not a file) of the
// user, closed to the group and the others.
func checkSocketFile(path string, uid int) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil // nobody is listening: the dial fails as usual
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return &Error{Kind: NotSocket, Path: path, Detail: fi.Mode().String()}
	}
	if owner, ok := FileOwner(fi); ok && owner != uid {
		return &Error{Kind: SocketOwner, Path: path, Detail: fmt.Sprintf("owner uid %d, want %d", owner, uid)}
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return &Error{Kind: SocketMode, Path: path, Detail: fmt.Sprintf("mode %04o", fi.Mode().Perm())}
	}
	return nil
}

// checkPeer: the process listening on the socket runs as the user.
func checkPeer(c net.Conn, path string, uid int) error {
	got, err := UID(c)
	if errors.Is(err, ErrUnsupported) {
		return nil
	}
	if err != nil {
		return &Error{Kind: PeerUID, Path: path, Detail: "peer credentials unreadable: " + err.Error()}
	}
	if got != uid {
		return &Error{Kind: PeerUID, Path: path, Detail: fmt.Sprintf("listener uid %d, want %d", got, uid)}
	}
	return nil
}
