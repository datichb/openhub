//go:build !darwin && !linux

package peercheck

import "net"

// UID is not available on this system.
func UID(net.Conn) (int, error) { return -1, ErrUnsupported }
