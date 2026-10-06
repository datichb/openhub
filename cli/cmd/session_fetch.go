package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/sysnotify"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

func init() {
	cmd := &cobra.Command{
		Use:   "fetch <session-id>",
		Short: i18n.T("cmd.session.fetch.short"),
		Long:  i18n.T("cmd.session.fetch.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionFetch,
	}
	cmd.Flags().Bool("no-import", false, i18n.T("cmd.session.fetch.flags.no_import"))
	sessionCmd.AddCommand(cmd)
}

// newRemoteReturnService wires the remote service for following, fetching
// and replaying remote sessions.
func newRemoteReturnService(ctx context.Context, a *app.App) *remotesvc.Service {
	svc := newRemoteSendService(a)
	svc.SessionsDir, svc.BundlesDir = ohSessionsDir(), ohBundlesDir()
	svc.Target = func(name string) (*config.RemoteTarget, bool) {
		t := a.Config.Remote.Target(name)
		return t, t != nil
	}
	svc.Worktree = worktree.ResolveOrCreate
	svc.Adopt = func(ctx context.Context, sid string, trs [][]byte, location string) error {
		rs, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		sess, err := a.Sessions.Get(ctx, sid)
		if err != nil {
			return err
		}
		project, err := a.Projects.Get(ctx, sess.ProjectID)
		if err != nil {
			return err
		}
		req := v5Request(a, project, "")
		req.Location = location
		return rs.AdoptSession(ctx, sid, trs, req)
	}
	if a.Config.Session.NotifyEnabled() {
		n := sysnotify.New()
		svc.Notify = func(title, status string) {
			_ = n.Notify(ctx, sysnotify.Note{Title: i18n.T("tui.remote.notify.title"),
				Message: i18n.Tf("tui.remote.notify."+status, title), Group: "oh-sessions", Sound: true})
		}
	}
	return svc
}

// trackRemoteSessions refreshes the remote sessions still running (quiet:
// a GitLab outage must not break the session list).
func trackRemoteSessions(ctx context.Context, a *app.App) {
	if len(a.Config.Remote.Targets) == 0 || store == nil {
		return
	}
	if _, err := newRemoteReturnService(ctx, a).Track(ctx); err != nil {
		slog.Debug("remote sessions not refreshed", "error", err)
	}
}

func runSessionFetch(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()
	out := cmd.OutOrStdout()
	id, err := resolveSessionRef(ctx, args[0])
	if err != nil {
		return err
	}
	noImport, _ := cmd.Flags().GetBool("no-import")
	fmt.Fprintln(out, i18n.Tf("cmd.session.fetch.running", id))
	res, err := newRemoteReturnService(ctx, a).Fetch(ctx, id, remotesvc.FetchOptions{NoImport: noImport})
	switch {
	case errors.Is(err, remotesvc.ErrNotFinished):
		return errors.New(i18n.T("cmd.session.fetch.not_finished"))
	case errors.Is(err, remotesvc.ErrNotRemote):
		return errors.New(i18n.T("cmd.session.fetch.not_remote"))
	case errors.Is(err, remotesvc.ErrNoArtifacts):
		return fmt.Errorf("%s (%w)", i18n.T("cmd.session.fetch.no_artifacts"), err)
	case err != nil:
		return err
	}
	s := res.Summary
	ok := theme.SuccessStyle.Render(theme.IconSuccess)
	fmt.Fprintf(out, "%s %s\n", ok, i18n.Tf("cmd.session.fetch.summary", i18n.T("cmd.session.fetch.outcome."+s.Outcome), s.Cost, len(res.Journal)))
	if s.Error != "" {
		fmt.Fprintf(out, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), s.Error)
	}
	if s.MRURL != "" {
		fmt.Fprintf(out, "  %s %s\n", theme.IconArrow, i18n.Tf("cmd.session.fetch.mr", s.MRURL))
	}
	if s.Deferred != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.IconArrow, i18n.Tf("cmd.session.fetch.deferred", s.Deferred.ID, firstNonEmpty(s.Deferred.Label, s.Deferred.ID)))
	}
	if s.Question != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.IconArrow, i18n.Tf("cmd.session.fetch.question", s.Question.Label))
	}
	for _, d := range s.Decisions {
		if d.Answer == "rejected" {
			fmt.Fprintf(out, "  %s %s\n", theme.IconDot, i18n.Tf("cmd.session.fetch.rejected", d.Label, fmt.Sprint(d.Resources)))
		}
	}
	if res.Imported {
		fmt.Fprintf(out, "%s %s\n", ok, i18n.Tf("cmd.session.fetch.imported", res.Location, id))
	}
	if len(res.Journal) > 0 {
		fmt.Fprintf(out, "  %s %s\n", theme.IconArrow, i18n.Tf("cmd.session.fetch.resolve_hint", len(res.Journal), id))
	}
	return nil
}
