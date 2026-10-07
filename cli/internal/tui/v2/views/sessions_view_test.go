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
)

type fakeSessions struct {
	mu      sync.Mutex
	rows    []SessionRow
	decided []string
	follow  chan SessionFeedLine
}

func (f *fakeSessions) List(context.Context, string, bool) ([]SessionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rows, nil
}
func (f *fakeSessions) Changes(ctx context.Context) <-chan struct{} { return make(chan struct{}) }
func (f *fakeSessions) Follow(context.Context, string) (<-chan SessionFeedLine, error) {
	return f.follow, nil
}
func (f *fakeSessions) Decide(_ context.Context, id, choice, message string, _ map[string]string) error {
	f.mu.Lock()
	entry := id + "=" + choice
	if message != "" {
		entry += ":" + message
	}
	f.decided = append(f.decided, entry)
	f.mu.Unlock()
	return nil
}
func (f *fakeSessions) Send(context.Context, string, string) error        { return nil }
func (f *fakeSessions) Interrupt(context.Context, string) error           { return nil }
func (f *fakeSessions) SwitchModel(context.Context, string, string) error { return nil }
func (f *fakeSessions) Stop(context.Context, string) error                { return nil }
func (f *fakeSessions) Resume(context.Context, string) error              { return nil }
func (f *fakeSessions) Attach(string, string)                             {}
func (f *fakeSessions) OpenBrowser(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeSessions) MRDescription(context.Context, string) (string, error) { return "", nil }

func (f *fakeSessions) decisions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.decided...)
}

func sampleRows() []SessionRow {
	now := time.Now()
	return []SessionRow{
		{ID: "ses_a", Project: "openhub", Workflow: "ticket", Agent: "developer", State: "waiting", StateIcon: "⏸", Started: now,
			Decisions: []SessionDecision{{ID: "permission:ses_a:per_1", SessionID: "ses_a", Kind: DecisionKindPermission, Icon: "!", Summary: "shell npm test", Created: now}}},
		{ID: "ses_b", Project: "api", Agent: "reviewer", State: "active", StateIcon: "●", Started: now},
		{ID: "ses_c", Project: "api", Agent: "reviewer", State: "sleeping", StateIcon: "◌"},
		{ID: "ses_d", Project: "api", Agent: "auditor", State: "stopped", Finished: true, Changed: now.Add(-time.Hour)},
		{ID: "ses_old", Project: "api", Agent: "auditor", State: "stopped", Finished: true, Changed: now.Add(-30 * 24 * time.Hour)},
	}
}

func TestSessionsSections(t *testing.T) {
	d, running, sleeping, finished := sessionsSections(sampleRows(), time.Now())
	assert.Len(t, d, 1)
	assert.Len(t, running, 2)
	assert.Len(t, sleeping, 1)
	require.Len(t, finished, 1, "finished sessions older than 7 days are hidden")
	assert.Equal(t, "ses_d", finished[0].ID)
}

func TestSessionsView_MountRenderKeys(t *testing.T) {
	be := &fakeSessions{rows: sampleRows(), follow: make(chan SessionFeedLine, 4)}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := tview.NewApplication()
	v.Mount(content, app)
	assert.Equal(t, "sessions", v.ID())

	// Render synchronously (the async load needs a running app).
	v.rows = sampleRows()
	v.render()
	var headers []string
	for _, it := range v.list.GetItems() {
		if it.IsHeader {
			headers = append(headers, it.MainText)
		}
	}
	assert.Len(t, headers, 4, "to handle, running, sleeping, finished")

	// The first selectable item is the decision: y approves it once.
	r, d := v.selected()
	require.NotNil(t, d)
	assert.Equal(t, "ses_a", r.ID)
	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0)))
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "permission:ses_a:per_1=once", be.decisions()[0])

	// Focus a session, toggle the live feed.
	v.Focus("ses_b")
	v.render()
	r, d = v.selected()
	require.NotNil(t, r)
	assert.Nil(t, d)
	assert.Equal(t, "ses_b", r.ID)
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 't', 0))
	assert.Equal(t, 2, v.top.GetItemCount(), "feed panel shown")
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 't', 0))
	assert.Equal(t, 1, v.top.GetItemCount())

	assert.NotNil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'j', 0)), "navigation keys go to the list")
	v.Unmount()
	assert.Nil(t, v.list)
}

func TestQuestionForm(t *testing.T) {
	fields := []SessionDecisionField{
		{Key: "q0", Title: "Colour", Type: "string", Options: []SelectOption{{Label: "Blue", Value: "Blue"}, {Label: "Red", Value: "Red"}}, Custom: true},
		{Key: "ok", Type: "boolean"},
		{Key: "tags", Type: "multiselect", Options: []SelectOption{{Label: "a", Value: "a"}}},
		{Key: "n", Type: "integer"},
		{Key: "ext", Type: "external"},
	}
	form, answers := questionForm(fields)
	require.Len(t, form, 6, "choice + free answer, bool, multi, text, external note")
	assert.Equal(t, FieldSelect, form[0].Type)
	assert.Equal(t, FieldText, form[1].Type)
	assert.Equal(t, FieldBool, form[2].Type)
	assert.Equal(t, FieldMultiSelect, form[3].Type)

	got := answers(map[string]string{"q0": "Blue", "ok": "true", "n": "3"}, map[string][]string{"tags": {"a"}})
	assert.Equal(t, map[string]string{"q0": "Blue", "ok": "true", "tags": "a", "n": "3"}, got)
	got = answers(map[string]string{"q0": "Blue", "q0" + customSuffix: "Green", "ok": "false"}, nil)
	assert.Equal(t, "Green", got["q0"], "a free answer wins over the option")
}

func TestSessionsSectionAndBadge(t *testing.T) {
	assert.Equal(t, "", SessionsBadge(0, 0))
	assert.Equal(t, "● 2  ⏸ 1", SessionsBadge(2, 1))
	assert.Equal(t, "● 3", SessionsBadge(3, 0))

	_, _, ok := sessionsSection(SessionsSectionConfig{}, SessionsScope{}, "tui.sessions.home_section")
	assert.False(t, ok)
	var opened []string
	cfg := SessionsSectionConfig{
		Summary: func(SessionsScope) SessionsSummary {
			return SessionsSummary{Running: 2, Decisions: 1, Lines: []SessionLine{{ID: "ses_a", Icon: "⏸", Label: "ticket"}}}
		},
		Open: func(id string) { opened = append(opened, id) },
	}
	header, items, ok := sessionsSection(cfg, SessionsScope{}, "tui.sessions.home_section")
	require.True(t, ok)
	assert.Contains(t, header, "● 2  ⏸ 1")
	require.Len(t, items, 2)
	items[0].Action()
	items[1].Action()
	assert.Equal(t, []string{"ses_a", ""}, opened)

	// The landings show the section.
	home := NewHomeView(HomeViewConfig{Sessions: cfg})
	found := false
	for _, it := range home.buildStaticItems() {
		if it.Label == "ticket" {
			found = true
		}
	}
	assert.True(t, found)
}

// With a running application: asynchronous load, live feed, reload on change.
func TestSessionsView_RunningApp(t *testing.T) {
	be := &fakeSessions{rows: sampleRows(), follow: make(chan SessionFeedLine, 4)}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(&mockShell{})
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	sim.SetSize(140, 40)
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
	onLoop(func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n := 0
		onLoop(func() { n = len(v.rows) })
		return n == len(sampleRows())
	}, 3*time.Second, 20*time.Millisecond)

	onLoop(func() {
		v.Focus("ses_b")
		v.render()
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 't', 0))
	})
	be.follow <- SessionFeedLine{Time: time.Now(), Text: "developer › shell go test ✔"}
	be.follow <- SessionFeedLine{Time: time.Now(), Cost: 0.42}
	require.Eventually(t, func() bool {
		txt, title := "", ""
		onLoop(func() { txt, title = v.feed.GetText(true), v.feed.GetTitle() })
		return assert.ObjectsAreEqual(true, len(txt) > 0 && strings.Contains(txt, "go test") && strings.Contains(title, "0.42"))
	}, 3*time.Second, 20*time.Millisecond)
	onLoop(func() { v.Unmount() })
}

func TestLandingsShowSessions(t *testing.T) {
	var scopes []SessionsScope
	cfg := SessionsSectionConfig{Summary: func(s SessionsScope) SessionsSummary {
		scopes = append(scopes, s)
		return SessionsSummary{Running: 1, Lines: []SessionLine{{ID: "ses_a", Icon: "●", Label: "review"}}}
	}}
	pm := NewProjectModeView(ProjectModeConfig{Sessions: cfg})
	pm.project = &ActiveProject{ID: "p1", Name: "openhub"}
	found := false
	for _, it := range pm.buildItems() {
		if it.Label == "review" {
			found = true
		}
		assert.NotEqual(t, "parallel", it.Label)
	}
	assert.True(t, found)

	tm := NewTeamModeView(TeamModeConfig{Sessions: cfg})
	tm.team = &ActiveTeam{ID: "core", Name: "Core"}
	found = false
	for _, it := range tm.buildItems() {
		if it.Label == "review" {
			found = true
		}
	}
	assert.True(t, found)
	assert.Equal(t, []SessionsScope{{ProjectID: "p1"}, {TeamID: "core"}}, scopes)
}

// A budget decision is raised from the view ($ or the card button), as
// `oh budget raise` (v5 finalisation, Q3-5).
func TestSessionsViewRaiseBudget(t *testing.T) {
	now := time.Now()
	rows := []SessionRow{{ID: "ses_a", Agent: "developer", State: "waiting", Started: now,
		Decisions: []SessionDecision{{ID: "budget:ses_a:1", SessionID: "ses_a", Kind: DecisionKindBudget, Icon: "$", Summary: "budget dépassé", Created: now}}}}
	be := &fakeSessions{rows: rows}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	sh := &formShell{}
	v.SetShell(sh)
	v.Mount(tview.NewFlex(), tview.NewApplication())
	v.rows = rows
	v.render()

	_, d := v.selected()
	require.NotNil(t, d)
	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, '$', 0)))
	require.NotNil(t, sh.input, "amount asked")
	sh.input(" 5 ")
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "budget:ses_a:1=raise:5", be.decisions()[0])

	sh.input = nil
	v.openDecision(v.row("ses_a"), d)
	require.NotEmpty(t, sh.modal)
	assert.Equal(t, i18n.T("tui.inbox.raise"), sh.modal[0].Label)
	sh.modal[0].Callback()
	require.NotNil(t, sh.input)
	sh.input("")
	require.Eventually(t, func() bool { return len(be.decisions()) == 2 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "budget:ses_a:1=raise", be.decisions()[1], "empty amount: default raise")
}
