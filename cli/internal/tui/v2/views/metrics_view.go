package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// MetricsViewConfig holds dependencies for the metrics view.
type MetricsViewConfig struct {
	AgentEvents domain.AgentEventStore // optional — if nil, agent table is hidden
	Stats       platform.StatsProvider // platform stats provider (ADR-036)
	// TeamProjects lists the projects of a team (team mode scope).
	TeamProjects func(teamID string) []ActiveProject
}

// MetricsView displays the usage metrics of the session registry.
type MetricsView struct {
	app    *tview.Application
	tv     *tview.TextView
	period string // "7d", "30d", "all"
	mode   string // "usage" or "agents"
	shell  ShellAccess
	cfg    MetricsViewConfig
	ctx    context.Context
	cancel context.CancelFunc
	loads  loadSlot
}

var _ View = (*MetricsView)(nil)

// metricsScope is what the metrics cover: the whole hub, a team (its
// projects) or a project.
type metricsScope struct {
	label    string
	projects []ActiveProject // nil = whole hub
	teamID   string          // set until the projects of the team are listed (resolveScope)
}

// scopeRef reads the shell (event loop only). The projects of a team are
// listed later, by resolveScope.
func (v *MetricsView) scopeRef() metricsScope {
	if v.shell == nil {
		return metricsScope{label: i18n.T("tui.metrics.scope_hub")}
	}
	switch v.shell.Mode() {
	case ModeProject:
		if ap := v.shell.ActiveProject(); ap != nil {
			return metricsScope{label: ap.Name, projects: []ActiveProject{*ap}}
		}
	case ModeTeam:
		if t := v.shell.ActiveTeam(); t != nil {
			return metricsScope{label: i18n.Tf("tui.metrics.scope_team", t.Name), projects: []ActiveProject{}, teamID: t.ID}
		}
	}
	return metricsScope{label: i18n.T("tui.metrics.scope_hub")}
}

// resolveScope lists the projects of a team scope (it may do I/O: off the
// event loop).
func (v *MetricsView) resolveScope(sc metricsScope) metricsScope {
	if sc.teamID != "" && v.cfg.TeamProjects != nil {
		sc.projects = v.cfg.TeamProjects(sc.teamID)
	}
	sc.teamID = ""
	return sc
}

func (v *MetricsView) scope() metricsScope { return v.resolveScope(v.scopeRef()) }

// metricsQuery is what a load needs from the view, read on the event loop
// before the load starts.
type metricsQuery struct {
	mode, period string
	scope        metricsScope
}

func (v *MetricsView) query() metricsQuery {
	return metricsQuery{mode: v.mode, period: v.period, scope: v.scopeRef()}
}

// mergeAgentMetrics sums the metrics of several projects, by agent.
func mergeAgentMetrics(lists ...[]domain.AgentMetrics) []domain.AgentMetrics {
	byName := map[string]*domain.AgentMetrics{}
	var order []string
	for _, list := range lists {
		for _, m := range list {
			acc, ok := byName[m.AgentName]
			if !ok {
				c := m
				c.TopSkills = append([]string(nil), m.TopSkills...)
				byName[m.AgentName] = &c
				order = append(order, m.AgentName)
				continue
			}
			runs := acc.TotalRuns + m.TotalRuns
			if runs > 0 {
				acc.AvgDurationSec = (acc.AvgDurationSec*float64(acc.TotalRuns) + m.AvgDurationSec*float64(m.TotalRuns)) / float64(runs)
			}
			acc.TotalRuns = runs
			acc.SuccessCount += m.SuccessCount
			acc.FailureCount += m.FailureCount
			acc.TotalTokensIn += m.TotalTokensIn
			acc.TotalTokensOut += m.TotalTokensOut
			acc.TotalCostUSD += m.TotalCostUSD
		}
	}
	out := make([]domain.AgentMetrics, 0, len(order))
	for _, name := range order {
		m := byName[name]
		if m.TotalRuns > 0 {
			m.SuccessRate = float64(m.SuccessCount) / float64(m.TotalRuns) * 100
		}
		out = append(out, *m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TotalRuns > out[j].TotalRuns })
	return out
}

// addStats adds b to a (the projects of a team).
func addStats(a, b *platform.AggregateStats) {
	a.TotalSessions += b.TotalSessions
	a.TotalTokensIn += b.TotalTokensIn
	a.TotalTokensOut += b.TotalTokensOut
	a.TotalCost += b.TotalCost
	a.TodaySessions += b.TodaySessions
	a.ActiveProjects += b.ActiveProjects
	a.CacheReadTokens += b.CacheReadTokens
	a.ReasoningTokens += b.ReasoningTokens
}

// NewMetricsView creates a new metrics view.
func NewMetricsView(cfg MetricsViewConfig) *MetricsView {
	return &MetricsView{period: "all", mode: "usage", cfg: cfg}
}

// SetShell provides the shell reference (used to read the active project).
func (v *MetricsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MetricsView) ID() string { return "metrics" }

// Title returns the display title.
func (v *MetricsView) Title() string { return i18n.T("tui.metrics.title") }

// StatusHints returns keybinding hints.
func (v *MetricsView) StatusHints() string {
	return keys.Hints(weekKey.Hint(), monthKey.Hint(), allKey.Hint(), "Tab "+i18n.T("tui.hints.usage_agents"), keys.Back.HintAs(i18n.T("tui.hints.back")))
}

// The periods of the metrics: 7 (week), 3 (month), 0 (all).
var (
	weekKey  = keys.Own("metrics.week", '7', "tui.hints.week")
	monthKey = keys.Own("metrics.month", '3', "tui.hints.month")
	allKey   = keys.Own("metrics.all", '0', "tui.hints.all")
)

// Mount builds the metrics display with real data.
func (v *MetricsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	parent := context.Background()
	if v.shell != nil {
		parent = v.shell.Context()
	}
	v.ctx, v.cancel = context.WithCancel(parent)

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	setState(v.tv, stateLoading, i18n.T("tui.metrics.loading_full"))
	content.AddItem(v.tv, 0, 1, true)

	v.reload()
}

// Unmount cleans up resources.
func (v *MetricsView) Unmount() {
	v.loads.Invalidate()
	if v.cancel != nil {
		v.cancel()
	}
	v.app = nil
	v.tv = nil
}

// HandleKey processes metrics view key events (period switching).
func (v *MetricsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyTab:
		if v.mode == "usage" {
			v.mode = "agents"
		} else {
			v.mode = "usage"
		}
		v.asyncRender()
		return nil
	case weekKey.Matches(event):
		v.period = "7d"
		v.asyncRender()
		return nil
	case monthKey.Matches(event):
		v.period = "30d"
		v.asyncRender()
		return nil
	case allKey.Matches(event):
		v.period = "all"
		v.asyncRender()
		return nil
	}
	return event
}

func (v *MetricsView) asyncRender() {
	if v.tv == nil {
		return
	}
	setState(v.tv, stateLoading, i18n.T("tui.metrics.loading"))
	v.reload()
}

// reload builds the text off the event loop (it opens the database). What it
// reads of the view (mode, period, scope) is captured here, on the loop.
func (v *MetricsView) reload() {
	q := v.query()
	load(v.app, v.ctx, &v.loads, func(ctx context.Context) (string, error) {
		return v.buildRenderText(ctx, q), nil
	}, func(text string, _ error) {
		if v.tv == nil {
			return
		}
		v.tv.SetText(text)
	})
}

// buildRenderText builds the metrics display text. It reads nothing of the
// view state (q has it): safe to call from any goroutine.
func (v *MetricsView) buildRenderText(ctx context.Context, q metricsQuery) string {
	sc := v.resolveScope(q.scope)
	if q.mode == "agents" {
		return v.buildAgentsText(ctx, sc)
	}
	return v.buildUsageText(sc, q.period)
}

func (v *MetricsView) buildAgentsText(ctx context.Context, sc metricsScope) string {
	if v.cfg.AgentEvents == nil {
		return "\n  " + theme.StyleTag(theme.AttrBold) + i18n.T("tui.metrics.agent_telemetry") + theme.TagAttrs + "\n\n  " +
			theme.Paint(theme.TextSecondaryHex, i18n.T("tui.metrics.agent_store_unavailable")) +
			"\n\n  " + theme.Paint(theme.TextSecondaryHex, i18n.T("tui.metrics.press_tab_usage"))
	}

	scopeLabel := sc.label
	var metrics []domain.AgentMetrics
	var err error
	if sc.projects == nil {
		metrics, err = v.cfg.AgentEvents.Metrics(ctx, "")
	} else {
		lists := make([][]domain.AgentMetrics, 0, len(sc.projects))
		for _, p := range sc.projects {
			m, e := v.cfg.AgentEvents.Metrics(ctx, p.ID)
			if e != nil {
				err = e
				break
			}
			lists = append(lists, m)
		}
		metrics = mergeAgentMetrics(lists...)
	}
	if err != nil {
		return "  " + stateText(stateError, err.Error())
	}

	var sb strings.Builder
	title := i18n.Tf("tui.metrics.agent_telemetry_scoped", scopeLabel)
	fmt.Fprintf(&sb, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), title, theme.TagAttrs)

	if len(metrics) == 0 {
		sb.WriteString("  " + stateText(stateEmpty, i18n.T("tui.metrics.no_agent_data")) + "\n")
	} else {
		// Table header
		fmt.Fprintf(&sb, "  %s\n", theme.Paint(theme.TextSecondaryHex, fmt.Sprintf("%-18s %6s %6s %8s %10s %10s %8s",
			i18n.T("tui.metrics.col_agent"), i18n.T("tui.metrics.col_runs"), i18n.T("tui.metrics.col_success"), i18n.T("tui.metrics.col_duration"), i18n.T("tui.metrics.col_tokens_in"), i18n.T("tui.metrics.col_tokens_out"), i18n.T("tui.metrics.col_cost"))))
		fmt.Fprintf(&sb, "  %s\n", theme.Paint(theme.TextSecondaryHex, strings.Repeat("─", 76)))

		for _, m := range metrics {
			fmt.Fprintf(&sb, "  %-18s %6d %5.0f%% %6.1fs %10s %10s %8s\n",
				textfmt.Clip(m.AgentName, 18),
				m.TotalRuns,
				m.SuccessRate,
				m.AvgDurationSec,
				textfmt.Tokens(m.TotalTokensIn),
				textfmt.Tokens(m.TotalTokensOut),
				textfmt.Cost(m.TotalCostUSD),
			)
		}
	}

	fmt.Fprintf(&sb, "\n  %s\n", theme.Paint(theme.TextSecondaryHex, i18n.T("tui.metrics.agent_footer")))

	return sb.String()
}

func (v *MetricsView) buildUsageText(sc metricsScope, period string) string {
	if v.cfg.Stats == nil || !v.cfg.Stats.Available() {
		return fmt.Sprintf("\n  %s%s%s\n\n  %s\n\n  %s\n",
			theme.StyleTag(theme.AttrBold), i18n.T("tui.metrics.title"), theme.TagAttrs,
			theme.Paint(theme.ErrorHex, theme.SymFailure+" "+i18n.T("tui.metrics.db_unavailable")),
			theme.Paint(theme.TextSecondaryHex, i18n.T("tui.metrics.db_hint")),
		)
	}

	// Scope: the whole hub, the projects of the team, or the project.
	scopeLabel := sc.label
	var stats *platform.AggregateStats
	var err error
	if sc.projects == nil {
		stats, err = v.cfg.Stats.AggregateStats(period)
	} else {
		stats = &platform.AggregateStats{}
		for _, p := range sc.projects {
			s, e := v.cfg.Stats.ProjectStats(p.Path, period)
			if e != nil {
				err = e
				break
			}
			addStats(stats, s)
		}
	}
	if err != nil {
		return "  " + stateText(stateError, err.Error())
	}

	// Period label
	periodLabel := i18n.T("tui.metrics.period_all")
	switch period {
	case "7d":
		periodLabel = i18n.T("tui.metrics.period_7d")
	case "30d":
		periodLabel = i18n.T("tui.metrics.period_30d")
	}

	var sb strings.Builder
	title := i18n.Tf("tui.metrics.title_scoped_period", scopeLabel, periodLabel)
	fmt.Fprintf(&sb, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), title, theme.TagAttrs)

	// Main stats
	fmt.Fprintf(&sb, "  %s %d\n", secondaryLabel(i18n.T("tui.metrics.label_sessions")), stats.TotalSessions)
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.metrics.label_tokens_in")), textfmt.Tokens(stats.TotalTokensIn))
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.metrics.label_tokens_out")), textfmt.Tokens(stats.TotalTokensOut))

	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		fmt.Fprintf(&sb, "  %s %.0f%%\n", secondaryLabel(i18n.T("tui.metrics.label_cache_ratio")), cacheRatio)
	}

	if stats.TotalCost > 0 {
		fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.metrics.label_estimated_cost")), textfmt.Cost(stats.TotalCost))
	}

	fmt.Fprintf(&sb, "\n  %s\n", theme.Paint(theme.TextSecondaryHex, i18n.T("tui.metrics.usage_footer")))

	// Optimization suggestions
	fmt.Fprintf(&sb, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), i18n.T("tui.metrics.suggestions_title"), theme.TagAttrs)

	bullet := theme.Paint(theme.ActiveMode.PrimaryHex, "•")
	hasSuggestions := false
	if stats.TotalTokensIn > 100_000 {
		fmt.Fprintf(&sb, "  %s %s\n", bullet, i18n.Tf("tui.metrics.suggest_high_tokens_in", textfmt.Tokens(stats.TotalTokensIn)))
		hasSuggestions = true
	}
	if stats.TotalSessions > 10 && stats.TotalTokensOut > 0 {
		avgOut := stats.TotalTokensOut / int64(stats.TotalSessions)
		if avgOut > 5000 {
			fmt.Fprintf(&sb, "  %s %s\n", bullet, i18n.Tf("tui.metrics.suggest_high_avg_out", textfmt.Tokens(avgOut)))
			hasSuggestions = true
		}
	}
	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		if cacheRatio < 30 {
			fmt.Fprintf(&sb, "  %s %s\n", bullet, i18n.Tf("tui.metrics.suggest_low_cache", fmt.Sprintf("%.0f%%", cacheRatio)))
			hasSuggestions = true
		}
	}
	if !hasSuggestions {
		fmt.Fprintf(&sb, "  %s\n", theme.Paint(theme.SuccessHex, i18n.T("tui.metrics.no_suggestions")))
	}

	return sb.String()
}
