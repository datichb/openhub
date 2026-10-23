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

// Metrics of the project wiki (modernisation LD-I): coverage of the
// domains, median age of the pages, share of INCERTAIN and proposals
// waiting, computed by oh (WikiService.Metrics). The view only shows them;
// page paths come from the project: escaped.

// ProjectWikiMetrics are the figures the view shows.
type ProjectWikiMetrics struct {
	// NoWiki: the project has no docs/wiki/.
	NoWiki bool

	Pages, Specs, Decisions, Postmortems int

	Domains, DomainsCovered int
	Uncovered               []string

	MedianAgeDays, Dated int
	Oldest               string
	OldestDays           int

	Tagged, Uncertain, Confirmed, Deduced int

	// Pending and Accepted are the open proposals (-1: unknown).
	Pending, Accepted int
	Synced            string
}

// ProjectWikiViewConfig holds the dependencies of the view.
type ProjectWikiViewConfig struct {
	// Project returns the name and the path of the active project ("" none).
	Project func() (name, path string)
	// Metrics computes the figures of the wiki of a project.
	Metrics func(ctx context.Context, path string) (ProjectWikiMetrics, error)
	// Proposals reads the queue of the project: pending, accepted (to
	// apply) and stale proposals (nil: metrics only).
	Proposals func(ctx context.Context, path string) ([]WikiProposalView, error)
	// Decide records the decision of the user on a proposal (`oh wiki
	// decide`); content is the new content of an edit, or the optional
	// reason of a refusal.
	Decide func(ctx context.Context, path, id, choice, content string) error
	// Edit edits a content in $EDITOR, the TUI suspended (event loop).
	Edit func(name, content string) (string, error)
	// Now is the clock of the ages (tests).
	Now func() time.Time
}

// ProjectWikiView shows the wiki of the active project: a summary of its
// metrics (m: all of them) and the queue of its proposals outside any
// checkpoint (tui-wiki-bricks), decided like on the card of the commit
// checkpoint: pending (workflows without a commit, doc-sync), accepted to
// apply at the next commit, stale.
type ProjectWikiView struct {
	app     *tview.Application
	tv      *tview.TextView
	shell   ShellAccess
	cfg     ProjectWikiViewConfig
	loads   loadSlot // the reads of the metrics: only the latest is applied
	wiki    *wikiList
	name    string
	path    string
	metrics ProjectWikiMetrics
	merr    error
	loaded  bool
}

var _ View = (*ProjectWikiView)(nil)

// NewProjectWikiView creates the view.
func NewProjectWikiView(cfg ProjectWikiViewConfig) *ProjectWikiView {
	return &ProjectWikiView{cfg: cfg}
}

// SetShell provides the shell reference.
func (v *ProjectWikiView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ProjectWikiView) ID() string { return "project.wiki" }

// Title returns the display title.
func (v *ProjectWikiView) Title() string { return i18n.T("tui.project_wiki.title") }

// StatusHints returns keybinding hints.
func (v *ProjectWikiView) StatusHints() string {
	parts := []string{}
	if v.wiki != nil {
		parts = append(parts, v.wiki.hints(), keys.Filter.Hint())
	}
	return keys.Hints(append(parts, metricsKey.Hint(), keys.Refresh.Hint(), keys.Back.Hint())...)
}

// Mount builds the view and loads the metrics and the queue in the
// background.
func (v *ProjectWikiView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.tv = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(0, 0, 1, 1)
	v.name, v.path = "", ""
	if v.cfg.Project != nil {
		v.name, v.path = v.cfg.Project()
	}
	if v.cfg.Proposals == nil {
		v.wiki = nil
		v.tv.SetBorderPadding(1, 0, 2, 2)
		content.AddItem(v.tv, 0, 1, true)
		v.refresh()
		return
	}
	path := v.path
	opts := wikiListOpts{
		Title:  i18n.Tf("tui.project_wiki.heading", v.name),
		Layout: DetailRight,
		Queue:  true,
		Shell:  v.shell,
		Load:   func(ctx context.Context) ([]WikiProposalView, error) { return v.cfg.Proposals(ctx, path) },
		Edit:   v.cfg.Edit,
		Now:    v.cfg.Now,
	}
	if v.cfg.Decide != nil {
		opts.Decide = func(ctx context.Context, id, choice, content string) error {
			return v.cfg.Decide(ctx, path, id, choice, content)
		}
	}
	v.wiki = newWikiList(opts)
	box := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.tv, 2, 0, false).
		AddItem(v.wiki.list.Primitive(), 0, 1, true)
	box.SetBackgroundColor(theme.BgPanel)
	content.AddItem(box, 0, 1, true)
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	v.wiki.mount(app, ctx)
	v.refresh()
}

func (v *ProjectWikiView) refresh() {
	if v.tv == nil {
		return
	}
	name, path := v.name, v.path
	if path == "" || v.cfg.Metrics == nil {
		v.loads.Invalidate()
		v.tv.SetText("  " + stateText(stateEmpty, i18n.T("tui.project.no_active")))
		return
	}
	v.tv.SetText("  " + stateText(stateLoading, i18n.T("tui.project_wiki.loading")))
	if v.wiki != nil {
		v.wiki.reload()
	}
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	full, metrics := v.wiki == nil, v.cfg.Metrics
	load(v.app, ctx, &v.loads, func(ctx context.Context) (ProjectWikiMetrics, error) {
		return metrics(ctx, path)
	}, func(m ProjectWikiMetrics, err error) {
		if v.tv == nil {
			return
		}
		v.metrics, v.merr, v.loaded = m, err, true
		if full {
			v.tv.SetText(renderProjectWiki(name, m, err))
			return
		}
		v.tv.SetText(renderProjectWikiSummary(m, err))
	})
}

// renderProjectWikiSummary is the line of the metrics above the queue.
func renderProjectWikiSummary(m ProjectWikiMetrics, err error) string {
	switch {
	case err != nil:
		return "  " + stateText(stateError, err.Error())
	case m.NoWiki:
		return "  " + stateText(stateEmpty, i18n.T("tui.project_wiki.no_wiki"))
	}
	parts := []string{i18n.Tf("tui.project_wiki.summary.pages", m.Pages)}
	if m.Domains > 0 {
		parts = append(parts, i18n.Tf("tui.project_wiki.summary.coverage", wikiPercent(m.DomainsCovered, m.Domains)))
	}
	if m.Tagged > 0 {
		parts = append(parts, i18n.Tf("tui.project_wiki.summary.confirmed", wikiPercent(m.Confirmed, m.Tagged)),
			i18n.Tf("tui.project_wiki.summary.uncertain", wikiPercent(m.Uncertain, m.Tagged)))
	}
	if m.Synced != "" {
		parts = append(parts, i18n.Tf("tui.project_wiki.summary.synced", wikiShortRef(m.Synced)))
	} else {
		parts = append(parts, i18n.T("tui.project_wiki.summary.never_synced"))
	}
	return "  " + tview.Escape(strings.Join(parts, " · ")) + "  " + muted(i18n.T("tui.project_wiki.summary.more"))
}

// renderProjectWiki renders the metrics of the wiki of a project.
func renderProjectWiki(project string, m ProjectWikiMetrics, err error) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), tview.Escape(i18n.Tf("tui.project_wiki.heading", project)), theme.TagAttrs)
	switch {
	case err != nil:
		fmt.Fprintf(&b, "  %s\n", stateText(stateError, err.Error()))
		return b.String()
	case m.NoWiki:
		fmt.Fprintf(&b, "  %s\n", stateText(stateEmpty, i18n.T("tui.project_wiki.no_wiki")))
		return b.String()
	}
	row := func(label, value string) {
		fmt.Fprintf(&b, "  %s %s\n", theme.Paint(theme.ActiveMode.PrimaryHex, fmt.Sprintf("%-28s", label)), value)
	}
	row(i18n.T("tui.project_wiki.pages"), i18n.Tf("tui.project_wiki.pages_value", m.Pages, m.Specs, m.Decisions, m.Postmortems))
	if m.Domains == 0 {
		row(i18n.T("tui.project_wiki.coverage"), muted(i18n.T("tui.project_wiki.no_domain")))
	} else {
		pct := wikiPercent(m.DomainsCovered, m.Domains)
		row(i18n.T("tui.project_wiki.coverage"), fmt.Sprintf("%s %d %% (%d/%d)", wikiGauge(pct), pct, m.DomainsCovered, m.Domains))
		if len(m.Uncovered) > 0 {
			fmt.Fprintf(&b, "  %-28s %s\n", "", muted(tview.Escape(i18n.Tf("tui.project_wiki.uncovered", strings.Join(wikiClip(m.Uncovered, 5), ", ")))))
		}
	}
	if m.Dated == 0 {
		row(i18n.T("tui.project_wiki.age"), muted(i18n.T("tui.project_wiki.no_date")))
	} else {
		row(i18n.T("tui.project_wiki.age"), tview.Escape(i18n.Tf("tui.project_wiki.age_value", m.MedianAgeDays, m.Oldest, m.OldestDays)))
	}
	if m.Tagged == 0 {
		row(i18n.T("tui.project_wiki.uncertain"), muted(i18n.T("tui.project_wiki.no_tag")))
	} else {
		pct := wikiPercent(m.Uncertain, m.Tagged)
		row(i18n.T("tui.project_wiki.uncertain"), i18n.Tf("tui.project_wiki.uncertain_value", pct, m.Uncertain, m.Tagged, m.Confirmed, m.Deduced))
	}
	switch {
	case m.Pending < 0:
		row(i18n.T("tui.project_wiki.pending"), muted(i18n.T("tui.project_wiki.no_queue")))
	default:
		row(i18n.T("tui.project_wiki.pending"), i18n.Tf("tui.project_wiki.pending_value", m.Pending, m.Accepted))
	}
	if m.Synced != "" {
		row(i18n.T("tui.project_wiki.synced"), tview.Escape(wikiShortRef(m.Synced)))
	} else {
		row(i18n.T("tui.project_wiki.synced"), muted(i18n.T("tui.project_wiki.never_synced")))
	}
	if m.Pending > 0 || m.Accepted > 0 {
		fmt.Fprintf(&b, "\n  %s\n", muted(i18n.T("tui.project_wiki.hint_sync")))
	}
	return b.String()
}

func wikiPercent(n, total int) int {
	if total <= 0 {
		return 0
	}
	return (n*100 + total/2) / total
}

// wikiGauge renders a share as a bar of ten cells.
func wikiGauge(pct int) string {
	full := (pct + 5) / 10
	if full > 10 {
		full = 10
	}
	return strings.Repeat("█", full) + muted(strings.Repeat("░", 10-full))
}

func wikiClip(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return append(append([]string(nil), list[:n]...), fmt.Sprintf("+%d", len(list)-n))
}

func wikiShortRef(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

// Unmount stops the pending loads.
func (v *ProjectWikiView) Unmount() {
	v.loads.Invalidate()
	if v.wiki != nil {
		v.wiki.unmount()
	}
	v.tv, v.app = nil, nil
}

// HandleKey: r refreshes, m shows every metric, / filters the queue; the
// decisions go to the list.
func (v *ProjectWikiView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.refresh()
		return nil
	case metricsKey.Matches(event):
		v.showMetrics()
		return nil
	case v.wiki != nil && keys.Filter.Matches(event):
		if v.shell != nil {
			v.shell.ShowInputModal(i18n.T("tui.project_wiki.filter"), v.wiki.query, v.wiki.setQuery)
		}
		return nil
	}
	if v.wiki != nil {
		return v.wiki.list.HandleKey(event)
	}
	return event
}

// metricsKey shows every metric of the wiki.
var metricsKey = keys.Own("project_wiki.metrics", 'm', "tui.project_wiki.key.metrics")

// showMetrics shows every metric of the wiki (m).
func (v *ProjectWikiView) showMetrics() {
	if v.shell == nil || !v.loaded {
		return
	}
	v.shell.ShowScrollableModal(tview.Escape(i18n.Tf("tui.project_wiki.heading", v.name)), renderProjectWiki(v.name, v.metrics, v.merr), nil)
}
