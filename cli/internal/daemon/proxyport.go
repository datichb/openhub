package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Proxy port changes. A tool server reaches the proxy (and the gateways on
// the same listeners: OH_GATEWAY_URL, remote oh MCP servers) at the URL
// written in its configuration when it started. When a restarted daemon
// cannot bind the previous port, the servers that still use it can no longer
// reach their provider: they are put to sleep right away (resumable: the
// resume starts a new server with the current URL), and the sessions that
// were working get an error decision.

// ProxyURLPath is the file recording the proxy URL a group's server was
// started with (~/.oh/servers/<group>/proxy_url), as seen by the server.
func ProxyURLPath(serversDir, group string) string {
	return filepath.Join(serversDir, group, "proxy_url")
}

// proxyPortRetry is how long a restarted daemon retries the previous proxy
// port (released by a daemon that just stopped).
var proxyPortRetry = 3 * time.Second

// proxyPorts returns the ports of all the proxy listeners.
func (d *Daemon) proxyPorts() map[int]bool {
	ports := map[int]bool{}
	if p := urlPort(d.proxy.URL()); p > 0 {
		ports[p] = true
	}
	for _, u := range d.proxy.Listeners() {
		if p := urlPort(u); p > 0 {
			ports[p] = true
		}
	}
	return ports
}

func urlPort(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(u.Port())
	return p
}

// staleProxyPort returns the proxy port a ready server uses when that port
// is no longer one of the proxy's (0 = up to date or unknown). Servers
// without a recorded URL (started by an older oh) are stale when they
// started before this daemon and its main port changed.
func (d *Daemon) staleProxyPort(srv domain.Server, ports map[int]bool) int {
	if d.opts.ServersDir != "" {
		if data, err := os.ReadFile(ProxyURLPath(d.opts.ServersDir, srv.GroupKey)); err == nil {
			p := urlPort(strings.TrimSpace(string(data)))
			if p > 0 && !ports[p] {
				return p
			}
			return 0
		}
	}
	if d.prevProxyPort > 0 && !ports[d.prevProxyPort] && srv.CreatedAt.Before(d.started) {
		return d.prevProxyPort
	}
	return 0
}

// sleepStaleGroups puts to sleep the ready groups whose server uses a proxy
// port the daemon no longer listens on.
func (d *Daemon) sleepStaleGroups(ctx context.Context, ready []domain.Server) []domain.Server {
	ports := d.proxyPorts()
	keep := ready[:0:0]
	for _, srv := range ready {
		old := d.staleProxyPort(srv, ports)
		if old == 0 {
			keep = append(keep, srv)
			continue
		}
		slog.Warn("ohd: server uses a previous proxy port, putting its group to sleep", "group", srv.GroupKey, "port", old, "proxy", d.proxy.URL())
		d.raisePortChanged(ctx, srv, old)
		d.sleepLocked(ctx, srv, false)
	}
	return keep
}

// raisePortChanged opens an error decision for the working sessions of a
// group whose proxy port changed.
func (d *Daemon) raisePortChanged(ctx context.Context, srv domain.Server, old int) {
	if d.opts.Decisions == nil || d.opts.Sessions == nil {
		return
	}
	sessions, err := d.opts.Sessions.List(ctx, srv.ProjectID)
	if err != nil {
		return
	}
	msg := i18n.Tf("cmd.daemon.proxy_port_changed", old, urlPort(d.proxy.URL()))
	for _, s := range sessions {
		if s.GroupKey != srv.GroupKey || (s.State != domain.RunActive && s.State != domain.RunWaiting) {
			continue
		}
		dec := domain.Decision{ID: domain.DecisionID(domain.DecisionError, s.ID, fmt.Sprintf("proxy-port-%d", old)),
			SessionID: s.ID, GroupKey: srv.GroupKey, Kind: domain.DecisionError, Payload: domain.DecisionPayload{Message: msg}}
		if err := d.opts.Decisions.Upsert(ctx, &dec); err != nil {
			slog.Debug("ohd: decision update failed", "decision", dec.ID, "error", err)
			continue
		}
		d.feed.publishChange(domain.SessionChange{SessionID: s.ID, GroupKey: srv.GroupKey, Decisions: true})
	}
}

// bindPreviousPort retries the previous proxy port for a short while.
func (d *Daemon) bindPreviousPort(port int) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(proxyPortRetry)
	for {
		if err := d.proxy.Start(addr); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
