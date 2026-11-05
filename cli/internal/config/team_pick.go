package config

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
)

// TeamSource says which rule of PickTeam gave the team.
type TeamSource string

const (
	// TeamFromFlag: the team named by --team.
	TeamFromFlag TeamSource = "flag"
	// TeamFromTUI: the team chosen in the TUI (team mode, or opened from the
	// list of teams).
	TeamFromTUI TeamSource = "tui"
	// TeamFromProject: the team the project belongs to.
	TeamFromProject TeamSource = "project"
	// TeamFromDefault: the team marked `default = true`.
	TeamFromDefault TeamSource = "default"
	// TeamFromFirst: the first enabled team, when none is marked default.
	TeamFromFirst TeamSource = "first"
	// TeamFromLegacy: the former [team] section of hub.toml.
	TeamFromLegacy TeamSource = "legacy"
	// TeamNone: no team applies.
	TeamNone TeamSource = ""
)

// UnknownTeamError is returned for a team ID that hub.toml does not know.
type UnknownTeamError struct{ ID string }

func (e *UnknownTeamError) Error() string { return fmt.Sprintf("unknown team %q", e.ID) }

// TeamSelection is what points to a team, from the most to the least specific.
type TeamSelection struct {
	// Flag is the value of --team ("" when not given).
	Flag string
	// TUI is the team chosen in the TUI ("" outside of it, or when none).
	TUI string
	// Project is the project the command works on (nil: none).
	Project *domain.Project
}

// PickTeam is the one rule that says which team applies:
//
//  1. --team (Flag): the team of that ID, archived or solo ones included,
//     because the member asked for it; an unknown ID is an error;
//  2. the team chosen in the TUI (a team that is gone is ignored);
//  3. the team of the project, when it is a team with team features on
//     (neither archived nor a solo space);
//  4. the default team (`default = true`);
//  5. the first enabled team that is not a solo space;
//  6. the former [team] section.
//
// Source is TeamNone, with a zero TeamConfig, when nothing applies. An
// archived team is never given by 2–5.
func (c *Config) PickTeam(sel TeamSelection) (TeamConfig, TeamSource, error) {
	if sel.Flag != "" {
		t := c.FindTeam(sel.Flag)
		if t == nil {
			return TeamConfig{}, TeamNone, &UnknownTeamError{ID: sel.Flag}
		}
		return *t, TeamFromFlag, nil
	}
	if sel.TUI != "" {
		if t := c.FindTeam(sel.TUI); t != nil {
			return *t, TeamFromTUI, nil
		}
	}
	if p := sel.Project; p != nil && p.TeamID != nil && *p.TeamID != "" {
		if t := c.FindTeam(*p.TeamID); t != nil && t.usable() {
			return *t, TeamFromProject, nil
		}
	}
	return c.hubTeam()
}

// hubTeam is the team nothing in particular points to.
func (c *Config) hubTeam() (TeamConfig, TeamSource, error) {
	if t := c.DefaultTeam(); t != nil {
		if t.Default {
			return *t, TeamFromDefault, nil
		}
		return *t, TeamFromFirst, nil
	}
	if len(c.Teams) == 0 && c.Team.StateRepo != "" {
		return c.Team, TeamFromLegacy, nil
	}
	return TeamConfig{}, TeamNone, nil
}

// Resolved is the team as the commands use it: enabled when team features are
// active for it (neither archived nor a solo space). A team without ID gets the
// name of its repository, which its keychain keys use.
func (t TeamConfig) Resolved() ResolvedTeamConfig {
	return ResolvedTeamConfig{
		Enabled:   t.Enabled && !t.Solo,
		TeamID:    t.ID,
		StateRepo: t.StateRepo,
		StatePath: ResolveStatePath(t.StateRepo, t.StatePath),
		GitLabURL: t.GitLabURL,
		MemberID:  t.MemberID,
		Solo:      t.Solo && t.Enabled,
	}
}
