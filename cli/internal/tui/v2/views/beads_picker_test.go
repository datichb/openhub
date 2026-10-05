package views

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/workflow"
)

type fakeBeads struct {
	tickets []beads.Ticket
	err     error
	details map[string]*beads.TicketDetail
	shown   []string
}

func (f *fakeBeads) Tickets(context.Context) ([]beads.Ticket, error) { return f.tickets, f.err }
func (f *fakeBeads) Detail(_ context.Context, id string) (*beads.TicketDetail, error) {
	f.shown = append(f.shown, id)
	if d, ok := f.details[id]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

func sampleBeads() *fakeBeads {
	ai := []string{"ai-delegated"}
	return &fakeBeads{
		tickets: []beads.Ticket{
			{ID: "E-15", Title: "Auth", Type: "epic", Status: "open"},
			{ID: "E-12", Title: "Exports", Type: "epic", Status: "open"},
			{ID: "bd-43", Title: "Export Excel", Status: "open", Priority: "P2", Parent: "E-12", Labels: ai},
			{ID: "bd-42", Title: "Ajouter l'export CSV", Status: "open", Priority: "P1", Parent: "E-12", Labels: ai},
			{ID: "bd-51", Title: "Refresh token", Status: "in_progress", Priority: "P1", Parent: "E-15", Labels: ai},
			{ID: "bd-60", Title: "Doc README", Status: "open", Priority: "P3", Labels: []string{"docs"}},
			{ID: "bd-70", Title: "Done already", Status: "closed", Priority: "P1", Parent: "E-12", Labels: ai},
		},
		details: map[string]*beads.TicketDetail{
			"bd-42": {ID: "bd-42", Description: "Exporter la liste en CSV.", Acceptance: "- colonnes\n- encodage UTF-8\n- en-tête"},
		},
	}
}

func rowIDs(m *beadsPickerModel) []string {
	var out []string
	for _, r := range m.rows {
		if r.header {
			out = append(out, "#"+r.epicID)
		} else {
			out = append(out, r.ticket.ID)
		}
	}
	return out
}

func TestBeadsPickerModelGroupsAndFilters(t *testing.T) {
	m := newBeadsPickerModel(sampleBeads().tickets, "ai-delegated", "", nil)
	assert.Equal(t, []string{"#E-12", "bd-42", "bd-43", "#E-15", "bd-51"}, rowIDs(m), "grouped by epic, priority order, closed and other labels hidden")
	assert.Equal(t, "bd-42", m.current().ID)

	m.cycleLabel() // all labels
	assert.Equal(t, "", m.labelFilter())
	assert.Equal(t, []string{"#E-12", "bd-42", "bd-43", "#E-15", "bd-51", "#", "bd-60"}, rowIDs(m), "tickets without epic last")
	assert.Equal(t, "bd-42", m.current().ID, "cursor kept on the same ticket")

	m.cycleLabel() // docs
	assert.Equal(t, "docs", m.labelFilter())
	assert.Equal(t, []string{"#", "bd-60"}, rowIDs(m))
	m.cycleLabel()
	assert.Equal(t, "ai-delegated", m.labelFilter(), "cycles back")

	assert.Equal(t, []string{beadsEpicAll, "E-12", "E-15", beadsEpicNone}, m.epicOpts)
	m.cycleEpic()
	assert.Equal(t, []string{"#E-12", "bd-42", "bd-43"}, rowIDs(m))

	m = newBeadsPickerModel(sampleBeads().tickets, "", "E-15", nil)
	assert.Equal(t, []string{"#E-15", "bd-51"}, rowIDs(m), "initial epic from the workflow input")
	m = newBeadsPickerModel(sampleBeads().tickets, "", "E-99", nil)
	assert.Empty(t, m.rows, "unknown epic: empty list")
	assert.Nil(t, m.current())
}

func TestBeadsPickerModelSearchAndCursor(t *testing.T) {
	m := newBeadsPickerModel(sampleBeads().tickets, "", "", nil)
	m.setQuery("EXPORT csv")
	assert.Equal(t, []string{"#E-12", "bd-42"}, rowIDs(m), "every word must match (id, title, labels)")
	m.setQuery("docs")
	assert.Equal(t, []string{"#", "bd-60"}, rowIDs(m))
	m.setQuery("")
	assert.Equal(t, "bd-60", m.current().ID, "cursor kept on the same ticket when the query is cleared")

	m.moveToEdge(false)
	assert.Equal(t, "bd-42", m.current().ID)
	m.move(1)
	assert.Equal(t, "bd-43", m.current().ID)
	m.move(1)
	assert.Equal(t, "bd-51", m.current().ID, "headers are skipped")
	m.move(10)
	assert.Equal(t, "bd-60", m.current().ID, "clamped at the end")
	m.move(-10)
	assert.Equal(t, "bd-42", m.current().ID)
	m.moveToEdge(true)
	assert.Equal(t, "bd-60", m.current().ID)
}

func TestBeadsPickerModelSelection(t *testing.T) {
	m := newBeadsPickerModel(sampleBeads().tickets, "", "", []string{"bd-51", "ghost", "bd-51"})
	assert.Equal(t, []string{"bd-51"}, m.selected, "unknown and duplicate preselections dropped")
	assert.Equal(t, []string{"bd-42"}, m.result(false), "single: current ticket")
	m.toggle()
	assert.Equal(t, []string{"bd-51", "bd-42"}, m.result(true), "selection order")
	m.toggle()
	assert.Equal(t, []string{"bd-51"}, m.result(true))
	m.selected = nil
	assert.Equal(t, []string{"bd-42"}, m.result(true), "multi without selection: current ticket")
}

func TestBeadsPickerConfigApplyInput(t *testing.T) {
	var c BeadsPickerConfig
	c.ApplyInput(workflow.Input{Type: workflow.InputBeadsID, Picker: &workflow.Picker{Filter: "ai-delegated", Epic: "E-12"}})
	assert.Equal(t, BeadsPickerConfig{Filter: "ai-delegated", Epic: "E-12"}, c)
	c = BeadsPickerConfig{}
	c.ApplyInput(workflow.Input{Type: workflow.InputBeadsIDs})
	assert.True(t, c.Multi)
	c = BeadsPickerConfig{}
	c.ApplyInput(workflow.Input{Type: workflow.InputBeadsID, Picker: &workflow.Picker{Multi: true}})
	assert.True(t, c.Multi)
}

func TestAcceptanceCount(t *testing.T) {
	assert.Equal(t, 0, acceptanceCount(""))
	assert.Equal(t, 1, acceptanceCount("Doit marcher."))
	assert.Equal(t, 3, acceptanceCount("- a\n* b\n1. c\ntexte"))
}

// ── View: rendering and keys on a simulation screen ─────────────────────────

func withFrench(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

func drawPicker(t *testing.T, p *BeadsPicker) string {
	t.Helper()
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	t.Cleanup(sc.Fini)
	sc.SetSize(110, 22)
	p.SetRect(0, 0, 110, 22)
	p.Draw(sc)
	sc.Show()
	cells, w, h := sc.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r := cells[y*w+x].Runes; len(r) > 0 {
				b.WriteRune(r[0])
			} else {
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func press(p *BeadsPicker, keys ...any) {
	h := p.InputHandler()
	for _, k := range keys {
		switch v := k.(type) {
		case tcell.Key:
			h(tcell.NewEventKey(v, 0, tcell.ModNone), func(tview.Primitive) {})
		case string:
			for _, r := range v {
				h(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
			}
		}
	}
}

func TestBeadsPickerRendersGroupsClaimsAndPreview(t *testing.T) {
	withFrench(t)
	src := sampleBeads()
	p := NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{
		Source: src, Filter: "ai-delegated",
		ClaimedBy: func(id string) string {
			if id == "bd-51" {
				return "bob"
			}
			return ""
		},
	})
	p.Load()
	out := drawPicker(t, p)
	for _, want := range []string{
		"Choisir un ticket", "filtre : ai-delegated ▾", "épopée : tout ▾",
		"─ Épopée E-12 · Exports", "▸ bd-42", "Ajouter l'export CSV", "─ Épopée E-15 · Auth",
		"réservé par bob", "Aperçu", "bd-42 · Ajouter l'export CSV — critères d'acceptation : 3",
		"Exporter la liste en CSV.", "Enter choisir",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "bd-60", "label filter")
	assert.NotContains(t, out, "bd-70", "closed tickets hidden")
	assert.Equal(t, []string{"bd-42"}, src.shown, "preview loaded for the current ticket only")

	press(p, tcell.KeyDown)
	assert.Contains(t, drawPicker(t, p), "aperçu indisponible", "missing detail reported in the preview")
}

func TestBeadsPickerKeysSingle(t *testing.T) {
	var done []string
	canceled := false
	p := NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{
		Source:   sampleBeads(),
		OnDone:   func(ids []string) { done = ids },
		OnCancel: func() { canceled = true },
	})
	p.Load()

	press(p, "/")
	assert.True(t, p.CapturesInput(), "search mode captures printable keys")
	press(p, "excel")
	assert.Equal(t, "bd-43", p.m.current().ID)
	press(p, tcell.KeyEnter)
	assert.False(t, p.CapturesInput())
	assert.Nil(t, done, "Enter in the search field only leaves it")

	press(p, " ") // no multi-selection in single mode
	assert.Empty(t, p.Selected())
	press(p, tcell.KeyEnter)
	assert.Equal(t, []string{"bd-43"}, done)

	press(p, "/", "zzz", tcell.KeyEscape)
	assert.Equal(t, "", p.m.query, "Esc clears the search")
	assert.Equal(t, 4, p.m.ticketCount())
	assert.False(t, canceled)
	press(p, tcell.KeyEscape)
	assert.True(t, canceled)
}

func TestBeadsPickerKeysMultiAndFilters(t *testing.T) {
	withFrench(t)
	var done []string
	p := NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{
		Source: sampleBeads(), Filter: "ai-delegated", Multi: true,
		OnDone: func(ids []string) { done = ids },
	})
	p.Load()
	press(p, " ", " ") // selects bd-42 then bd-43 (cursor advances)
	assert.Equal(t, []string{"bd-42", "bd-43"}, p.Selected())
	out := drawPicker(t, p)
	assert.Contains(t, out, "Choisir des tickets")
	assert.Contains(t, out, "2 sélectionné(s)")
	assert.Contains(t, out, "● bd-42")

	press(p, "e") // epic E-12
	assert.Contains(t, drawPicker(t, p), "épopée : E-12 · Exports ▾")
	press(p, "e", "e") // E-15, then no epic
	press(p, "f")      // all labels
	assert.Equal(t, []string{"#", "bd-60"}, rowIDs(p.m))
	press(p, " ", tcell.KeyEnter)
	assert.Equal(t, []string{"bd-42", "bd-43", "bd-60"}, done, "selection kept across filters")
}

func TestBeadsPickerStates(t *testing.T) {
	withFrench(t)
	p := NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{Source: &fakeBeads{err: errors.New("bd absent")}})
	p.Load()
	assert.Contains(t, drawPicker(t, p), "Impossible de lire les tickets : bd absent")

	p = NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{Source: &fakeBeads{}})
	p.Load()
	assert.Contains(t, drawPicker(t, p), "Aucun ticket ne correspond")
	press(p, tcell.KeyEnter) // nothing to pick: no callback, no panic

	p = NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{})
	p.Load()
	assert.Error(t, p.loadErr)
}

// The list scrolls to keep the cursor visible on a small screen.
func TestBeadsPickerScrolls(t *testing.T) {
	var ts []beads.Ticket
	for i := 0; i < 40; i++ {
		ts = append(ts, beads.Ticket{ID: "t-" + string(rune('a'+i/26)) + string(rune('a'+i%26)), Title: "x", Status: "open"})
	}
	p := NewBeadsPicker(context.Background(), nil, BeadsPickerConfig{Source: &fakeBeads{tickets: ts}})
	p.Load()
	press(p, tcell.KeyEnd)
	out := drawPicker(t, p)
	assert.Contains(t, out, "▸ t-bn", "last ticket visible")
	assert.NotContains(t, out, "t-aa")
}

// With an application, tickets and previews load in the background and are
// applied on the event loop.
func TestBeadsPickerAsyncLoad(t *testing.T) {
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(100, 20)
	app := tview.NewApplication().SetScreen(sc)
	p := NewBeadsPicker(context.Background(), app, BeadsPickerConfig{Source: sampleBeads()})
	app.SetRoot(p, true).SetFocus(p)
	errc := make(chan error, 1)
	go func() { errc <- app.Run() }()
	t.Cleanup(func() { app.Stop(); <-errc })

	p.Load()
	require.Eventually(t, func() bool {
		ready := make(chan bool, 1)
		app.QueueUpdate(func() { _, ok := p.details["bd-42"]; ready <- !p.loading && ok })
		return <-ready
	}, 5*time.Second, 20*time.Millisecond, "tickets then preview loaded")
}
