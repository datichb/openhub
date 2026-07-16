package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Picker — full-screen interactive selection (single or multi-select)
// ─────────────────────────────────────────────────────────────────────────────

// PickerItem represents a selectable item in the picker.
type PickerItem struct {
	ID          string
	Label       string
	Description string
	Category    string
	Selected    bool
}

// PickerConfig configures the picker behavior.
type PickerConfig struct {
	Layout      layout.Config
	Items       []PickerItem
	MultiSelect bool
}

// PickerResult is the output after user interaction.
type PickerResult struct {
	Selected []PickerItem
	Aborted  bool
}

// RunPicker launches a full-screen interactive picker.
func RunPicker(cfg PickerConfig) (PickerResult, error) {
	if len(cfg.Items) == 0 {
		return PickerResult{Aborted: true}, nil
	}

	var result PickerResult

	cfg.Layout.OnQuit = func() {
		result.Aborted = true
	}

	shell := layout.Build(cfg.Layout)

	// State
	items := make([]PickerItem, len(cfg.Items))
	copy(items, cfg.Items)
	allItems := make([]PickerItem, len(cfg.Items))
	copy(allItems, cfg.Items)
	filter := ""
	filtering := false

	// ── Build the list widget ──
	list := tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetMainTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)

	// ── Filter input ──
	filterInput := tview.NewInputField().
		SetLabel("/ ").
		SetLabelColor(theme.Accent).
		SetFieldBackgroundColor(theme.BgPanel).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder("type to filter...").
		SetPlaceholderTextColor(theme.FgMuted)
	filterInput.SetBackgroundColor(theme.BgPanel)

	// ── Populate list ──
	populateList := func(items []PickerItem) {
		list.Clear()
		for _, item := range items {
			prefix := ""
			if cfg.MultiSelect {
				if item.Selected {
					prefix = fmt.Sprintf("[%s]✓[-] ", widgets.ColorTag(theme.Success))
				} else {
					prefix = fmt.Sprintf("[%s]○[-] ", widgets.ColorTag(theme.FgMuted))
				}
			}
			desc := item.Description
			if item.Category != "" && desc == "" {
				desc = item.Category
			} else if item.Category != "" {
				desc = item.Category + " · " + desc
			}
			list.AddItem(prefix+item.Label, "  "+desc, 0, nil)
		}
	}
	populateList(items)

	// ── Filter logic ──
	applyFilter := func(f string) []PickerItem {
		return filterPickerItems(allItems, f)
	}

	// ── Insert into shell content ──
	shell.Content.AddItem(filterInput, 1, 0, false)
	shell.Content.AddItem(list, 0, 1, true)

	// ── Input handling ──
	filterInput.SetChangedFunc(func(text string) {
		filter = text
		items = applyFilter(filter)
		populateList(items)
	})
	filterInput.SetDoneFunc(func(key tcell.Key) {
		filtering = false
		shell.StatusBar.SetHints(pickerHints(cfg.MultiSelect, false))
		shell.App.SetFocus(list)
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape || event.Rune() == 'q':
			if filtering {
				filtering = false
				filter = ""
				filterInput.SetText("")
				items = applyFilter("")
				populateList(items)
				shell.StatusBar.SetHints(pickerHints(cfg.MultiSelect, false))
				shell.App.SetFocus(list)
				return nil
			}
			result = PickerResult{Aborted: true}
			shell.App.Stop()
			return nil

		case event.Rune() == '/':
			filtering = true
			filterInput.SetText("")
			shell.StatusBar.SetHints("enter confirm filter · esc cancel filter · ctrl+c quit")
			shell.App.SetFocus(filterInput)
			return nil

		case event.Rune() == ' ' && cfg.MultiSelect:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(items) {
				items[idx].Selected = !items[idx].Selected
				for i := range allItems {
					if allItems[i].ID == items[idx].ID {
						allItems[i].Selected = items[idx].Selected
						break
					}
				}
				populateList(items)
				list.SetCurrentItem(idx)
			}
			return nil

		case event.Rune() == '*' && cfg.MultiSelect:
			allSelected := true
			for _, item := range items {
				if !item.Selected {
					allSelected = false
					break
				}
			}
			for i := range items {
				items[i].Selected = !allSelected
				for j := range allItems {
					if allItems[j].ID == items[i].ID {
						allItems[j].Selected = items[i].Selected
						break
					}
				}
			}
			populateList(items)
			return nil

		case event.Key() == tcell.KeyEnter:
			if cfg.MultiSelect {
				var selected []PickerItem
				for _, item := range allItems {
					if item.Selected {
						selected = append(selected, item)
					}
				}
				result = PickerResult{Selected: selected}
			} else {
				idx := list.GetCurrentItem()
				if idx >= 0 && idx < len(items) {
					result = PickerResult{Selected: []PickerItem{items[idx]}}
				} else {
					result = PickerResult{Aborted: true}
				}
			}
			shell.App.Stop()
			return nil
		}
		return event
	})

	// ── Run ──
	if err := shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run(); err != nil {
		return PickerResult{Aborted: true}, err
	}

	return result, nil
}

func pickerHints(multiSelect, filtering bool) string {
	if filtering {
		return "enter confirm · esc cancel"
	}
	if multiSelect {
		return "↑↓ navigate · space toggle · * toggle all · / filter · enter confirm · ctrl+c quit"
	}
	return "↑↓ navigate · / filter · enter select · ctrl+c quit"
}

// filterPickerItems returns items matching the filter string (case-insensitive).
// An empty filter returns a copy of all items.
func filterPickerItems(allItems []PickerItem, filter string) []PickerItem {
	if filter == "" {
		out := make([]PickerItem, len(allItems))
		copy(out, allItems)
		return out
	}
	lower := strings.ToLower(filter)
	var filtered []PickerItem
	for _, item := range allItems {
		if strings.Contains(strings.ToLower(item.Label), lower) ||
			strings.Contains(strings.ToLower(item.Description), lower) ||
			strings.Contains(strings.ToLower(item.Category), lower) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
