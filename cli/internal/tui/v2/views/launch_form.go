package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ─────────────────────────────────────────────────────────────────────────────
// Launch form (P1-T22, 10-tui §4): generated from the workflow YAML — inputs,
// then options (mode, runtime with availability, location, opening), then a
// recap computed by the wiring layer (bundle, plan, warnings). Ctrl+S
// launches from any step, Ctrl+B goes back, Esc closes. Beads inputs open the
// tview picker in place.
// ─────────────────────────────────────────────────────────────────────────────

// LaunchFormConfig configures a launch form.
type LaunchFormConfig struct {
	WorkflowID string
	// Origin is shown in the title (« hub », « équipe v3 »…).
	Origin string
	Spec   *workflow.Spec
	Lang   string
	// Prefill sets input values (board: the ticket); Tickets the tickets.
	Prefill map[string]string
	Tickets []string
	// AtOptions opens the form on the options step (inputs prefilled).
	AtOptions bool

	DefaultMode    string // "" = workflow default
	DefaultRuntime string // "" = workflow default
	Runtimes       []LaunchRuntime
	// Locations: base, existing worktrees, new (first = default).
	Locations     []SelectOption
	Attach        []SelectOption
	DefaultAttach string

	// Beads is the ticket source of the picker (nil = typed ids).
	Beads     BeadsSource
	ClaimedBy func(id string) string

	// Recap computes the recap of the choices (called off the event loop).
	Recap func(ctx context.Context, c LaunchChoices) (*LaunchRecap, error)
	// Launch starts the sessions (called off the event loop). The form shows
	// « lancement en cours » until it returns; a nil error closes the form.
	Launch func(ctx context.Context, c LaunchChoices) error
	// LaunchFirst starts a suggested workflow instead (failed precondition),
	// remembering this launch when the workflow asks to come back.
	LaunchFirst func(ctx context.Context, c LaunchChoices, workflowID string) error
	// RuntimeStatus details the selected runtime outside the machine (nil =
	// none); Progress relays the preparation output while launching.
	RuntimeStatus RuntimeStatusFunc
	Progress      *LaunchProgress
}

const (
	launchStepInputs = iota
	launchStepOptions
	launchStepRecap
)

// LaunchFormView is the launch form, pushed on the router stack.
type LaunchFormView struct {
	cfg   LaunchFormConfig
	m     *launchModel
	shell ShellAccess

	app     *tview.Application
	content *tview.Flex
	header  *tview.TextView
	body    *tview.Flex
	hints   *tview.TextView
	form    *tview.Form
	picker  *BeadsPicker
	recapTV *tview.TextView

	step      int
	recap     *LaunchRecap
	recapErr  error
	recapGen  int
	launching bool
	launchErr error
	rt        runtimeState
	ctx       context.Context
	cancel    context.CancelFunc
}

var (
	_ View           = (*LaunchFormView)(nil)
	_ InputCapturing = (*LaunchFormView)(nil)
)

// NewLaunchFormView builds the form.
func NewLaunchFormView(cfg LaunchFormConfig) *LaunchFormView {
	v := &LaunchFormView{cfg: cfg, m: newLaunchModel(cfg)}
	if cfg.AtOptions {
		v.step = launchStepOptions
	}
	return v
}

// SetShell implements the shell-aware interface.
func (v *LaunchFormView) SetShell(s ShellAccess) { v.shell = s }

func (v *LaunchFormView) ID() string { return "launch." + v.cfg.WorkflowID }

func (v *LaunchFormView) Title() string { return i18n.Tf("tui.launch.title", v.cfg.WorkflowID) }

// CapturesInput keeps every key for the form while it is mounted.
func (v *LaunchFormView) CapturesInput() bool { return v.app != nil }

func (v *LaunchFormView) StatusHints() string {
	if v.picker != nil {
		return v.picker.StatusHints()
	}
	return i18n.T("tui.launch.hints")
}

func (v *LaunchFormView) Mount(content *tview.Flex, app *tview.Application) {
	v.app, v.content = app, content
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.header = tview.NewTextView().SetDynamicColors(true)
	v.header.SetBackgroundColor(theme.BgPanel)
	v.body = tview.NewFlex().SetDirection(tview.FlexRow)
	v.body.SetBackgroundColor(theme.BgPanel)
	v.hints = tview.NewTextView().SetDynamicColors(true)
	v.hints.SetBackgroundColor(theme.BgPanel)
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.header, 3, 0, false).
		AddItem(v.body, 0, 1, true).
		AddItem(v.hints, 2, 0, false)
	root.SetBorder(true).SetBorderColor(theme.ActiveMode.Primary).
		SetTitle(" " + v.Title() + " ").SetTitleAlign(tview.AlignLeft)
	root.SetBackgroundColor(theme.BgPanel)
	content.Clear()
	content.AddItem(root, 0, 1, true)
	v.render()
}

func (v *LaunchFormView) Unmount() {
	if v.cancel != nil {
		v.cancel()
	}
	v.stopProgress()
	v.app, v.content, v.form, v.picker, v.recapTV, v.rt.tv = nil, nil, nil, nil, nil, nil
}

func (v *LaunchFormView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.picker != nil {
		return event // the picker has the focus and its own keys
	}
	switch event.Key() {
	case tcell.KeyCtrlS:
		v.launch()
		return nil
	case tcell.KeyCtrlB:
		v.back()
		return nil
	case tcell.KeyEscape:
		if v.form != nil {
			if idx, _ := v.form.GetFocusedItemIndex(); idx >= 0 {
				if _, isDD := v.form.GetFormItem(idx).(*tview.DropDown); isDD {
					return event // closes the drop-down list
				}
			}
		}
		v.close()
		return nil
	}
	if v.form != nil {
		if remapped := remapArrowToTab(v.form, event); remapped != nil {
			return remapped
		}
	}
	return event
}

// ── rendering ──────────────────────────────────────────────────────────────

func (v *LaunchFormView) render() {
	if v.app == nil {
		return
	}
	v.picker, v.form, v.recapTV, v.rt.tv = nil, nil, nil, nil
	v.body.Clear()
	v.header.SetText(v.headerText())
	v.hints.SetText("[" + theme.TextMutedHex + "]" + v.StatusHints())
	switch v.step {
	case launchStepInputs:
		v.renderInputs()
	case launchStepOptions:
		v.renderOptions()
	default:
		v.renderRecap()
	}
}

func (v *LaunchFormView) headerText() string {
	steps := []string{i18n.T("tui.launch.step_inputs"), i18n.T("tui.launch.step_options"), i18n.T("tui.launch.step_recap")}
	var parts []string
	for i, s := range steps {
		mark := "○"
		if i == v.step {
			mark = "●"
		} else if i < v.step {
			mark = "✔"
		}
		parts = append(parts, mark+" "+s)
	}
	origin := ""
	if v.cfg.Origin != "" {
		origin = "   [" + theme.TextMutedHex + "]" + v.cfg.Origin + "[-]"
	}
	desc := ""
	if d := v.cfg.Spec.Description.Text(v.m.lang); d != "" {
		desc = "\n [" + theme.TextMutedHex + "]" + tview.Escape(d) + "[-]"
	}
	return " " + strings.Join(parts, " ─── ") + origin + desc
}

func (v *LaunchFormView) newForm() *tview.Form {
	f := tview.NewForm()
	f.SetBackgroundColor(theme.BgPanel)
	f.SetFieldBackgroundColor(theme.BgElement)
	f.SetFieldTextColor(theme.FgPrimary)
	f.SetLabelColor(theme.FgPrimary)
	f.SetButtonStyle(tcell.StyleDefault.Background(theme.ActiveMode.Primary).Foreground(theme.BgPanel))
	f.SetButtonActivatedStyle(tcell.StyleDefault.Background(theme.ActiveMode.Secondary).Foreground(theme.BgPanel))
	f.SetBorderPadding(1, 0, 2, 2)
	return f
}

func (v *LaunchFormView) mountForm(f *tview.Form) {
	fixFormDropDownStyles(f)
	v.form = f
	v.body.AddItem(f, 0, 1, true)
	if v.app != nil {
		v.app.SetFocus(f)
	}
}

func (v *LaunchFormView) renderInputs() {
	f := v.newForm()
	m := v.m
	for _, k := range m.spec.Inputs.Keys() {
		k := k
		in, _ := m.spec.Inputs.Get(k)
		label := m.inputLabel(k)
		switch {
		case k == m.ticketInput:
			f.AddInputField(label, strings.Join(m.tickets, ", "), 40, nil, func(t string) { m.tickets = splitTickets(t) })
			if v.cfg.Beads != nil {
				f.AddButton(i18n.T("tui.launch.pick_tickets"), func() { v.openPicker(in) })
			}
		case in.Type == workflow.InputBool:
			f.AddCheckbox(label, m.values[k] == "true", func(c bool) { m.values[k] = boolString(c) })
		case in.Type == workflow.InputEnum:
			cur := 0
			for i, val := range in.Values {
				if val == m.values[k] {
					cur = i
				}
			}
			if m.values[k] == "" && len(in.Values) > 0 {
				m.values[k] = in.Values[cur]
			}
			f.AddDropDown(label, in.Values, cur, func(opt string, _ int) { m.values[k] = opt })
		case in.Type == workflow.InputText:
			f.AddTextArea(label, m.values[k], 0, 4, 0, func(t string) { m.values[k] = t })
		default:
			field := tview.NewInputField().SetLabel(label).SetText(m.values[k]).SetFieldWidth(48)
			if d, ok := in.Default.(string); ok && strings.Contains(d, "{{") {
				field.SetPlaceholder(d)
			}
			field.SetChangedFunc(func(t string) { m.values[k] = t })
			f.AddFormItem(field)
		}
	}
	if m.spec.Inputs.Len() == 0 {
		v.body.AddItem(tview.NewTextView().SetText("  "+i18n.T("tui.launch.no_inputs")), 2, 0, false)
	}
	f.AddButton(i18n.T("tui.launch.next"), v.next)
	v.mountForm(f)
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (v *LaunchFormView) renderOptions() {
	f := v.newForm()
	m := v.m
	modes := m.spec.AllowedModes()
	f.AddDropDown(i18n.T("tui.launch.mode"), modes, indexOf(modes, m.mode), func(opt string, _ int) { m.mode = opt })
	var rts []string
	cur := 0
	for i, r := range v.cfg.Runtimes {
		label := r.Label
		if !r.Available {
			label += "  ✗ " + r.Reason
		}
		rts = append(rts, label)
		if r.Kind == m.runtime {
			cur = i
		}
	}
	if len(rts) > 0 {
		f.AddDropDown(i18n.T("tui.launch.runtime"), rts, cur, func(_ string, i int) {
			if i >= 0 && i < len(v.cfg.Runtimes) {
				m.runtime = v.cfg.Runtimes[i].Kind
				v.loadRuntimeStatus()
			}
		})
		v.runtimeStatusLine()
	}
	if len(v.cfg.Locations) > 0 {
		labels, values := selectLists(v.cfg.Locations)
		f.AddDropDown(i18n.T("tui.launch.location"), labels, indexOf(values, m.location), func(_ string, i int) {
			if i >= 0 && i < len(values) {
				m.location = values[i]
			}
		})
	}
	if len(v.cfg.Attach) > 0 {
		labels, values := selectLists(v.cfg.Attach)
		f.AddDropDown(i18n.T("tui.launch.attach"), labels, max(indexOf(values, m.attach), 0), func(_ string, i int) {
			if i >= 0 && i < len(values) {
				m.attach = values[i]
			}
		})
	}
	if m.perSession && len(m.tickets) > 1 {
		f.AddCheckbox(i18n.T("tui.launch.one_session"), m.oneSession, func(c bool) { m.oneSession = c; v.render() })
	}
	if line := m.sessionsLine(); line != "" {
		v.body.AddItem(tview.NewTextView().SetDynamicColors(true).SetText("  ["+theme.TextMutedHex+"]"+line), 1, 0, false)
	}
	f.AddButton(i18n.T("tui.launch.back"), v.back)
	f.AddButton(i18n.T("tui.launch.next"), v.next)
	v.mountForm(f)
}

func (v *LaunchFormView) renderRecap() {
	tv := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetBorderPadding(1, 0, 2, 2)
	v.recapTV = tv
	tv.SetText(v.recapText())
	v.body.AddItem(tv, 0, 1, false)
	f := v.newForm()
	f.AddButton(i18n.T("tui.launch.back"), v.back)
	f.AddButton(i18n.T("tui.launch.launch"), v.launch)
	if v.recap != nil && v.cfg.LaunchFirst != nil {
		for _, s := range v.recap.Suggestions {
			id := s.WorkflowID
			f.AddButton(s.Label, func() {
				v.launchWith(func(ctx context.Context, c LaunchChoices) error { return v.cfg.LaunchFirst(ctx, c, id) })
			})
		}
	}
	f.SetButtonsAlign(tview.AlignLeft)
	v.form = f
	fixFormDropDownStyles(f)
	v.body.AddItem(f, 3, 0, true)
	if v.app != nil {
		v.app.SetFocus(f)
	}
}

// recapText renders the recap step.
func (v *LaunchFormView) recapText() string {
	var b strings.Builder
	muted := "[" + theme.TextMutedHex + "]"
	switch {
	case v.launching:
		b.WriteString("⠋ " + i18n.T("tui.launch.launching") + "\n")
		b.WriteString(v.progressText())
	case v.recap == nil && v.recapErr == nil:
		b.WriteString("⠋ " + i18n.T("tui.launch.preparing") + "\n")
	}
	if v.recapErr != nil {
		b.WriteString("[" + theme.ErrorHex + "]✗ " + tview.Escape(v.recapErr.Error()) + "[-]\n")
	}
	if v.launchErr != nil {
		b.WriteString("[" + theme.ErrorHex + "]✗ " + tview.Escape(v.launchErr.Error()) + "[-]\n")
	}
	if v.recap != nil {
		for _, r := range v.recap.Rows {
			fmt.Fprintf(&b, "%s%-14s[-] %s\n", muted, tview.Escape(r.Label), tview.Escape(r.Value))
		}
		for _, w := range v.recap.Warnings {
			b.WriteString("[" + theme.WarningHex + "]⚠ " + tview.Escape(w) + "[-]\n")
		}
	}
	return b.String()
}

// loadRecap computes the recap off the event loop.
func (v *LaunchFormView) loadRecap() {
	v.recap, v.recapErr = nil, nil
	if v.cfg.Recap == nil {
		v.recap = &LaunchRecap{}
		return
	}
	v.recapGen++
	gen, app, ctx, choices := v.recapGen, v.app, v.ctx, v.m.choices()
	go func() {
		r, err := v.cfg.Recap(ctx, choices)
		if app == nil || ctx.Err() != nil {
			return
		}
		app.QueueUpdateDraw(func() {
			if v.recapGen != gen {
				return
			}
			v.recap, v.recapErr = r, err
			if r != nil && len(r.Suggestions) > 0 && v.step == launchStepRecap {
				v.render() // suggestion buttons
				return
			}
			if v.recapTV != nil {
				v.recapTV.SetText(v.recapText())
			}
		})
	}()
}

// ── navigation ─────────────────────────────────────────────────────────────

func (v *LaunchFormView) next() {
	if v.step == launchStepInputs && v.blocked() {
		return
	}
	if v.step < launchStepRecap {
		v.step++
	}
	if v.step == launchStepRecap {
		if v.blocked() {
			v.step = launchStepOptions
			return
		}
		v.loadRecap()
	}
	v.render()
}

func (v *LaunchFormView) back() {
	if v.launching || v.step == launchStepInputs {
		return
	}
	v.step--
	v.render()
}

// blocked shows the validation problems; it reports whether there are any.
func (v *LaunchFormView) blocked() bool {
	problems := v.m.validate()
	if len(problems) == 0 {
		return false
	}
	if v.shell != nil {
		v.shell.ShowToastMsg(strings.Join(problems, " · "), false)
	}
	return true
}

func (v *LaunchFormView) launch() { v.launchWith(v.cfg.Launch) }

// launchWith runs a launch function off the event loop (double launch
// protection: ignored while a launch is in progress, m12).
func (v *LaunchFormView) launchWith(fn func(ctx context.Context, c LaunchChoices) error) {
	if v.launching || v.blocked() || fn == nil {
		return
	}
	v.launching, v.launchErr = true, nil
	if v.step != launchStepRecap {
		v.step = launchStepRecap
		v.render()
	} else if v.recapTV != nil {
		v.recapTV.SetText(v.recapText())
	}
	app, ctx, choices := v.app, v.ctx, v.m.choices()
	v.listenProgress()
	go func() {
		err := fn(ctx, choices)
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			v.stopProgress()
			v.launching = false
			if err == nil {
				v.close()
				return
			}
			v.launchErr = err
			if v.recapTV != nil {
				v.recapTV.SetText(v.recapText())
			}
		})
	}()
}

func (v *LaunchFormView) close() {
	if v.launching {
		return
	}
	if v.shell != nil {
		v.shell.PopView()
	}
}

// ── Beads picker ───────────────────────────────────────────────────────────

func (v *LaunchFormView) openPicker(in workflow.Input) {
	cfg := BeadsPickerConfig{Source: v.cfg.Beads, Preselected: v.m.tickets, ClaimedBy: v.cfg.ClaimedBy}
	cfg.ApplyInput(in)
	cfg.OnDone = func(ids []string) {
		v.m.tickets = ids
		v.render()
	}
	cfg.OnCancel = v.render
	v.body.Clear()
	v.form = nil
	v.picker = NewBeadsPicker(v.ctx, v.app, cfg)
	v.body.AddItem(v.picker, 0, 1, true)
	v.hints.SetText("[" + theme.TextMutedHex + "]" + v.picker.StatusHints())
	if v.app != nil {
		v.app.SetFocus(v.picker)
	}
	v.picker.Load()
}

func selectLists(opts []SelectOption) (labels, values []string) {
	for _, o := range opts {
		labels = append(labels, o.Label)
		values = append(values, o.Value)
	}
	return labels, values
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return 0
}
