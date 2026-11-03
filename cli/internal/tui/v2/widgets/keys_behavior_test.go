package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

// The keys of the widgets, pinned before they went through the keys package:
// each key does what it did, whatever way the code reads it.

func keyEvent(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }
func runeEvent(r rune) *tcell.EventKey     { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func send(h func(*tcell.EventKey, func(tview.Primitive)), evs ...*tcell.EventKey) {
	for _, ev := range evs {
		h(ev, func(tview.Primitive) {})
	}
}

func TestCardColumnKeys(t *testing.T) {
	col := func() *CardColumn {
		c := NewCardColumn("col", tcell.ColorWhite)
		for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
			c.AddCard(Card{MainText: s, SecondaryText: s})
		}
		return c
	}
	for _, tc := range []struct {
		name string
		from int
		keys []*tcell.EventKey
		want int
	}{
		{"j moves down", 0, []*tcell.EventKey{runeEvent('j')}, 1},
		{"k moves up", 3, []*tcell.EventKey{runeEvent('k')}, 2},
		{"Down moves down", 0, []*tcell.EventKey{keyEvent(tcell.KeyDown)}, 1},
		{"Up moves up", 3, []*tcell.EventKey{keyEvent(tcell.KeyUp)}, 2},
		{"g goes to the first", 5, []*tcell.EventKey{runeEvent('g')}, 0},
		{"G goes to the last", 2, []*tcell.EventKey{runeEvent('G')}, 7},
		{"Home goes to the first", 5, []*tcell.EventKey{keyEvent(tcell.KeyHome)}, 0},
		{"End goes to the last", 2, []*tcell.EventKey{keyEvent(tcell.KeyEnd)}, 7},
		{"PgDn moves by five", 0, []*tcell.EventKey{keyEvent(tcell.KeyPgDn)}, 5},
		{"PgUp moves by five", 7, []*tcell.EventKey{keyEvent(tcell.KeyPgUp)}, 2},
		{"k stops at the first", 0, []*tcell.EventKey{runeEvent('k')}, 0},
		{"j stops at the last", 7, []*tcell.EventKey{runeEvent('j')}, 7},
		{"other letters do nothing", 3, []*tcell.EventKey{runeEvent('x'), runeEvent('h'), runeEvent(' ')}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := col()
			c.SetCurrentItem(tc.from)
			send(c.InputHandler(), tc.keys...)
			assert.Equal(t, tc.want, c.GetCurrentItem())
		})
	}

	empty := NewCardColumn("col", tcell.ColorWhite)
	send(empty.InputHandler(), runeEvent('G'), runeEvent('j'), keyEvent(tcell.KeyEnd))
	assert.Equal(t, -1, empty.GetCurrentItem(), "no card: nothing moves")
}

func TestSectionedListKeys(t *testing.T) {
	// items: 0 header, 1 header, 2 enabled, 3 url, 4 header, 5 header, 6 enabled, 7 url, 8 write
	cur := func(sl *SectionedList) int {
		i, _, ok := sl.CurrentItem()
		assert.True(t, ok)
		return i
	}
	for _, tc := range []struct {
		name string
		from int
		keys []*tcell.EventKey
		want int
	}{
		{"j moves down", 2, []*tcell.EventKey{runeEvent('j')}, 3},
		{"j skips the headers", 3, []*tcell.EventKey{runeEvent('j')}, 6},
		{"k moves up", 3, []*tcell.EventKey{runeEvent('k')}, 2},
		{"k skips the headers", 6, []*tcell.EventKey{runeEvent('k')}, 3},
		{"Down moves down", 3, []*tcell.EventKey{keyEvent(tcell.KeyDown)}, 6},
		{"Up moves up", 6, []*tcell.EventKey{keyEvent(tcell.KeyUp)}, 3},
		{"g goes to the first field", 8, []*tcell.EventKey{runeEvent('g')}, 2},
		{"G goes to the last field", 2, []*tcell.EventKey{runeEvent('G')}, 8},
		{"} jumps to the next section", 2, []*tcell.EventKey{runeEvent('}')}, 6},
		{"{ jumps back over the headers", 7, []*tcell.EventKey{runeEvent('{')}, 6},
		{"{ in the first section goes to its first field", 3, []*tcell.EventKey{runeEvent('{')}, 2},
		{"Tab jumps to the next section", 2, []*tcell.EventKey{keyEvent(tcell.KeyTab)}, 6},
		{"Backtab does what { does", 7, []*tcell.EventKey{keyEvent(tcell.KeyBacktab)}, 6},
		{"j at the end stays", 8, []*tcell.EventKey{runeEvent('j')}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sl := NewSectionedList()
			sl.SetItems(sectionFixture())
			sl.SelectIndex(tc.from)
			send(sl.InputHandler(), tc.keys...)
			assert.Equal(t, tc.want, cur(sl))
		})
	}

	t.Run("other letters are left to the list", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems(sectionFixture())
		x := runeEvent('x')
		assert.Same(t, x, sl.handleInput(x), "the event goes on")
		assert.Nil(t, sl.handleInput(runeEvent('j')), "j is consumed")
	})
	t.Run("Tab goes on when the parent switches columns", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems(sectionFixture())
		sl.SetTabCaptureDisabled(true)
		ev := keyEvent(tcell.KeyTab)
		assert.Same(t, ev, sl.handleInput(ev))
	})
}

func TestWorkflowGraphKeys(t *testing.T) {
	newGraph := func() *WorkflowGraph {
		g := NewModelGraph(SpecGraphModel(graphSpecOf(t), "semi-auto", "fr", "▶ Start", nil), false)
		g.SetRect(0, 0, 100, 24)
		return g
	}
	selection := func(g *WorkflowGraph) string { return g.SelectedElement().ID }
	// A letter does what its arrow does, from every element of the graph.
	for _, p := range []struct {
		letter rune
		arrow  tcell.Key
	}{{'h', tcell.KeyLeft}, {'l', tcell.KeyRight}, {'k', tcell.KeyUp}, {'j', tcell.KeyDown}} {
		moved := false
		for i := range newGraph().layout.Elements {
			a, b := newGraph(), newGraph()
			a.selected, b.selected = i, i
			start := selection(a)
			send(a.InputHandler(), keyEvent(p.arrow))
			send(b.InputHandler(), runeEvent(p.letter))
			assert.Equal(t, selection(a), selection(b), "%c from %s", p.letter, start)
			moved = moved || selection(a) != start
		}
		assert.True(t, moved, "%c moves the selection somewhere", p.letter)
	}

	t.Run("the selection change is announced", func(t *testing.T) {
		g := newGraph()
		changed := 0
		g.SetOnChange(func() { changed++ })
		send(g.InputHandler(), runeEvent('l'))
		assert.Equal(t, 1, changed)
		send(g.InputHandler(), runeEvent('x'))
		assert.Equal(t, 1, changed, "an unknown letter changes nothing")
	})
	t.Run("Enter activates the element", func(t *testing.T) {
		g := newGraph()
		var got string
		g.SetOnSelect(func(e GraphElement) { got = e.ID })
		send(g.InputHandler(), keyEvent(tcell.KeyEnter))
		assert.Equal(t, selection(g), got)
	})
}

func TestInlineSelectKeys(t *testing.T) {
	newSelect := func(changes *[]string) *InlineSelect {
		s := NewInlineSelect("Type", []string{"a", "b", "c"}, 0, func(v string, _ int) { *changes = append(*changes, v) })
		s.Focus(func(tview.Primitive) {})
		return s
	}
	var changes []string
	s := newSelect(&changes)
	cursor := func() int { return s.cursor }

	send(s.InputHandler(), runeEvent('j'))
	assert.Equal(t, 1, cursor(), "j moves the cursor down")
	assert.Equal(t, 0, s.GetSelectedIndex(), "without choosing")
	send(s.InputHandler(), runeEvent('j'), runeEvent('j'))
	assert.Equal(t, 2, cursor(), "j stops at the last option")
	send(s.InputHandler(), runeEvent('k'))
	assert.Equal(t, 1, cursor(), "k moves the cursor up")
	send(s.InputHandler(), keyEvent(tcell.KeyUp), keyEvent(tcell.KeyUp))
	assert.Equal(t, 0, cursor(), "Up moves up and stops at the first")
	send(s.InputHandler(), keyEvent(tcell.KeyDown))
	assert.Equal(t, 1, cursor(), "Down moves down")
	assert.Empty(t, changes)

	send(s.InputHandler(), runeEvent(' '))
	assert.Equal(t, 1, s.GetSelectedIndex(), "Space chooses the option under the cursor")
	assert.Equal(t, []string{"b"}, changes)
	send(s.InputHandler(), runeEvent(' '))
	assert.Equal(t, []string{"b"}, changes, "choosing the chosen option again changes nothing")

	send(s.InputHandler(), runeEvent('j'), keyEvent(tcell.KeyEnter))
	assert.Equal(t, 2, s.GetSelectedIndex(), "Enter chooses too")
	assert.Equal(t, []string{"b", "c"}, changes)

	send(s.InputHandler(), runeEvent('x'), runeEvent('g'))
	assert.Equal(t, 2, cursor(), "other letters do nothing")
}
