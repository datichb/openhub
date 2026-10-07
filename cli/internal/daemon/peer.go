package daemon

import (
	"errors"
	"log/slog"
	"net"
	"os"
)

var errUnsupportedPeer = errors.New("peer credentials not supported")

// peerListener accepts only the connections of processes running as the
// daemon's user (M12): the socket mode (0600) is checked again on each
// connection, by the kernel's peer credentials.
type peerListener struct {
	net.Listener
}

func (l peerListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		uid, err := peerUID(c)
		if errors.Is(err, errUnsupportedPeer) || (err == nil && uid == os.Getuid()) {
			return c, nil
		}
		slog.Warn("ohd: connection refused (peer of another user)", "uid", uid, "error", err)
		_ = c.Close()
	}
}
