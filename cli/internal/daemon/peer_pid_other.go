//go:build !darwin && !linux

package daemon

import "net"

func peerPID(net.Conn) (int, error) { return -1, errUnsupportedPeer }
