package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › orphan tool servers (N18): servers started by oh (or by its
// tests) that nobody owns any more — interrupted tests, a discovery server
// left by a killed oh, a server of this hub that oh believes stopped, a
// server of a hub that was deleted. They keep memory until killed.

func init() { registerDoctorCheck(orphanServersCheck) }

// Reasons of an orphan server.
const (
	orphanTemp    = "temp"     // test or discovery server whose parent is gone
	orphanUnowned = "unowned"  // server of this hub that oh does not run any more
	orphanHubGone = "hub_gone" // server of an oh home that no longer exists
)

type orphanServer struct {
	adapters.ServerProcess
	Reason string
}

// classifyOrphans keeps the processes that belong to nobody. live are the
// PIDs of the servers this hub runs (nil: unknown, the servers of this hub
// are then left alone); exists tells whether a file exists, recent whether
// a log was written in the last minutes (a server being started).
func classifyOrphans(procs []adapters.ServerProcess, hubDir, tmpDir string, live map[int]bool, exists, recent func(string) bool) []orphanServer {
	hubServers := filepath.Join(resolvedPath(hubDir), "servers") + string(filepath.Separator)
	tmp := resolvedPath(tmpDir) + string(filepath.Separator)
	var out []orphanServer
	for _, p := range procs {
		work, logPath := resolvedPath(p.WorkDir), resolvedPath(p.LogPath)
		switch {
		case p.PPID == 1 && (isTempServerDir(work, tmp) || isTempServerDir(logPath, tmp)):
			out = append(out, orphanServer{p, orphanTemp})
		case logPath != "" && strings.HasPrefix(logPath, hubServers):
			if live != nil && !live[p.PID] && !recent(logPath) {
				out = append(out, orphanServer{p, orphanUnowned})
			}
		case logPath != "":
			// <oh home>/servers/<group>/data/<log>
			data := filepath.Dir(logPath)
			if filepath.Base(data) != "data" {
				continue
			}
			servers := filepath.Dir(filepath.Dir(data))
			if filepath.Base(servers) != "servers" {
				continue
			}
			if home := filepath.Dir(servers); !exists(filepath.Join(home, "oh.db")) {
				out = append(out, orphanServer{p, orphanHubGone})
			}
		}
	}
	return out
}

// isTempServerDir reports a directory of a test (t.TempDir: <tmp>/Test…) or
// of the native agents discovery (<tmp>/oh-natives-…).
func isTempServerDir(p, tmp string) bool {
	rest, ok := strings.CutPrefix(p, tmp)
	return ok && (strings.HasPrefix(rest, "Test") || strings.HasPrefix(rest, "oh-natives-"))
}

func resolvedPath(p string) string {
	if p == "" {
		return ""
	}
	// Resolve the closest existing ancestor (a deleted test directory, a
	// group whose data directory is gone), keep the rest as is.
	p = filepath.Clean(p)
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		if parent := filepath.Dir(dir); parent == dir {
			return p
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// serverLister is the capability of the preferred adapter (nil: none).
func serverLister() adapters.ServerProcessLister {
	l, _ := preferredAdapter().(adapters.ServerProcessLister)
	return l
}

// liveServerPIDs are the servers this hub runs (starting or ready); nil
// when the database cannot tell.
func liveServerPIDs(ctx context.Context) map[int]bool {
	if store == nil {
		return nil
	}
	servers, err := sqlite.NewServerStore(store).List(ctx)
	if err != nil {
		return nil
	}
	live := map[int]bool{}
	for _, s := range servers {
		if s.PID > 0 && (s.Status == domain.ServerStarting || s.Status == domain.ServerReady) {
			live[s.PID] = true
		}
	}
	return live
}

// recentlyWritten reports a file changed in the last two minutes.
func recentlyWritten(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && time.Since(fi.ModTime()) < 2*time.Minute
}

// findOrphanServers lists the orphan servers of the machine.
func findOrphanServers(ctx context.Context) ([]orphanServer, adapters.ServerProcessLister, error) {
	l := serverLister()
	if l == nil {
		return nil, nil, nil
	}
	procs, err := l.ServerProcesses(ctx)
	if err != nil {
		return nil, l, err
	}
	return classifyOrphans(procs, config.HubDir(), os.TempDir(), liveServerPIDs(ctx), fileExists, recentlyWritten), l, nil
}

func orphanServersCheck(ctx context.Context) []views.DoctorCheck {
	name := i18n.Tf("cmd.doctor.orphans.name", toolName())
	orphans, l, err := findOrphanServers(ctx)
	switch {
	case l == nil:
		return nil
	case err != nil:
		return []views.DoctorCheck{{Name: name, OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.orphans.unknown", err)}}
	case len(orphans) == 0:
		return []views.DoctorCheck{{Name: name, OK: true, Detail: i18n.T("cmd.doctor.orphans.none")}}
	}
	return []views.DoctorCheck{{Name: name, Detail: i18n.Tf("cmd.doctor.orphans.found", len(orphans), describeOrphans(orphans, 3))}}
}

func describeOrphans(orphans []orphanServer, limit int) string {
	parts := make([]string, 0, len(orphans))
	for i, o := range orphans {
		if i == limit {
			parts = append(parts, i18n.Tf("cmd.doctor.orphans.more", len(orphans)-limit))
			break
		}
		where := o.LogPath
		if where == "" {
			where = o.WorkDir
		}
		parts = append(parts, i18n.Tf("cmd.doctor.orphans.item", o.PID, i18n.T("cmd.doctor.orphans.reason."+o.Reason), where))
	}
	return strings.Join(parts, "; ")
}

// fixOrphanServers stops the orphan servers (oh doctor --fix), after a
// confirmation (--yes without a terminal).
func fixOrphanServers(cmd *cobra.Command) error {
	ctx := cmd.Context()
	orphans, l, err := findOrphanServers(ctx)
	if err != nil || len(orphans) == 0 {
		return nil //nolint:nilerr // listing failed: the check reports it
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, theme.Bold.Render(i18n.Tf("cmd.doctor.orphans.plan", toolName())))
	for _, o := range orphans {
		fmt.Fprintf(w, "  %s\n", describeOrphans([]orphanServer{o}, 1))
	}
	if yes, _ := cmd.Flags().GetBool("yes"); !yes {
		if !stdinIsTerminal() {
			return errors.New(i18n.T("cmd.doctor.fix.needs_yes"))
		}
		ok, err := cliConfirm(cmd.ErrOrStderr(), i18n.T("cmd.doctor.orphans.confirm"))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
			return nil
		}
	}
	stopped := 0
	for _, o := range orphans {
		sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := l.StopServerProcess(sctx, o.PID); err == nil {
			stopped++
		}
		cancel()
	}
	fmt.Fprintf(w, "%s %s\n\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.doctor.orphans.stopped", stopped))
	return nil
}
