package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSectionedList(t *testing.T) {
	t.Run("next and previous selectable skip the headers", func(t *testing.T) {
		sl := NewSectionedList()
		sl.items = []SectionItem{
			{MainText: "Section A", IsHeader: true},
			{MainText: "item1"},
			{MainText: "item2"},
			{MainText: "Section B", IsHeader: true},
			{MainText: "item3"},
		}
		tests := []struct {
			name       string
			from       int
			next, prev int
		}{
			{"before all", -1, 1, -1},
			{"first header", 0, 1, -1},
			{"first item", 1, 2, -1},
			{"second item", 2, 4, 1},
			{"second header", 3, 4, 2},
			{"last item", 4, -1, 2},
			{"past the end", 5, -1, 4},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.next, sl.nextSelectable(tc.from), "nextSelectable(%d)", tc.from)
				assert.Equal(t, tc.prev, sl.prevSelectable(tc.from), "prevSelectable(%d)", tc.from)
			})
		}
	})

	t.Run("the cursor lands on the first selectable item", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems([]SectionItem{
			{MainText: "Section", IsHeader: true},
			{MainText: "item1", Reference: "ref1"},
		})
		idx, item, ok := sl.CurrentItem()
		require.True(t, ok, "a selectable item is current")
		assert.Equal(t, 1, idx, "index")
		assert.Equal(t, "item1", item.MainText, "item")
		assert.Equal(t, "ref1", item.Reference, "reference preserved")
	})

	t.Run("only headers: nothing is current", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems([]SectionItem{{MainText: "Section", IsHeader: true}})
		_, _, ok := sl.CurrentItem()
		assert.False(t, ok, "no selectable item")
	})

	t.Run("a locked item is still selectable", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems([]SectionItem{
			{MainText: "Section", IsHeader: true},
			{MainText: "locked_field", Locked: true, SecondaryText: "enforced"},
			{MainText: "normal_field", SecondaryText: "editable"},
		})
		sl.SelectIndex(1)
		idx, item, ok := sl.CurrentItem()
		require.True(t, ok, "locked item selectable")
		assert.Equal(t, 1, idx, "index")
		assert.True(t, item.Locked, "locked flag kept")
	})

	t.Run("selecting a header goes to the next item", func(t *testing.T) {
		sl := NewSectionedList()
		sl.SetItems([]SectionItem{
			{MainText: "Section A", IsHeader: true},
			{MainText: "item1"},
			{MainText: "Section B", IsHeader: true},
			{MainText: "item2"},
		})
		sl.SelectIndex(2)
		idx, _, ok := sl.CurrentItem()
		require.True(t, ok, "selectable item")
		assert.Equal(t, 3, idx, "skipped to the item after the header")
	})

	t.Run("formatting", func(t *testing.T) {
		sl := NewSectionedList()
		main, secondary := sl.formatItem(SectionItem{MainText: "General", IsHeader: true})
		assert.Contains(t, main, "General", "header shows the section name")
		assert.Empty(t, secondary, "header has no secondary text")

		main, secondary = sl.formatItem(SectionItem{MainText: "language", SecondaryText: "fr"})
		assert.Contains(t, main, "language", "main text")
		assert.Contains(t, secondary, "fr", "secondary text")
	})
}
