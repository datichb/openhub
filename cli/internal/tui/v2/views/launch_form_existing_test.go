package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// deleteShell records the confirmations and modals.
type deleteShell struct {
	recordingShell
	cmu      sync.Mutex
	confirms []recordedConfirm
	modal    []ModalAction
	body     string
}

type recordedConfirm struct {
	title, detail string
	danger        bool
	onYes         func()
}

func (s *deleteShell) Confirm(title, detail string, danger bool, onYes func()) {
	s.cmu.Lock()
	s.confirms = append(s.confirms, recordedConfirm{title, detail, danger, onYes})
	s.cmu.Unlock()
}

func (s *deleteShell) ShowScrollableModal(_ string, body string, actions []ModalAction) {
	s.cmu.Lock()
	s.body, s.modal = body, actions
	s.cmu.Unlock()
}

func (s *deleteShell) lastConfirm() (recordedConfirm, bool) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	if len(s.confirms) == 0 {
		return recordedConfirm{}, false
	}
	return s.confirms[len(s.confirms)-1], true
}

func (s *deleteShell) actions() []ModalAction {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return s.modal
}

func sameWorkflowExisting() LaunchExisting {
	return LaunchExisting{ID: "ses_old", Shared: []string{"bd-1"}, SameWorkflow: true, Target: true,
		State: "idle", StateLabel: "inactive", OpenDecisions: 2,
		Workflow: "ticket", PlannedWorkflow: "ticket", Version: 3, PlannedVersion: 3,
		Runtime: "container", PlannedRuntime: "local", Mode: "semi-auto", PlannedMode: "semi-auto",
		Model: "anthropic/opus", PlannedModel: "anthropic/opus", Path: "/p-bd-1", PlannedPath: "/p-bd-1", Branch: "feat/bd-1", PlannedBranch: "feat/bd-1",
		WorkKnown: true, CommitsUnpushed: 4, ChangedFiles: 1, Cost: 1.5,
		Diffs:   []string{LaunchDiffRuntime},
		Actions: []string{LaunchExistingResume, LaunchExistingTransfer, LaunchExistingKeep, LaunchExistingDelete, LaunchExistingParallel}}
}

type existingForm struct {
	v        *LaunchFormView
	app      *tview.Application
	sh       *deleteShell
	mu       sync.Mutex
	launched []LaunchChoices
	err      func(LaunchChoices) error
}

func (f *existingForm) launches() []LaunchChoices {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]LaunchChoices(nil), f.launched...)
}

// assertNoLaunch checks that nothing launched, without waiting: a launch sets
// launching on the event loop before its goroutine runs, and clears it only
// once the launch function returned (after it recorded the launch).
func (f *existingForm) assertNoLaunch(t *testing.T, msgAndArgs ...any) {
	t.Helper()
	var launching bool
	onLoop(f.app, func() { launching = f.v.launching })
	assert.False(t, launching, msgAndArgs...)
	assert.Empty(t, f.launches(), msgAndArgs...)
}

func (f *existingForm) recapText() string {
	var txt string
	onLoop(f.app, func() { txt = f.v.recapTV.GetText(true) })
	return txt
}

func (f *existingForm) key(r rune) {
	onLoop(f.app, func() { f.v.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)) })
}

// newExistingForm opens a form on its recap, the recap holding existing.
func newExistingForm(t *testing.T, needsChoice bool, existing ...LaunchExisting) *existingForm {
	t.Helper()
	f := &existingForm{sh: &deleteShell{}}
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) {
		return &LaunchRecap{Rows: []InfoField{{Label: "Agents", Value: "developer"}}, Existing: existing, NeedsChoice: needsChoice}, nil
	}
	cfg.Launch = func(_ context.Context, c LaunchChoices) error {
		f.mu.Lock()
		f.launched = append(f.launched, c)
		fn := f.err
		f.mu.Unlock()
		if fn != nil {
			return fn(c)
		}
		return nil
	}
	cfg.PlanDelete = func(_ context.Context, id string) (DeleteSummary, error) {
		return DeleteSummary{SessionID: id, Items: []string{"  → worktree /p-bd-1", "  ⚠ 4 commit(s) perdus"}, Losses: true, BaseChanges: 2}, nil
	}
	f.v = NewLaunchFormView(cfg)
	f.v.SetShell(f.sh)
	content := tview.NewFlex()
	f.app = runApp(t, content)
	onLoop(f.app, func() { f.v.Mount(content, f.app); f.v.toRecap() })
	require.Eventually(t, func() bool { return strings.Contains(f.recapText(), "developer") }, 2*time.Second, 10*time.Millisecond)
	return f
}

func focusedButton(app *tview.Application) string {
	var label string
	onLoop(app, func() {
		if b, ok := app.GetFocus().(*tview.Button); ok {
			label = widgets.ButtonLabel(b)
		}
	})
	return label
}

// Same workflow: the comparison block marks the differing rows, « Lancer »
// gives way to the allowed choices, « Reprendre » focused.
func TestLaunchFormExistingSameWorkflow(t *testing.T) {
	f := newExistingForm(t, true, sameWorkflowExisting())
	txt := f.recapText()
	assert.Contains(t, txt, i18n.Tf("tui.launch.existing.same", "ses_old", "bd-1"))
	assert.Contains(t, txt, i18n.T("tui.launch.existing.col.existing"))
	assert.Contains(t, txt, i18n.T("tui.launch.existing.col.new"))
	lineOf := func(label string) string {
		for _, l := range strings.Split(txt, "\n") {
			if strings.Contains(l, " "+label+" ") {
				return l
			}
		}
		t.Fatalf("no row %q in %s", label, txt)
		return ""
	}
	assert.True(t, strings.HasPrefix(lineOf(i18n.T("tui.launch.existing.row.runtime")), "⚠"), "runtime differs")
	assert.Contains(t, lineOf(i18n.T("tui.launch.existing.row.runtime")), i18n.T("tui.launch.runtime_container"))
	assert.False(t, strings.HasPrefix(lineOf(i18n.T("tui.launch.existing.row.mode")), "⚠"), "same mode")
	assert.Contains(t, lineOf(i18n.T("tui.launch.existing.row.state")), i18n.Tf("tui.sessions.waiting_decisions", 2))
	assert.Contains(t, lineOf(i18n.T("tui.launch.existing.row.workflow")), "ticket v3")
	assert.Contains(t, lineOf(i18n.T("tui.launch.existing.row.location")), "feat/bd-1")
	assert.Contains(t, lineOf(i18n.T("tui.launch.existing.row.work")), i18n.Tf("tui.launch.existing.work", 4, 1, 1.5))

	assert.Equal(t, []string{i18n.T("tui.launch.back"), i18n.T("tui.launch.existing.choice.resume"),
		i18n.T("tui.launch.existing.choice.transfer_local"), i18n.T("tui.launch.existing.choice.keep"),
		i18n.T("tui.launch.existing.choice.delete"), i18n.T("tui.launch.existing.choice.parallel")}, recapButtons(f.app, f.v))
	assert.Equal(t, i18n.T("tui.launch.existing.choice.resume"), focusedButton(f.app))
	assert.Equal(t, i18n.T("tui.keys.launch_recap_existing"), f.v.StatusHints())

	// Ctrl+S does not launch without a choice.
	onLoop(f.app, func() { f.v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)) })
	f.assertNoLaunch(t)
	f.sh.mu.Lock()
	assert.Contains(t, f.sh.toasts, i18n.T("tui.launch.existing.choose"))
	f.sh.mu.Unlock()
}

// Another workflow: informational, « Lancer » (in parallel) stays focused.
func TestLaunchFormExistingOtherWorkflow(t *testing.T) {
	e := sameWorkflowExisting()
	e.SameWorkflow, e.Workflow, e.Diffs = false, "review", []string{LaunchDiffWorkflow, LaunchDiffRuntime}
	e.Actions = []string{LaunchExistingResume, LaunchExistingKeep, LaunchExistingDelete, LaunchExistingParallel}
	f := newExistingForm(t, false, e)
	assert.Contains(t, f.recapText(), i18n.Tf("tui.launch.existing.other", "ses_old", "review", "bd-1"))
	assert.Equal(t, []string{i18n.T("tui.launch.back"), i18n.T("tui.launch.launch"), i18n.T("tui.launch.existing.choice.resume"),
		i18n.T("tui.launch.existing.choice.keep"), i18n.T("tui.launch.existing.choice.delete")}, recapButtons(f.app, f.v))
	assert.Equal(t, i18n.T("tui.launch.launch"), focusedButton(f.app))

	onLoop(f.app, func() { f.v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)) })
	require.Eventually(t, func() bool { return len(f.launches()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "", f.launches()[0].OnExisting, "plain launch: in parallel")
}

// Only the allowed choices (a remote session: in parallel only).
func TestLaunchFormExistingOnlyAllowedChoices(t *testing.T) {
	e := sameWorkflowExisting()
	e.Actions = []string{LaunchExistingParallel}
	f := newExistingForm(t, true, e)
	assert.Equal(t, []string{i18n.T("tui.launch.back"), i18n.T("tui.launch.existing.choice.parallel")}, recapButtons(f.app, f.v))
	assert.Equal(t, i18n.T("tui.launch.existing.choice.parallel"), focusedButton(f.app))
	f.key('R') // not allowed
	f.assertNoLaunch(t)
}

// R / T / k / p launch at once with the choice (R: the resume key of the
// convention; r stays refresh, t is no longer a choice).
func TestLaunchFormExistingKeys(t *testing.T) {
	f := newExistingForm(t, true, sameWorkflowExisting())
	f.key('r')
	f.key('t')
	f.assertNoLaunch(t, "r and t launch nothing")
	for key, want := range map[rune]string{'R': LaunchExistingResume, 'T': LaunchExistingTransfer, 'k': LaunchExistingKeep, 'p': LaunchExistingParallel} {
		t.Run(want, func(t *testing.T) {
			f := newExistingForm(t, true, sameWorkflowExisting())
			f.key(key)
			require.Eventually(t, func() bool { return len(f.launches()) == 1 }, 2*time.Second, 10*time.Millisecond)
			assert.Equal(t, want, f.launches()[0].OnExisting)
			require.Eventually(t, func() bool {
				f.sh.mu.Lock()
				defer f.sh.mu.Unlock()
				return f.sh.popped == 1
			}, 2*time.Second, 10*time.Millisecond, "closed after the launch")
		})
	}
}

// d: the losses are confirmed (danger), then the changes of the project
// directory are kept or discarded, then the launch deletes.
func TestLaunchFormExistingDeleteConfirms(t *testing.T) {
	f := newExistingForm(t, true, sameWorkflowExisting())
	f.key('d')
	var c recordedConfirm
	require.Eventually(t, func() bool {
		var ok bool
		c, ok = f.sh.lastConfirm()
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	assert.True(t, c.danger)
	assert.Equal(t, i18n.Tf("tui.launch.existing.delete_title", "ses_old"), c.title)
	assert.Contains(t, c.detail, "4 commit(s) perdus")
	assert.Contains(t, c.detail, i18n.T("tui.delete.losses"))
	assert.Empty(t, f.launches(), "not before the confirmation")

	onLoop(f.app, c.onYes)
	acts := f.sh.actions()
	require.Len(t, acts, 2)
	assert.True(t, acts[1].Danger)
	assert.Empty(t, f.launches(), "not before the second question")
	onLoop(f.app, acts[1].Callback) // discard them too
	require.Eventually(t, func() bool { return len(f.launches()) == 1 }, 2*time.Second, 10*time.Millisecond)
	got := f.launches()[0]
	assert.Equal(t, LaunchExistingDelete, got.OnExisting)
	assert.True(t, got.DiscardBaseChanges)
}

// A failed transfer offers to replace the session while keeping its work.
func TestLaunchFormExistingTransferFallback(t *testing.T) {
	f := newExistingForm(t, true, sameWorkflowExisting())
	f.mu.Lock()
	f.err = func(c LaunchChoices) error {
		if c.OnExisting == LaunchExistingTransfer {
			return TransferFailedError("transfert impossible : boom")
		}
		return nil
	}
	f.mu.Unlock()
	f.key('T')
	var c recordedConfirm
	require.Eventually(t, func() bool {
		var ok bool
		c, ok = f.sh.lastConfirm()
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, i18n.T("tui.launch.existing.transfer_failed_title"), c.title)
	assert.Contains(t, c.detail, "boom")
	assert.Contains(t, f.recapText(), "boom")
	onLoop(f.app, c.onYes)
	require.Eventually(t, func() bool { return len(f.launches()) == 2 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, LaunchExistingKeep, f.launches()[1].OnExisting)
}

// Launched runs once the form closed after a successful launch.
func TestLaunchFormLaunchedCallback(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) { return &LaunchRecap{}, nil }
	cfg.Launch = func(context.Context, LaunchChoices) error { return nil }
	called := make(chan struct{}, 1)
	cfg.Launched = func() { called <- struct{}{} }
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app); v.toRecap() })
	require.Eventually(t, func() bool {
		loaded := false
		onLoop(app, func() { loaded = v.recap != nil })
		return loaded
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() { v.launch() })
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("Launched not called")
	}
}

// D in the Sessions view: plan, confirmation (danger), second question for
// the changes of the project directory, deletion, toast and reload.
type deletingSessions struct {
	fakeSessions
	deleted []string
}

func (f *deletingSessions) PlanDelete(_ context.Context, id string) (DeleteSummary, error) {
	return DeleteSummary{SessionID: id, Items: []string{"  → branche locale feat/x"}, BaseChanges: 1}, nil
}

func (f *deletingSessions) DeleteSession(_ context.Context, id string, discard bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := id
	if discard {
		entry += ":discard"
	}
	f.deleted = append(f.deleted, entry)
	return "Session " + id + " supprimée", nil
}

func (f *deletingSessions) deletions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

func TestSessionsViewDelete(t *testing.T) {
	be := &deletingSessions{fakeSessions: fakeSessions{rows: sampleRows()}}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	sh := &deleteShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n := 0
		onLoop(app, func() { n = len(v.rows) })
		return n > 0
	}, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, v.HelpKeys(), "d ") // in the help (?), no longer in the essential hints
	onLoop(app, func() {
		v.Focus("ses_b")
		v.render()
		assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)))
	})
	var c recordedConfirm
	require.Eventually(t, func() bool {
		var ok bool
		c, ok = sh.lastConfirm()
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	assert.True(t, c.danger)
	assert.Contains(t, c.title, "b") // short id of ses_b
	assert.Contains(t, c.detail, "feat/x")
	assert.Empty(t, be.deletions())
	onLoop(app, c.onYes)
	acts := sh.actions()
	require.Len(t, acts, 2)
	onLoop(app, acts[0].Callback) // keep the changes
	require.Eventually(t, func() bool { return len(be.deletions()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "ses_b", be.deletions()[0])
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		for _, m := range sh.toasts {
			if m == "Session ses_b supprimée" {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() { v.Unmount() })
}
