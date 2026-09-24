package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MetricsViewConfig holds dependencies for the metrics view.
type MetricsViewConfig struct {
	AgentEvents domain.AgentEventStore // optional — if nil, agent table is hidden
}

// MetricsView displays real usage metrics from opencode's database.
type MetricsView struct {
	app    *tview.Application
	tv     *tview.TextView
	period string // "7d", "30d", "all"
	mode   string // "usage" or "agents"
	shell  ShellAccess
	cfg    MetricsViewConfig
}

var _ View = (*MetricsView)(nil)

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
	return fmt.Sprintf("7 %s · 3 %s · 0 %s · Tab %s · Esc %s", i18n.T("tui.hints.week"), i18n.T("tui.hints.month"), i18n.T("tui.hints.all"), i18n.T("tui.hints.usage_agents"), i18n.T("tui.hints.back"))
}

// Mount builds the metrics display with real data.
func (v *MetricsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.metrics.loading_full"), theme.TagColor))
	content.AddItem(v.tv, 0, 1, true)

	// Load metrics data asynchronously (opens database)
	go func() {
		text := v.buildRenderText()
		app.QueueUpdateDraw(func() {
			if v.tv == nil {
				return
			}
			v.tv.SetText(text)
		})
	}()
}

// Unmount cleans up resources.
func (v *MetricsView) Unmount() {
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
	case event.Rune() == '7':
		v.period = "7d"
		v.asyncRender()
		return nil
	case event.Rune() == '3':
		v.period = "30d"
		v.asyncRender()
		return nil
	case event.Rune() == '0':
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
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.metrics.loading"), theme.TagColor))
	go func() {
		text := v.buildRenderText()
		v.app.QueueUpdateDraw(func() {
			if v.tv == nil || v.app == nil {
				return
			}
			v.tv.SetText(text)
		})
	}()
}

// buildRenderText builds the metrics display text. Safe to call from any goroutine.
func (v *MetricsView) buildRenderText() string {
	if v.mode == "agents" {
		return v.buildAgentsText()
	}
	return v.buildUsageText()
}

func (v *MetricsView) buildAgentsText() string {
	if v.cfg.AgentEvents == nil {
		return "\n  [::b]" + i18n.T("tui.metrics.agent_telemetry") + theme.TagReset + "\n\n  " +
			theme.ColorTag(theme.TextSecondaryHex) + i18n.T("tui.metrics.agent_store_unavailable") + theme.TagColor +
			"\n\n  " + theme.ColorTag(theme.TextSecondaryHex) + i18n.T("tui.metrics.press_tab_usage") + theme.TagColor
	}

	projectID := ""
	scopeLabel := ""
	if v.shell != nil {
		if ap := v.shell.ActiveProject(); ap != nil {
			projectID = ap.ID
			scopeLabel = ap.Name
		}
	}

	metrics, err := v.cfg.AgentEvents.Metrics(context.Background(), projectID)
	if err != nil {
		return fmt.Sprintf("  %s", i18n.Tf("tui.metrics.error", err.Error()))
	}

	var sb strings.Builder
	title := i18n.T("tui.metrics.agent_telemetry")
	if scopeLabel != "" {
		title = i18n.Tf("tui.metrics.agent_telemetry_scoped", scopeLabel)
	}
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", title, theme.TagReset)

	if len(metrics) == 0 {
		fmt.Fprintf(&sb, "  %s%s%s\n",
			theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.no_agent_data"), theme.TagColor)
	} else {
		// Table header
		fmt.Fprintf(&sb, "  %s%-18s %6s %6s %8s %10s %10s %8s%s\n",
			theme.ColorTag(theme.TextSecondaryHex),
			i18n.T("tui.metrics.col_agent"), i18n.T("tui.metrics.col_runs"), i18n.T("tui.metrics.col_success"), i18n.T("tui.metrics.col_duration"), i18n.T("tui.metrics.col_tokens_in"), i18n.T("tui.metrics.col_tokens_out"), i18n.T("tui.metrics.col_cost"),
			theme.TagColor)
		fmt.Fprintf(&sb, "  %s%s%s\n",
			theme.ColorTag(theme.TextSecondaryHex),
			strings.Repeat("─", 76),
			theme.TagColor)

		for _, m := range metrics {
			name := m.AgentName
			if len(name) > 18 {
				name = name[:15] + "..."
			}
			fmt.Fprintf(&sb, "  %-18s %6d %5.0f%% %6.1fs %10s %10s   $%.2f\n",
				name,
				m.TotalRuns,
				m.SuccessRate,
				m.AvgDurationSec,
				formatTokens(m.TotalTokensIn),
				formatTokens(m.TotalTokensOut),
				m.TotalCostUSD,
			)
		}
	}

	fmt.Fprintf(&sb, "\n  %s%s%s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.agent_footer"), theme.TagColor)

	return sb.String()
}

func (v *MetricsView) buildUsageText() string {
	db, err := opencode.OpenStatsDB()
	if err != nil {
		return fmt.Sprintf(`
  [::b]%s%s

  %s%s%s
  %s%s%s

  %s%s%s
`,
			i18n.T("tui.metrics.title"),
			theme.TagReset,
			theme.ColorTag(theme.ErrorHex), i18n.T("tui.metrics.db_unavailable"), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), err.Error(), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.db_hint"), theme.TagColor,
		)
	}
	defer db.Close()

	// Determine scope: project-scoped or global
	var stats *opencode.AggregateStats
	var scopeLabel string
	if v.shell != nil {
		if ap := v.shell.ActiveProject(); ap != nil {
			stats, err = opencode.ProjectPeriodStats(db, ap.Path, v.period)
			scopeLabel = ap.Name
		}
	}
	if stats == nil {
		stats, err = opencode.PeriodStats(db, v.period)
	}
	if err != nil {
		return fmt.Sprintf("  %s", i18n.Tf("tui.metrics.error", err.Error()))
	}

	// Period label
	periodLabel := i18n.T("tui.metrics.period_all")
	switch v.period {
	case "7d":
		periodLabel = i18n.T("tui.metrics.period_7d")
	case "30d":
		periodLabel = i18n.T("tui.metrics.period_30d")
	}

	var sb strings.Builder
	title := i18n.Tf("tui.metrics.title_period", periodLabel)
	if scopeLabel != "" {
		title = i18n.Tf("tui.metrics.title_scoped_period", scopeLabel, periodLabel)
	}
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", title, theme.TagReset)

	// Main stats
	fmt.Fprintf(&sb, "  %s%-18s%s %d\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.label_sessions"), theme.TagColor, stats.TotalSessions)
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.label_tokens_in"), theme.TagColor, formatTokens(stats.TotalTokensIn))
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.label_tokens_out"), theme.TagColor, formatTokens(stats.TotalTokensOut))

	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		fmt.Fprintf(&sb, "  %s%-18s%s %.0f%%\n",
			theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.label_cache_ratio"), theme.TagColor, cacheRatio)
	}

	if stats.TotalCost > 0 {
		fmt.Fprintf(&sb, "  %s%-18s%s $%.2f\n",
			theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.label_estimated_cost"), theme.TagColor, stats.TotalCost)
	}

	fmt.Fprintf(&sb, "\n  %s%s%s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.metrics.usage_footer"), theme.TagColor)

	// Optimization suggestions
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", i18n.T("tui.metrics.suggestions_title"), theme.TagReset)

	hasSuggestions := false
	if stats.TotalTokensIn > 100_000 {
		fmt.Fprintf(&sb, "  %s•%s %s\n",
			theme.ColorTag(theme.ActiveMode.PrimaryHex), theme.TagColor, i18n.Tf("tui.metrics.suggest_high_tokens_in", formatTokens(stats.TotalTokensIn)))
		hasSuggestions = true
	}
	if stats.TotalSessions > 10 && stats.TotalTokensOut > 0 {
		avgOut := stats.TotalTokensOut / int64(stats.TotalSessions)
		if avgOut > 5000 {
			fmt.Fprintf(&sb, "  %s•%s %s\n",
				theme.ColorTag(theme.ActiveMode.PrimaryHex), theme.TagColor, i18n.Tf("tui.metrics.suggest_high_avg_out", formatTokens(avgOut)))
			hasSuggestions = true
		}
	}
	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		if cacheRatio < 30 {
			fmt.Fprintf(&sb, "  %s•%s %s\n",
				theme.ColorTag(theme.ActiveMode.PrimaryHex), theme.TagColor, i18n.Tf("tui.metrics.suggest_low_cache", fmt.Sprintf("%.0f%%", cacheRatio)))
			hasSuggestions = true
		}
	}
	if !hasSuggestions {
		fmt.Fprintf(&sb, "  %s%s%s\n",
			"[green]", i18n.T("tui.metrics.no_suggestions"), "[-]")
	}

	return sb.String()
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
