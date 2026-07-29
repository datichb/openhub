package widgets

import (
	"testing"
)

func TestSectionedList_NextSelectable(t *testing.T) {
	sl := NewSectionedList()
	sl.items = []SectionItem{
		{MainText: "Section A", IsHeader: true},
		{MainText: "item1"},
		{MainText: "item2"},
		{MainText: "Section B", IsHeader: true},
		{MainText: "item3"},
	}

	tests := []struct {
		from     int
		expected int
	}{
		{-1, 1},  // before all → first selectable
		{0, 1},   // header → next item
		{1, 2},   // item → next item
		{2, 4},   // item → skip header to item3
		{3, 4},   // header → item3
		{4, -1},  // last item → no more
	}

	for _, tt := range tests {
		got := sl.nextSelectable(tt.from)
		if got != tt.expected {
			t.Errorf("nextSelectable(%d) = %d, want %d", tt.from, got, tt.expected)
		}
	}
}

func TestSectionedList_PrevSelectable(t *testing.T) {
	sl := NewSectionedList()
	sl.items = []SectionItem{
		{MainText: "Section A", IsHeader: true},
		{MainText: "item1"},
		{MainText: "item2"},
		{MainText: "Section B", IsHeader: true},
		{MainText: "item3"},
	}

	tests := []struct {
		from     int
		expected int
	}{
		{0, -1},  // header, nothing before
		{1, -1},  // first item, nothing before
		{2, 1},   // item2 → item1
		{3, 2},   // header → item2
		{4, 2},   // item3 → skip header → item2
		{5, 4},   // past end → item3
	}

	for _, tt := range tests {
		got := sl.prevSelectable(tt.from)
		if got != tt.expected {
			t.Errorf("prevSelectable(%d) = %d, want %d", tt.from, got, tt.expected)
		}
	}
}

func TestSectionedList_CurrentItem_Header(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems([]SectionItem{
		{MainText: "Section", IsHeader: true},
		{MainText: "item1", Reference: "ref1"},
	})

	// After SetItems, cursor should auto-skip to first selectable
	idx, item, ok := sl.CurrentItem()
	if !ok {
		t.Fatal("expected a selectable item to be current")
	}
	if idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if item.MainText != "item1" {
		t.Errorf("expected item1, got %s", item.MainText)
	}
	if item.Reference != "ref1" {
		t.Error("expected reference to be preserved")
	}
}

func TestSectionedList_CurrentItem_Empty(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems([]SectionItem{
		{MainText: "Section", IsHeader: true},
	})

	_, _, ok := sl.CurrentItem()
	if ok {
		t.Error("expected no selectable item when only headers exist")
	}
}

func TestSectionedList_LockedItem(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems([]SectionItem{
		{MainText: "Section", IsHeader: true},
		{MainText: "locked_field", Locked: true, SecondaryText: "enforced"},
		{MainText: "normal_field", SecondaryText: "editable"},
	})

	// Locked items are still selectable (locked is visual only)
	sl.SelectIndex(1)
	idx, item, ok := sl.CurrentItem()
	if !ok || idx != 1 {
		t.Fatal("locked item should be selectable")
	}
	if !item.Locked {
		t.Error("expected item.Locked to be true")
	}
}

func TestSectionedList_SelectIndex_SkipsHeader(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems([]SectionItem{
		{MainText: "Section A", IsHeader: true},
		{MainText: "item1"},
		{MainText: "Section B", IsHeader: true},
		{MainText: "item2"},
	})

	// Trying to select a header should skip to next selectable
	sl.SelectIndex(2) // header B
	idx, _, ok := sl.CurrentItem()
	if !ok {
		t.Fatal("expected selectable item")
	}
	if idx != 3 {
		t.Errorf("expected skip to index 3, got %d", idx)
	}
}

func TestSectionedList_FormatItem_Header(t *testing.T) {
	sl := NewSectionedList()
	item := SectionItem{MainText: "General", IsHeader: true}
	main, secondary := sl.formatItem(item)

	if secondary != "" {
		t.Errorf("header secondary should be empty, got %q", secondary)
	}
	// Should contain the section name
	if len(main) == 0 {
		t.Error("header main text should not be empty")
	}
	// Should contain accent color tag
	if !containsSubstring(main, "General") {
		t.Error("header should contain section name")
	}
}

func TestSectionedList_FormatItem_Regular(t *testing.T) {
	sl := NewSectionedList()
	item := SectionItem{MainText: "language", SecondaryText: "fr"}
	main, secondary := sl.formatItem(item)

	if !containsSubstring(main, "language") {
		t.Errorf("expected main to contain 'language', got %q", main)
	}
	if !containsSubstring(secondary, "fr") {
		t.Errorf("expected secondary to contain 'fr', got %q", secondary)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
