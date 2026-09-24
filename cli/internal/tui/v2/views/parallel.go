package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Parallel — session monitor for parallel coding sessions
// ─────────────────────────────────────────────────────────────────────────────

// ParallelSession represents a running coding session.
type ParallelSession struct {
	ID              string
	Name            string
	Status          string // "running", "idle", "retrying", "failed", "pending", "starting", "completed", "aborted"
	Branch          string
	Duration        time.Duration
	Agent           string
	SessionID       string // opencode session ID (for attach/resume)
	WorktreePath    string // working directory for the session
	Priority        bool
	EstimateMinutes int
	Error           string
	FilesModified   int
	FilesCreated    int
	FilesList       []string
	ConflictCount   int
	MaxSeverity     string           // "high", "medium", "low", ""
	Conflicts       []ConflictDetail // detailed conflicts for this session
	RetryCount      int
	RetryErrors     []string
}

// ConflictDetail holds per-file conflict info for a session.
type ConflictDetail struct {
	File     string
	Others   []string // other ticket IDs involved
	Severity string   // "high", "medium", "low"
}

// ParallelConfig configures the parallel monitor.
type ParallelConfig struct {
	Layout      layout.Config
	Sessions    []ParallelSession
	RefreshFunc func() []ParallelSession
	Phase       string // global phase: "setup", "running", "merging", "done"
	RefreshRate time.Duration
	AttachFunc  func(sessionID string) error // called to attach to a running session interactively
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
		SetBorderColor(theme.ActiveMode.Primary).
		SetTitle(" Sessions ").
		SetTitleColor(theme.ActiveMode.Primary)

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

			// Main text: [color]icon[-] [★ ]name
			name := s.Name
			if s.Priority {
				name = "★ " + name
			}
			mainText := fmt.Sprintf("[%s]%s[-] %s", widgets.ColorTag(color), icon, name)

			// Secondary text: branch · agent · duration [· NM/NC] [· ⚠ conflicts] [· retry N]
			secondary := fmt.Sprintf("  %s · %s · %s", s.Branch, s.Agent, s.Duration.Round(time.Second).String())
			if s.FilesModified > 0 || s.FilesCreated > 0 {
				secondary += fmt.Sprintf(" · %dM/%dC", s.FilesModified, s.FilesCreated)
			}
			if s.ConflictCount > 0 {
				secondary += fmt.Sprintf(" · ⚠ %d conflicts (%s)", s.ConflictCount, s.MaxSeverity)
			}
			if s.RetryCount > 0 {
				secondary += fmt.Sprintf(" · retry %d", s.RetryCount)
			}

			sessionList.AddItem(mainText, secondary, 0, nil)
		}
	}
	populateSessions(cfg.Sessions)

	// Update detail on selection change
	sessionList.SetChangedFunc(func(idx int, _, _ string, _ rune) {
		if idx >= 0 && idx < len(cfg.Sessions) {
			s := cfg.Sessions[idx]
			detail := fmt.Sprintf(
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
			)

			// Files
			if s.FilesModified > 0 || s.FilesCreated > 0 {
				detail += fmt.Sprintf("  %sFiles[-]    %dM / %dC\n",
					widgets.ColorTag(theme.FgSecondary), s.FilesModified, s.FilesCreated)
				if len(s.FilesList) > 0 && len(s.FilesList) <= 10 {
					detail += fmt.Sprintf("  %sFilesList[-] %s\n",
						widgets.ColorTag(theme.FgSecondary), strings.Join(s.FilesList, ", "))
				}
			}

			// Conflicts
			if s.ConflictCount > 0 {
				sevColor := theme.Warning
				if s.MaxSeverity == "high" {
					sevColor = theme.Error
				}
				detail += fmt.Sprintf("  %sConflicts[-] [%s]%d (%s)[-]\n",
					widgets.ColorTag(theme.FgSecondary), widgets.ColorTag(sevColor), s.ConflictCount, s.MaxSeverity)
			}

			// Error
			if s.Error != "" {
				detail += fmt.Sprintf("  %sError[-]    [%s]%s[-]\n",
					widgets.ColorTag(theme.FgSecondary), widgets.ColorTag(theme.Error), s.Error)
			}

			// Recovery / Retry
			if s.RetryCount > 0 {
				prevErrs := "none"
				if len(s.RetryErrors) > 0 {
					prevErrs = strings.Join(s.RetryErrors, ", ")
				}
				detail += fmt.Sprintf("  %sRecovery[-] attempt %d, previous: [%s]\n",
					widgets.ColorTag(theme.FgSecondary), s.RetryCount, prevErrs)
			}

			// Progress bar
			if s.EstimateMinutes > 0 && s.Status == "running" {
				bar := renderProgressBarTcell(s.Duration, s.EstimateMinutes, 20)
				if bar != "" {
					detail += fmt.Sprintf("  %sProgress[-] %s\n",
						widgets.ColorTag(theme.FgSecondary), bar)
				}
			}

			detailView.SetText(detail)
		}
	})

	// ── Layout: session list + detail side by side ──
	contentFlex := tview.NewFlex().
		AddItem(sessionList, 0, 1, true).
		AddItem(detailView, 0, 1, false)
	contentFlex.SetBackgroundColor(theme.BgPanel)

	// ── Phase indicator ──
	if cfg.Phase != "" {
		phaseColor := theme.FgSecondary
		switch cfg.Phase {
		case "running":
			phaseColor = theme.ActiveMode.Primary
		case "merging":
			phaseColor = theme.Warning
		case "done":
			phaseColor = theme.Success
		}
		phaseView := tview.NewTextView().
			SetDynamicColors(true).
			SetText(fmt.Sprintf(" [%s]Phase: %s[-]", widgets.ColorTag(phaseColor), cfg.Phase))
		phaseView.SetBackgroundColor(theme.BgPanel)

		wrapper := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(phaseView, 1, 0, false).
			AddItem(contentFlex, 0, 1, true)
		wrapper.SetBackgroundColor(theme.BgPanel)
		shell.Content.AddItem(wrapper, 0, 1, true)
	} else {
		shell.Content.AddItem(contentFlex, 0, 1, true)
	}

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
		case event.Key() == tcell.KeyEnter:
			if cfg.AttachFunc != nil {
				idx := sessionList.GetCurrentItem()
				if idx >= 0 && idx < len(cfg.Sessions) {
					sess := cfg.Sessions[idx]
					if sess.Status == "running" && sess.SessionID != "" {
						shell.App.Suspend(func() {
							_ = cfg.AttachFunc(sess.SessionID)
						})
						shell.App.Sync()
					}
				}
			}
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
	case "retrying":
		return "↻"
	case "failed":
		return "✗"
	case "pending":
		return "○"
	case "starting":
		return "◐"
	case "aborted":
		return "⊘"
	case "completed":
		return "✓"
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
		return theme.ActiveMode.Primary
	case "idle":
		return theme.FgSecondary
	case "retrying":
		return theme.Warning
	case "failed":
		return theme.Error
	case "pending":
		return theme.FgMuted
	case "starting":
		return theme.FgSecondary
	case "aborted":
		return theme.FgMuted
	case "completed":
		return theme.Success
	case "conflict":
		return theme.Error
	case "done":
		return theme.Success
	default:
		return theme.FgMuted
	}
}

// renderProgressBarTcell builds a text-based progress bar for the tcell TUI.
func renderProgressBarTcell(elapsed time.Duration, estimateMin int, width int) string {
	if estimateMin <= 0 {
		return ""
	}
	pct := int(elapsed.Minutes() / float64(estimateMin) * 100)
	if pct > 100 {
		pct = 100
	}
	filled := width * pct / 100
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %d%%", bar, pct)
}
