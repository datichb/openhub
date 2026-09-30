package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// initWizardState — shared mutable state for all init wizard step builders
// ─────────────────────────────────────────────────────────────────────────────
//
// This struct replaces the 22+ closure-captured variables that previously lived
// inside buildFirstRunInlineWizard. All step builders receive a pointer to this
// struct and read/write fields through it. The semantics are identical to the
// closure pattern (pointer capture), but the state is explicit and testable.

type initStepState struct {
	// ── Setup mode (selected on welcome screen) ──
	// "solo"  = skip team group entirely
	// "team"  = skip provider (inherited from team config)
	// "full"  = show all steps (default)
	SetupMode string

	// ── Language ──
	SelectedLang string
	LangIdx      int

	// ── Provider ──
	SelectedProvider string
	ProviderIdx      int
	AuthMode         string
	AuthIdx          int
	Token            string
	Region           string
	ProfileName      string
	RegionIdx        int
	CustomRegion     bool
	HasKeychainToken bool
	ProviderSkipped  bool

	// ── Project ──
	ProjectName     string
	ProjectPath     string
	ProjectID       string
	ProjectCreated  bool
	ProjectSkipped  bool
	DeployConfirmed bool

	// ── MCP ──
	FigmaToken   string
	GitlabToken  string
	GitlabWrite  bool
	GslidesToken string
	MCPSkipped   bool

	// ── Deploy ──
	SelectedAgents []string
	DeploySkipped  bool

	// ── Shared refs ──
	TeamState *initWizardTeamState
	AppPtr    **app.App    // double pointer: team/deploy steps reload the app
	FocusBtn  *bool        // pointer shared with InlineWizardConfig.FocusButtonAfterRender
	Steps     *[]views.WizardStep // pointer to the assembled steps slice (for Rerender lookup)

	// ── Step index caches (resolved after assembly) ──
	LangStepIdx     int
	ProviderStepIdx int

	// ── Provider options (immutable after init) ──
	ProviderOptions []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Layout helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildFormStepLayout builds the standard CustomView layout for steps that have
// an intro text block above interactive form fields. The layout is:
//
//	topSpacer → tv (intro) → [header separator] → form → flexSpacer → buttonForm → bottomSpacer
//
// The intro text is displayed centered (AlignCenter) at its natural height.
// When sectionTitle is non-empty, a visual header separator is inserted between
// the intro and the form:  ╶─── Title ─────────────────────╴
// The form is centered horizontally at the same width as the intro text and
// takes remaining flexible space. A flexible spacer between form and button
// keeps the button anchored at the bottom.
//
// Parameters:
//   - tvApp: the tview Application
//   - container: the Flex container (stepContent) to add items to
//   - introText: the formatted intro text with tview color tags
//   - sectionTitle: title for the header separator (empty = no header)
//   - form: the interactive form (DropDowns, InputFields, etc.)
//   - buttonForm: the button bar (Valider, Skip, etc.)
//   - focusTarget: the primitive to focus initially (usually form)
func buildFormStepLayout(
	tvApp *tview.Application,
	container *tview.Flex,
	introText string,
	sectionTitle string,
	form *tview.Form,
	buttonForm *tview.Form,
	focusTarget tview.Primitive,
) {
	_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
	if termH <= 0 {
		termH = 50
	}
	availH := termH - 7

	// Intro text view — centered, fixed height computed from content.
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(true)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetBorderPadding(1, 0, 2, 2)
	tv.SetText(introText)
	tvHeight := strings.Count(introText, "\n") + 2 // lines + top border padding

	// Compute the width of the widest visible line in the intro text.
	// Used to center the form and build the header separator at the same width.
	maxTextWidth := 0
	for _, line := range strings.Split(introText, "\n") {
		w := tview.TaggedStringWidth(line)
		if w > maxTextWidth {
			maxTextWidth = w
		}
	}

	// Center the form horizontally at the same width as the intro text.
	formWidth := maxTextWidth
	form.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		fw := formWidth
		if fw > width {
			fw = width
		}
		if fw < width {
			pad := (width - fw) / 2
			return x + pad, y, fw, height
		}
		return x, y, width, height
	})
	form.SetBackgroundColor(theme.BgPanel)
	form.SetFieldBackgroundColor(theme.BgElement)
	form.SetFieldTextColor(theme.FgPrimary)
	form.SetLabelColor(theme.FgPrimary)
	form.SetBorder(false)
	form.SetBorderPadding(1, 1, 2, 2)

	// Build the section header separator if a title is provided.
	// Format: ╶─── Title ─────────────────────╴ (centered, accent title, muted dashes)
	var headerView *tview.TextView
	headerFixedH := 0
	if sectionTitle != "" {
		accent := theme.ColorTag(theme.ActiveMode.AccentHex)
		muted := theme.ColorTag(theme.TextMutedHex)
		reset := theme.TagColor

		titleLen := utf8.RuneCountInString(sectionTitle)
		leftDashes := 3
		rightDashes := maxTextWidth - titleLen - leftDashes - 5 // 5 = len("╶") + "─"*left + " " + " " + "╴"
		if rightDashes < 3 {
			rightDashes = 3
		}

		headerText := fmt.Sprintf("%s╶%s %s%s%s %s%s╴%s",
			muted, strings.Repeat("─", leftDashes), accent, sectionTitle, reset,
			muted, strings.Repeat("─", rightDashes), reset)

		headerView = tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		headerView.SetBackgroundColor(theme.BgPanel)
		headerView.SetText(headerText)
		headerFixedH = 3 // 1 line gap above + 1 line header + 1 line gap below
	}

	// Navigation: ↑/↓ remap to Backtab/Tab, Tab on last form item → buttonForm, ↑ on buttonForm → form.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if remapped := views.RemapArrowToTab(form, event); remapped != nil {
			return remapped
		}
		if event.Key() == tcell.KeyTab {
			itemIdx, _ := form.GetFocusedItemIndex()
			if views.IsLastFocusableFormItem(form, itemIdx) {
				tvApp.SetFocus(buttonForm)
				return nil
			}
		}
		return event
	})
	buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyRight:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		case tcell.KeyBacktab, tcell.KeyUp:
			tvApp.SetFocus(form)
			return nil
		}
		return event
	})

	// addHeader inserts gap + header separator + gap into the container.
	addHeader := func() {
		if headerView != nil {
			gapAbove := tview.NewBox()
			gapAbove.SetBackgroundColor(theme.BgPanel)
			container.AddItem(gapAbove, 1, 0, false)
			container.AddItem(headerView, 1, 0, false)
			gapBelow := tview.NewBox()
			gapBelow.SetBackgroundColor(theme.BgPanel)
			container.AddItem(gapBelow, 1, 0, false)
		}
	}

	// Adaptive layout: same thresholds as engine/intro (35/20/minimal).
	// The form is the sole flexible item — it absorbs all remaining space.
	// Its fields render at the top; the empty area below acts as a natural
	// spacer between the form content and the button bar (same pattern as
	// the welcome step).
	if availH >= 35 {
		topSpacer := tview.NewBox()
		topSpacer.SetBackgroundColor(theme.BgPanel)
		bottomSpacer := tview.NewBox()
		bottomSpacer.SetBackgroundColor(theme.BgPanel)

		// Clamp tvHeight so button is always visible.
		maxTvH := availH - 3 - headerFixedH - 5 - 3 // top + header + btn + bottom
		if tvHeight > maxTvH && maxTvH > 5 {
			tvHeight = maxTvH
		}

		container.AddItem(topSpacer, 3, 0, false)
		container.AddItem(tv, tvHeight, 0, false)
		addHeader()
		container.AddItem(form, 0, 1, true)
		container.AddItem(buttonForm, 5, 0, false)
		container.AddItem(bottomSpacer, 3, 0, false)
	} else if availH >= 20 {
		topSpacer := tview.NewBox()
		topSpacer.SetBackgroundColor(theme.BgPanel)

		maxTvH := availH - 1 - headerFixedH - 3 // top + header + btn
		if tvHeight > maxTvH && maxTvH > 5 {
			tvHeight = maxTvH
		}

		container.AddItem(topSpacer, 1, 0, false)
		container.AddItem(tv, tvHeight, 0, false)
		addHeader()
		container.AddItem(form, 0, 1, true)
		container.AddItem(buttonForm, 3, 0, false)
	} else {
		container.AddItem(tv, 0, 1, false)
		container.AddItem(form, 0, 2, true)
		container.AddItem(buttonForm, 3, 0, false)
	}
	tvApp.SetFocus(focusTarget)
}

// ─────────────────────────────────────────────────────────────────────────────
// Step builders
// ─────────────────────────────────────────────────────────────────────────────

// infoColor wraps a value string in a tview color tag for the sidebar InfoFields.
func infoSuccess(v string) string { return theme.ColorTag(theme.SuccessHex) + v + theme.TagReset }
func infoMuted(v string) string   { return theme.ColorTag(theme.TextMutedHex) + v + theme.TagReset }
func infoWarning(v string) string { return theme.ColorTag(theme.WarningHex) + v + theme.TagReset }

// buildWelcomeStep creates the welcome/splash screen step.
func buildWelcomeStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:            "welcome",
		Label:         i18n.T("cmd.init.wizard_step_welcome"),
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			// Detect terminal height for adaptive layout.
			_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
			if termH <= 0 {
				termH = 50
			}
			availH := termH - 7 // shell overhead (omnibar + border + hints)

			// Build welcome text — full (with ASCII art) or compact (title only).
			tv := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter).
				SetScrollable(true)
			tv.SetBackgroundColor(theme.BgPanel)
			tv.SetBorderPadding(1, 0, 2, 2)

			var tvText string
			if availH >= 35 {
				// Full layout: ASCII art banner
				tvText = fmt.Sprintf(`%s██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s
%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s
%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s
%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s
%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s
%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s

%s`+i18n.T("cmd.init.wizard_welcome_title")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_desc")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_detail")+`%s

%s1.%s `+i18n.T("cmd.init.wizard_step_lang_desc")+`
%s2.%s `+i18n.T("cmd.init.wizard_step_provider_desc")+`
%s3.%s `+i18n.T("cmd.init.wizard_step_team_desc_welcome")+`
%s4.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
%s5.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`

%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s

%s`+i18n.T("cmd.init.wizard_checklist_title")+`%s
%s☐%s `+i18n.T("cmd.init.wizard_checklist_provider")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_team")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_mcp")+`
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
					accent, reset,
					muted, reset,
					secondary, reset,
				accent, reset,
				accent, reset,
				accent, reset,
			)
		} else {
			// Compact layout: no ASCII art, title line only
			tvText = fmt.Sprintf(`%s`+i18n.T("cmd.init.wizard_welcome_title_compact")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_desc")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_detail")+`%s

%s1.%s `+i18n.T("cmd.init.wizard_step_lang_desc")+`
%s2.%s `+i18n.T("cmd.init.wizard_step_provider_desc")+`
%s3.%s `+i18n.T("cmd.init.wizard_step_team_desc_welcome")+`
%s4.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
%s5.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`

%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s

%s`+i18n.T("cmd.init.wizard_checklist_title")+`%s
%s☐%s `+i18n.T("cmd.init.wizard_checklist_provider")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_team")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_mcp")+`
`,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
					accent, reset,
					muted, reset,
					secondary, reset,
				accent, reset,
				accent, reset,
				accent, reset,
			)
		}
		tv.SetText(tvText)

		// Compute the fixed height for the tv based on its actual content.
		// This avoids the tv being either too large (pushing modeForm to the
		// bottom) or too small (truncating text) when sharing flex space.
		tvHeight := strings.Count(tvText, "\n") + 2 // text lines + top border padding

		// Compute the width of the longest visible line in the tv text.
		// This is used to size the mode selector block so it visually
		// aligns with the centered text above.
		maxTextWidth := 0
		for _, line := range strings.Split(tvText, "\n") {
			w := tview.TaggedStringWidth(line)
			if w > maxTextWidth {
				maxTextWidth = w
			}
		}

		// ── Setup mode selector ──
			modeOptions := []string{
				i18n.T("cmd.init.wizard_mode_solo"),
				i18n.T("cmd.init.wizard_mode_team"),
				i18n.T("cmd.init.wizard_mode_full"),
			}
			modeKeys := []string{"solo", "team", "full"}
			defaultMode := 2 // "full" by default — all steps visible
			if s.SetupMode == "solo" {
				defaultMode = 0
			} else if s.SetupMode == "team" {
				defaultMode = 1
			}
			if s.SetupMode == "" {
				s.SetupMode = "full"
			}

			modeSelect := widgets.NewInlineSelect(
				i18n.T("cmd.init.wizard_mode_label"),
				modeOptions,
				defaultMode,
				func(_ string, idx int) {
					if idx >= 0 && idx < len(modeKeys) {
						s.SetupMode = modeKeys[idx]
					}
				},
			)
			modeSelect.SetCentered(true).SetContentWidth(maxTextWidth)

			// Show descriptions only when there's enough vertical space.
			// With multi-line descriptions (2 lines each): 3 options × (1 label + 2 desc)
			// + 2 blank separators = 11 field lines. Plus label row + padding = 13.
			modeFormHeight := 1 + len(modeOptions) + 1 // label + options + padding
			if availH >= 30 {
				modeSelect.SetDescriptions([]string{
					i18n.T("cmd.init.wizard_mode_solo_desc"),
					i18n.T("cmd.init.wizard_mode_team_desc"),
					i18n.T("cmd.init.wizard_mode_full_desc"),
				})
				modeFormHeight = 1 + modeSelect.GetFieldHeight() + 1
			}

			modeForm := tview.NewForm()
			modeForm.SetBackgroundColor(theme.BgPanel)
			modeForm.SetLabelColor(theme.FgPrimary)
			modeForm.SetFieldBackgroundColor(theme.BgPanel)
			modeForm.SetFieldTextColor(theme.FgPrimary)
			modeForm.SetBorder(false)
			modeForm.AddFormItem(modeSelect)

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_welcome_start")+"  ", onDone)

			// Tab / ↓ from modeForm → buttonForm
			lastModeIdx := len(modeOptions) - 1
			modeForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyTab:
					tvApp.SetFocus(buttonForm)
					return nil
				case tcell.KeyDown:
					if modeSelect.GetCursorIndex() == lastModeIdx {
						tvApp.SetFocus(buttonForm)
						return nil
					}
				}
				return event
			})

			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				case tcell.KeyBacktab, tcell.KeyUp:
					tvApp.SetFocus(modeForm)
					return nil
				}
				return event
			})

			// ── Adaptive layout ──
			// Build section header for the mode selector.
			sectionTitle := i18n.T("cmd.init.wizard_section_mode")
			titleLen := utf8.RuneCountInString(sectionTitle)
			leftDashes := 3
			rightDashes := maxTextWidth - titleLen - leftDashes - 5
			if rightDashes < 3 {
				rightDashes = 3
			}
			headerText := fmt.Sprintf("%s╶%s %s%s%s %s%s╴%s",
				muted, strings.Repeat("─", leftDashes), accent, sectionTitle, reset,
				muted, strings.Repeat("─", rightDashes), reset)
			modeHeader := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter)
			modeHeader.SetBackgroundColor(theme.BgPanel)
			modeHeader.SetText(headerText)

			const headerFixedH = 3 // gap above + header + gap below

			// addModeHeader inserts gap + header + gap into the container.
			addModeHeader := func() {
				gapAbove := tview.NewBox()
				gapAbove.SetBackgroundColor(theme.BgPanel)
				container.AddItem(gapAbove, 1, 0, false)
				container.AddItem(modeHeader, 1, 0, false)
				gapBelow := tview.NewBox()
				gapBelow.SetBackgroundColor(theme.BgPanel)
				container.AddItem(gapBelow, 1, 0, false)
			}

			// The tv has a fixed height computed from its content so the mode
			// selector sits directly below the text. A single flexible spacer
			// between modeForm and buttonForm takes all remaining space,
			// keeping the button anchored at the bottom.
			// On small terminals, if the fixed content exceeds the available
			// space, the flexSpacer shrinks to 0 and tv is scrollable.
			if availH >= 35 {
				// Full: topSpacer + text + header + mode(desc) + flex + button + bottomSpacer
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				flexSpacer := tview.NewBox()
				flexSpacer.SetBackgroundColor(theme.BgPanel)
				bottomSpacer := tview.NewBox()
				bottomSpacer.SetBackgroundColor(theme.BgPanel)

				// Clamp tvHeight so button + bottomSpacer are always visible.
				maxTvH := availH - 3 - headerFixedH - modeFormHeight - 5 - 3 // top + header + mode + btn + bottom
				if tvHeight > maxTvH && maxTvH > 5 {
					tvHeight = maxTvH
				}

				container.AddItem(topSpacer, 3, 0, false)
				container.AddItem(tv, tvHeight, 0, false)
				addModeHeader()
				container.AddItem(modeForm, modeFormHeight, 0, true)
				container.AddItem(flexSpacer, 0, 1, false)
				container.AddItem(buttonForm, 5, 0, false)
				container.AddItem(bottomSpacer, 3, 0, false)
			} else if availH >= 20 {
				// Compact: topSpacer + text + header + mode(desc?) + flex + button
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				flexSpacer := tview.NewBox()
				flexSpacer.SetBackgroundColor(theme.BgPanel)

				maxTvH := availH - 1 - headerFixedH - modeFormHeight - 3 // top + header + mode + btn
				if tvHeight > maxTvH && maxTvH > 5 {
					tvHeight = maxTvH
				}

				container.AddItem(topSpacer, 1, 0, false)
				container.AddItem(tv, tvHeight, 0, false)
				addModeHeader()
				container.AddItem(modeForm, modeFormHeight, 0, true)
				container.AddItem(flexSpacer, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, false)
			} else {
				// Minimal: text + mode(no desc) + button
				container.AddItem(tv, 0, 1, false)
				container.AddItem(modeForm, modeFormHeight, 0, true)
				container.AddItem(buttonForm, 3, 0, false)
			}
			tvApp.SetFocus(modeForm)
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Status", Value: i18n.T("cmd.init.wizard_started")}}
		},
	}
}

// buildLangStep creates the language selection step with live locale switching.
func buildLangStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:       "lang",
		Label:    i18n.T("cmd.init.wizard_step_lang"),
		Required: true,
		Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
			langOptions := []string{"Français", "English"}

			rerenderLang := func() {
				if fn := (*s.Steps)[s.LangStepIdx].Rerender; fn != nil {
					go func() { tvApp.QueueUpdateDraw(func() { fn() }) }()
				}
			}

			form := tview.NewForm()
			langMounted := false
			form.AddDropDown(i18n.T("cmd.init.wizard_lang_select"), langOptions, s.LangIdx, func(_ string, idx int) {
				if s.LangIdx == idx {
					return
				}
				s.LangIdx = idx
				if idx == 1 {
					s.SelectedLang = "en"
				} else {
					s.SelectedLang = "fr"
				}
				i18n.SetLocale(s.SelectedLang)

				if langMounted {
					*s.FocusBtn = true
					rerenderLang()
				}
			})
			form.AddButton(i18n.T("wizard.hint.submit"), onDone)
			langMounted = true
			return form
		},
		OnDone: func() error {
			i18n.SetLocale(s.SelectedLang)
			return config.Update(func(c *config.Config) error {
				c.CLI.Language = s.SelectedLang
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			label := "Français"
			if s.SelectedLang == "en" {
				label = "English"
			}
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_lang"), Value: label}}
		},
	}
}

// buildProviderStep creates the single dynamic provider + credentials step.
// The form adapts its fields based on the selected provider (and auth mode
// for bedrock). DropDown changes trigger a full step re-render via Rerender.
// The intro text (title, description, prerequisites) is embedded at the top
// of the form, eliminating the need for a separate intro page.
func buildProviderStep(s *initStepState) views.WizardStep {
	a := *s.AppPtr
	return views.WizardStep{
		ID:    "provider",
		Label: i18n.T("cmd.init.wizard_step_provider_label"),
		SkipIf: func() bool {
			return s.ProviderSkipped
		},
		Validate: func() string {
			if s.ProviderSkipped {
				return "" // Skip mode: bypass validation.
			}
			switch s.SelectedProvider {
			case "anthropic", "openrouter":
				if s.Token == "" && !s.HasKeychainToken {
					return i18n.T("cmd.init.wizard_token_required")
				}
			case "bedrock":
				if s.AuthMode == "bearer" && s.Token == "" && !s.HasKeychainToken {
					return i18n.T("cmd.init.wizard_token_required")
				}
				if s.Region == "" {
					return i18n.T("cmd.init.wizard_region_required")
				}
			}
			return ""
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Auto-detection: pre-select the first available provider ──
			detectedSource := ""
			reusedFromConfig := false
			if s.SelectedProvider == "" {
				// Run provider detection to find available credentials.
				detections := providerPkg.DetectAll()
				for _, d := range detections {
					if d.Available {
						name := string(d.Provider)
						for idx, opt := range s.ProviderOptions {
							if opt == name {
								s.SelectedProvider = name
								s.ProviderIdx = idx
								detectedSource = d.Source
								if d.Details != "" {
									detectedSource += " (" + d.Details + ")"
								}
								break
							}
						}
						if s.SelectedProvider != "" {
							break
						}
					}
				}
			} else {
				// Provider was already set (from config pre-fill) — mark as reused.
				reusedFromConfig = true
			}
			if s.SelectedProvider == "" {
				s.SelectedProvider = s.ProviderOptions[0]
			}
			if s.AuthMode == "" {
				s.AuthMode = "bearer"
			}
			if s.ProfileName == "" {
				s.ProfileName = "default"
			}

			// Credential detection: check keychain for existing token
			s.HasKeychainToken = false
			if s.Token == "" {
				name := providerPkg.Name(s.SelectedProvider)
				if a.Secrets != nil {
					if key := providerPkg.KeychainKey(name, ""); key != "" {
						if existing, err := a.Secrets.Get(context.Background(), key); err == nil && existing != "" {
							s.HasKeychainToken = true
						}
					}
				}
			}

			rerenderSafe := func() {
				if fn := (*s.Steps)[s.ProviderStepIdx].Rerender; fn != nil {
					go func() { tvApp.QueueUpdateDraw(func() { fn() }) }()
				}
			}

			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			reset := theme.TagColor

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_provider_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_provider_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			intro.WriteString("\n")
			fmt.Fprintf(&intro, "%s%s%s  %s%s%s\n", muted, i18n.T("cmd.init.wizard_intro_provider_list"), reset, accent, i18n.T("cmd.init.wizard_provider_list_items"), reset)
			intro.WriteString("\n")
			fmt.Fprintf(&intro, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s\n", muted, reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_provider_prereq"), "\n") {
				if strings.HasPrefix(line, "• ") {
					fmt.Fprintf(&intro, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
				} else {
					fmt.Fprintf(&intro, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
				}
			}

			// ── Form with interactive fields only ──
			form := tview.NewForm()
			authModes := []string{"bearer", "profile", "env"}

			// Provider dropdown
			form.AddDropDown(
				i18n.T("cmd.init.wizard_provider_select"),
				s.ProviderOptions, s.ProviderIdx,
				func(_ string, idx int) {
					if s.ProviderIdx == idx {
						return
					}
					s.ProviderIdx = idx
					s.SelectedProvider = s.ProviderOptions[idx]
					s.Token = ""
					s.Region = ""
					s.RegionIdx = 0
					s.CustomRegion = false
					s.HasKeychainToken = false
					s.ProfileName = "default"
					s.AuthMode = "bearer"
					s.AuthIdx = 0
					rerenderSafe()
				},
			)

			// Show detection/reuse source hint when provider was pre-selected.
			if detectedSource != "" {
				info := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", info, i18n.Tf("cmd.init.wizard_detected_from", detectedSource), reset), 60, 1, true, false)
			} else if reusedFromConfig {
				info := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", info, i18n.T("cmd.init.wizard_reused_from"), reset), 60, 1, true, false)
			}

			// Conditional fields based on current provider
			switch s.SelectedProvider {
			case "bedrock":
				form.AddDropDown(
					i18n.T("cmd.init.wizard_auth_mode"),
					authModes, s.AuthIdx,
					func(_ string, idx int) {
						if s.AuthIdx == idx {
							return
						}
						s.AuthIdx = idx
						s.AuthMode = authModes[idx]
						s.Token = ""
						s.ProfileName = "default"
						s.Region = ""
						s.RegionIdx = 0
						s.CustomRegion = false
						s.HasKeychainToken = false
						rerenderSafe()
					},
				)
				switch s.AuthMode {
				case "bearer":
					if s.HasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
					}
					form.AddPasswordField(i18n.T("cmd.init.wizard_bearer_token"), s.Token, 0, '*', func(t string) { s.Token = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_bearer"), 60, 1, true, false)
				case "profile":
					form.AddInputField(i18n.T("cmd.init.wizard_aws_profile"), s.ProfileName, 0, nil, func(t string) { s.ProfileName = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_profile"), 60, 1, true, false)
				case "env":
					// No extra fields before the region dropdown.
				}

				// Region dropdown (shared by all bedrock auth modes)
				addBedrockRegionDropDown(form, &s.Region, &s.RegionIdx, &s.CustomRegion, rerenderSafe)

			case "anthropic":
				if s.HasKeychainToken {
					form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
				}
				form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), s.Token, 0, '*', func(t string) { s.Token = t })
				form.AddTextView("", i18n.T("cmd.init.provider_hint_anthropic"), 60, 1, true, false)

			case "openrouter":
				if s.HasKeychainToken {
					form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
				}
				form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), s.Token, 0, '*', func(t string) { s.Token = t })
				form.AddTextView("", i18n.T("cmd.init.provider_hint_openrouter"), 60, 1, true, false)

			case "github-copilot":
				form.AddTextView("", i18n.T("cmd.init.wizard_copilot_desc"), 60, 2, true, false)
				form.AddTextView("", "$ gh auth login", 60, 1, true, false)
			}

			views.FixFormDropDownStyles(form)
			views.FixFormLabelFocus(form)

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.ProviderSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.ProviderSkipped = true
				s.Token = ""
				onDone()
			})

			buildFormStepLayout(tvApp, container, intro.String(), i18n.T("cmd.init.wizard_section_config"), form, buttonForm, form)
		},
		OnDone: func() error {
			if s.ProviderSkipped {
				return nil // Skip mode: nothing to persist.
			}
			a := *s.AppPtr
			switch s.SelectedProvider {
			case "bedrock":
				if s.AuthMode == "bearer" && s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.Bedrock, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			case "anthropic":
				if s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.Anthropic, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			case "openrouter":
				if s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.OpenRouter, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			}

			return config.Update(func(c *config.Config) error {
				c.Opencode.DefaultProvider = s.SelectedProvider
				if s.SelectedProvider == "bedrock" {
					c.Provider.Bedrock.AuthMode = s.AuthMode
					if s.Region != "" {
						c.Provider.Bedrock.AWSRegion = s.Region
					}
					if s.AuthMode == "profile" && s.ProfileName != "" {
						c.Provider.Bedrock.AWSProfile = s.ProfileName
					}
				}
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			a := *s.AppPtr
			fields := []views.InfoField{{Label: "Provider", Value: s.SelectedProvider}}
			if s.SelectedProvider == "bedrock" {
				fields = append(fields, views.InfoField{Label: "Auth", Value: s.AuthMode})
				if s.Region != "" {
					fields = append(fields, views.InfoField{Label: "Region", Value: s.Region})
				}
			}
			if a.Secrets == nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: infoWarning(i18n.T("cmd.init.wizard_no_keyring")),
				})
			}
			if _, err := opencode.FindBinary(); err != nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: infoWarning(i18n.T("cmd.init.wizard_opencode_not_found")),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_credentials"),
	}
}

// buildProjectStep creates the first project creation step.
// The intro text (title, description, optional note) is embedded at the top
// of the form, eliminating the need for a separate intro page.
func buildProjectStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "project",
		Label: i18n.T("cmd.init.wizard_step_project"),
		SkipIf: func() bool {
			return s.ProjectSkipped
		},
		Validate: func() string {
			if s.ProjectSkipped {
				return "" // Skip mode: bypass validation.
			}
			if s.ProjectName == "" {
				return i18n.T("cmd.init.wizard_project_name_required")
			}
			p := expandPath(s.ProjectPath)
			abs, err := filepath.Abs(p)
			if err != nil {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			info, err := os.Stat(abs)
			if err != nil {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			if !info.IsDir() {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			s.ProjectPath = abs

			// Reject if another project (different path) already uses this name.
			if store := (*s.AppPtr).Projects; store != nil {
				if existing, err := store.GetByName(context.Background(), s.ProjectName); err == nil && existing != nil {
					if existing.Path != abs {
						return i18n.Tf("cmd.init.wizard_project_name_duplicate", s.ProjectName)
					}
					// Same path → will be updated in OnDone, not a conflict.
				}
			}
			return ""
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_project_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_project_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			note := i18n.T("cmd.init.wizard_intro_project_optional")
			if note != "" {
				fmt.Fprintf(&intro, "%s%s%s\n", muted, note, reset)
			}
			fmt.Fprintf(&intro, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s", muted, reset)

			// ── Auto-detection: pre-fill project name from git remote ──
			detectedProject := ""
			if s.ProjectName == "" {
				if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
					remote := strings.TrimSpace(string(out))
					if remote != "" {
						s.ProjectName = config.RepoNameFromRemote(remote)
						detectedProject = "git remote"
					}
				}
			}
			if s.ProjectPath == "" {
				s.ProjectPath = "."
			}

			// ── Form with interactive fields only ──
			form := tview.NewForm()
			initialName := s.ProjectName
			initialPath := s.ProjectPath
			form.AddInputField(i18n.T("cmd.init.wizard_project_name"), initialName, 0, nil, func(t string) { s.ProjectName = t })
			if detectedProject != "" {
				infoColor := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", infoColor, i18n.Tf("cmd.init.wizard_detected_from", detectedProject), reset), 60, 1, true, false)
			}
			form.AddInputField(i18n.T("cmd.init.wizard_project_path"), initialPath, 0, nil, func(t string) { s.ProjectPath = t })

			if s.TeamState.Configured && s.TeamState.TeamID != "" {
				attachOptions := []string{
					i18n.Tf("cmd.init.wizard_project_attach_yes", s.TeamState.TeamID),
					i18n.T("cmd.init.wizard_project_attach_no"),
				}
				s.TeamState.attachProject = true
				attachMounted := false
				form.AddDropDown(i18n.T("cmd.init.wizard_project_attach_team"), attachOptions, 0, func(_ string, idx int) {
					s.TeamState.attachProject = idx == 0
					if attachMounted {
						views.AutoAdvanceFromDropDown(tvApp, form, 2)
					}
				})
				// Mark as mounted after the closure is created to avoid
				// triggering auto-advance during initial render.
				defer func() { attachMounted = true }()
			}

			views.FixFormDropDownStyles(form)

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.ProjectSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.ProjectSkipped = true
				s.ProjectName = ""
				s.ProjectPath = ""
				onDone()
			})

			buildFormStepLayout(tvApp, container, intro.String(), i18n.T("cmd.init.wizard_section_project"), form, buttonForm, form)
		},
		OnDone: func() error {
			if s.ProjectSkipped {
				return nil // Skip mode: nothing to persist.
			}
			store := (*s.AppPtr).Projects
			if store == nil {
				return fmt.Errorf("project store not initialized")
			}
			ctx := context.Background()

			p := &domain.Project{
				ID:     uuid.New().String()[:8],
				Name:   s.ProjectName,
				Path:   s.ProjectPath,
				Status: domain.ProjectStatusActive,
			}
			if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
				tid := s.TeamState.TeamID
				p.TeamID = &tid
			}

			result, _, err := upsertProject(ctx, store, p)
			if err != nil {
				return err
			}
			s.ProjectID = result.ID
			s.ProjectCreated = true
			return nil
		},
		InfoFields: func() []views.InfoField {
			fields := []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_project"), Value: infoSuccess(i18n.T("cmd.init.wizard_project_added"))}}
			if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
				fields = append(fields, views.InfoField{
					Label: i18n.T("cmd.init.wizard_step_team"),
					Value: infoSuccess(i18n.Tf("cmd.init.wizard_project_attached", s.TeamState.TeamID)),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_project"),
	}
}

// countHubContent returns the number of agent and skill files found
// in the hub content directory. Returns (0, 0) when hubDir is empty or unreadable.
func countHubContent(hubDir string) (agents int, skills int) {
	if hubDir == "" {
		return 0, 0
	}
	agentDir := filepath.Join(hubDir, "agents")
	if entries, err := os.ReadDir(agentDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				agents++
			}
		}
	}
	skillDir := filepath.Join(hubDir, "skills")
	if entries, err := os.ReadDir(skillDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skills++
			}
		}
	}
	return
}

// ─────────────────────────────────────────────────────────────────────────────
// buildAgentSelectionStep — checkbox form for agent selection
// ─────────────────────────────────────────────────────────────────────────────

// buildAgentSelectionStep creates a form step with a checkbox per hub agent.
// All agents are selected by default. The result is stored in s.SelectedAgents.
// The collection of selected agents happens in Validate (called by the wizard
// engine before advancing) rather than in a form button callback, because in
// grouped mode the engine strips form buttons and replaces them with its own.
// The deploy intro text is embedded at the top of the form.
func buildAgentSelectionStep(s *initStepState) views.WizardStep {
	// Shared between CustomView and Validate closures.
	var available []string
	var selected map[string]bool

	return views.WizardStep{
		ID:    "agents",
		Label: i18n.T("cmd.init.wizard_step_agents"),
		SkipIf: func() bool {
			return s.DeploySkipped || s.ProjectSkipped || !s.ProjectCreated
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_deploy_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_deploy_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			intro.WriteString("\n")
			listTitle := i18n.T("cmd.init.wizard_intro_deploy_list")
			listItems := i18n.T("cmd.init.wizard_deploy_list_items")
			if listTitle != "" && listItems != "" {
				fmt.Fprintf(&intro, "%s%s%s  %s%s%s\n", muted, listTitle, reset, accent, listItems, reset)
			}
			note := i18n.T("cmd.init.wizard_intro_deploy_note")
			if note != "" {
				fmt.Fprintf(&intro, "\n%s%s%s", muted, note, reset)
			}
			fmt.Fprintf(&intro, "\n%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s", muted, reset)

			// ── Form with checkboxes only ──
			form := tview.NewForm()
			available = discoverAgents()
			selected = make(map[string]bool, len(available))
			for _, ag := range available {
				selected[ag] = true
			}
			for _, ag := range available {
				agName := ag
				form.AddCheckbox(agName, true, func(checked bool) {
					selected[agName] = checked
				})
			}

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.DeploySkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.DeploySkipped = true
				s.SelectedAgents = nil
				onDone()
			})

			buildFormStepLayout(tvApp, container, intro.String(), i18n.T("cmd.init.wizard_section_agents"), form, buttonForm, form)
		},
		Validate: func() string {
			if s.DeploySkipped {
				return "" // Skip mode: bypass validation.
			}
			// Collect selected agents into shared state before advancing.
			s.SelectedAgents = nil
			for _, ag := range available {
				if selected[ag] {
					s.SelectedAgents = append(s.SelectedAgents, ag)
				}
			}
			return "" // no validation error
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{
				Label: i18n.T("cmd.init.wizard_step_agents"),
				Value: i18n.Tf("cmd.init.wizard_deploy_recap_agents", len(s.SelectedAgents)),
			}}
		},
	}
}

// buildDeployStep creates the deploy confirmation step with a recap of what
// will be deployed, and two buttons: "Deploy now" / "Deploy later".
// The step displays a summary of selected agents, detected skills, configured
// MCP servers, and the chosen provider.
func buildDeployStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "deploy",
		Label: i18n.T("cmd.init.wizard_step_deploy_confirm"),
		SkipIf: func() bool {
			return s.DeploySkipped || s.ProjectSkipped || !s.ProjectCreated
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			s.DeployConfirmed = false

			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			hubDir := findHubDir()
			_, skillCount := countHubContent(hubDir)

			// Build recap text.
			var b strings.Builder
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_deploy_recap_title"), reset)

			// ── Agents recap ──
			agentCount := len(s.SelectedAgents)
			fmt.Fprintf(&b, "  %s%s%s\n", secondary,
				i18n.Tf("cmd.init.wizard_deploy_recap_agents", agentCount), reset)
			if agentCount > 0 {
				const maxShow = 6
				shown := s.SelectedAgents
				if len(shown) > maxShow {
					list := strings.Join(shown[:maxShow], ", ")
					fmt.Fprintf(&b, "  %s%s%s\n", muted,
						i18n.Tf("cmd.init.wizard_deploy_recap_agents_more", list, agentCount-maxShow), reset)
				} else {
					fmt.Fprintf(&b, "  %s%s%s\n", muted,
						i18n.Tf("cmd.init.wizard_deploy_recap_agents_list", strings.Join(shown, ", ")), reset)
				}
			}
			b.WriteString("\n")

			// ── Skills recap ──
			fmt.Fprintf(&b, "  %s%s%s\n\n", secondary,
				i18n.Tf("cmd.init.wizard_deploy_recap_skills", skillCount), reset)

			// ── MCP recap ──
			var mcpList []string
			if s.FigmaToken != "" {
				mcpList = append(mcpList, "Figma")
			}
			if s.GitlabToken != "" {
				mcpList = append(mcpList, "GitLab")
			}
			if s.GslidesToken != "" {
				mcpList = append(mcpList, "Google Slides")
			}
			if len(mcpList) > 0 {
				fmt.Fprintf(&b, "  %s%s%s\n\n", secondary,
					i18n.Tf("cmd.init.wizard_deploy_recap_mcp", strings.Join(mcpList, ", ")), reset)
			} else {
				fmt.Fprintf(&b, "  %s%s%s\n\n", muted,
					i18n.T("cmd.init.wizard_deploy_recap_mcp_none"), reset)
			}

			// ── Provider recap ──
			if s.SelectedProvider != "" {
				fmt.Fprintf(&b, "  %s%s%s\n", secondary,
					i18n.Tf("cmd.init.wizard_deploy_recap_provider", s.SelectedProvider), reset)
			}

			tv := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter)
			tv.SetBackgroundColor(theme.BgPanel)
			tv.SetText(b.String())

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_deploy_now")+"  ", func() {
				s.DeployConfirmed = true
				onDone()
			})
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_deploy_skip_btn")+"  ", func() {
				s.DeployConfirmed = false
				onDone()
			})

			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})

			_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
			if termH <= 0 {
				termH = 50
			}
			availH := termH - 7

			if availH >= 35 {
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				bottomSpacer := tview.NewBox()
				bottomSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 3, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 5, 0, true)
				container.AddItem(bottomSpacer, 3, 0, false)
			} else if availH >= 20 {
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 1, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			} else {
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			}
			tvApp.SetFocus(buttonForm)
		},
		OnDone: func() error {
			ctx := context.Background()
			a := *s.AppPtr

			// Always persist the agent selection to the project in DB,
			// whether deploying now or later. A future 'oh deploy' will
			// pick up project.Agents automatically.
			if s.ProjectCreated && s.ProjectID != "" && a.Projects != nil {
				project, err := a.Projects.Get(ctx, s.ProjectID)
				if err != nil {
					slog.Warn("deploy step: could not fetch project for agent update", "err", err)
				} else {
					project.Agents = s.SelectedAgents
					project.Provider = s.SelectedProvider
					if err := a.Projects.Update(ctx, project); err != nil {
						slog.Warn("deploy step: could not update project agents", "err", err)
					}
				}
			}

			if !s.DeployConfirmed {
				return nil
			}

			config.Reset()
			newApp, err := ReloadApp()
			if err != nil {
				return fmt.Errorf("reload: %w", err)
			}
			*s.AppPtr = newApp

			hubDir := findHubDir()
			if hubDir == "" {
				slog.Warn("deploy step skipped: hub content directory not found")
				return nil
			}

			// Fetch the updated project so buildDeployPlan gets the full
			// context (Agents, MCPConfig, ModelOverrides, WorkflowConfig).
			a = *s.AppPtr
			var proj *domain.Project
			if a.Projects != nil && s.ProjectID != "" {
				proj, _ = a.Projects.Get(ctx, s.ProjectID)
			}

			plan := buildDeployPlan(a, DeployRequest{
				Project:        proj,
				ProjectPath:    s.ProjectPath,
				HubDir:         hubDir,
				Provider:       s.SelectedProvider,
				SelectedAgents: s.SelectedAgents,
			})
			_, err = deploy.Execute(ctx, plan)
			return err
		},
		InfoFields: func() []views.InfoField {
			if s.DeploySkipped {
				return []views.InfoField{{Label: "Deploy", Value: infoMuted(i18n.T("cmd.init.wizard_deploy_section_skipped"))}}
			}
			if s.DeployConfirmed {
				return []views.InfoField{{Label: "Deploy", Value: infoSuccess(i18n.T("cmd.init.wizard_deploy_done"))}}
			}
			return []views.InfoField{{Label: "Deploy", Value: infoMuted(i18n.T("cmd.init.wizard_deploy_skipped"))}}
		},
		Processing: i18n.T("cmd.init.wizard_deploy_processing"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP Consolidated Step — combines intro + 3 token steps into a single page
// ─────────────────────────────────────────────────────────────────────────────

// buildMCPConsolidatedStep returns a single WizardStep that replaces the MCP
// intro page and the 3 individual token steps (Figma, GitLab, Google Slides).
// Each integration has a checkbox to enable/disable it and a password field
// that appears only when the checkbox is checked.
func buildMCPConsolidatedStep(s *initStepState, a *app.App) views.WizardStep {
	type mcpEntry struct {
		name          string
		tokenVar      *string
		tokenKey      string
		hintKey       string
		checkboxVar   *bool
		checkboxLabel string
		checkboxDesc  string
		afterStore    func() error
	}

	entries := []mcpEntry{
		{
			name: "Figma", tokenVar: &s.FigmaToken,
			tokenKey: config.DefaultFigmaTokenKey, hintKey: "cmd.init.mcp_hint_figma",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error { c.MCP.Figma.Enabled = true; return nil })
			},
		},
		{
			name: "GitLab", tokenVar: &s.GitlabToken,
			tokenKey: config.DefaultGitLabTokenKey, hintKey: "cmd.init.mcp_hint_gitlab",
			checkboxVar: &s.GitlabWrite, checkboxLabel: i18n.T("cmd.init.mcp_gitlab_write_short"),
			checkboxDesc: "cmd.init.mcp_gitlab_write_desc",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error {
					c.MCP.Gitlab.Enabled = true
					if s.GitlabWrite {
						c.MCP.Gitlab.WriteEnabled = true
					}
					return nil
				})
			},
		},
		{
			name: "Google Slides", tokenVar: &s.GslidesToken,
			tokenKey: config.DefaultGslidesTokenKey, hintKey: "cmd.init.mcp_hint_gslides",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error { c.MCP.Gslides.Enabled = true; return nil })
			},
		},
	}

	return views.WizardStep{
		ID: "mcp_consolidated", Label: "MCP",
		SkipIf: func() bool { return s.MCPSkipped },
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			reset := theme.TagColor

			// Render intro text above the form.
			var b strings.Builder
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_intro_mcp_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_mcp_desc"), "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n", muted, i18n.T("cmd.init.wizard_intro_mcp_list"), reset)
			fmt.Fprintf(&b, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_mcp_list_items"), reset)
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s\n\n", muted, reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_mcp_prereq"), "\n") {
				if strings.HasPrefix(line, "• ") {
					fmt.Fprintf(&b, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
				} else {
					fmt.Fprintf(&b, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
				}
			}

			// Build the form with checkbox + token per integration.
			form := tview.NewForm()

			enabled := make([]bool, len(entries))
			for i := range entries {
				enabled[i] = true
			}

			// rebuildForm reconstructs the form after a checkbox toggle.
			// focusEntry is the index of the entry whose checkbox was toggled
			// (-1 on the initial build). After rebuild, focus is restored to
			// that checkbox so keyboard navigation keeps working.
			var rebuildForm func(focusEntry int)
			rebuildForm = func(focusEntry int) {
				form.Clear(true)
				targetFormIdx := 0
				for idx := range entries {
					ci := idx
					e := entries[idx]
					if ci == focusEntry {
						targetFormIdx = form.GetFormItemCount()
					}
					form.AddCheckbox(e.name, enabled[ci], func(checked bool) {
						enabled[ci] = checked
						go func() { tvApp.QueueUpdateDraw(func() { rebuildForm(ci) }) }()
					})
					if !enabled[ci] {
						continue
					}
					hasKeychainToken := false
					if a.Secrets != nil && *e.tokenVar == "" {
						if existing, err := a.Secrets.Get(context.Background(), e.tokenKey); err == nil && existing != "" {
							hasKeychainToken = true
						}
					}
					if hasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 1, true, false)
					}
					form.AddPasswordField(
						i18n.Tf("cmd.init.mcp_token_prompt", e.name), *e.tokenVar, 0, '*',
						func(t string) { *entries[ci].tokenVar = t },
					)
					if e.hintKey != "" {
						form.AddTextView("", i18n.T(e.hintKey), 60, 2, true, false)
					}
					if e.checkboxVar != nil {
						if e.checkboxDesc != "" {
							form.AddTextView("", i18n.T(e.checkboxDesc), 60, 2, true, false)
						}
						cbVar := e.checkboxVar
						form.AddCheckbox(e.checkboxLabel, *cbVar, func(checked bool) { *cbVar = checked })
					}
				}
				views.FixFormDropDownStyles(form)
				form.SetFocus(targetFormIdx)
				views.FixFormLabelFocus(form)
				tvApp.SetFocus(form)
			}
			rebuildForm(-1)

			// Button bar with Continue + Skip (double-click confirm).
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.MCPSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.MCPSkipped = true
				s.FigmaToken = ""
				s.GitlabToken = ""
				s.GslidesToken = ""
				onDone()
			})

			buildFormStepLayout(tvApp, container, b.String(), i18n.T("cmd.init.wizard_section_mcp"), form, buttonForm, form)
		},
		OnDone: func() error {
			for _, entry := range entries {
				if *entry.tokenVar == "" {
					continue
				}
				if a.Secrets != nil {
					if err := a.Secrets.Set(context.Background(), entry.tokenKey, *entry.tokenVar); err != nil {
						return fmt.Errorf("keychain %s: %w", entry.name, err)
					}
				}
				if entry.afterStore != nil {
					if err := entry.afterStore(); err != nil {
						return err
					}
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			var fields []views.InfoField
			for _, entry := range entries {
				if *entry.tokenVar == "" {
					fields = append(fields, views.InfoField{Label: entry.name, Value: infoMuted(i18n.T("cmd.init.wizard_mcp_skipped"))})
				} else {
					fields = append(fields, views.InfoField{Label: entry.name, Value: infoSuccess(i18n.T("cmd.init.wizard_mcp_configured"))})
				}
			}
			if s.GitlabToken != "" && s.GitlabWrite {
				fields = append(fields, views.InfoField{Label: "Write", Value: infoSuccess(i18n.T("cmd.init.wizard_mcp_enabled"))})
			}
			return fields
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildIntroStep — group introduction page
// ─────────────────────────────────────────────────────────────────────────────

// buildIntroStep creates a CustomView step that serves as a group introduction.
// Parameters are i18n KEYS (not resolved values) so the content is freshly
// translated each time the step is rendered — essential for live locale switching.
// When onSkip is non-nil, a "Skip" button is added alongside "Continue".
// When onContinue is non-nil, it is called when "Continue" is clicked
// (use to reset a skip flag when the user goes back and re-enters a group).
func buildIntroStep(badge, titleKey, descKey, listTitleKey, listItemsKey, prereqsKey, noteKey string, onContinue, onSkip func()) views.WizardStep {
	return views.WizardStep{
		Label:         badge,
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			reset := theme.TagColor

			title := i18n.T(titleKey)
			desc := i18n.T(descKey)
			listTitle := ""
			listItems := ""
			if listTitleKey != "" {
				listTitle = i18n.T(listTitleKey)
			}
			if listItemsKey != "" {
				listItems = i18n.T(listItemsKey)
			}
			prereqs := ""
			if prereqsKey != "" {
				prereqs = i18n.T(prereqsKey)
			}
			note := ""
			if noteKey != "" {
				note = i18n.T(noteKey)
			}

			var b strings.Builder
			b.WriteString("\n")

			fmt.Fprintf(&b, "%s%s%s\n\n", accent, title, reset)

			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")

			if listTitle != "" && listItems != "" {
				fmt.Fprintf(&b, "%s%s%s\n", muted, listTitle, reset)
				fmt.Fprintf(&b, "%s%s%s\n", accent, listItems, reset)
				b.WriteString("\n")
			}

			if prereqs != "" {
				fmt.Fprintf(&b, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s\n\n", muted, reset)
				for _, line := range strings.Split(prereqs, "\n") {
					if strings.HasPrefix(line, "• ") {
						fmt.Fprintf(&b, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
					} else {
						fmt.Fprintf(&b, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
					}
				}
				b.WriteString("\n")
			}

			if note != "" {
				for _, line := range strings.Split(note, "\n") {
					fmt.Fprintf(&b, "%s%s%s\n", muted, line, reset)
				}
				b.WriteString("\n")
			}

			tv := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter)
			tv.SetBackgroundColor(theme.BgPanel)
			tv.SetText(b.String())

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.intro.continue")+"  ", func() {
				if onContinue != nil {
					onContinue()
				}
				onDone()
			})
			if onSkip != nil {
				skipConfirmed := false
				buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
					if !skipConfirmed {
						skipConfirmed = true
						if btn := buttonForm.GetButton(1); btn != nil {
							btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
						}
						return
					}
					onSkip()
					onDone()
				})
			}

			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})

			_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
			if termH <= 0 {
				termH = 50
			}
			availH := termH - 7

			if availH >= 35 {
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				badgeView := views.BuildStepBadge(badge, false)
				gapSpacer := tview.NewBox()
				gapSpacer.SetBackgroundColor(theme.BgPanel)
				bottomSpacer := tview.NewBox()
				bottomSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 3, 0, false)
				container.AddItem(badgeView, 5, 0, false)
				container.AddItem(gapSpacer, 2, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 5, 0, true)
				container.AddItem(bottomSpacer, 3, 0, false)
			} else if availH >= 20 {
				badgeView := views.BuildStepBadge(badge, true)
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 1, 0, false)
				container.AddItem(badgeView, 3, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			} else {
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			}
			tvApp.SetFocus(buttonForm)
		},
	}
}
