package views

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func frLocale(t *testing.T) {
	t.Helper()
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

// assertGolden compares got with testdata/<name>.golden (-update rewrites
// it).
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file (go test ./internal/tui/v2/views -run %s -update)", t.Name())
	assert.Equal(t, string(want), got)
}

// wikiProposals are the proposals of the sketch of the checkpoint card.
func wikiProposals(n int) []ActionItem {
	base := []ActionItem{
		{ID: "wp-1", Cells: []string{"wp-1", "technical/stack.md", "ajout", "CONFIRMÉ"}, State: ItemAccepted},
		{ID: "wp-2", Cells: []string{"wp-2", "technical/stack.md", "remplace", "PROBABLE"}, State: ItemPending,
			DetailTitle: "wp-2 · technical/stack.md · section « Paquets »",
			Detail:      "[red]− - `internal/store` : stockage SQLite[-]\n[green]+ - `internal/storage` : stockage SQLite (renommé, commit 3f2a1c)[-]\nsource : internal/storage/store.go:1 · auteur onboarder"},
		{ID: "wp-3", Cells: []string{"wp-3", "business/index.md", "ajout", "PROBABLE"}, State: ItemRefused},
		{ID: "wp-4", Cells: []string{"wp-4", "technical/tests.md", "remplace", "CONFIRMÉ"}, State: ItemPending},
	}
	out := append([]ActionItem(nil), base...)
	for i := len(out); i < n; i++ {
		id := fmt.Sprintf("wp-%d", i+1)
		out = append(out, ActionItem{ID: id, Cells: []string{id, "technical/conventions.md", "ajout", "PROBABLE"}, State: ItemPending})
	}
	return out[:n]
}

// fakeDialogs records the confirmations, the instruction prompts and the
// toasts of a list.
type fakeDialogs struct {
	confirms []string
	danger   bool
	yes      func()
	reasons  []string
	answer   func(string)
	toasts   []string
}

func (f *fakeDialogs) config(cfg ActionListConfig) ActionListConfig {
	cfg.Confirm = func(title, _ string, danger bool, onYes func()) {
		f.confirms, f.danger, f.yes = append(f.confirms, title), danger, onYes
	}
	cfg.AskReason = func(title string, onDone func(string)) { f.reasons, f.answer = append(f.reasons, title), onDone }
	cfg.Toast = func(msg string, _ bool) { f.toasts = append(f.toasts, msg) }
	return cfg
}

func wikiListConfig(rec *[]string) ActionListConfig {
	run := func(name string) func(ActionItem, string) {
		return func(it ActionItem, reason string) { *rec = append(*rec, name+" "+it.ID+" "+reason) }
	}
	pending := func(it ActionItem) bool { return it.State == ItemPending }
	return ActionListConfig{
		Title:       "cp-wiki · onboarding · 9 propositions",
		StateLabels: map[ItemState]string{ItemAccepted: "✓ acceptée", ItemRefused: "✗ refusée"},
		Actions: []ItemAction{
			{Key: keys.Accept, Label: "accepter", Applies: pending, Run: run("accept")},
			{Key: keys.Refuse, Applies: pending, Run: run("refuse")},
			{Key: keys.Modify, Run: run("modify")},
			{Key: keys.Leave, Run: run("leave")},
		},
		Global: []GlobalAction{{Key: keys.AcceptAll, Run: func(items []ActionItem) {
			*rec = append(*rec, fmt.Sprintf("accept all %d", len(items)))
		}}},
		OnOpen:    func(it ActionItem) { *rec = append(*rec, "open "+it.ID) },
		OpenLabel: "valider cp-wiki",
	}
}

func pressList(l *ActionList, k tcell.Key, r rune) {
	l.table.InputHandler()(tcell.NewEventKey(k, r, tcell.ModNone), func(tview.Primitive) {})
}

// Rendering of the sketch: proposals, state, detail below; the list says
// how many are below.
func TestActionListGoldenBelow(t *testing.T) {
	frLocale(t)
	var rec []string
	l := NewActionList(wikiListConfig(&rec))
	l.SetItems(wikiProposals(9))
	l.Select("wp-2")
	assertGolden(t, "action_list_below", screenText(t, l.Primitive(), 80, 18))
	assert.Equal(t, "y accepter · n refuser · e modifier · s laisser · A tout accepter · Entrée valider cp-wiki", l.Hints())
}

// Sections, column headers and the detail beside the list (project wiki
// queue).
func TestActionListGoldenRight(t *testing.T) {
	frLocale(t)
	l := NewActionList(ActionListConfig{
		Title:   "Wiki du projet · openhub",
		Layout:  DetailRight,
		Headers: []string{"id", "page", "type", "âge"},
		Actions: []ItemAction{{Key: keys.Accept, Run: func(ActionItem, string) {}}},
	})
	l.SetItems([]ActionItem{
		{ID: "wp-17", Section: "En attente (doc-sync, sans commit)", Cells: []string{"wp-17", "conventions.md", "ajout", "3 h"},
			DetailTitle: "wp-17 · technical/conventions.md", Detail: "doc-sync · ses_41c0 · il y a 3 h · PROBABLE\n+ - Les commits suivent Conventional Commits."},
		{ID: "wp-18", Section: "En attente (doc-sync, sans commit)", Cells: []string{"wp-18", "stack.md", "remplace", "3 h"}},
		{ID: "wp-12", Section: "Acceptées, à appliquer au prochain commit", Cells: []string{"wp-12", "tests.md", "ajout", "1 j"}},
		{ID: "wp-9", Section: "Périmées (ligne disparue)", Cells: []string{"wp-9", "architecture.md", "remplace", "2 j"}},
	})
	assertGolden(t, "action_list_right", screenText(t, l.Primitive(), 110, 14))
}

func TestActionListEmpty(t *testing.T) {
	frLocale(t)
	l := NewActionList(ActionListConfig{Title: "Briques", Empty: "Aucune proposition", Layout: DetailNone})
	l.SetItems(nil)
	assert.Contains(t, screenText(t, l.Primitive(), 60, 6), "Aucune proposition")
	_, ok := l.Current()
	assert.False(t, ok)
}

// y, e, s run at once on the selected element; n asks an instruction
// (required); A and Enter reach the global action and OnOpen.
func TestActionListDecisionKeys(t *testing.T) {
	frLocale(t)
	var rec []string
	f := &fakeDialogs{}
	l := NewActionList(f.config(wikiListConfig(&rec)))
	l.SetItems(wikiProposals(9))
	l.Select("wp-2")

	pressList(l, tcell.KeyRune, 'y')
	pressList(l, tcell.KeyRune, 'e')
	pressList(l, tcell.KeyRune, 's')
	assert.Equal(t, []string{"accept wp-2 ", "modify wp-2 ", "leave wp-2 "}, rec)

	rec = nil
	pressList(l, tcell.KeyRune, 'n')
	require.Equal(t, []string{"Refuser wp-2 : consigne"}, f.reasons)
	f.answer("  ")
	assert.Empty(t, rec, "an empty instruction refuses nothing")
	assert.Equal(t, []string{"Une consigne est obligatoire"}, f.toasts)
	f.answer("pas de source")
	assert.Equal(t, []string{"refuse wp-2 pas de source"}, rec)

	// Not applicable: accepted element.
	rec, f.toasts = nil, nil
	l.Select("wp-1")
	pressList(l, tcell.KeyRune, 'y')
	assert.Empty(t, rec)
	assert.Equal(t, []string{"« y accepter » ne s'applique pas à wp-1"}, f.toasts)

	pressList(l, tcell.KeyRune, 'A')
	pressList(l, tcell.KeyEnter, 0)
	assert.Equal(t, []string{"accept all 9", "open wp-1"}, rec)

	// Moves are left to the table.
	assert.NotNil(t, l.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)))
	pressList(l, tcell.KeyDown, 0)
	it, _ := l.Current()
	assert.Equal(t, "wp-2", it.ID)
}

// d always asks a confirmation (destructive); a global action with a
// confirmation waits for it.
func TestActionListConfirmations(t *testing.T) {
	frLocale(t)
	var rec []string
	f := &fakeDialogs{}
	l := NewActionList(f.config(ActionListConfig{
		Actions: []ItemAction{
			{Key: keys.Delete, Run: func(it ActionItem, _ string) { rec = append(rec, "delete "+it.ID) }},
			{Key: keys.Archive, Run: func(it ActionItem, _ string) { rec = append(rec, "archive "+it.ID) }},
		},
		Global: []GlobalAction{{Key: keys.AcceptAll, Confirm: func(items []ActionItem) (string, string) {
			return fmt.Sprintf("Tout accepter (%d) ?", len(items)), ""
		}, Run: func([]ActionItem) { rec = append(rec, "all") }}},
	}))
	l.SetItems(wikiProposals(3))

	pressList(l, tcell.KeyRune, 'd')
	assert.Empty(t, rec)
	assert.Equal(t, []string{"Supprimer wp-1 ?"}, f.confirms)
	assert.True(t, f.danger)
	f.yes()
	assert.Equal(t, []string{"delete wp-1"}, rec)

	pressList(l, tcell.KeyRune, 'x')
	assert.Equal(t, []string{"delete wp-1", "archive wp-1"}, rec, "x never asks")

	pressList(l, tcell.KeyRune, 'A')
	assert.Equal(t, "Tout accepter (3) ?", f.confirms[1])
	f.yes()
	assert.Equal(t, "all", rec[2])
}

// No limit: the list scrolls and tells the position and how many are
// below.
func TestActionListScrolls(t *testing.T) {
	frLocale(t)
	l := NewActionList(ActionListConfig{Layout: DetailNone})
	l.SetItems(wikiProposals(50))
	screen := screenText(t, l.Primitive(), 60, 8)
	assert.Contains(t, screen, "1/50  ·  45 de plus ↓")
	pressList(l, tcell.KeyEnd, 0)
	screen = screenText(t, l.Primitive(), 60, 8)
	assert.Contains(t, screen, "50/50")
	assert.NotContains(t, screen, "de plus")
	assert.Contains(t, screen, "▸ wp-50")
}

// SetItems keeps the selected element; SetState and Update change one in
// place.
func TestActionListUpdates(t *testing.T) {
	frLocale(t)
	var changed []string
	l := NewActionList(ActionListConfig{OnChange: func(it ActionItem) { changed = append(changed, it.ID) }})
	l.SetItems(wikiProposals(4))
	require.True(t, l.Select("wp-3"))
	l.SetItems(wikiProposals(5))
	it, _ := l.Current()
	assert.Equal(t, "wp-3", it.ID)
	assert.Equal(t, 5, l.Len())

	require.True(t, l.SetState("wp-4", ItemAccepted))
	assert.Equal(t, ItemAccepted, l.Items()[3].State)
	assert.Contains(t, screenText(t, l.Primitive(), 80, 12), "✓ accepté")
	assert.False(t, l.SetState("wp-99", ItemAccepted))
	assert.False(t, l.Select("wp-99"))
	assert.Contains(t, changed, "wp-3")
}

// Validate refuses two actions on one key, a key every view keeps, an
// action without Run.
func TestActionListValidate(t *testing.T) {
	noop := func(ActionItem, string) {}
	assert.NoError(t, wikiListConfig(new([]string)).Validate())
	assert.Error(t, ActionListConfig{Actions: []ItemAction{{Key: keys.Refuse, Run: noop}, {Key: keys.Create, Run: noop}}}.Validate())
	assert.Error(t, ActionListConfig{Actions: []ItemAction{{Key: keys.Refresh, Run: noop}}}.Validate())
	assert.Error(t, ActionListConfig{Actions: []ItemAction{{Key: keys.Accept}}}.Validate())
	assert.Error(t, ActionListConfig{Global: []GlobalAction{{Key: keys.AcceptAll}}}.Validate())
}
