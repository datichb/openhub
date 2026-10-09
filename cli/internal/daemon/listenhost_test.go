package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/hostnet"
)

// lanAddress is an IPv4 address of this machine outside loopback and the
// container engine bridges ("" when there is none).
func lanAddress(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	require.NoError(t, err)
	for _, i := range ifaces {
		if hostnet.EngineInterface(i.Name) || i.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() {
				return n.IP.String()
			}
		}
	}
	return ""
}

// S-25 (audit v5): the proxy listens for containers only on loopback and
// the container engine bridges, also when restoring ohd.json.
func TestProxyListenerOnlyEngineInterfaces(t *testing.T) {
	lan := lanAddress(t)
	if lan == "" {
		t.Skip("no local network address")
	}
	e := newEnv(t)
	data, err := json.Marshal(stateFile{ListenHosts: []string{lan, "10.255.255.254"}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(e.paths.Dir, 0o700))
	require.NoError(t, os.WriteFile(e.paths.State(), data, 0o600))
	ctx := context.Background()
	c, _ := e.start(t, nil, time.Hour)

	_, err = c.ProxyListen(ctx, lan)
	assert.Error(t, err, "an address of the local network")
	_, err = c.ProxyListen(ctx, "10.255.255.254")
	assert.Error(t, err, "not an address of this machine")

	h, err := c.Health(ctx)
	require.NoError(t, err)
	u, err := url.Parse(h.ProxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(lan, u.Port()), time.Second)
	if err == nil {
		conn.Close()
	}
	assert.Error(t, err, "the listener of ohd.json on the local network is not restored")
}
