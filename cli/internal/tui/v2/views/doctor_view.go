package views

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// DoctorCheck represents a single health check result.
type DoctorCheck struct {
	Name   string
	Detail string
	OK     bool
}

// DoctorView displays system health checks with real execution.
type DoctorView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
	checks []DoctorCheck
}

var _ View = (*DoctorView)(nil)

// NewDoctorView creates a new doctor view.
func NewDoctorView(a *app.App) *DoctorView {
	return &DoctorView{appCtx: a}
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
	checks := []DoctorCheck{
		v.checkOS(),
		v.checkBinary("git"),
		v.checkConfig(),
		v.checkDatabase(),
	}
	if ExtraDoctorChecks != nil {
		checks = append(checks, ExtraDoctorChecks()...)
	}
	return checks
}

// ExtraDoctorChecks lets the command layer add runtime checks (v5 runtime,
// oh daemon, terminal integration) without the view depending on them.
var ExtraDoctorChecks func() []DoctorCheck

func (v *DoctorView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", i18n.T("tui.doctor.check_title"), theme.TagReset)

	passed := 0
	for _, c := range v.checks {
		icon := theme.ColorTag(theme.SuccessHex) + theme.IconSuccess + theme.TagColor
		if !c.OK {
			icon = theme.ColorTag(theme.ErrorHex) + theme.IconError + theme.TagColor
		} else {
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

func (v *DoctorView) checkOS() DoctorCheck {
	return DoctorCheck{
		Name:   "OS / Architecture",
		Detail: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		OK:     true,
	}
}

func (v *DoctorView) checkBinary(name string) DoctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return DoctorCheck{Name: name, Detail: i18n.T("tui.doctor.not_found_in_path"), OK: false}
	}
	// Try to get version
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return DoctorCheck{Name: name, Detail: path, OK: true}
	}
	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if len(version) > 40 {
		version = version[:40]
	}
	return DoctorCheck{Name: name, Detail: version, OK: true}
}

func (v *DoctorView) checkConfig() DoctorCheck {
	if v.appCtx == nil || v.appCtx.Config == nil {
		return DoctorCheck{Name: i18n.T("tui.doctor.check_config"), Detail: i18n.T("tui.doctor.not_loaded"), OK: false}
	}
	return DoctorCheck{Name: i18n.T("tui.doctor.check_config"), Detail: "hub.toml OK", OK: true}
}

func (v *DoctorView) checkDatabase() DoctorCheck {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return DoctorCheck{Name: i18n.T("tui.doctor.check_database"), Detail: i18n.T("tui.doctor.not_connected"), OK: false}
	}
	projects, err := v.appCtx.Projects.List(context.Background(), "")
	if err != nil {
		return DoctorCheck{Name: i18n.T("tui.doctor.check_database"), Detail: err.Error(), OK: false}
	}
	return DoctorCheck{Name: i18n.T("tui.doctor.check_database"), Detail: i18n.Tf("tui.doctor.db_ok_projects", len(projects)), OK: true}
}
