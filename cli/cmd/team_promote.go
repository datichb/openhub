package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// `oh team promote --remote <url>`: share a solo team-state (v5 phase 2,
// P2-T10). The team keeps its id, folder and history: attached projects
// and published workflows are unchanged.
func init() {
	cmd := &cobra.Command{
		Use:   "promote",
		Short: i18n.T("cmd.team.promote.short"),
		Long:  i18n.T("cmd.team.promote.long"),
		// teamPreRunE skips "promote": the solo team is never the active team.
		RunE: runTeamPromote,
	}
	cmd.Flags().String("remote", "", i18n.T("cmd.team.promote.flags.remote"))
	cmd.Flags().String("team", "", i18n.T("cmd.team.promote.flags.team"))
	_ = cmd.MarkFlagRequired("remote")
	teamCmd.AddCommand(cmd)
}

func runTeamPromote(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	remote, _ := cmd.Flags().GetString("remote")
	teamID, _ := cmd.Flags().GetString("team")
	team, err := promoteSoloTeam(cmd.Context(), a, teamID, remote)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.team.promote.done", team.ID, remote))
	fmt.Fprintf(out, "  %s\n", i18n.T("cmd.team.promote.invite"))
	fmt.Fprintf(out, "    %s\n", theme.Bold.Render("oh team init"))
	fmt.Fprintf(out, "  %s\n", i18n.Tf("cmd.team.promote.invite_repo", remote))
	return nil
}

// promoteSoloTeam pushes the solo team-state to remote and turns its hub.toml
// entry into a regular team.
func promoteSoloTeam(ctx context.Context, a *app.App, teamID, remote string) (*config.TeamConfig, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil, fmt.Errorf("%s", i18n.T("cmd.team.promote.remote_required"))
	}
	team, err := findSoloTeam(a.Config, teamID)
	if err != nil {
		return nil, err
	}
	if other := a.Config.FindTeamByRepo(remote); other != nil {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.promote.remote_used", remote, other.ID))
	}
	repo := teamstate.NewRepo("", team.StatePath)
	if err := repo.Promote(ctx, remote); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Tf("cmd.team.promote.failed", remote), err)
	}
	team.Solo = false
	team.StateRepo = remote
	if err := config.Save(a.Config); err != nil {
		return nil, fmt.Errorf("writing hub.toml: %w", err)
	}
	return team, nil
}

// findSoloTeam returns the solo team id, or the only solo team when id is
// empty.
func findSoloTeam(cfg *config.Config, id string) (*config.TeamConfig, error) {
	if id != "" {
		t := cfg.FindTeam(id)
		if t == nil {
			return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.promote.unknown_team", id))
		}
		if !t.Solo {
			return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.promote.not_solo", id))
		}
		return t, nil
	}
	var found *config.TeamConfig
	var ids []string
	for i := range cfg.Teams {
		if cfg.Teams[i].Solo {
			found = &cfg.Teams[i]
			ids = append(ids, cfg.Teams[i].ID)
		}
	}
	switch len(ids) {
	case 0:
		return nil, fmt.Errorf("%s", i18n.T("cmd.team.promote.no_solo"))
	case 1:
		return found, nil
	}
	return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.promote.ambiguous", strings.Join(ids, ", ")))
}
