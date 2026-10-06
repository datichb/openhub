package cmd

import (
	"context"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI screen of `oh migrate deploy-cleanup` (P3-T28, 10-tui §12): shown
// once after the update when former deployments remain (and from the
// `cleanup` omnibar command): what goes, what is kept, Nettoyer / Voir le
// diff / Plus tard. The projects are scanned off the event loop.

// offerCleanupInTUI opens the cleanup screen once (startup).
func offerCleanupInTUI(ctx context.Context, a *app.App, sh *shell.Shell) {
	if sh == nil || cleanupOffered(ctx, a) {
		return
	}
	go func() {
		list, err := scanDeployLeftovers(ctx, a, "")
		if err != nil {
			return
		}
		markCleanupOffered(ctx, a)
		if len(list) > 0 {
			sh.App().QueueUpdateDraw(func() { showCleanupScreen(a, sh, list) })
		}
	}()
}

// actionCleanup is the `cleanup` omnibar command.
func actionCleanup() {
	sh := tuiShell
	if sh == nil {
		return
	}
	a := MustApp()
	go func() {
		list, err := scanDeployLeftovers(context.Background(), a, "")
		sh.App().QueueUpdateDraw(func() {
			switch {
			case err != nil:
				sh.ShowToastMsg(err.Error(), false)
			case len(list) == 0:
				sh.ShowToastMsg(i18n.T("cmd.migrate.cleanup.none"), true)
			default:
				showCleanupScreen(a, sh, list)
			}
		})
	}()
}

func showCleanupScreen(a *app.App, sh *shell.Shell, list []projectCleanup) {
	var b strings.Builder
	b.WriteString(i18n.T("tui.cleanup.intro") + "\n\n")
	for _, pc := range list {
		b.WriteString("[::b]" + tview.Escape(pc.Name) + "[::-]  " + theme.ColorTag(theme.TextMutedHex) + tview.Escape(pc.Project.Path) + theme.TagColor + "\n")
		for _, l := range cleanupLines(pc.Plan) {
			b.WriteString("  " + tview.Escape(l) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(theme.ColorTag(theme.TextMutedHex) + i18n.T("tui.cleanup.kept_note") + theme.TagColor)
	sh.ShowScrollableModal(i18n.T("tui.cleanup.title"), b.String(), cleanupActions(a, sh, list, true))
}

func cleanupActions(a *app.App, sh *shell.Shell, list []projectCleanup, withDiff bool) []views.ModalAction {
	actions := []views.ModalAction{{Label: i18n.T("tui.cleanup.apply"), Callback: func() { applyCleanupInTUI(sh, list) }}}
	if withDiff {
		actions = append(actions, views.ModalAction{Label: i18n.T("tui.cleanup.diff"), Callback: func() { showCleanupDiff(a, sh, list) }})
	}
	return append(actions, views.ModalAction{Label: i18n.T("tui.cleanup.later"), Callback: func() {}, Separator: true})
}

func showCleanupDiff(a *app.App, sh *shell.Shell, list []projectCleanup) {
	var b strings.Builder
	for _, pc := range list {
		b.WriteString("[::b]" + tview.Escape(pc.Name) + "[::-]\n")
		d := pc.Plan.Diff()
		if d == "" {
			d = i18n.T("tui.cleanup.no_diff") + "\n"
		}
		for _, line := range strings.SplitAfter(d, "\n") {
			color := ""
			switch {
			case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
				color = theme.ColorTag(theme.ErrorHex)
			case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
				color = theme.ColorTag(theme.SuccessHex)
			}
			if color != "" {
				b.WriteString(color + tview.Escape(line) + theme.TagColor)
			} else {
				b.WriteString(tview.Escape(line))
			}
		}
		b.WriteString("\n")
	}
	sh.ShowScrollableModal(i18n.T("tui.cleanup.diff_title"), b.String(), cleanupActions(a, sh, list, false))
}

func applyCleanupInTUI(sh *shell.Shell, list []projectCleanup) {
	go func() {
		var failed []string
		for _, pc := range list {
			if err := pc.Plan.Apply(); err != nil {
				failed = append(failed, pc.Name+": "+err.Error())
			}
		}
		sh.App().QueueUpdateDraw(func() {
			if len(failed) > 0 {
				sh.ShowToastMsg(i18n.Tf("tui.cleanup.failed", strings.Join(failed, "; ")), false)
				return
			}
			sh.ShowToastMsg(i18n.Tf("tui.cleanup.done", len(list)), true)
		})
	}()
}
