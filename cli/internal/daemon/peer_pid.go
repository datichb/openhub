//go:build darwin || linux

package daemon

import (
	"errors"
	"net"
)

// peerPID returns the process at the other end of a Unix socket connection.
func peerPID(c net.Conn) (int, error) {
	if c == nil {
		return -1, errors.New("no connection")
	}
	return sockoptUID(c, peerPIDOpt)
}
