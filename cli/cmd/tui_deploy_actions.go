package cmd

import (
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Deploy / Sync / Upgrade actions
// ─────────────────────────────────────────────────────────────────────────────

// onDeployComplete is called after a successful deploy to notify the project
// mode view so it can refresh its deploy status badge. Set during TUI wiring.
var onDeployComplete func()

func notifyDeployComplete() {
	if onDeployComplete != nil {
		onDeployComplete()
	}
}

func actionDeploy() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	if a.Projects == nil {
		tuiShell.ShowToast(i18n.T("tui.deploy.hub_not_init"), shell.ToastError)
		return
	}
	project, err := resolveActiveProject(a)
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.deploy.no_active_project"), shell.ToastWarning)
		return
	}

	hubDir := findHubDir()
	if hubDir == "" {
		tuiShell.ShowToast(i18n.T("tui.deploy.hub_not_found"), shell.ToastError)
		return
	}

	tuiShell.ShowToast(i18n.T("tui.deploy.analyzing"), shell.ToastInfo)
	ctx := tuiShell.Context()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		report, err := deploy.ComputeDiff(hubDir, project.Path, project.Agents, resolveWorkflowGeneratedSkills(a, project))
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.Tf("tui.deploy.diff_error", err.Error()), shell.ToastError)
				return
			}

			if !report.HasChanges() {
				tuiShell.ShowToast(i18n.T("tui.deploy.already_up_to_date"), shell.ToastSuccess)
				return
			}

			diffContent := deploy.FormatDiffReport(report, false)
			tuiShell.ShowScrollableModal(
				i18n.Tf("tui.deploy.preview_title", project.Name),
				diffContent,
				[]views.ModalAction{
					{Label: i18n.T("tui.deploy.apply"), Callback: func() {
						tuiShell.ShowToast(i18n.T("tui.deploy.in_progress"), shell.ToastInfo)
						go func() {
							select {
							case <-ctx.Done():
								return
							default:
							}
							err := runDeployForProject(a, project)
							tuiShell.App().QueueUpdateDraw(func() {
								if err != nil {
									tuiShell.ShowToast(i18n.Tf("tui.deploy.failed", err.Error()), shell.ToastError)
								} else {
									tuiShell.ShowToast(i18n.T("tui.deploy.success"), shell.ToastSuccess)
									notifyDeployComplete()
									// Informational toast for optional MCP integrations
									if mcpInfo := collectMissingMCPInfo(a, project); mcpInfo != "" {
										tuiShell.ShowToast(
											i18n.T("cmd.deploy.mcp_optional_toast"),
											shell.ToastInfo,
										)
									}
								}
							})
						}()
					}},
					{Label: i18n.T("tui.deploy.cancel"), Callback: func() {}},
				},
			)
		})
	}()
}

func actionSync() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	if a.Projects == nil {
		tuiShell.ShowToast(i18n.T("tui.deploy.hub_not_init"), shell.ToastError)
		return
	}

	tuiShell.ShowToast(i18n.T("tui.deploy.sync_in_progress"), shell.ToastInfo)
	ctx := tuiShell.Context()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := runSyncAll(a)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.Tf("tui.deploy.sync_failed", err.Error()), shell.ToastError)
			} else {
				tuiShell.ShowToast(i18n.T("tui.deploy.sync_success"), shell.ToastSuccess)
				notifyDeployComplete()
			}
		})
	}()
}

func actionUpgrade() {
	if tuiShell == nil {
		return
	}

	tuiShell.ShowToast(i18n.T("tui.deploy.upgrade_in_progress"), shell.ToastInfo)
	ctx := tuiShell.Context()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Progress callback: updates the toast with download percentage.
		progressFn := opencode.ProgressFunc(func(downloaded, total int64) {
			if total <= 0 {
				return
			}
			pct := int(float64(downloaded) / float64(total) * 100)
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(i18n.Tf("tui.deploy.upgrade_download", pct), shell.ToastInfo)
			})
		})

		err := runUpgradeOpencode(progressFn)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.Tf("tui.deploy.upgrade_failed", err.Error()), shell.ToastError)
			} else {
				tuiShell.ShowToast(i18n.T("tui.deploy.upgrade_success"), shell.ToastSuccess)
			}
		})
	}()
}

// actionViewDiff shows a read-only diff modal comparing hub vs project.
func actionViewDiff(projectPath string) {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	project, err := resolveActiveProject(a)
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.deploy.no_active_project"), shell.ToastWarning)
		return
	}

	hubDir := findHubDir()
	if hubDir == "" {
		tuiShell.ShowToast(i18n.T("tui.deploy.hub_not_found"), shell.ToastError)
		return
	}

	tuiShell.ShowToast(i18n.T("tui.deploy.analyzing"), shell.ToastInfo)
	ctx := tuiShell.Context()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		report, err := deploy.ComputeDiff(hubDir, projectPath, project.Agents, resolveWorkflowGeneratedSkills(a, project))
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.Tf("tui.deploy.diff_error", err.Error()), shell.ToastError)
				return
			}

			if !report.HasChanges() {
				tuiShell.ShowToast(i18n.T("tui.deploy.no_changes"), shell.ToastSuccess)
				return
			}

			diffContent := deploy.FormatDiffReport(report, false)
			tuiShell.ShowScrollableModal(
				i18n.Tf("tui.deploy.changes_title", project.Name),
				diffContent,
				[]views.ModalAction{
					{Label: i18n.T("tui.deploy.close"), Callback: func() {}},
				},
			)
		})
	}()
}

// canLaunchTUI returns true if the environment supports launching the TUI shell.
func canLaunchTUI() bool {
	return common.UseRichTUI()
}
