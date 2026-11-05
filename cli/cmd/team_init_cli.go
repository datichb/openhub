package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	teamInitCmd.Flags().Bool("reconfigure", false, i18n.T("v54a.init.flags.reconfigure"))
}

// teamToReconfigure is the team `oh team init --reconfigure` works on: the one
// --team names (the persistent flag of oh team), else the team the commands
// work on by default.
func teamToReconfigure(cmd *cobra.Command, a *app.App) (config.TeamConfig, error) {
	if f := cmd.Flags().Lookup("team"); f != nil && f.Value.String() != "" {
		t := a.Config.FindTeam(f.Value.String())
		if t == nil {
			return config.TeamConfig{}, fmt.Errorf("%s", i18n.Tf("cmd.teams.not_found", f.Value.String(), theme.Bold.Render("oh teams list")))
		}
		return *t, nil
	}
	return a.Config.ActiveTeam(), nil
}

// runTeamInit is `oh team init`: the assistant that creates a team on a new
// repository (teamAssistant), or, with --reconfigure, the one that sets the
// team that is open up again. Nothing is cloned, pulled or pushed before the
// first screen: the repository, the probe of the access and the clone are
// steps of the assistant.
func runTeamInit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	a := MustApp()

	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.init.hub_not_configured", theme.Bold.Render("oh init")))
	}

	var team *config.TeamConfig
	if reconfigure, _ := cmd.Flags().GetBool("reconfigure"); reconfigure {
		t, err := teamToReconfigure(cmd, a)
		if err != nil {
			return err
		}
		if !reconfigurable(t) {
			return fmt.Errorf("%s", i18n.Tf("v54a.init.nothing_to_reconfigure", theme.Bold.Render("oh team init"), theme.Bold.Render("oh team rejoin --repo <url>")))
		}
		team = &t
	}
	w := newTeamAssistant(ctx, a, team)

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Team Setup  "))
	fmt.Fprintln(a.IO.Out)
	intro := i18n.T("v54a.init.intro_create")
	if team != nil {
		intro = i18n.Tf("v54a.init.intro_reconfigure", team.ID)
	}
	fmt.Fprintln(a.IO.Out, theme.Box.Render(intro))
	fmt.Fprintln(a.IO.Out)

	res := runCLIWizard(a, cliWizard{Command: "team init", Steps: w.steps()})
	if res.Aborted {
		return nil
	}
	if res.Err != nil {
		return res.Err
	}
	if w.repo == nil { // every step skipped: nothing was set up
		return nil
	}
	printTeamInitSummary(a, w)
	return nil
}

// printTeamInitSummary tells what the assistant set up.
func printTeamInitSummary(a *app.App, w *teamAssistant) {
	tss := w.tss
	title := i18n.T("cmd.team.init.done")
	if tss.HasConfig || tss.HasPolicies {
		title = i18n.T("cmd.team.init.done_update")
	}
	fields := []summary.Field{
		{Label: "Repo", Value: maskRepoURL(w.stateRepo)},
		{Label: "Clone", Value: shortenPath(w.statePath)},
	}
	if tss.MemberID != "" {
		memberDesc := tss.MemberID
		if tss.DisplayName != "" {
			memberDesc = fmt.Sprintf("%s (%s)", tss.DisplayName, tss.Role)
		}
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.member"), Value: memberDesc})
	}
	if tss.StaleDaysStr != "" {
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.stale_days"), Value: tss.StaleDaysStr})
	}
	if tss.WebhookURL != "" {
		fields = append(fields, summary.Field{
			Label: i18n.T("cmd.team.init.recap.notifs"),
			Value: fmt.Sprintf("mattermost · %s", maskSecret(tss.WebhookURL)),
		})
	} else {
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.notifs"), Value: i18n.T("cmd.team.init.recap.notifs_none")})
	}
	if len(tss.SelectedPolicies) > 0 {
		fields = append(fields, summary.Field{
			Label: i18n.T("cmd.team.init.recap.policies"),
			Value: fmt.Sprintf("%s (%d)", strings.Join(tss.SelectedPolicies, ", "), len(tss.SelectedPolicies)),
		})
	}
	fmt.Fprint(a.IO.Out, summary.Render(summary.Config{
		Title:     title,
		Icon:      theme.IconSuccess,
		IconColor: theme.LipSuccess,
		Fields:    fields,
		Footer:    i18n.Tf("cmd.team.init.done_hint_status", "oh team status"),
	}))
	fmt.Fprintf(a.IO.Out, "  %s %s\n\n", theme.InfoStyle.Render(theme.IconInfo), i18n.T("cmd.team.tracker_hint"))
}
