package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// Clean-up view (modernisation tui-launch, merged by tui-docs), in two
// sections (Tab):
//   - « Anciens déploiements » (cleanup_deploy.go): what the former
//     per-project deployments left, as `oh migrate deploy-cleanup` cleans
//     it;
//   - « Données d'oh » (`oh gc`): what oh can remove, by category (session
//     bundles, server data, stopped sessions, worktrees, fan-outs,
//     temporary folders, orphan servers, history), computed without
//     removing anything (the dry run of oh gc); d removes the selected
//     element, A everything, each after a confirmation; the result is
//     shown above the new plan.

// GCViewConfig wires the view.
type GCViewConfig struct {
	// Plan computes what oh gc would remove (off the event loop).
	Plan func(ctx context.Context) (gc.Report, error)
	// Run removes the elements of only that are still planned (off the
	// event loop).
	Run func(ctx context.Context, only gc.Report) (gc.Report, error)
	// Bytes formats a size (« 1.2 MB »).
	Bytes func(int64) string
	// Retention describes the retentions applied (« sessions arrêtées
	// gardées 30 j… », optional).
	Retention string
	// Deploy is the section of the former deployments (nil: none).
	Deploy *DeployCleanupConfig
	// Section is the section shown first.
	Section CleanupSection
}

// CleanupSection is a section of the clean-up view.
type CleanupSection int

const (
	// CleanupData is « Données d'oh » (oh gc).
	CleanupData CleanupSection = iota
	// CleanupDeploy is « Anciens déploiements » (oh migrate deploy-cleanup).
	CleanupDeploy
)

// GCView is the clean-up view.
type GCView struct {
	cfg   GCViewConfig
	shell ShellAccess
	app   *tview.Application

	root    *tview.Flex
	bar     *tview.TextView
	pages   *tview.Pages
	data    *tview.Flex
	header  *tview.TextView
	list    *ActionList
	deploy  *deployPane
	section CleanupSection

	plan    gc.Report
	planErr error
	loaded  bool
	running bool
	result  *gc.Report
	ctx     context.Context
	cancel  context.CancelFunc
	// loads numbers the plans and the removals: a result is applied only
	// while it is the latest request.
	loads loadSlot
}

var _ View = (*GCView)(nil)

// gcHistoryID is the element of the history rows.
const gcHistoryID = "history"

// NewGCView builds the view.
func NewGCView(cfg GCViewConfig) *GCView {
	if cfg.Bytes == nil {
		cfg.Bytes = func(n int64) string { return fmt.Sprintf("%d B", n) }
	}
	return &GCView{cfg: cfg}
}

// SetShell implements the shell-aware interface.
func (v *GCView) SetShell(s ShellAccess) { v.shell = s }

func (v *GCView) ID() string    { return "gc" }
func (v *GCView) Title() string { return i18n.T("tui.cleanup.view_title") }

func (v *GCView) StatusHints() string {
	tab := ""
	if v.cfg.Deploy != nil {
		tab = i18n.T("tui.cleanup.tab")
	}
	if v.section == CleanupDeploy && v.deploy != nil {
		return keys.Hints(v.deploy.hints(), tab, keys.Back.Hint())
	}
	if v.list == nil {
		return keys.HintsOf(keys.Refresh, keys.Back)
	}
	return keys.Hints(keys.Navigate.Hint(), v.list.Hints(), keys.Refresh.HintAs(i18n.T("tui.gc.replan")), tab, keys.Back.Hint())
}

func (v *GCView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)
	v.running = false
	v.build()
	content.Clear()
	content.AddItem(v.root, 0, 1, true)
	v.reload()
	if v.deploy != nil {
		v.deploy.scan()
	}
	v.showSection(v.cfg.Section)
}

// showSection shows a section and gives it the focus.
func (v *GCView) showSection(s CleanupSection) {
	if v.deploy == nil {
		s = CleanupData
	}
	v.section = s
	if v.pages != nil {
		v.pages.SwitchToPage(map[CleanupSection]string{CleanupData: "data", CleanupDeploy: "deploy"}[s])
	}
	v.renderBar()
	if v.app != nil {
		if s == CleanupDeploy {
			v.app.SetFocus(v.deploy.text)
		} else {
			v.app.SetFocus(v.list.Focusable())
		}
	}
	if r, ok := v.shell.(HintsRefresher); ok {
		r.RefreshHints()
	}
}

// renderBar writes the sections, the one shown in the accent colour.
func (v *GCView) renderBar() {
	if v.bar == nil {
		return
	}
	var parts []string
	for _, s := range []CleanupSection{CleanupDeploy, CleanupData} {
		label := i18n.T(map[CleanupSection]string{CleanupData: "tui.cleanup.section.data", CleanupDeploy: "tui.cleanup.section.deploy"}[s])
		if s == v.section {
			parts = append(parts, theme.ColorTag(theme.ActiveMode.PrimaryHex)+tview.Escape("["+label+"]")+theme.TagColor)
		} else {
			parts = append(parts, " "+muted(label)+" ")
		}
	}
	v.bar.SetText(strings.Join(parts, " "))
}

func (v *GCView) Unmount() {
	v.loads.Invalidate()
	if v.deploy != nil {
		v.deploy.loads.Invalidate()
	}
	if v.cancel != nil {
		v.cancel()
	}
	v.app = nil
}

func (v *GCView) HandleKey(ev *tcell.EventKey) *tcell.EventKey {
	if v.deploy != nil && (ev.Key() == tcell.KeyTab || ev.Key() == tcell.KeyBacktab) {
		v.showSection(1 - v.section)
		return nil
	}
	if v.section == CleanupDeploy && v.deploy != nil {
		return v.deploy.handleKey(ev)
	}
	if keys.Refresh.Matches(ev) {
		v.reload()
		return nil
	}
	return ev
}

// build creates the widgets (no data).
func (v *GCView) build() {
	v.header = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	v.header.SetBackgroundColor(theme.BgPanel)
	v.header.SetBorderPadding(0, 0, 1, 1)
	v.list = NewActionList(ActionListConfig{
		Title:   i18n.T("tui.gc.title"),
		Headers: []string{i18n.T("tui.gc.col.what"), i18n.T("tui.gc.col.size"), i18n.T("tui.gc.col.where")},
		Layout:  DetailBelow,
		Empty:   i18n.T("tui.gc.nothing"),
		Shell:   v.shell,
		Actions: []ItemAction{{Key: keys.Delete, Confirm: v.confirmItem, Run: func(it ActionItem, _ string) { v.run(v.only(it)) }}},
		Global: []GlobalAction{{Key: keys.AcceptAll, Label: i18n.T("tui.gc.clean_all"), Danger: true,
			Confirm: func([]ActionItem) (string, string) { return v.confirmAll() },
			Run:     func([]ActionItem) { v.run(v.plan) }}},
	})
	v.data = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.header, 3, 0, false).
		AddItem(v.list.Primitive(), 0, 1, true)
	v.data.SetBackgroundColor(theme.BgPanel)
	v.pages = tview.NewPages().AddPage("data", v.data, true, true)
	v.root = tview.NewFlex().SetDirection(tview.FlexRow)
	v.root.SetBackgroundColor(theme.BgPanel)
	if v.cfg.Deploy != nil {
		v.deploy = newDeployPane(v)
		v.pages.AddPage("deploy", v.deploy.root, true, false)
		v.bar = tview.NewTextView().SetDynamicColors(true)
		v.bar.SetBackgroundColor(theme.BgPanel)
		v.bar.SetBorderPadding(0, 0, 1, 1)
		v.root.AddItem(v.bar, 1, 0, false)
	}
	v.root.AddItem(v.pages, 0, 1, true)
	v.renderHeader()
	v.renderBar()
}

// reload computes the plan again (dry run) off the event loop.
func (v *GCView) reload() {
	if v.cfg.Plan == nil || v.app == nil || v.running {
		return
	}
	v.loaded = false
	v.renderHeader()
	load(v.app, v.ctx, &v.loads, v.cfg.Plan, func(plan gc.Report, err error) { v.showPlan(plan, err) })
}

// showPlan shows a plan (event loop).
func (v *GCView) showPlan(plan gc.Report, err error) {
	v.plan, v.planErr, v.loaded = plan, err, true
	if v.list != nil {
		v.list.SetItems(v.items())
	}
	v.renderHeader()
}

// run removes only, after the confirmation, off the event loop; then the
// plan is computed again.
func (v *GCView) run(only gc.Report) {
	if v.cfg.Run == nil || v.running || v.app == nil {
		return
	}
	v.running = true
	v.renderHeader()
	load(v.app, v.ctx, &v.loads, func(ctx context.Context) (gc.Report, error) { return v.cfg.Run(ctx, only) },
		func(done gc.Report, err error) {
			v.running = false
			if err != nil {
				v.result = &gc.Report{Errors: []string{err.Error()}}
				if v.shell != nil {
					v.shell.ShowToastMsg(err.Error(), false)
				}
			} else {
				v.result = &done
				if v.shell != nil {
					v.shell.ShowToastMsg(v.resultLine(done), len(done.Errors) == 0)
				}
			}
			v.reload()
		})
}

// only is the plan of one element.
func (v *GCView) only(it ActionItem) gc.Report {
	if it.ID == gcHistoryID {
		return gc.Report{History: v.plan.History}
	}
	if x, ok := it.Ref.(gc.Item); ok {
		return gc.Report{Items: []gc.Item{x}}
	}
	return gc.Report{}
}

func (v *GCView) confirmItem(it ActionItem) (title, detail string) {
	detail = strings.Join(it.Cells, " · ") + "\n\n" + i18n.T("tui.gc.irreversible")
	return i18n.Tf("tui.gc.confirm_item", it.Cells[0]), detail
}

func (v *GCView) confirmAll() (title, detail string) {
	var lines []string
	by := v.plan.ByKind()
	for _, k := range gc.Kinds {
		if items := by[k]; len(items) > 0 {
			lines = append(lines, "• "+v.sectionTitle(k, items))
		}
	}
	if h := v.plan.History; h.Total() > 0 {
		lines = append(lines, "• "+i18n.Tf("tui.gc.history_rows", h.Total()))
	}
	return i18n.Tf("tui.gc.confirm_all", v.cfg.Bytes(v.plan.Bytes())), strings.Join(lines, "\n") + "\n\n" + i18n.T("tui.gc.irreversible")
}

// kindLabel is the name of a category.
func kindLabel(k gc.Kind) string { return i18n.T("tui.gc.kind." + string(k)) }

func (v *GCView) sectionTitle(k gc.Kind, items []gc.Item) string {
	var size int64
	for _, it := range items {
		size += it.Bytes
	}
	if k == gc.KindOrphans {
		return i18n.Tf("tui.gc.section_count", kindLabel(k), len(items))
	}
	return i18n.Tf("tui.gc.section", kindLabel(k), len(items), v.cfg.Bytes(size))
}

// items are the elements of the plan, by category, the biggest first.
func (v *GCView) items() []ActionItem {
	var out []ActionItem
	by := v.plan.ByKind()
	for _, k := range gc.Kinds {
		list := append([]gc.Item(nil), by[k]...)
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].Bytes > list[j].Bytes })
		section := v.sectionTitle(k, list)
		for _, it := range list {
			what := it.ID
			if what == "" {
				what = it.Path
			}
			size := v.cfg.Bytes(it.Bytes)
			if k == gc.KindOrphans {
				size = "—"
			}
			out = append(out, ActionItem{ID: string(it.Kind) + ":" + it.ID + ":" + it.Path, Section: section,
				Cells: []string{clipText(what, 28), size, clipLeft(it.Path, 44)}, DetailTitle: kindLabel(k), Detail: v.itemDetail(it), Ref: it})
		}
	}
	if h := v.plan.History; h.Total() > 0 {
		out = append(out, ActionItem{ID: gcHistoryID, Section: kindLabel(gc.KindHistory),
			Cells:       []string{i18n.Tf("tui.gc.history_rows", h.Total()), "—", ""},
			DetailTitle: kindLabel(gc.KindHistory),
			Detail: tview.Escape(i18n.Tf("tui.gc.history_detail", h.Sessions, h.AgentEvents, h.Decisions,
				h.UsageSessions+h.UsageProxy+h.BudgetExtra, h.WikiProposals))})
	}
	return out
}

func (v *GCView) itemDetail(it gc.Item) string {
	muted := "[" + theme.TextMutedHex + "]"
	var b strings.Builder
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s%s[-] %s\n", muted, pad(label, 12), tview.Escape(value))
		}
	}
	row(i18n.T("tui.gc.detail.id"), it.ID)
	row(i18n.T("tui.gc.detail.path"), it.Path)
	row(i18n.T("tui.gc.detail.repo"), it.Repo)
	if it.Kind != gc.KindOrphans {
		row(i18n.T("tui.gc.col.size"), v.cfg.Bytes(it.Bytes))
	}
	row(i18n.T("tui.gc.detail.reason"), it.Reason)
	if it.KeepResults {
		b.WriteString(muted + tview.Escape(i18n.T("tui.gc.detail.keep_results")) + "[-]\n")
	}
	return b.String()
}

func (v *GCView) resultLine(r gc.Report) string {
	return i18n.Tf("tui.gc.done", len(r.Items), v.cfg.Bytes(r.Bytes()), r.History.Total())
}

// renderHeader writes the state of the plan and the last result.
func (v *GCView) renderHeader() {
	if v.header == nil {
		return
	}
	muted := "[" + theme.TextMutedHex + "]"
	var lines []string
	switch {
	case v.running:
		lines = append(lines, "⠋ "+i18n.T("tui.gc.running"))
	case !v.loaded:
		lines = append(lines, "⠋ "+i18n.T("tui.gc.planning"))
	case v.planErr != nil:
		lines = append(lines, "["+theme.ErrorHex+"]✗ "+tview.Escape(v.planErr.Error())+"[-]")
	case v.plan.Empty():
		lines = append(lines, "["+theme.SuccessHex+"]✔ "+i18n.T("tui.gc.nothing")+"[-]")
	default:
		lines = append(lines, "[::b]"+i18n.Tf("tui.gc.plan", v.cfg.Bytes(v.plan.Bytes()))+"[::-]  "+muted+i18n.T("tui.gc.preview")+"[-]")
	}
	if r := v.result; r != nil {
		if len(r.Items) > 0 || r.History.Total() > 0 || len(r.Errors) == 0 {
			lines = append(lines, "["+theme.SuccessHex+"]✔ "+tview.Escape(v.resultLine(*r))+"[-]")
		}
		for _, e := range r.Errors {
			lines = append(lines, "["+theme.WarningHex+"]⚠ "+tview.Escape(e)+"[-]")
		}
	} else if v.cfg.Retention != "" {
		lines = append(lines, muted+tview.Escape(v.cfg.Retention)+"[-]")
	}
	v.header.SetText(strings.Join(lines, "\n"))
}
