package shell

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/menu"
)

// PaletteEntry represents a single entry in the command palette.
type PaletteEntry struct {
	// Label is the display text.
	Label string
	// Category is the group (e.g., "Sessions", "Projets").
	Category string
	// Action is executed when the entry is selected.
	Action func()
}

// commandPalette is the Ctrl+P fuzzy-search overlay.
type commandPalette struct {
	shell   *Shell
	entries []PaletteEntry
	visible []PaletteEntry
	input   *tview.InputField
	list    *tview.List
	frame   *tview.Flex
}

// ShowCommandPalette displays the Ctrl+P command palette overlay.
func (s *Shell) ShowCommandPalette() {
	entries := s.buildPaletteEntries()
	cp := &commandPalette{
		shell:   s,
		entries: entries,
		visible: entries,
	}
	cp.build()
	cp.show()
}

func (s *Shell) buildPaletteEntries() []PaletteEntry {
	var entries []PaletteEntry
	s.collectMenuEntries(s.menuItems, "", &entries)
	return entries
}

func (s *Shell) collectMenuEntries(items []*menu.MenuItem, prefix string, entries *[]PaletteEntry) {
	for _, item := range items {
		if item.IsCategory() {
			s.collectMenuEntries(item.Children, item.Label, entries)
		} else {
			entry := PaletteEntry{
				Label:    item.Label,
				Category: prefix,
			}
			// Capture item for closure
			capturedItem := item
			if capturedItem.Action != nil {
				entry.Action = capturedItem.Action
			} else if capturedItem.ViewID != "" {
				viewID := capturedItem.ViewID
				entry.Action = func() {
					s.router.NavigateTo(viewID)
				}
			}
			*entries = append(*entries, entry)
		}
	}
}

func (cp *commandPalette) build() {
	// Input field
	cp.input = tview.NewInputField().
		SetLabel(" " + theme.IconArrow + " ").
		SetLabelColor(theme.Action).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder("Rechercher une commande...").
		SetPlaceholderTextColor(theme.FgMuted)
	cp.input.SetBackgroundColor(theme.BgElement)

	// Results list
	cp.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	cp.list.SetBackgroundColor(theme.BgPanel)

	cp.populateList(cp.entries)

	// Wire input change to fuzzy filter
	cp.input.SetChangedFunc(func(text string) {
		cp.filter(text)
	})

	// Wire input key handling
	cp.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			cp.dismiss()
			return nil
		case tcell.KeyEnter:
			cp.selectCurrent()
			return nil
		case tcell.KeyDown, tcell.KeyTab:
			// Move selection down in the list
			current := cp.list.GetCurrentItem()
			if current < cp.list.GetItemCount()-1 {
				cp.list.SetCurrentItem(current + 1)
			}
			return nil
		case tcell.KeyUp, tcell.KeyBacktab:
			current := cp.list.GetCurrentItem()
			if current > 0 {
				cp.list.SetCurrentItem(current - 1)
			}
			return nil
		}
		return event
	})

	// Build frame
	cp.frame = tview.NewFlex().SetDirection(tview.FlexRow)
	cp.frame.AddItem(cp.input, 1, 0, true)
	cp.frame.AddItem(cp.list, 0, 1, false)
	cp.frame.SetBackgroundColor(theme.BgPanel)
	cp.frame.SetBorder(true)
	cp.frame.SetBorderColor(theme.Accent)
	cp.frame.SetTitle(" Commandes · Esc fermer ")
	cp.frame.SetTitleColor(theme.Accent)
}

func (cp *commandPalette) show() {
	cp.shell.overlayActive = true
	cp.shell.app.EnableMouse(false)

	// Use a Grid to center the palette (60% width, 60% height)
	grid := tview.NewGrid().
		SetColumns(0, -3, 0).
		SetRows(3, -3, 0)
	grid.AddItem(cp.frame, 1, 1, 1, 1, 0, 0, true)

	cp.shell.pages.AddPage("palette", grid, true, true)
	cp.shell.app.SetFocus(cp.input)
}

func (cp *commandPalette) dismiss() {
	cp.shell.pages.RemovePage("palette")
	cp.shell.overlayActive = false
	cp.shell.app.EnableMouse(true)
	cp.shell.app.SetFocus(cp.shell.content)
}

func (cp *commandPalette) selectCurrent() {
	idx := cp.list.GetCurrentItem()
	if idx < 0 || idx >= len(cp.visible) {
		cp.dismiss()
		return
	}
	entry := cp.visible[idx]
	cp.dismiss()
	if entry.Action != nil {
		entry.Action()
	}
}

func (cp *commandPalette) filter(query string) {
	if query == "" {
		cp.visible = cp.entries
		cp.populateList(cp.entries)
		return
	}

	query = strings.ToLower(query)
	var filtered []PaletteEntry
	for _, e := range cp.entries {
		label := strings.ToLower(e.Label)
		cat := strings.ToLower(e.Category)
		if fuzzyMatch(query, label) || fuzzyMatch(query, cat) || strings.Contains(label, query) || strings.Contains(cat, query) {
			filtered = append(filtered, e)
		}
	}
	cp.visible = filtered
	cp.populateList(filtered)
}

func (cp *commandPalette) populateList(entries []PaletteEntry) {
	cp.list.Clear()
	for _, e := range entries {
		secondary := ""
		if e.Category != "" {
			secondary = "  " + e.Category
		}
		cp.list.AddItem(e.Label, secondary, 0, nil)
	}
}

// fuzzyMatch checks if all chars in pattern appear in str in order.
func fuzzyMatch(pattern, str string) bool {
	pi := 0
	for si := 0; si < len(str) && pi < len(pattern); si++ {
		if str[si] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}
