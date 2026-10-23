package api

import (
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/peercheck"
)

// ErrNotRunning is returned when the daemon socket does not answer.
var ErrNotRunning = errors.New("ohd is not running")

// ErrUnsupported: no daemon can be started (in-process mode without an
// in-process starter).
var ErrUnsupported = errors.New("ohd cannot be started here")

// APIError is an error answer of the daemon.
type APIError struct {
	Method, Path string
	Status       int
	Message      string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ohd %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

// ErrUntrustedSocket is returned by the client when the daemon socket (or
// its folder) does not belong to the user, or when the process listening is
// not the user's (audit v5, S-28). Nothing was sent to it. Test it with
// errors.Is; the error is an *UntrustedSocketError.
var ErrUntrustedSocket = errors.New("ohd socket not trusted")

// UntrustedSocketError says which check refused the socket, in the language
// of the user.
type UntrustedSocketError struct {
	Kind   peercheck.Kind
	Path   string
	Detail string // technical complement, English
}

func (e *UntrustedSocketError) Error() string {
	switch e.Kind {
	case peercheck.DirNotDirectory:
		return i18n.Tf("daemon.socket.dir_not_directory", e.Path)
	case peercheck.DirOwner:
		return i18n.Tf("daemon.socket.dir_owner", e.Path)
	case peercheck.DirWritable:
		return i18n.Tf("daemon.socket.dir_writable", e.Path)
	case peercheck.NotSocket:
		return i18n.Tf("daemon.socket.not_socket", e.Path)
	case peercheck.SocketOwner:
		return i18n.Tf("daemon.socket.socket_owner", e.Path)
	case peercheck.SocketMode:
		return i18n.Tf("daemon.socket.socket_mode", e.Path)
	case peercheck.PeerUID:
		return i18n.Tf("daemon.socket.peer_uid", e.Path)
	}
	return i18n.Tf("daemon.socket.untrusted", e.Path)
}

// Is makes errors.Is(err, ErrUntrustedSocket) true.
func (e *UntrustedSocketError) Is(target error) bool { return target == ErrUntrustedSocket }
