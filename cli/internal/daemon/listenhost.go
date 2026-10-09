package daemon

import (
	"net"

	"github.com/datichb/openhub/cli/internal/hostnet"
)

// listenHost checks an additional listen host of the proxy (audit v5,
// S-25): loopback or an address of a container engine bridge of this
// machine, never an address of the local network or a VPN.
var listenHost = func(host string) (string, error) {
	if err := hostnet.CheckListenHost(host); err != nil {
		return "", err
	}
	return net.ParseIP(host).String(), nil
}
