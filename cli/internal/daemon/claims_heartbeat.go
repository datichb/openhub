package daemon

import (
	"context"
	"log/slog"
	"time"
)

// Signal of life of the team claims. The daemon knows nothing of the
// team-state: Options.OnClaimsHeartbeat does the work (collect the claims of
// the open sessions, renew their leases, empty the queue of claim
// operations) and the daemon only calls it, every ClaimsHeartbeatEvery, for
// as long as it runs. Like the tasks, it never keeps the daemon awake.

// runClaimsHeartbeat calls OnClaimsHeartbeat once the grants are restored,
// then every ClaimsHeartbeatEvery, until the daemon stops. Nothing runs when
// the callback is missing or the period is zero (heartbeat off).
func (d *Daemon) runClaimsHeartbeat(ctx context.Context) {
	beat, every := d.opts.OnClaimsHeartbeat, d.opts.ClaimsHeartbeatEvery
	if beat == nil || every <= 0 {
		return
	}
	select {
	case <-d.restored:
	case <-ctx.Done():
		return
	case <-d.stop:
		return
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		d.claimsBeat(ctx, beat)
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case <-tick.C:
		}
	}
}

// claimsBeat calls the callback; an error or a panic is logged, the loop
// goes on.
func (d *Daemon) claimsBeat(ctx context.Context, beat func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ohd: claims heartbeat panicked", "panic", r)
		}
	}()
	if err := beat(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Debug("ohd: claims heartbeat interrupted", "error", err)
			return
		}
		slog.Warn("ohd: claims heartbeat failed", "error", err)
	}
}
