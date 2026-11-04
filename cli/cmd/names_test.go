package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// The team board reads the names of the projects with each read of the
// tickets: a project added or renamed after the board was built shows at the
// next refresh, and a project the registry does not know keeps its ID.
func TestTeamBoardReadsTheNamesOfTheProjectsAgain(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	store := &mockProjectStore{}
	a.Projects = store
	claimsCreate(t, repo, "T-1", "testuser", "")
	cfg := buildTeamBoardViewConfig(a)
	require.Len(t, cfg.Tickets, 1)
	assert.Equal(t, claimsProject, cfg.Tickets[0].ProjectName, "unknown: the ID")

	require.NoError(t, store.Create(t.Context(), &domain.Project{ID: claimsProject, Name: "Alpha", Path: "/p/alpha"}))
	tickets := cfg.RefreshFunc()
	require.Len(t, tickets, 1)
	assert.Equal(t, "Alpha", tickets[0].ProjectName, "added after the board was built")
	assert.Equal(t, claimsProject, tickets[0].Project, "the ID is still what filters")

	store.projects[0].Name = "Alpha 2"
	tickets = cfg.RefreshFunc()
	assert.Equal(t, "Alpha 2", tickets[0].ProjectName, "renamed")
}

// The quick actions of the boards resolve the project of a card when the
// action is asked: a project added after the start is found, and one that was
// moved is launched where it is now.
func TestBoardQuickActionsResolveTheProjectAtEachCall(t *testing.T) {
	a, _ := setupBeat(t)
	store := &mockProjectStore{}
	a.Projects = store
	resolve := buildBoardQuickActions(a).ResolveProjectByDirID
	require.NotNil(t, resolve)

	_, _, ok := resolve("api")
	assert.False(t, ok, "not registered yet")

	require.NoError(t, store.Create(t.Context(), &domain.Project{ID: "api", Name: "API", Path: "/work/api"}))
	id, path, ok := resolve("api")
	require.True(t, ok, "added after the start")
	assert.Equal(t, "api", id)
	assert.Equal(t, "/work/api", path)

	store.projects[0].Path = "/work/moved/api"
	_, path, _ = resolve("api")
	assert.Equal(t, "/work/moved/api", path, "the new path, not the one of the start")
}

// A project item names its team (the ID when the hub does not list it).
func TestProjectItemNamesItsTeam(t *testing.T) {
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "plat-1", Name: "Platform"}, {ID: "bare"}}}
	team := func(id string) domain.Project { return domain.Project{ID: "p", Name: "P", TeamID: &id} }
	assert.Equal(t, "Platform", domainProjectToViewItem(team("plat-1"), cfg).TeamName)
	assert.Equal(t, "plat-1", domainProjectToViewItem(team("plat-1"), cfg).TeamID, "the ID stays what the view filters on")
	assert.Equal(t, "bare", domainProjectToViewItem(team("bare"), cfg).TeamName, "no name: the ID")
	assert.Equal(t, "ghost", domainProjectToViewItem(team("ghost"), cfg).TeamName, "unknown team: the ID")
	assert.Empty(t, domainProjectToViewItem(domain.Project{ID: "solo"}, cfg).TeamName)
	assert.Equal(t, "x", domainProjectToViewItem(team("x"), nil).TeamName)
}

// The list of the teams names the member of each team from its team-state; a team
// that is not cloned here is missing, and so is a member the team does not list.
func TestTeamMemberNamesReadTheTeamStates(t *testing.T) {
	a, _, _ := setupClaimTestApp(t)
	teams := append([]config.TeamConfig(nil), a.Config.Teams...)
	teams = append(teams,
		config.TeamConfig{ID: "far", Enabled: true, StateRepo: "git@example.com:far/state.git", StatePath: t.TempDir() + "/absent", MemberID: "alice"},
		config.TeamConfig{ID: "stranger", Enabled: true, StateRepo: teams[0].StateRepo, StatePath: teams[0].StatePath, MemberID: "nobody"})
	assert.Equal(t, map[string]string{"test-team": "Test User"}, teamMemberNames(t.Context(), teams))
}

// A goroutine that ends after the TUI stopped queues nothing on its event loop
// (QueueUpdateDraw would wait for a loop that is gone).
func TestQueueUnlessEndedQueuesNothingOnceTheShellEnded(t *testing.T) {
	var queued int
	queue := func(f func()) { queued++; f() }
	ran := 0

	ctx, end := context.WithCancel(t.Context())
	queueUnlessEnded(ctx, queue, func() { ran++ })
	assert.Equal(t, 1, queued, "queued while the shell lives")
	assert.Equal(t, 1, ran)

	end()
	queueUnlessEnded(ctx, queue, func() { ran++ })
	assert.Equal(t, 1, queued, "nothing queued once it ended")
	assert.Equal(t, 1, ran)

	queueOnShell(nil, func() { ran++ }) // no shell (command line): no panic, no call
	assert.Equal(t, 1, ran)
}
