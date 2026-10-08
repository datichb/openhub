package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// DoctorCheck represents a single health check result. Warn marks a passed
// check that deserves attention (newer oh release…).
type DoctorCheck struct {
	Name   string
	Detail string
	OK     bool
	Warn   bool
}

// DoctorView displays system health checks with real execution.
type DoctorView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
	checks []DoctorCheck
	run    func() []DoctorCheck
}

var _ View = (*DoctorView)(nil)

// NewDoctorView creates a new doctor view.
func NewDoctorView(a *app.App) *DoctorView {
	return &DoctorView{appCtx: a, run: DoctorChecks}
}

// ID returns the view identifier.
func (v *DoctorView) ID() string { return "doctor" }

// Title returns the display title.
func (v *DoctorView) Title() string { return i18n.T("tui.doctor.title") }

// StatusHints returns keybinding hints.
func (v *DoctorView) StatusHints() string {
	return fmt.Sprintf("r %s · Esc %s", i18n.T("tui.hints.recheck"), i18n.T("tui.hints.back"))
}

// Mount builds the doctor checks display and runs all checks.
func (v *DoctorView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.doctor.loading"), theme.TagColor))
	content.AddItem(v.tv, 0, 1, true)

	// Run checks asynchronously (they invoke subprocesses)
	go func() {
		checks := v.collectChecks()
		tvApp.QueueUpdateDraw(func() {
			if v.tv == nil {
				return
			}
			v.checks = checks
			v.render()
		})
	}()
}

// Unmount cleans up resources.
func (v *DoctorView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes view-specific key events.
func (v *DoctorView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == 'r' {
		muted := theme.ColorTag(theme.TextMutedHex)
		v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.doctor.rechecking"), theme.TagColor))
		go func() {
			checks := v.collectChecks()
			v.app.QueueUpdateDraw(func() {
				if v.tv == nil || v.app == nil {
					return
				}
				v.checks = checks
				v.render()
			})
		}()
		return nil
	}
	return event
}

// collectChecks runs all health checks and returns the results.
// Safe to call from any goroutine.
func (v *DoctorView) collectChecks() []DoctorCheck {
	if v.run == nil {
		return nil
	}
	return v.run()
}

// DoctorChecks runs the checks of `oh doctor` (set by the command layer): the
// view shows exactly the same checks as the CLI.
var DoctorChecks func() []DoctorCheck

func (v *DoctorView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", i18n.T("tui.doctor.check_title"), theme.TagReset)

	passed := 0
	for _, c := range v.checks {
		icon := theme.ColorTag(theme.SuccessHex) + theme.IconSuccess + theme.TagColor
		switch {
		case !c.OK:
			icon = theme.ColorTag(theme.ErrorHex) + theme.IconError + theme.TagColor
		case c.Warn:
			icon = theme.ColorTag(theme.WarningHex) + theme.IconWarning + theme.TagColor
			passed++
		default:
			passed++
		}
		fmt.Fprintf(&sb, "  %s  %-28s %s%s%s\n",
			icon, c.Name,
			theme.ColorTag(theme.TextSecondaryHex), c.Detail, theme.TagColor)
	}

	fmt.Fprintf(&sb, "\n  %s%s%s\n",
		theme.ColorTag(theme.TextPrimaryHex), i18n.Tf("tui.doctor.summary", passed, len(v.checks)), theme.TagColor)

	if passed == len(v.checks) {
		fmt.Fprintf(&sb, "\n  %s%s %s%s\n",
			theme.ColorTag(theme.SuccessHex), theme.IconSuccess, i18n.T("tui.doctor.all_passed"), theme.TagColor)
	}

	v.tv.SetText(sb.String())
}
