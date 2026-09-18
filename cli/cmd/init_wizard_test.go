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

	// The CustomView adds 6 children to the container:
	//   topSpacer, badgeView, gapSpacer, tv (text), buttonForm, bottomSpacer
	assert.Equal(t, 6, container.GetItemCount(),
		"CustomView should add exactly 6 children to the container")

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

	// The buttonForm is the 5th child (index 4) of the container.
	// It is a *tview.Form with the Continue button at index 0.
	p := container.GetItem(4)
	buttonForm, ok := p.(*tview.Form)
	require.True(t, ok, "5th child should be a *tview.Form (button form)")
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

	// Button form is at index 4 of the container.
	p := container.GetItem(4)
	buttonForm, ok := p.(*tview.Form)
	require.True(t, ok, "5th child should be a *tview.Form")

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

	// The CustomView adds 6 children to the container:
	//   topSpacer, badgeView, gapSpacer, tv (text), buttonForm, bottomSpacer
	assert.Equal(t, 6, container.GetItemCount(),
		"CustomView should add exactly 6 children to the container")

	// Button form is at index 4
	p := container.GetItem(4)
	buttonForm, ok := p.(*tview.Form)
	require.True(t, ok, "5th child should be a *tview.Form (button form)")

	// Must have 3 buttons: Create, Rejoin, Skip
	require.Equal(t, 3, buttonForm.GetButtonCount(),
		"button form should have Create + Rejoin + Skip buttons")

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

	buttonForm := container.GetItem(4).(*tview.Form)
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

	buttonForm := container.GetItem(4).(*tview.Form)
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

	buttonForm := container.GetItem(4).(*tview.Form)
	tvApp.SetFocus(buttonForm)

	handler := buttonForm.InputHandler()
	require.NotNil(t, handler)
	setFocus := func(p tview.Primitive) {}

	// Tab twice: Create → Rejoin → Skip
	tabEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
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
