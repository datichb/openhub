package views

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func gcTestPlan() gc.Report {
	return gc.Report{Items: []gc.Item{
		{Kind: gc.KindBundles, ID: "3f2a1c9e", Path: "/home/me/.oh/bundles/3f2a1c9e", Bytes: 2 << 20, Reason: "unused"},
		{Kind: gc.KindBundles, ID: "77b0d1aa", Path: "/home/me/.oh/bundles/77b0d1aa", Bytes: 1 << 20, Reason: "unused"},
		{Kind: gc.KindSessions, ID: "ses_41c0", Path: "/home/me/.oh/sessions/ses_41c0", Bytes: 5 << 20, Reason: "stopped", KeepResults: true},
		{Kind: gc.KindWorktrees, ID: "oh/ticket/bd-12", Path: "/work/demo-wt/bd-12", Repo: "/work/demo", Bytes: 12 << 20, Reason: "clean"},
		{Kind: gc.KindFanout, ID: "ses_9e1f", Path: "/work/demo-wt/integration", Bytes: 3 << 20, Reason: "2 worktrees, 3 branches"},
		{Kind: gc.KindOrphans, ID: "4242", PID: 4242, Path: "/tmp/oh-test/server.log", Reason: "test"},
	}, History: gc.History{Sessions: 4, AgentEvents: 120, Decisions: 6}}
}

func gcBytes(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/(1<<20)) }

// The oh gc view: the elements by category, sizes, the preview line.
func TestGCViewGolden(t *testing.T) {
	frLocale(t)
	v := NewGCView(GCViewConfig{Bytes: gcBytes, Retention: i18n.Tf("tui.gc.retention", 30, 90, 365)})
	v.build()
	v.showPlan(gcTestPlan(), nil)
	assert.Equal(t, "gc", v.ID())
	assertGolden(t, "gc_view", screenText(t, v.root, 100, 32))
}

type gcRecorder struct {
	mu   sync.Mutex
	runs []gc.Report
}

func (r *gcRecorder) all() []gc.Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]gc.Report(nil), r.runs...)
}

func mountGC(t *testing.T) (*GCView, *deleteShell, *gcRecorder, *tview.Application) {
	t.Helper()
	rec := &gcRecorder{}
	v := NewGCView(GCViewConfig{Bytes: gcBytes,
		Plan: func(context.Context) (gc.Report, error) { return gcTestPlan(), nil },
		Run: func(_ context.Context, only gc.Report) (gc.Report, error) {
			rec.mu.Lock()
			rec.runs = append(rec.runs, only)
			rec.mu.Unlock()
			return gc.Report{Items: only.Items, History: only.History, Applied: true}, nil
		}})
	sh := &deleteShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { n = v.list.Len() })
		return n == 7
	}, 2*time.Second, 10*time.Millisecond, "six elements and the history")
	require.NoError(t, v.list.cfg.Validate())
	return v, sh, rec, app
}

// d removes the selected element after a confirmation (danger), nothing
// before; the plan is computed again after.
func TestGCViewDeleteOne(t *testing.T) {
	v, sh, rec, app := mountGC(t)
	var first ActionItem
	onLoop(app, func() { first, _ = v.list.Current() })
	onLoop(app, func() { v.list.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)) })
	c, ok := sh.lastConfirm()
	require.True(t, ok)
	assert.True(t, c.danger)
	assert.Empty(t, rec.all(), "not before the confirmation")
	onLoop(app, c.onYes)
	require.Eventually(t, func() bool { return len(rec.all()) == 1 }, 2*time.Second, 10*time.Millisecond)
	only := rec.all()[0]
	require.Len(t, only.Items, 1)
	assert.Equal(t, first.Ref, only.Items[0])
	assert.Zero(t, only.History.Total())
}

// A cleans up everything after one confirmation listing the categories.
func TestGCViewCleanAll(t *testing.T) {
	v, sh, rec, app := mountGC(t)
	onLoop(app, func() { v.list.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone)) })
	c, ok := sh.lastConfirm()
	require.True(t, ok)
	assert.True(t, c.danger)
	assert.Contains(t, c.detail, kindLabel(gc.KindWorktrees))
	assert.Contains(t, c.detail, i18n.Tf("tui.gc.history_rows", 130))
	onLoop(app, c.onYes)
	require.Eventually(t, func() bool { return len(rec.all()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Len(t, rec.all()[0].Items, 6)
	assert.Equal(t, 130, rec.all()[0].History.Total())
	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.header.GetText(true) })
		return strings.Contains(txt, "✔ "+i18n.Tf("tui.gc.done", 6, gcBytes(23<<20), 130))
	}, 2*time.Second, 10*time.Millisecond, "result shown")
}

// deployRecorder is the section of the former deployments of a test.
type deployRecorder struct {
	mu      sync.Mutex
	applied int
	left    int
}

func (r *deployRecorder) config() *DeployCleanupConfig {
	return &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.left == 0 {
				return DeployCleanupPlan{}, nil
			}
			return DeployCleanupPlan{Projects: r.left, Text: "demo  /work/demo\n  − .opencode/agents/ (12 fichiers)\n",
				DiffTitle: "opencode.json : diff du nettoyage", Diff: "- \"agent\": {…}\n"}, nil
		},
		Apply: func(context.Context) (string, bool) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.applied++
			r.left = 0
			return i18n.Tf("tui.cleanup.done", 1), true
		},
	}
}

func (r *deployRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.applied
}

// The clean-up view in two sections: « Anciens déploiements » (oh migrate
// deploy-cleanup: c diff, d cleans after a confirmation) and « Données
// d'oh » (oh gc); Tab switches; the hints follow the section.
func TestCleanupViewSections(t *testing.T) {
	frLocale(t)
	rec := &deployRecorder{left: 1}
	v := NewGCView(GCViewConfig{Bytes: gcBytes,
		Plan:    func(context.Context) (gc.Report, error) { return gcTestPlan(), nil },
		Deploy:  rec.config(),
		Section: CleanupDeploy})
	sh := &deleteShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.deploy.text.GetText(true) })
		return strings.Contains(txt, ".opencode/agents/")
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		assert.Equal(t, "Nettoyage", v.Title())
		assert.Equal(t, CleanupDeploy, v.section)
		assert.Contains(t, v.StatusHints(), "d nettoyer · c voir le diff")
		assertGolden(t, "cleanup_view_deploy", screenText(t, v.root, 80, 8))
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	})
	sh.cmu.Lock()
	assert.Contains(t, sh.body, "agent")
	sh.cmu.Unlock()
	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)) })
	c, ok := sh.lastConfirm()
	require.True(t, ok)
	assert.True(t, c.danger)
	assert.Zero(t, rec.count(), "not before the confirmation")
	onLoop(app, c.onYes)
	require.Eventually(t, func() bool { return rec.count() == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.deploy.text.GetText(true) })
		return strings.Contains(txt, i18n.T("cmd.migrate.cleanup.none"))
	}, 2*time.Second, 10*time.Millisecond, "scanned again")

	onLoop(app, func() {
		v.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		assert.Equal(t, CleanupData, v.section)
		assert.Contains(t, v.StatusHints(), "A tout nettoyer")
	})
	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { n = v.list.Len() })
		return n == 7
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		assertGolden(t, "cleanup_view_data", screenText(t, v.root, 100, 24))
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)) // d of the data: the list
	})
}
