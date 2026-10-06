//go:build darwin

package daemon

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUID returns the user id of the process at the other end of a Unix
// socket connection (getpeereid, LOCAL_PEERCRED).
func peerUID(c net.Conn) (int, error) {
	return sockoptUID(c, func(fd int) (int, error) {
		cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			return -1, err
		}
		return int(cred.Uid), nil
	})
}
