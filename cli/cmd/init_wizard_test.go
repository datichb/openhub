package cmd

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// findButtonForm scans a Flex container for the last *tview.Form child
// that has buttons (the button bar). This is layout-agnostic — it doesn't
// depend on a specific child index.
func findButtonForm(container *tview.Flex) *tview.Form {
	for i := container.GetItemCount() - 1; i >= 0; i-- {
		if f, ok := container.GetItem(i).(*tview.Form); ok && f.GetButtonCount() > 0 {
			return f
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// TestBuildFirstRunInlineWizard_Structure
//
// Builds the wizard with a mock app and verifies the overall shape:
//   - ID is "wizard.init"
//   - The wizard mounts without panicking (validates step/group integrity)
//
// The shape of the steps and of the groups is checked on the configuration
// (TestInitWizard_Groups, TestInitWizard_ScreenCount); here the view itself
// is built: ID(), Title(), and a successful Mount/Unmount cycle which
// validates the internal coherence of steps ↔ groups.
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_Structure(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}

	wiz := buildFirstRunInlineWizard(a)
	require.NotNil(t, wiz)

	// Public ID and Title
	assert.Equal(t, "wizard.init", wiz.ID())
	assert.NotEmpty(t, wiz.Title())

	// Mount/Unmount cycle — exercises all the steps and groups without panicking.
	tvApp := tview.NewApplication()
	content := tview.NewFlex().SetDirection(tview.FlexRow)

	require.NotPanics(t, func() {
		wiz.Mount(content, tvApp)
	})
	// After mount the content container should have at least one child
	// (the wizard's mainFlex).
	assert.Greater(t, content.GetItemCount(), 0,
		"mounted wizard should add items to the content container")

	wiz.Unmount()
}

// ---------------------------------------------------------------------------
// TestBuildFirstRunInlineWizard_ProviderSkip
//
// The provider step is skipped when the user skips it, or when the setup
// type is "team" (the provider comes from the team). Here the wizard is built
// and mounted with an empty config (provider not configured).
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_ProviderSkip(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}

	wiz := buildFirstRunInlineWizard(a)
	require.NotNil(t, wiz)

	// The wizard should build and expose the correct ID regardless of
	// provider configuration.
	assert.Equal(t, "wizard.init", wiz.ID())

	// Mount should succeed without panicking — this exercises the SkipIf
	// closures that depend on providerSkipped.
	tvApp := tview.NewApplication()
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	require.NotPanics(t, func() {
		wiz.Mount(content, tvApp)
	})

	wiz.Unmount()
}

// ---------------------------------------------------------------------------
// The optional screens (provider, project) have a Submit and a Skip button;
// Skip asks for a second click.
// ---------------------------------------------------------------------------

func TestInitWizard_OptionalScreensSkip(t *testing.T) {
	i18ntest.Restore(t)
	for _, tc := range []struct {
		name    string
		build   func(s *initStepState) views.WizardStep
		skipped func(s *initStepState) bool
	}{
		{"provider", buildProviderStep, func(s *initStepState) bool { return s.ProviderSkipped }},
		{"project", buildProjectStep, func(s *initStepState) bool { return s.ProjectSkipped }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestState()
			step := tc.build(s)
			*s.Steps = []views.WizardStep{step}

			tvApp := tview.NewApplication()
			container := tview.NewFlex().SetDirection(tview.FlexRow)
			doneCalled := false
			step.CustomView(tvApp, container, func() { doneCalled = true })

			buttonForm := findButtonForm(container)
			require.NotNil(t, buttonForm)
			require.Equal(t, 2, buttonForm.GetButtonCount(), "Submit + Skip")
			tvApp.SetFocus(buttonForm)
			handler := buttonForm.InputHandler()
			setFocus := func(p tview.Primitive) {}
			handler(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), setFocus)

			enter := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			handler(enter, setFocus)
			assert.False(t, tc.skipped(s), "the first click asks for a confirmation")
			assert.False(t, doneCalled)
			handler(enter, setFocus)
			assert.True(t, tc.skipped(s), "the second click skips the screen")
			assert.True(t, doneCalled)

			fields := step.InfoFields()
			require.Len(t, fields, 1)
			assert.Contains(t, fields[0].Value, i18n.T("cmd.init.wizard_not_configured"),
				"a skipped screen is recapped as not configured")
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildTeamModeIntroStep_Layout
//
// buildTeamModeIntroStep returns a views.WizardStep with a CustomView
// that has 3 buttons: Create / Rejoin / Skip.
// ---------------------------------------------------------------------------

func TestBuildTeamModeIntroStep_Layout(t *testing.T) {
	state := &initWizardTeamState{}
	step := buildTeamModeIntroStep(state)

	assert.True(t, step.Required, "team mode intro step must be Required")
	assert.True(t, step.SidebarHidden, "team mode intro step must be SidebarHidden")
	require.NotNil(t, step.CustomView, "team mode intro step must have a CustomView")

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	onDone := func() { doneCalled = true }

	step.CustomView(tvApp, container, onDone)

	// Find the button form by scanning container children (layout-agnostic).
	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button *tview.Form")

	// Must have 4 buttons: Create, Rejoin, Solo space (P2-T16), Skip
	require.Equal(t, 4, buttonForm.GetButtonCount(),
		"button form should have Create + Rejoin + Solo + Skip buttons")

	// onDone should not have been called yet (no button pressed)
	assert.False(t, doneCalled, "onDone should not fire without user interaction")
}

// ---------------------------------------------------------------------------
// TestBuildTeamModeIntroStep_InitCallback
//
// Activates the "Create" button and verifies state.Mode is set to "init".
// ---------------------------------------------------------------------------

func TestBuildTeamModeIntroStep_InitCallback(t *testing.T) {
	state := &initWizardTeamState{}
	step := buildTeamModeIntroStep(state)

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	step.CustomView(tvApp, container, func() { doneCalled = true })

	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button form")
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)

	// Enter activates the first button (Create)
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(enterEvent, func(p tview.Primitive) {})

	assert.Equal(t, "init", state.Mode, "state.Mode should be 'init' after Create button")
	assert.False(t, state.Skipped, "state.Skipped should be false after Create button")
	assert.True(t, doneCalled, "onDone should have been called after Create")
}

// ---------------------------------------------------------------------------
// TestBuildTeamModeIntroStep_RejoinCallback
//
// Activates the "Rejoin" button and verifies state.Mode is set to "rejoin".
// ---------------------------------------------------------------------------

func TestBuildTeamModeIntroStep_RejoinCallback(t *testing.T) {
	state := &initWizardTeamState{}
	step := buildTeamModeIntroStep(state)

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	step.CustomView(tvApp, container, func() { doneCalled = true })

	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button form")
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)
	setFocus := func(p tview.Primitive) {}

	// Tab to move from Create → Rejoin
	tabEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	handler(tabEvent, setFocus)

	// Enter activates the Rejoin button
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(enterEvent, setFocus)

	assert.Equal(t, "rejoin", state.Mode, "state.Mode should be 'rejoin' after Rejoin button")
	assert.False(t, state.Skipped, "state.Skipped should be false after Rejoin button")
	assert.True(t, doneCalled, "onDone should have been called after Rejoin")
}

// ---------------------------------------------------------------------------
// TestBuildTeamModeIntroStep_SkipCallback
//
// Activates the "Skip" button (with double-click confirmation) and verifies
// state.Skipped is set to true.
// ---------------------------------------------------------------------------

func TestBuildTeamModeIntroStep_SkipCallback(t *testing.T) {
	state := &initWizardTeamState{}
	step := buildTeamModeIntroStep(state)

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	step.CustomView(tvApp, container, func() { doneCalled = true })

	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button form")
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)
	setFocus := func(p tview.Primitive) {}

	// Tab three times: Create → Rejoin → Solo → Skip
	tabEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	handler(tabEvent, setFocus)
	handler(tabEvent, setFocus)
	handler(tabEvent, setFocus)

	// First Enter: triggers confirmation (button label changes)
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(enterEvent, setFocus)
	assert.False(t, state.Skipped, "state.Skipped should NOT be true on first click (confirmation pending)")
	assert.False(t, doneCalled, "onDone should NOT fire on first click (confirmation pending)")

	// Second Enter: confirms skip
	handler(enterEvent, setFocus)
	assert.True(t, state.Skipped, "state.Skipped should be true after confirmed skip")
	assert.Empty(t, state.Mode, "state.Mode should be empty after skip")
	assert.True(t, doneCalled, "onDone should have been called after confirmed skip")
}

// ---------------------------------------------------------------------------
// TestBuildFirstRunInlineWizard_E2E_MountWithPreconfig
//
// P2.5: Verifies the wizard can be built and mounted with various
// pre-existing configurations without panicking. Since the steps are
// internal to the views package, we cannot programmatically advance
// through all the steps from cmd. Instead we verify that different
// configurations produce a valid wizard that mounts cleanly.
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_E2E_MountWithPreconfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E wizard test in short mode")
	}

	configtest.Home(t)
	i18ntest.Restore(t)

	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name:   "empty_config",
			config: &config.Config{},
		},
		{
			name: "provider_already_set",
			config: &config.Config{
				LLM: config.LLMConfig{
					DefaultProvider: "anthropic",
				},
			},
		},
		{
			name: "team_already_configured",
			config: &config.Config{
				Teams: []config.TeamConfig{
					{
						ID:        "my-team",
						Enabled:   true,
						StateRepo: "https://example.com/team-state.git",
						MemberID:  "alice",
					},
				},
			},
		},
		{
			name: "full_config",
			config: &config.Config{
				CLI: config.CLIConfig{Language: "fr"},
				LLM: config.LLMConfig{
					DefaultProvider: "bedrock",
				},
				Teams: []config.TeamConfig{
					{
						ID:        "my-team",
						Enabled:   true,
						StateRepo: "https://example.com/team-state.git",
						MemberID:  "bob",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newMockApp(nil, nil)
			a.Config = tt.config
			a.Projects = &mockProjectStore{projects: []domain.Project{}}

			wiz := buildFirstRunInlineWizard(a)
			require.NotNil(t, wiz, "wizard should be built")

			assert.Equal(t, "wizard.init", wiz.ID())

			tvApp := tview.NewApplication()
			content := tview.NewFlex().SetDirection(tview.FlexRow)
			require.NotPanics(t, func() {
				wiz.Mount(content, tvApp)
			}, "wizard should mount without panic for config %s", tt.name)

			assert.Greater(t, content.GetItemCount(), 0,
				"mounted wizard should add items to the content container")

			wiz.Unmount()
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildFirstRunInlineWizard_E2E_HandleKey
//
// R20: Verifies that HandleKey events (Ctrl+B, Esc) on the real wizard
// don't panic and behave correctly on the welcome CustomView step.
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_E2E_HandleKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E wizard HandleKey test in short mode")
	}

	configtest.Home(t)
	i18ntest.Restore(t)

	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}

	wiz := buildFirstRunInlineWizard(a)
	require.NotNil(t, wiz)

	tvApp := tview.NewApplication()
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	wiz.Mount(content, tvApp)

	// The first step is the Welcome CustomView (Required). Verify HandleKey
	// processes events without panicking.

	// Esc on Required welcome: should be consumed without skip
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	require.NotPanics(t, func() {
		wiz.HandleKey(escEvent)
	}, "Esc on welcome step should not panic")

	// Ctrl+B on first step: should not panic (no previous step)
	ctrlBEvent := tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModCtrl)
	require.NotPanics(t, func() {
		wiz.HandleKey(ctrlBEvent)
	}, "Ctrl+B on first step should not panic")

	// Tab: should not panic
	tabEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	require.NotPanics(t, func() {
		wiz.HandleKey(tabEvent)
	}, "Tab on welcome step should not panic")

	// Enter: should not panic
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	require.NotPanics(t, func() {
		wiz.HandleKey(enterEvent)
	}, "Enter on welcome step should not panic")

	wiz.Unmount()
}

// P2-T16: the « solo space » choice of the first-run wizard.
func TestBuildTeamModeIntroStep_SoloSpace(t *testing.T) {
	state := &initWizardTeamState{}
	step := buildTeamModeIntroStep(state)
	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	step.CustomView(tvApp, container, func() { doneCalled = true })
	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm)
	tvApp.SetFocus(buttonForm)
	handler := buttonForm.InputHandler()
	setFocus := func(p tview.Primitive) {}
	tab := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	handler(tab, setFocus)
	handler(tab, setFocus)
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
	assert.True(t, doneCalled)
	assert.True(t, state.SoloSpace)
	assert.True(t, state.Skipped, "no team configured")
}
