package daemon

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
)

// Fan-out driver started again by the daemon (p6-debts). The driver of a
// fan-out (`oh fanout drive`) runs in its own process: it ends when the
// parent ends before its merge checkpoint (step over, session completed),
// and nothing would drive the children, the merge or a fix round once the
// parent goes on. The daemon starts it again when the parent's activity
// changes (step started or ended, reconnection) and its fan-out is not
// over: one driver at a time (the driver's lock, held while it runs), at
// most once per driverRetry, traced by the driver in the fan-out events.

// driverRetry spaces the starts of a driver for one session.
var driverRetry = time.Minute

// ensureFanoutDriver starts the driver of the fan-out of session id when
// it should run and does not.
func (d *Daemon) ensureFanoutDriver(ctx context.Context, id string) {
	if d.opts.FanoutDriver == nil || d.opts.Sessions == nil || d.opts.SessionsDir == "" {
		return
	}
	s, err := d.opts.Sessions.Get(ctx, id)
	if err != nil || !needsDriver(s) {
		return
	}
	d.driverMu.Lock()
	if d.drivers == nil {
		d.drivers = map[string]time.Time{}
	}
	if at, ok := d.drivers[id]; ok && time.Since(at) < driverRetry {
		d.driverMu.Unlock()
		return
	}
	d.drivers[id] = time.Now()
	d.driverMu.Unlock()
	unlock, err := filelock.TryLock(filepath.Join(d.opts.SessionsDir, id, "fanout.lock"))
	if err != nil {
		return // a driver runs
	}
	unlock()
	if err := d.opts.FanoutDriver(ctx, id); err != nil {
		slog.Warn("ohd: fan-out driver not started", "session", id, "error", err)
		return
	}
	slog.Info("ohd: fan-out driver started again", "session", id)
}

// needsDriver reports whether the fan-out of a parent is not over: its
// driver has work left (children, merge checkpoint, fix round).
func needsDriver(s *domain.Session) bool {
	f := s.Fanout
	if f == nil || f.Role != domain.RoleParent || !f.Experimental {
		return false
	}
	if s.State == domain.RunStopped || s.State == domain.RunFailed {
		return false
	}
	switch f.Status {
	case domain.FanoutWaiting, domain.FanoutRunning, domain.FanoutReady, domain.FanoutFixing:
		return true
	}
	return false
}
