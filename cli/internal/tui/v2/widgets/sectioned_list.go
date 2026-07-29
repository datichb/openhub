package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// SectionItem represents one item in a SectionedList.
type SectionItem struct {
	// MainText is the primary display text (label).
	MainText string
	// SecondaryText is the value/description line.
	SecondaryText string
	// IsHeader marks this item as a section header (non-selectable).
	IsHeader bool
	// Locked indicates an enforced/non-editable field (visual indicator only).
	Locked bool
	// Reference is opaque user data for identifying the item.
	Reference interface{}
}

// SectionedList wraps a tview.List with automatic section header skipping.
// Headers are rendered with accent styling and the cursor automatically
// skips over them during j/k navigation.
//
// Features:
//   - Automatic header skip on j/k/Up/Down navigation
//   - Section jump with { and } (previous/next section)
//   - Tab/Shift+Tab also jump between sections
//   - Headers styled with AccentHex box-drawing characters
//   - Locked items show a lock indicator
type SectionedList struct {
	*tview.List
	items     []SectionItem
	app       *tview.Application
	onSelect  func(index int, item SectionItem)
	onChange  func(index int, item SectionItem)
}

// NewSectionedList creates a SectionedList with standard styling.
func NewSectionedList() *SectionedList {
	sl := &SectionedList{
		List: tview.NewList(),
	}
	sl.List.ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	sl.List.SetBackgroundColor(theme.BgPanel)

	sl.List.SetInputCapture(sl.handleInput)
	sl.List.SetChangedFunc(sl.handleChanged)

	return sl
}

// SetApp sets the tview.Application for draw updates.
func (sl *SectionedList) SetApp(app *tview.Application) *SectionedList {
	sl.app = app
	return sl
}

// SetSelectedFunc registers a callback invoked when Enter is pressed on a
// non-header item. The index corresponds to the position in the items slice.
func (sl *SectionedList) SetItemSelectedFunc(fn func(index int, item SectionItem)) *SectionedList {
	sl.onSelect = fn
	sl.List.SetSelectedFunc(func(listIdx int, _ string, _ string, _ rune) {
		if listIdx < 0 || listIdx >= len(sl.items) {
			return
		}
		item := sl.items[listIdx]
		if item.IsHeader {
			return
		}
		if sl.onSelect != nil {
			sl.onSelect(listIdx, item)
		}
	})
	return sl
}

// SetItemChangedFunc registers a callback invoked when the cursor moves to a
// non-header item.
func (sl *SectionedList) SetItemChangedFunc(fn func(index int, item SectionItem)) *SectionedList {
	sl.onChange = fn
	return sl
}

// SetItems replaces all items and rebuilds the list display.
func (sl *SectionedList) SetItems(items []SectionItem) *SectionedList {
	sl.items = items
	sl.rebuild()
	return sl
}

// GetItems returns the current items slice.
func (sl *SectionedList) GetItems() []SectionItem {
	return sl.items
}

// CurrentItem returns the currently selected non-header item and its index.
// Returns -1 and empty item if no selectable item is active.
func (sl *SectionedList) CurrentItem() (int, SectionItem, bool) {
	idx := sl.List.GetCurrentItem()
	if idx < 0 || idx >= len(sl.items) {
		return -1, SectionItem{}, false
	}
	item := sl.items[idx]
	if item.IsHeader {
		return -1, SectionItem{}, false
	}
	return idx, item, true
}

// SelectIndex moves the cursor to the given index, skipping if it's a header.
func (sl *SectionedList) SelectIndex(idx int) {
	if idx < 0 || idx >= len(sl.items) {
		return
	}
	if sl.items[idx].IsHeader {
		// Try to find next selectable
		next := sl.nextSelectable(idx)
		if next >= 0 {
			idx = next
		}
	}
	sl.List.SetCurrentItem(idx)
}

// rebuild clears and re-adds all items to the underlying tview.List.
func (sl *SectionedList) rebuild() {
	sl.List.Clear()
	for _, item := range sl.items {
		main, secondary := sl.formatItem(item)
		sl.List.AddItem(main, secondary, 0, nil)
	}
	// Ensure cursor is on a selectable item
	current := sl.List.GetCurrentItem()
	if current >= 0 && current < len(sl.items) && sl.items[current].IsHeader {
		next := sl.nextSelectable(current)
		if next >= 0 {
			sl.List.SetCurrentItem(next)
		}
	}
}

// formatItem returns the styled main and secondary text for a list item.
func (sl *SectionedList) formatItem(item SectionItem) (string, string) {
	if item.IsHeader {
		return fmt.Sprintf("  %s─── %s ──────────────────%s",
			theme.ColorTag(theme.AccentHex), item.MainText, theme.TagColor), ""
	}
	prefix := "  "
	if item.Locked {
		prefix = fmt.Sprintf("  %s\U0001F512%s ", theme.ColorTag(theme.WarningHex), theme.TagColor)
	}
	return prefix + item.MainText, "    " + item.SecondaryText
}

// handleInput captures key events for header skipping and section jumps.
func (sl *SectionedList) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'j':
			sl.moveDown()
			return nil
		case 'k':
			sl.moveUp()
			return nil
		case '{':
			sl.jumpPrevSection()
			return nil
		case '}':
			sl.jumpNextSection()
			return nil
		}
	case tcell.KeyDown:
		sl.moveDown()
		return nil
	case tcell.KeyUp:
		sl.moveUp()
		return nil
	case tcell.KeyTab:
		sl.jumpNextSection()
		return nil
	case tcell.KeyBacktab:
		sl.jumpPrevSection()
		return nil
	}
	return event
}

// handleChanged is called by tview when the current item changes.
func (sl *SectionedList) handleChanged(index int, _ string, _ string, _ rune) {
	if index < 0 || index >= len(sl.items) {
		return
	}
	item := sl.items[index]
	if item.IsHeader {
		return
	}
	if sl.onChange != nil {
		sl.onChange(index, item)
	}
}

// moveDown moves cursor down, skipping headers.
func (sl *SectionedList) moveDown() {
	current := sl.List.GetCurrentItem()
	next := sl.nextSelectable(current)
	if next >= 0 {
		sl.List.SetCurrentItem(next)
	}
}

// moveUp moves cursor up, skipping headers.
func (sl *SectionedList) moveUp() {
	current := sl.List.GetCurrentItem()
	prev := sl.prevSelectable(current)
	if prev >= 0 {
		sl.List.SetCurrentItem(prev)
	}
}

// nextSelectable finds the next non-header item after the given index.
func (sl *SectionedList) nextSelectable(from int) int {
	for i := from + 1; i < len(sl.items); i++ {
		if !sl.items[i].IsHeader {
			return i
		}
	}
	return -1
}

// prevSelectable finds the previous non-header item before the given index.
func (sl *SectionedList) prevSelectable(from int) int {
	for i := from - 1; i >= 0; i-- {
		if !sl.items[i].IsHeader {
			return i
		}
	}
	return -1
}

// jumpNextSection moves to the first selectable item of the next section.
func (sl *SectionedList) jumpNextSection() {
	current := sl.List.GetCurrentItem()
	// Find the next header after current
	for i := current + 1; i < len(sl.items); i++ {
		if sl.items[i].IsHeader {
			// Found next section header, jump to first item after it
			next := sl.nextSelectable(i)
			if next >= 0 {
				sl.List.SetCurrentItem(next)
				return
			}
		}
	}
}

// jumpPrevSection moves to the first selectable item of the previous section.
func (sl *SectionedList) jumpPrevSection() {
	current := sl.List.GetCurrentItem()
	// Find current section's header
	currentHeader := -1
	for i := current - 1; i >= 0; i-- {
		if sl.items[i].IsHeader {
			currentHeader = i
			break
		}
	}
	if currentHeader < 0 {
		return // already in first section
	}
	// Find the header before that
	for i := currentHeader - 1; i >= 0; i-- {
		if sl.items[i].IsHeader {
			next := sl.nextSelectable(i)
			if next >= 0 {
				sl.List.SetCurrentItem(next)
				return
			}
		}
	}
	// No previous header found — jump to first selectable
	first := sl.nextSelectable(-1)
	if first >= 0 {
		sl.List.SetCurrentItem(first)
	}
}
