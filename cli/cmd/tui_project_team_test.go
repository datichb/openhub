package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

func choiceValues(t *testing.T, cfg *config.Config, current string) []string {
	t.Helper()
	var out []string
	for _, o := range projectTeamChoices(cfg, current) {
		out = append(out, o.Value)
	}
	return out
}

// The team of a project is chosen among every team (the default one first),
// the solo spaces and « none »; an archived team or a solo space that is not in
// hub.toml any more is not offered, but the team the project is in stays.
func TestProjectTeamChoices(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "alpha", Enabled: true, StateRepo: "git@x:a.git", MemberID: "me"},
		{ID: "beta", Enabled: true, StateRepo: "git@x:b.git", MemberID: "me", Default: true},
		{ID: "old", Enabled: false, StateRepo: "git@x:o.git", MemberID: "me"},
		{ID: "mine", Enabled: true, Solo: true, MemberID: "me"},
	}}

	assert.Equal(t, []string{"beta", "alpha", "mine", ""}, choiceValues(t, cfg, ""),
		"the default team first, the solo space, then none")
	assert.Equal(t, []string{"beta", "alpha", "mine", "old", ""}, choiceValues(t, cfg, "old"),
		"the archived team the project is in stays in the list")
	got := projectTeamChoices(cfg, "ghost")
	require.Len(t, got, 5)
	assert.Equal(t, "ghost", got[3].Value)
	assert.Contains(t, got[3].Label, "unknown team")
	assert.Contains(t, got[0].Label, "★", "the default team is marked")
	assert.Equal(t, []string{""}, choiceValues(t, &config.Config{}, ""), "no team at all: only none")
}

// The attachment goes through the same rules as `oh project set-team`: a team
// of hub.toml, an archived team refused, none detaches.
func TestAttachProjectToATeam(t *testing.T) {
	i18ntest.Set(t, "en")
	team := "alpha"
	store := &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "api", Status: domain.ProjectStatusActive, TeamID: &team}}}
	a := &app.App{Projects: store, Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "alpha", Enabled: true, StateRepo: "git@x:a.git", MemberID: "me"},
		{ID: "beta", Enabled: true, StateRepo: "git@x:b.git", MemberID: "me"},
		{ID: "old", Enabled: false, StateRepo: "git@x:o.git", MemberID: "me"},
	}}}
	ctx := context.Background()

	label, name, err := attachProject(ctx, a, "p1", "beta")
	require.NoError(t, err)
	assert.Equal(t, "beta", label)
	assert.Equal(t, "api", name)
	assert.Equal(t, "beta", projectTeamID(&store.projects[0]))

	_, _, err = attachProject(ctx, a, "p1", "old")
	require.Error(t, err, "an archived team is restored first")
	assert.Equal(t, "beta", projectTeamID(&store.projects[0]), "nothing changed")

	_, _, err = attachProject(ctx, a, "p1", "ghost")
	require.Error(t, err)

	label, _, err = attachProject(ctx, a, "p1", "")
	require.NoError(t, err)
	assert.Equal(t, "none", label)
	assert.Equal(t, "", projectTeamID(&store.projects[0]), "none detaches the project")

	_, _, err = attachProject(ctx, a, "missing", "alpha")
	require.Error(t, err)
}

// The commands of the team are always in the palette, so that a team created
// in the TUI brings them without a restart: the team mode shows them, and the
// four ways to start a team are in the category Team and visible everywhere.
func TestTeamCommandsAreAlwaysRegistered(t *testing.T) {
	i18ntest.Set(t, "en")
	byID := map[string]shell.Command{}
	for _, c := range buildCommands(&app.App{Config: &config.Config{}}) {
		byID[c.ID] = c
	}
	for _, id := range []string{"team.board", "team.status", "team.activity", "team.sync", "team.reconfigure", "team.switch", "team-detail", "team.discover"} {
		c, ok := byID[id]
		require.True(t, ok, "%s is registered with no team configured", id)
		assert.Equal(t, modeTeam, c.Modes, id)
		assert.Equal(t, "Team", c.Category, id)
	}
	for _, id := range []string{"team.new", "team.init", "team.rejoin"} {
		c := byID[id]
		assert.Empty(t, c.Modes, "%s shows in every mode", id)
		assert.Equal(t, "Team", c.Category, id)
	}
	assert.Equal(t, modeProject, byID["team.configure"].Modes, "attaching a project is a command of the project mode")
	assert.Contains(t, byID["team.configure"].Label, "Attach the project")
	assert.NotContains(t, byID, "history.team", "« Team History » was the activity under another name")
	assert.Contains(t, byID["team.activity"].Aliases, "team history")
	assert.Equal(t, "Team", byID["teams"].Category)
}
