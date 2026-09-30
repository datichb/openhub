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
// Replaces the duplicated 3-tier adaptive layout blocks scattered across
// init_steps.go, team_helpers.go, and inline_wizard.go.
//
// All vertical spacing uses proportional Flex weights — no hardcoded row
// counts. Horizontal centering uses a Flex wrapper with proportional spacers
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
	// When IntroFlex is true, the intro takes flexible space (for text-only
	// pages like intro/recap). Otherwise, height is derived from content.
	Intro     string
	IntroFlex bool

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

// Proportional weights for the vertical Flex layout.
// These are relative — a weight of 5 gets 5× the space of weight 1.
// When Content is a *tview.Form, its height is computed from field count
// (fixed sizing). The flexSpacer absorbs remaining space and pushes
// buttons to the bottom.
const (
	wPad     = 1 // top padding
	wGap     = 1 // gap after badge
	wContent = 1 // fallback for non-Form content or IntroFlex text
	wPush    = 1 // spacer that pushes buttons to the bottom
	wButtons = 1 // button bar
)

// defaultFormVerticalPadding is the sum of top + bottom border padding
// applied to wizard forms. This must match the values set by
// styleWizardForm() (1,1,2,2 → vertical=2) and the engine Form path
// (1,1,1,1 → vertical=2). If padding values change, update this constant.
const defaultFormVerticalPadding = 2

// minButtonReserve is the minimum number of rows reserved for the
// flexSpacer + buttons zone so buttons are never pushed off-screen.
const minButtonReserve = 3

// BuildWizardPage assembles a wizard page into the given container using
// proportional Flex layout. All spacing is proportional to available terminal
// height — no hardcoded row counts.
//
// The container is expected to be a vertical Flex (tview.FlexRow) that has
// been cleared by the caller (the wizard engine clears stepContent before
// each render).
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

	// ── Decide what chrome fits ──

	fixedH := introH + headerH
	if layout.IntroFlex {
		fixedH = headerH // intro will be flex, not fixed
	}

	// Estimate proportional space consumed by chrome to decide badge visibility.
	// Badge is the first item to drop when space is tight.
	remaining := availH - fixedH
	showBadge := layout.Badge != "" && remaining > 20
	compactBadge := showBadge && remaining <= 28

	// ── Top padding ──
	container.AddItem(newSpacer(bg), 0, wPad, false)

	// ── Badge (fixed, content-derived height) ──
	if showBadge {
		badgeH := 5
		if compactBadge {
			badgeH = 3
		}
		badge := BuildStepBadge(layout.Badge, compactBadge)
		container.AddItem(badge, badgeH, 0, false)
		container.AddItem(newSpacer(bg), 0, wGap, false)
	}

	// ── Intro text ──
	if introTV != nil {
		if layout.IntroFlex {
			container.AddItem(introTV, 0, wContent, false)
		} else {
			// Clamp intro height so content and buttons remain visible.
			maxIntroH := availH - fixedH - 8 // leave room for content + buttons
			if introH > maxIntroH && maxIntroH > 5 {
				introH = maxIntroH
			}
			container.AddItem(introTV, introH, 0, false)
		}
	}

	// ── Section header separator ──
	if headerTV != nil {
		addSectionHeaderToContainer(container, headerTV, bg)
	}

	// ── Compute fixed chrome consumed so far ──
	fixedChrome := 0
	if showBadge {
		if compactBadge {
			fixedChrome += 3
		} else {
			fixedChrome += 5
		}
	}
	if introTV != nil && !layout.IntroFlex {
		fixedChrome += introH
	}
	fixedChrome += headerH

	// ── Content (horizontally centered if ContentMaxWidth > 0) ──
	// When Content is a *tview.Form, compute its natural height from field
	// count and use fixed sizing. This ensures the form gets exactly the
	// space it needs — no more, no less. The flexSpacer absorbs the
	// remaining space and pushes buttons to the bottom.
	// When Content is not a Form (or nil), use proportional sizing.
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

		if form, ok := layout.Content.(*tview.Form); ok {
			contentH := estimateFormHeight(form)
			// Clamp: reserve minimum space for spacer + buttons.
			maxContentH := availH - fixedChrome - minButtonReserve
			if maxContentH < 3 {
				maxContentH = 3 // absolute minimum for at least 1 field
			}
			if contentH > maxContentH {
				contentH = maxContentH // form will scroll internally
			}
			container.AddItem(contentItem, contentH, 0, true)

			// ResizeContent closure for callers that use rebuildForm.
			capturedContainer := container
			capturedItem := contentItem
			result.ResizeContent = func(f *tview.Form) {
				newH := estimateFormHeight(f)
				if newH > maxContentH {
					newH = maxContentH
				}
				capturedContainer.ResizeItem(capturedItem, newH, 0)
			}
		} else {
			// Non-Form content (custom widget): proportional fallback.
			container.AddItem(contentItem, 0, wContent, true)
		}
	}

	// ── Flex spacer — pushes buttons to the bottom ──
	container.AddItem(newSpacer(bg), 0, wPush, false)

	// ── Buttons (anchored at the bottom via flexSpacer above) ──
	if layout.Buttons != nil {
		result.ButtonForm = layout.Buttons
		container.AddItem(layout.Buttons, 0, wButtons, false)
	}

	// No bottomPad — the buttons zone (wButtons=1) provides natural bottom
	// margin since the button renders at the top of its allocated zone.

	// ── Focus ──
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
