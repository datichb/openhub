package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MetricsView displays real usage metrics from opencode's database.
type MetricsView struct {
	app    *tview.Application
	tv     *tview.TextView
	period string // "7d", "30d", "all"
}

var _ View = (*MetricsView)(nil)

// NewMetricsView creates a new metrics view.
func NewMetricsView() *MetricsView {
	return &MetricsView{period: "all"}
}

// ID returns the view identifier.
func (v *MetricsView) ID() string { return "metrics" }

// Title returns the display title.
func (v *MetricsView) Title() string { return "Métriques" }

// StatusHints returns keybinding hints.
func (v *MetricsView) StatusHints() string {
	return "7 semaine · 3 mois · a tout · Esc retour"
}

// Mount builds the metrics display with real data.
func (v *MetricsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	v.render()
	content.AddItem(v.tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *MetricsView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes metrics view key events (period switching).
func (v *MetricsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case '7':
		v.period = "7d"
		v.render()
		return nil
	case '3':
		v.period = "30d"
		v.render()
		return nil
	case 'a':
		v.period = "all"
		v.render()
		return nil
	}
	return event
}

func (v *MetricsView) render() {
	if v.tv == nil {
		return
	}

	db, err := opencode.OpenStatsDB()
	if err != nil {
		v.tv.SetText(fmt.Sprintf(`
  [::b]Métriques%s

  %sBase de données opencode non accessible :%s
  %s%s%s

  %sVérifiez que opencode a été utilisé au moins une fois.%s
`,
			theme.TagReset,
			theme.ColorTag(theme.ErrorHex), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), err.Error(), theme.TagColor,
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		))
		return
	}
	defer db.Close()

	stats, err := opencode.PeriodStats(db, v.period)
	if err != nil {
		v.tv.SetText(fmt.Sprintf("  Erreur: %s", err.Error()))
		return
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
	sb.WriteString(fmt.Sprintf("\n  [::b]Métriques — %s%s\n\n", periodLabel, theme.TagReset))

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

	v.tv.SetText(sb.String())
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
