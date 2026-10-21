package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// tabSessions is a backend with the tabs of the detail: `next:` steps,
// trace, reports and fan-out.
type tabSessions struct {
	fakeSessions
	tmu       sync.Mutex
	next      []SessionNextStep
	trace     string
	reports   []CheckpointReport
	bodies    map[string]string
	fanout    FanoutView
	relaunch  []string
	cleans    []string
	cleanPlan FanoutCleanView
}

func (s *tabSessions) NextSteps(context.Context, string) ([]SessionNextStep, error) {
	return s.next, nil
}
func (s *tabSessions) SessionTrace(context.Context, string) (string, error) { return s.trace, nil }
func (s *tabSessions) SessionReports(context.Context, string) ([]CheckpointReport, error) {
	return s.reports, nil
}
func (s *tabSessions) CheckpointReport(_ context.Context, _, id string) (string, string, error) {
	return id, s.bodies[id], nil
}
func (s *tabSessions) Fanout(context.Context, string) (FanoutView, error) { return s.fanout, nil }
func (s *tabSessions) RelaunchFanout(_ context.Context, id string) (string, error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	s.relaunch = append(s.relaunch, id)
	return "Pilote relancé", nil
}
func (s *tabSessions) CleanFanout(_ context.Context, id string, discard, dry bool) (FanoutCleanView, error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	entry := id
	if discard {
		entry += " --discard"
	}
	if dry {
		entry += " --dry-run"
	}
	s.cleans = append(s.cleans, entry)
	plan := s.cleanPlan
	if discard {
		plan.Discarded = []string{"oh/fanout/8f2a/oh-14 9c1e2d3"}
		plan.Kept = []string{"oh/fanout/8f2a/oh-13: running"}
	}
	return plan, nil
}

func (s *tabSessions) calls() ([]string, []string) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	return append([]string(nil), s.relaunch...), append([]string(nil), s.cleans...)
}

// fanoutRows is the fan-out of the sketch: a parent, three children (one
// waiting at cp-2) and two other sessions.
func fanoutRows() []SessionRow {
	now := time.Now()
	at := now.Add(-2 * time.Minute)
	return []SessionRow{
		{ID: "ses_8f2a01", Workflow: "implement-spec ⑂", Agent: "orchestrator-dev", Mode: "semi-auto", State: "active", StateIcon: "●", StateLabel: "en cours",
			Cost: 1.84, Started: at, FanoutRole: FanoutRoleParent, FanoutLine: "⑂ Fan-out : 1/3 enfants mergés · en cours",
			Outputs: []SessionOutput{{Name: "method", Value: "tdd"}, {Name: "tickets", Value: "oh-12,oh-13,oh-14"}}, Next: "audit"},
		{ID: "ses_91aa01", Workflow: "↳ ticket (oh-12)", Agent: "developer", State: "completed", StateIcon: "✔", StateLabel: "terminée", Cost: 0.41,
			Started: at, FanoutRole: FanoutRoleChild, FanoutParent: "ses_8f2a01", Finished: true, Changed: now},
		{ID: "ses_91ab01", Workflow: "↳ ticket (oh-13)", Agent: "developer", State: "active", StateIcon: "●", StateLabel: "en cours", Cost: 0.37,
			Started: at, FanoutRole: FanoutRoleChild, FanoutParent: "ses_8f2a01"},
		{ID: "ses_91ac01", Workflow: "↳ ticket (oh-14)", Agent: "developer", State: "waiting", StateIcon: "◐", StateLabel: "en attente", Cost: 0.22,
			Started: at, FanoutRole: FanoutRoleChild, FanoutParent: "ses_8f2a01",
			Decisions: []SessionDecision{{ID: "checkpoint:ses_91ac01:1", SessionID: "ses_91ac01", Kind: DecisionKindCheckpoint, Icon: "⏸", Summary: "cp-2 « Commit »", Created: at}}},
		{ID: "ses_spec01", Workflow: "spec", Agent: "planner", State: "active", StateIcon: "●", StateLabel: "en cours", Cost: 0.93, Started: at},
		{ID: "ses_orphan", Workflow: "↳ ticket (oh-99)", Agent: "developer", State: "active", StateIcon: "●", StateLabel: "en cours", Started: at,
			FanoutRole: FanoutRoleChild, FanoutParent: "ses_gone"},
	}
}

func sampleFanout() FanoutView {
	return FanoutView{Parent: "ses_8f2a01", Experimental: true, Status: "running", Parallel: 2, Driver: true, BudgetUSD: 10, SpentUSD: 1.84,
		Children: []FanoutChildView{
			{Ticket: "oh-12", Session: "ses_91aa01", State: "merged", Branch: "oh/fanout/8f2a/oh-12", CostUSD: 0.41, End: "signal"},
			{Ticket: "oh-13", Session: "ses_91ab01", State: "running", Branch: "oh/fanout/8f2a/oh-13", CostUSD: 0.37},
			{Ticket: "oh-14", Session: "ses_91ac01", State: "running", Branch: "oh/fanout/8f2a/oh-14", CostUSD: 0.22},
			{Ticket: "oh-15", State: "pending"},
		},
		FixRounds: 1, FixMax: 3, LastRefusal: "tests auth", Integration: "oh/fanout/8f2a/integration", Commits: 3}
}

func listKeys(v *SessionsView) []string {
	var out []string
	for _, it := range v.list.GetItems() {
		if k := refKey(it.Reference); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// The children of a parent are grouped under it, as a tree; the group goes
// to « À traiter » when one of them waits, its decision under the child.
// A child whose parent is not listed stays alone.
func TestSessionsFanoutTree(t *testing.T) {
	frLocale(t)
	v := NewSessionsView(SessionsViewConfig{Backend: &tabSessions{fakeSessions: fakeSessions{rows: fanoutRows()}}})
	v.SetShell(&mockShell{})
	onLoop := harness(t, v, fanoutRows())
	onLoop(func() {
		assert.Equal(t, []string{"s:ses_8f2a01", "s:ses_91aa01", "s:ses_91ab01", "s:ses_91ac01", "d:checkpoint:ses_91ac01:1", "s:ses_spec01", "s:ses_orphan"},
			listKeys(v))
		items := v.list.GetItems()
		assert.Contains(t, items[0].MainText, "À traiter (1)")
		var texts []string
		for _, it := range items {
			texts = append(texts, it.MainText)
		}
		all := strings.Join(texts, "\n")
		assert.Contains(t, all, "├")
		assert.Contains(t, all, "└")
		_, d := v.selected()
		if assert.NotNil(t, d, "the cursor lands on the decision of the child") {
			assert.Equal(t, "checkpoint:ses_91ac01:1", d.ID)
		}
	})
}

// The detail in tabs: Détail shows the outputs recorded by oh and the
// `next:` steps apart from the other suggestion; Tab goes to Trace (reader:
// search, n/N), Rapports (list and reader), Fan-out (children, p, d), Esc
// back to Détail and the list.
func TestSessionsDetailTabs(t *testing.T) {
	frLocale(t)
	be := &tabSessions{fakeSessions: fakeSessions{rows: fanoutRows()},
		next: []SessionNextStep{{Workflow: "review", Label: "Revue", Inputs: "branch=feat/auth"}},
		trace: "Trace de ses_8f2a01 (source : en direct)\n" +
			"10:02:00  checkpoint orchestrator-dev cp-plan approved (oh)\n" +
			"10:05:00  tool       ↳              bash  git commit -m \"feat(auth)\"  completed\n" +
			"10:31:00  checkpoint oh             cp-merge refused (oh)\n" +
			"10:32:00  fanout     oh             fix ses_93f0\n" +
			"10:40:00  checkpoint oh             cp-merge approved (oh)  hors valideur",
		reports: []CheckpointReport{
			{ID: "rp-5", Agent: "developer", Title: "oh-12", Bytes: 2100, At: time.Date(2026, 10, 15, 10, 9, 0, 0, time.Local)},
			{ID: "rp-7", Agent: "reviewer", Title: "Revue de l'intégration", Bytes: 4200, At: time.Date(2026, 10, 15, 10, 28, 0, 0, time.Local)},
		},
		bodies:    map[string]string{"rp-7": "## Résumé\nDeux constats bloquants sur le rafraîchissement du jeton.", "rp-5": "## Fait\noh-12"},
		fanout:    sampleFanout(),
		cleanPlan: FanoutCleanView{Worktrees: []string{"/repo/.oh/worktrees/oh-12"}, Branches: []string{"oh/fanout/8f2a/oh-12"}},
	}
	sh := &dialogShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)
	onLoop(func() { v.selectKey("s:ses_8f2a01"); v.onSelectionChanged() })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = strings.Contains(v.detail.GetText(true), "review") })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
	onLoop(func() {
		text := v.detail.GetText(true)
		for _, want := range []string{"Sorties relevées par oh", "tickets  oh-12,oh-13,oh-14", "Suites proposées (next:)", "review — Revue  branch=feat/auth", "Enchaîner avec audit (e)"} {
			assert.Contains(t, text, want)
		}
		golden.Assert(t, "sessions_tab_detail", screenText(t, v.tabs.root, 70, 22))
	})

	// Trace.
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0)) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = len(v.tabs.trace.lines) > 0 })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
	sh.answer("cp-merge")
	onLoop(func() {
		assert.True(t, v.CapturesInput())
		assert.Equal(t, tabTrace, v.tabs.cur)
		assert.Contains(t, v.StatusHints(), "/ rechercher", "the hints line follows the tab")
		assert.True(t, v.PassesShellKeys(), "? and Ctrl+P stay with the shell")
		assert.Equal(t, "?", string(v.HandleKey(key('?')).Rune()), "? is left to the shell")
		v.HandleKey(key('/'))
		assert.Equal(t, []int{3, 5}, v.tabs.trace.hits)
		assert.Equal(t, 0, v.tabs.trace.cur)
		v.HandleKey(key('n'))
		assert.Equal(t, 1, v.tabs.trace.cur)
		v.HandleKey(key('n'))
		assert.Equal(t, 0, v.tabs.trace.cur, "wraps around")
		v.HandleKey(key('N'))
		assert.Equal(t, 1, v.tabs.trace.cur)
		golden.Assert(t, "sessions_tab_trace", screenText(t, v.tabs.root, 90, 12))
	})

	// Rapports: the last one first, read in the reader.
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0)) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = len(v.tabs.reports.reader.lines) > 0 })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
	onLoop(func() {
		it, _ := v.tabs.reports.list.Current()
		assert.Equal(t, "rp-7", it.ID)
		golden.Assert(t, "sessions_tab_reports", screenText(t, v.tabs.root, 100, 10))
		v.tabs.reports.list.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		assert.True(t, v.tabs.reports.reading)
		v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
		assert.False(t, v.tabs.reports.reading, "Esc: back to the list of reports")
		assert.True(t, v.CapturesInput())
	})

	// Fan-out.
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0)) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = v.tabs.fanout.view != nil })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
	onLoop(func() {
		assert.Equal(t, tabFanout, v.tabs.cur)
		golden.Assert(t, "sessions_tab_fanout", screenText(t, v.tabs.root, 100, 16))
		v.HandleKey(key('p'))
		assert.Contains(t, strings.Join(sh.toasts, "\n"), "tourne déjà", "the driver runs: not started twice")
		v.tabs.fanout.view.Driver = false
		v.HandleKey(key('p'))
	})
	require.Eventually(t, func() bool { r, _ := be.calls(); return len(r) == 1 }, 3*time.Second, 10*time.Millisecond)
	relaunch, _ := be.calls()
	assert.Equal(t, []string{"ses_8f2a01"}, relaunch)
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = v.tabs.fanout.view != nil })
		return ok
	}, 3*time.Second, 10*time.Millisecond)

	// d: not done yet, refused; once done, the preview, --discard ticked,
	// then the confirmation.
	onLoop(func() {
		v.tabs.fanout.view.Cleanable = false
		v.HandleKey(key('d'))
		assert.Contains(t, sh.toasts[len(sh.toasts)-1], "pas terminé")
		v.tabs.fanout.view.Cleanable = true
		v.HandleKey(key('d'))
	})
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return strings.Contains(title, "Nettoyer") }, 3*time.Second, 10*time.Millisecond)
	_, body, actions := sh.modal()
	assert.Contains(t, body, "oh/fanout/8f2a/oh-12")
	require.Len(t, actions, 3)
	assert.Contains(t, actions[0].Label, "[ []")
	onLoop(actions[0].Callback) // tick --discard
	require.Eventually(t, func() bool { _, _, a := sh.modal(); return strings.Contains(a[0].Label, "[x[]") }, 3*time.Second, 10*time.Millisecond)
	_, body, actions = sh.modal()
	assert.Contains(t, body, "9c1e2d3", "what --discard deletes is shown")
	assert.Contains(t, body, "running")
	onLoop(actions[1].Callback)
	sh.cmu.Lock()
	confirm := sh.confirm
	sh.cmu.Unlock()
	require.NotNil(t, confirm)
	assert.True(t, sh.danger)
	_, cleans := be.calls()
	assert.Equal(t, []string{"ses_8f2a01 --dry-run", "ses_8f2a01 --discard --dry-run"}, cleans, "nothing removed before the confirmation")
	onLoop(confirm)
	require.Eventually(t, func() bool { _, c := be.calls(); return len(c) == 3 }, 3*time.Second, 10*time.Millisecond)
	_, cleans = be.calls()
	assert.Equal(t, "ses_8f2a01 --discard", cleans[2])
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(func() { ok = v.tabs.fanout.view != nil })
		return ok
	}, 3*time.Second, 10*time.Millisecond)

	// Enter on a child: back to the list, on the child.
	onLoop(func() {
		v.tabs.fanout.list.Select("oh-13")
		v.tabs.fanout.list.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		assert.False(t, v.CapturesInput())
		if r, _ := v.selected(); assert.NotNil(t, r) {
			assert.Equal(t, "ses_91ab01", r.ID)
		}
	})

	// Esc from a tab: back to Détail.
	onLoop(func() {
		v.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
		assert.True(t, v.CapturesInput())
		v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
		assert.Equal(t, tabDetail, v.tabs.cur)
		assert.False(t, v.CapturesInput())
		assert.Equal(t, i18n.T("tui.sessions.hints"), v.StatusHints())
	})
}

// A session out of a fan-out has no Fan-out tab.
func TestSessionsTabsWithoutFanout(t *testing.T) {
	assert.Len(t, availableTabs(&SessionRow{ID: "s"}), 3)
	assert.Len(t, availableTabs(&SessionRow{ID: "s", FanoutRole: FanoutRoleChild}), 4)
	assert.Len(t, availableTabs(nil), 3)
}

func TestTextReaderSearch(t *testing.T) {
	r := newTextReader()
	r.SetText("alpha\nBeta [red]tag[-]\nalphabet")
	r.search("ALPHA")
	assert.Equal(t, []int{0, 2}, r.hits)
	assert.Contains(t, r.text.GetText(false), "[red[]", "text from agents is escaped")
	r.search("zzz")
	assert.Empty(t, r.hits)
	assert.Equal(t, -1, r.cur)
	r.move(1) // nothing to move to
	assert.Equal(t, -1, r.cur)
	assert.Equal(t, "a"+"[#f9e2af]b[-]"+"c", markQuery("abc", "B"))
}
