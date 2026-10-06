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
// Workflow catalogue (P1-T26, 10-tui §8), read only in phase 1: workflows by
// layer with version, risk, runtimes, chain and validity; a detail panel;
// Enter opens the launch form, `*` pins. Editing and publishing: phase 2.
// ─────────────────────────────────────────────────────────────────────────────

// CatalogEntry is a workflow of the catalogue.
type CatalogEntry struct {
	ID       string
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
}

// WorkflowCatalogConfig wires the catalogue.
type WorkflowCatalogConfig struct {
	// Load reads the catalogue (called off the event loop).
	Load func(ctx context.Context) ([]CatalogEntry, error)
	// Launch opens the launch form of a workflow.
	Launch func(id string)
	// TogglePin pins or unpins a workflow in the current context.
	TogglePin func(id string)
}

// WorkflowCatalogView lists the workflows.
type WorkflowCatalogView struct {
	cfg     WorkflowCatalogConfig
	shell   ShellAccess
	app     *tview.Application
	list    *widgets.SectionedList
	detail  *tview.TextView
	entries []CatalogEntry
	err     error
	gen     uint64
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
	return i18n.T("tui.catalog.hints")
}

func (v *WorkflowCatalogView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.gen++
	gen := v.gen
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetItemSelectedFunc(func(_ int, it widgets.SectionItem) {
		if id, ok := it.Reference.(string); ok && v.cfg.Launch != nil {
			v.cfg.Launch(id)
		}
	})
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) {
		id, _ := it.Reference.(string)
		v.showDetail(id)
	})
	v.list.SetBorder(true).SetTitle(" " + v.Title() + " ").SetTitleAlign(tview.AlignLeft)
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
	if v.cfg.Load == nil {
		return
	}
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	go func() {
		entries, err := v.cfg.Load(ctx)
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.gen != gen {
				return
			}
			v.entries, v.err = entries, err
			v.render()
		})
	}()
}

func (v *WorkflowCatalogView) Unmount() {
	v.gen++
	v.app, v.list, v.detail = nil, nil, nil
}

// SetEntries replaces the entries (tests).
func (v *WorkflowCatalogView) SetEntries(e []CatalogEntry) {
	v.entries = e
	v.render()
}

func (v *WorkflowCatalogView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == '*' && v.list != nil && v.cfg.TogglePin != nil {
		if _, it, ok := v.list.CurrentItem(); ok {
			if id, ok := it.Reference.(string); ok {
				v.cfg.TogglePin(id)
				for i := range v.entries {
					if v.entries[i].ID == id {
						v.entries[i].Pinned = !v.entries[i].Pinned
					}
				}
				v.render()
				return nil
			}
		}
	}
	return event
}

// catalogLayers is the display order of the layers.
var catalogLayers = []string{"hub", "team", "project"}

func (v *WorkflowCatalogView) render() {
	if v.list == nil {
		return
	}
	var items []widgets.SectionItem
	for _, layer := range catalogLayers {
		var rows []widgets.SectionItem
		for _, e := range v.entries {
			if e.Layer != layer {
				continue
			}
			rows = append(rows, widgets.SectionItem{MainText: catalogMain(e), SecondaryText: catalogSecondary(e), Reference: e.ID})
		}
		if len(rows) == 0 {
			continue
		}
		items = append(items, widgets.SectionItem{MainText: i18n.T("tui.catalog.layer." + layer), IsHeader: true})
		items = append(items, rows...)
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
	v.list.SetItems(items)
	if cur >= 0 && cur < len(items) {
		v.list.SelectIndex(cur)
	}
	if _, it, ok := v.list.CurrentItem(); ok {
		id, _ := it.Reference.(string)
		v.showDetail(id)
	} else {
		v.showDetail("")
	}
}

func catalogMain(e CatalogEntry) string {
	star := "  "
	if e.Pinned {
		star = "★ "
	}
	version := "—"
	if e.Version > 0 {
		version = fmt.Sprintf("v%d", e.Version)
	}
	status := "[" + theme.SuccessHex + "]✔[-]"
	if !e.Valid {
		status = "[" + theme.ErrorHex + "]✗[-]"
	}
	return fmt.Sprintf("%s%-18s %-4s %-8s %s  %s", star, e.ID, version, e.Risk, runtimeIcons(e.Runtimes), status)
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

func (v *WorkflowCatalogView) showDetail(id string) {
	if v.detail == nil {
		return
	}
	var e *CatalogEntry
	for i := range v.entries {
		if v.entries[i].ID == id {
			e = &v.entries[i]
		}
	}
	if e == nil {
		v.detail.SetText("")
		return
	}
	muted := "[" + theme.TextMutedHex + "]"
	var b strings.Builder
	fmt.Fprintf(&b, "[::b]%s[::-]\n", tview.Escape(e.ID))
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
	if e.Valid {
		fmt.Fprintf(&b, "\n[%s]✔ %s[-]\n", theme.SuccessHex, i18n.T("tui.catalog.valid"))
	} else {
		fmt.Fprintf(&b, "\n[%s]✗ %s[-]\n", theme.ErrorHex, i18n.T("tui.catalog.invalid"))
		for _, p := range e.Problems {
			fmt.Fprintf(&b, "  · %s\n", tview.Escape(p))
		}
	}
	v.detail.SetText(b.String())
}
