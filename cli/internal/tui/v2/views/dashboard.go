package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Dashboard — multi-panel stats view
// ─────────────────────────────────────────────────────────────────────────────

// DashboardStat represents a single stat entry.
type DashboardStat struct {
	Label string
	Value string
	Color tcell.Color // optional, defaults to FgPrimary
}

// TokenBar represents a bar in the token usage chart.
type TokenBar struct {
	Label   string
	Current int
	Max     int
}

// DashboardConfig configures the dashboard view.
type DashboardConfig struct {
	Layout      layout.Config
	Stats       []DashboardStat
	TokenUsage  []TokenBar
	RecentItems []string
}

// RunDashboard launches the full-screen dashboard.
func RunDashboard(cfg DashboardConfig) error {
	shell := layout.Build(cfg.Layout)

	// ── Stats panel ──
	statsView := tview.NewTextView().
		SetDynamicColors(true)
	statsView.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderNormal).
		SetTitle(" Stats ").
		SetTitleColor(theme.Accent)

	var statsContent strings.Builder
	for _, stat := range cfg.Stats {
		color := theme.FgPrimary
		if stat.Color != 0 {
			color = stat.Color
		}
		statsContent.WriteString(fmt.Sprintf("  %s%-16s[-] %s%s[-]\n",
			widgets.ColorTag(theme.FgSecondary), stat.Label,
			widgets.ColorTag(color), stat.Value))
	}
	statsView.SetText(statsContent.String())

	// ── Token usage panel (bar chart) ──
	tokenView := tview.NewTextView().
		SetDynamicColors(true)
	tokenView.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderNormal).
		SetTitle(" Token Usage ").
		SetTitleColor(theme.Accent)

	var tokenContent strings.Builder
	for _, bar := range cfg.TokenUsage {
		pct := 0
		if bar.Max > 0 {
			pct = (bar.Current * 100) / bar.Max
		}
		barWidth := 30
		filled := (pct * barWidth) / 100
		if filled > barWidth {
			filled = barWidth
		}
		barColor := theme.Success
		if pct > 80 {
			barColor = theme.Error
		} else if pct > 60 {
			barColor = theme.Warning
		}
		tokenContent.WriteString(fmt.Sprintf("  %-12s %s%s[-]%s%s[-] %d%%\n",
			bar.Label,
			widgets.ColorTag(barColor), strings.Repeat("█", filled),
			widgets.ColorTag(theme.FgMuted), strings.Repeat("░", barWidth-filled),
			pct))
	}
	if len(cfg.TokenUsage) == 0 {
		tokenContent.WriteString(fmt.Sprintf("  %sNo data[-]", widgets.ColorTag(theme.FgMuted)))
	}
	tokenView.SetText(tokenContent.String())

	// ── Recent activity panel ──
	activityView := tview.NewTextView().
		SetDynamicColors(true)
	activityView.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderNormal).
		SetTitle(" Recent Activity ").
		SetTitleColor(theme.Accent)

	var activityContent strings.Builder
	for _, item := range cfg.RecentItems {
		activityContent.WriteString(fmt.Sprintf("  %s%s[-] %s\n",
			widgets.ColorTag(theme.FgMuted), theme.IconDot, item))
	}
	if len(cfg.RecentItems) == 0 {
		activityContent.WriteString(fmt.Sprintf("  %sNo recent activity[-]", widgets.ColorTag(theme.FgMuted)))
	}
	activityView.SetText(activityContent.String())

	// ── Layout: 2-column grid in top, activity full width bottom ──
	topRow := tview.NewFlex().
		AddItem(statsView, 0, 1, false).
		AddItem(tokenView, 0, 1, false)
	topRow.SetBackgroundColor(theme.BgPanel)

	shell.Content.AddItem(topRow, 0, 1, false)
	shell.Content.AddItem(activityView, 0, 1, true)

	// ── Input ──
	shell.Content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			shell.App.Stop()
			return nil
		}
		return event
	})

	return shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run()
}
