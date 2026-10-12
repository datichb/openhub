package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway"
)

// Beads leases of local sessions. A claim (bd update --claim) carries a
// lease that bd 1.3.1 lets expire after 5 minutes; `bd reclaim` then gives
// the ticket back to the pool, while the session still works on it. The
// daemon refreshes (bd heartbeat) the leases of the tickets each open
// session claimed through the gateway, or that its workflow state lists as
// claimed and not closed, until the session ends.

// leaseEvery is well below the 5-minute lease of bd.
const leaseEvery = time.Minute

// trackLease remembers the tickets a session claims, and forgets those it
// closes.
func (d *Daemon) trackLease(g gateway.Grant, op gateway.BeadsOp) {
	if len(op.IDs) == 0 || (op.Kind != gateway.OpClaim && op.Kind != gateway.OpClose) {
		return
	}
	d.leaseMu.Lock()
	defer d.leaseMu.Unlock()
	if d.leases == nil {
		d.leases = map[string]map[string]bool{}
	}
	ids := d.leases[g.SessionID]
	if ids == nil {
		ids = map[string]bool{}
		d.leases[g.SessionID] = ids
	}
	for _, id := range op.IDs {
		if op.Kind == gateway.OpClaim {
			ids[id] = true
		} else {
			delete(ids, id)
		}
	}
}

// runLeases refreshes the leases every leaseEvery, once the grants are
// restored.
func (d *Daemon) runLeases(ctx context.Context) {
	select {
	case <-d.restored:
	case <-ctx.Done():
		return
	}
	t := time.NewTicker(leaseEvery)
	defer t.Stop()
	for {
		d.refreshLeases(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// refreshLeases refreshes the leases of every open session with a grant.
func (d *Daemon) refreshLeases(ctx context.Context) {
	gw, st := d.beadsGW, d.gateway
	if gw == nil || st == nil || d.opts.Sessions == nil {
		return
	}
	for _, g := range st.Roots() {
		s, err := d.opts.Sessions.Get(ctx, g.SessionID)
		if err != nil || s == nil || isTerminal(s.State) {
			d.leaseMu.Lock()
			delete(d.leases, g.SessionID)
			d.leaseMu.Unlock()
			continue
		}
		for _, id := range d.leasedTickets(ctx, g.SessionID) {
			if err := gw.Heartbeat(ctx, g, id); err != nil {
				// Closed, reclaimed or claimed by someone else: stop.
				slog.Debug("ohd: Beads lease not refreshed", "session", g.SessionID, "ticket", id, "error", err)
				d.trackLease(g, gateway.BeadsOp{Kind: gateway.OpClose, IDs: []string{id}})
			}
		}
	}
}

// leasedTickets are the tickets a session holds: claimed through the
// gateway since the daemon started, and claimed and not closed in its
// workflow state (which survives a daemon restart).
func (d *Daemon) leasedTickets(ctx context.Context, session string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	d.leaseMu.Lock()
	for id := range d.leases[session] {
		add(id)
	}
	d.leaseMu.Unlock()
	if cp := d.opts.Checkpoints; cp != nil {
		if state, err := cp.State(ctx, session); err == nil {
			closed := map[string]bool{}
			for _, id := range state.Closed {
				closed[id] = true
			}
			for _, id := range state.Claimed {
				if !closed[id] {
					add(id)
				}
			}
		}
	}
	return out
}
