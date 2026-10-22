package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A purge stops what oh itself tracks for this hub, and nothing else: the
// daemon (through its socket, under the hub's run folder) and the tool
// servers registered in the database (through their adapter, which proves
// the process is the tool server before signalling it). No process is looked
// up by name: other hubs (another OH_HOME), other users and unrelated
// programs whose command line says "oh serve" are never touched. What oh
// does not track is listed to be stopped by hand.

// purgeDaemon is the daemon of the hub (a *daemon.Client).
type purgeDaemon interface {
	Health(ctx context.Context) (daemon.Health, error)
	Shutdown(ctx context.Context, force bool) error
}

// purgeServers lists the tool servers registered in the database.
type purgeServers interface {
	List(ctx context.Context) ([]domain.Server, error)
}

// purgeProcesses is what a purge uses to stop the processes of the hub; each
// part may be nil (not available: the step says so).
type purgeProcesses struct {
	daemon  purgeDaemon                       // nil: no daemon to stop
	servers purgeServers                      // nil: database unreadable
	adapter func(string) adapters.ToolAdapter // nil adapter: tool not available
	alive   func(pid int) bool
	// daemonErr: why the daemon could not be reached (capability unreadable…).
	daemonErr error
}

// purgeProcessReport is the result of stopping the processes.
type purgeProcessReport struct {
	daemonStopped  bool
	serversStopped int
	manual         []string // what is left to stop by hand
}

const purgeDaemonWait = 5 * time.Second

// pidAlive tells whether a process exists (signal 0: nothing is sent).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// stopPurgeProcesses stops the daemon, then the tool servers, of this hub.
func stopPurgeProcesses(ctx context.Context, out io.Writer, p purgeProcesses) purgeProcessReport {
	var rep purgeProcessReport
	p.stopDaemon(ctx, &rep)
	p.stopServers(ctx, &rep)
	// Not tracked by oh: never searched by name.
	rep.manual = append(rep.manual, i18n.T("cmd.purge.processes.untracked"))

	var parts []string
	if rep.daemonStopped {
		parts = append(parts, i18n.T("cmd.purge.processes.daemon"))
	}
	if rep.serversStopped > 0 {
		parts = append(parts, i18n.Tf("cmd.purge.processes.servers", rep.serversStopped))
	}
	detail := i18n.T("cmd.purge.none_detected")
	if len(parts) > 0 {
		detail = strings.Join(parts, ", ")
	}
	printStepDone(out, detail)
	for _, m := range rep.manual {
		fmt.Fprintf(out, "      - %s\n", m)
	}
	return rep
}

// stopDaemon asks the daemon to stop (it is the only way: the process is
// never signalled) and waits until its socket stops answering.
func (p purgeProcesses) stopDaemon(ctx context.Context, rep *purgeProcessReport) {
	if p.daemonErr != nil {
		rep.manual = append(rep.manual, i18n.Tf("cmd.purge.processes.daemon_unreachable", p.daemonErr))
		return
	}
	if p.daemon == nil {
		return
	}
	if _, err := p.daemon.Health(ctx); errors.Is(err, daemon.ErrNotRunning) {
		return
	}
	err := p.daemon.Shutdown(ctx, true)
	if errors.Is(err, daemon.ErrNotRunning) {
		return
	}
	if err != nil {
		rep.manual = append(rep.manual, i18n.Tf("cmd.purge.processes.daemon_failed", err))
		return
	}
	wctx, cancel := context.WithTimeout(ctx, purgeDaemonWait)
	defer cancel()
	for {
		if _, err := p.daemon.Health(wctx); errors.Is(err, daemon.ErrNotRunning) {
			rep.daemonStopped = true
			return
		}
		select {
		case <-wctx.Done():
			rep.manual = append(rep.manual, i18n.T("cmd.purge.processes.daemon_still_running"))
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// stopServers stops the tool servers of the database that run on this
// machine. A server that does not answer with its own credentials is not
// signalled (its pid may belong to another program): it is listed.
func (p purgeProcesses) stopServers(ctx context.Context, rep *purgeProcessReport) {
	if p.servers == nil {
		rep.manual = append(rep.manual, i18n.T("cmd.purge.processes.servers_unknown"))
		return
	}
	servers, err := p.servers.List(ctx)
	if err != nil {
		rep.manual = append(rep.manual, i18n.T("cmd.purge.processes.servers_unknown"))
		return
	}
	for _, srv := range servers {
		if srv.Status != domain.ServerReady && srv.Status != domain.ServerStarting {
			continue
		}
		switch sessionspec.RuntimeKind(srv.Runtime) {
		case sessionspec.RuntimeContainer:
			rep.manual = append(rep.manual, i18n.Tf("cmd.purge.processes.container", srv.GroupKey))
			continue
		case sessionspec.RuntimeRemote:
			continue // no process on this machine
		}
		var ad adapters.ToolAdapter
		if p.adapter != nil {
			ad = p.adapter(srv.Adapter)
		}
		if ad == nil {
			rep.manual = append(rep.manual, i18n.Tf("cmd.purge.processes.no_adapter", srv.PID, srv.GroupKey, srv.Adapter))
			continue
		}
		h := adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}
		serr := ad.StopServer(ctx, h)
		if serr == nil && (p.alive == nil || !p.alive(srv.PID)) {
			rep.serversStopped++
			continue
		}
		rep.manual = append(rep.manual, i18n.Tf("cmd.purge.processes.not_stopped", srv.PID, srv.GroupKey))
	}
}
