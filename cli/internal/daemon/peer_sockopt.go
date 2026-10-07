//go:build darwin || linux

package daemon

import (
	"errors"
	"net"
)

// sockoptUID runs get on the file descriptor of a Unix connection.
func sockoptUID(c net.Conn, get func(fd int) (int, error)) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return -1, errors.New("not a unix socket connection")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return -1, err
	}
	uid, gerr := -1, error(nil)
	if err := raw.Control(func(fd uintptr) { uid, gerr = get(int(fd)) }); err != nil {
		return -1, err
	}
	return uid, gerr
}
