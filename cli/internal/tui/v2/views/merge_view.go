package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// MergeView — TUI view for proposing and executing branch merges
// ─────────────────────────────────────────────────────────────────────────────

// MergeBranch represents a completed branch ready for merge consideration.
type MergeBranch struct {
	TicketID     string
	Branch       string
	IsBeads      bool   // true = auto-mergeable; false = external (report only)
	DiffStat     string // git diff --stat summary (pre-computed)
	CommitCount  int    // number of commits ahead of base
	Duration     time.Duration
	Status       string // "pending", "merged", "skipped", "conflict"
}

// MergeViewConfig configures the merge view.
type MergeViewConfig struct {
	Branches  []MergeBranch
	MergeFunc func(branch MergeBranch) error // called inside app.Suspend to execute the merge
}

// MergeView implements View for the branch merge workflow.
type MergeView struct {
	cfg         MergeViewConfig
	list        *tview.List
	detailView  *tview.TextView
	contentFlex *tview.Flex
	app         *tview.Application
	shell       ShellAccess
}

var _ View = (*MergeView)(nil)

// NewMergeView creates a new merge view.
func NewMergeView(cfg MergeViewConfig) *MergeView {
	return &MergeView{cfg: cfg}
}

// SetShell injects the shell for modal dialogs and toasts.
func (v *MergeView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MergeView) ID() string { return "merge" }

// Title returns the display title.
func (v *MergeView) Title() string { return "Merge" }

// StatusHints returns keybinding hints.
func (v *MergeView) StatusHints() string {
	return fmt.Sprintf("j/k %s · m %s · s %s · Esc %s",
		i18n.T("tui.hints.branches"),
		i18n.T("tui.hints.merge"),
		i18n.T("tui.hints.skip"),
		i18n.T("tui.hints.back"),
	)
}

// Mount builds the merge view.
func (v *MergeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement des branches...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build UI and populate list asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // view was unmounted before the goroutine finished
			}

			// Branch list
			v.list = tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary)
			v.list.SetBackgroundColor(theme.BgPanel)
			v.list.SetBorderPadding(1, 0, 2, 2)

			// Detail panel (diff stats)
			v.detailView = tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignLeft)
			v.detailView.SetBackgroundColor(theme.BgPanel)
			v.detailView.SetBorderPadding(1, 0, 2, 2)

			// Wire selection to detail
			v.list.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
				if index >= 0 && index < len(v.cfg.Branches) {
					v.updateDetail(v.cfg.Branches[index])
				}
			})

			// Vertical layout: list (3/5) + detail (2/5)
			v.contentFlex = tview.NewFlex().SetDirection(tview.FlexRow)
			v.contentFlex.AddItem(v.list, 0, 3, true)
			v.contentFlex.AddItem(v.detailView, 0, 2, false)

			v.populateList()
			content.RemoveItem(loading)
			content.AddItem(v.contentFlex, 0, 1, true)
		})
	}()
}

// Unmount cleans up.
func (v *MergeView) Unmount() {
	v.app = nil
	v.list = nil
	v.detailView = nil
	v.contentFlex = nil
}

// HandleKey processes merge view key events.
func (v *MergeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'm':
		v.handleMerge()
		return nil
	case 's':
		v.handleSkip()
		return nil
	}
	return event
}

func (v *MergeView) handleMerge() {
	if v.app == nil || v.list == nil || v.cfg.MergeFunc == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Branches) {
		return
	}
	branch := &v.cfg.Branches[idx]
	if branch.Status != "pending" {
		return // already merged/skipped/conflict
	}
	if !branch.IsBeads {
		return // external branches can't be auto-merged
	}

	doMerge := func() {
		// Suspend TUI and run merge in terminal
		v.app.Suspend(func() {
			err := v.cfg.MergeFunc(*branch)
			if err != nil {
				branch.Status = "conflict"
			} else {
				branch.Status = "merged"
			}
		})
		v.app.Sync()
		v.populateList()
	}

	if v.shell != nil {
		v.shell.ShowSelectModal(fmt.Sprintf("Merger la branche %s ?", branch.Branch), []SelectOption{
			{Label: "Confirmer le merge", Value: "yes"},
			{Label: "Annuler", Value: ""},
		}, "", func(choice string) {
			if choice == "yes" {
				doMerge()
			}
		})
	} else {
		// Fallback without shell: merge directly (legacy behavior)
		doMerge()
	}
}

func (v *MergeView) handleSkip() {
	if v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Branches) {
		return
	}
	branch := &v.cfg.Branches[idx]
	if branch.Status != "pending" {
		return
	}
	branch.Status = "skipped"
	v.populateList()
}

func (v *MergeView) populateList() {
	if v.list == nil {
		return
	}
	v.list.Clear()

	if len(v.cfg.Branches) == 0 {
		muted := theme.ColorTag(theme.TextMutedHex)
		v.list.AddItem(fmt.Sprintf("%sAucune branche à merger.%s", muted, theme.TagColor), "", 0, nil)
		return
	}

	for _, b := range v.cfg.Branches {
		icon := mergeStatusIcon(b.Status, b.IsBeads)
		typeLabel := "beads"
		if !b.IsBeads {
			typeLabel = "external"
		}
		mainText := fmt.Sprintf("%s %s (%s)", icon, b.TicketID, typeLabel)
		secondary := fmt.Sprintf("  %s · %d commit(s) · %s", b.Branch, b.CommitCount, b.Duration.Round(time.Second))
		v.list.AddItem(mainText, secondary, 0, nil)
	}
	if len(v.cfg.Branches) > 0 {
		v.updateDetail(v.cfg.Branches[0])
	}
}

func (v *MergeView) updateDetail(b MergeBranch) {
	if v.detailView == nil {
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]%s%s — %s\n\n", b.TicketID, theme.TagReset, b.Branch))

	statusColor := theme.TextSecondaryHex
	switch b.Status {
	case "merged":
		statusColor = theme.SuccessHex
	case "conflict":
		statusColor = theme.ErrorHex
	case "skipped":
		statusColor = theme.TextMutedHex
	}
	sb.WriteString(fmt.Sprintf("  %sStatut:%s    %s%s%s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(statusColor), b.Status, theme.TagColor))

	if b.IsBeads {
		sb.WriteString(fmt.Sprintf("  %sType:%s      beads (auto-mergeable)\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	} else {
		sb.WriteString(fmt.Sprintf("  %sType:%s      external (merge manuel via MR/PR)\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	}

	sb.WriteString(fmt.Sprintf("  %sCommits:%s   %d\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, b.CommitCount))

	if b.DiffStat != "" {
		sb.WriteString(fmt.Sprintf("\n  %sDiff:%s\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		for _, line := range strings.Split(b.DiffStat, "\n") {
			sb.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}

	v.detailView.SetText(sb.String())
}

func mergeStatusIcon(status string, isBeads bool) string {
	switch status {
	case "merged":
		return theme.ColorTag(theme.SuccessHex) + "✓" + theme.TagColor
	case "skipped":
		return theme.ColorTag(theme.TextMutedHex) + "→" + theme.TagColor
	case "conflict":
		return theme.ColorTag(theme.ErrorHex) + "!" + theme.TagColor
	default: // pending
		if isBeads {
			return theme.ColorTag(theme.AccentHex) + "●" + theme.TagColor
		}
		return theme.ColorTag(theme.TextMutedHex) + "○" + theme.TagColor
	}
}
