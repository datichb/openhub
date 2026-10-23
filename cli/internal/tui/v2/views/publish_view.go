package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Publication screen (P2-T15, 10-tui §10): next version, validation, impact
// summary (widenings flagged), new team bricks, diff of the draft against the
// published version, mandatory message. Ctrl+S publishes once; an offline
// publication is queued and shown as such.
// ─────────────────────────────────────────────────────────────────────────────

// PublishImpact is one line of the impact summary.
type PublishImpact struct {
	Widen bool // more rights / less control
	Text  string
}

// PublishPreview is what a publication would do.
type PublishPreview struct {
	Ref string
	// Current is the published version (0: new workflow); Next the version
	// the publication creates.
	Current, Next int
	Valid         bool
	Findings      []string // « ✗ path: message »
	Impact        []PublishImpact
	New           bool // no previous version
	NewBricks     []string
	Diff          string // unified diff (document, then prompt template)
	Governance    string // « publication : tout membre »
	// Queued: a publication of this workflow already waits for the network.
	Queued bool
}

// PublishResult is the outcome of a publication.
type PublishResult struct {
	Version  int
	Queued   bool
	Warnings []string
}

// PublishViewConfig wires the publication screen.
type PublishViewConfig struct {
	Ref string
	// Load computes the preview (off the event loop).
	Load func(ctx context.Context) (*PublishPreview, error)
	// Publish publishes with message (off the event loop).
	Publish func(ctx context.Context, message string) (*PublishResult, error)
	// OnDone is called on the event loop after a publication.
	OnDone func(*PublishResult)
}

// PublishView is the publication screen, pushed on the router stack.
type PublishView struct {
	cfg   PublishViewConfig
	shell ShellAccess
	app   *tview.Application

	body  *tview.TextView
	form  *tview.Form
	input *tview.InputField
	hints *tview.TextView

	preview    *PublishPreview
	err        error
	message    string
	publishing bool
	ctx        context.Context
	cancel     context.CancelFunc
	loads      loadSlot // the read of the preview
}

var (
	_ View           = (*PublishView)(nil)
	_ InputCapturing = (*PublishView)(nil)
)

// NewPublishView builds the screen.
func NewPublishView(cfg PublishViewConfig) *PublishView { return &PublishView{cfg: cfg} }

// SetShell implements the shell-aware interface.
func (v *PublishView) SetShell(s ShellAccess) { v.shell = s }

func (v *PublishView) ID() string          { return "workflow.publish." + v.cfg.Ref }
func (v *PublishView) Title() string       { return i18n.Tf("tui.publish.title", v.cfg.Ref) }
func (v *PublishView) StatusHints() string { return i18n.T("tui.publish.hints") }

// CapturesInput keeps the keys for the message field.
func (v *PublishView) CapturesInput() bool { return v.app != nil }

func (v *PublishView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.body = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	v.body.SetBackgroundColor(theme.BgPanel)
	v.input = tview.NewInputField().SetLabel(i18n.T("tui.publish.message") + " ").SetText(v.message).
		SetChangedFunc(func(t string) { v.message = t })
	v.form = tview.NewForm().AddFormItem(v.input).
		AddButton(i18n.T("tui.publish.publish"), v.publish).
		AddButton(i18n.T("tui.catalog.edit.cancel"), v.close)
	v.form.SetBackgroundColor(theme.BgPanel)
	widgets.StyleFormButtons(v.form, 0) // « Publier »
	v.hints = tview.NewTextView().SetDynamicColors(true)
	v.hints.SetBackgroundColor(theme.BgPanel)
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.body, 0, 1, false).
		AddItem(v.form, 5, 0, true).
		AddItem(v.hints, 1, 0, false)
	root.SetBorder(true).SetBorderColor(theme.ActiveMode.Primary).SetTitle(" " + v.Title() + " ").SetTitleAlign(tview.AlignLeft)
	root.SetBackgroundColor(theme.BgPanel)
	content.Clear()
	content.AddItem(root, 0, 1, true)
	v.render()
	if app != nil {
		app.SetFocus(v.input)
	}
	v.load()
}

func (v *PublishView) Unmount() {
	if v.cancel != nil {
		v.cancel()
	}
	v.app, v.body, v.form, v.input, v.hints = nil, nil, nil, nil, nil
}

func (v *PublishView) load() {
	if v.cfg.Load == nil || v.app == nil {
		return
	}
	load(v.app, v.ctx, &v.loads, v.cfg.Load, func(p *PublishPreview, err error) {
		if v.app == nil {
			return
		}
		v.preview, v.err = p, err
		v.render()
	})
}

// SetPreview sets the preview (tests).
func (v *PublishView) SetPreview(p *PublishPreview, err error) {
	v.preview, v.err = p, err
	v.render()
}

func (v *PublishView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlS:
		v.publish()
		return nil
	case tcell.KeyEscape:
		v.close()
		return nil
	case tcell.KeyPgDn, tcell.KeyPgUp:
		if v.body != nil {
			row, col := v.body.GetScrollOffset()
			step := 10
			if event.Key() == tcell.KeyPgUp {
				step = -10
			}
			v.body.ScrollTo(max(0, row+step), col)
		}
		return nil
	}
	return event
}

func (v *PublishView) close() {
	if v.publishing {
		return
	}
	if v.shell != nil {
		v.shell.PopView()
	}
}

func (v *PublishView) toast(msg string, ok bool) {
	if v.shell != nil {
		v.shell.ShowToastMsg(msg, ok)
	}
}

// publish publishes once (Ctrl+S repeated or the button pressed again while
// publishing are ignored).
func (v *PublishView) publish() {
	if v.publishing || v.cfg.Publish == nil {
		return
	}
	msg := strings.TrimSpace(v.message)
	switch {
	case v.preview == nil:
		v.toast(i18n.T("tui.publish.loading"), false)
		return
	case !v.preview.Valid:
		v.toast(i18n.T("tui.publish.invalid"), false)
		return
	case msg == "":
		v.toast(i18n.T("tui.publish.message_required"), false)
		return
	}
	v.publishing = true
	v.render()
	app, ctx := v.app, v.ctx
	run := func() {
		res, err := v.cfg.Publish(ctx, msg)
		done := func() {
			v.publishing = false
			if err != nil {
				v.err = err
				v.render()
				v.toast(err.Error(), false)
				return
			}
			switch {
			case res.Queued:
				v.toast(i18n.Tf("tui.publish.queued", v.cfg.Ref), true)
			default:
				v.toast(i18n.Tf("tui.publish.done", v.cfg.Ref, res.Version), true)
			}
			if v.cfg.OnDone != nil {
				v.cfg.OnDone(res)
			}
			if v.shell != nil && v.body != nil { // still mounted
				v.shell.PopView()
			}
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
	safego.Go(run) // an action, not a read: its result is applied even if the view was left
}

func (v *PublishView) render() {
	if v.body == nil {
		return
	}
	v.body.SetText(PublishText(v.preview, v.err))
	hint := i18n.T("tui.publish.hints")
	if v.publishing {
		hint = "⠋ " + i18n.T("tui.publish.publishing")
	}
	v.hints.SetText(" " + muted(hint))
}

// PublishText renders a publication preview (also used for `D`).
func PublishText(p *PublishPreview, err error) string {
	var b strings.Builder
	if p == nil {
		if err != nil {
			b.WriteString(stateText(stateError, err.Error()) + "\n")
		} else {
			b.WriteString(stateText(stateLoading, i18n.T("tui.publish.loading")) + "\n")
		}
		return b.String()
	}
	from := i18n.T("tui.publish.new")
	if p.Current > 0 {
		from = fmt.Sprintf("v%d", p.Current)
	}
	fmt.Fprintf(&b, "%s%s%s  %s → v%d\n", theme.StyleTag(theme.AttrBold), tview.Escape(p.Ref), theme.TagAttrs, from, p.Next)
	if p.Governance != "" {
		b.WriteString(muted(tview.Escape(p.Governance)) + "\n")
	}
	if p.Queued {
		b.WriteString(theme.Paint(theme.WarningHex, theme.IconQueued+" "+i18n.T("tui.publish.already_queued")) + "\n")
	}
	b.WriteString("\n")
	if p.Valid {
		b.WriteString(theme.Paint(theme.SuccessHex, theme.SymSuccess+" "+i18n.T("tui.publish.valid")) + "\n")
	} else {
		b.WriteString(theme.Paint(theme.ErrorHex, theme.SymFailure+" "+i18n.T("tui.publish.invalid")) + "\n")
	}
	for _, f := range p.Findings {
		fmt.Fprintf(&b, "  %s\n", tview.Escape(f))
	}
	if err != nil {
		b.WriteString(theme.Paint(theme.ErrorHex, theme.SymFailure+" "+tview.Escape(err.Error())) + "\n")
	}

	fmt.Fprintf(&b, "\n%s─ %s%s\n", theme.StyleTag(theme.AttrBold), i18n.T("tui.publish.impact"), theme.TagAttrs)
	switch {
	case p.New:
		fmt.Fprintf(&b, "  %s\n", i18n.T("tui.publish.impact_new"))
	case len(p.Impact) == 0:
		fmt.Fprintf(&b, "  %s %s\n", theme.Paint(theme.SuccessHex, theme.SymSuccess), i18n.T("tui.publish.impact_none"))
	}
	for _, it := range p.Impact {
		if it.Widen {
			fmt.Fprintf(&b, "  %s %s\n", theme.Paint(theme.WarningHex, theme.SymWarning), tview.Escape(it.Text))
		} else {
			fmt.Fprintf(&b, "  · %s\n", tview.Escape(it.Text))
		}
	}
	if len(p.NewBricks) > 0 {
		fmt.Fprintf(&b, "  %s %s\n", theme.Paint(theme.InfoHex, theme.IconNewBrick+" "+i18n.T("tui.catalog.edit.new_brick")), tview.Escape(strings.Join(p.NewBricks, ", ")))
	}

	fmt.Fprintf(&b, "\n%s─ %s%s\n", theme.StyleTag(theme.AttrBold), i18n.T("tui.publish.diff"), theme.TagAttrs)
	if strings.TrimSpace(p.Diff) == "" {
		fmt.Fprintf(&b, "  %s\n", i18n.T("tui.publish.diff_none"))
	} else {
		b.WriteString(ColorDiff(p.Diff))
	}
	return b.String()
}

// ColorDiff colors a unified diff for a dynamic-color TextView.
func ColorDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		esc := tview.Escape(line)
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "@@"):
			b.WriteString(muted(esc) + "\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(theme.Paint(theme.SuccessHex, esc) + "\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString(theme.Paint(theme.ErrorHex, esc) + "\n")
		default:
			b.WriteString(esc + "\n")
		}
	}
	return b.String()
}
