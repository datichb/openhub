package cmd

import (
	"context"
	"strings"
	"sync"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI screen of `oh migrate deploy-cleanup` (P3-T28, 10-tui §12): shown
// once after the update when former deployments remain: what goes, what
// is kept, Nettoyer / Voir le diff / Plus tard. The same clean-up is the
// section « Anciens déploiements » of the clean-up view (`cleanup` omnibar
// command, tui_gc.go). The projects are scanned off the event loop.

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

// deployCleanupConfig is the section « Anciens déploiements » of the
// clean-up view: Apply cleans the projects of the last scan.
func deployCleanupConfig(a *app.App) *views.DeployCleanupConfig {
	var (
		mu   sync.Mutex
		last []projectCleanup
	)
	return &views.DeployCleanupConfig{
		Scan: func(ctx context.Context) (views.DeployCleanupPlan, error) {
			list, err := scanDeployLeftovers(ctx, a, "")
			if err != nil {
				return views.DeployCleanupPlan{}, err
			}
			mu.Lock()
			last = list
			mu.Unlock()
			return views.DeployCleanupPlan{Projects: len(list), Text: cleanupText(list),
				DiffTitle: i18n.Tf("tui.cleanup.diff_title", cleanupConfigFile(list)), Diff: cleanupDiffText(list)}, nil
		},
		Apply: func(context.Context) (string, bool) {
			mu.Lock()
			list := last
			mu.Unlock()
			return applyCleanup(list)
		},
	}
}

func showCleanupScreen(a *app.App, sh *shell.Shell, list []projectCleanup) {
	sh.ShowScrollableModal(i18n.T("tui.cleanup.title"), cleanupText(list), cleanupActions(a, sh, list, true))
}

// cleanupText is what goes and what is kept, by project.
func cleanupText(list []projectCleanup) string {
	var b strings.Builder
	b.WriteString(i18n.T("tui.cleanup.intro") + "\n\n")
	for _, pc := range list {
		b.WriteString("[::b]" + tview.Escape(pc.Name) + "[::-]  " + theme.ColorTag(theme.TextMutedHex) + tview.Escape(pc.Project.Path) + theme.TagColor + "\n")
		for _, l := range cleanupLines(pc.Plan) {
			b.WriteString("  " + tview.Escape(l) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(theme.ColorTag(theme.TextMutedHex) + i18n.Tf("tui.cleanup.kept_note", cleanupConfigFile(list)) + theme.TagColor)
	return b.String()
}

func cleanupActions(a *app.App, sh *shell.Shell, list []projectCleanup, withDiff bool) []views.ModalAction {
	actions := []views.ModalAction{{Label: i18n.T("tui.cleanup.apply"), Callback: func() { applyCleanupInTUI(sh, list) }}}
	if withDiff {
		actions = append(actions, views.ModalAction{Label: i18n.T("tui.cleanup.diff"), Callback: func() { showCleanupDiff(a, sh, list) }})
	}
	return append(actions, views.ModalAction{Label: i18n.T("tui.cleanup.later"), Callback: func() {}, Separator: true})
}

func showCleanupDiff(a *app.App, sh *shell.Shell, list []projectCleanup) {
	sh.ShowScrollableModal(i18n.Tf("tui.cleanup.diff_title", cleanupConfigFile(list)), cleanupDiffText(list), cleanupActions(a, sh, list, false))
}

// cleanupDiffText is the diff of the configuration, by project.
func cleanupDiffText(list []projectCleanup) string {
	var b strings.Builder
	for _, pc := range list {
		b.WriteString("[::b]" + tview.Escape(pc.Name) + "[::-]\n")
		d := pc.Plan.Diff()
		if d == "" {
			d = i18n.Tf("tui.cleanup.no_diff", pc.Plan.ConfigFile) + "\n"
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
	return b.String()
}

// cleanupConfigFile is the tool configuration file the plans clean.
func cleanupConfigFile(list []projectCleanup) string {
	for _, pc := range list {
		if pc.Plan != nil && pc.Plan.ConfigFile != "" {
			return pc.Plan.ConfigFile
		}
	}
	return ""
}

func applyCleanupInTUI(sh *shell.Shell, list []projectCleanup) {
	go func() {
		msg, ok := applyCleanup(list)
		sh.App().QueueUpdateDraw(func() { sh.ShowToastMsg(msg, ok) })
	}()
}

// applyCleanup cleans the projects: the message of the result and whether
// everything was cleaned.
func applyCleanup(list []projectCleanup) (string, bool) {
	var failed []string
	for _, pc := range list {
		if err := pc.Plan.Apply(); err != nil {
			failed = append(failed, pc.Name+": "+err.Error())
		}
	}
	if len(failed) > 0 {
		return i18n.Tf("tui.cleanup.failed", strings.Join(failed, "; ")), false
	}
	return i18n.Tf("tui.cleanup.done", len(list)), true
}
