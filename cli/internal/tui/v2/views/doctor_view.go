package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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
	app     *tview.Application
	appCtx  *app.App
	summary *tview.TextView
	list    *widgets.SectionedList
	page    *widgets.ListDetailPage
	checks  []DoctorCheck
	loading string // non-empty while the checks run
	run     func() []DoctorCheck
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
	return fmt.Sprintf("j/k %s · r %s · Esc %s", i18n.T("tui.hints.nav"), i18n.T("tui.hints.recheck"), i18n.T("tui.hints.back"))
}

// Mount builds the doctor checks display and runs all checks.
func (v *DoctorView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.summary = tview.NewTextView().SetDynamicColors(true)
	v.summary.SetBackgroundColor(theme.BgPanel)
	v.list = widgets.NewSectionedList()
	v.list.SetApp(tvApp)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showDetail(it) })

	v.page = widgets.NewListDetailPage(v.Title(), i18n.T("tui.catalog.detail"))
	v.page.AddHeader(v.summary, 2)
	v.page.SetMain(v.list)
	v.page.Mount(content)
	if tvApp != nil {
		tvApp.SetFocus(v.list)
	}

	// Show loading placeholder immediately
	v.loading = i18n.T("tui.doctor.loading")
	v.render()

	// Run checks asynchronously (they invoke subprocesses)
	go func() {
		checks := v.collectChecks()
		tvApp.QueueUpdateDraw(func() {
			if v.list == nil {
				return
			}
			v.checks, v.loading = checks, ""
			v.render()
		})
	}()
}

// Unmount cleans up resources.
func (v *DoctorView) Unmount() {
	v.app = nil
	v.summary, v.list, v.page = nil, nil, nil
}

// HandleKey processes view-specific key events.
func (v *DoctorView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == 'r' {
		if v.list == nil || v.app == nil {
			return nil
		}
		v.loading = i18n.T("tui.doctor.rechecking")
		v.render()
		go func() {
			checks := v.collectChecks()
			v.app.QueueUpdateDraw(func() {
				if v.list == nil || v.app == nil {
					return
				}
				v.checks, v.loading = checks, ""
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
	if v.list == nil {
		return
	}
	muted := theme.ColorTag(theme.TextMutedHex)
	if v.loading != "" {
		if v.summary != nil {
			v.summary.SetText(muted + v.loading + theme.TagColor)
		}
		v.list.SetItems([]widgets.SectionItem{{MainText: muted + v.loading + theme.TagColor, IsHeader: true}})
		v.showDetail(widgets.SectionItem{})
		return
	}

	passed := 0
	items := make([]widgets.SectionItem, 0, len(v.checks))
	for i, c := range v.checks {
		if c.OK {
			passed++
		}
		items = append(items, widgets.SectionItem{
			MainText:      doctorIcon(c) + "  " + tview.Escape(c.Name),
			SecondaryText: theme.ColorTag(theme.TextSecondaryHex) + tview.Escape(c.Detail) + theme.TagColor,
			Reference:     i,
		})
	}

	if v.summary != nil {
		line := fmt.Sprintf("[::b]%s[::-]  %s%s%s",
			i18n.T("tui.doctor.check_title"),
			theme.ColorTag(theme.TextPrimaryHex), i18n.Tf("tui.doctor.summary", passed, len(v.checks)), theme.TagColor)
		if passed == len(v.checks) {
			line += fmt.Sprintf("  %s%s %s%s", theme.ColorTag(theme.SuccessHex), theme.IconSuccess, i18n.T("tui.doctor.all_passed"), theme.TagColor)
		}
		v.summary.SetText(line)
	}
	v.list.SetItems(items)
	_, it, _ := v.list.CurrentItem()
	v.showDetail(it)
}

// doctorIcon is the status icon of a check.
func doctorIcon(c DoctorCheck) string {
	switch {
	case !c.OK:
		return theme.ColorTag(theme.ErrorHex) + theme.IconError + theme.TagColor
	case c.Warn:
		return theme.ColorTag(theme.WarningHex) + theme.IconWarning + theme.TagColor
	default:
		return theme.ColorTag(theme.SuccessHex) + theme.IconSuccess + theme.TagColor
	}
}

// showDetail fills the detail pane with the check of it.
func (v *DoctorView) showDetail(it widgets.SectionItem) {
	if v.page == nil {
		return
	}
	i, ok := it.Reference.(int)
	if !ok || i < 0 || i >= len(v.checks) {
		v.page.SetDetail(emptyDetail())
		return
	}
	c := v.checks[i]
	status := i18n.T("tui.detail.ok")
	switch {
	case !c.OK:
		status = i18n.T("tui.detail.failed")
	case c.Warn:
		status = i18n.T("tui.detail.warn")
	}
	var d detailText
	d.title(tview.Escape(c.Name))
	d.field(i18n.T("tui.detail.status"), doctorIcon(c)+" "+status)
	d.gap().para(tview.Escape(c.Detail))
	d.keys("r", i18n.T("tui.hints.recheck"))
	v.page.SetDetail(d.String())
}
