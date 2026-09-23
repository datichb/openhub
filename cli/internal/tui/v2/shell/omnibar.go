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
// Layout (bottom of screen):
//
//	┌─┬───────────────────────────────────────────┐
//	│▎│  hints or input (pages container)          │  ← omnibar
//	│▎│  🏠 Hub                    openhub v1.x    │  ← mode bar
//	└─┴───────────────────────────────────────────┘
//	 gutter (1 col)          rightPane (flex)
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
	modeBar     *tview.TextView
	gutter      *tview.Box
	gutterFg    tcell.Color // foreground color for the gutter indicator, updated per mode
	wrapper     *tview.Flex // outermost primitive: gutter | rightPane
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
	o.hints.SetBorderPadding(1, 0, 1, 2)

	// Input field (active mode)
	o.input = tview.NewInputField().
		SetLabel(fmt.Sprintf(" %s%s%s ", theme.ColorTag(theme.ActionHex), theme.IconActive, theme.TagColor)).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder(i18n.T("tui.omnibar.placeholder")).
		SetPlaceholderTextColor(theme.FgMuted)
	o.input.SetBackgroundColor(theme.BgElement)
	o.input.SetBorderPadding(1, 0, 1, 2)

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

	// Defense-in-depth: if the suggestions list accidentally receives focus
	// (e.g. after a toast AddPage/RemovePage steals focus from the input
	// field), ensure Enter/Escape/arrow keys still behave correctly instead
	// of being silently swallowed by tview's default List handler.
	o.suggestions.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			o.executeCurrent()
			return nil
		case tcell.KeyEscape:
			o.Deactivate()
			return nil
		case tcell.KeyDown, tcell.KeyTab:
			current := o.suggestions.GetCurrentItem()
			if current < len(o.visible)-1 {
				o.suggestions.SetCurrentItem(current + 1)
			}
			return nil
		case tcell.KeyUp, tcell.KeyBacktab:
			current := o.suggestions.GetCurrentItem()
			if current > 0 {
				o.suggestions.SetCurrentItem(current - 1)
			}
			return nil
		case tcell.KeyRune:
			// Forward printable runes back to the input field so the user
			// can keep typing even if focus drifted to the suggestions list.
			o.shell.app.SetFocus(o.input)
			return event
		}
		return event
	})

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

	// Container: pages switching between hints and input.
	o.container = tview.NewPages()
	o.container.SetBackgroundColor(theme.BgElement)
	o.container.AddPage("hints", o.hints, true, true)
	o.container.AddPage("input", o.input, true, false)

	// Mode bar: single-line bar below the omnibar showing mode + context info.
	o.modeBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	o.modeBar.SetBackgroundColor(theme.BgElement)
	o.modeBar.SetBorderPadding(0, 0, 1, 2)

	// Gutter: 1-column colored indicator on the left edge spanning the full height.
	// Uses a DrawFunc to paint the "▎" character (1/8 block) on every row with
	// the active mode color as foreground. The background matches BgPanel so
	// only the thin vertical stroke is visible — not a solid color block.
	o.gutterFg = theme.ModeHubColor
	o.gutter = tview.NewBox()
	o.gutter.SetBackgroundColor(theme.BgPanel)
	o.gutter.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Background(theme.BgPanel).Foreground(o.gutterFg)
		for dy := 0; dy < height; dy++ {
			screen.SetContent(x, y+dy, '▎', nil, style)
		}
		return x, y, width, height
	})

	// Right pane: vertical stack of omnibar container (flex=1) + mode bar (1 row).
	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
	rightPane.SetBackgroundColor(theme.BgPanel)
	rightPane.AddItem(o.container, 0, 1, true) // omnibar fills available space
	rightPane.AddItem(o.modeBar, 1, 0, false)  // mode bar: 1 row at bottom

	// Wrapper: horizontal layout — gutter (1 col) | rightPane (fills rest).
	// BgPanel background creates the visible margin around the bar.
	// Top/bottom padding provides vertical spacing.
	o.wrapper = tview.NewFlex().SetDirection(tview.FlexColumn)
	o.wrapper.SetBackgroundColor(theme.BgPanel)
	o.wrapper.SetBorderPadding(1, 1, 1, 0)
	o.wrapper.AddItem(o.gutter, 1, 0, false)   // gutter: fixed 1 col
	o.wrapper.AddItem(rightPane, 0, 1, true)    // right pane: fills rest

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

// Primitive returns the outermost omnibar primitive (wrapper) for layout integration.
func (o *Omnibar) Primitive() tview.Primitive {
	return o.wrapper
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

// UpdateModeBar updates the mode bar content and gutter color based on the
// active navigation mode and the contextual information provided by the view.
func (o *Omnibar) UpdateModeBar(mode views.Mode, info views.ModeBarInfo) {
	// Resolve the color hex for this mode.
	colorHex := theme.ModeHubHex
	gutterColor := theme.ModeHubColor
	switch mode {
	case views.ModeTeam:
		colorHex = theme.ModeTeamHex
		gutterColor = theme.ModeTeamColor
	case views.ModeProject:
		colorHex = theme.ModeProjectHex
		gutterColor = theme.ModeProjectColor
	}

	// Update gutter foreground color — the DrawFunc reads o.gutterFg on each render.
	o.gutterFg = gutterColor

	// Build mode bar text: icon + label on the left, context info right-aligned.
	left := fmt.Sprintf("%s%s %s%s",
		theme.ColorTag(colorHex), info.Icon,
		theme.ColorTag(theme.TextPrimaryHex), info.Label)

	if info.Right != "" {
		// Right-aligned info is muted to avoid visual competition with the label.
		right := fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), info.Right, theme.TagColor)
		o.modeBar.SetText(left + right)
	} else {
		o.modeBar.SetText(left)
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
	// Block activation when the current view captures all input (e.g. inline wizard).
	if cur := o.shell.router.Current(); cur != nil {
		if ic, ok := cur.(views.InputCapturing); ok && ic.CapturesInput() {
			return
		}
	}
	o.active = true
	// Suppress ChangedFunc during SetText to avoid a redundant
	// updateSuggestions call — we call it explicitly right after.
	o.input.SetChangedFunc(nil)
	o.input.SetText("")
	o.input.SetChangedFunc(func(text string) {
		o.updateSuggestions(text)
	})
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
	// Block activation when the current view captures all input (e.g. inline wizard).
	if cur := o.shell.router.Current(); cur != nil {
		if ic, ok := cur.(views.InputCapturing); ok && ic.CapturesInput() {
			return
		}
	}
	o.active = true
	// Suppress ChangedFunc during SetText to avoid a redundant
	// updateSuggestions call — we call it explicitly right after.
	o.input.SetChangedFunc(nil)
	o.input.SetText(string(r))
	o.input.SetChangedFunc(func(text string) {
		o.updateSuggestions(text)
	})
	o.container.SwitchToPage("input")
	o.updateSuggestions(string(r))
	o.shell.showSuggestionsInLayout()
	o.shell.app.SetFocus(o.input)
}

// RestoreFocus re-focuses the omnibar input field if the omnibar is active.
// This must be called after any operation that may steal focus (e.g. Pages
// AddPage/RemovePage in toast lifecycle) to ensure keyboard events still
// reach the omnibar's input capture handler.
func (o *Omnibar) RestoreFocus() {
	if o.active {
		o.shell.app.SetFocus(o.input)
	}
}

// Deactivate switches back to passive mode.
func (o *Omnibar) Deactivate() {
	if !o.active {
		return
	}
	o.active = false
	// Suppress the ChangedFunc callback while clearing the text — otherwise
	// SetText("") triggers updateSuggestions("") re-entrantly, rebuilding
	// the full suggestion list right before we hide the overlay.
	o.input.SetChangedFunc(nil)
	o.input.SetText("")
	o.input.SetChangedFunc(func(text string) {
		o.updateSuggestions(text)
	})
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
		// Bound navigation to executable commands only — skip the overflow
		// indicator row ("...et N autres") which has no backing command.
		if current < len(o.visible)-1 {
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

	// 2. Get global commands (filtered by active mode — ADR-032 Phase 3)
	global := o.registry.Search(query, o.shell.activeMode)

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
		o.suggestions.AddItem(fmt.Sprintf("  %s%s%s", muted, i18n.T("tui.omnibar.no_result"), reset), "", 0, nil)
		if o.active {
			o.shell.updateSuggestionsHeight()
		}
		return
	}

	// Limit displayed results — keep it small to avoid covering too much content
	maxVisible := 7
	if len(merged) < maxVisible {
		maxVisible = len(merged)
	}

	// Truncate visible to match displayed items — prevents executeCurrent()
	// from accidentally executing a hidden command via the overflow item.
	o.visible = merged[:maxVisible]

	for i := 0; i < maxVisible; i++ {
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
		o.suggestions.AddItem(fmt.Sprintf("  %s%s%s", muted, i18n.Tf("tui.omnibar.more_results", len(merged)-7), reset), "", 0, nil)
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
