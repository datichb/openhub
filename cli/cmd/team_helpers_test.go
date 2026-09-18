package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
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
	require.Len(t, steps, 2, "should return exactly 2 steps (form + processing)")

	// Step 0: Team form
	assert.NotNil(t, steps[0].Form, "team form step should have a Form")
	assert.NotNil(t, steps[0].SkipIf, "team form step should have a SkipIf")
	assert.NotNil(t, steps[0].Validate, "team form step should have a Validate")
	assert.NotNil(t, steps[0].InfoFields, "team form step should have InfoFields")

	// Step 1: Team processing
	assert.NotNil(t, steps[1].OnDone, "team processing step should have OnDone")
	assert.NotNil(t, steps[1].SkipIf, "team processing step should have SkipIf")
	assert.NotEmpty(t, steps[1].Processing, "team processing step should have Processing label")
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

func TestBuildInitWizardTeamSteps_Validate(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a
	state := &initWizardTeamState{Mode: "init"}
	steps := buildInitWizardTeamSteps(appPtr, state)

	t.Run("empty_state", func(t *testing.T) {
		assert.NotEmpty(t, steps[0].Validate(), "should fail when both Repo and MemberID are empty")
	})

	t.Run("repo_only", func(t *testing.T) {
		state.Repo = "https://example.com/team-state.git"
		state.MemberID = ""
		assert.NotEmpty(t, steps[0].Validate(), "should fail when MemberID is empty")
	})

	t.Run("repo_and_member", func(t *testing.T) {
		state.Repo = "https://example.com/team-state.git"
		state.MemberID = "alice"
		assert.Empty(t, steps[0].Validate(), "should pass when both Repo and MemberID are set")
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

	tvApp := tview.NewApplication()
	form := steps[0].Form(tvApp, func() {})
	require.NotNil(t, form)
	assert.Equal(t, 3, form.GetFormItemCount(), "form should have 3 fields: Repo, MemberID, DisplayName")
	assert.Equal(t, 1, form.GetButtonCount(), "form should have 1 submit button")
}

// ─────────────────────────────────────────────────────────────────────────────
// newMockApp helper (if not already available from session_launch_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// mockApp creates a minimal *app.App for testing wizard step structures.
// Note: newMockApp should be defined in session_launch_test.go or similar.
// If it's not accessible, this helper provides a minimal version.
func mockAppForTeamTests() *app.App {
	return &app.App{
		Config:  &config.Config{},
		Secrets: &mockSecretStore{secrets: map[string]string{}},
	}
}
