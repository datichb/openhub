package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
