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
func (v *MetricsView) Title() string { return "Métriques" }

// StatusHints returns keybinding hints.
func (v *MetricsView) StatusHints() string {
	return fmt.Sprintf("7 %s · 3 %s · a %s · Tab %s · Esc %s", i18n.T("tui.hints.week"), i18n.T("tui.hints.month"), i18n.T("tui.hints.all"), i18n.T("tui.hints.usage_agents"), i18n.T("tui.hints.back"))
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
	v.tv.SetText(fmt.Sprintf("\n  %sChargement des métriques...%s", muted, theme.TagColor))
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
	case event.Rune() == 'a':
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
	v.tv.SetText(fmt.Sprintf("\n  %sChargement...%s", muted, theme.TagColor))
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
		return "\n  [::b]Télémétrie agents" + theme.TagReset + "\n\n  " +
			theme.ColorTag(theme.TextSecondaryHex) + "Agent event store non disponible." + theme.TagColor +
			"\n\n  " + theme.ColorTag(theme.TextSecondaryHex) + "Appuyez sur Tab pour revenir aux métriques d'usage." + theme.TagColor
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
		return fmt.Sprintf("  Erreur: %s", err.Error())
	}

	var sb strings.Builder
	title := "Télémétrie agents"
	if scopeLabel != "" {
		title = fmt.Sprintf("Télémétrie agents · %s", scopeLabel)
	}
	sb.WriteString(fmt.Sprintf("\n  [::b]%s%s\n\n", title, theme.TagReset))

	if len(metrics) == 0 {
		sb.WriteString(fmt.Sprintf("  %sAucune donnée d'agent disponible.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	} else {
		// Table header
		sb.WriteString(fmt.Sprintf("  %s%-18s %6s %6s %8s %10s %10s %8s%s\n",
			theme.ColorTag(theme.TextSecondaryHex),
			"Agent", "Runs", "Succ%", "Durée", "Tokens In", "Tokens Out", "Coût",
			theme.TagColor))
		sb.WriteString(fmt.Sprintf("  %s%s%s\n",
			theme.ColorTag(theme.TextSecondaryHex),
			strings.Repeat("─", 76),
			theme.TagColor))

		for _, m := range metrics {
			name := m.AgentName
			if len(name) > 18 {
				name = name[:15] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %-18s %6d %5.0f%% %6.1fs %10s %10s   $%.2f\n",
				name,
				m.TotalRuns,
				m.SuccessRate,
				m.AvgDurationSec,
				formatTokens(m.TotalTokensIn),
				formatTokens(m.TotalTokensOut),
				m.TotalCostUSD,
			))
		}
	}

	sb.WriteString(fmt.Sprintf("\n  %s─── Tab: basculer usage/agents · [7] semaine · [3] mois · [a] tout ───%s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))

	return sb.String()
}

func (v *MetricsView) buildUsageText() string {
	db, err := opencode.OpenStatsDB()
	if err != nil {
		return fmt.Sprintf(`
  [::b]Métriques%s

  %sBase de données opencode non accessible :%s
  %s%s%s

  %sVérifiez que opencode a été utilisé au moins une fois.%s
`,
			theme.TagReset,
			theme.ColorTag(theme.ErrorHex), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), err.Error(), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
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
		return fmt.Sprintf("  Erreur: %s", err.Error())
	}

	// Period label
	periodLabel := "toutes périodes"
	switch v.period {
	case "7d":
		periodLabel = "7 derniers jours"
	case "30d":
		periodLabel = "30 derniers jours"
	}

	var sb strings.Builder
	title := fmt.Sprintf("Métriques — %s", periodLabel)
	if scopeLabel != "" {
		title = fmt.Sprintf("Métriques · %s — %s", scopeLabel, periodLabel)
	}
	sb.WriteString(fmt.Sprintf("\n  [::b]%s%s\n\n", title, theme.TagReset))

	// Main stats
	sb.WriteString(fmt.Sprintf("  %sSessions :%s          %d\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, stats.TotalSessions))
	sb.WriteString(fmt.Sprintf("  %sTokens in :%s         %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, formatTokens(stats.TotalTokensIn)))
	sb.WriteString(fmt.Sprintf("  %sTokens out :%s        %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, formatTokens(stats.TotalTokensOut)))

	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		sb.WriteString(fmt.Sprintf("  %sCache ratio :%s       %.0f%%\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, cacheRatio))
	}

	if stats.TotalCost > 0 {
		sb.WriteString(fmt.Sprintf("  %sCoût estimé :%s      $%.2f\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, stats.TotalCost))
	}

	sb.WriteString(fmt.Sprintf("\n  %s─── Période: [7] semaine · [3] mois · [a] tout ───%s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))

	// Optimization suggestions
	sb.WriteString(fmt.Sprintf("\n  [::b]Suggestions%s\n\n", theme.TagReset))

	hasSuggestions := false
	if stats.TotalTokensIn > 100_000 {
		sb.WriteString(fmt.Sprintf("  %s•%s Tokens in élevés (%s) — pensez au pattern RTK pour réduire le contexte\n",
			theme.ColorTag(theme.AccentHex), theme.TagColor, formatTokens(stats.TotalTokensIn)))
		hasSuggestions = true
	}
	if stats.TotalSessions > 10 && stats.TotalTokensOut > 0 {
		avgOut := stats.TotalTokensOut / int64(stats.TotalSessions)
		if avgOut > 5000 {
			sb.WriteString(fmt.Sprintf("  %s•%s Output moyen élevé (%s/session) — des réponses plus concises réduisent les coûts\n",
				theme.ColorTag(theme.AccentHex), theme.TagColor, formatTokens(avgOut)))
			hasSuggestions = true
		}
	}
	if stats.TotalTokensIn > 0 {
		cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100
		if cacheRatio < 30 {
			sb.WriteString(fmt.Sprintf("  %s•%s Cache ratio faible (%.0f%%) — activez la compaction pour améliorer le cache\n",
				theme.ColorTag(theme.AccentHex), theme.TagColor, cacheRatio))
			hasSuggestions = true
		}
	}
	if !hasSuggestions {
		sb.WriteString(fmt.Sprintf("  %s✓ Aucune suggestion — utilisation optimale%s\n",
			"[green]", "[-]"))
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
