package picker

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func testItems() []Item {
	return []Item{
		{ID: "1", Label: "Alpha", Category: "Group A", Description: "First item"},
		{ID: "2", Label: "Beta", Category: "Group A", Description: "Second item"},
		{ID: "3", Label: "Gamma", Category: "Group B", Description: "Third item"},
		{ID: "4", Label: "Delta", Category: "Group B", Description: "Fourth item"},
		{ID: "5", Label: "Epsilon", Category: "Group C", Description: "Fifth item"},
	}
}

func TestNew(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, 5, len(m.items))
	assert.Equal(t, "Test", m.config.Title)
	assert.Equal(t, 10, m.config.PageSize)
}

func TestNavigation_Down(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	assert.Equal(t, 1, m.cursor)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	assert.Equal(t, 2, m.cursor)
}

func TestNavigation_Up(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})
	m.cursor = 3

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)
	assert.Equal(t, 2, m.cursor)
}

func TestNavigation_BoundsCheck(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	// Can't go above 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)
	assert.Equal(t, 0, m.cursor)

	// Can't go below last item
	m.cursor = 4
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	assert.Equal(t, 4, m.cursor)
}

func TestSingleSelect_Enter(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})
	m.cursor = 2

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	assert.True(t, m.done)
	assert.NotNil(t, cmd) // tea.Quit
	assert.False(t, m.result.Aborted)
	assert.Len(t, m.result.Selected, 1)
	assert.Equal(t, "3", m.result.Selected[0].ID)
	assert.Equal(t, "Gamma", m.result.Selected[0].Label)
}

func TestMultiSelect_Space(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems(), MultiSelect: true})

	// Select first item
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = updated.(Model)
	assert.True(t, m.items[0].Selected)

	// Move down and select second
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = updated.(Model)
	assert.True(t, m.items[1].Selected)

	// Confirm
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.Len(t, m.result.Selected, 2)
	assert.Equal(t, "1", m.result.Selected[0].ID)
	assert.Equal(t, "2", m.result.Selected[1].ID)
}

func TestMultiSelect_ToggleAll(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems(), MultiSelect: true})

	// Select all with *
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("*")})
	m = updated.(Model)
	for _, item := range m.items {
		assert.True(t, item.Selected)
	}

	// Deselect all
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("*")})
	m = updated.(Model)
	for _, item := range m.items {
		assert.False(t, item.Selected)
	}
}

func TestFilter(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	// Enter filter mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	assert.True(t, m.filtering)

	// Type "gam"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(Model)

	assert.Equal(t, "gam", m.filter)
	assert.Len(t, m.items, 1)
	assert.Equal(t, "Gamma", m.items[0].Label)

	// Exit filter mode
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.False(t, m.filtering)
	assert.Len(t, m.items, 1) // filter stays active
}

func TestFilter_Escape(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	// Enter filter and type
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	assert.Len(t, m.items, 0) // no match

	// Escape clears filter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.False(t, m.filtering)
	assert.Equal(t, "", m.filter)
	assert.Len(t, m.items, 5) // all items back
}

func TestAbort_Escape(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	assert.True(t, m.done)
	assert.NotNil(t, cmd)
	assert.True(t, m.result.Aborted)
}

func TestAbort_Q(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems()})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(Model)
	assert.True(t, m.result.Aborted)
}

func TestView_NotEmpty(t *testing.T) {
	m := New(Config{Title: "Test Picker", Items: testItems()})
	m.width = 80
	m.height = 24

	view := m.View()
	assert.Contains(t, view, "Test Picker")
	assert.Contains(t, view, "Alpha")
	assert.Contains(t, view, "Beta")
}

func TestScrolling(t *testing.T) {
	m := New(Config{Title: "Test", Items: testItems(), PageSize: 3})

	// Move to item 4 (should scroll)
	for i := 0; i < 4; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(Model)
	}
	assert.Equal(t, 4, m.cursor)
	assert.Equal(t, 2, m.offset) // scrolled to show cursor
}
