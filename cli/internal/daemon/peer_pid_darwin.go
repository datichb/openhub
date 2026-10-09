//go:build darwin

package daemon

import "golang.org/x/sys/unix"

func peerPIDOpt(fd int) (int, error) {
	return unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
}
