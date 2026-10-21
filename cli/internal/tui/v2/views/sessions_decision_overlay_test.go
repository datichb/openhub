package views

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlayShell records the overlays it shows and lets them be closed.
type overlayShell struct {
	cardShell
	omu    sync.Mutex
	seq    int
	cur    any
	closed int
	toasts []string
}

func (s *overlayShell) show() {
	s.omu.Lock()
	s.seq++
	s.cur = s.seq
	s.omu.Unlock()
}
func (s *overlayShell) ShowScrollableModal(title, body string, actions []ModalAction) {
	s.cardShell.ShowScrollableModal(title, body, actions)
	s.show()
}
func (s *overlayShell) ShowInlineForm(cfg InlineFormConfig) {
	s.cardShell.ShowInlineForm(cfg)
	s.show()
}
func (s *overlayShell) ShowToastMsg(msg string, _ bool) {
	s.omu.Lock()
	s.toasts = append(s.toasts, msg)
	s.omu.Unlock()
}
func (s *overlayShell) CurrentOverlay() any {
	s.omu.Lock()
	defer s.omu.Unlock()
	return s.cur
}
func (s *overlayShell) CloseOverlay(token any) bool {
	s.omu.Lock()
	defer s.omu.Unlock()
	if token == nil || token != s.cur {
		return false
	}
	s.cur = nil
	s.closed++
	return true
}
func (s *overlayShell) state() (closed int, toasts string) {
	s.omu.Lock()
	defer s.omu.Unlock()
	return s.closed, strings.Join(s.toasts, "\n")
}

func permissionRows() []SessionRow {
	return []SessionRow{{ID: "ses_a", Project: "openhub", Agent: "developer", State: "waiting", Started: time.Now(),
		Decisions: []SessionDecision{{ID: "perm:ses_a:1", SessionID: "ses_a", Kind: DecisionKindPermission, Icon: "🔒", Summary: "bash git status", Created: time.Now()}}}}
}

// A37: the window of a decision settled elsewhere (tool interface) closes
// with the reload of the list; one answered here does not announce it.
func TestDecisionSettledElsewhereClosesItsWindow(t *testing.T) {
	be := &fakeSessions{rows: permissionRows()}
	sh := &overlayShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	app := tview.NewApplication().SetScreen(sim)
	content := tview.NewFlex()
	app.SetRoot(content, true)
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	t.Cleanup(func() { app.Stop(); <-done })
	onLoop := func(fn func()) {
		ch := make(chan struct{})
		app.QueueUpdateDraw(func() { fn(); close(ch) })
		<-ch
	}
	open := func() {
		onLoop(func() {
			v.selectKey(refKey(decisionRef{id: "perm:ses_a:1", session: "ses_a"}))
			v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		})
		require.NotNil(t, sh.CurrentOverlay(), "permission form shown")
	}
	onLoop(func() { v.Mount(content, app); v.rows = permissionRows(); v.render() })

	// reload makes the backend answer rows and waits until the view applied
	// them (the same slice, not just equal rows), with its reaction to the
	// decisions that are gone.
	reload := func(rows []SessionRow) {
		be.mu.Lock()
		be.rows = rows
		be.mu.Unlock()
		onLoop(func() { v.reload() })
		require.Eventually(t, func() bool {
			applied := false
			onLoop(func() { applied = len(v.rows) == len(rows) && (len(rows) == 0 || &v.rows[0] == &rows[0]) })
			return applied
		}, 3*time.Second, time.Millisecond, "the reload applied")
	}

	// Still open after a reload: the window stays.
	open()
	reload(permissionRows())
	closed, _ := sh.state()
	assert.Zero(t, closed)

	// Answered in the tool interface: gone from the list → window closed.
	reload([]SessionRow{{ID: "ses_a", Project: "openhub", Agent: "developer", State: "active", Started: time.Now()}})
	require.Eventually(t, func() bool { c, ts := sh.state(); return c == 1 && ts != "" }, 3*time.Second, 20*time.Millisecond)
	_, toasts := sh.state()
	assert.Contains(t, toasts, "elsewhere")

	// Answered here: no "settled elsewhere".
	reload(permissionRows())
	open()
	sh.mu.Lock()
	form := sh.form
	sh.mu.Unlock()
	form.OnSubmit(map[string]string{"decision": "once"}, nil)
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, 3*time.Second, 20*time.Millisecond)
	reload(nil)
	closed, toasts = sh.state()
	assert.Equal(t, 1, closed)
	assert.Equal(t, 1, strings.Count(toasts, "elsewhere"))
	onLoop(func() { v.Unmount() })
}
