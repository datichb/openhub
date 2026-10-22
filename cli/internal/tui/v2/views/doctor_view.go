package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// DoctorCheck represents a single health check result. Warn marks a passed
// check that deserves attention (newer oh release…).
type DoctorCheck struct {
	Name   string
	Detail string
	OK     bool
	Warn   bool
	// Optional marks a check of something oh can do without (container
	// engine, bd…): when it fails it is reported as a warning and does not
	// change the exit code of oh doctor (U-05). See Normalized.
	Optional bool
	// Fix repairs what the check found (oh doctor --fix); nil: nothing to
	// repair from the TUI.
	Fix *DoctorFix
}

// Normalized turns the failure of an optional check into a warning.
func (c DoctorCheck) Normalized() DoctorCheck {
	if c.Optional && !c.OK {
		c.OK, c.Warn = true, true
	}
	return c
}

// DoctorFix is the repair of a check (key f « corriger »): Plan describes
// it for the confirmation, then Apply repairs (both off the event loop);
// or Open shows the view that repairs it (event loop).
type DoctorFix struct {
	Plan  func(ctx context.Context) (string, error)
	Apply func(ctx context.Context) (string, error)
	Open  func()
}

// DoctorFixFor gives the repair of a check (set by the command layer; nil
// or a nil result: none).
var DoctorFixFor func(DoctorCheck) *DoctorFix

// DoctorOfflineSwitch is the offline mode of the checks (oh doctor
// --offline): no network call (published version, provider key).
type DoctorOfflineSwitch struct {
	On  func() bool
	Set func(on bool)
}

// DoctorOffline is set by the command layer (nil: no offline option).
var DoctorOffline *DoctorOfflineSwitch

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
	shell   ShellAccess
	fixing  bool
	ctx     context.Context
	cancel  context.CancelFunc
	// loads numbers the checks (mount, recheck); fixes the repairs. Both are
	// invalidated by Unmount.
	loads, fixes loadSlot
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

// SetShell implements the shell-aware interface.
func (v *DoctorView) SetShell(s ShellAccess) { v.shell = s }

// doctorFixKey repairs the selected check; doctorOfflineKey switches the
// offline mode.
var (
	doctorFixKey     = keys.Own("doctor.fix", 'f', "tui.doctor.fix")
	doctorOfflineKey = keys.Own("doctor.offline", 'o', "tui.doctor.offline")
)

// StatusHints returns keybinding hints.
func (v *DoctorView) StatusHints() string {
	offline := ""
	if DoctorOffline != nil {
		offline = doctorOfflineKey.Hint()
	}
	return keys.Hints(keys.Navigate.Hint(), doctorFixKey.Hint(), offline,
		keys.Refresh.HintAs(i18n.T("tui.hints.recheck")), keys.Back.Hint())
}

// Mount builds the doctor checks display and runs all checks.
func (v *DoctorView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.fixing = false

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

	v.reload()
}

// reload runs the checks off the event loop (they invoke subprocesses); the
// latest request wins.
func (v *DoctorView) reload() {
	load(v.app, v.ctx, &v.loads, func(context.Context) ([]DoctorCheck, error) {
		return v.collectChecks(), nil
	}, func(checks []DoctorCheck, _ error) {
		if v.list == nil {
			return
		}
		v.checks, v.loading = checks, ""
		v.render()
	})
}

// Unmount cleans up resources.
func (v *DoctorView) Unmount() {
	v.loads.Invalidate()
	v.fixes.Invalidate()
	if v.cancel != nil {
		v.cancel()
	}
	v.app = nil
	v.summary, v.list, v.page = nil, nil, nil
}

// HandleKey processes view-specific key events.
func (v *DoctorView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.recheck()
		return nil
	case doctorFixKey.Matches(event):
		v.fixSelected()
		return nil
	case doctorOfflineKey.Matches(event) && DoctorOffline != nil:
		DoctorOffline.Set(!DoctorOffline.On())
		v.recheck()
		return nil
	}
	return event
}

// recheck runs the checks again off the event loop (not while a repair runs:
// it checks again itself).
func (v *DoctorView) recheck() {
	if v.list == nil || v.app == nil || v.fixing {
		return
	}
	v.loading = i18n.T("tui.doctor.rechecking")
	v.render()
	v.reload()
}

// selected is the check under the cursor.
func (v *DoctorView) selected() (DoctorCheck, bool) {
	if v.list == nil {
		return DoctorCheck{}, false
	}
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return DoctorCheck{}, false
	}
	i, ok := it.Reference.(int)
	if !ok || i < 0 || i >= len(v.checks) {
		return DoctorCheck{}, false
	}
	return v.checks[i], true
}

// fixSelected repairs the selected check: its plan, a confirmation, the
// repair, its report, then the checks again.
func (v *DoctorView) fixSelected() {
	c, ok := v.selected()
	if !ok || v.loading != "" || v.fixing {
		return
	}
	if c.Fix == nil {
		v.toast(i18n.T("tui.doctor.no_fix"), false)
		return
	}
	if c.Fix.Open != nil {
		c.Fix.Open()
		return
	}
	if c.Fix.Apply == nil || v.shell == nil || v.app == nil {
		return
	}
	v.fixing = true
	type planned struct{ plan string }
	load(v.app, v.ctx, &v.fixes, func(ctx context.Context) (planned, error) {
		if c.Fix.Plan == nil {
			return planned{}, nil
		}
		plan, err := c.Fix.Plan(ctx)
		return planned{plan}, err
	}, func(p planned, err error) {
		v.fixing = false
		if err != nil {
			v.toast(err.Error(), false)
			return
		}
		detail := strings.TrimSpace(p.plan)
		if detail != "" {
			detail += "\n\n"
		}
		v.shell.Confirm(i18n.Tf("tui.doctor.fix_title", c.Name), detail+i18n.T("tui.doctor.fix_confirm"), true, func() { v.applyFix(c, c.Fix) })
	})
}

func (v *DoctorView) applyFix(c DoctorCheck, fix *DoctorFix) {
	if v.app == nil || v.fixing {
		return
	}
	v.fixing = true
	v.loading = i18n.Tf("tui.doctor.fixing", c.Name)
	v.render()
	type fixed struct {
		report string
		checks []DoctorCheck
	}
	load(v.app, v.ctx, &v.fixes, func(ctx context.Context) (fixed, error) {
		report, err := fix.Apply(ctx)
		return fixed{report, v.collectChecks()}, err
	}, func(f fixed, err error) {
		v.fixing = false
		if v.list == nil {
			return
		}
		v.checks, v.loading = f.checks, ""
		v.render()
		if err != nil {
			v.toast(err.Error(), false)
			return
		}
		if report := strings.TrimSpace(f.report); report != "" && v.shell != nil {
			v.shell.ShowScrollableModal(i18n.Tf("tui.doctor.fixed_title", c.Name), tview.Escape(report), nil)
			return
		}
		v.toast(i18n.Tf("tui.doctor.fixed", c.Name), true)
	})
}

func (v *DoctorView) toast(msg string, ok bool) {
	if v.shell != nil {
		v.shell.ShowToastMsg(msg, ok)
	}
}

// collectChecks runs all health checks and returns the results.
// Safe to call from any goroutine.
func (v *DoctorView) collectChecks() []DoctorCheck {
	if v.run == nil {
		return nil
	}
	checks := v.run()
	for i := range checks {
		if checks[i].Fix == nil && DoctorFixFor != nil {
			checks[i].Fix = DoctorFixFor(checks[i])
		}
	}
	return checks
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
		name := doctorIcon(c) + "  " + tview.Escape(c.Name)
		if c.Fix != nil {
			name += "  " + theme.ColorTag(theme.TextMutedHex) + "[" + doctorFixKey.Keys() + "[] " + doctorFixKey.Label() + theme.TagColor
		}
		items = append(items, widgets.SectionItem{
			MainText:      name,
			SecondaryText: theme.ColorTag(theme.TextSecondaryHex) + tview.Escape(c.Detail) + theme.TagColor,
			Reference:     i,
		})
	}

	if v.summary != nil {
		line := fmt.Sprintf("[::b]%s[::-]  %s%s%s",
			i18n.T("tui.doctor.check_title"),
			theme.ColorTag(theme.TextPrimaryHex), i18n.Tf("tui.doctor.summary", passed, len(v.checks)), theme.TagColor)
		if DoctorOffline != nil && DoctorOffline.On() {
			line += "  " + theme.ColorTag(theme.WarningHex) + i18n.T("tui.doctor.offline_on") + theme.TagColor
		}
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
	if c.Fix != nil {
		d.keys(doctorFixKey.Keys(), doctorFixKey.Label(), keys.Refresh.Keys(), i18n.T("tui.hints.recheck"))
	} else {
		d.keys("r", i18n.T("tui.hints.recheck"))
	}
	v.page.SetDetail(d.String())
}
