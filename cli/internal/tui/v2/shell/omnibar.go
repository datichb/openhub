package shell

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Omnibar is the unified command input at the bottom of the shell.
// It replaces the menu sidebar, command palette, header, and status bar
// with a single interaction point.
//
// Modes:
//   - Passive: displays contextual hints (view keybindings)
//   - Active: accepts input with fuzzy suggestions above
type Omnibar struct {
	shell       *Shell
	registry    *CommandRegistry
	input       *tview.InputField
	hints       *tview.TextView
	container   *tview.Pages // switches between input and hints
	active      bool
	suggestions *tview.List
	visible     []Command
}

// NewOmnibar creates the omnibar component.
func NewOmnibar(s *Shell, registry *CommandRegistry) *Omnibar {
	o := &Omnibar{
		shell:    s,
		registry: registry,
	}

	// Hints view (passive mode)
	o.hints = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	o.hints.SetBackgroundColor(theme.BgElement)
	o.hints.SetBorderPadding(1, 1, 2, 2)

	// Input field (active mode)
	o.input = tview.NewInputField().
		SetLabel(fmt.Sprintf(" %s%s%s ", theme.ColorTag(theme.ActionHex), theme.IconActive, theme.TagColor)).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder("commande...").
		SetPlaceholderTextColor(theme.FgMuted)
	o.input.SetBackgroundColor(theme.BgElement)
	o.input.SetBorderPadding(1, 1, 2, 2)

	// Wire input change to filter suggestions
	o.input.SetChangedFunc(func(text string) {
		o.updateSuggestions(text)
	})

	// Wire input key handling
	o.input.SetInputCapture(o.handleInputKey)

	// Suggestions list (inserted dynamically into the root layout)
	o.suggestions = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgMuted).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	o.suggestions.SetBackgroundColor(theme.BgPanel)
	o.suggestions.SetBorder(true)
	o.suggestions.SetBorderColor(theme.BorderNormal)
	o.suggestions.SetBorderPadding(0, 0, 1, 1)

	// Click on a suggestion = execute it immediately (standard list UX).
	o.suggestions.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			// Let tview update the highlighted item, then execute on next draw.
			o.shell.app.QueueUpdateDraw(func() {
				o.executeCurrent()
			})
			return action, event
		}
		return action, event
	})

	// Container: BgPanel background creates the visible margin around the bar.
	// BorderPadding provides spacing: 1 row top/bottom, 2 chars left/right.
	// The child pages (hints/input) have BgElement — they appear as a "floating bar".
	o.container = tview.NewPages()
	o.container.SetBackgroundColor(theme.BgPanel)
	o.container.SetBorderPadding(1, 1, 2, 2)
	o.container.AddPage("hints", o.hints, true, true)
	o.container.AddPage("input", o.input, true, false)

	// Click-to-activate: un clic gauche sur la barre passive active l'omnibar.
	o.hints.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick && !o.active {
			o.Activate()
			return action, nil
		}
		return action, event
	})

	return o
}

// Primitive returns the omnibar container for layout integration.
func (o *Omnibar) Primitive() tview.Primitive {
	return o.container
}

// SuggestionsPrimitive returns the suggestions list primitive.
func (o *Omnibar) SuggestionsPrimitive() tview.Primitive {
	return o.suggestions
}

// SuggestionsHeight returns the current desired height of the suggestions list.
func (o *Omnibar) SuggestionsHeight() int {
	count := o.suggestions.GetItemCount()
	// Each item = 2 rows (main + secondary text), plus 2 for border
	height := count*2 + 2
	if height < 4 {
		height = 4
	}
	if height > 22 {
		height = 22
	}
	return height
}

// SetHints updates the passive-mode hint text.
// Displays exactly what the view provides — no automatic additions.
func (o *Omnibar) SetHints(hints string) {
	if hints != "" {
		o.hints.SetText(fmt.Sprintf("%s%s%s",
			theme.ColorTag(theme.TextSecondaryHex), hints, theme.TagColor))
	} else {
		o.hints.SetText("")
	}
}

// IsActive returns whether the omnibar is in input mode.
func (o *Omnibar) IsActive() bool {
	return o.active
}

// Activate switches the omnibar to input mode and focuses it.
func (o *Omnibar) Activate() {
	if o.active {
		return
	}
	o.active = true
	o.input.SetText("")
	o.container.SwitchToPage("input")
	o.updateSuggestions("")
	o.shell.showSuggestionsInLayout()
	o.shell.app.SetFocus(o.input)
}

// ActivateWithRune activates and pre-fills with the given character.
func (o *Omnibar) ActivateWithRune(r rune) {
	if o.active {
		return
	}
	o.active = true
	o.input.SetText(string(r))
	o.container.SwitchToPage("input")
	o.updateSuggestions(string(r))
	o.shell.showSuggestionsInLayout()
	o.shell.app.SetFocus(o.input)
}

// Deactivate switches back to passive mode.
func (o *Omnibar) Deactivate() {
	if !o.active {
		return
	}
	o.active = false
	o.input.SetText("")
	o.container.SwitchToPage("hints")
	o.shell.hideSuggestionsFromLayout()
	o.shell.app.SetFocus(o.shell.content)
}

// HandleKey processes key events when the omnibar is active.
func (o *Omnibar) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (o *Omnibar) handleInputKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		o.Deactivate()
		return nil
	case tcell.KeyEnter:
		o.executeCurrent()
		return nil
	case tcell.KeyDown, tcell.KeyTab:
		current := o.suggestions.GetCurrentItem()
		if current < o.suggestions.GetItemCount()-1 {
			o.suggestions.SetCurrentItem(current + 1)
		}
		return nil
	case tcell.KeyUp, tcell.KeyBacktab:
		current := o.suggestions.GetCurrentItem()
		if current > 0 {
			o.suggestions.SetCurrentItem(current - 1)
		}
		return nil
	}
	return event
}

func (o *Omnibar) updateSuggestions(query string) {
	// 1. Get contextual commands from active view (if it implements CommandProvider)
	var contextual []Command
	if cur := o.shell.router.Current(); cur != nil {
		if cp, ok := cur.(views.CommandProvider); ok {
			for _, cc := range cp.ContextCommands() {
				if MatchesQuery(query, cc.ID, cc.Label, cc.Aliases, cc.Category) {
					contextual = append(contextual, Command{
						ID:          cc.ID,
						Label:       cc.Label,
						Aliases:     cc.Aliases,
						Description: cc.Description,
						Category:    cc.Category,
						Action:      cc.Action,
					})
				}
			}
		}
	}

	// 2. Get global commands
	global := o.registry.Search(query)

	// 3. Merge: contextual first, then global
	merged := make([]Command, 0, len(contextual)+len(global))
	merged = append(merged, contextual...)
	merged = append(merged, global...)

	o.visible = merged
	o.suggestions.Clear()

	// Limit displayed results
	max := 10
	if len(merged) < max {
		max = len(merged)
	}

	for i := 0; i < max; i++ {
		cmd := merged[i]
		desc := ""
		if cmd.Description != "" {
			desc = "  " + cmd.Description
		} else if cmd.Category != "" {
			desc = "  " + cmd.Category
		}
		o.suggestions.AddItem(cmd.Label, desc, 0, nil)
	}

	// Update the layout height if suggestions are visible
	if o.active {
		o.shell.updateSuggestionsHeight()
	}
}

func (o *Omnibar) executeCurrent() {
	idx := o.suggestions.GetCurrentItem()
	if idx < 0 || idx >= len(o.visible) {
		o.Deactivate()
		return
	}

	cmd := o.visible[idx]
	o.Deactivate()

	if cmd.Action != nil {
		cmd.Action()
	} else if cmd.ViewID != "" {
		o.shell.router.NavigateTo(cmd.ViewID)
	}
}
