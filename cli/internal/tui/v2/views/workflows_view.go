package views

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
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
	// NoTeamState is called by `n` without a team-state (solo space); done
	// is called once the space is created.
	NoTeamState func(done func())
}

// WorkflowCatalogView lists the workflows.
type WorkflowCatalogView struct {
	cfg    WorkflowCatalogConfig
	shell  ShellAccess
	app    *tview.Application
	list   *widgets.SectionedList
	detail *tview.TextView
	page   *widgets.ListDetailPage
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

// Legend implements LegendProvider (help overlay).
func (v *WorkflowCatalogView) Legend() string {
	_, body, _ := strings.Cut(catalogLegend(), "\n")
	return body
}
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
	v.page = widgets.NewListDetailPage(v.Title(), i18n.T("tui.catalog.detail"))
	v.page.SetMain(v.list)
	v.detail = v.page.Detail
	v.page.Mount(content)
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
	v.app, v.list, v.detail, v.page = nil, nil, nil, nil
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
	case 'p', keys.Compare.Rune():
		fn := v.cfg.Publish
		if event.Rune() == keys.Compare.Rune() {
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
	case keys.Archive.Rune():
		if ok {
			v.askArchive(e)
			return nil
		}
	case keys.Delete.Rune():
		if ok {
			v.askDiscard(e)
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
			// Once the solo space exists, go on with the new workflow (A42).
			v.cfg.NoTeamState(func() {
				v.reload(func() {
					if v.data.Editable {
						v.askNew(e, hasEntry, kind)
					}
				})
			})
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
			{Key: "source", Label: i18n.T("tui.catalog.edit.new_source"), Type: FieldSelect, Default: source,
				Options: v.sourceOptions(source), Hint: i18n.T("tui.catalog.edit.new_source_hint")},
			{Key: "layer", Label: i18n.T("tui.catalog.edit.new_layer"), Type: FieldSelect, Default: layer, Options: layers},
			{Key: "id", Label: i18n.T("tui.catalog.edit.new_id"), Type: FieldText, Default: id, Required: true,
				Hint: i18n.T("tui.catalog.edit.new_id_hint")},
		},
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			n := CatalogNew{Kind: vals["kind"], Source: strings.TrimSpace(vals["source"]), ID: strings.TrimSpace(vals["id"]), Layer: vals["layer"]}
			if n.Kind == CatalogNewEmpty {
				n.Source = ""
			}
			if n.Source == catalogSourceOther {
				v.shell.ShowInputModal(i18n.T("tui.catalog.edit.new_source"), "", func(s string) {
					n.Source = strings.TrimSpace(s)
					v.submitNew(n)
				})
				return
			}
			v.submitNew(n)
		},
	})
}

// catalogSourceOther is the « other… » choice of the source (typed).
const catalogSourceOther = "\x00other"

// sourceOptions are the references a new workflow can extend or copy: the
// published documents of the catalogue (cur kept when unknown), « none »
// and « other… ».
func (v *WorkflowCatalogView) sourceOptions(cur string) []SelectOption {
	var refs []string
	for _, e := range v.data.Entries {
		if e.Draft {
			continue
		}
		refs = append(refs, e.Ref)
		refs = append(refs, e.Chain...) // shadowed parents (hub:ticket under team:ticket)
	}
	refs = append(refs, cur)
	out := []SelectOption{{Label: i18n.T("tui.catalog.edit.source_none"), Value: ""}}
	for _, ref := range sortRefs(refs) {
		out = append(out, SelectOption{Label: ref, Value: ref})
	}
	return append(out, SelectOption{Label: i18n.T("tui.catalog.edit.source_other"), Value: catalogSourceOther})
}

// sortRefs dedups layer:id references (empty ones dropped) and sorts them
// by layer (hub, team, project) then id.
func sortRefs(refs []string) []string {
	rank := func(ref string) int {
		layer, _, _ := strings.Cut(ref, ":")
		for i, l := range catalogLayers {
			if l == layer {
				return i
			}
		}
		return len(catalogLayers)
	}
	var out []string
	for _, r := range refs {
		if r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a, b)
	})
	return out
}

func (v *WorkflowCatalogView) submitNew(n CatalogNew) {
	if n.Kind != CatalogNewEmpty && n.Source == "" {
		v.toast(i18n.T("tui.catalog.edit.new_source_required"), false)
		return
	}
	v.cfg.New(n)
}

// askRemove archives a published workflow (with a message) or discards a
// draft, after confirmation.
// askArchive is x: archive a published workflow (its versions stay in the
// history, R restores one). A draft is discarded with d.
func (v *WorkflowCatalogView) askArchive(e CatalogEntry) {
	if v.shell == nil {
		return
	}
	switch {
	case e.Draft:
		v.toast(i18n.T("tui.catalog.edit.archive_draft"), false)
	case e.ReadOnly:
		v.toast(i18n.T("tui.catalog.edit.hub_read_only"), false)
	case v.cfg.Archive != nil:
		v.shell.ShowInputModal(i18n.Tf("tui.catalog.edit.archive_title", e.Ref), "", func(msg string) {
			v.cfg.Archive(e, strings.TrimSpace(msg))
		})
	}
}

// askDiscard is d: discard a draft, after a confirmation (destructive).
func (v *WorkflowCatalogView) askDiscard(e CatalogEntry) {
	if v.shell == nil {
		return
	}
	switch {
	case !e.Draft:
		v.toast(i18n.T("tui.catalog.edit.discard_published"), false)
	case v.cfg.Discard != nil:
		v.shell.Confirm(i18n.Tf("tui.catalog.edit.discard_title", e.Ref), i18n.T("tui.catalog.edit.discard_body"), true,
			func() { v.cfg.Discard(e) })
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
		fmt.Fprintf(&b, "[%s]%s %s[-]\n", theme.SuccessHex, theme.IconValid, i18n.T("tui.catalog.valid"))
	} else {
		fmt.Fprintf(&b, "[%s]%s %s[-]\n", theme.ErrorHex, theme.IconInvalid, i18n.Tf("tui.catalog.edit.errors", e.Errors))
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
	title := v.Title()
	if v.data.Team != "" {
		title += " · " + i18n.Tf("tui.catalog.edit.team", v.data.Team)
	}
	if v.data.Project != "" {
		title += " · " + i18n.Tf("tui.catalog.edit.project", v.data.Project)
	}
	v.page.SetTitle(title)
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
			items = append(items, widgets.SectionItem{MainText: "[" + theme.WarningHex + "]" + theme.IconWarn + "[-] " + tview.Escape(d.Message),
				SecondaryText: tview.Escape(d.Source), Reference: itemRef{integrity: i + 1}})
		}
	}
	if len(items) == 0 {
		msg := i18n.T("tui.catalog.empty")
		if v.err != nil {
			msg = v.err.Error()
		}
		items = append(items, widgets.SectionItem{MainText: msg, IsHeader: true})
	} else if len(v.data.Entries) > 0 {
		items = append([]widgets.SectionItem{{MainText: catalogColumns(), IsHeader: true}}, items...)
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
		star = theme.IconDraft + " "
	case e.Pinned:
		star = theme.IconPinned + " "
	}
	version := "—"
	if e.Version > 0 {
		version = fmt.Sprintf("v%d", e.Version)
	}
	status := "[" + theme.SuccessHex + "]" + theme.IconValid + "[-]"
	switch {
	case !e.Valid && e.Errors > 0:
		status = "[" + theme.ErrorHex + "]" + theme.IconInvalid + " " + i18n.Tf("tui.catalog.edit.errors", e.Errors) + "[-]"
	case !e.Valid:
		status = "[" + theme.ErrorHex + "]" + theme.IconInvalid + "[-]"
	case e.Warnings > 0:
		status = "[" + theme.WarningHex + "]" + theme.IconWarn + "[-]"
	}
	var marks []string
	if e.HasDraft {
		marks = append(marks, theme.IconDraft)
	}
	if e.Queued {
		marks = append(marks, "["+theme.WarningHex+"]"+theme.IconQueued+" "+i18n.T("tui.catalog.edit.pending")+"[-]")
	}
	if len(e.NewBricks) > 0 {
		marks = append(marks, "["+theme.InfoHex+"]"+theme.IconNewBrick+" "+i18n.T("tui.catalog.edit.new_brick")+"[-]")
	}
	line := fmt.Sprintf("%s%-18s %-4s %-8s %-5s  %s", star, e.ID, version, e.Risk, runtimeIcons(e.Runtimes), status)
	if len(marks) > 0 {
		line += "  " + strings.Join(marks, " ")
	}
	return line
}

// catalogColumns is the column header of the list, aligned on catalogMain
// (the header style adds « ── » before the text: one column less for the id).
func catalogColumns() string {
	t := i18n.T
	return fmt.Sprintf("%-17s %-4s %-8s %-5s  %s", t("tui.catalog.col.id"), t("tui.catalog.col.version"),
		t("tui.catalog.col.risk"), t("tui.catalog.col.runtimes"), t("tui.catalog.col.status"))
}

func catalogSecondary(e CatalogEntry) string {
	d := e.Desc
	if d == "" {
		d = e.Label
	}
	if len(e.Chain) > 1 {
		d += "  " + theme.IconExtends + " " + strings.Join(e.Chain[:len(e.Chain)-1], " → ")
	}
	return d
}

// runtimeIcons renders ⌂ ▣ ☁ for local, container, remote.
func runtimeIcons(rs []string) string {
	var out []string
	for _, r := range rs {
		switch r {
		case "local", "container", "remote":
			out = append(out, theme.RuntimeIcon(r))
		}
	}
	return strings.Join(out, " ")
}

// runtimeWords renders « ⌂ local · ▣ conteneur » (detail pane).
func runtimeWords(rs []string) string {
	var out []string
	for _, r := range rs {
		switch r {
		case "local", "container", "remote":
			out = append(out, theme.RuntimeIcon(r)+" "+i18n.T("tui.catalog.runtime."+r))
		default:
			out = append(out, r)
		}
	}
	return strings.Join(out, " · ")
}

// RiskDescription renders « write — modifie fichiers et Beads » ("" stays "").
func RiskDescription(risk string) string {
	switch risk {
	case "read", "plan", "write", "publish":
		return risk + " — " + i18n.T("tui.catalog.risk_desc."+risk)
	}
	return risk
}

// catalogLegend explains the pictograms of the list (detail pane, help).
func catalogLegend() string {
	t := i18n.T
	lines := []string{
		theme.IconDraft + "  " + t("tui.catalog.legend.draft"),
		theme.IconPinned + "  " + t("tui.catalog.legend.pinned"),
		"v<N> / —  " + t("tui.catalog.legend.version"),
		"read · plan · write · publish  " + t("tui.catalog.legend.risk"),
		runtimeWords([]string{"local", "container", "remote"}),
		theme.IconValid + " " + t("tui.catalog.legend.valid") + " · " + theme.IconInvalid + " " + t("tui.catalog.legend.errors") +
			" · " + theme.IconWarn + " " + t("tui.catalog.legend.warnings"),
		theme.IconQueued + "  " + t("tui.catalog.legend.queued"),
		theme.IconNewBrick + " " + t("tui.catalog.edit.new_brick") + "  " + t("tui.catalog.legend.new_brick"),
		theme.IconExtends + "  " + t("tui.catalog.legend.extends"),
	}
	return t("tui.catalog.legend.title") + "\n" + strings.Join(lines, "\n")
}

func (v *WorkflowCatalogView) showDetail(it widgets.SectionItem) {
	if v.detail == nil {
		return
	}
	if r, ok := it.Reference.(itemRef); ok && r.integrity > 0 && r.integrity <= len(v.data.Integrity) {
		d := v.data.Integrity[r.integrity-1]
		v.detail.SetText(fmt.Sprintf("[%s]%s[-] %s\n\n[%s]%s[-]\n\n%s", theme.WarningHex, theme.IconWarn, tview.Escape(d.Message),
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
		fmt.Fprintf(&b, "%s %s\n", theme.IconDraft, i18n.T("tui.catalog.edit.draft"))
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
	row("tui.catalog.risk", RiskDescription(e.Risk))
	row("tui.catalog.runtimes", runtimeWords(e.Runtimes))
	row("tui.catalog.inputs", strings.Join(e.Inputs, ", "))
	row("tui.catalog.edit.team_bricks", strings.Join(e.TeamBricks, ", "))
	if len(e.NewBricks) > 0 {
		fmt.Fprintf(&b, "[%s]%s %s[-] %s\n", theme.InfoHex, theme.IconNewBrick, i18n.T("tui.catalog.edit.new_brick"), tview.Escape(strings.Join(e.NewBricks, ", ")))
	}
	if e.Queued {
		fmt.Fprintf(&b, "[%s]%s %s[-]\n", theme.WarningHex, theme.IconQueued, i18n.T("tui.catalog.edit.queued_detail"))
	}
	if e.Valid {
		fmt.Fprintf(&b, "\n[%s]%s %s[-]\n", theme.SuccessHex, theme.IconValid, i18n.T("tui.catalog.valid"))
	} else {
		fmt.Fprintf(&b, "\n[%s]%s %s[-]\n", theme.ErrorHex, theme.IconInvalid, i18n.T("tui.catalog.invalid"))
		for _, p := range e.Problems {
			fmt.Fprintf(&b, "  · %s\n", tview.Escape(p))
		}
	}
	if v.editing() {
		fmt.Fprintf(&b, "\n%s%s[-]\n", muted, catalogActions(e))
	}
	fmt.Fprintf(&b, "\n%s%s[-]\n", muted, tview.Escape(catalogLegend()))
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
