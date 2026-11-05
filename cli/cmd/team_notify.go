package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamNotifyCmd = &cobra.Command{
	Use: "notify",
}

var teamNotifyTestCmd = &cobra.Command{
	Use:  "test",
	RunE: runTeamNotifyTest,
}

func init() {
	teamCmd.AddCommand(teamNotifyCmd)
	teamNotifyCmd.AddCommand(teamNotifyTestCmd)
	teamNotifyTestCmd.Flags().StringP("message", "m", "", "")
}

func runTeamNotifyTest(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo
	if repo == nil {
		return errors.New(i18n.T("cmd.team_notify.team_state_not_configured_run_oh"))
	}

	// Pull latest config
	if err := repo.Pull(ctx); err != nil {
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team_notify.sync_failed", cliout.WarnIcon(), err))
	}

	teamCfg, err := repo.LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team_notify.cannot_load_the_team_config"), err)
	}

	if !teamCfg.Notification.Enabled {
		return errors.New(i18n.T("cmd.team_notify.notifications_are_disabled_in_the_team"))
	}

	// Build test message
	message, _ := cmd.Flags().GetString("message")
	if message == "" {
		message = i18n.T("v54n.notify.test_message")
	}

	// Resolve member ID for the actor field
	memberID := "unknown"
	if m := commandMember(ctx, a); m != "" {
		memberID = m
	}

	event := teamstate.Event{
		Type:      teamstate.EventCustomNotification,
		Actor:     memberID,
		Project:   "openhub",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"message": message},
	}

	// No outbox: a test says at once whether the webhook answers. It goes to
	// every destination, whatever events each one chose.
	teamCfg.Notification.Events = nil
	for i := range teamCfg.Notification.Destinations {
		teamCfg.Notification.Destinations[i].Events = nil
	}
	d := notify.NewDispatcher(teamCfg)
	if !d.Enabled() {
		return errors.New(i18n.T("v54n.notify.test_no_destination"))
	}
	if rep := d.Send(ctx, event); rep.Err != nil || rep.Sent == 0 {
		err := rep.Err
		if err == nil {
			err = errors.New(i18n.T("v54n.notify.test_no_destination"))
		}
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team_notify.error", theme.ErrorStyle.Render(glyph.Failure), err))
		return err
	}

	// Report which backends were reached
	notifyType := teamCfg.Notification.Type
	if notifyType == "" {
		notifyType = "webhook"
	}
	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_notify.notification_sent_via", cliout.SuccessIcon(), notifyType))

	return nil
}
