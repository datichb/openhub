package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gc"
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

type orphanServer = gc.Orphan

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
	return gc.ClassifyOrphans(procs, config.HubDir(), os.TempDir(), liveServerPIDs(ctx), fileExists, gc.RecentlyWritten), l, nil
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
