package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Workflow catalogue (P1-T26, 10-tui §8): workflows by layer with version,
// risk, runtimes, chain and validity; a detail panel; Enter opens the launch
// form, `*` pins. Phase 2 (P2-T13): the current member's drafts, integrity
// warnings, « new brick » badge and the editing actions (n new, e edit,
// v validate, t test the draft, p publish, D diff, h history, x archive).
// ─────────────────────────────────────────────────────────────────────────────

// CatalogEntry is a workflow of the catalogue (published, or a draft of the
// current member).
type CatalogEntry struct {
	ID       string
	Ref      string // layer:id of the document
	Layer    string // hub | team | project
	Version  int
	Label    string
	Desc     string
	Risk     string
	Runtimes []string
	Entry    string
	Chain    []string
	Inputs   []string // « ticket (beads-id) * »
	Valid    bool
	Problems []string // validation findings (first ones)
	Pinned   bool

	// Phase 2.
	ReadOnly bool // hub workflow (built in)
	Draft    bool // draft of the current member
	// HasDraft: the member has a draft of this published document.
	HasDraft bool
	Queued   bool // publication waiting for the network
	Errors   int
	Warnings int
	// Findings are every validation finding, « ✗ path: message ».
	Findings []string
	// NewBricks are the team bricks a draft introduces (« new brick »).
	NewBricks []string
	// TeamBricks are the team bricks the workflow uses.
	TeamBricks []string
}

// CatalogIntegrity is a team-state file or brick skipped by the integrity
// check.
type CatalogIntegrity struct {
	Message string
	Source  string
}

// CatalogData is what the catalogue shows.
type CatalogData struct {
	Entries   []CatalogEntry
	Integrity []CatalogIntegrity
	// Team and Project name the context; Member is the current member.
	Team, Project, Member string
	// Editable: a team-state is available (drafts can be created).
	Editable bool
	// Queue is the number of operations waiting for the network.
	Queue int
}

// CatalogNew is a new workflow asked for with `n`.
type CatalogNew struct {
	// Kind is "empty", "extends" or "copy".
	Kind string
	// Source is the reference extended or copied.
	Source string
	ID     string
	Layer  string // team | project
}

// Kinds of CatalogNew.
const (
	CatalogNewEmpty   = "empty"
	CatalogNewExtends = "extends"
	CatalogNewCopy    = "copy"
)

// WorkflowCatalogConfig wires the catalogue.
type WorkflowCatalogConfig struct {
	// Load reads the catalogue (called off the event loop).
	Load func(ctx context.Context) (CatalogData, error)
	// Launch opens the launch form of a workflow.
	Launch func(id string)
	// TogglePin pins or unpins a workflow in the current context.
	TogglePin func(id string)

	// Editing (phase 2); a nil function disables its action.
	New       func(n CatalogNew)
	Edit      func(e CatalogEntry)
	TestDraft func(e CatalogEntry)
	Publish   func(e CatalogEntry)
	Diff      func(e CatalogEntry)
	History   func(e CatalogEntry)
	Archive   func(e CatalogEntry, message string)
	Discard   func(e CatalogEntry)
	// NoTeamState is called by `n` without a team-state (solo space).
	NoTeamState func()
}

// WorkflowCatalogView lists the workflows.
type WorkflowCatalogView struct {
	cfg    WorkflowCatalogConfig
	shell  ShellAccess
	app    *tview.Application
	list   *widgets.SectionedList
	detail *tview.TextView
	data   CatalogData
	err    error
	gen    uint64
	// selKey is the selected entry, kept across remounts (back from the
	// editor, the publication…).
	selKey string
}

var _ View = (*WorkflowCatalogView)(nil)

// NewWorkflowCatalogView creates the view.
func NewWorkflowCatalogView(cfg WorkflowCatalogConfig) *WorkflowCatalogView {
	return &WorkflowCatalogView{cfg: cfg}
}

// SetShell implements the shell-aware interface.
func (v *WorkflowCatalogView) SetShell(s ShellAccess) { v.shell = s }

func (v *WorkflowCatalogView) ID() string    { return "workflows" }
func (v *WorkflowCatalogView) Title() string { return i18n.T("tui.catalog.title") }
func (v *WorkflowCatalogView) StatusHints() string {
	if v.editing() {
		return i18n.T("tui.catalog.edit.hints")
	}
	return i18n.T("tui.catalog.hints")
}

func (v *WorkflowCatalogView) editing() bool { return v.cfg.Edit != nil || v.cfg.New != nil }

func (v *WorkflowCatalogView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) {
		if e, ok := v.entryOf(it); ok {
			v.activate(e)
		}
	})
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) {
		if r, ok := it.Reference.(itemRef); ok && r.key != "" {
			v.selKey = r.key
		}
		v.showDetail(it)
	})
	v.list.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	v.list.SetBorderColor(theme.ActiveMode.Primary)
	v.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	v.detail.SetBackgroundColor(theme.BgPanel)
	v.detail.SetBorder(true).SetTitle(" " + i18n.T("tui.catalog.detail") + " ").SetTitleAlign(tview.AlignLeft)
	v.detail.SetBorderColor(theme.FgMuted)
	v.detail.SetBorderPadding(0, 0, 1, 1)
	row := tview.NewFlex().AddItem(v.list, 0, 3, true).AddItem(v.detail, 0, 2, false)
	content.Clear()
	content.AddItem(row, 0, 1, true)
	v.render()
	if app != nil {
		app.SetFocus(v.list)
	}
	v.reload(nil)
}

// reload reads the catalogue off the event loop, then calls then (on the
// loop) when not nil.
func (v *WorkflowCatalogView) reload(then func()) {
	if v.cfg.Load == nil || v.app == nil {
		return
	}
	v.gen++
	gen, app := v.gen, v.app
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	go func() {
		data, err := v.cfg.Load(ctx)
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.gen != gen {
				return
			}
			v.data, v.err = data, err
			v.render()
			if then != nil {
				then()
			}
		})
	}()
}

// Refresh reloads the catalogue (after an edit, a publication…).
func (v *WorkflowCatalogView) Refresh() { v.reload(nil) }

func (v *WorkflowCatalogView) Unmount() {
	v.gen++
	v.app, v.list, v.detail = nil, nil, nil
}

// SetEntries replaces the entries (tests).
func (v *WorkflowCatalogView) SetEntries(e []CatalogEntry) {
	v.data.Entries = e
	v.render()
}

// SetData replaces the catalogue data (tests).
func (v *WorkflowCatalogView) SetData(d CatalogData) {
	v.data = d
	v.render()
}

// itemRef identifies a list item: the entry key or an integrity warning.
type itemRef struct {
	key       string
	integrity int // 1-based index of an integrity warning
}

func entryKey(e CatalogEntry) string {
	if e.Draft {
		return "draft:" + e.Ref
	}
	return e.Ref + "|" + e.ID
}

func (v *WorkflowCatalogView) entryOf(it widgets.SectionItem) (CatalogEntry, bool) {
	r, ok := it.Reference.(itemRef)
	if !ok || r.key == "" {
		return CatalogEntry{}, false
	}
	for _, e := range v.data.Entries {
		if entryKey(e) == r.key {
			return e, true
		}
	}
	return CatalogEntry{}, false
}

func (v *WorkflowCatalogView) current() (CatalogEntry, bool) {
	if v.list == nil {
		return CatalogEntry{}, false
	}
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return CatalogEntry{}, false
	}
	return v.entryOf(it)
}

// activate is Enter: launch a published workflow, test a draft; an
// integrity warning shows its detail.
func (v *WorkflowCatalogView) activate(e CatalogEntry) {
	if e.Draft {
		if v.cfg.TestDraft != nil {
			v.cfg.TestDraft(e)
		}
		return
	}
	if v.cfg.Launch != nil {
		v.cfg.Launch(e.ID)
	}
}

func (v *WorkflowCatalogView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil || event.Key() != tcell.KeyRune {
		return event
	}
	e, ok := v.current()
	switch event.Rune() {
	case '*':
		if ok && !e.Draft && v.cfg.TogglePin != nil {
			v.cfg.TogglePin(e.ID)
			for i := range v.data.Entries {
				if v.data.Entries[i].ID == e.ID {
					v.data.Entries[i].Pinned = !v.data.Entries[i].Pinned
				}
			}
			v.render()
			return nil
		}
	case 'n':
		if v.cfg.New != nil {
			v.askNew(e, ok, CatalogNewEmpty)
			return nil
		}
	case 'e':
		if ok && v.cfg.Edit != nil {
			if e.ReadOnly && !e.Draft {
				v.askNew(e, true, CatalogNewExtends)
			} else {
				v.cfg.Edit(e)
			}
			return nil
		}
	case 'v':
		if ok {
			v.reload(func() { v.showFindings(e) })
			return nil
		}
	case 't':
		if ok && v.cfg.TestDraft != nil {
			if !e.Draft && !e.HasDraft {
				v.toast(i18n.T("tui.catalog.edit.no_draft"), false)
			} else {
				v.cfg.TestDraft(e)
			}
			return nil
		}
	case 'p', 'D':
		fn := v.cfg.Publish
		if event.Rune() == 'D' {
			fn = v.cfg.Diff
		}
		if ok && fn != nil {
			if !e.Draft && !e.HasDraft {
				v.toast(i18n.T("tui.catalog.edit.no_draft"), false)
			} else {
				fn(e)
			}
			return nil
		}
	case 'h':
		if ok && v.cfg.History != nil {
			if e.ReadOnly && !e.Draft {
				v.toast(i18n.T("tui.catalog.edit.hub_read_only"), false)
			} else {
				v.cfg.History(e)
			}
			return nil
		}
	case 'x':
		if ok {
			v.askRemove(e)
			return nil
		}
	case 'r':
		v.reload(nil)
		return nil
	}
	return event
}

func (v *WorkflowCatalogView) toast(msg string, success bool) {
	if v.shell != nil {
		v.shell.ShowToastMsg(msg, success)
	}
}

// askNew shows the « new workflow » form: kind, source, layer, id.
func (v *WorkflowCatalogView) askNew(e CatalogEntry, hasEntry bool, kind string) {
	if !v.data.Editable {
		if v.cfg.NoTeamState != nil {
			v.cfg.NoTeamState()
		} else {
			v.toast(i18n.T("tui.catalog.edit.no_team_state"), false)
		}
		return
	}
	if v.shell == nil {
		return
	}
	source := ""
	if hasEntry && !e.Draft {
		source = e.Ref
	}
	id := ""
	if kind != CatalogNewEmpty && hasEntry {
		id = e.ID
	}
	layers := []SelectOption{{Label: i18n.T("tui.catalog.layer.team"), Value: "team"}}
	if v.data.Project != "" {
		layers = append(layers, SelectOption{Label: i18n.T("tui.catalog.layer.project"), Value: "project"})
	}
	layer := "team"
	if hasEntry && e.Layer == "team" && v.data.Project != "" {
		layer = "project" // a patch of a team workflow goes to the project
	}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.T("tui.catalog.edit.new_title"),
		Fields: []FormField{
			{Key: "kind", Label: i18n.T("tui.catalog.edit.new_kind"), Type: FieldSelect, Default: kind, Options: []SelectOption{
				{Label: i18n.T("tui.catalog.edit.new_empty"), Value: CatalogNewEmpty},
				{Label: i18n.T("tui.catalog.edit.new_extends"), Value: CatalogNewExtends},
				{Label: i18n.T("tui.catalog.edit.new_copy"), Value: CatalogNewCopy},
			}},
			{Key: "source", Label: i18n.T("tui.catalog.edit.new_source"), Type: FieldText, Default: source,
				Hint: i18n.T("tui.catalog.edit.new_source_hint")},
			{Key: "layer", Label: i18n.T("tui.catalog.edit.new_layer"), Type: FieldSelect, Default: layer, Options: layers},
			{Key: "id", Label: i18n.T("tui.catalog.edit.new_id"), Type: FieldText, Default: id, Required: true,
				Hint: i18n.T("tui.catalog.edit.new_id_hint")},
		},
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			n := CatalogNew{Kind: vals["kind"], Source: strings.TrimSpace(vals["source"]), ID: strings.TrimSpace(vals["id"]), Layer: vals["layer"]}
			if n.Kind != CatalogNewEmpty && n.Source == "" {
				v.toast(i18n.T("tui.catalog.edit.new_source_required"), false)
				return
			}
			if n.Kind == CatalogNewEmpty {
				n.Source = ""
			}
			v.cfg.New(n)
		},
	})
}

// askRemove archives a published workflow (with a message) or discards a
// draft, after confirmation.
func (v *WorkflowCatalogView) askRemove(e CatalogEntry) {
	if v.shell == nil {
		return
	}
	switch {
	case e.Draft && v.cfg.Discard != nil:
		v.shell.ShowScrollableModal(i18n.Tf("tui.catalog.edit.discard_title", e.Ref), i18n.T("tui.catalog.edit.discard_body"), []ModalAction{
			{Label: i18n.T("tui.catalog.edit.discard"), Callback: func() { v.cfg.Discard(e) }},
			{Label: i18n.T("tui.catalog.edit.cancel"), Callback: func() {}, Separator: true},
		})
	case e.ReadOnly:
		v.toast(i18n.T("tui.catalog.edit.hub_read_only"), false)
	case v.cfg.Archive != nil:
		v.shell.ShowInputModal(i18n.Tf("tui.catalog.edit.archive_title", e.Ref), "", func(msg string) {
			v.cfg.Archive(e, strings.TrimSpace(msg))
		})
	}
}

// showFindings shows the validation of e (after a reload).
func (v *WorkflowCatalogView) showFindings(e CatalogEntry) {
	for _, cur := range v.data.Entries {
		if entryKey(cur) == entryKey(e) {
			e = cur
		}
	}
	if v.shell == nil {
		return
	}
	var b strings.Builder
	if e.Valid {
		fmt.Fprintf(&b, "[%s]✔ %s[-]\n", theme.SuccessHex, i18n.T("tui.catalog.valid"))
	} else {
		fmt.Fprintf(&b, "[%s]✗ %s[-]\n", theme.ErrorHex, i18n.Tf("tui.catalog.edit.errors", e.Errors))
	}
	for _, f := range e.Findings {
		fmt.Fprintf(&b, "  %s\n", tview.Escape(f))
	}
	v.shell.ShowScrollableModal(i18n.Tf("tui.catalog.edit.validation_title", e.Ref), b.String(), []ModalAction{
		{Label: i18n.T("tui.catalog.edit.close"), Callback: func() {}},
	})
}

// catalogLayers is the display order of the layers.
var catalogLayers = []string{"hub", "team", "project"}

func (v *WorkflowCatalogView) render() {
	if v.list == nil {
		return
	}
	title := " " + v.Title()
	if v.data.Team != "" {
		title += " · " + i18n.Tf("tui.catalog.edit.team", v.data.Team)
	}
	if v.data.Project != "" {
		title += " · " + i18n.Tf("tui.catalog.edit.project", v.data.Project)
	}
	v.list.SetTitle(title + " ")
	var items []widgets.SectionItem
	for _, layer := range catalogLayers {
		var rows []widgets.SectionItem
		for _, e := range v.data.Entries {
			if e.Layer != layer || e.Draft {
				continue
			}
			rows = append(rows, widgets.SectionItem{MainText: catalogMain(e), SecondaryText: catalogSecondary(e), Reference: itemRef{key: entryKey(e)}})
		}
		if len(rows) == 0 {
			continue
		}
		header := i18n.T("tui.catalog.layer." + layer)
		if layer == "hub" && v.editing() {
			header = i18n.T("tui.catalog.edit.layer_hub")
		}
		items = append(items, widgets.SectionItem{MainText: header, IsHeader: true})
		items = append(items, rows...)
	}
	var drafts []widgets.SectionItem
	for _, e := range v.data.Entries {
		if e.Draft {
			drafts = append(drafts, widgets.SectionItem{MainText: catalogMain(e), SecondaryText: catalogSecondary(e), Reference: itemRef{key: entryKey(e)}})
		}
	}
	if len(drafts) > 0 {
		items = append(items, widgets.SectionItem{MainText: i18n.Tf("tui.catalog.edit.drafts", v.data.Member), IsHeader: true})
		items = append(items, drafts...)
	}
	if len(v.data.Integrity) > 0 {
		items = append(items, widgets.SectionItem{MainText: i18n.Tf("tui.catalog.edit.integrity", len(v.data.Integrity)), IsHeader: true})
		for i, d := range v.data.Integrity {
			items = append(items, widgets.SectionItem{MainText: "[" + theme.WarningHex + "]⚠[-] " + tview.Escape(d.Message),
				SecondaryText: tview.Escape(d.Source), Reference: itemRef{integrity: i + 1}})
		}
	}
	if len(items) == 0 {
		msg := i18n.T("tui.catalog.empty")
		if v.err != nil {
			msg = v.err.Error()
		}
		items = append(items, widgets.SectionItem{MainText: msg, IsHeader: true})
	}
	cur := -1
	if idx, _, ok := v.list.CurrentItem(); ok {
		cur = idx
	}
	if v.selKey != "" {
		for i, it := range items {
			if r, ok := it.Reference.(itemRef); ok && r.key == v.selKey {
				cur = i
			}
		}
	}
	v.list.SetItems(items)
	if cur >= 0 && cur < len(items) {
		v.list.SelectIndex(cur)
	}
	if _, it, ok := v.list.CurrentItem(); ok {
		v.showDetail(it)
	} else {
		v.showDetail(widgets.SectionItem{})
	}
}

func catalogMain(e CatalogEntry) string {
	star := "  "
	switch {
	case e.Draft:
		star = "✎ "
	case e.Pinned:
		star = "★ "
	}
	version := "—"
	if e.Version > 0 {
		version = fmt.Sprintf("v%d", e.Version)
	}
	status := "[" + theme.SuccessHex + "]✔[-]"
	switch {
	case !e.Valid && e.Errors > 0:
		status = "[" + theme.ErrorHex + "]✗ " + i18n.Tf("tui.catalog.edit.errors", e.Errors) + "[-]"
	case !e.Valid:
		status = "[" + theme.ErrorHex + "]✗[-]"
	case e.Warnings > 0:
		status = "[" + theme.WarningHex + "]⚠[-]"
	}
	var marks []string
	if e.HasDraft {
		marks = append(marks, "✎")
	}
	if e.Queued {
		marks = append(marks, "["+theme.WarningHex+"]⏳ "+i18n.T("tui.catalog.edit.pending")+"[-]")
	}
	if len(e.NewBricks) > 0 {
		marks = append(marks, "["+theme.InfoHex+"]+ "+i18n.T("tui.catalog.edit.new_brick")+"[-]")
	}
	line := fmt.Sprintf("%s%-18s %-4s %-8s %s  %s", star, e.ID, version, e.Risk, runtimeIcons(e.Runtimes), status)
	if len(marks) > 0 {
		line += "  " + strings.Join(marks, " ")
	}
	return line
}

func catalogSecondary(e CatalogEntry) string {
	d := e.Desc
	if d == "" {
		d = e.Label
	}
	if len(e.Chain) > 1 {
		d += "  ↳ " + strings.Join(e.Chain[:len(e.Chain)-1], " → ")
	}
	return d
}

// runtimeIcons renders ⌂ ▣ ☁ for local, container, remote.
func runtimeIcons(rs []string) string {
	icons := map[string]string{"local": "⌂", "container": "▣", "remote": "☁"}
	var out []string
	for _, r := range rs {
		if ic, ok := icons[r]; ok {
			out = append(out, ic)
		}
	}
	return strings.Join(out, " ")
}

func (v *WorkflowCatalogView) showDetail(it widgets.SectionItem) {
	if v.detail == nil {
		return
	}
	if r, ok := it.Reference.(itemRef); ok && r.integrity > 0 && r.integrity <= len(v.data.Integrity) {
		d := v.data.Integrity[r.integrity-1]
		v.detail.SetText(fmt.Sprintf("[%s]⚠[-] %s\n\n[%s]%s[-]\n\n%s", theme.WarningHex, tview.Escape(d.Message),
			theme.TextMutedHex, tview.Escape(d.Source), i18n.T("tui.catalog.edit.integrity_hint")))
		return
	}
	e, ok := v.entryOf(it)
	if !ok {
		v.detail.SetText("")
		return
	}
	muted := "[" + theme.TextMutedHex + "]"
	var b strings.Builder
	fmt.Fprintf(&b, "[::b]%s[::-]  %s%s[-]\n", tview.Escape(e.ID), muted, tview.Escape(e.Ref))
	if e.Draft {
		fmt.Fprintf(&b, "✎ %s\n", i18n.T("tui.catalog.edit.draft"))
	}
	if e.Desc != "" {
		fmt.Fprintf(&b, "%s\n", tview.Escape(e.Desc))
	}
	b.WriteString("\n")
	row := func(k, val string) {
		if val != "" {
			fmt.Fprintf(&b, "%s%-12s[-] %s\n", muted, i18n.T(k), tview.Escape(val))
		}
	}
	row("tui.catalog.chain", strings.Join(e.Chain, " → "))
	row("tui.catalog.entry", e.Entry)
	row("tui.catalog.risk", e.Risk)
	row("tui.catalog.runtimes", strings.Join(e.Runtimes, ", "))
	row("tui.catalog.inputs", strings.Join(e.Inputs, ", "))
	row("tui.catalog.edit.team_bricks", strings.Join(e.TeamBricks, ", "))
	if len(e.NewBricks) > 0 {
		fmt.Fprintf(&b, "[%s]+ %s[-] %s\n", theme.InfoHex, i18n.T("tui.catalog.edit.new_brick"), tview.Escape(strings.Join(e.NewBricks, ", ")))
	}
	if e.Queued {
		fmt.Fprintf(&b, "[%s]⏳ %s[-]\n", theme.WarningHex, i18n.T("tui.catalog.edit.queued_detail"))
	}
	if e.Valid {
		fmt.Fprintf(&b, "\n[%s]✔ %s[-]\n", theme.SuccessHex, i18n.T("tui.catalog.valid"))
	} else {
		fmt.Fprintf(&b, "\n[%s]✗ %s[-]\n", theme.ErrorHex, i18n.T("tui.catalog.invalid"))
		for _, p := range e.Problems {
			fmt.Fprintf(&b, "  · %s\n", tview.Escape(p))
		}
	}
	if v.editing() {
		fmt.Fprintf(&b, "\n%s%s[-]\n", muted, catalogActions(e))
	}
	v.detail.SetText(b.String())
}

// catalogActions lists the actions available on e.
func catalogActions(e CatalogEntry) string {
	switch {
	case e.Draft:
		return i18n.T("tui.catalog.edit.actions_draft")
	case e.ReadOnly:
		return i18n.T("tui.catalog.edit.actions_hub")
	case e.HasDraft:
		return i18n.T("tui.catalog.edit.actions_with_draft")
	}
	return i18n.T("tui.catalog.edit.actions_published")
}
