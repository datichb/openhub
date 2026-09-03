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
	"github.com/datichb/openhub/cli/internal/opencode"
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
func (v *DoctorView) Title() string { return "Doctor" }

// StatusHints returns keybinding hints.
func (v *DoctorView) StatusHints() string {
	return fmt.Sprintf("r %s · Esc %s", i18n.T("tui.hints.recheck"), i18n.T("tui.hints.back"))
}

// Mount builds the doctor checks display and runs all checks.
func (v *DoctorView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %sVérification du système...%s", muted, theme.TagColor))
	content.AddItem(v.tv, 0, 1, true)

	// Run checks asynchronously (they invoke subprocesses)
	go func() {
		checks := v.collectChecks()
		app.QueueUpdateDraw(func() {
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
		v.tv.SetText(fmt.Sprintf("\n  %sVérification...%s", muted, theme.TagColor))
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
	return []DoctorCheck{
		v.checkOS(),
		v.checkBinary("git"),
		v.checkOpencode(),
		v.checkConfig(),
		v.checkDatabase(),
	}
}

func (v *DoctorView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]Vérification système%s\n\n", theme.TagReset))

	passed := 0
	for _, c := range v.checks {
		icon := theme.ColorTag(theme.SuccessHex) + theme.IconSuccess + theme.TagColor
		if !c.OK {
			icon = theme.ColorTag(theme.ErrorHex) + theme.IconError + theme.TagColor
		} else {
			passed++
		}
		sb.WriteString(fmt.Sprintf("  %s  %-28s %s%s%s\n",
			icon, c.Name,
			theme.ColorTag(theme.TextSecondaryHex), c.Detail, theme.TagColor))
	}

	sb.WriteString(fmt.Sprintf("\n  %s%d/%d checks OK%s\n",
		theme.ColorTag(theme.TextPrimaryHex), passed, len(v.checks), theme.TagColor))

	if passed == len(v.checks) {
		sb.WriteString(fmt.Sprintf("\n  %s%s Tout est en ordre.%s\n",
			theme.ColorTag(theme.SuccessHex), theme.IconSuccess, theme.TagColor))
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
		return DoctorCheck{Name: name, Detail: "non trouvé dans PATH", OK: false}
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

func (v *DoctorView) checkOpencode() DoctorCheck {
	ver, err := opencode.Version()
	if err != nil {
		_, findErr := opencode.FindBinary()
		if findErr != nil {
			return DoctorCheck{Name: "opencode", Detail: "non trouvé", OK: false}
		}
		return DoctorCheck{Name: "opencode", Detail: "installé (version inconnue)", OK: true}
	}
	return DoctorCheck{Name: "opencode", Detail: ver, OK: true}
}

func (v *DoctorView) checkConfig() DoctorCheck {
	if v.appCtx == nil || v.appCtx.Config == nil {
		return DoctorCheck{Name: "Configuration", Detail: "non chargée", OK: false}
	}
	return DoctorCheck{Name: "Configuration", Detail: "hub.toml OK", OK: true}
}

func (v *DoctorView) checkDatabase() DoctorCheck {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return DoctorCheck{Name: "Base de données", Detail: "non connectée", OK: false}
	}
	projects, err := v.appCtx.Projects.List(context.Background(), "")
	if err != nil {
		return DoctorCheck{Name: "Base de données", Detail: err.Error(), OK: false}
	}
	return DoctorCheck{Name: "Base de données", Detail: fmt.Sprintf("OK (%d projets)", len(projects)), OK: true}
}
