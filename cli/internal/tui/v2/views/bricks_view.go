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
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// ─────────────────────────────────────────────────────────────────────────────
// Brick catalogue (P3-T27, 10-tui §11; tui-wiki-bricks), read-only: the
// agents and skills a session may receive, their origin (hub, team, team
// brick that replaces a hub brick, team brick not sealed), published
// version, state of their checks, estimated cost and the agents and
// workflows that use them. Writing goes through `oh brick …`. It replaces
// the per-project agent selection of the former deployment (view id kept:
// `project.agents`). Keys: `/` search, `f` agents/skills/all, `r` refresh,
// Enter on a brick used by workflows opens one of them.
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

	// Team bricks (ADR-054).
	TeamID  string // ref of the team file ("" = ID)
	Source  string // file in the team-state
	Shadows string // hub brick it replaces ("hub:review/x")
	// Version is the published version (catalog.lock; 0 = none).
	Version     int
	PublishedBy string
	PublishedAt time.Time
	// Unsealed: loaded without catalog.lock (catalogue never sealed).
	Unsealed bool
	// Problems are the findings of the checks of the brick.
	Problems []BrickProblem
}

// BrickProblem is a finding of the checks of a team brick.
type BrickProblem struct {
	Error   bool
	Message string
}

// ref is the reference of the brick for `oh brick` (kind:id).
func (r BrickRow) ref() string { return r.Kind + ":" + firstText(r.TeamID, r.ID) }

func (r BrickRow) errors() int {
	n := 0
	for _, p := range r.Problems {
		if p.Error {
			n++
		}
	}
	return n
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
	list   *ActionList
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

func (v *BricksView) ID() string    { return "project.agents" }
func (v *BricksView) Title() string { return i18n.T("tui.bricks.title") }

// StatusHints returns the keys of the view and the commands that write.
func (v *BricksView) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), keys.Filter.Hint(), "f "+i18n.T("tui.bricks.kind_filter"),
		keys.Open.HintAs(i18n.T("tui.bricks.open_label")), keys.Refresh.Hint(), keys.Back.Hint()) +
		"   " + i18n.T("tui.bricks.write_note")
}

func (v *BricksView) newList() *ActionList {
	return NewActionList(ActionListConfig{
		Title:       v.Title(),
		Layout:      DetailRight,
		Headers:     []string{i18n.T("tui.bricks.col.id"), i18n.T("tui.bricks.col.origin"), i18n.T("tui.bricks.col.version")},
		StateHeader: i18n.T("tui.bricks.col.state"),
		Empty:       i18n.T("tui.bricks.loading"),
		Shell:       v.shell,
		OnOpen:      func(it ActionItem) { v.openWorkflows(it) },
		OpenLabel:   i18n.T("tui.bricks.open_label"),
	})
}

func (v *BricksView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.list = v.newList()
	content.AddItem(v.list.Primitive(), 0, 1, true)
	v.render()
	if app != nil {
		app.SetFocus(v.list.Focusable())
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
	v.app, v.list = nil, nil
}

// SetRows replaces the bricks (tests).
func (v *BricksView) SetRows(rows []BrickRow) {
	v.rows, v.loaded = rows, true
	v.render()
}

func (v *BricksView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}
	switch {
	case keys.Filter.Matches(event):
		if v.shell != nil {
			v.shell.ShowInputModal(i18n.T("tui.bricks.search"), v.query, func(q string) {
				v.query = strings.TrimSpace(q)
				v.render()
			})
		}
		return nil
	case keys.Refresh.Matches(event):
		v.reload()
		return nil
	case event.Key() == tcell.KeyRune && event.Rune() == 'f' && event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0:
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
	return v.list.HandleKey(event)
}

func (v *BricksView) matches(r BrickRow) bool {
	if v.kind != "" && r.Kind != v.kind {
		return false
	}
	if v.query == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{r.ID, r.TeamID, r.Name, r.Label, r.Description, r.Family, r.Shadows}, " "))
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
	title := v.Title()
	if v.kind != "" {
		title += " · " + i18n.T("tui.bricks.kind."+v.kind)
	}
	if v.query != "" {
		title += " · /" + v.query
	}
	v.list.SetTitle(title)
	switch {
	case v.err != nil:
		v.list.cfg.Empty = v.err.Error()
	case v.loaded:
		v.list.cfg.Empty = i18n.T("tui.bricks.empty")
	default:
		v.list.cfg.Empty = i18n.T("tui.bricks.loading")
	}
	var items []ActionItem
	for _, kind := range []string{"agent", "skill"} {
		var group []int
		for i, r := range v.rows {
			if r.Kind == kind && v.matches(r) {
				group = append(group, i)
			}
		}
		section := i18n.Tf("tui.bricks.section."+kind, len(group))
		for _, i := range group {
			r := v.rows[i]
			state, text := brickState(r)
			items = append(items, ActionItem{ID: r.Kind + ":" + r.ID, Section: section,
				Cells: []string{r.ID, brickOriginCell(r), brickVersion(r)}, State: state, StateText: text,
				DetailTitle: r.ref(), Detail: brickDetail(r), Ref: i})
		}
	}
	v.list.SetItems(items)
}

// brickState is ✓ (no finding), ! (warnings) or ✗ (errors).
func brickState(r BrickRow) (state ItemState, text string) {
	switch {
	case r.errors() > 0:
		return ItemRefused, "✗"
	case len(r.Problems) > 0:
		return ItemPending, "!"
	}
	return ItemAccepted, "✓"
}

// brickOriginCell is « hub », « équipe », « équipe ⟳ hub » (replaces a hub
// brick) or « équipe (non scellée) ».
func brickOriginCell(r BrickRow) string {
	if r.Origin != "team" {
		return i18n.T("tui.bricks.origin.hub")
	}
	o := i18n.T("tui.bricks.origin.team")
	if r.Shadows != "" {
		o = i18n.T("tui.bricks.origin.team_shadows")
	}
	if r.Unsealed {
		o += " " + i18n.T("tui.bricks.origin.unsealed")
	}
	return o
}

func brickVersion(r BrickRow) string {
	if r.Version > 0 {
		return fmt.Sprintf("%d", r.Version)
	}
	return "—"
}

// brickDetail renders the detail pane of a brick (tview tags, escaped).
func brickDetail(r BrickRow) string {
	label, reset := theme.ColorTag(theme.TextSecondaryHex), theme.TagColor
	var b strings.Builder
	line := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s%s%s %s\n", label, name, reset, value)
		}
	}
	if r.Label != "" {
		fmt.Fprintf(&b, "[::b]%s[::-]\n", tview.Escape(r.Label))
	}
	if r.Description != "" {
		fmt.Fprintf(&b, "%s\n", tview.Escape(r.Description))
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	line(i18n.T("tui.bricks.field.origin"), tview.Escape(brickOriginText(r)))
	line(i18n.T("tui.bricks.field.source"), tview.Escape(r.Source))
	if r.Origin == "team" {
		line(i18n.T("tui.bricks.field.version"), tview.Escape(brickVersionText(r)))
	}
	line(i18n.T("tui.bricks.field.kind"), i18n.T("tui.bricks.kind."+r.Kind))
	line(i18n.T("tui.bricks.field.name"), tview.Escape(r.Name))
	line(i18n.T("tui.bricks.field.family"), tview.Escape(r.Family))
	line(i18n.T("tui.bricks.field.mode"), tview.Escape(r.Mode))
	line(i18n.T("tui.bricks.field.skills"), tview.Escape(strings.Join(r.Skills, ", ")))
	line(i18n.T("tui.bricks.field.requires"), tview.Escape(strings.Join(r.Requires, ", ")))
	line(i18n.T("tui.bricks.field.agents"), tview.Escape(strings.Join(r.Agents, ", ")))
	workflows := i18n.T("tui.bricks.unused")
	if len(r.Workflows) > 0 {
		workflows = strings.Join(r.Workflows, ", ")
	}
	line(i18n.T("tui.bricks.field.workflows"), tview.Escape(workflows))
	line(i18n.T("tui.bricks.field.cost"), i18n.Tf("tui.bricks.tokens", r.Tokens))
	if len(r.Problems) > 0 {
		fmt.Fprintf(&b, "\n%s%s%s\n", label, i18n.Tf("tui.bricks.field.problems", len(r.Problems)), reset)
		for _, p := range r.Problems {
			icon, color := "!", theme.WarningHex
			if p.Error {
				icon, color = "✗", theme.ErrorHex
			}
			fmt.Fprintf(&b, "%s%s%s %s\n", theme.ColorTag(color), icon, reset, tview.Escape(p.Message))
		}
	}
	b.WriteString("\n" + muted(tview.Escape(brickWriteText(r))) + "\n")
	if len(r.Workflows) > 0 {
		var d detailText
		d.keys(keys.Open.Keys(), i18n.T("tui.bricks.open_label"))
		b.WriteString(d.String())
	}
	return b.String()
}

func brickOriginText(r BrickRow) string {
	if r.Origin != "team" {
		return i18n.T("tui.bricks.origin.hub")
	}
	o := i18n.T("tui.bricks.origin.team")
	if r.Shadows != "" {
		o += " " + i18n.Tf("tui.bricks.origin.replaces", r.Shadows)
	}
	if r.Unsealed {
		o += " · " + i18n.T("tui.bricks.origin.unsealed_detail")
	}
	return o
}

func brickVersionText(r BrickRow) string {
	if r.Version == 0 {
		return i18n.T("tui.bricks.not_published")
	}
	v := fmt.Sprintf("v%d", r.Version)
	switch {
	case r.PublishedBy != "" && !r.PublishedAt.IsZero():
		v += " · " + i18n.Tf("tui.bricks.published_by_on", r.PublishedBy, r.PublishedAt.Local().Format("02/01/2006"))
	case r.PublishedBy != "":
		v += " · " + i18n.Tf("tui.bricks.published_by", r.PublishedBy)
	}
	return v
}

// brickWriteText names the commands that change the brick (not the TUI).
func brickWriteText(r BrickRow) string {
	if r.Origin != "team" {
		return i18n.Tf("tui.bricks.write_hub", r.ref())
	}
	return i18n.Tf("tui.bricks.write_team", r.ref(), r.ref())
}

func (v *BricksView) rowOf(it ActionItem) (BrickRow, bool) {
	i, ok := it.Ref.(int)
	if !ok || i < 0 || i >= len(v.rows) {
		return BrickRow{}, false
	}
	return v.rows[i], true
}

// openWorkflows is Enter: choose a workflow that ships the brick and open it.
func (v *BricksView) openWorkflows(it ActionItem) {
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

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}
