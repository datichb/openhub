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
// Brick catalogue (P3-T27, 10-tui §11), read-only: the agents and skills a
// session may receive, their origin (hub, team catalogue), estimated cost
// and the workflows that ship them. It replaces the per-project agent
// selection of the former deployment (view id kept: `project.agents`).
// Keys: `/` search, `f` agents/skills/all, Enter on a workflow brick lists
// its workflows.
// ─────────────────────────────────────────────────────────────────────────────

// BrickRow is a brick of the catalogue.
type BrickRow struct {
	Kind        string // agent | skill
	ID          string // agent id or skill ref
	Name        string // skill name
	Label       string
	Description string
	Family      string
	Mode        string
	Origin      string // hub | team
	Skills      []string
	Requires    []string
	Tokens      int
	Agents      []string
	Workflows   []string
}

// BricksViewConfig wires the brick catalogue.
type BricksViewConfig struct {
	// Load reads the catalogue of the active context (off the event loop).
	Load func(ctx context.Context) ([]BrickRow, error)
	// OpenWorkflow opens a workflow of the catalogue (nil: no action).
	OpenWorkflow func(id string)
}

// BricksView is the read-only brick catalogue.
type BricksView struct {
	cfg    BricksViewConfig
	shell  ShellAccess
	app    *tview.Application
	list   *widgets.SectionedList
	detail *tview.TextView
	rows   []BrickRow
	err    error
	loaded bool
	gen    uint64
	query  string
	kind   string // "" = all, agent, skill
}

var _ View = (*BricksView)(nil)

// NewBricksView creates the view.
func NewBricksView(cfg BricksViewConfig) *BricksView { return &BricksView{cfg: cfg} }

// SetShell implements the shell-aware interface.
func (v *BricksView) SetShell(s ShellAccess) { v.shell = s }

func (v *BricksView) ID() string          { return "project.agents" }
func (v *BricksView) Title() string       { return i18n.T("tui.bricks.title") }
func (v *BricksView) StatusHints() string { return i18n.T("tui.bricks.hints") }

func (v *BricksView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showDetail(it) })
	v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) { v.openWorkflows(it) })
	v.list.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	v.list.SetBorderColor(theme.ActiveMode.Primary)
	v.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	v.detail.SetBackgroundColor(theme.BgPanel)
	v.detail.SetBorder(true).SetTitle(" " + i18n.T("tui.bricks.detail") + " ").SetTitleAlign(tview.AlignLeft)
	v.detail.SetBorderColor(theme.FgMuted)
	v.detail.SetBorderPadding(0, 0, 1, 1)
	row := tview.NewFlex().AddItem(v.list, 0, 3, true).AddItem(v.detail, 0, 2, false)
	content.Clear()
	content.AddItem(row, 0, 1, true)
	v.render()
	if app != nil {
		app.SetFocus(v.list)
	}
	v.reload()
}

func (v *BricksView) reload() {
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
		rows, err := v.cfg.Load(ctx)
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.gen != gen {
				return
			}
			v.rows, v.err, v.loaded = rows, err, true
			v.render()
		})
	}()
}

func (v *BricksView) Unmount() {
	v.gen++
	v.app, v.list, v.detail = nil, nil, nil
}

// SetRows replaces the bricks (tests).
func (v *BricksView) SetRows(rows []BrickRow) {
	v.rows, v.loaded = rows, true
	v.render()
}

func (v *BricksView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil || event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case '/':
		if v.shell != nil {
			v.shell.ShowInputModal(i18n.T("tui.bricks.search"), v.query, func(q string) {
				v.query = strings.TrimSpace(q)
				v.render()
			})
		}
		return nil
	case 'f':
		switch v.kind {
		case "":
			v.kind = "agent"
		case "agent":
			v.kind = "skill"
		default:
			v.kind = ""
		}
		v.render()
		return nil
	}
	return event
}

func (v *BricksView) matches(r BrickRow) bool {
	if v.kind != "" && r.Kind != v.kind {
		return false
	}
	if v.query == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{r.ID, r.Name, r.Label, r.Description, r.Family}, " "))
	for _, w := range strings.Fields(strings.ToLower(v.query)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func (v *BricksView) render() {
	if v.list == nil {
		return
	}
	title := " " + v.Title()
	if v.kind != "" {
		title += " · " + i18n.T("tui.bricks.kind."+v.kind)
	}
	if v.query != "" {
		title += " · /" + v.query
	}
	v.list.SetTitle(title + " ")
	var items []widgets.SectionItem
	for _, kind := range []string{"agent", "skill"} {
		var rows []widgets.SectionItem
		for i, r := range v.rows {
			if r.Kind != kind || !v.matches(r) {
				continue
			}
			rows = append(rows, widgets.SectionItem{MainText: brickMain(r), SecondaryText: brickSecondary(r), Reference: i})
		}
		if len(rows) == 0 {
			continue
		}
		items = append(items, widgets.SectionItem{MainText: i18n.Tf("tui.bricks.section."+kind, len(rows)), IsHeader: true})
		items = append(items, rows...)
	}
	if len(items) == 0 {
		msg := i18n.T("tui.bricks.loading")
		switch {
		case v.err != nil:
			msg = v.err.Error()
		case v.loaded:
			msg = i18n.T("tui.bricks.empty")
		}
		items = append(items, widgets.SectionItem{MainText: msg, IsHeader: true})
	}
	v.list.SetItems(items)
	if _, it, ok := v.list.CurrentItem(); ok {
		v.showDetail(it)
	} else {
		v.showDetail(widgets.SectionItem{})
	}
}

func originBadge(origin string) string {
	if origin == "team" {
		return "[" + theme.InfoHex + "]" + i18n.T("tui.bricks.origin.team") + "[-]"
	}
	return "[" + theme.TextMutedHex + "]" + i18n.T("tui.bricks.origin.hub") + "[-]"
}

func brickMain(r BrickRow) string {
	used := "  "
	if len(r.Workflows) > 0 {
		used = "● "
	}
	return fmt.Sprintf("%s%s  %s", used, tview.Escape(r.ID), originBadge(r.Origin))
}

func brickSecondary(r BrickRow) string {
	parts := []string{i18n.Tf("tui.bricks.tokens", r.Tokens)}
	if n := len(r.Workflows); n > 0 {
		parts = append(parts, i18n.Tf("tui.bricks.used_by", n))
	} else {
		parts = append(parts, i18n.T("tui.bricks.unused"))
	}
	if r.Description != "" {
		parts = append(parts, tview.Escape(truncateRunes(r.Description, 60)))
	}
	return "    " + strings.Join(parts, " · ")
}

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}

func (v *BricksView) rowOf(it widgets.SectionItem) (BrickRow, bool) {
	i, ok := it.Reference.(int)
	if !ok || i < 0 || i >= len(v.rows) {
		return BrickRow{}, false
	}
	return v.rows[i], true
}

func (v *BricksView) showDetail(it widgets.SectionItem) {
	if v.detail == nil {
		return
	}
	r, ok := v.rowOf(it)
	if !ok {
		v.detail.SetText("")
		return
	}
	muted, reset := theme.ColorTag(theme.TextSecondaryHex), theme.TagColor
	var b strings.Builder
	line := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s%s%s %s\n", muted, label, reset, value)
		}
	}
	fmt.Fprintf(&b, "[::b]%s[::-]  %s\n\n", tview.Escape(r.ID), originBadge(r.Origin))
	if r.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", tview.Escape(r.Description))
	}
	line(i18n.T("tui.bricks.field.kind"), i18n.T("tui.bricks.kind."+r.Kind))
	line(i18n.T("tui.bricks.field.name"), tview.Escape(r.Name))
	line(i18n.T("tui.bricks.field.family"), tview.Escape(r.Family))
	line(i18n.T("tui.bricks.field.mode"), r.Mode)
	line(i18n.T("tui.bricks.field.cost"), i18n.Tf("tui.bricks.tokens", r.Tokens))
	line(i18n.T("tui.bricks.field.skills"), tview.Escape(strings.Join(r.Skills, ", ")))
	line(i18n.T("tui.bricks.field.requires"), tview.Escape(strings.Join(r.Requires, ", ")))
	line(i18n.T("tui.bricks.field.agents"), tview.Escape(strings.Join(r.Agents, ", ")))
	workflows := i18n.T("tui.bricks.unused")
	if len(r.Workflows) > 0 {
		workflows = strings.Join(r.Workflows, ", ")
	}
	line(i18n.T("tui.bricks.field.workflows"), tview.Escape(workflows))
	v.detail.SetText(b.String())
	v.detail.ScrollToBeginning()
}

// openWorkflows is Enter: choose a workflow that ships the brick and open it.
func (v *BricksView) openWorkflows(it widgets.SectionItem) {
	r, ok := v.rowOf(it)
	if !ok || v.cfg.OpenWorkflow == nil || v.shell == nil || len(r.Workflows) == 0 {
		return
	}
	opts := make([]SelectOption, len(r.Workflows))
	for i, id := range r.Workflows {
		opts[i] = SelectOption{Label: id, Value: id}
	}
	v.shell.ShowSelectModal(i18n.Tf("tui.bricks.open_workflow", r.ID), opts, "", v.cfg.OpenWorkflow)
}
