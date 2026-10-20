//go:build darwin

package peercheck

import (
	"net"

	"golang.org/x/sys/unix"
)

// UID returns the user id of the process at the other end of a Unix socket
// connection (LOCAL_PEERCRED: for a connecting socket, the credentials the
// listening process had).
func UID(c net.Conn) (int, error) {
	return sockoptUID(c, func(fd int) (int, error) {
		cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			return -1, err
		}
		return int(cred.Uid), nil
	})
}
