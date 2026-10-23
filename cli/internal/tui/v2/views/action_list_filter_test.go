package views

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

var (
	slash = tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
	esc   = tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
)

// filterable is the list of the sketch with a filter; asked records the
// prompts of the filter, and typed is what the user answers.
type filterable struct {
	*ActionList
	rec    []string
	asked  []string
	titles []string
	typed  string
}

func newFilterable(t *testing.T, filter *ActionListFilter) *filterable {
	t.Helper()
	f := &filterable{}
	cfg := wikiListConfig(&f.rec)
	cfg.Filter = filter
	cfg.AskQuery = func(title, current string, onDone func(string)) {
		f.titles, f.asked = append(f.titles, title), append(f.asked, current)
		onDone(f.typed)
	}
	f.ActionList = NewActionList(cfg)
	f.SetItems(wikiProposals(9))
	return f
}

func ids(items []ActionItem) (out []string) {
	for _, it := range items {
		out = append(out, it.ID)
	}
	return
}

// A list without Filter leaves / to the shell (palette), and does not list it
// in its hints.
func TestActionListWithoutFilterLeavesTheSlash(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, nil)
	assert.Same(t, slash, f.HandleKey(slash))
	assert.Empty(t, f.asked)
	assert.NotContains(t, f.Hints(), "/")
	f.SetQuery("stack")
	assert.Empty(t, f.Query(), "no filter, no query")
	assert.Len(t, f.Visible(), 9)
}

// / asks words, the list keeps the elements that hold all of them, whatever
// the case, in any cell or in the state; the query is in the title; Esc
// clears the filter, then leaves.
func TestActionListFilterNarrowsTheList(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, &ActionListFilter{})
	assert.Contains(t, f.Hints(), "/ filtrer", "the hint of the filter")

	f.typed = "STACK remplace"
	assert.Nil(t, f.HandleKey(slash), "the key is used")
	assert.Equal(t, []string{""}, f.asked, "the current query is offered")
	assert.Equal(t, []string{"Filtrer (mots séparés par des espaces, vide pour tout afficher)"}, f.titles)
	assert.Equal(t, "STACK remplace", f.Query())
	assert.Equal(t, []string{"wp-2"}, ids(f.Visible()))
	assert.Len(t, f.Items(), 9, "the elements are still there")
	assert.Equal(t, 9, f.Len())
	text := screenText(t, f.Primitive(), 90, 14)
	assert.Contains(t, text, "· /STACK remplace")
	assert.Contains(t, text, "wp-2")
	assert.NotContains(t, text, "wp-3")
	assert.Contains(t, text, "1/1", "the position counts what is shown")
	it, ok := f.Current()
	require.True(t, ok)
	assert.Equal(t, "wp-2", it.ID)

	f.typed = "refus" // the state is searched too
	f.HandleKey(slash)
	assert.Equal(t, []string{"STACK remplace"}, f.asked[1:])
	assert.Equal(t, []string{"wp-3"}, ids(f.Visible()))

	assert.Nil(t, f.HandleKey(esc), "the first Esc clears the filter")
	assert.Empty(t, f.Query())
	assert.Len(t, f.Visible(), 9)
	assert.NotContains(t, screenText(t, f.Primitive(), 90, 14), "/refus")
	assert.Same(t, esc, f.HandleKey(esc), "the next one leaves the view")
}

// An empty answer shows everything again; a query nobody matches says so.
func TestActionListFilterWithoutMatch(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, &ActionListFilter{})
	f.typed = "zzz"
	f.HandleKey(slash)
	assert.Empty(t, f.Visible())
	_, ok := f.Current()
	assert.False(t, ok, "nothing under the cursor")
	assert.Contains(t, screenText(t, f.Primitive(), 90, 14), "Aucun élément ne correspond à « zzz »")
	f.rec = nil
	pressList(f.ActionList, tcell.KeyRune, 'y')
	assert.Empty(t, f.rec, "no action on nothing")

	f.typed = "  "
	f.HandleKey(slash)
	assert.Empty(t, f.Query())
	assert.Len(t, f.Visible(), 9)
}

// The filter belongs to the list: the elements it receives later are
// filtered, the selected element stays selected while it passes, and the
// sections only show for the elements shown.
func TestActionListFilterKeepsTheSelectionAndTheSections(t *testing.T) {
	frLocale(t)
	cfg := ActionListConfig{Title: "q", Filter: &ActionListFilter{}, Layout: DetailNone,
		Actions: []ItemAction{{Key: keys.Accept, Run: func(ActionItem, string) {}}}}
	l := NewActionList(cfg)
	items := []ActionItem{
		{ID: "a", Section: "En attente", Cells: []string{"a", "docs"}},
		{ID: "b", Section: "En attente", Cells: []string{"b", "code"}},
		{ID: "c", Section: "Terminées", Cells: []string{"c", "docs"}},
		{ID: "d", Section: "Périmées", Cells: []string{"d", "code"}},
	}
	l.SetItems(items)
	l.Select("c")
	l.SetQuery("docs")
	assert.Equal(t, []string{"a", "c"}, ids(l.Visible()))
	cur, _ := l.Current()
	assert.Equal(t, "c", cur.ID, "still selected")
	text := screenText(t, l.Primitive(), 60, 10)
	assert.Contains(t, text, "En attente")
	assert.Contains(t, text, "Terminées")
	assert.NotContains(t, text, "Périmées", "a section without element shown has no title")
	assert.False(t, l.Select("b"), "a hidden element cannot be selected")

	l.SetItems(append(items, ActionItem{ID: "e", Section: "En attente", Cells: []string{"e", "docs"}}))
	assert.Equal(t, []string{"a", "c", "e"}, ids(l.Visible()), "the new elements are filtered")
	cur, _ = l.Current()
	assert.Equal(t, "c", cur.ID)

	// An element that changes can leave the filter.
	l.SetQuery("termin")
	assert.Equal(t, []string{"c"}, ids(l.Visible()), "the section is searched")
	l.SetQuery("")
	assert.True(t, l.Update("b", func(it *ActionItem) { it.State = ItemAccepted }))
	l.SetQuery("accepté")
	assert.Equal(t, []string{"b"}, ids(l.Visible()), "the label of the state is searched")
}

// A global action applies to the elements shown: what the user sees is what
// is confirmed and done.
func TestActionListFilterGlobalActionAppliesToTheVisible(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, &ActionListFilter{})
	f.typed = "tests"
	f.HandleKey(slash)
	pressList(f.ActionList, tcell.KeyRune, 'A')
	assert.Equal(t, []string{"accept all 1"}, f.rec)
}

// A view may decide what passes (Match), and the prompt can be its own.
func TestActionListFilterCustomMatch(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, &ActionListFilter{
		Prompt: "Page",
		Match:  func(it ActionItem, q string) bool { return it.Cells[1] == q },
	})
	f.typed = "business/index.md"
	f.HandleKey(slash)
	assert.Equal(t, []string{"Page"}, f.titles)
	assert.Equal(t, []string{"wp-3"}, ids(f.Visible()))
	f.typed = "index"
	f.HandleKey(slash)
	assert.Empty(t, f.Visible(), "the match is the view's")
}

// Without AskQuery the words are asked through the shell, like the filters
// of the other views.
func TestActionListFilterAsksThroughTheShell(t *testing.T) {
	frLocale(t)
	sh := &inputShell{answer: "technical"}
	l := NewActionList(ActionListConfig{Filter: &ActionListFilter{}, Shell: sh})
	items := []ActionItem{{ID: "a", Cells: []string{"technical/a.md"}}, {ID: "b", Cells: []string{"business/b.md"}}}
	l.SetItems(items)
	assert.Nil(t, l.HandleKey(slash))
	assert.Equal(t, "technical", l.Query())
	assert.Equal(t, []string{"a"}, ids(l.Visible()))
	assert.Equal(t, "technical", func() string { l.HandleKey(slash); return sh.current }(), "the query is offered again")
}

type inputShell struct {
	mockShell
	answer, current string
}

func (s *inputShell) ShowInputModal(_, current string, onConfirm func(string)) {
	s.current = current
	onConfirm(s.answer)
}

// The footer and the empty text follow the filter.
func TestActionListFilterFooterCountsWhatIsShown(t *testing.T) {
	frLocale(t)
	f := newFilterable(t, &ActionListFilter{})
	assert.Contains(t, screenText(t, f.Primitive(), 90, 14), fmt.Sprintf("1/%d", 9))
	f.SetQuery("conventions")
	assert.Equal(t, 5, len(f.Visible()))
	assert.Contains(t, screenText(t, f.Primitive(), 90, 14), "1/5")
}

// A list without elements and without text says so in the common words.
func TestActionListEmptyDefaultText(t *testing.T) {
	frLocale(t)
	l := NewActionList(ActionListConfig{Layout: DetailNone})
	assert.Contains(t, screenText(t, l.Primitive(), 60, 6), "Rien à afficher.")
}
