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
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ─────────────────────────────────────────────────────────────────────────────
// Workflow editor (P2-T14, 10-tui §9): five sections — General, Graph,
// Inputs & prompt, Resources, Bundle preview — over the draft's YAML text.
// Every change goes through workflow.DocEdit (comments and field presence
// kept), is checked off the event loop (resolution, origins, locks,
// diagnostics, prompt preview, bundle) and can be undone (u / U). y edits
// the raw YAML in $EDITOR, w saves the draft, Esc asks before dropping
// unsaved changes.
// ─────────────────────────────────────────────────────────────────────────────

// EditorDiag is a finding of the editor's check.
type EditorDiag struct {
	Error   bool
	Path    string // field path ("agents.developer.after")
	Line    int    // line in the edited document (0: unknown / another document)
	Message string
	Hint    string
}

// EditorCheck is the result of a check of the edited text.
type EditorCheck struct {
	// Spec is the resolved workflow (nil when it cannot be resolved).
	Spec *workflow.Spec
	// Origins gives the document that set each path of Spec.
	Origins workflow.Origins
	// Locked are the fields locked by the parent chain ("*" = all).
	Locked      []string
	Diagnostics []EditorDiag
	// Prompt is the initial prompt rendered with example values.
	Prompt    string
	PromptErr string
	// Bundle summarises the bundle built from the draft.
	Bundle    []string
	BundleErr string
}

// Errors counts the errors.
func (c *EditorCheck) Errors() int {
	n := 0
	for _, d := range c.Diagnostics {
		if d.Error {
			n++
		}
	}
	return n
}

func (c *EditorCheck) locked(field string) bool {
	if c == nil {
		return false
	}
	for _, f := range c.Locked {
		if f == field || f == workflow.EnforceAll {
			return true
		}
	}
	return false
}

// EditorSaved is the result of a save.
type EditorSaved struct {
	Ref    string
	Pushed bool
}

// WorkflowEditorConfig wires the editor.
type WorkflowEditorConfig struct {
	// Ref is the draft reference (layer:id); Layer its layer.
	Ref   string
	Layer workflow.Layer
	// YAML is the text to edit; Prompt the prompt template it uses.
	YAML   []byte
	Prompt []byte
	// Unsaved: the text is not a saved draft yet (new workflow).
	Unsaved bool
	// PromptOwn: Prompt must be saved as the draft's own template (copy).
	PromptOwn bool
	Lang      string
	// Agents lists the catalogue agents (choices).
	Agents []string
	// Check validates the text (off the event loop).
	Check func(ctx context.Context, yaml, prompt []byte) (*EditorCheck, error)
	// Save saves the draft (off the event loop); prompt nil keeps the
	// current template.
	Save func(ctx context.Context, yaml, prompt []byte) (*EditorSaved, error)
	// External edits content in $EDITOR (terminal suspended), at line when
	// > 0; it runs on the event loop.
	External func(name string, content []byte, line int) ([]byte, error)
	// OnSaved is called on the event loop after a save.
	OnSaved func(*EditorSaved)
}

// Editor sections.
const (
	edGeneral = iota
	edGraph
	edInputs
	edResources
	edPreview
	edSections
)

var editorSectionKeys = [edSections]string{"tui.editor.section.general", "tui.editor.section.graph",
	"tui.editor.section.inputs", "tui.editor.section.resources", "tui.editor.section.preview"}

// WorkflowEditorView is the editor, pushed on the router stack.
type WorkflowEditorView struct {
	cfg   WorkflowEditorConfig
	m     *editorModel
	shell ShellAccess
	app   *tview.Application

	root    *tview.Flex
	tabs    *tview.TextView
	body    *tview.Flex
	status  *tview.TextView
	list    *widgets.SectionedList
	graph   *widgets.WorkflowGraph
	detail  *tview.TextView
	section int
	mode    string // mode shown by the graph

	check    *EditorCheck
	checkErr error
	checkGen uint64
	checking bool
	saving   bool
	timer    *time.Timer
	ctx      context.Context
	cancel   context.CancelFunc
	// focusPath is selected after the next render (diagnostic jump).
	focusPath string
}

var (
	_ View           = (*WorkflowEditorView)(nil)
	_ InputCapturing = (*WorkflowEditorView)(nil)
)

// NewWorkflowEditorView builds the editor.
func NewWorkflowEditorView(cfg WorkflowEditorConfig) *WorkflowEditorView {
	v := &WorkflowEditorView{cfg: cfg, m: newEditorModel(cfg.YAML, cfg.Prompt, cfg.Unsaved)}
	v.m.promptChanged = cfg.PromptOwn
	return v
}

// SetShell implements the shell-aware interface.
func (v *WorkflowEditorView) SetShell(s ShellAccess) { v.shell = s }

func (v *WorkflowEditorView) ID() string    { return "workflow.edit." + v.cfg.Ref }
func (v *WorkflowEditorView) Title() string { return i18n.Tf("tui.editor.title", v.cfg.Ref) }
func (v *WorkflowEditorView) StatusHints() string {
	return i18n.T("tui.editor.hints." + [edSections]string{"general", "graph", "inputs", "resources", "preview"}[v.section])
}

// CapturesInput keeps the editor keys while it is mounted.
func (v *WorkflowEditorView) CapturesInput() bool { return v.app != nil }

// Dirty reports unsaved changes.
func (v *WorkflowEditorView) Dirty() bool { return v.m.dirty() }

func (v *WorkflowEditorView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.tabs = tview.NewTextView().SetDynamicColors(true)
	v.tabs.SetBackgroundColor(theme.BgPanel)
	v.body = tview.NewFlex() // the editor's own container: content keeps its direction (B10)
	v.body.SetBackgroundColor(theme.BgPanel)
	v.status = tview.NewTextView().SetDynamicColors(true)
	v.status.SetBackgroundColor(theme.BgPanel)
	v.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.tabs, 1, 0, false).
		AddItem(v.body, 0, 1, true).
		AddItem(v.status, 1, 0, false)
	v.root.SetBorder(true).SetBorderColor(theme.ActiveMode.Primary).SetTitle(" " + v.Title() + " ").SetTitleAlign(tview.AlignLeft)
	v.root.SetBackgroundColor(theme.BgPanel)
	content.Clear()
	content.AddItem(v.root, 0, 1, true)
	v.render()
	v.recheck(0)
}

func (v *WorkflowEditorView) Unmount() {
	if v.cancel != nil {
		v.cancel()
	}
	if v.timer != nil {
		v.timer.Stop()
	}
	v.checkGen++
	v.app, v.root, v.tabs, v.body, v.status, v.list, v.graph, v.detail = nil, nil, nil, nil, nil, nil, nil, nil
}

// ── checking ───────────────────────────────────────────────────────────────

// recheck checks the text after delay (edits in a row are checked once).
func (v *WorkflowEditorView) recheck(delay time.Duration) {
	if v.cfg.Check == nil {
		return
	}
	v.checkGen++
	gen := v.checkGen
	yaml, prompt := v.m.cur.yaml, v.m.cur.prompt
	app, ctx := v.app, v.ctx
	v.checking = true
	run := func() {
		chk, err := v.cfg.Check(ctx, yaml, prompt)
		apply := func() {
			if v.checkGen != gen {
				return
			}
			v.check, v.checkErr, v.checking = chk, err, false
			v.render()
		}
		if app == nil {
			apply()
			return
		}
		app.QueueUpdateDraw(func() {
			if v.app != nil {
				apply()
			}
		})
	}
	if v.timer != nil {
		v.timer.Stop()
	}
	if app == nil {
		run()
		return
	}
	v.timer = time.AfterFunc(delay, run)
}

// SetCheck sets the check result (tests).
func (v *WorkflowEditorView) SetCheck(c *EditorCheck) {
	v.checkGen++
	v.check, v.checking = c, false
	v.render()
}

// changed is called after every edit.
func (v *WorkflowEditorView) changed() {
	v.render()
	v.recheck(150 * time.Millisecond)
}

func (v *WorkflowEditorView) toast(msg string, ok bool) {
	if v.shell != nil {
		v.shell.ShowToastMsg(msg, ok)
	}
}

// edit applies fn to the document and rechecks.
func (v *WorkflowEditorView) edit(fn func(*workflow.DocEdit) error) {
	if err := v.m.edit(fn); err != nil {
		v.toast(err.Error(), false)
		return
	}
	v.changed()
}

// ── keys ───────────────────────────────────────────────────────────────────

func (v *WorkflowEditorView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		v.setSection((v.section + 1) % edSections)
		return nil
	case tcell.KeyBacktab:
		v.setSection((v.section + edSections - 1) % edSections)
		return nil
	case tcell.KeyEscape:
		v.close()
		return nil
	case tcell.KeyCtrlS:
		v.save()
		return nil
	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		v.removeCurrent()
		return nil
	case tcell.KeyRune:
	default:
		return event
	}
	switch event.Rune() {
	case 'u':
		if v.m.undoOnce() {
			v.changed()
		}
		return nil
	case 'U':
		if v.m.redoOnce() {
			v.changed()
		}
		return nil
	case 'y':
		v.editRaw(0)
		return nil
	case 'w':
		v.save()
		return nil
	case 'x':
		v.removeCurrent()
		return nil
	case 'a':
		v.add()
		return nil
	case 'c':
		if v.section == edGraph {
			v.addCheckpoint()
			return nil
		}
	case 'm':
		if v.section == edGraph {
			v.cycleMode()
			return nil
		}
	case 'P':
		v.editPrompt()
		return nil
	}
	return event
}

func (v *WorkflowEditorView) setSection(s int) {
	v.section = s
	v.render()
}

// close leaves the editor, asking first when there are unsaved changes.
func (v *WorkflowEditorView) close() {
	if v.saving || v.shell == nil {
		return
	}
	if !v.m.dirty() {
		v.shell.PopView()
		return
	}
	var actions []ModalAction
	if v.check != nil && v.check.Errors() == 0 && v.check.Spec != nil {
		actions = append(actions, ModalAction{Label: i18n.T("tui.editor.guard.save"), Callback: func() { v.saveThen(func() { v.shell.PopView() }) }})
	}
	actions = append(actions,
		ModalAction{Label: i18n.T("tui.editor.guard.discard"), Callback: func() { v.shell.PopView() }},
		ModalAction{Label: i18n.T("tui.editor.guard.continue"), Callback: func() {}, Separator: true})
	body := i18n.T("tui.editor.guard.body")
	if len(actions) == 2 {
		body += "\n\n" + i18n.T("tui.editor.guard.invalid")
	}
	v.shell.ShowScrollableModal(i18n.T("tui.editor.guard.title"), body, actions)
}

// ── saving ─────────────────────────────────────────────────────────────────

func (v *WorkflowEditorView) save() { v.saveThen(nil) }

func (v *WorkflowEditorView) saveThen(then func()) {
	if v.saving || v.cfg.Save == nil {
		return
	}
	if v.check != nil && v.check.Errors() > 0 {
		v.toast(i18n.Tf("tui.editor.save_invalid", v.check.Errors()), false)
		v.setSection(edPreview)
		return
	}
	v.saving = true
	v.render()
	snap, prompt := v.m.cur, v.m.savePrompt()
	app, ctx := v.app, v.ctx
	run := func() {
		res, err := v.cfg.Save(ctx, snap.yaml, prompt)
		done := func() {
			v.saving = false
			if err != nil {
				v.toast(err.Error(), false)
				v.render()
				return
			}
			v.m.saved, v.m.unsaved = snap, false
			if prompt != nil && snap.equal(v.m.cur) {
				v.m.promptChanged = false
			}
			msg := i18n.Tf("tui.catalog.edit.saved", res.Ref)
			if !res.Pushed {
				msg = i18n.Tf("tui.catalog.edit.saved_local", res.Ref)
			}
			v.toast(msg, true)
			if v.cfg.OnSaved != nil {
				v.cfg.OnSaved(res)
			}
			v.render()
			if then != nil {
				then()
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
	go run()
}

// ── external editor ────────────────────────────────────────────────────────

// editRaw opens the YAML in $EDITOR (at line when > 0).
func (v *WorkflowEditorView) editRaw(line int) {
	if v.cfg.External == nil {
		return
	}
	out, err := v.cfg.External(v.cfg.Ref[strings.LastIndex(v.cfg.Ref, ":")+1:]+".yaml", v.m.cur.yaml, line)
	if err != nil {
		v.toast(err.Error(), false)
		return
	}
	v.m.setYAML(out)
	v.changed()
}

// editPrompt opens the prompt template in $EDITOR; it becomes the draft's
// own template.
func (v *WorkflowEditorView) editPrompt() {
	if v.cfg.External == nil {
		return
	}
	out, err := v.cfg.External("prompt.md.tmpl", v.m.cur.prompt, 0)
	if err != nil {
		v.toast(err.Error(), false)
		return
	}
	v.m.setPrompt(out)
	if doc := v.m.document(); doc == nil || !doc.Has("prompt") {
		// The draft's own template replaces the template of the document
		// that writes `prompt`: this one must write it (same path as the
		// inherited one, else named after the workflow).
		path := "prompts/" + v.cfg.Ref[strings.LastIndex(v.cfg.Ref, ":")+1:] + ".md.tmpl"
		if sp := v.spec(); sp != nil && sp.Prompt != nil && sp.Prompt.Template != "" {
			path = sp.Prompt.Template
		}
		_ = v.m.edit(func(e *workflow.DocEdit) error { return e.Set(path, "prompt", "template") })
	}
	v.changed()
}

// ── rendering ──────────────────────────────────────────────────────────────

func (v *WorkflowEditorView) render() {
	if v.root == nil {
		return
	}
	var tabs []string
	for i, k := range editorSectionKeys {
		label := i18n.T(k)
		if i == edPreview && v.check != nil && v.check.Errors() > 0 {
			label += fmt.Sprintf(" [%s]✗%d[-]", theme.ErrorHex, v.check.Errors())
		}
		if i == v.section {
			label = "[" + theme.ActiveMode.AccentHex + "::b][ " + label + " ][-::-]"
		}
		tabs = append(tabs, label)
	}
	v.tabs.SetText(" " + strings.Join(tabs, "  "))
	v.status.SetText(" " + v.statusLine())
	v.renderBody()
}

func (v *WorkflowEditorView) statusLine() string {
	var parts []string
	if v.m.dirty() {
		parts = append(parts, "["+theme.WarningHex+"]✎ "+i18n.T("tui.editor.modified")+"[-]")
	} else {
		parts = append(parts, "["+theme.TextMutedHex+"]"+i18n.T("tui.editor.saved")+"[-]")
	}
	switch {
	case v.saving:
		parts = append(parts, "⠋ "+i18n.T("tui.editor.saving"))
	case v.checking && v.check == nil:
		parts = append(parts, "⠋ "+i18n.T("tui.editor.checking"))
	case v.checkErr != nil:
		parts = append(parts, "["+theme.ErrorHex+"]✗ "+tview.Escape(v.checkErr.Error())+"[-]")
	case v.check != nil && v.check.Errors() > 0:
		parts = append(parts, "["+theme.ErrorHex+"]✗ "+i18n.Tf("tui.catalog.edit.errors", v.check.Errors())+"[-]")
	case v.check != nil:
		parts = append(parts, "["+theme.SuccessHex+"]✔ "+i18n.T("tui.catalog.valid")+"[-]")
	}
	parts = append(parts, "["+theme.TextMutedHex+"]"+i18n.T("tui.editor.keys")+"[-]")
	return strings.Join(parts, " · ")
}

// selectedRef is the reference of the selected list item.
func (v *WorkflowEditorView) selectedRef() any {
	if v.list == nil {
		return nil
	}
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return nil
	}
	return it.Reference
}

func (v *WorkflowEditorView) renderBody() {
	var keep any
	keepIdx := -1
	if v.list != nil {
		keep = v.selectedRef()
		keepIdx, _, _ = v.list.CurrentItem()
	}
	graphSel := ""
	if v.graph != nil {
		if sel := v.graph.SelectedElement(); sel != nil {
			graphSel = sel.ID
		}
	}
	v.body.Clear()
	v.body.SetDirection(tview.FlexColumn)
	v.list, v.graph, v.detail = nil, nil, nil
	switch v.section {
	case edGraph:
		v.renderGraph(graphSel)
		return
	case edPreview:
		v.renderPreview()
	default:
		v.list = v.newList()
		v.list.SetItems(v.fieldItems())
		v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) { v.activate(it.Reference) })
		v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showFieldDetail(it.Reference) })
		v.detail = v.newDetail()
		if v.section == edInputs {
			flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(v.list, 0, 1, true).AddItem(v.detail, 0, 1, false)
			v.body.AddItem(flex, 0, 1, true)
		} else {
			v.body.AddItem(v.list, 0, 3, true).AddItem(v.detail, 0, 2, false)
		}
	}
	if v.list == nil {
		return
	}
	v.restoreSelection(keep, keepIdx)
	if v.app != nil {
		v.app.SetFocus(v.list)
	}
}

func (v *WorkflowEditorView) restoreSelection(keep any, idx int) {
	items := v.list.GetItems()
	if v.focusPath != "" {
		for i, it := range items {
			if f, ok := it.Reference.(*edField); ok && strings.Join(f.path, ".") == v.focusPath {
				v.list.SelectIndex(i)
				v.focusPath = ""
				v.showFieldDetail(it.Reference)
				return
			}
		}
		v.focusPath = ""
	}
	for i, it := range items {
		if sameRef(it.Reference, keep) {
			v.list.SelectIndex(i)
			v.showFieldDetail(it.Reference)
			return
		}
	}
	if idx >= 0 && idx < len(items) {
		v.list.SelectIndex(idx)
	}
	if _, it, ok := v.list.CurrentItem(); ok {
		v.showFieldDetail(it.Reference)
	}
}

func sameRef(a, b any) bool {
	switch x := a.(type) {
	case *edField:
		y, ok := b.(*edField)
		return ok && strings.Join(x.path, ".") == strings.Join(y.path, ".") && x.label == y.label
	case EditorDiag:
		y, ok := b.(EditorDiag)
		return ok && x == y
	}
	return false
}

func (v *WorkflowEditorView) newList() *widgets.SectionedList {
	l := widgets.NewSectionedList()
	l.SetApp(v.app)
	l.SetTabCaptureDisabled(true)
	l.SetBorder(false)
	return l
}

func (v *WorkflowEditorView) newDetail() *tview.TextView {
	d := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	d.SetBackgroundColor(theme.BgPanel)
	d.SetBorder(true).SetBorderColor(theme.FgMuted).SetTitleAlign(tview.AlignLeft)
	d.SetBorderPadding(0, 0, 1, 1)
	return d
}

// spec is the resolved workflow of the last check (nil before it).
func (v *WorkflowEditorView) spec() *workflow.Spec {
	if v.check == nil {
		return nil
	}
	return v.check.Spec
}

// originLabel names the document that set path ("" when this one).
func (v *WorkflowEditorView) originLabel(path string) string {
	if v.check == nil || v.check.Origins == nil {
		return ""
	}
	o, ok := v.check.Origins.Of(path)
	if !ok {
		return i18n.T("tui.editor.origin.default")
	}
	ref := string(o.Layer) + ":" + o.ID
	if ref == v.cfg.Ref {
		return ""
	}
	if o.Version > 0 {
		ref += fmt.Sprintf(" v%d", o.Version)
	}
	return ref
}

// ── graph section ──────────────────────────────────────────────────────────

func (v *WorkflowEditorView) graphMode() string {
	sp := v.spec()
	if sp == nil {
		return v.mode
	}
	allowed := sp.AllowedModes()
	for _, m := range allowed {
		if m == v.mode {
			return m
		}
	}
	return sp.DefaultMode()
}

func (v *WorkflowEditorView) cycleMode() {
	sp := v.spec()
	if sp == nil {
		return
	}
	allowed := sp.AllowedModes()
	cur := v.graphMode()
	for i, m := range allowed {
		if m == cur {
			v.mode = allowed[(i+1)%len(allowed)]
		}
	}
	v.render()
}

func (v *WorkflowEditorView) graphModel() widgets.GraphModel {
	return widgets.SpecGraphModel(v.spec(), v.graphMode(), v.cfg.Lang, i18n.T("tui.editor.graph.start"), func(path string) string {
		if l := v.originLabel(path); l != "" {
			return "↳ " + l
		}
		return ""
	})
}

func (v *WorkflowEditorView) renderGraph(prev string) {
	v.graph = widgets.NewModelGraph(v.graphModel(), false)
	v.graph.SetBorder(true).SetBorderColor(theme.FgMuted).
		SetTitle(" " + i18n.Tf("tui.editor.graph.mode", v.graphMode()) + " ").SetTitleAlign(tview.AlignLeft)
	if prev != "" {
		v.graph.Select(prev)
	} else if v.focusPath != "" {
		parts := strings.SplitN(v.focusPath, ".", 3)
		if len(parts) > 1 {
			v.graph.Select(parts[1])
		}
		v.focusPath = ""
	}
	v.graph.SetOnChange(v.showGraphDetail)
	v.graph.SetOnSelect(func(e widgets.GraphElement) { v.editGraphElement(e) })
	v.detail = v.newDetail()
	v.body.AddItem(v.graph, 0, 7, true).AddItem(v.detail, 0, 3, false)
	v.showGraphDetail()
	if v.app != nil {
		v.app.SetFocus(v.graph)
	}
}

func (v *WorkflowEditorView) showGraphDetail() {
	if v.detail == nil || v.graph == nil {
		return
	}
	sel := v.graph.SelectedElement()
	sp := v.spec()
	if sel == nil || sp == nil {
		v.detail.SetText(i18n.T("tui.editor.graph.empty"))
		return
	}
	var b strings.Builder
	muted := "[" + theme.TextMutedHex + "]"
	row := func(k, val, path string) {
		if val == "" {
			val = "—"
		}
		o := v.originLabel(path)
		if o != "" {
			o = " " + muted + "← " + tview.Escape(o) + "[-]"
		}
		fmt.Fprintf(&b, "%s%-11s[-] %s%s\n", muted, k, tview.Escape(val), o)
	}
	switch sel.Type {
	case widgets.ElementCheckpoint:
		cp, _ := sp.Checkpoints.Get(sel.ID)
		v.detail.SetTitle(" ◆ " + sel.ID + " ")
		p := "checkpoints." + sel.ID
		row(i18n.T("tui.editor.field.label"), cp.Label.Text(v.cfg.Lang), p+".label")
		for _, m := range sp.AllowedModes() {
			bh := string(cp.Mode[m])
			if bh == "" {
				bh = "pause"
			}
			row(m, bh, p+".mode."+m)
		}
		row(i18n.T("tui.editor.field.condition"), cp.Condition, p+".condition")
		row(i18n.T("tui.editor.field.remote"), string(cp.Remote), p+".remote")
		row(i18n.T("tui.editor.field.mandatory"), boolText(cp.Mandatory), p+".mandatory")
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		a, declared := sp.Agents.Get(sel.ID)
		v.detail.SetTitle(" " + sel.ID + " ")
		p := "agents." + sel.ID
		if !declared {
			fmt.Fprintf(&b, "%s%s[-]\n", muted, i18n.T("tui.editor.graph.implicit_entry"))
		}
		row(i18n.T("tui.editor.field.role"), string(a.Role), p+".role")
		row(i18n.T("tui.editor.field.agent_mode"), string(a.Mode), p+".mode")
		row(i18n.T("tui.editor.field.after"), a.After, p+".after")
		row(i18n.T("tui.editor.field.calls"), strings.Join(a.Calls, ", "), p+".calls")
		if sel.ID == sp.EntryAgent() {
			fmt.Fprintf(&b, "\n▶ %s\n", i18n.T("tui.editor.graph.entry"))
		}
	case widgets.ElementStart:
		v.detail.SetTitle(" ▶ ")
		b.WriteString(i18n.T("tui.editor.graph.start_help"))
	case widgets.ElementEdge:
		v.detail.SetTitle(" → ")
		b.WriteString(i18n.T("tui.editor.graph.edge_help"))
	}
	b.WriteString("\n" + muted + i18n.T("tui.editor.graph.help") + "[-]")
	v.detail.SetText(b.String())
}

func boolText(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "true"
	}
	return "false"
}

// ── preview section ────────────────────────────────────────────────────────

func (v *WorkflowEditorView) renderPreview() {
	summary := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	summary.SetBackgroundColor(theme.BgPanel)
	summary.SetBorder(true).SetBorderColor(theme.FgMuted).SetTitle(" " + i18n.T("tui.editor.preview.bundle") + " ").SetTitleAlign(tview.AlignLeft)
	summary.SetBorderPadding(0, 0, 1, 1)
	var b strings.Builder
	switch {
	case v.check == nil:
		b.WriteString(i18n.T("tui.editor.checking"))
	case v.check.BundleErr != "":
		fmt.Fprintf(&b, "[%s]✗ %s[-]\n", theme.ErrorHex, tview.Escape(v.check.BundleErr))
	case len(v.check.Bundle) == 0:
		b.WriteString(i18n.T("tui.editor.preview.no_bundle"))
	}
	if v.check != nil {
		for _, l := range v.check.Bundle {
			b.WriteString(tview.Escape(l) + "\n")
		}
	}
	summary.SetText(b.String())
	v.list = v.newList()
	v.list.SetBorder(true).SetBorderColor(theme.FgMuted).SetTitle(" " + i18n.T("tui.editor.preview.diagnostics") + " ").SetTitleAlign(tview.AlignLeft)
	var items []widgets.SectionItem
	if v.check != nil {
		for _, d := range v.check.Diagnostics {
			icon := "[" + theme.WarningHex + "]⚠[-]"
			if d.Error {
				icon = "[" + theme.ErrorHex + "]✗[-]"
			}
			main := icon + " " + tview.Escape(d.Message)
			sec := d.Path
			if d.Hint != "" {
				sec = strings.TrimSpace(sec + "  " + d.Hint)
			}
			items = append(items, widgets.SectionItem{MainText: main, SecondaryText: tview.Escape(sec), Reference: d})
		}
	}
	if len(items) == 0 {
		items = append(items, widgets.SectionItem{MainText: "[" + theme.SuccessHex + "]✔[-] " + i18n.T("tui.editor.preview.no_diagnostics"), IsHeader: true})
	}
	v.list.SetItems(items)
	v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) {
		if d, ok := it.Reference.(EditorDiag); ok {
			v.jumpTo(d)
		}
	})
	v.body.SetDirection(tview.FlexRow)
	v.body.AddItem(summary, 0, 1, false).AddItem(v.list, 0, 1, true)
}

// jumpTo shows the field a diagnostic points to: its section and field, or
// the raw YAML at its line.
func (v *WorkflowEditorView) jumpTo(d EditorDiag) {
	top, _, _ := strings.Cut(d.Path, ".")
	top, _, _ = strings.Cut(top, "[")
	section := -1
	switch top {
	case "agents", "checkpoints":
		section = edGraph
	case "inputs", "prompt":
		section = edInputs
	case "skills", "mcp", "beads", "plugins", "outputs":
		section = edResources
	case "":
	default:
		for _, f := range v.generalFields() {
			if f.lock == top {
				section = edGeneral
			}
		}
	}
	if section < 0 {
		v.editRaw(d.Line)
		return
	}
	v.focusPath = fieldPathOf(d.Path)
	v.setSection(section)
}

// fieldPathOf maps a diagnostic path to the editor field path.
func fieldPathOf(path string) string {
	parts := strings.Split(path, ".")
	switch parts[0] {
	case "inputs":
		if len(parts) > 1 {
			return "inputs." + parts[1]
		}
	case "agents", "checkpoints":
		return path
	}
	return path
}

// ── field lists (general, inputs, resources) ───────────────────────────────

// fieldItems lists the fields of the current section.
func (v *WorkflowEditorView) fieldItems() []widgets.SectionItem {
	var fields []*edField
	switch v.section {
	case edGeneral:
		fields = v.generalFields()
	case edInputs:
		fields = v.inputFields()
	case edResources:
		fields = v.resourceFields()
	}
	doc := v.m.document()
	var items []widgets.SectionItem
	for _, f := range fields {
		if f.header {
			items = append(items, widgets.SectionItem{MainText: f.label, IsHeader: true})
			continue
		}
		items = append(items, widgets.SectionItem{MainText: v.fieldMain(f, doc), SecondaryText: v.fieldSecondary(f, doc),
			Locked: f.lock != "" && v.check.locked(f.lock), Reference: f})
	}
	return items
}

func (v *WorkflowEditorView) fieldMain(f *edField, doc *workflow.Document) string {
	val := ""
	if f.value != nil {
		val = f.value(v.spec())
	}
	if val == "" {
		val = "—"
	}
	mark := "  "
	if f.action == nil && len(f.path) > 0 && doc != nil && doc.Has(strings.Join(f.path, ".")) {
		mark = "[" + theme.WarningHex + "]✎[-] "
	}
	return fmt.Sprintf("%s%-24s %s", mark, tview.Escape(f.label), tview.Escape(val))
}

func (v *WorkflowEditorView) fieldSecondary(f *edField, doc *workflow.Document) string {
	if f.action != nil {
		return tview.Escape(f.help)
	}
	var parts []string
	if f.lock != "" && v.check.locked(f.lock) && (doc == nil || !doc.Has(strings.Join(f.path, "."))) {
		parts = append(parts, "🔒 "+i18n.T("tui.editor.locked"))
	}
	if len(f.path) > 0 {
		if o := v.originLabel(strings.Join(f.path, ".")); o != "" {
			parts = append(parts, "← "+o)
		}
	}
	return tview.Escape(strings.Join(parts, "  "))
}

func (v *WorkflowEditorView) showFieldDetail(ref any) {
	if v.detail == nil {
		return
	}
	if v.section == edInputs {
		v.detail.SetTitle(" " + i18n.T("tui.editor.inputs.preview") + " ")
		v.detail.SetText(v.promptPreviewText())
		return
	}
	f, ok := ref.(*edField)
	if !ok {
		v.detail.SetText("")
		return
	}
	v.detail.SetTitle(" " + f.label + " ")
	var b strings.Builder
	if f.help != "" {
		b.WriteString(tview.Escape(f.help) + "\n\n")
	}
	if len(f.path) > 0 {
		fmt.Fprintf(&b, "[%s]%s[-]\n", theme.TextMutedHex, strings.Join(f.path, "."))
	}
	if f.lock != "" && v.check.locked(f.lock) {
		fmt.Fprintf(&b, "\n🔒 %s\n", i18n.Tf("tui.editor.locked_detail", f.lock))
	}
	b.WriteString("\n[" + theme.TextMutedHex + "]" + i18n.T("tui.editor.field_help") + "[-]")
	v.detail.SetText(b.String())
}

func (v *WorkflowEditorView) promptPreviewText() string {
	switch {
	case v.check == nil:
		return i18n.T("tui.editor.checking")
	case v.check.PromptErr != "":
		return "[" + theme.ErrorHex + "]✗ " + tview.Escape(v.check.PromptErr) + "[-]"
	case v.check.Prompt == "":
		return i18n.T("tui.editor.inputs.no_prompt")
	}
	return tview.Escape(v.check.Prompt)
}

// activate edits a field (Enter).
func (v *WorkflowEditorView) activate(ref any) {
	f, ok := ref.(*edField)
	if !ok {
		return
	}
	if f.action != nil {
		f.action()
		return
	}
	doc := v.m.document()
	if f.lock != "" && v.check.locked(f.lock) && (doc == nil || !doc.Has(strings.Join(f.path, "."))) {
		v.toast(i18n.Tf("tui.editor.locked_detail", f.lock), false)
		return
	}
	if f.edit != nil {
		f.edit()
	}
}

// removeCurrent removes the selected element (x / Delete): a field written
// in this document goes back to the inherited value; graph elements and
// inputs are removed (disabled when inherited).
func (v *WorkflowEditorView) removeCurrent() {
	switch v.section {
	case edGraph:
		v.removeGraphElement()
		return
	case edPreview:
		return
	}
	f, ok := v.selectedRef().(*edField)
	if !ok {
		return
	}
	if f.remove != nil {
		f.remove()
		return
	}
	if len(f.path) == 0 || f.action != nil {
		return
	}
	doc := v.m.document()
	if doc == nil || !doc.Has(strings.Join(f.path, ".")) {
		v.toast(i18n.T("tui.editor.not_written"), false)
		return
	}
	v.edit(func(e *workflow.DocEdit) error { e.Unset(f.path...); return nil })
}

// add adds an element to the current section (a).
func (v *WorkflowEditorView) add() {
	switch v.section {
	case edGraph:
		v.addAgent()
	case edInputs:
		v.addInput()
	}
}
