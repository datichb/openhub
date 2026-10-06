package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Proxy traffic ledger: the usage the proxy counts per server group goes to
// the usage store (flushed at each supervision and at shutdown), and a grant
// of a group (restored after a restart, or issued for a restarted server)
// continues the group's counters.

func (d *Daemon) onProxyUsage(owner string, delta credproxy.Usage) {
	if d.opts.Usage == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.proxyUsage == nil {
		d.proxyUsage = map[string]domain.ProxyUsage{}
	}
	u := d.proxyUsage[owner]
	u.Requests += delta.Requests
	u.TokensIn += delta.InputTokens
	u.TokensOut += delta.OutputTokens
	d.proxyUsage[owner] = u
}

// flushProxyUsage writes the accumulated proxy traffic.
func (d *Daemon) flushProxyUsage(ctx context.Context) {
	if d.opts.Usage == nil {
		return
	}
	d.mu.Lock()
	pending := d.proxyUsage
	d.proxyUsage = nil
	d.mu.Unlock()
	day := domain.UsageDay(time.Now())
	for group, u := range pending {
		if err := d.opts.Usage.AddProxy(ctx, group, day, u); err != nil {
			slog.Debug("ohd: proxy usage not recorded", "group", group, "error", err)
			d.onProxyUsage(group, credproxy.Usage{Requests: u.Requests, InputTokens: u.TokensIn, OutputTokens: u.TokensOut})
		}
	}
}

// continueProxyUsage starts a grant of a group from the group's counters.
func (d *Daemon) continueProxyUsage(ctx context.Context, group, hash string) {
	if d.opts.Usage == nil {
		return
	}
	tot, err := d.opts.Usage.ProxyTotal(ctx, group)
	if err != nil {
		return
	}
	d.mu.Lock()
	p := d.proxyUsage[group]
	d.mu.Unlock()
	d.proxy.SetUsage(hash, credproxy.Usage{Requests: tot.Requests + p.Requests,
		InputTokens: tot.TokensIn + p.TokensIn, OutputTokens: tot.TokensOut + p.TokensOut})
}
