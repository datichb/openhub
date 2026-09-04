package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

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
		return fmt.Errorf("team-state non configuré — lancez 'oh team init'")
	}

	// Pull latest config
	if err := repo.Pull(ctx); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s sync échouée: %s\n",
			theme.WarningStyle.Render(theme.IconWarning), err)
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return fmt.Errorf("impossible de charger la config team: %w", err)
	}

	if !teamCfg.Notification.Enabled {
		return fmt.Errorf("notifications désactivées dans la config team — configurez-les via le TUI (Team Detail > Notifications) ou 'oh team init'")
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
	fmt.Fprintf(a.IO.Out, "%s Notification envoyée via %s\n",
		theme.SuccessStyle.Render("✓"), notifyType)

	return nil
}
