package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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
}

// ProjectWikiView shows the metrics of the wiki of the active project.
type ProjectWikiView struct {
	app   *tview.Application
	tv    *tview.TextView
	shell ShellAccess
	cfg   ProjectWikiViewConfig
	gen   uint64
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
	return fmt.Sprintf("r %s · Esc %s", i18n.T("tui.hints.refresh"), i18n.T("tui.hints.back"))
}

// Mount builds the view and loads the metrics in the background.
func (v *ProjectWikiView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.tv = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)
	content.AddItem(v.tv, 0, 1, true)
	v.refresh()
}

func (v *ProjectWikiView) refresh() {
	if v.tv == nil {
		return
	}
	v.gen++
	gen := v.gen
	name, path := "", ""
	if v.cfg.Project != nil {
		name, path = v.cfg.Project()
	}
	if path == "" || v.cfg.Metrics == nil {
		v.tv.SetText("\n  " + muted(i18n.T("tui.project.no_active")))
		return
	}
	v.tv.SetText("\n  " + muted(i18n.T("tui.project_wiki.loading")))
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	app := v.app
	go func() {
		m, err := v.cfg.Metrics(ctx, path)
		text := renderProjectWiki(name, m, err)
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			if v.tv != nil && v.gen == gen {
				v.tv.SetText(text)
			}
		})
	}()
}

// renderProjectWiki renders the metrics of the wiki of a project.
func renderProjectWiki(project string, m ProjectWikiMetrics, err error) string {
	var b strings.Builder
	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	c := theme.TagColor
	fmt.Fprintf(&b, "\n  [::b]%s%s\n\n", tview.Escape(i18n.Tf("tui.project_wiki.heading", project)), theme.TagReset)
	switch {
	case err != nil:
		fmt.Fprintf(&b, "  %s\n", tview.Escape(i18n.Tf("tui.project_wiki.error", err.Error())))
		return b.String()
	case m.NoWiki:
		fmt.Fprintf(&b, "  %s\n", muted(i18n.T("tui.project_wiki.no_wiki")))
		return b.String()
	}
	row := func(label, value string) {
		fmt.Fprintf(&b, "  %s%-28s%s %s\n", accent, label, c, value)
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

// Unmount stops the pending load.
func (v *ProjectWikiView) Unmount() {
	v.gen++
	v.tv, v.app = nil, nil
}

// HandleKey refreshes on r.
func (v *ProjectWikiView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && (event.Rune() == 'r' || event.Rune() == 'R') {
		v.refresh()
		return nil
	}
	return event
}
