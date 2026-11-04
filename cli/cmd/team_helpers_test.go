package cmd

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// ─────────────────────────────────────────────────────────────────────────────
// buildInitWizardTeamSteps tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildInitWizardTeamSteps_Structure(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	appPtr := &a
	state := &initWizardTeamState{}

	steps := buildInitWizardTeamSteps(appPtr, state)
	require.Len(t, steps, 4, "should return exactly 4 steps (form + processing + tracker_setup + tracker_token)")

	// Step 0: Team form
	assert.NotNil(t, steps[0].Form, "team form step should have a Form")
	assert.NotNil(t, steps[0].SkipIf, "team form step should have a SkipIf")
	assert.NotNil(t, steps[0].Validate, "team form step should have a Validate")
	assert.NotNil(t, steps[0].InfoFields, "team form step should have InfoFields")

	// Step 1: Team processing
	assert.NotNil(t, steps[1].OnDone, "team processing step should have OnDone")
	assert.NotNil(t, steps[1].SkipIf, "team processing step should have SkipIf")
	assert.NotEmpty(t, steps[1].Processing, "team processing step should have Processing label")

	// Step 2: Tracker setup (choice)
	assert.NotNil(t, steps[2].SkipIf, "tracker choice step should have SkipIf")

	// Step 3: Tracker token (conditional)
	assert.NotNil(t, steps[3].SkipIf, "tracker token step should have SkipIf")
}

func TestBuildInitWizardTeamSteps_SkipIf(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a

	t.Run("skipped_true", func(t *testing.T) {
		state := &initWizardTeamState{Skipped: true}
		steps := buildInitWizardTeamSteps(appPtr, state)
		assert.True(t, steps[0].SkipIf(), "form should be skipped when Skipped=true")
		assert.True(t, steps[1].SkipIf(), "processing should be skipped when Skipped=true")
	})

	t.Run("wrong_mode", func(t *testing.T) {
		state := &initWizardTeamState{Skipped: false, Mode: "rejoin"}
		steps := buildInitWizardTeamSteps(appPtr, state)
		assert.True(t, steps[0].SkipIf(), "form should be skipped when Mode != init")
		assert.True(t, steps[1].SkipIf(), "processing should be skipped when Mode != init")
	})

	t.Run("active_init_mode", func(t *testing.T) {
		state := &initWizardTeamState{Skipped: false, Mode: "init"}
		steps := buildInitWizardTeamSteps(appPtr, state)
		assert.False(t, steps[0].SkipIf(), "form should not be skipped in init mode")
		assert.False(t, steps[1].SkipIf(), "processing should not be skipped in init mode")
	})
}

// The member's identity is the form of « Créer »; the repository and its
// access came before (buildInitWizardRepoSteps).
func TestBuildInitWizardTeamSteps_Validate(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a
	state := &initWizardTeamState{Mode: "init"}
	steps := buildInitWizardTeamSteps(appPtr, state)
	repoSteps := buildInitWizardRepoSteps(appPtr, state)

	t.Run("empty_state", func(t *testing.T) {
		assert.NotEmpty(t, repoSteps[0].Validate(), "the repository is required")
		assert.NotEmpty(t, steps[0].Validate(), "the member ID is required")
	})

	t.Run("repo_only", func(t *testing.T) {
		state.Repo = "https://example.com/team-state.git"
		state.MemberID = ""
		assert.Empty(t, repoSteps[0].Validate())
		assert.NotEmpty(t, steps[0].Validate(), "should fail when MemberID is empty")
	})

	t.Run("repo_and_member", func(t *testing.T) {
		state.Repo = "https://example.com/team-state.git"
		state.MemberID = "alice"
		assert.Empty(t, repoSteps[0].Validate())
		assert.Empty(t, steps[0].Validate(), "should pass when the MemberID is set")
	})
}

func TestBuildInitWizardTeamSteps_FormRendering(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a
	state := &initWizardTeamState{
		Mode: "init",
		Repo: "https://example.com/team-state.git",
	}
	steps := buildInitWizardTeamSteps(appPtr, state)
	repoSteps := buildInitWizardRepoSteps(appPtr, state)

	tvApp := tview.NewApplication()
	form := steps[0].Form(tvApp, func() {})
	require.NotNil(t, form)
	assert.Equal(t, 2, form.GetFormItemCount(), "identity form: MemberID and DisplayName")
	assert.Equal(t, 1, form.GetButtonCount(), "form should have 1 submit button")

	form = repoSteps[0].Form(tvApp, func() {})
	require.NotNil(t, form)
	assert.Equal(t, 3, form.GetFormItemCount(), "repository form: Repo, the GitLab of an aliased repo and its hint")
	assert.Equal(t, 1, form.GetButtonCount())
}

// The member step of the rejoin wizard lists « add a new member » then the
// members; / filters them and Enter picks (FilterableList is gone).
func TestMemberPickerFiltersAndPicks(t *testing.T) {
	state := &initWizardTeamState{Members: []teamstate.Member{
		{ID: "alice", DisplayName: "Alice", Role: "dev"},
		{ID: "bob", DisplayName: "Bob", Role: "lead", GitLabUsername: "bobl"},
	}}
	tvApp := tview.NewApplication()
	container := tview.NewFlex()
	done := 0
	showMemberPicker(tvApp, container, state, func() { done++ })
	box, ok := container.GetItem(0).(*tview.Flex)
	require.True(t, ok)
	press := func(ev *tcell.EventKey) {
		h := container.InputHandler()
		require.NotNil(t, h)
		h(ev, func(p tview.Primitive) { tvApp.SetFocus(p) })
	}

	press(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	press(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	press(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	assert.Equal(t, 1, done)
	assert.Equal(t, "bob", state.MemberID)
	assert.False(t, state.AddingNewMember)

	// / opens the input under the list; words keep the matching members.
	press(tcell.NewEventKey(tcell.KeyRune, '/', 0))
	require.Equal(t, 2, box.GetItemCount(), "the filter input is shown under the list")
	input, ok := box.GetItem(1).(*tview.InputField)
	require.True(t, ok)
	input.SetText("ali")
	input.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { tvApp.SetFocus(p) })
	assert.Equal(t, 1, box.GetItemCount(), "Enter closes the input")
	press(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	assert.Equal(t, 2, done)
	assert.Equal(t, "alice", state.MemberID)

	// The first element adds a member.
	state.MemberID = ""
	container.Clear()
	showMemberPicker(tvApp, container, state, func() { done++ })
	press(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	assert.True(t, state.AddingNewMember)
	assert.Equal(t, "", state.MemberID)
}
