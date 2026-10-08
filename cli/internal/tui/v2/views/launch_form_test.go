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
	"github.com/datichb/openhub/cli/internal/workflow"
)

const launchTestSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket
description: { fr: Implémenter un ticket, en: Implement a ticket }
risk: write
inputs:
  ticket: { type: beads-id, required: true, picker: { multi: true } }
  branch: { type: branch, default: "feat/{{ .ticket }}" }
  publish: { type: bool, default: false }
  kind: { type: enum, values: [a, b], default: b }
  request: { type: text }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
runtime: { default: local, allowed: [local, container] }
`

func launchSpec(t *testing.T) *workflow.Spec {
	t.Helper()
	doc, diags := workflow.Parse([]byte(launchTestSpec), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	return doc.Spec
}

func launchCfg(t *testing.T) LaunchFormConfig {
	return LaunchFormConfig{WorkflowID: "ticket", Spec: launchSpec(t), Lang: "fr",
		Runtimes:  []LaunchRuntime{{Kind: "local", Label: "local", Available: true}, {Kind: "container", Label: "conteneur", Reason: "colima arrêté"}},
		Locations: []SelectOption{{Label: "base", Value: "base"}, {Label: "+ nouveau", Value: "new"}},
		Attach:    []SelectOption{{Label: "auto", Value: "auto"}, {Label: "aucune", Value: "none"}}, DefaultAttach: "auto"}
}

func TestLaunchModelDefaultsAndValidation(t *testing.T) {
	m := newLaunchModel(launchCfg(t))
	assert.Equal(t, "ticket", m.ticketInput)
	assert.True(t, m.perSession)
	assert.Equal(t, "semi-auto", m.mode)
	assert.Equal(t, "local", m.runtime)
	assert.Equal(t, "base", m.location)
	assert.Equal(t, map[string]string{"kind": "b"}, m.values, "templated and false defaults are left to the resolution")
	assert.Equal(t, []string{i18n.Tf("tui.launch.required", "ticket *")}, m.validate())

	m.tickets = []string{"bd-1", "bd-2"}
	assert.Empty(t, m.validate())
	assert.NotEmpty(t, m.sessionsLine())
	m.runtime = "container"
	assert.Len(t, m.validate(), 1, "unavailable runtime")

	c := m.choices()
	assert.Equal(t, []string{"bd-1", "bd-2"}, c.Tickets)
	assert.Equal(t, map[string]string{"kind": "b"}, c.Inputs)
}

func TestLaunchModelPrefill(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Prefill = map[string]string{"ticket": "bd-7, bd-8", "branch": "feat/x"}
	m := newLaunchModel(cfg)
	assert.Equal(t, []string{"bd-7", "bd-8"}, m.tickets)
	assert.Equal(t, "feat/x", m.values["branch"])
	cfg.Tickets = []string{"bd-9"}
	assert.Equal(t, []string{"bd-9"}, newLaunchModel(cfg).tickets, "explicit tickets win")
}

type recordingShell struct {
	mockShell
	mu      sync.Mutex
	toasts  []string
	popped  int
	options []SelectOption
}

func (s *recordingShell) ShowToastMsg(msg string, _ bool) {
	s.mu.Lock()
	s.toasts = append(s.toasts, msg)
	s.mu.Unlock()
}
func (s *recordingShell) PopView() bool {
	s.mu.Lock()
	s.popped++
	s.mu.Unlock()
	return true
}
func (s *recordingShell) ShowSelectModal(_ string, opts []SelectOption, _ string, _ func(string)) {
	s.options = opts
}

// runApp runs a tview application on a simulation screen.
func runApp(t *testing.T, root tview.Primitive) *tview.Application {
	t.Helper()
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(120, 40)
	app := tview.NewApplication().SetScreen(sc)
	app.SetRoot(root, true)
	errc := make(chan error, 1)
	go func() { errc <- app.Run() }()
	t.Cleanup(func() { app.Stop(); <-errc })
	return app
}

func onLoop(app *tview.Application, fn func()) {
	done := make(chan struct{})
	app.QueueUpdate(func() { fn(); close(done) })
	<-done
}

func TestLaunchFormFlow(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	var launched []LaunchChoices
	var mu sync.Mutex
	cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) {
		return &LaunchRecap{Rows: []InfoField{{Label: "Agents", Value: "orchestrator-dev"}}, Warnings: []string{"dirty"}}, nil
	}
	cfg.Launch = func(_ context.Context, c LaunchChoices) error {
		mu.Lock()
		launched = append(launched, c)
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return nil
	}
	v := NewLaunchFormView(cfg)
	sh := &recordingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	assert.Equal(t, "launch.ticket", v.ID())
	assert.True(t, v.CapturesInput())

	onLoop(app, func() { v.next() })
	assert.Equal(t, launchStepOptions, v.step)
	onLoop(app, func() { v.next() })
	assert.Equal(t, launchStepRecap, v.step)
	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.recapTV.GetText(true) })
		return strings.Contains(txt, "orchestrator-dev") && strings.Contains(txt, "dirty")
	}, 2*time.Second, 10*time.Millisecond)

	// Ctrl+S twice: a single launch (m12).
	onLoop(app, func() {
		v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
		v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
	})
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		return sh.popped == 1
	}, 2*time.Second, 10*time.Millisecond, "closed after the launch")
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, launched, 1)
	assert.Equal(t, []string{"bd-1"}, launched[0].Tickets)
	assert.Equal(t, "semi-auto", launched[0].Mode)
}

// A35/A18: a directory with uncommitted changes offers [w] worktree (the
// default) · [s] stash · [b] launch anyway on the recap; the choice is
// recomputed and launched.
func TestLaunchFormDirtyChoices(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	var mu sync.Mutex
	var recaps, launched []LaunchChoices
	cfg.Recap = func(_ context.Context, c LaunchChoices) (*LaunchRecap, error) {
		mu.Lock()
		recaps = append(recaps, c)
		mu.Unlock()
		return &LaunchRecap{Rows: []InfoField{{Label: "Checkpoints", Value: "cp-2 ⏸ pause (obligatoire)"}}, Dirty: true}, nil
	}
	cfg.Launch = func(_ context.Context, c LaunchChoices) error {
		mu.Lock()
		launched = append(launched, c)
		mu.Unlock()
		return nil
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app); v.next(); v.next() })
	recapText := func() string {
		var txt string
		onLoop(app, func() { txt = v.recapTV.GetText(true) })
		return txt
	}
	require.Eventually(t, func() bool {
		return strings.Contains(recapText(), "[w] (●) "+i18n.T("tui.launch.dirty.worktree"))
	}, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, recapText(), "cp-2 ⏸ pause (obligatoire)")
	assert.Contains(t, recapText(), "[s] ( ) "+i18n.T("tui.launch.dirty.stash"))

	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)) })
	require.Eventually(t, func() bool {
		return strings.Contains(recapText(), "[s] (●) "+i18n.T("tui.launch.dirty.stash"))
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)) })
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(launched) == 1
	}, 2*time.Second, 10*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, LaunchDirtyWorktree, recaps[0].Dirty)
	assert.Equal(t, LaunchDirtyStash, recaps[len(recaps)-1].Dirty, "recap recomputed with the choice")
	assert.Equal(t, LaunchDirtyStash, launched[0].Dirty)
}

func TestLaunchFormBlocksMissingInputs(t *testing.T) {
	v := NewLaunchFormView(launchCfg(t))
	sh := &recordingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app); v.next() })
	assert.Equal(t, launchStepInputs, v.step, "required ticket missing")
	sh.mu.Lock()
	assert.NotEmpty(t, sh.toasts)
	sh.mu.Unlock()
	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)) })
	assert.Equal(t, 1, sh.popped, "Esc closes the form")
}

func TestLaunchFormOpensAtOptions(t *testing.T) {
	cfg := launchCfg(t)
	cfg.AtOptions, cfg.Tickets = true, []string{"bd-4"}
	v := NewLaunchFormView(cfg)
	assert.Equal(t, launchStepOptions, v.step)
}

func TestStartSection(t *testing.T) {
	var launched, pinned string
	cfg := StartSectionConfig{
		Entries: func(StartScope) StartEntries {
			return StartEntries{Loaded: true, Total: 3,
				Pinned:     []StartEntry{{ID: "ticket", Label: "Implémenter", Origin: "hub", Pinned: true}},
				Recents:    []StartEntry{{ID: "review", Ago: "il y a 2 h"}},
				Categories: []StartCategory{{ID: "develop", Label: "Développer", Entries: []StartEntry{{ID: "ticket"}, {ID: "quick"}}}}}
		},
		Launch:    func(_ StartScope, id string) { launched = id },
		TogglePin: func(_ StartScope, e StartEntry) { pinned = e.ID },
	}
	header, items, ok := startSection(cfg, StartScope{}, false, nil)
	require.True(t, ok)
	assert.Equal(t, i18n.T("tui.start.section"), header)
	var labels []string
	for _, it := range items {
		labels = append(labels, it.Icon+" "+it.Label)
	}
	assert.Equal(t, []string{"★ ticket", "· " + i18n.T("tui.start.recents"), "↺ review", "… " + i18n.Tf("tui.start.all", 3)}, labels)
	items[0].Action()
	assert.Equal(t, "ticket", launched)
	assert.True(t, togglePinOf(cfg, StartScope{}, &items[0]))
	assert.Equal(t, "ticket", pinned)
	assert.False(t, togglePinOf(cfg, StartScope{}, &items[1]), "headers are not pinnable")

	var picked []SelectOption
	_, items, _ = startSection(cfg, StartScope{ProjectID: "p"}, true, func(_ string, opts []SelectOption, _ func(string)) { picked = opts })
	cat := items[len(items)-2]
	assert.Equal(t, "Développer (2)", cat.Label)
	cat.Action()
	assert.Len(t, picked, 2)

	empty := StartSectionConfig{Entries: func(StartScope) StartEntries { return StartEntries{Loaded: true} }, FreeSession: func(StartScope) {}}
	_, items, _ = startSection(empty, StartScope{}, false, nil)
	require.Len(t, items, 1)
	assert.Equal(t, i18n.T("tui.start.free"), items[0].Label, "free session without workflow")
}

func TestBoardQuickActions(t *testing.T) {
	var launched string
	qa := &BoardQuickActions{
		Workflows: func() []TicketWorkflow {
			return []TicketWorkflow{{ID: "ticket", Label: "Implémenter", Risk: "write", Runtimes: []string{"local", "container"}, Pinned: true}, {ID: "review", Risk: "read"}}
		},
		Launch: func(id string, tc TicketContext) { launched = id + ":" + tc.ID },
	}
	sh := &recordingShell{}
	showQuickActionModal(sh, TicketContext{ID: "bd-42", Title: "Export CSV"}, qa)
	require.Len(t, sh.options, 2)
	assert.True(t, strings.HasPrefix(sh.options[0].Label, "★ ticket"))
	assert.Contains(t, sh.options[0].Label, "⌂ ▣")

	cmds := ticketContextCommands("board.run.", TicketContext{ID: "bd-42"}, qa)
	require.Len(t, cmds, 2)
	assert.Equal(t, "run ticket ⟨bd-42⟩", cmds[0].Label)
	cmds[1].Action()
	assert.Equal(t, "review:bd-42", launched)
}

func TestWorkflowCatalogView(t *testing.T) {
	var launched, pinned string
	v := NewWorkflowCatalogView(WorkflowCatalogConfig{
		Launch:    func(id string) { launched = id },
		TogglePin: func(id string) { pinned = id },
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() {
		v.Mount(content, app)
		v.SetEntries([]CatalogEntry{
			{ID: "ticket", Layer: "hub", Risk: "write", Runtimes: []string{"local"}, Valid: true, Chain: []string{"hub:ticket"}},
			{ID: "broken", Layer: "hub", Valid: false, Problems: []string{"agent inconnu"}},
		})
	})
	var main string
	onLoop(app, func() {
		_, it, ok := v.list.CurrentItem()
		require.True(t, ok)
		main = it.MainText
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModNone))
	})
	assert.Contains(t, main, "ticket")
	assert.Equal(t, "ticket", pinned)
	assert.Empty(t, launched)
}

func TestLaunchModelOneSession(t *testing.T) {
	m := newLaunchModel(launchCfg(t))
	m.tickets = []string{"bd-1", "bd-2"}
	assert.NotEmpty(t, m.sessionsLine())
	m.oneSession = true
	assert.Empty(t, m.sessionsLine(), "one session: no « N sessions » line")
	assert.True(t, m.choices().OneSession)
}

func TestLaunchFormSuggestionRunsFirst(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) {
		return &LaunchRecap{Warnings: []string{"wiki absent"}, Suggestions: []LaunchSuggestion{{WorkflowID: "onboarding", Label: "Lancer onboarding d'abord"}}}, nil
	}
	first := make(chan string, 1)
	cfg.LaunchFirst = func(_ context.Context, _ LaunchChoices, id string) error { first <- id; return nil }
	cfg.Launch = func(context.Context, LaunchChoices) error { t.Error("original launch started"); return nil }
	v := NewLaunchFormView(cfg)
	sh := &recordingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app); v.next(); v.next() })
	var idx int
	require.Eventually(t, func() bool {
		idx = -1
		onLoop(app, func() { idx = v.form.GetButtonIndex("Lancer onboarding d'abord") })
		return idx >= 0
	}, 2*time.Second, 10*time.Millisecond, "suggestion button shown once the recap is loaded")
	onLoop(app, func() {
		v.app.SetFocus(v.form.GetButton(idx))
		v.form.GetButton(idx).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	})
	select {
	case id := <-first:
		assert.Equal(t, "onboarding", id)
	case <-time.After(2 * time.Second):
		t.Fatal("suggested workflow not launched")
	}
}

func TestSessionsDetailShowsNext(t *testing.T) {
	v := &SessionsView{detail: tview.NewTextView()}
	v.renderDetail(&SessionRow{ID: "ses_1", Workflow: "ticket", Next: "review"})
	assert.Contains(t, v.detail.GetText(true), i18n.Tf("tui.launch.chain_next", "review"))
}
