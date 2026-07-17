// Package widgets provides reusable tview primitives for the TUI shell.
package widgets

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// FilterItem represents a single item in a filterable list.
type FilterItem struct {
	// MainText is the primary display text.
	MainText string
	// SecondaryText is the secondary line (optional).
	SecondaryText string
	// Reference is an opaque user data field.
	Reference interface{}
}

// FilterableList is a tview.List with an integrated filter input.
// Activate the filter with '/' and dismiss with Esc.
type FilterableList struct {
	*tview.Flex
	list        *tview.List
	input       *tview.InputField
	allItems    []FilterItem
	visible     []FilterItem
	filtering   bool
	onSelect    func(item FilterItem)
	onCancel    func()
}

// NewFilterableList creates a filterable list with the given items.
func NewFilterableList(items []FilterItem, onSelect func(FilterItem)) *FilterableList {
	fl := &FilterableList{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		allItems: items,
		visible:  items,
		onSelect: onSelect,
	}

	fl.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	fl.list.SetBackgroundColor(theme.BgPanel)

	fl.input = tview.NewInputField().
		SetLabel(" / ").
		SetLabelColor(theme.Action).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder("filtrer...").
		SetPlaceholderTextColor(theme.FgMuted)
	fl.input.SetBackgroundColor(theme.BgElement)

	fl.populateList(items)
	fl.AddItem(fl.list, 0, 1, true)

	fl.setupKeys()
	return fl
}

// SetOnCancel sets the callback for when filtering is cancelled.
func (fl *FilterableList) SetOnCancel(fn func()) {
	fl.onCancel = fn
}

// ActivateFilter shows the filter input and focuses it.
func (fl *FilterableList) ActivateFilter(app *tview.Application) {
	if fl.filtering {
		return
	}
	fl.filtering = true
	fl.input.SetText("")
	fl.AddItem(fl.input, 1, 0, true)
	app.SetFocus(fl.input)

	fl.input.SetChangedFunc(func(text string) {
		fl.filter(text)
	})

	fl.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			fl.deactivateFilter(app)
			return nil
		case tcell.KeyEnter:
			fl.deactivateFilter(app)
			fl.selectCurrent()
			return nil
		case tcell.KeyDown:
			cur := fl.list.GetCurrentItem()
			if cur < fl.list.GetItemCount()-1 {
				fl.list.SetCurrentItem(cur + 1)
			}
			return nil
		case tcell.KeyUp:
			cur := fl.list.GetCurrentItem()
			if cur > 0 {
				fl.list.SetCurrentItem(cur - 1)
			}
			return nil
		}
		return event
	})
}

func (fl *FilterableList) deactivateFilter(app *tview.Application) {
	fl.filtering = false
	fl.RemoveItem(fl.input)
	fl.visible = fl.allItems
	fl.populateList(fl.allItems)
	app.SetFocus(fl.list)
}

func (fl *FilterableList) setupKeys() {
	fl.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == '/' {
			// Need app reference — handled via caller
			return event
		}
		if event.Key() == tcell.KeyEnter {
			fl.selectCurrent()
			return nil
		}
		return event
	})
}

func (fl *FilterableList) selectCurrent() {
	idx := fl.list.GetCurrentItem()
	if idx >= 0 && idx < len(fl.visible) && fl.onSelect != nil {
		fl.onSelect(fl.visible[idx])
	}
}

func (fl *FilterableList) filter(query string) {
	if query == "" {
		fl.visible = fl.allItems
		fl.populateList(fl.allItems)
		return
	}

	query = strings.ToLower(query)
	var filtered []FilterItem
	for _, item := range fl.allItems {
		main := strings.ToLower(item.MainText)
		sec := strings.ToLower(item.SecondaryText)
		if strings.Contains(main, query) || strings.Contains(sec, query) {
			filtered = append(filtered, item)
		}
	}
	fl.visible = filtered
	fl.populateList(filtered)
}

func (fl *FilterableList) populateList(items []FilterItem) {
	fl.list.Clear()
	for _, item := range items {
		fl.list.AddItem(item.MainText, item.SecondaryText, 0, nil)
	}
}
