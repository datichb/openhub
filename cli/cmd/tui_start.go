package cmd

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of the « Démarrer » section (P1-T20): a cache of the entries
// of each landing scope, filled off the event loop from the WorkflowService
// (catalogue) and the PreferenceService (pins, recents, suggestions).

const tuiStartTTL = 30 * time.Second

type tuiStart struct {
	a     *app.App
	prefs *prefsvc.Service

	mu      sync.Mutex
	entries map[views.StartScope]views.StartEntries
	loaded  map[views.StartScope]time.Time
	loading map[views.StartScope]bool
	catalog []workflowsvc.Summary
}

func newTUIStart(a *app.App) *tuiStart {
	return &tuiStart{a: a, prefs: prefsvc.New(a.Preferences, a.WorkflowUsage),
		entries: map[views.StartScope]views.StartEntries{}, loaded: map[views.StartScope]time.Time{},
		loading: map[views.StartScope]bool{}}
}

// sectionConfig is the « Démarrer » section of the landings.
func (t *tuiStart) sectionConfig() views.StartSectionConfig {
	return views.StartSectionConfig{
		Entries: t.get,
		Launch: func(scope views.StartScope, id string) {
			openLaunchForm(t.a, tuiLaunchRequest{WorkflowID: id, ProjectID: scope.ProjectID})
		},
		TogglePin:   t.togglePin,
		OpenCatalog: func(views.StartScope) { openWorkflowCatalog() },
		FreeSession: func(views.StartScope) { launchSessionWithPrompt("", "") },
	}
}

// get returns the cached entries (event loop safe) and refreshes them in the
// background when missing or stale.
func (t *tuiStart) get(scope views.StartScope) views.StartEntries {
	t.mu.Lock()
	e, ok := t.entries[scope]
	stale := !ok || time.Since(t.loaded[scope]) > tuiStartTTL
	if stale && !t.loading[scope] {
		t.loading[scope] = true
		go t.load(scope)
	}
	t.mu.Unlock()
	return e
}

// invalidate forgets every cached scope (after a pin, a launch).
func (t *tuiStart) invalidate() {
	t.mu.Lock()
	t.loaded = map[views.StartScope]time.Time{}
	t.mu.Unlock()
}

func (t *tuiStart) load(scope views.StartScope) {
	ctx := context.Background()
	e, err := t.compute(ctx, scope)
	t.mu.Lock()
	t.loading[scope] = false
	if err != nil {
		slog.Debug("start section", "error", err)
		e = views.StartEntries{Loaded: true}
	}
	t.entries[scope] = e
	t.loaded[scope] = time.Now()
	t.mu.Unlock()
	if sh := tuiShell; sh != nil {
		sh.App().QueueUpdateDraw(func() { sh.RemountIf("home", "project.mode", "team.mode") })
	}
}

// compute builds the entries of a scope.
func (t *tuiStart) compute(ctx context.Context, scope views.StartScope) (views.StartEntries, error) {
	if !v5Available(ctx) {
		// Workflows need opencode V2: the free session stays (V1).
		return views.StartEntries{Loaded: true}, nil
	}
	list, err := newWorkflowService(ctx).Catalog(ctx, workflowsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID})
	if err != nil {
		return views.StartEntries{}, err
	}
	var valid []workflowsvc.Summary
	for _, s := range list {
		if s.Valid {
			valid = append(valid, s)
		}
	}
	t.mu.Lock()
	t.catalog = valid
	t.mu.Unlock()
	registerWorkflowCommands(t.a, valid)
	out := views.StartEntries{Loaded: true, Total: len(valid)}
	if len(valid) == 0 {
		return out, nil
	}
	start, err := t.prefs.Start(ctx, prefsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID}, workflowsvc.IDs(valid))
	if err != nil {
		return out, err
	}
	entry := func(id string) views.StartEntry {
		s, _ := workflowsvc.Find(valid, id)
		return views.StartEntry{ID: id, Label: summaryDesc(s), Origin: summaryOrigin(s)}
	}
	for _, p := range start.Pinned {
		e := entry(p.WorkflowID)
		e.Pinned, e.PinScope = true, p.Scope
		out.Pinned = append(out.Pinned, e)
	}
	for _, r := range start.Recent {
		e := entry(r.WorkflowID)
		e.Ago = agoLabel(time.Since(r.LastUsed))
		out.Recents = append(out.Recents, e)
	}
	for _, id := range start.Suggested {
		out.Suggestions = append(out.Suggestions, entry(id))
	}
	for _, c := range workflowsvc.CategoryOrder {
		cat := views.StartCategory{ID: string(c), Label: i18n.T("tui.start.category." + string(c))}
		for _, s := range valid {
			if s.Category == c || (c == "other" && s.Category == "") {
				cat.Entries = append(cat.Entries, entry(s.ID))
			}
		}
		if len(cat.Entries) > 0 {
			out.Categories = append(out.Categories, cat)
		}
	}
	return out, nil
}

// togglePin pins or unpins in the scope of the landing (or the entry's).
func (t *tuiStart) togglePin(scope views.StartScope, e views.StartEntry) {
	pinScope := e.PinScope
	if pinScope == "" {
		pinScope = prefsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID}.Scopes()[0]
	}
	go func() {
		pinned, err := t.prefs.TogglePin(context.Background(), pinScope, e.ID)
		msg, ok := prefsvc.PinMessage(e.ID, pinScope, pinned), true
		if err != nil {
			msg, ok = prefsvc.ErrorMessage(err), false
		}
		t.invalidate()
		if sh := tuiShell; sh != nil {
			sh.App().QueueUpdateDraw(func() {
				if ok {
					sh.ShowToast(msg, shell.ToastSuccess)
				} else {
					sh.ShowToast(msg, shell.ToastWarning)
				}
			})
		}
		t.load(scope)
	}()
}

// summaryDesc is the short description of a workflow.
func summaryDesc(s workflowsvc.Summary) string {
	if s.Description != "" {
		return s.Description
	}
	if s.Label != s.ID {
		return s.Label
	}
	return ""
}

// summaryOrigin is « hub », « équipe v3 »…
func summaryOrigin(s workflowsvc.Summary) string {
	o := i18n.T("tui.start.origin_" + string(s.Layer))
	if s.Version > 0 {
		o += " v" + strconv.Itoa(s.Version)
	}
	return o
}

func agoLabel(d time.Duration) string {
	switch {
	case d < time.Hour:
		return i18n.Tf("tui.start.ago_minutes", max(int(d.Minutes()), 1))
	case d < 48*time.Hour:
		return i18n.Tf("tui.start.ago_hours", int(d.Hours()))
	default:
		return i18n.Tf("tui.start.ago_days", int(d.Hours()/24))
	}
}

// tuiStartWiring is the « Démarrer » wiring of the running TUI.
var tuiStartWiring *tuiStart

// ticketWorkflows returns the workflows taking a Beads ticket, pinned ones
// first (board quick actions; event loop safe: cached catalogue).
func (t *tuiStart) ticketWorkflows(scope views.StartScope) []views.TicketWorkflow {
	t.mu.Lock()
	catalog := t.catalog
	e := t.entries[scope]
	t.mu.Unlock()
	if catalog == nil {
		t.get(scope) // first use: load in the background
	}
	pinned := map[string]bool{}
	for _, p := range e.Pinned {
		pinned[p.ID] = true
	}
	var first, rest []views.TicketWorkflow
	for _, s := range catalog {
		if s.TicketInput == "" {
			continue
		}
		w := views.TicketWorkflow{ID: s.ID, Label: summaryDesc(s), Risk: string(s.Risk), Pinned: pinned[s.ID]}
		for _, r := range s.Runtimes {
			w.Runtimes = append(w.Runtimes, string(r))
		}
		if w.Pinned {
			first = append(first, w)
		} else {
			rest = append(rest, w)
		}
	}
	return append(first, rest...)
}
