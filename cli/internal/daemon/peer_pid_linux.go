//go:build linux

package daemon

import "golang.org/x/sys/unix"

func peerPIDOpt(fd int) (int, error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return -1, err
	}
	return int(cred.Pid), nil
}
