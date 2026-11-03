package shell

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Mouse sweep of the Sessions view mounted in the real shell: every cell of
// the screen gets clicks (single, double, triple), drags, the wheel and the
// other buttons, in each state of the view. A panic of the event loop fails
// the test with its stack.

// fakeSessionsBackend is a Sessions backend with every optional capability
// the real one has, over fixed rows.
type fakeSessionsBackend struct {
	rows    []views.SessionRow
	listErr error
	mu      sync.Mutex
	listed  int
}

func (f *fakeSessionsBackend) List(context.Context, string, bool) ([]views.SessionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listed++
	return f.rows, f.listErr
}
func (f *fakeSessionsBackend) Changes(ctx context.Context) <-chan struct{} {
	return make(chan struct{})
}
func (f *fakeSessionsBackend) Follow(ctx context.Context, _ string) (<-chan views.SessionFeedLine, error) {
	ch := make(chan views.SessionFeedLine, 2)
	ch <- views.SessionFeedLine{Time: time.Now(), Text: "tool: read file.go", Cost: 0.01}
	ch <- views.SessionFeedLine{Time: time.Now(), Text: "assistant: done"}
	return ch, nil
}
func (f *fakeSessionsBackend) Decide(context.Context, string, string, string, map[string]string) error {
	return nil
}
func (f *fakeSessionsBackend) Send(context.Context, string, string) error        { return nil }
func (f *fakeSessionsBackend) Interrupt(context.Context, string) error           { return nil }
func (f *fakeSessionsBackend) SwitchModel(context.Context, string, string) error { return nil }
func (f *fakeSessionsBackend) Stop(context.Context, string) error                { return nil }
func (f *fakeSessionsBackend) Resume(context.Context, string) error              { return nil }
func (f *fakeSessionsBackend) Attach(string, string)                             {}
func (f *fakeSessionsBackend) OpenBrowser(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeSessionsBackend) MRDescription(context.Context, string) (string, error) {
	return "MR description", nil
}

func (f *fakeSessionsBackend) NextSteps(context.Context, string) ([]views.SessionNextStep, error) {
	return []views.SessionNextStep{{Workflow: "review", Label: "Review", Inputs: "branch=feat/x"}}, nil
}

func (f *fakeSessionsBackend) SessionTrace(context.Context, string) (string, error) {
	var b strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&b, "%03d [tool] step %d with a [bracket] and a very long line to wrap around the reader %s\n", i, i, strings.Repeat("x", i%90))
	}
	return b.String(), nil
}

func (f *fakeSessionsBackend) SessionReports(context.Context, string) ([]views.CheckpointReport, error) {
	return []views.CheckpointReport{
		{ID: "rp-1", Agent: "dev", Title: "Report one", Bytes: 120, At: time.Now()},
		{ID: "rp-2", Agent: "qa", Title: "Report two", Bytes: 4000, At: time.Now(), Session: "ses_child1", Child: "oh-1"},
	}, nil
}

func (f *fakeSessionsBackend) CheckpointReport(context.Context, string, string) (string, string, error) {
	return "Report", strings.Repeat("line of the report [x]\n", 80), nil
}

func (f *fakeSessionsBackend) CheckpointCard(_ context.Context, id string) (views.CheckpointCardView, error) {
	return views.CheckpointCardView{
		Checkpoint: "plan", Label: "Plan validation", Summary: "The plan",
		Files:     []views.CheckpointFile{{File: "a.go", Status: "M", Additions: 3, Deletions: 1}},
		Additions: 3, Deletions: 1, Patch: "@@ -1 +1 @@\n-a\n+b\n",
		Messages: []string{"dev › hello"},
		Reports:  []views.CheckpointReport{{ID: "rp-1", Agent: "dev", Title: "Report one", Bytes: 10}},
		Wiki:     []views.CheckpointWikiProposal{{ID: "w-1", Agent: "dev", Page: "Home", Status: "pending"}},
	}, nil
}

func (f *fakeSessionsBackend) Fanout(_ context.Context, id string) (views.FanoutView, error) {
	return views.FanoutView{
		Parent: "ses_parent", Status: "running", Parallel: 2, Driver: true, BudgetUSD: 5, SpentUSD: 1.5, FixMax: 2,
		Children: []views.FanoutChildView{
			{Ticket: "oh-1", Session: "ses_child1", State: "running", Branch: "feat/oh-1", CostUSD: 0.5},
			{Ticket: "oh-2", Session: "ses_child2", State: "conflict", Branch: "feat/oh-2", Detail: "a.go"},
			{Ticket: "oh-3", State: "pending"},
		},
		Integration: "integ/x", Commits: -1,
	}, nil
}
func (f *fakeSessionsBackend) RelaunchFanout(context.Context, string) (string, error) {
	return "relaunched", nil
}
func (f *fakeSessionsBackend) CleanFanout(context.Context, string, bool, bool) (views.FanoutCleanView, error) {
	return views.FanoutCleanView{Worktrees: []string{"/tmp/wt"}, Branches: []string{"feat/oh-1"}}, nil
}

func (f *fakeSessionsBackend) BatchItems(context.Context, string) ([]views.BatchDecision, error) {
	return []views.BatchDecision{
		{ID: "d-cp1", SessionID: "ses_inbox", Checkpoint: "plan", Summary: "plan", Verify: "pass"},
		{ID: "d-cp2", SessionID: "ses_child1", Checkpoint: "merge", Summary: "merge", Verify: "fail"},
	}, nil
}
func (f *fakeSessionsBackend) ApplyBatch(_ context.Context, c []views.BatchChoice) ([]views.BatchResult, error) {
	return nil, nil
}

func (f *fakeSessionsBackend) FetchRemote(context.Context, string) (views.RemoteFetchReport, error) {
	return views.RemoteFetchReport{Lines: []string{"fetched"}}, nil
}
func (f *fakeSessionsBackend) PlanReplay(context.Context, string) (*views.ReplayView, error) {
	return &views.ReplayView{Lines: []string{"entry 1"}, Pending: 1}, nil
}
func (f *fakeSessionsBackend) ApplyReplay(context.Context, string, map[string]string) (*views.ReplayView, error) {
	return &views.ReplayView{}, nil
}

func (f *fakeSessionsBackend) ChainOptions(context.Context, string) ([]views.SelectOption, error) {
	return []views.SelectOption{{Label: "Review", Value: "review"}}, nil
}
func (f *fakeSessionsBackend) Chain(string, string) {}

func (f *fakeSessionsBackend) PlanDelete(context.Context, string) (views.DeleteSummary, error) {
	return views.DeleteSummary{}, nil
}
func (f *fakeSessionsBackend) DeleteSession(context.Context, string, bool) (string, error) {
	return "deleted", nil
}

// sessionsMouseRows holds every kind of session the view shows.
func sessionsMouseRows() []views.SessionRow {
	now := time.Now()
	dec := func(id, session, kind string, age time.Duration) views.SessionDecision {
		return views.SessionDecision{ID: id, SessionID: session, Kind: kind, Icon: "?", Summary: kind + " " + id,
			Message: "message of " + id, Created: now.Add(-age)}
	}
	question := dec("d-q1", "ses_question", views.DecisionKindQuestion, 3*time.Minute)
	question.Fields = []views.SessionDecisionField{
		{Key: "color", Title: "Color", Type: "string", Options: []views.SelectOption{{Label: "Red", Value: "red"}, {Label: "Blue", Value: "blue"}}, Custom: true},
		{Key: "n", Title: "Count", Type: "integer", Required: true},
		{Key: "ok", Title: "Sure", Type: "boolean"},
		{Key: "many", Title: "Many", Type: "multiselect", Options: []views.SelectOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}},
	}
	return []views.SessionRow{
		{ID: "ses_inbox", ProjectID: "p1", Project: "alpha", Workflow: "feature", Agent: "orchestrator-dev", Mode: "semi-auto", Runtime: "local",
			Location: "/work/alpha", Bundle: "abcdef0123456789", State: "waiting", StateLabel: "waiting", StateIcon: "◐", Cost: 1.2,
			Started: now.Add(-time.Hour), Changed: now.Add(-time.Minute), Timeline: "plan ✓ → dev ● → review ○",
			Decisions: []views.SessionDecision{dec("d-cp1", "ses_inbox", views.DecisionKindCheckpoint, 5*time.Minute)},
			Outputs:   []views.SessionOutput{{Name: "branch", Value: "feat/x"}}, Next: "review"},
		{ID: "ses_perm", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "waiting", StateLabel: "waiting", StateIcon: "◐",
			Started: now.Add(-2 * time.Hour), Changed: now.Add(-time.Minute), Title: "perm",
			Decisions: []views.SessionDecision{dec("d-p1", "ses_perm", views.DecisionKindPermission, time.Minute), dec("d-b1", "ses_perm", views.DecisionKindBudget, 2*time.Minute), dec("d-e1", "ses_perm", views.DecisionKindError, 4*time.Minute)}},
		{ID: "ses_question", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "waiting", StateLabel: "waiting", StateIcon: "◐",
			Started: now.Add(-time.Hour), Changed: now.Add(-time.Minute), Decisions: []views.SessionDecision{question}},
		{ID: "ses_run", ProjectID: "p1", Project: "alpha", Workflow: "fix", Agent: "dev", State: "active", StateLabel: "running", StateIcon: "●",
			Cost: 0.3, Started: now.Add(-10 * time.Minute), Changed: now},
		{ID: "ses_sleep", ProjectID: "p2", Project: "beta", Agent: "dev", State: "sleeping", StateLabel: "sleeping", StateIcon: "○",
			Started: now.Add(-3 * time.Hour), Changed: now.Add(-time.Hour)},
		{ID: "ses_done", ProjectID: "p2", Project: "beta", Workflow: "feature", Agent: "dev", State: "completed", StateLabel: "completed", StateIcon: "✓",
			Finished: true, Cost: 4, Started: now.Add(-5 * time.Hour), Changed: now.Add(-4 * time.Hour)},
		{ID: "ses_failed", ProjectID: "p2", Project: "beta", Agent: "dev", State: "failed", StateLabel: "failed", StateIcon: "✗",
			Finished: true, Started: now.Add(-5 * time.Hour), Changed: now.Add(-4 * time.Hour)},
		{ID: "ses_noproject", Agent: "dev", State: "active", StateLabel: "running", StateIcon: "●", Started: now.Add(-time.Minute)},
		{ID: "ses_gone", ProjectID: "p-deleted", Agent: "dev", State: "idle", StateLabel: "idle", StateIcon: "●", Started: now.Add(-time.Minute)},
		{ID: "ses_queued", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "queued", StateLabel: "queued", StateIcon: "…"},
		{ID: "ses_remote_ready", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "completed", StateLabel: "completed", StateIcon: "✓",
			Runtime: "remote", Remote: views.RemoteStatusReady, RemoteDetail: "pipeline #12 · MR !3", Finished: true, Changed: now},
		{ID: "ses_remote_run", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "active", StateLabel: "running", StateIcon: "●",
			Runtime: "remote", Remote: views.RemoteStatusRunning, Started: now},
		{ID: "ses_parent", ProjectID: "p1", Project: "alpha", Workflow: "fanout", Agent: "orchestrator", State: "active", StateLabel: "running", StateIcon: "●",
			FanoutRole: views.FanoutRoleParent, FanoutLine: "2/3 children", Started: now.Add(-time.Hour), Changed: now},
		{ID: "ses_child1", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "waiting", StateLabel: "waiting", StateIcon: "◐",
			FanoutRole: views.FanoutRoleChild, FanoutParent: "ses_parent", Started: now.Add(-time.Hour), Changed: now,
			Decisions: []views.SessionDecision{dec("d-c1", "ses_child1", views.DecisionKindCheckpoint, 6*time.Minute)}},
		{ID: "ses_child2", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "active", StateLabel: "running", StateIcon: "●",
			FanoutRole: views.FanoutRoleChild, FanoutParent: "ses_parent", Started: now.Add(-time.Hour), Changed: now},
		{ID: "ses_orphan", ProjectID: "p1", Project: "alpha", Agent: "dev", State: "active", StateLabel: "running", StateIcon: "●",
			FanoutRole: views.FanoutRoleChild, FanoutParent: "ses_nowhere", Started: now},
	}
}

// panicSim is a shell on a simulation screen whose event loop panic is
// caught and reported (with the stack of the panic) instead of crashing
// the test binary.
type panicSim struct {
	*simShell
	crash chan string
}

func runPanicSim(t *testing.T, cols, rows int, cfg Config) *panicSim {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	s := New(cfg)
	s.selection.copy = func(string) error { return nil } // not the real clipboard
	s.selection.onCopy = nil                             // nor a toast per click
	s.app.SetScreen(sim)
	sim.SetSize(cols, rows)
	done := make(chan struct{})
	crash := make(chan string, 1)
	go func() {
		defer close(done)
		defer func() {
			if p := recover(); p != nil {
				crash <- fmt.Sprintf("%v\n%s", p, debug.Stack())
			}
		}()
		_ = s.Run()
	}()
	t.Cleanup(func() {
		select {
		case <-crash:
		default:
			s.app.Stop()
		}
		<-done
	})
	ss := &simShell{Shell: s, sim: sim, fence: make(chan struct{}, 1), t: t}
	global := s.app.GetInputCapture()
	s.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == fenceKey {
			ss.fence <- struct{}{}
			return nil
		}
		return global(event)
	})
	require.Eventually(t, func() bool { return s.IsRunning() }, 2*time.Second, 5*time.Millisecond)
	return &panicSim{simShell: ss, crash: crash}
}

// settle waits until the event loop handled everything injected before.
func (p *panicSim) settle() {
	p.t.Helper()
	p.sim.InjectKey(fenceKey, 0, tcell.ModNone)
	select {
	case <-p.fence:
	case c := <-p.crash:
		p.t.Fatalf("panic of the event loop:\n%s", c)
	case <-time.After(20 * time.Second):
		p.t.Fatal("the event loop did not handle the events")
	}
}

func (p *panicSim) press(k tcell.Key, r rune) {
	p.t.Helper()
	p.sim.InjectKey(k, r, tcell.ModNone)
	p.settle()
	p.loopSafe(func() {})
}

// loopSafe is loop, but a panic of the loop fails with its stack.
func (p *panicSim) loopSafe(fn func()) {
	p.t.Helper()
	ch := make(chan struct{})
	p.app.QueueUpdateDraw(func() { fn(); close(ch) })
	select {
	case <-ch:
	case c := <-p.crash:
		p.t.Fatalf("panic of the event loop:\n%s", c)
	case <-time.After(20 * time.Second):
		p.t.Fatal("the event loop did not run the update")
	}
}

// text is the screen as text, read on the event loop: the loop draws into
// the simulation screen, reading it from the test goroutine races with it.
func (p *panicSim) text() (s string) {
	p.loopSafe(func() { s = p.screenText() })
	return s
}

func (p *panicSim) screenText() string {
	cells, w, h := p.sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 || c.Runes[0] == 0 {
				b.WriteByte(' ')
				continue
			}
			b.WriteRune(c.Runes[0])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (p *panicSim) mouse(x, y int, buttons tcell.ButtonMask) {
	p.sim.InjectMouse(x, y, buttons, tcell.ModNone)
}

// mouseFull sweeps every cell with every action, in every state (several
// minutes): go test -run TestSessionsViewMouseSweep ./internal/tui/v2/shell/ -args -mouse-full
var mouseFull = flag.Bool("mouse-full", false, "sweep every cell of the screen in every state")

// sweep sends, on the cells of a grid (every xStep-th column of every
// yStep-th row, shifted from row to row), the clicks of the three buttons
// (consecutive left clicks make the double and triple clicks), a drag to the
// opposite corner and the wheel.
func (p *panicSim) sweep(cols, rows, xStep, yStep int) {
	p.t.Helper()
	switch {
	case *mouseFull:
		xStep, yStep = 1, 1
	case raceEnabled:
		xStep, yStep = xStep*2, yStep*2
	}
	n := 0
	for y := 0; y < rows; y += yStep {
		for x := (y / yStep) % xStep; x < cols; x += xStep {
			n++
			p.mouse(x, y, tcell.Button1)
			p.mouse(x, y, tcell.ButtonNone)
			p.mouse(x, y, tcell.WheelUp)
			p.mouse(x, y, tcell.WheelDown)
			p.mouse(x, y, tcell.ButtonNone)
			if n%3 == 0 || *mouseFull {
				p.mouse(x, y, tcell.Button2)
				p.mouse(x, y, tcell.ButtonNone)
				p.mouse(x, y, tcell.Button3)
				p.mouse(x, y, tcell.ButtonNone)
				p.mouse(x, y, tcell.WheelLeft)
				p.mouse(x, y, tcell.WheelRight)
				p.mouse(x, y, tcell.ButtonNone)
			}
			if n%3 == 1 || *mouseFull {
				p.mouse(x, y, tcell.Button1)
				p.mouse((x+7)%cols, (y+3)%rows, tcell.Button1)
				p.mouse(cols-1-x, rows-1-y, tcell.Button1)
				p.mouse(cols-1-x, rows-1-y, tcell.ButtonNone)
			}
		}
	}
	p.settle()
	p.loopSafe(func() {})
}

type step struct {
	key tcell.Key
	r   rune
}

func k(key tcell.Key) step { return step{key: key} }
func r(c rune) step        { return step{key: tcell.KeyRune, r: c} }

// TestSessionsViewMouseSweep mounts the Sessions view in the shell and sweeps
// the mouse over the screen in each state of the view: every kind of session,
// each tab, with the cards, forms, modals and the palette opened over it.
func TestSessionsViewMouseSweep(t *testing.T) {
	const cols, rows = 120, 36
	tab := k(tcell.KeyTab)
	enter := k(tcell.KeyEnter)
	scenarios := []struct {
		name    string
		backend func() *fakeSessionsBackend
		project *views.ActiveProject
		focus   string // session ("ses_…") or decision ("d:…") to select
		steps   []step
		want    string // what the screen shows once there
		fine    bool   // swept on a fine grid (else a coarse one)
		extra   bool   // only with -mouse-full
	}{
		{name: "list, first decision", fine: true, want: "To handle"},
		{name: "detail of a checkpoint", extra: true, focus: "ses_inbox", want: "plan"},
		{name: "trace tab", fine: true, focus: "ses_inbox", steps: []step{tab}, want: "step 0"},
		{name: "trace search", extra: true, focus: "ses_inbox", steps: []step{tab, r('/')}, want: "Search"},
		{name: "reports tab", fine: true, focus: "ses_inbox", steps: []step{tab, tab}, want: "line of the report"},
		{name: "checkpoint card", fine: true, focus: "d:d-cp1", steps: []step{enter}, want: "Plan validation"},
		{name: "permission decision", extra: true, focus: "d:d-p1", steps: []step{enter}, want: "permission d-p1"},
		{name: "budget decision", extra: true, focus: "d:d-b1", steps: []step{enter}, want: "budget d-b1"},
		{name: "error decision", extra: true, focus: "d:d-e1", steps: []step{enter}, want: "error d-e1"},
		{name: "question form", fine: true, focus: "d:d-q1", steps: []step{enter}, want: "Color"},
		{name: "live feed", focus: "ses_run", steps: []step{r('t')}, want: "assistant: done"},
		{name: "batch", fine: true, steps: []step{r('B')}, want: "Validate by batch"},
		{name: "running session", extra: true, focus: "ses_run", want: "fix"},
		{name: "sleeping session", extra: true, focus: "ses_sleep", want: "beta"},
		{name: "finished session", extra: true, focus: "ses_done", want: "completed"},
		{name: "failed session", extra: true, focus: "ses_failed", want: "failed"},
		{name: "queued session", extra: true, focus: "ses_queued", want: "queued"},
		{name: "no project", focus: "ses_noproject", want: "ses_noproject"},
		{name: "deleted project", extra: true, focus: "ses_gone", want: "ses_gone"},
		{name: "remote to fetch", extra: true, focus: "ses_remote_ready", want: "pipeline #12"},
		{name: "remote running", extra: true, focus: "ses_remote_run", want: "ses_remote_run"},
		{name: "fan-out parent", extra: true, focus: "ses_parent", want: "2/3 children"},
		{name: "fan-out tab", fine: true, focus: "ses_parent", steps: []step{tab, tab, tab}, want: "oh-2"},
		{name: "fan-out child, reports", extra: true, focus: "ses_child1", steps: []step{tab, tab}, want: "line of the report"},
		{name: "fan-out orphan child", extra: true, focus: "ses_orphan", want: "ses_orphan"},
		{name: "send modal", fine: true, focus: "ses_run", steps: []step{r('m')}, want: "nstruction"},
		{name: "stop modal", focus: "ses_run", steps: []step{r('s')}, want: "Stop"},
		{name: "attach modal", extra: true, focus: "ses_run", steps: []step{r('A')}, want: "tmux"},
		{name: "chain modal", extra: true, focus: "ses_done", steps: []step{r('e')}, want: "Review"},
		{name: "merge request", extra: true, focus: "ses_done", steps: []step{r('o')}, want: "MR description"},
		{name: "help", steps: []step{r('?')}, want: "Global shortcuts"},
		{name: "palette", steps: []step{k(tcell.KeyCtrlP)}, want: "Hub"},
		{name: "empty", fine: true, backend: func() *fakeSessionsBackend { return &fakeSessionsBackend{} }},
		{name: "list error", fine: true, backend: func() *fakeSessionsBackend { return &fakeSessionsBackend{listErr: errors.New("db is locked")} }, want: "db is locked"},
		{name: "one project", project: &views.ActiveProject{ID: "p1", Name: "alpha"}, want: "To handle"},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			if sc.extra && !*mouseFull {
				t.Skip("swept with -mouse-full")
			}
			if raceEnabled && !sc.fine && !*mouseFull {
				t.Skip("the race detector slows the sweep: the main states only")
			}
			be := &fakeSessionsBackend{rows: sessionsMouseRows()}
			if sc.backend != nil {
				be = sc.backend()
			}
			sv := views.NewSessionsView(views.SessionsViewConfig{Backend: be})
			home := &keyView{id: "home", focusList: true}
			p := runPanicSim(t, cols, rows, Config{ProjectName: "test", Views: []views.View{home, sv}, HomeViewID: "home"})
			p.loopSafe(func() {
				p.NavigateHome("home", views.ModeHub)
				if sc.project != nil {
					p.SetActiveProject(sc.project)
				}
				sv.Focus(sc.focus)
				p.NavigateTo(sv.ID())
			})
			// The rows are read off the loop: wait until they are drawn.
			require.Eventually(t, func() bool {
				p.loopSafe(func() {})
				be.mu.Lock()
				defer be.mu.Unlock()
				return be.listed > 0
			}, 3*time.Second, 10*time.Millisecond)
			for _, st := range sc.steps {
				p.press(st.key, st.r)
			}
			if sc.want != "" {
				require.Eventually(t, func() bool {
					p.loopSafe(func() {})
					return strings.Contains(p.text(), sc.want)
				}, 3*time.Second, 20*time.Millisecond, "the screen shows %q:\n%s", sc.want, p.text())
			}
			if sc.fine {
				p.sweep(cols, rows, 6, 3)
			} else {
				p.sweep(cols, rows, 12, 5)
			}
		})
	}
}
