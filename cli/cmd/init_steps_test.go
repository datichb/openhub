package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// newTestState creates a minimal initStepState for testing.
func newTestState() *initStepState {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	appPtr := &a
	focusBtn := false
	var steps []views.WizardStep

	return &initStepState{
		SelectedLang:    "en",
		ProviderOptions: []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        &focusBtn,
		Steps:           &steps,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildWelcomeStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildWelcomeStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildWelcomeStep(s)

	assert.Equal(t, "welcome", step.ID)
	assert.True(t, step.Required, "welcome must be Required")
	assert.True(t, step.SidebarHidden, "welcome must be SidebarHidden")
	assert.NotNil(t, step.CustomView, "welcome must have CustomView")
	assert.Nil(t, step.Form, "welcome must not have Form")
	assert.NotNil(t, step.InfoFields, "welcome must have InfoFields")
}

func TestBuildWelcomeStep_Renders(t *testing.T) {
	s := newTestState()
	step := buildWelcomeStep(s)

	app := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false

	require.NotPanics(t, func() {
		step.CustomView(app, container, func() { doneCalled = true })
	})
	assert.Greater(t, container.GetItemCount(), 0, "container should have children")
	assert.False(t, doneCalled, "onDone should not be called during render")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildLangStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildLangStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildLangStep(s)

	assert.Equal(t, "lang", step.ID)
	assert.True(t, step.Required, "lang must be Required")
	assert.NotNil(t, step.Form, "lang must have Form")
	assert.NotNil(t, step.OnDone, "lang must have OnDone")
	assert.NotNil(t, step.InfoFields, "lang must have InfoFields")
}

func TestBuildLangStep_FormRendering(t *testing.T) {
	s := newTestState()
	steps := []views.WizardStep{buildWelcomeStep(s), buildLangStep(s)}
	*s.Steps = steps
	s.LangStepIdx = 1

	app := tview.NewApplication()
	form := steps[1].Form(app, func() {})
	require.NotNil(t, form)
	assert.Equal(t, 1, form.GetFormItemCount(), "lang form should have 1 field (DropDown)")
	assert.Equal(t, 1, form.GetButtonCount(), "lang form should have 1 button")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildProviderStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProviderStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildProviderStep(s)

	assert.Equal(t, "provider", step.ID)
	assert.NotNil(t, step.Form, "provider must have Form")
	assert.NotNil(t, step.SkipIf, "provider must have SkipIf")
	assert.NotNil(t, step.Validate, "provider must have Validate")
	assert.NotNil(t, step.OnDone, "provider must have OnDone")
	assert.NotNil(t, step.InfoFields, "provider must have InfoFields")
	assert.NotEmpty(t, step.Processing, "provider must have Processing label")
}

func TestBuildProviderStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildProviderStep(s)

	s.ProviderSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.ProviderSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildProviderStep_FormRendering(t *testing.T) {
	s := newTestState()
	steps := []views.WizardStep{buildProviderStep(s)}
	*s.Steps = steps
	s.ProviderStepIdx = 0

	app := tview.NewApplication()
	form := steps[0].Form(app, func() {})
	require.NotNil(t, form)
	// At minimum: provider dropdown + submit button
	assert.GreaterOrEqual(t, form.GetFormItemCount(), 1, "provider form should have at least 1 field")
	assert.GreaterOrEqual(t, form.GetButtonCount(), 1, "provider form should have at least 1 button")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildProjectStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProjectStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	assert.Equal(t, "project", step.ID)
	assert.NotNil(t, step.Form, "project must have Form")
	assert.NotNil(t, step.SkipIf, "project must have SkipIf")
	assert.NotNil(t, step.Validate, "project must have Validate")
	assert.NotNil(t, step.OnDone, "project must have OnDone")
	assert.NotNil(t, step.InfoFields, "project must have InfoFields")
	assert.NotEmpty(t, step.Processing, "project must have Processing label")
}

func TestBuildProjectStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	s.ProjectSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.ProjectSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildProjectStep_ValidationEmpty(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	s.ProjectName = ""
	errMsg := step.Validate()
	assert.NotEmpty(t, errMsg, "should fail validation when name is empty")
}

func TestBuildProjectStep_FormRendering(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	app := tview.NewApplication()
	form := step.Form(app, func() {})
	require.NotNil(t, form)
	// name + path fields + submit button
	assert.GreaterOrEqual(t, form.GetFormItemCount(), 2, "project form should have at least 2 fields")
	assert.GreaterOrEqual(t, form.GetButtonCount(), 1, "project form should have at least 1 button")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildDeployStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildDeployStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildDeployStep(s)

	assert.Equal(t, "deploy", step.ID)
	assert.NotNil(t, step.Form, "deploy must have Form")
	assert.NotNil(t, step.SkipIf, "deploy must have SkipIf")
	assert.NotNil(t, step.OnDone, "deploy must have OnDone")
	assert.NotNil(t, step.InfoFields, "deploy must have InfoFields")
	assert.NotEmpty(t, step.Processing, "deploy must have Processing label")
}

func TestBuildDeployStep_SkipWithoutProject(t *testing.T) {
	s := newTestState()
	step := buildDeployStep(s)

	s.ProjectSkipped = false
	s.ProjectCreated = false
	assert.True(t, step.SkipIf(), "should skip when no project was created")

	s.ProjectCreated = true
	assert.False(t, step.SkipIf(), "should not skip when project was created")

	s.ProjectSkipped = true
	assert.True(t, step.SkipIf(), "should skip when project section was skipped")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildMCPGitLabStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildMCPGitLabStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildMCPGitLabStep(s)

	assert.Equal(t, "mcp_gitlab", step.ID)
	assert.NotNil(t, step.Form, "gitlab must have Form")
	assert.NotNil(t, step.SkipIf, "gitlab must have SkipIf")
	assert.NotNil(t, step.OnDone, "gitlab must have OnDone")
	assert.NotNil(t, step.InfoFields, "gitlab must have InfoFields")
}

func TestBuildMCPGitLabStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildMCPGitLabStep(s)

	s.MCPSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.MCPSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildMCPGitLabStep_FormRendering(t *testing.T) {
	s := newTestState()
	step := buildMCPGitLabStep(s)

	app := tview.NewApplication()
	form := step.Form(app, func() {})
	require.NotNil(t, form)
	// password field + write checkbox + hints + submit
	assert.GreaterOrEqual(t, form.GetFormItemCount(), 2, "gitlab form should have at least 2 fields")
	assert.GreaterOrEqual(t, form.GetButtonCount(), 1, "gitlab form should have at least 1 button")
}
