package views

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Theming helpers of the forms of the wizard steps (drop-down styles, label
// focus, arrow keys as Tab), exported for the hybrid steps of cmd/ that embed a
// form in a custom view.

// BuildStepBadge creates a centered rounded badge for use above form steps
// in grouped mode. When compact is false, returns a 5-row badge with vertical
// padding; when true, returns a tight 3-row badge.
func BuildStepBadge(label string, compact bool) *tview.TextView {
	accent := theme.ColorTagOf(theme.ActiveMode.Primary)
	border := theme.ColorTagOf(theme.BorderCard)
	reset := "[-]"

	badgeText := fmt.Sprintf("   ◇  %s   ", label)
	badgeWidth := len([]rune(badgeText))
	top := "╭" + strings.Repeat("─", badgeWidth) + "╮"
	bot := "╰" + strings.Repeat("─", badgeWidth) + "╯"

	var text string
	if compact {
		text = fmt.Sprintf("%s%s%s\n%s│%s%s%s%s│%s\n%s%s%s",
			border, top, reset,
			border, reset, accent, badgeText, border, reset,
			border, bot, reset)
	} else {
		text = fmt.Sprintf("\n%s%s%s\n%s│%s%s%s%s│%s\n%s%s%s\n",
			border, top, reset,
			border, reset, accent, badgeText, border, reset,
			border, bot, reset)
	}

	tv := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(text)
	return tv
}

// NewStyledButtonForm creates a themed form with centered buttons for use
// as the fixed-position button bar in grouped mode.
func NewStyledButtonForm() *tview.Form {
	f := tview.NewForm()
	f.SetBackgroundColor(theme.BgPanel)
	f.SetButtonsAlign(tview.AlignCenter)
	widgets.StyleFormButtons(f, 0) // buttons added later are styled on draw
	f.SetBorder(false)
	f.SetBorderPadding(1, 1, 1, 1) // must match estimateButtonFormHeight assumptions
	return f
}

// remapArrowToTab converts Up/Down arrow keys to Backtab/Tab for inter-field
// navigation in forms. DropDown fields are excluded because they use Down/Up
// to open/navigate their popup list.
// Returns the remapped event, or nil if no remapping was needed.
func remapArrowToTab(form *tview.Form, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyDown && event.Key() != tcell.KeyUp {
		return nil
	}
	itemIdx, _ := form.GetFocusedItemIndex()
	if itemIdx >= 0 && itemIdx < form.GetFormItemCount() {
		if _, isDD := form.GetFormItem(itemIdx).(*tview.DropDown); isDD {
			return nil // let DropDown handle Down/Up natively
		}
	}
	if event.Key() == tcell.KeyDown {
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
}

// isLastFocusableFormItem returns true if itemIdx is the last focusable
// (interactive) item in the form. Non-scrollable TextViews are considered
// non-focusable because tview's form handler skips them during Tab cycling.
// This is used in grouped mode to decide when Tab should jump from the form
// to the external buttonForm.
func isLastFocusableFormItem(form *tview.Form, itemIdx int) bool {
	if form == nil || itemIdx < 0 {
		return false
	}
	count := form.GetFormItemCount()
	if itemIdx >= count {
		return false
	}
	// Check whether any item after itemIdx is focusable (interactive).
	for i := itemIdx + 1; i < count; i++ {
		item := form.GetFormItem(i)
		// tview.TextView form items (added via AddTextView) are not interactive.
		if _, isTV := item.(*tview.TextView); isTV {
			continue
		}
		// Any other item type (InputField, DropDown, Checkbox, etc.) is focusable.
		return false
	}
	return true
}

// fixFormDropDownStyles iterates over all form items and applies the correct
// list popup styles to any DropDown. This is needed because tview.DropDown
// bakes the popup list colors from tview.Styles at construction time, and
// our shell theme produces invisible text (same color as background).
// See ADR-034 for the full analysis.
func fixFormDropDownStyles(form *tview.Form) {
	if form == nil {
		return
	}
	unselected := tcell.StyleDefault.
		Background(theme.BgElement).
		Foreground(theme.FgPrimary)
	selected := tcell.StyleDefault.
		Background(theme.ActiveMode.Primary).
		Foreground(theme.BgPanel)

	for i := 0; i < form.GetFormItemCount(); i++ {
		if dd, ok := form.GetFormItem(i).(*tview.DropDown); ok {
			dd.SetListStyles(unselected, selected)
			// Add inner padding to the dropdown field and popup list items.
			// The suffix " ▼ " provides a visual affordance that the field is
			// a selectable dropdown (not static text).
			dd.SetTextOptions("  ", "  ", "  ", "  ▼ ", "")
		}
	}
}

// isDropDownFocused returns true if the currently focused form item is a DropDown.
func isDropDownFocused(form *tview.Form) bool {
	if form == nil {
		return false
	}
	itemIdx, _ := form.GetFocusedItemIndex()
	if itemIdx < 0 || itemIdx >= form.GetFormItemCount() {
		return false
	}
	_, isDD := form.GetFormItem(itemIdx).(*tview.DropDown)
	return isDD
}

// tviewTagRe matches tview color/style tags like [#hex], [::b], [-:-:-], etc.
var tviewTagRe = regexp.MustCompile(`\[[^\[\]]*\]`)

// stripTviewTags removes all tview color/style tags from a string.
func stripTviewTags(s string) string {
	return tviewTagRe.ReplaceAllString(s, "")
}

// fixFormLabelFocus applies visual hierarchy to form labels based on which
// field currently has focus. The focused field's label is rendered bold with
// the primary text color; all other labels use the secondary text color.
// This creates a clear visual trail guiding the user through the form.
func fixFormLabelFocus(form *tview.Form) {
	if form == nil {
		return
	}
	focusedIdx, _ := form.GetFocusedItemIndex()
	primary := theme.ColorTagOf(theme.FgPrimary)
	secondary := theme.ColorTagOf(theme.FgSecondary)

	for i := 0; i < form.GetFormItemCount(); i++ {
		item := form.GetFormItem(i)
		// Skip TextViews — they are info/hint elements, not interactive fields.
		if _, isTV := item.(*tview.TextView); isTV {
			continue
		}
		raw := stripTviewTags(item.GetLabel())
		if raw == "" {
			continue
		}

		var styled string
		if i == focusedIdx {
			styled = primary + theme.StyleTag(theme.AttrBold) + raw + theme.TagAttrs + theme.TagReset
		} else {
			styled = secondary + raw + theme.TagReset
		}

		// SetLabel is not on the FormItem interface — type-assert to concrete types.
		switch v := item.(type) {
		case *tview.InputField:
			v.SetLabel(styled)
		case *tview.DropDown:
			v.SetLabel(styled)
		case *tview.Checkbox:
			v.SetLabel(styled)
		}
	}
}

// AutoAdvanceFromDropDown moves focus to the next interactive form field
// after a DropDown selection. Call this at the end of your AddDropDown
// callback to get auto-advance behavior without overwriting the callback.
//
// Example:
//
//	form.AddDropDown("Lang", opts, 0, func(text string, idx int) {
//	    selectedLang = text
//	    views.AutoAdvanceFromDropDown(app, form, 0) // 0 = this DD's index
//	})
func AutoAdvanceFromDropDown(app *tview.Application, form *tview.Form, currentIdx int) {
	if app == nil || form == nil {
		return
	}
	// Wrap in a goroutine: QueueUpdateDraw blocks on a done-channel until
	// the event loop processes the update. If called directly from a
	// SetSelectedFunc callback (which runs ON the event loop), this would
	// deadlock. The goroutine moves the blocking <-ch off the event loop.
	safego.Go(func() {
		app.QueueUpdateDraw(func() {
			next := currentIdx + 1
			count := form.GetFormItemCount()
			for next < count {
				if _, isTV := form.GetFormItem(next).(*tview.TextView); !isTV {
					break
				}
				next++
			}
			if next < count {
				form.SetFocus(next)
				app.SetFocus(form)
			}
		})
	})
}

// FixFormDropDownStyles is the exported wrapper for fixFormDropDownStyles.
func FixFormDropDownStyles(form *tview.Form) { fixFormDropDownStyles(form) }

// FixFormLabelFocus is the exported wrapper for fixFormLabelFocus.
func FixFormLabelFocus(form *tview.Form) { fixFormLabelFocus(form) }
