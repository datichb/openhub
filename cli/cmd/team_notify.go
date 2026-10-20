package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamNotifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Gestion des notifications d'équipe",
}

var teamNotifyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Envoie une notification de test au webhook configuré",
	Long: `Envoie un message de test au(x) webhook(s) configuré(s) dans le team-state.
Utile pour vérifier que la configuration des notifications fonctionne.

Exemples:
  oh team notify test
  oh team notify test --message "Déploiement en cours"`,
	RunE: runTeamNotifyTest,
}

func init() {
	teamCmd.AddCommand(teamNotifyCmd)
	teamNotifyCmd.AddCommand(teamNotifyTestCmd)
	teamNotifyTestCmd.Flags().StringP("message", "m", "", "Message personnalisé (défaut: message de test standard)")
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
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team_notify.sync_failed", theme.WarningStyle.Render(theme.IconWarning), err))
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team_notify.cannot_load_the_team_config"), err)
	}

	if !teamCfg.Notification.Enabled {
		return errors.New(i18n.T("cmd.team_notify.notifications_are_disabled_in_the_team"))
	}

	// Build test message
	message, _ := cmd.Flags().GetString("message")
	if message == "" {
		message = "Test de notification OpenHub — webhook fonctionnel !"
	}

	// Resolve member ID for the actor field
	memberID := "unknown"
	if tc := a.Config.ActiveTeam(); tc.MemberID != "" {
		memberID = tc.MemberID
	}

	event := teamstate.Event{
		Type:      "custom.notification",
		Actor:     memberID,
		Project:   "openhub",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"message": message},
	}

	d := notify.NewDispatcher(teamCfg)
	if err := d.Dispatch(ctx, event); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s Erreur: %s\n",
			theme.ErrorStyle.Render("✗"), err)
		return err
	}

	// Report which backends were reached
	notifyType := teamCfg.Notification.Type
	if notifyType == "" {
		notifyType = "webhook"
	}
	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_notify.notification_sent_via", theme.SuccessStyle.Render("✓"), notifyType))

	return nil
}
