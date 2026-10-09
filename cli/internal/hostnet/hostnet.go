// Package hostnet tells which addresses of the machine the oh daemon may
// listen on for containers (audit v5, S-25): loopback, or an address of a
// virtual interface of a container engine (Docker, Podman, CNI, libvirt,
// LXC/Incus bridges). An address of the local network, a VPN or any other
// interface is refused, whoever asks for it (an image can answer any
// address for host.docker.internal).
package hostnet

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// ErrNotEngineInterface: the address is not loopback nor on a virtual
// interface of a container engine.
var ErrNotEngineInterface = errors.New("not an address of a container engine interface")

// Iface is a network interface and its addresses.
type Iface struct {
	Name  string
	Addrs []net.IP
}

// engineIfacePrefixes are the names of the bridges created by the
// container engines on Linux.
var engineIfacePrefixes = []string{
	"docker", "br-", "podman", "cni", "virbr", "lxcbr", "lxdbr", "incusbr", "nerdctl", "cali", "flannel", "kube-bridge", "weave",
}

// EngineInterface reports whether name is a container engine bridge.
func EngineInterface(name string) bool {
	for _, p := range engineIfacePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// CheckListenHost accepts a loopback address or an address of a container
// engine interface of this machine.
func CheckListenHost(host string) error { return check(host, systemIfaces) }

func check(host string, list func() ([]Iface, error)) error {
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() {
		return fmt.Errorf("%w: %q is not an IP address", ErrNotEngineInterface, host)
	}
	if ip.IsLoopback() {
		return nil
	}
	ifaces, err := list()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotEngineInterface, err)
	}
	for _, i := range ifaces {
		for _, a := range i.Addrs {
			if a.Equal(ip) {
				if EngineInterface(i.Name) {
					return nil
				}
				return fmt.Errorf("%w: %s is on %s", ErrNotEngineInterface, ip, i.Name)
			}
		}
	}
	return fmt.Errorf("%w: %s is not an address of this machine", ErrNotEngineInterface, ip)
}

func systemIfaces() ([]Iface, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]Iface, 0, len(list))
	for _, i := range list {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		f := Iface{Name: i.Name}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				f.Addrs = append(f.Addrs, n.IP)
			}
		}
		out = append(out, f)
	}
	return out, nil
}
