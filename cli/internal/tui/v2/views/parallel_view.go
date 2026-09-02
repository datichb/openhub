package views

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// ParallelView — View interface adapter for parallel session monitoring
// ─────────────────────────────────────────────────────────────────────────────

// ParallelViewConfig holds the data for the parallel view inside the shell.
type ParallelViewConfig struct {
	Sessions    []ParallelSession
	RefreshFunc func() []ParallelSession
	RefreshRate time.Duration
	AttachFunc  func(sessionID string) error // called to attach to a running session interactively
}

// ParallelView implements View for monitoring parallel sessions.
type ParallelView struct {
	cfg         ParallelViewConfig
	sessionList *tview.List
	detailView  *tview.TextView
	contentFlex *tview.Flex
	app         *tview.Application
	done        chan struct{}
	once        sync.Once
}

var _ View = (*ParallelView)(nil)

// NewParallelView creates a new parallel sessions monitor view.
func NewParallelView(cfg ParallelViewConfig) *ParallelView {
	return &ParallelView{cfg: cfg}
}

// ID returns the view identifier.
func (v *ParallelView) ID() string { return "parallel" }

// Title returns the display title.
func (v *ParallelView) Title() string { return "Parallel" }

// StatusHints returns keybinding hints.
func (v *ParallelView) StatusHints() string {
	return "j/k sessions · Enter attach · r refresh · Esc retour"
}

// Mount builds the parallel monitor and inserts it into the content panel.
func (v *ParallelView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.done = make(chan struct{})

	// Session list
	v.sessionList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.sessionList.SetBackgroundColor(theme.BgPanel)
	v.sessionList.SetBorderPadding(1, 0, 2, 2)

	// Detail footer
	v.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.detailView.SetBackgroundColor(theme.BgPanel)
	v.detailView.SetBorderPadding(0, 0, 2, 2)

	// Wire list selection to detail update
	v.sessionList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index >= 0 && index < len(v.cfg.Sessions) {
			v.updateDetail(v.cfg.Sessions[index])
		}
	})

	// Vertical layout: list on top, footer detail at bottom
	v.contentFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	v.contentFlex.AddItem(v.sessionList, 0, 3, true)
	v.contentFlex.AddItem(v.detailView, 5, 0, false)

	v.populateSessions(v.cfg.Sessions)
	content.AddItem(v.contentFlex, 0, 1, true)

	// Start refresh goroutine
	if v.cfg.RefreshFunc != nil {
		rate := v.cfg.RefreshRate
		if rate == 0 {
			rate = 3 * time.Second
		}
		go v.refreshLoop(rate)
	}
}

// Unmount stops the refresh goroutine.
func (v *ParallelView) Unmount() {
	v.once.Do(func() {
		if v.done != nil {
			close(v.done)
		}
	})
	v.app = nil
	v.sessionList = nil
	v.detailView = nil
	v.contentFlex = nil
	v.once = sync.Once{}
}

// HandleKey processes parallel view key events.
func (v *ParallelView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyEnter:
		if v.cfg.AttachFunc != nil && v.app != nil && v.sessionList != nil {
			idx := v.sessionList.GetCurrentItem()
			if idx >= 0 && idx < len(v.cfg.Sessions) {
				sess := v.cfg.Sessions[idx]
				if sess.Status == "running" && sess.SessionID != "" {
					v.app.Suspend(func() {
						_ = v.cfg.AttachFunc(sess.SessionID)
					})
					v.app.Sync()
				}
			}
		}
		return nil
	case event.Rune() == 'r':
		if v.cfg.RefreshFunc != nil {
			v.refresh()
		}
		return nil
	}
	return event
}

func (v *ParallelView) populateSessions(sessions []ParallelSession) {
	v.sessionList.Clear()

	if len(sessions) == 0 {
		muted := theme.ColorTag(theme.TextMutedHex)
		v.sessionList.AddItem(fmt.Sprintf("%sAucune session parallèle en cours.%s", muted, theme.TagColor), "", 0, nil)
		return
	}

	for _, s := range sessions {
		icon := parallelStatusIcon(s.Status)
		mainText := fmt.Sprintf("%s %s", icon, s.Name)
		secondary := fmt.Sprintf("  %s · %s", s.Branch, s.Agent)
		v.sessionList.AddItem(mainText, secondary, 0, nil)
	}
	if len(sessions) > 0 {
		v.updateDetail(sessions[0])
	}
}

func (v *ParallelView) updateDetail(s ParallelSession) {
	if v.detailView == nil {
		return
	}
	statusColor := parallelStatusColorHex(s.Status)
	sep := theme.ColorTag(theme.TextMutedHex) + "─────────────────────────────────────────────" + theme.TagColor
	v.detailView.SetText(fmt.Sprintf("%s\n  [::b]%s%s  %s%s%s  %s·%s  %s  %s·%s  %s  %s·%s  %s",
		sep,
		s.Name, theme.TagReset,
		theme.ColorTag(statusColor), s.Status, theme.TagColor,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		s.Branch,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		s.Agent,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		s.Duration.Round(time.Second).String(),
	))
}

func (v *ParallelView) refreshLoop(rate time.Duration) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-v.done:
			return
		case <-ticker.C:
			v.refresh()
		}
	}
}

func (v *ParallelView) refresh() {
	if v.cfg.RefreshFunc == nil || v.app == nil {
		return
	}
	sessions := v.cfg.RefreshFunc()
	v.cfg.Sessions = sessions
	v.app.QueueUpdateDraw(func() {
	v.populateSessions(v.cfg.Sessions)
	})
}

func parallelStatusIcon(status string) string {
	switch status {
	case "running":
		return theme.ColorTag(theme.InfoHex) + theme.IconDone + theme.TagColor
	case "done":
		return theme.ColorTag(theme.SuccessHex) + theme.IconSuccess + theme.TagColor
	case "conflict":
		return theme.ColorTag(theme.ErrorHex) + theme.IconWarning + theme.TagColor
	case "idle":
		return theme.ColorTag(theme.TextMutedHex) + theme.IconPending + theme.TagColor
	default:
		return theme.ColorTag(theme.TextMutedHex) + theme.IconDot + theme.TagColor
	}
}

func parallelStatusColorHex(status string) string {
	switch status {
	case "running":
		return theme.InfoHex
	case "done":
		return theme.SuccessHex
	case "conflict":
		return theme.ErrorHex
	default:
		return theme.TextMutedHex
	}
}
