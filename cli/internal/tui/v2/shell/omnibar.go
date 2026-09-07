package shell

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
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
		SetPlaceholder(i18n.T("tui.omnibar.placeholder")).
		SetPlaceholderTextColor(theme.FgMuted)
	o.input.SetBackgroundColor(theme.BgElement)
	o.input.SetBorderPadding(1, 1, 2, 2)

	// Wire input change to filter suggestions
	o.input.SetChangedFunc(func(text string) {
		o.updateSuggestions(text)
	})

	// Wire input key handling
	o.input.SetInputCapture(o.handleInputKey)

	// Suggestions list (displayed as floating overlay when omnibar is active)
	o.suggestions = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	o.suggestions.SetBackgroundColor(theme.BgPanel)
	o.suggestions.SetBorder(true)
	o.suggestions.SetBorderColor(theme.BorderNormal)
	o.suggestions.SetBorderPadding(0, 0, 1, 1)

	// Click on a suggestion = execute it immediately (standard list UX).
	//
	// IMPORTANT: do NOT use QueueUpdateDraw here — this handler runs on the
	// tview event loop. QueueUpdateDraw blocks waiting for the event loop to
	// drain its update queue, which it never will because we're currently
	// inside it → deadlock. Call executeCurrent() directly instead.
	// Also guard with InRect: the suggestions-overlay page receives ALL mouse
	// clicks when visible (Pages dispatches back-to-front without coord check),
	// so we must ignore clicks outside our bounds.
	o.suggestions.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			x, y := event.Position()
			sx, sy, sw, sh := o.suggestions.GetRect()
			if x < sx || x >= sx+sw || y < sy || y >= sy+sh {
				// Click is outside the suggestions list — do not intercept.
				return action, event
			}
			// Execute directly on the event loop (no QueueUpdateDraw).
			o.executeCurrent()
			return tview.MouseConsumed, nil
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

	// Click-to-activate: a left click on the passive hint bar activates the omnibar.
	// Return MouseConsumed so tview marks the event as consumed and triggers a
	// redraw — without this the omnibar switches internally but the screen is
	// not refreshed, leaving the user with no visual feedback.
	o.hints.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick && !o.active {
			o.Activate()
			return tview.MouseConsumed, nil
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

// SuggestionsList returns the suggestions list as a typed *tview.List
// so callers can call SetRect for manual positioning.
func (o *Omnibar) SuggestionsList() *tview.List {
	return o.suggestions
}

// SuggestionsHeight returns the current desired height of the suggestions list.
func (o *Omnibar) SuggestionsHeight() int {
	count := o.suggestions.GetItemCount()
	// Each item = 1 row (no secondary text), plus 2 for border
	height := count + 2
	if height < 3 {
		height = 3
	}
	if height > 11 {
		height = 11
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
					RunsDirect:  cc.RunsDirect,
				})
				}
			}
		}
	}

	// 2. Get global commands
	global := o.registry.Search(query)

	// 3. Merge: contextual first, then global — deduplicate by ViewID (ADR-032)
	merged := make([]Command, 0, len(contextual)+len(global))
	merged = append(merged, contextual...)

	// Build a set of ViewIDs already covered by contextual commands to avoid
	// showing duplicate entries for the same destination.
	seen := make(map[string]bool, len(contextual))
	for _, c := range contextual {
		if c.ViewID != "" {
			seen[c.ViewID] = true
		}
	}
	for _, g := range global {
		if g.ViewID != "" && seen[g.ViewID] {
			continue // skip global duplicate — contextual version takes priority
		}
		merged = append(merged, g)
	}

	o.visible = merged
	o.suggestions.Clear()

	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	if len(merged) == 0 && query != "" {
		o.suggestions.AddItem(fmt.Sprintf("  %sAucun résultat%s", muted, reset), "", 0, nil)
		if o.active {
			o.shell.updateSuggestionsHeight()
		}
		return
	}

	// Limit displayed results — keep it small to avoid covering too much content
	max := 7
	if len(merged) < max {
		max = len(merged)
	}

	// Truncate visible to match displayed items — prevents executeCurrent()
	// from accidentally executing a hidden command via the overflow item.
	o.visible = merged[:max]

	for i := 0; i < max; i++ {
		cmd := merged[i]
		// Single-line format: label padded to 18 chars + dimmed description
		// This gives clean column alignment regardless of label length.
		desc := cmd.Description
		if desc == "" {
			desc = cmd.Category
		}
		aliasHint := ""
		if len(cmd.Aliases) > 0 {
			n := len(cmd.Aliases)
			if n > 2 {
				n = 2
			}
			aliasHint = fmt.Sprintf(" %s(%s)%s", muted, strings.Join(cmd.Aliases[:n], ", "), reset)
		}
		text := fmt.Sprintf("%-18s%s  %s%s%s", cmd.Label, aliasHint, muted, desc, reset)
		o.suggestions.AddItem(text, "", 0, nil)
	}

	if len(merged) > 7 {
		o.suggestions.AddItem(fmt.Sprintf("  %s...et %d autres%s", muted, len(merged)-7, reset), "", 0, nil)
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
	o.registry.RecordUsage(cmd.ID)
	o.Deactivate()

	if cmd.Action != nil {
		if cmd.RunsDirect {
			// RunsDirect=true: action calls SuspendAndExec which must run directly
			// on the event loop — it cannot be deferred via QueueUpdateDraw because
			// app.Suspend() would deadlock waiting for the event loop to be idle.
			cmd.Action()
		} else {
			// Spawn a goroutine that calls QueueUpdateDraw — the canonical tview
			// pattern for scheduling UI work from inside an event-loop handler.
			//
			// WHY NOT call QueueUpdateDraw directly:
			//   QueueUpdateDraw / QueueUpdate BLOCK the caller via an unbuffered
			//   done-channel. Calling it from inside a handler deadlocks: the event
			//   loop can't drain updates while it's still executing the current
			//   handler, so the <-ch wait never resolves.
			//
			// WHY the goroutine works:
			//   The goroutine blocks on <-ch outside the event loop. The current
			//   handler returns immediately → the event loop finishes the draw cycle
			//   → drains updates → executes cmd.Action() → signals done → goroutine
			//   exits. No leak, no race.
			go func() {
				o.shell.app.QueueUpdateDraw(func() {
					cmd.Action()
				})
			}()
		}
	} else if cmd.ViewID != "" {
		// NavigateTo only manipulates the widget tree (no pages.AddPage) — safe to
		// call synchronously from within the event loop.
		o.shell.router.NavigateTo(cmd.ViewID)
	}
}
