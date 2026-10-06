package cmd

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Daemon hooks for work that is not a local server (lot R6).

// daemonTeardown removes the runtime environment of a server group stopped
// by the daemon (container), like runsvc.Service.stopServer.
func daemonTeardown(a *app.App) func(ctx context.Context, srv domain.Server) {
	return func(ctx context.Context, srv domain.Server) {
		rt := v5Runtimes(a)[sessionspec.RuntimeKind(srv.Runtime)]
		if rt == nil {
			return
		}
		pg, err := rt.Load(ctx, ohruntime.Group{GroupID: srv.GroupKey, DataDir: srv.DataDir})
		if err != nil {
			return
		}
		if err := rt.Teardown(ctx, pg); err != nil {
			slog.Debug("ohd: runtime not torn down", "group", srv.GroupKey, "error", err)
		}
	}
}

// daemonRemoteTracking follows the remote sessions while oh is closed
// (BL-17): pipeline status (and the « ready to fetch » notification) and
// the Beads leases of their tickets. Busy while a remote session runs.
func daemonRemoteTracking(a *app.App) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		if store == nil {
			return false
		}
		rs := sqlite.NewRemoteStore(store)
		if !anyPendingRemote(ctx, rs) {
			return false
		}
		tctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, err := newRemoteReturnService(tctx, a).Track(tctx); err != nil {
			slog.Debug("ohd: remote tracking", "error", err)
		}
		refs, err := rs.ListRemote(ctx)
		if err != nil {
			return false
		}
		busy := false
		for _, ref := range refs {
			if !remotesvc.Pending(ref) {
				continue
			}
			busy = true
			heartbeatTickets(ctx, ref)
		}
		return busy
	}
}

func anyPendingRemote(ctx context.Context, rs domain.RemoteStore) bool {
	refs, err := rs.ListRemote(ctx)
	if err != nil {
		return false
	}
	for _, ref := range refs {
		if remotesvc.Pending(ref) {
			return true
		}
	}
	return false
}

// heartbeatTickets refreshes the Beads leases of the tickets a remote
// session reserved on this machine (`bd heartbeat`), so that they are not
// reclaimed while the pipeline runs.
func heartbeatTickets(ctx context.Context, ref domain.RemoteRef) {
	if ref.ProjectDir == "" || len(ref.Tickets) == 0 {
		return
	}
	bd, err := exec.LookPath("bd")
	if err != nil {
		return
	}
	for _, t := range ref.Tickets {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(cctx, bd, "heartbeat", t)
		cmd.Dir = ref.ProjectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Debug("ohd: Beads lease not refreshed", "ticket", t, "error", err, "output", strings.TrimSpace(string(out)))
		}
		cancel()
	}
}
