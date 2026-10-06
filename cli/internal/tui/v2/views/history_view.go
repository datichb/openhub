package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// History screen (P2-T15, 10-tui §10): the published version then the
// previous ones (author, date, message); Enter shows the diff of a version
// against the current one, r restores it (republished as a new version).
// ─────────────────────────────────────────────────────────────────────────────

// HistoryVersion is one version of a workflow.
type HistoryVersion struct {
	Version int
	By      string
	At      time.Time
	Message string
	Current bool
}

// HistoryViewConfig wires the history screen.
type HistoryViewConfig struct {
	Ref string
	// Load lists the versions, most recent first (off the event loop).
	Load func(ctx context.Context) ([]HistoryVersion, error)
	// Diff returns the unified diff of version against the current one.
	Diff func(ctx context.Context, version int) (string, error)
	// Restore republishes version (off the event loop); nil hides `r`.
	Restore func(ctx context.Context, version int) (*PublishResult, error)
	// OnRestored is called on the event loop after a restore.
	OnRestored func(*PublishResult)
}

// HistoryView lists the versions of a workflow.
type HistoryView struct {
	cfg      HistoryViewConfig
	shell    ShellAccess
	app      *tview.Application
	list     *widgets.SectionedList
	versions []HistoryVersion
	err      error
	busy     bool
	ctx      context.Context
	cancel   context.CancelFunc
}

var _ View = (*HistoryView)(nil)

// NewHistoryView builds the screen.
func NewHistoryView(cfg HistoryViewConfig) *HistoryView { return &HistoryView{cfg: cfg} }

// SetShell implements the shell-aware interface.
func (v *HistoryView) SetShell(s ShellAccess) { v.shell = s }

func (v *HistoryView) ID() string          { return "workflow.history." + v.cfg.Ref }
func (v *HistoryView) Title() string       { return i18n.Tf("tui.history.title", v.cfg.Ref) }
func (v *HistoryView) StatusHints() string { return i18n.T("tui.history.hints") }

func (v *HistoryView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetBorder(true).SetTitle(" " + v.Title() + " ").SetTitleAlign(tview.AlignLeft)
	v.list.SetBorderColor(theme.ActiveMode.Primary)
	v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) {
		if n, ok := it.Reference.(int); ok {
			v.showDiff(n)
		}
	})
	content.Clear()
	content.AddItem(v.list, 0, 1, true)
	v.render()
	if app != nil {
		app.SetFocus(v.list)
	}
	v.load()
}

func (v *HistoryView) Unmount() {
	if v.cancel != nil {
		v.cancel()
	}
	v.app, v.list = nil, nil
}

func (v *HistoryView) load() {
	if v.cfg.Load == nil || v.app == nil {
		return
	}
	app, ctx := v.app, v.ctx
	go func() {
		vs, err := v.cfg.Load(ctx)
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return
			}
			v.versions, v.err = vs, err
			v.render()
		})
	}()
}

// SetVersions sets the versions (tests).
func (v *HistoryView) SetVersions(vs []HistoryVersion) {
	v.versions = vs
	v.render()
}

func (v *HistoryView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && event.Rune() == 'r' && v.cfg.Restore != nil {
		if n, ok := v.currentVersion(); ok {
			v.askRestore(n)
		}
		return nil
	}
	return event
}

func (v *HistoryView) currentVersion() (int, bool) {
	if v.list == nil {
		return 0, false
	}
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return 0, false
	}
	n, ok := it.Reference.(int)
	return n, ok
}

func (v *HistoryView) render() {
	if v.list == nil {
		return
	}
	var items []widgets.SectionItem
	for _, h := range v.versions {
		main := tview.Escape(fmt.Sprintf("v%-3d %-10s %s  %s", h.Version, h.By, h.At.Local().Format("02/01 15:04"), h.Message))
		if h.Current {
			main += "  [" + theme.SuccessHex + "](" + i18n.T("tui.history.current") + ")[-]"
		}
		items = append(items, widgets.SectionItem{MainText: main, Reference: h.Version})
	}
	if len(items) == 0 {
		msg := i18n.T("tui.history.empty")
		if v.err != nil {
			msg = v.err.Error()
		}
		items = append(items, widgets.SectionItem{MainText: tview.Escape(msg), IsHeader: true})
	}
	v.list.SetItems(items)
}

func (v *HistoryView) showDiff(n int) {
	if v.cfg.Diff == nil || v.shell == nil || v.app == nil {
		return
	}
	app, ctx, sh := v.app, v.ctx, v.shell
	go func() {
		diff, err := v.cfg.Diff(ctx, n)
		app.QueueUpdateDraw(func() {
			body := ColorDiff(diff)
			switch {
			case err != nil:
				body = "[" + theme.ErrorHex + "]" + tview.Escape(err.Error()) + "[-]"
			case strings.TrimSpace(diff) == "":
				body = i18n.T("tui.history.same")
			}
			actions := []ModalAction{{Label: i18n.T("tui.catalog.edit.close"), Callback: func() {}}}
			if v.cfg.Restore != nil && !v.isCurrent(n) {
				actions = append([]ModalAction{{Label: i18n.T("tui.history.restore"), Callback: func() { v.askRestore(n) }}}, actions...)
			}
			sh.ShowScrollableModal(i18n.Tf("tui.history.diff_title", v.cfg.Ref, n), body, actions)
		})
	}()
}

func (v *HistoryView) isCurrent(n int) bool {
	for _, h := range v.versions {
		if h.Version == n {
			return h.Current
		}
	}
	return false
}

func (v *HistoryView) askRestore(n int) {
	if v.shell == nil {
		return
	}
	if v.isCurrent(n) {
		v.shell.ShowToastMsg(i18n.T("tui.history.already_current"), false)
		return
	}
	v.shell.ShowScrollableModal(i18n.Tf("tui.history.restore_title", v.cfg.Ref, n), i18n.Tf("tui.history.restore_body", n), []ModalAction{
		{Label: i18n.T("tui.history.restore"), Callback: func() { v.restore(n) }},
		{Label: i18n.T("tui.catalog.edit.cancel"), Callback: func() {}, Separator: true},
	})
}

// restore runs once at a time.
func (v *HistoryView) restore(n int) {
	if v.busy || v.cfg.Restore == nil {
		return
	}
	v.busy = true
	app, ctx, sh := v.app, v.ctx, v.shell
	run := func() {
		res, err := v.cfg.Restore(ctx, n)
		done := func() {
			v.busy = false
			if err != nil {
				sh.ShowToastMsg(err.Error(), false)
				return
			}
			if res.Queued {
				sh.ShowToastMsg(i18n.Tf("tui.publish.queued", v.cfg.Ref), true)
			} else {
				sh.ShowToastMsg(i18n.Tf("tui.history.restored", n, res.Version), true)
			}
			if v.cfg.OnRestored != nil {
				v.cfg.OnRestored(res)
			}
			v.load()
		}
		if app == nil {
			done()
			return
		}
		app.QueueUpdateDraw(done)
	}
	if app == nil {
		run()
		return
	}
	go run()
}
