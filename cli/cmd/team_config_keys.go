package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// oh team config get|set|unset|show: the settings of the config.toml of the
// team, one by one, by their dotted key (tracker.type, claim.lease_hours,
// notification.enabled…), without a terminal. A value is parsed with the type
// of its key and bounded; a change is a publication (a commit, a config_changed
// event signed by the member) and the settings that bind the others
// (*_enforced, [limits.enforced], [governance]) are for the leads.

var teamConfigGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: i18n.T("v54c.team_config.get.short"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamConfigGet,
}

var teamConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: i18n.T("v54c.team_config.set.short"),
	Long:  i18n.T("v54c.team_config.set.long"),
	Args:  cobra.ExactArgs(2),
	RunE:  runTeamConfigSet,
}

var teamConfigUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: i18n.T("v54c.team_config.unset.short"),
	Long:  i18n.T("v54c.team_config.unset.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamConfigUnset,
}

var teamConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: i18n.T("v54c.team_config.show.short"),
	Args:  cobra.NoArgs,
	RunE:  runTeamConfigShow,
}

func init() {
	teamConfigCmd.AddCommand(teamConfigGetCmd, teamConfigSetCmd, teamConfigUnsetCmd, teamConfigShowCmd)
	teamConfigShowCmd.Flags().Bool("origin", false, i18n.T("v54c.team_config.show.flag_origin"))
}

// teamConfigRepo resolves the team of the command and its clone.
// teamConfigAccess is the clone of the team as these commands use it.
type teamConfigAccess interface {
	teamstate.ConfigReader
	teamstate.ConfigWriter
	ConfigKeysSet() (map[string]bool, error)
}

func teamConfigRepo(ctx context.Context, a *app.App) (teamConfigAccess, string, context.Context, error) {
	team, err := ResolveTeam(ctx, a)
	if err != nil {
		return nil, "", ctx, err
	}
	if !team.Enabled {
		return nil, "", ctx, errors.New(i18n.Tf("cmd.team_config.team_not_configured_run_first", cliout.WarnIcon(), theme.Bold.Render("oh team init")))
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		return nil, "", ctx, errors.New(i18n.Tf("cmd.team_config.team_state_not_cloned_run_first", cliout.WarnIcon(), theme.Bold.Render("oh team rejoin --repo "+team.StateRepo)))
	}
	return repo, team.TeamID, teamstate.WithActor(ctx, team.MemberID), nil
}

func runTeamConfigGet(cmd *cobra.Command, args []string) error {
	a := MustApp()
	repo, _, ctx, err := teamConfigRepo(ctxOf(cmd), a)
	if err != nil {
		return err
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil {
		return errors.New(i18n.Tf("v533b.team_config.unreadable", err))
	}
	val, err := cfg.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(a.IO.Out, teamstate.SettingText(args[0], val))
	return nil
}

func runTeamConfigSet(cmd *cobra.Command, args []string) error {
	return changeTeamSetting(cmd, args[0], func(cfg *teamstate.TeamConfig) error { return cfg.Set(args[0], args[1]) },
		func(a *app.App) {
			cliout.Success(a.IO.Out, i18n.Tf("v54c.team_config.set.done", theme.Bold.Render(args[0]), teamstate.SettingText(args[0], args[1])))
		})
}

func runTeamConfigUnset(cmd *cobra.Command, args []string) error {
	return changeTeamSetting(cmd, args[0], func(cfg *teamstate.TeamConfig) error { return cfg.Unset(args[0]) },
		func(a *app.App) {
			cliout.Success(a.IO.Out, i18n.Tf("v54c.team_config.unset.done", theme.Bold.Render(args[0])))
		})
}

// changeTeamSetting publishes one change of the config of the team.
func changeTeamSetting(cmd *cobra.Command, key string, change func(*teamstate.TeamConfig) error, done func(*app.App)) error {
	a := MustApp()
	repo, _, ctx, err := teamConfigRepo(ctxOf(cmd), a)
	if err != nil {
		return err
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil {
		return errors.New(i18n.Tf("v533b.team_config.unreadable", err))
	}
	// the key is checked on a copy first: an unknown key or a value of the wrong
	// type is said before the team-state is pulled
	if err := change(teamstate.CloneConfig(cfg)); err != nil {
		return err
	}
	if err := repo.UpdateConfig(ctx, change); err != nil {
		if c, ok := teamstate.IsConfigConflict(err); ok {
			return errors.New(i18n.Tf("v533b.team_config.conflict_cli", strings.Join(c.Keys, ", ")))
		}
		return err
	}
	done(a)
	return nil
}

func runTeamConfigShow(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	repo, teamID, ctx, err := teamConfigRepo(ctxOf(cmd), a)
	if err != nil {
		return err
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil {
		return errors.New(i18n.Tf("v533b.team_config.unreadable", err))
	}
	origin, _ := cmd.Flags().GetBool("origin")
	var set map[string]bool
	if origin {
		if set, err = repo.ConfigKeysSet(); err != nil {
			return err
		}
	}
	local := a.Config.TrackerFor(teamID)
	teamOverride := a.Config.TeamTracker(teamID)
	for _, s := range cfg.Settings() {
		line := fmt.Sprintf("%s = %s", s.Key, teamstate.SettingText(s.Key, s.Value))
		if origin {
			line += "  " + theme.Subtitle.Render("["+settingOrigin(s.Key, set, local, teamOverride)+"]")
		}
		fmt.Fprintln(a.IO.Out, line)
	}
	return nil
}

// settingOrigin says where the value in force of a setting comes from: the
// section of the team in hub.toml (the member, for this team), the default
// [tracker] of hub.toml (the member, for every team), config.toml (the team),
// or nothing written (the default of oh).
func settingOrigin(key string, set map[string]bool, local, teamOverride config.TrackerLocalConfig) string {
	if sub, ok := strings.CutPrefix(key, "tracker."); ok && local.Sets(sub) {
		if teamOverride.Sets(sub) {
			return i18n.T("v54c.origin.me_team")
		}
		return i18n.T("v54c.origin.me_hub")
	}
	if teamstate.InFile(set, key) {
		return i18n.T("v54c.origin.team")
	}
	return i18n.T("v54c.origin.default")
}
