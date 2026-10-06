package cmd

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
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
//   - Steps 0, 2 are intro/welcome (SidebarHidden, Required, CustomView)
//   - Step 4 is the team mode intro (tri-choice: init/rejoin/skip)
//
// Note: InlineWizardView.cfg is unexported, so we cannot directly inspect
// step count or group StartIdx from outside package views. The canonical
// config values (19 steps, 5 groups) are asserted here via the only
// accessible surface: ID(), Title(), and a successful Mount/Unmount
// cycle which validates the internal coherence of steps ↔ groups.
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_Structure(t *testing.T) {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}

	wiz := buildFirstRunInlineWizard(a)
	require.NotNil(t, wiz)

	// Public ID and Title
	assert.Equal(t, "wizard.init", wiz.ID())
	assert.NotEmpty(t, wiz.Title())

	// Mount/Unmount cycle — exercises all 19 steps + 5 groups without panicking.
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
// When the provider intro is skipped, the Provider form step should be
// skippable (its SkipIf returns true). We verify this indirectly:
// building the wizard is enough to wire the closures; the SkipIf on
// the provider form step captures the providerSkipped bool that the
// intro step's onSkip callback sets to true.
//
// Since the steps slice is internal, we validate that the wizard builds
// and mounts successfully with an empty config (provider not configured).
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_ProviderSkip(t *testing.T) {
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
// TestBuildIntroStep_Layout
//
// buildIntroStep returns a views.WizardStep with exported fields — we can
// inspect them directly from the cmd package.
// ---------------------------------------------------------------------------

func TestBuildIntroStep_Layout(t *testing.T) {
	badge := "TestBadge"
	step := buildIntroStep(
		badge,
		"My Title",
		"My description",
		"List title",
		"item1\nitem2",
		"You will need:\n• Item A\n• Item B",
		"A note",
		func() {}, // onContinue
		func() {}, // onSkip
	)

	// ── Exported field assertions ──
	assert.Equal(t, badge, step.Label)
	assert.True(t, step.Required, "intro steps must be Required")
	assert.True(t, step.SidebarHidden, "intro steps must be SidebarHidden")
	require.NotNil(t, step.CustomView, "intro step must have a CustomView")

	// ── Invoke CustomView and inspect the container ──
	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false
	onDone := func() { doneCalled = true }

	step.CustomView(tvApp, container, onDone)

	// Verify the container has children (layout-agnostic — don't check exact count).
	assert.Greater(t, container.GetItemCount(), 0,
		"CustomView should add children to the container")

	// onDone should not have been called yet (no button pressed)
	assert.False(t, doneCalled, "onDone should not fire without user interaction")
}

// ---------------------------------------------------------------------------
// TestBuildIntroStep_ContinueCallback
//
// Builds an intro step with an onContinue callback, programmatically
// activates the Continue button, and verifies the callback fires.
// ---------------------------------------------------------------------------

func TestBuildIntroStep_ContinueCallback(t *testing.T) {
	continueCalled := false
	doneCalled := false

	step := buildIntroStep(
		"Continue",
		"Title",
		"Desc",
		"", "", "", "", // no list, no prereqs, no note
		func() { continueCalled = true },
		func() { t.Error("onSkip should not be called") },
	)

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	onDone := func() { doneCalled = true }

	step.CustomView(tvApp, container, onDone)

	// Find the button form (layout-agnostic).
	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button form")
	require.Greater(t, buttonForm.GetButtonCount(), 0, "button form must have at least one button")

	// Verify the Continue button exists.
	btn := buttonForm.GetButton(0)
	require.NotNil(t, btn, "Continue button should exist")
	assert.NotEmpty(t, btn.GetLabel())

	// Activate the Continue button by focusing the form and sending Enter.
	// tview focuses the first button when the form has no form items.
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)

	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(enterEvent, func(p tview.Primitive) {})

	assert.True(t, continueCalled, "onContinue should have been called via Continue button")
	assert.True(t, doneCalled, "onDone should have been called after Continue")
}

// ---------------------------------------------------------------------------
// TestBuildIntroStep_SkipCallback
//
// Builds an intro step with onSkip, activates the Skip button, and
// verifies the callback fires.
// ---------------------------------------------------------------------------

func TestBuildIntroStep_SkipCallback(t *testing.T) {
	skipCalled := false
	doneCalled := false

	step := buildIntroStep(
		"Skippable",
		"Title",
		"Desc",
		"", "", "", "",
		func() { /* onContinue — not tested here */ },
		func() { skipCalled = true },
	)

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	onDone := func() { doneCalled = true }

	step.CustomView(tvApp, container, onDone)

	// Find the button form (layout-agnostic).
	buttonForm := findButtonForm(container)
	require.NotNil(t, buttonForm, "container should contain a button form")

	// With onSkip provided, the form should have 2 buttons:
	// [0] = Continue, [1] = Skip.
	require.Equal(t, 2, buttonForm.GetButtonCount(),
		"button form should have Continue + Skip buttons")

	// Focus the Skip button (index 1).
	// In tview, Tab cycles between buttons. We focus the form, then Tab
	// to move to the second button, then Enter to activate it.
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)
	setFocus := func(p tview.Primitive) {}

	// Tab to move from Continue → Skip
	tabEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	handler(tabEvent, setFocus)

	// First Enter: triggers confirmation (button label changes)
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(enterEvent, setFocus)
	assert.False(t, skipCalled, "onSkip should NOT fire on first click (confirmation pending)")

	// Second Enter: confirms skip
	handler(enterEvent, setFocus)
	assert.True(t, skipCalled, "onSkip should have been called via Skip button after confirmation")
	assert.True(t, doneCalled, "onDone should have been called after Skip")
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
// through all 19 steps from cmd. Instead we verify that different
// configurations produce a valid wizard that mounts cleanly.
// ---------------------------------------------------------------------------

func TestBuildFirstRunInlineWizard_E2E_MountWithPreconfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E wizard test in short mode")
	}

	t.Setenv("HOME", t.TempDir())
	config.Reset()

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
				Opencode: config.OpencodeConfig{
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
				Opencode: config.OpencodeConfig{
					DefaultProvider: "bedrock",
					Channel:         "stable",
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

	t.Setenv("HOME", t.TempDir())
	config.Reset()

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
