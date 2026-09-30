package views

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Wizard Page Layout — unified layout builder for all wizard step pages.
//
// Every item has a FIXED height derived from its content, except for a single
// flexSpacer that absorbs all remaining space. This produces a layout where
// all content is compact at the top and buttons are anchored at the bottom:
//
//	[badge]        fixed (5 or 3, content-derived)
//	[gap]          fixed 1 (only if badge present)
//	[intro]        fixed (line count + padding)
//	[header]       fixed 3 (gap + separator + gap)
//	[content]      fixed (estimateFormHeight)
//	flexSpacer     proportional — SOLE flex item, absorbs all free space
//	buttons        fixed (estimateButtonFormHeight)
//
// Horizontal centering uses a Flex wrapper with proportional spacers
// instead of SetDrawFunc (which silently breaks SetBorderPadding in tview).
// ─────────────────────────────────────────────────────────────────────────────

// WizardPageLayout describes the content slots for a wizard step page.
// Nil/zero fields are omitted from the layout.
type WizardPageLayout struct {
	// Badge is the label shown in a bordered box at the top.
	// Empty string = no badge.
	Badge string

	// Intro is raw text (with tview color tags) for the intro area.
	// When non-empty, a centered TextView is created with auto-padding.
	// Height is derived from line count.
	Intro string

	// SectionTitle is a header separator between intro and content.
	// Format: ╶─── Title ───╴ (centered, accent title, muted dashes).
	// Empty string = no separator.
	SectionTitle string

	// Content is the main interactive area (form, list, custom widget).
	// Horizontally centered at ContentMaxWidth if > 0.
	// nil = no content area (button-only pages).
	Content         tview.Primitive
	ContentMaxWidth int // 0 = full width (no centering wrapper)

	// Buttons is the bottom button bar (typically from NewStyledButtonForm).
	// nil = no button bar.
	Buttons *tview.Form

	// FocusTarget is the primitive to focus initially.
	// nil = no auto-focus.
	FocusTarget tview.Primitive
}

// BuildWizardPageResult holds references to layout components that callers
// may need after assembly (e.g., for focus management or dynamic resizing).
type BuildWizardPageResult struct {
	// ButtonForm is the button bar primitive, if Buttons were provided.
	ButtonForm *tview.Form

	// ResizeContent updates the content area height after a rebuildForm
	// (form.Clear + repopulate). Call this whenever the form's item count
	// changes dynamically. Not needed for Rerender (full page rebuild) or
	// for static forms.
	// nil when Content is nil or not a *tview.Form.
	ResizeContent func(form *tview.Form)
}

// defaultFormVerticalPadding is the sum of top + bottom border padding
// applied to wizard forms. This must match the values set by
// styleWizardForm() (1,1,2,2 → vertical=2) and the engine Form path
// (1,1,1,1 → vertical=2). If padding values change, update this constant.
const defaultFormVerticalPadding = 2

// BuildWizardPage assembles a wizard page into the given container.
// All items use fixed heights derived from their content. The only
// proportional item is the flexSpacer between content and buttons,
// which absorbs all remaining space — content stays at the top,
// buttons anchor at the bottom.
func BuildWizardPage(app *tview.Application, container *tview.Flex, layout WizardPageLayout) BuildWizardPageResult {
	bg := theme.BgPanel

	_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
	if termH <= 0 {
		termH = 50
	}
	availH := termH - 7 // omnibar(5) + border(1) + hints(1)

	// ── Compute content-derived heights ──

	introH := 0
	var introTV *tview.TextView
	if layout.Intro != "" {
		introTV = tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetScrollable(true)
		introTV.SetBackgroundColor(bg)
		introTV.SetBorderPadding(1, 0, 2, 2)
		introTV.SetText(layout.Intro)
		introH = strings.Count(layout.Intro, "\n") + 2 // lines + top border padding
	}

	headerH := 0
	var headerTV *tview.TextView
	if layout.SectionTitle != "" {
		headerTV = BuildSectionHeader(layout.Intro, layout.SectionTitle)
		headerH = 3 // gap above + line + gap below
	}

	contentH := 0
	if form, ok := layout.Content.(*tview.Form); ok {
		contentH = estimateFormHeight(form)
	}

	buttonH := 0
	if layout.Buttons != nil {
		buttonH = estimateButtonFormHeight(layout.Buttons)
	}

	// ── Decide what chrome fits ──

	badgeH := 0
	gapH := 0
	showBadge := false
	if layout.Badge != "" {
		// Badge + gap = 6 normal or 4 compact. Show only if enough room.
		totalFixed := introH + headerH + contentH + buttonH
		spare := availH - totalFixed
		if spare > 12 {
			badgeH = 5
			gapH = 1
			showBadge = true
		} else if spare > 8 {
			badgeH = 3
			gapH = 1
			showBadge = true
		}
	}

	// ── Clamp intro and content for small terminals ──
	// Reserve: badge + gap + header + content + spacer(1 min) + buttons.
	fixedAboveIntro := badgeH + gapH
	fixedBelowIntro := headerH + contentH + 1 + buttonH // 1 = min spacer
	maxIntroH := availH - fixedAboveIntro - fixedBelowIntro
	if introH > maxIntroH && maxIntroH > 3 {
		introH = maxIntroH
	}

	fixedAboveContent := badgeH + gapH + introH + headerH
	maxContentH := availH - fixedAboveContent - 1 - buttonH // 1 = min spacer
	if maxContentH < 3 {
		maxContentH = 3
	}
	if contentH > maxContentH {
		contentH = maxContentH // form will scroll internally
	}

	// ── Assemble layout: all fixed except the spacer ──

	// Breathing room at the top of the page.
	container.AddItem(newSpacer(bg), 1, 0, false)

	// Badge
	if showBadge {
		compact := badgeH == 3
		badge := BuildStepBadge(layout.Badge, compact)
		container.AddItem(badge, badgeH, 0, false)
		container.AddItem(newSpacer(bg), gapH, 0, false)
	}

	// Intro text (fixed height)
	if introTV != nil {
		container.AddItem(introTV, introH, 0, false)
	}

	// Section header separator
	if headerTV != nil {
		addSectionHeaderToContainer(container, headerTV, bg)
	}

	// Content (fixed height, horizontally centered if requested)
	result := BuildWizardPageResult{}
	if layout.Content != nil {
		var contentItem tview.Primitive
		if layout.ContentMaxWidth > 0 {
			if form, ok := layout.Content.(*tview.Form); ok {
				contentItem = CenteredForm(form, layout.ContentMaxWidth)
			} else {
				contentItem = CenteredPrimitive(layout.Content, layout.ContentMaxWidth)
			}
		} else {
			contentItem = layout.Content
		}

		container.AddItem(contentItem, contentH, 0, true)

		// ResizeContent closure for callers that use rebuildForm.
		if _, ok := layout.Content.(*tview.Form); ok {
			capturedContainer := container
			capturedItem := contentItem
			capturedMaxH := maxContentH
			result.ResizeContent = func(f *tview.Form) {
				newH := estimateFormHeight(f)
				if newH > capturedMaxH {
					newH = capturedMaxH
				}
				capturedContainer.ResizeItem(capturedItem, newH, 0)
			}
		}
	}

	// Flex spacer — SOLE proportional item, absorbs all remaining space.
	// Pushes buttons to the bottom of the page.
	container.AddItem(newSpacer(bg), 0, 1, false)

	// Buttons (fixed height, anchored at bottom)
	if layout.Buttons != nil {
		result.ButtonForm = layout.Buttons
		container.AddItem(layout.Buttons, buttonH, 0, false)
	}

	// Focus
	if layout.FocusTarget != nil && app != nil {
		app.SetFocus(layout.FocusTarget)
	}

	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Form height estimation
// ─────────────────────────────────────────────────────────────────────────────

// estimateFormHeight computes the natural height of a tview.Form based on
// its items' GetFieldHeight() values. This mirrors the layout algorithm in
// tview.Form.Draw() (form.go:539-600):
//
//	totalH = paddingTop + paddingBottom + sum(fieldH_i) + itemPadding * (n-1)
//
// where itemPadding defaults to 1 and fieldH_i is each item's GetFieldHeight()
// (with a fallback to 5 if <= 0, matching DefaultFormFieldHeight).
func estimateFormHeight(form *tview.Form) int {
	n := form.GetFormItemCount()
	if n == 0 {
		return defaultFormVerticalPadding
	}
	h := defaultFormVerticalPadding
	for i := 0; i < n; i++ {
		fh := form.GetFormItem(i).GetFieldHeight()
		if fh <= 0 {
			fh = 5 // tview.DefaultFormFieldHeight fallback
		}
		h += fh
		if i < n-1 {
			h += 1 // itemPadding (default = 1)
		}
	}
	return h
}

// estimateButtonFormHeight computes the natural height of a button-only form
// (NewStyledButtonForm). This mirrors tview.Form.Draw()'s button layout:
// padding(top=1) + gap(1, "empty line after items") + buttons(1 row) + padding(bottom=1).
func estimateButtonFormHeight(form *tview.Form) int {
	if form.GetButtonCount() == 0 {
		return 0
	}
	// default padding (1,1,1,1) → top(1) + bottom(1) = 2
	// + 1 gap line (form.Draw always inserts an empty line before buttons)
	// + 1 button row
	return defaultFormVerticalPadding + 1 + 1
}

// ─────────────────────────────────────────────────────────────────────────────
// Centering helpers
// ─────────────────────────────────────────────────────────────────────────────

// CenteredForm wraps a form in a horizontal Flex that centers it at the given
// max width. Unlike SetDrawFunc centering, this preserves SetBorderPadding on
// the form — the form can use padding normally.
//
// The returned Flex can be used in place of the form in any vertical layout.
// Focus is delegated to the form automatically (the form is the only
// focus=true child).
func CenteredForm(form *tview.Form, maxWidth int) *tview.Flex {
	bg := theme.BgPanel
	wrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	wrapper.SetBackgroundColor(bg)
	wrapper.AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)
	wrapper.AddItem(form, maxWidth, 0, true)
	wrapper.AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)
	return wrapper
}

// CenteredPrimitive wraps any tview.Primitive in a horizontal Flex for
// centering, analogous to CenteredForm but for non-Form primitives.
func CenteredPrimitive(p tview.Primitive, maxWidth int) *tview.Flex {
	bg := theme.BgPanel
	wrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	wrapper.SetBackgroundColor(bg)
	wrapper.AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)
	wrapper.AddItem(p, maxWidth, 0, true)
	wrapper.AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)
	return wrapper
}

// ─────────────────────────────────────────────────────────────────────────────
// Section header
// ─────────────────────────────────────────────────────────────────────────────

// BuildSectionHeader creates a ╶─── Title ───╴ separator centered at the
// width of the intro text. The intro text is used to compute the max visible
// line width.
func BuildSectionHeader(introText, title string) *tview.TextView {
	accent := theme.ColorTag(theme.ActiveMode.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	maxTextWidth := maxVisibleWidth(introText)

	titleLen := utf8.RuneCountInString(title)
	leftDashes := 3
	rightDashes := maxTextWidth - titleLen - leftDashes - 5 // 5 = "╶" + "─"*left + " " + " " + "╴"
	if rightDashes < 3 {
		rightDashes = 3
	}

	headerText := fmt.Sprintf("%s╶%s %s%s%s %s%s╴%s",
		muted, strings.Repeat("─", leftDashes), accent, title, reset,
		muted, strings.Repeat("─", rightDashes), reset)

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(headerText)
	return tv
}

// addSectionHeaderToContainer inserts a header with gap above and below.
func addSectionHeaderToContainer(container *tview.Flex, header *tview.TextView, bg tcell.Color) {
	gapAbove := tview.NewBox()
	gapAbove.SetBackgroundColor(bg)
	container.AddItem(gapAbove, 1, 0, false)
	container.AddItem(header, 1, 0, false)
	gapBelow := tview.NewBox()
	gapBelow.SetBackgroundColor(bg)
	container.AddItem(gapBelow, 1, 0, false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Utilities
// ─────────────────────────────────────────────────────────────────────────────

// newSpacer creates a transparent spacer box with the given background color.
func newSpacer(bg tcell.Color) *tview.Box {
	b := tview.NewBox()
	b.SetBackgroundColor(bg)
	return b
}

// maxVisibleWidth returns the width of the widest visible line in text
// (accounting for tview color tags).
func maxVisibleWidth(text string) int {
	maxW := 0
	for _, line := range strings.Split(text, "\n") {
		if w := tview.TaggedStringWidth(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// MaxVisibleWidth is the exported version of maxVisibleWidth.
func MaxVisibleWidth(text string) int {
	return maxVisibleWidth(text)
}
