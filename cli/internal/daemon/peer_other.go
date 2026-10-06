//go:build !darwin && !linux

package daemon

import "net"

// peerUID is not available on this system: connections are accepted on the
// socket file mode alone (0600).
func peerUID(net.Conn) (int, error) { return -1, errUnsupportedPeer }
