package views

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// Section « Anciens déploiements » of the clean-up view: what the former
// per-project deployments left in the projects (`oh migrate
// deploy-cleanup`), what is kept, the diff of the configuration (c) and
// the clean-up (d, confirmed). The projects are scanned off the event
// loop.

// DeployCleanupPlan is what the clean-up of the former deployments would do.
type DeployCleanupPlan struct {
	// Projects is the number of projects with something to clean (0:
	// nothing left).
	Projects int
	// Text lists what goes and what is kept, by project (tview tags).
	Text string
	// DiffTitle and Diff are the diff of the configuration (tview tags).
	DiffTitle, Diff string
}

// DeployCleanupConfig wires the section of the former deployments.
type DeployCleanupConfig struct {
	// Scan finds what is left in the projects (off the event loop).
	Scan func(ctx context.Context) (DeployCleanupPlan, error)
	// Apply cleans the projects of the last scan (off the event loop): the
	// message of the result and whether everything was cleaned.
	Apply func(ctx context.Context) (string, bool)
}

type deployPane struct {
	v       *GCView
	root    *tview.Flex
	text    *tview.TextView
	plan    DeployCleanupPlan
	err     error
	loaded  bool
	running bool
}

func newDeployPane(v *GCView) *deployPane {
	p := &deployPane{v: v}
	p.text = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	p.text.SetBackgroundColor(theme.BgPanel)
	p.text.SetBorder(true).SetBorderColor(theme.FgMuted).SetTitleAlign(tview.AlignLeft)
	p.text.SetTitle(" " + i18n.T("tui.cleanup.title") + " ")
	p.text.SetBorderPadding(0, 0, 1, 1)
	p.root = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(p.text, 0, 1, true)
	p.root.SetBackgroundColor(theme.BgPanel)
	p.render()
	return p
}

func (p *deployPane) hints() string {
	if !p.cleanable() {
		return keys.Hints(keys.Navigate.Hint(), keys.Refresh.Hint())
	}
	return keys.Hints(keys.Navigate.Hint(), keys.Delete.HintAs(i18n.T("tui.cleanup.key.apply")),
		keys.Compare.HintAs(i18n.T("tui.cleanup.key.diff")), keys.Refresh.Hint())
}

func (p *deployPane) cleanable() bool {
	return p.loaded && p.err == nil && p.plan.Projects > 0 && !p.running
}

func (p *deployPane) handleKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(ev):
		p.scan()
		return nil
	case keys.Delete.Matches(ev):
		p.confirmApply()
		return nil
	case keys.Compare.Matches(ev):
		p.showDiff()
		return nil
	}
	return ev
}

// scan finds what is left, off the event loop.
func (p *deployPane) scan() {
	cfg, app, ctx := p.v.cfg.Deploy, p.v.app, p.v.ctx
	if cfg == nil || cfg.Scan == nil || app == nil || p.running {
		return
	}
	p.loaded = false
	p.render()
	go func() {
		plan, err := cfg.Scan(ctx)
		if ctx.Err() != nil {
			return
		}
		app.QueueUpdateDraw(func() { p.show(plan, err) })
	}()
}

// show shows a scan (event loop).
func (p *deployPane) show(plan DeployCleanupPlan, err error) {
	p.plan, p.err, p.loaded = plan, err, true
	p.render()
	if r, ok := p.v.shell.(HintsRefresher); ok && p.v.section == CleanupDeploy {
		r.RefreshHints()
	}
}

func (p *deployPane) render() {
	switch {
	case p.running:
		p.text.SetText("⠋ " + i18n.T("tui.cleanup.running"))
	case !p.loaded:
		p.text.SetText("⠋ " + i18n.T("tui.cleanup.scanning"))
	case p.err != nil:
		p.text.SetText("[" + theme.ErrorHex + "]✗ " + tview.Escape(p.err.Error()) + "[-]")
	case p.plan.Projects == 0:
		p.text.SetText("[" + theme.SuccessHex + "]✔ " + i18n.T("cmd.migrate.cleanup.none") + "[-]")
	default:
		p.text.SetText(p.plan.Text)
	}
	p.text.ScrollToBeginning()
}

func (p *deployPane) confirmApply() {
	sh := p.v.shell
	if !p.cleanable() || sh == nil {
		return
	}
	sh.Confirm(i18n.Tf("tui.cleanup.confirm_title", p.plan.Projects), i18n.T("tui.cleanup.confirm_detail"), true, p.apply)
}

// apply cleans the projects, off the event loop, then scans again.
func (p *deployPane) apply() {
	cfg, app, ctx := p.v.cfg.Deploy, p.v.app, p.v.ctx
	if cfg == nil || cfg.Apply == nil || app == nil || p.running {
		return
	}
	p.running = true
	p.render()
	go func() {
		msg, ok := cfg.Apply(ctx)
		if ctx.Err() != nil {
			return
		}
		app.QueueUpdateDraw(func() {
			p.running = false
			if p.v.shell != nil {
				p.v.shell.ShowToastMsg(msg, ok)
			}
			p.scan()
		})
	}()
}

func (p *deployPane) showDiff() {
	sh := p.v.shell
	if !p.cleanable() || sh == nil {
		return
	}
	sh.ShowScrollableModal(p.plan.DiffTitle, p.plan.Diff, []ModalAction{
		{Label: i18n.T("tui.cleanup.apply"), Callback: p.confirmApply},
		{Label: i18n.T("tui.cleanup.close"), Callback: func() {}, Separator: true},
	})
}
