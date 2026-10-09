package hostnet

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// S-25 (audit v5): only loopback and container engine bridges.
func TestCheckListenHost(t *testing.T) {
	ifaces := func() ([]Iface, error) {
		return []Iface{
			{Name: "lo", Addrs: []net.IP{net.ParseIP("127.0.0.1")}},
			{Name: "eth0", Addrs: []net.IP{net.ParseIP("192.168.1.20")}},
			{Name: "wg0", Addrs: []net.IP{net.ParseIP("10.8.0.2")}},
			{Name: "docker0", Addrs: []net.IP{net.ParseIP("172.17.0.1")}},
			{Name: "br-1a2b3c", Addrs: []net.IP{net.ParseIP("172.18.0.1")}},
			{Name: "podman0", Addrs: []net.IP{net.ParseIP("10.88.0.1")}},
		}, nil
	}
	for _, ok := range []string{"127.0.0.1", "127.0.0.2", "::1", "172.17.0.1", "172.18.0.1", "10.88.0.1"} {
		assert.NoError(t, check(ok, ifaces), ok)
	}
	for _, bad := range []string{"192.168.1.20", "10.8.0.2", "172.17.0.2", "10.0.0.9", "0.0.0.0", "evil.example", ""} {
		assert.ErrorIs(t, check(bad, ifaces), ErrNotEngineInterface, bad)
	}
	assert.ErrorIs(t, check("172.17.0.1", func() ([]Iface, error) { return nil, errors.New("boom") }), ErrNotEngineInterface)
	assert.NoError(t, CheckListenHost("127.0.0.1"))
}
