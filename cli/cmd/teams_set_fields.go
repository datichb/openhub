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

// oh teams set <team-id> name|member-id|enabled|default <value>: the other
// settings of a team of hub.toml (the repository is teams_set.go). They are
// the same ones the team detail of the TUI edits.
//
//   - name: the display name ("" removes it, the id is shown again).
//   - member-id: the member the team knows this machine's user as; it must be
//     a profile of the team-state (a colleague's id would make their profile
//     yours), checked when the clone is there.
//   - enabled: false archives the team, true restores it (the commands
//     oh teams archive and restore).
//   - default: true makes it the default team (oh teams default), false
//     removes the mark.

// teamSetFields are the fields oh teams set takes besides repo.
const teamSetFields = "repo, name, member-id, enabled, default"

// setTeamField sets one field of a team other than its repository. value is
// "" with hasValue false when the command gave none (only default may omit it).
func setTeamField(cmd *cobra.Command, a *app.App, id, field, value string, hasValue bool) error {
	if a.Config.FindTeam(id) == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found", id, theme.Bold.Render("oh teams list")))
	}
	if !hasValue && field != "default" {
		return errors.New(i18n.Tf("v54c.teams.set.value_required", field))
	}
	switch field {
	case "name":
		return saveTeamField(a, id, func(t *config.TeamConfig) error { t.Name = value; return nil },
			i18n.Tf("v54c.teams.set.name_done", id, firstNonEmpty(value, id)))
	case "member-id":
		return setTeamMember(ctxOf(cmd), a, id, value)
	case "enabled":
		on, err := parseBoolValue(value)
		if err != nil {
			return errors.New(i18n.Tf("v54c.teams.set.want_bool", field, value))
		}
		if on {
			return runTeamsRestore(cmd, []string{id})
		}
		return runTeamsArchive(cmd, []string{id})
	case "default":
		on := true
		if hasValue {
			var err error
			if on, err = parseBoolValue(value); err != nil {
				return errors.New(i18n.Tf("v54c.teams.set.want_bool", field, value))
			}
		}
		if on {
			return runTeamsDefault(cmd, []string{id})
		}
		return saveTeamField(a, id, func(t *config.TeamConfig) error { t.Default = false; return nil },
			i18n.Tf("v54c.teams.set.default_off", id))
	}
	return errors.New(i18n.Tf("v54a.teams.set.unknown_field", field, teamSetFields))
}

func saveTeamField(a *app.App, id string, change func(*config.TeamConfig) error, done string) error {
	if err := changeTeamEntry(a, id, change); err != nil {
		return err
	}
	cliout.Success(a.IO.Out, done)
	return nil
}

// changeTeamEntry changes the entry of a team in hub.toml, under the lock.
func changeTeamEntry(a *app.App, id string, change func(*config.TeamConfig) error) error {
	if err := updateHubConfig(a, func(c *config.Config) error {
		t := c.FindTeam(id)
		if t == nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.teams.not_found", id, "oh teams list"))
		}
		return change(t)
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}
	return nil
}

// applyTeamField is what the detail of the TUI writes to hub.toml for a field
// of the team (views.TeamField*): the same rules as oh teams set, nothing
// printed.
func applyTeamField(ctx context.Context, a *app.App, id, field, value string) error {
	switch field {
	case "name":
		return changeTeamEntry(a, id, func(t *config.TeamConfig) error { t.Name = strings.TrimSpace(value); return nil })
	case "member_id":
		if err := checkTeamMember(ctx, a, id, value); err != nil {
			return err
		}
		return changeTeamEntry(a, id, func(t *config.TeamConfig) error { t.MemberID = value; return nil })
	case "default":
		on, err := parseBoolValue(value)
		if err != nil {
			return errors.New(i18n.Tf("v54c.teams.set.want_bool", field, value))
		}
		if on {
			return setDefaultTeam(a, id)
		}
		return changeTeamEntry(a, id, func(t *config.TeamConfig) error { t.Default = false; return nil })
	}
	return errors.New(i18n.Tf("v54a.teams.set.unknown_field", field, "name, member_id, default"))
}

// memberChecker is the clone of a team whose members are looked at.
type memberChecker interface {
	teamstate.RepoInfo
	HasMember(ctx context.Context, id string) bool
}

// checkTeamMember refuses a member id that is not a card of the team-state
// (when the clone is there to say).
func checkTeamMember(ctx context.Context, a *app.App, id, member string) error {
	if !validMemberID(member) {
		return errors.New(i18n.Tf("v54c.teams.set.member_invalid", member))
	}
	team := a.Config.FindTeam(id)
	if team == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found", id, theme.Bold.Render("oh teams list")))
	}
	if team.StateRepo != "" {
		var clone memberChecker = teamstate.NewRepo(team.StateRepo, config.ResolveStatePath(team.StateRepo, team.StatePath))
		if clone.IsCloned() && !clone.HasMember(ctx, member) {
			return errors.New(i18n.Tf("v54c.teams.set.member_unknown", member, id,
				theme.Bold.Render("oh team rejoin --repo "+team.StateRepo+" --new-member")))
		}
	}
	return nil
}

func setTeamMember(ctx context.Context, a *app.App, id, member string) error {
	if err := checkTeamMember(ctx, a, id, member); err != nil {
		return err
	}
	return saveTeamField(a, id, func(t *config.TeamConfig) error { t.MemberID = member; return nil },
		i18n.Tf("v54c.teams.set.member_done", id, member))
}

// validMemberID is the shape of a member id: what members.toml accepts as a
// table name, without spaces.
func validMemberID(id string) bool {
	return id != "" && id == strings.TrimSpace(id) && !strings.ContainsAny(id, " \t/\\.\"'")
}
