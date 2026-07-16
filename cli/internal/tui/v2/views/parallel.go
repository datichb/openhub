package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Parallel — session monitor for parallel coding sessions
// ─────────────────────────────────────────────────────────────────────────────

// ParallelSession represents a running coding session.
type ParallelSession struct {
	ID       string
	Name     string
	Status   string // "running", "idle", "conflict", "done"
	Branch   string
	Duration time.Duration
	Agent    string
}

// ParallelConfig configures the parallel monitor.
type ParallelConfig struct {
	Layout      layout.Config
	Sessions    []ParallelSession
	RefreshFunc func() []ParallelSession
	RefreshRate time.Duration
}

// RunParallel launches the full-screen parallel session monitor.
func RunParallel(cfg ParallelConfig) error {
	shell := layout.Build(cfg.Layout)

	// ── Session list ──
	sessionList := tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	sessionList.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderFocus).
		SetTitle(" Sessions ").
		SetTitleColor(theme.Accent)

	// ── Detail panel ──
	detailView := tview.NewTextView().
		SetDynamicColors(true)
	detailView.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderNormal).
		SetTitle(" Details ").
		SetTitleColor(theme.FgSecondary)

	// ── Populate sessions ──
	populateSessions := func(sessions []ParallelSession) {
		sessionList.Clear()
		for _, s := range sessions {
			icon := statusIcon(s.Status)
			color := statusColor(s.Status)
			sessionList.AddItem(
				fmt.Sprintf("[%s]%s[-] %s", widgets.ColorTag(color), icon, s.Name),
				fmt.Sprintf("  %s · %s · %s", s.Branch, s.Agent, s.Duration.Round(time.Second).String()),
				0, nil,
			)
		}
	}
	populateSessions(cfg.Sessions)

	// Update detail on selection change
	sessionList.SetChangedFunc(func(idx int, _, _ string, _ rune) {
		if idx >= 0 && idx < len(cfg.Sessions) {
			s := cfg.Sessions[idx]
			detailView.SetText(fmt.Sprintf(
				"  %sID[-]       %s\n"+
					"  %sName[-]     %s\n"+
					"  %sStatus[-]   [%s]%s[-]\n"+
					"  %sBranch[-]   %s\n"+
					"  %sAgent[-]    %s\n"+
					"  %sDuration[-] %s\n",
				widgets.ColorTag(theme.FgSecondary), s.ID,
				widgets.ColorTag(theme.FgSecondary), s.Name,
				widgets.ColorTag(theme.FgSecondary), widgets.ColorTag(statusColor(s.Status)), s.Status,
				widgets.ColorTag(theme.FgSecondary), s.Branch,
				widgets.ColorTag(theme.FgSecondary), s.Agent,
				widgets.ColorTag(theme.FgSecondary), s.Duration.Round(time.Second).String(),
			))
		}
	})

	// ── Layout: session list + detail side by side ──
	contentFlex := tview.NewFlex().
		AddItem(sessionList, 0, 1, true).
		AddItem(detailView, 0, 1, false)
	contentFlex.SetBackgroundColor(theme.BgPanel)

	shell.Content.AddItem(contentFlex, 0, 1, true)

	// ── Refresh ──
	done := make(chan struct{})
	if cfg.RefreshFunc != nil {
		rate := cfg.RefreshRate
		if rate == 0 {
			rate = 3 * time.Second
		}
		go func() {
			ticker := time.NewTicker(rate)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					sessions := cfg.RefreshFunc()
					shell.App.QueueUpdateDraw(func() {
						cfg.Sessions = sessions
						populateSessions(sessions)
					})
				}
			}
		}()
	}

	// ── Input ──
	shell.Content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape || event.Rune() == 'q':
			close(done)
			shell.App.Stop()
			return nil
		case event.Rune() == 'r':
			if cfg.RefreshFunc != nil {
				sessions := cfg.RefreshFunc()
				cfg.Sessions = sessions
				populateSessions(sessions)
			}
			return nil
		}
		return event
	})

	return shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run()
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return "●"
	case "idle":
		return "○"
	case "conflict":
		return "!"
	case "done":
		return "✓"
	default:
		return "·"
	}
}

func statusColor(status string) tcell.Color {
	switch status {
	case "running":
		return theme.Accent
	case "idle":
		return theme.FgSecondary
	case "conflict":
		return theme.Error
	case "done":
		return theme.Success
	default:
		return theme.FgMuted
	}
}
