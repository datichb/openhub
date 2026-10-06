package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

// The « solo space » choice of the project wizards (P2-T16): the project is
// attached to the existing solo space, else to a new one.
func TestAttachProjectSolo(t *testing.T) {
	a, _ := setupSoloTestApp(t)
	store := a.Projects.(*mockProjectStore)
	store.projects = append(store.projects, domain.Project{ID: "api", Name: "api", Path: t.TempDir()})
	ctx := t.Context()

	web, err := a.Projects.Get(ctx, "web")
	require.NoError(t, err)
	sp, err := attachProjectSolo(ctx, a, web)
	require.NoError(t, err)
	assert.True(t, sp.Created)
	assert.Equal(t, "solo", sp.Team.ID)

	api, err := a.Projects.Get(ctx, "api")
	require.NoError(t, err)
	sp2, err := attachProjectSolo(ctx, a, api)
	require.NoError(t, err)
	assert.False(t, sp2.Created, "the solo space is reused")
	assert.Equal(t, "solo", sp2.Team.ID)
	api, _ = a.Projects.Get(ctx, "api")
	require.NotNil(t, api.TeamID)
	assert.Equal(t, "solo", *api.TeamID)
	assert.Equal(t, "solo-2", nextSoloID(a.Config))
	assert.DirExists(t, sp.Repo.Path()+"/projects/api/workflows")
}

func TestProjectTeamStepSoloChoice(t *testing.T) {
	a := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@x:core.git", MemberID: "ben"}}}
	for _, tc := range []struct {
		idx      int
		wantTeam string
		wantSolo bool
	}{{0, "core", false}, {1, "", true}, {2, "", false}} {
		var team *string
		var solo bool
		step := buildProjectTeamStep(&app.App{Config: a}, &team, &solo)
		form := step.Form(nil, func() {})
		var dd *tview.DropDown
		for i := range form.GetFormItemCount() {
			if d, ok := form.GetFormItem(i).(*tview.DropDown); ok {
				dd = d
			}
		}
		require.NotNil(t, dd)
		dd.SetCurrentOption(tc.idx)
		require.NoError(t, step.OnDone())
		got := ""
		if team != nil {
			got = *team
		}
		assert.Equal(t, tc.wantTeam, got, "choice %d", tc.idx)
		assert.Equal(t, tc.wantSolo, solo, "choice %d", tc.idx)
	}
}

// First-run wizard: the project is attached to a solo space when chosen.
func TestInitWizardSoloSpace(t *testing.T) {
	a, _ := setupSoloTestApp(t)
	ctx := t.Context()
	web, err := a.Projects.Get(ctx, "web")
	require.NoError(t, err)
	s := &initStepState{AppPtr: &a, TeamState: &initWizardTeamState{}}
	initWizardSoloSpace(ctx, s, web)
	assert.Nil(t, a.Config.FindTeam("solo"), "not chosen: nothing created")
	s.TeamState.SoloSpace = true
	initWizardSoloSpace(ctx, s, web)
	require.NotNil(t, a.Config.FindTeam("solo"))
	web, _ = a.Projects.Get(ctx, "web")
	require.NotNil(t, web.TeamID)
	assert.Equal(t, "solo", *web.TeamID)
}
