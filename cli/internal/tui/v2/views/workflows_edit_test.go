package views

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// formShell records the modals the catalogue opens.
type formShell struct {
	recordingShell
	form   *InlineFormConfig
	modal  []ModalAction
	title  string
	body   string
	input  func(string)
	pushed View
}

func (s *formShell) ShowInlineForm(c InlineFormConfig) { s.form = &c }
func (s *formShell) ShowScrollableModal(title, body string, a []ModalAction) {
	s.title, s.body, s.modal = title, body, a
}
func (s *formShell) ShowInputModal(_ string, _ string, fn func(string)) { s.input = fn }
func (s *formShell) PushView(v View)                                    { s.pushed = v }

func key(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func catalogData() CatalogData {
	return CatalogData{Team: "core", Project: "web", Member: "alice", Editable: true, Entries: []CatalogEntry{
		{ID: "ticket", Ref: "hub:ticket", Layer: "hub", ReadOnly: true, Valid: true, Risk: "write"},
		{ID: "hotfix", Ref: "team:hotfix", Layer: "team", Version: 2, Valid: true, HasDraft: true, Queued: true},
		{ID: "release", Ref: "team:release", Layer: "team", Draft: true, Errors: 2, Findings: []string{"✗ risk: valeur inconnue"}},
		{ID: "hotfix", Ref: "team:hotfix", Layer: "team", Draft: true, Valid: true, NewBricks: []string{"agent:hotfixer"}},
	}, Integrity: []CatalogIntegrity{{Message: "fichier modifié à la main", Source: "/ts/workflows/published/x.yaml"}}}
}

// selectRef moves the cursor to the entry key (on the loop).
func selectRef(t *testing.T, v *WorkflowCatalogView, key string) {
	t.Helper()
	for i, it := range v.list.GetItems() {
		if r, ok := it.Reference.(itemRef); ok && r.key == key {
			v.list.SelectIndex(i)
			return
		}
	}
	t.Fatalf("no item %s", key)
}

func TestWorkflowCatalogEditing(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]string{}
	rec := func(name string) func(CatalogEntry) {
		return func(e CatalogEntry) { mu.Lock(); calls[name] = e.Ref; mu.Unlock() }
	}
	var created CatalogNew
	var archived string
	v := NewWorkflowCatalogView(WorkflowCatalogConfig{
		Launch: func(id string) { calls["launch"] = id },
		New:    func(n CatalogNew) { created = n },
		Edit:   rec("edit"), TestDraft: rec("test"), Publish: rec("publish"), Diff: rec("diff"), History: rec("history"),
		Archive: func(e CatalogEntry, msg string) { archived = e.Ref + ":" + msg },
		Discard: rec("discard"),
	})
	sh := &formShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() {
		v.Mount(content, app)
		v.SetData(catalogData())
	})

	var texts []string
	onLoop(app, func() {
		for _, it := range v.list.GetItems() {
			texts = append(texts, it.MainText+" "+it.SecondaryText)
		}
	})
	all := strings.Join(texts, "\n")
	for _, want := range []string{i18n.Tf("tui.catalog.edit.drafts", "alice"), "✎ release", i18n.Tf("tui.catalog.edit.errors", 2), "⏳",
		i18n.T("tui.catalog.edit.new_brick"), i18n.Tf("tui.catalog.edit.integrity", 1), "fichier modifié à la main"} {
		assert.Contains(t, all, want)
	}

	// Hub workflow: e = extend (form preset), h refused, Enter launches.
	onLoop(app, func() {
		selectRef(t, v, "hub:ticket|ticket")
		v.HandleKey(key('e'))
	})
	require.NotNil(t, sh.form)
	assert.Equal(t, CatalogNewExtends, sh.form.Fields[0].Default)
	assert.Equal(t, "hub:ticket", sh.form.Fields[1].Default)
	assert.Equal(t, "ticket", sh.form.Fields[3].Default)
	sh.form.OnSubmit(map[string]string{"kind": CatalogNewExtends, "source": "hub:ticket", "layer": "team", "id": "ticket-hotfix"}, nil)
	assert.Equal(t, CatalogNew{Kind: CatalogNewExtends, Source: "hub:ticket", ID: "ticket-hotfix", Layer: "team"}, created)
	onLoop(app, func() { v.HandleKey(key('h')) })
	assert.Empty(t, calls["history"])

	// Published with a draft: t, p, D, h act on it; x asks a message.
	onLoop(app, func() {
		selectRef(t, v, "team:hotfix|hotfix")
		for _, r := range "tpDhe" {
			v.HandleKey(key(r))
		}
		v.HandleKey(key('x'))
	})
	for _, name := range []string{"test", "publish", "diff", "history", "edit"} {
		assert.Equal(t, "team:hotfix", calls[name], name)
	}
	require.NotNil(t, sh.input)
	sh.input(" obsolète ")
	assert.Equal(t, "team:hotfix:obsolète", archived)

	// Draft: x discards after confirmation.
	onLoop(app, func() {
		selectRef(t, v, "draft:team:release")
		v.HandleKey(key('x'))
	})
	require.NotEmpty(t, sh.modal)
	sh.modal[0].Callback()
	assert.Equal(t, "team:release", calls["discard"])

	// Without a team-state, n is refused with a message.
	onLoop(app, func() {
		d := catalogData()
		d.Editable = false
		v.SetData(d)
		sh.form = nil
		v.HandleKey(key('n'))
	})
	assert.Nil(t, sh.form)
	assert.NotEmpty(t, sh.toasts)
}

func TestPublishViewPublishesOnce(t *testing.T) {
	var n atomic.Int32
	release := make(chan struct{})
	var got string
	v := NewPublishView(PublishViewConfig{
		Ref: "team:hotfix",
		Load: func(context.Context) (*PublishPreview, error) {
			return &PublishPreview{Ref: "team:hotfix", Current: 2, Next: 3, Valid: true,
				Impact: []PublishImpact{{Widen: true, Text: "distant autorisé"}}, NewBricks: []string{"agent:hotfixer"},
				Diff: "--- a\n+++ b\n-risk: read\n+risk: write\n", Governance: "Publication : tout membre"}, nil
		},
		Publish: func(_ context.Context, msg string) (*PublishResult, error) {
			n.Add(1)
			got = msg
			<-release
			return &PublishResult{Version: 3}, nil
		},
	})
	sh := &recordingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var text string
		onLoop(app, func() { text = v.body.GetText(true) })
		return strings.Contains(text, "v2 → v3")
	}, 2*time.Second, 20*time.Millisecond)
	var text string
	onLoop(app, func() { text = v.body.GetText(true) })
	for _, want := range []string{"distant autorisé", "agent:hotfixer", "+risk: write", "tout membre"} {
		assert.Contains(t, text, want)
	}

	// The message is required.
	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)) })
	assert.Zero(t, n.Load())
	onLoop(app, func() {
		v.input.SetText("Autorise le distant")
		v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
		v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
	})
	close(release)
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		return sh.popped == 1
	}, 2*time.Second, 20*time.Millisecond)
	assert.Equal(t, int32(1), n.Load(), "one publication for two Ctrl+S")
	assert.Equal(t, "Autorise le distant", got)
}

func TestPublishViewInvalidAndQueued(t *testing.T) {
	v := NewPublishView(PublishViewConfig{Ref: "team:x", Publish: func(context.Context, string) (*PublishResult, error) {
		return &PublishResult{Queued: true}, nil
	}})
	sh := &recordingShell{}
	v.SetShell(sh)
	v.message = "m"
	v.SetPreview(&PublishPreview{Ref: "team:x", Next: 1, New: true, Findings: []string{"✗ risk: inconnu"}}, nil)
	v.publish()
	assert.False(t, v.publishing, "an invalid draft is not published")
	v.SetPreview(&PublishPreview{Ref: "team:x", Next: 1, New: true, Valid: true, Queued: true}, nil)
	assert.Contains(t, PublishText(v.preview, nil), i18n.T("tui.publish.already_queued"))
	v.publish()
	require.NotEmpty(t, sh.toasts)
	assert.Equal(t, i18n.Tf("tui.publish.queued", "team:x"), sh.toasts[len(sh.toasts)-1])
	assert.Contains(t, PublishText(nil, errors.New("boom")), "boom")
}

func TestHistoryViewRestore(t *testing.T) {
	restored := make(chan int, 2)
	v := NewHistoryView(HistoryViewConfig{
		Ref: "team:hotfix",
		Load: func(context.Context) ([]HistoryVersion, error) {
			return []HistoryVersion{{Version: 2, By: "ben", Message: "v2", Current: true}, {Version: 1, By: "alice", Message: "v1"}}, nil
		},
		Diff: func(context.Context, int) (string, error) { return "-a\n+b\n", nil },
		Restore: func(_ context.Context, n int) (*PublishResult, error) {
			restored <- n
			return &PublishResult{Version: 3}, nil
		},
	})
	sh := &formShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { n = len(v.versions) })
		return n == 2
	}, 2*time.Second, 20*time.Millisecond)

	// The current version cannot be restored.
	onLoop(app, func() { v.list.SelectIndex(0); v.HandleKey(key('r')) })
	assert.Nil(t, sh.modal)
	onLoop(app, func() { v.list.SelectIndex(1); v.HandleKey(key('r')) })
	require.NotEmpty(t, sh.modal)
	sh.modal[0].Callback()
	select {
	case n := <-restored:
		assert.Equal(t, 1, n)
	case <-time.After(2 * time.Second):
		t.Fatal("not restored")
	}
}
