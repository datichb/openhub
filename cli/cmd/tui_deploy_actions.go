package cmd

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Deploy / Sync / Upgrade actions
// ─────────────────────────────────────────────────────────────────────────────

func actionDeploy() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	if a.Projects == nil {
		tuiShell.ShowToast("Hub non initialisé", shell.ToastError)
		return
	}
	project, err := resolveActiveProject(a)
	if err != nil {
		tuiShell.ShowToast("Aucun projet actif", shell.ToastWarning)
		return
	}

	hubDir := findHubDir()
	if hubDir == "" {
		tuiShell.ShowToast("Hub content non trouvé", shell.ToastError)
		return
	}

	tuiShell.ShowToast("Analyse des changements...", shell.ToastInfo)
	ctx := tuiShell.Context()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		report, err := deploy.ComputeDiff(hubDir, project.Path, project.Agents)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Erreur diff: "+err.Error(), shell.ToastError)
				return
			}

			if !report.HasChanges() {
				tuiShell.ShowToast("Déjà à jour — rien à déployer", shell.ToastSuccess)
				return
			}

			diffContent := deploy.FormatDiffReport(report, false)
			tuiShell.ShowScrollableModal(
				"Deploy Preview: "+project.Name,
				diffContent,
				[]views.ModalAction{
					{Label: "Appliquer", Callback: func() {
						tuiShell.ShowToast("Deploy en cours...", shell.ToastInfo)
						go func() {
							select {
							case <-ctx.Done():
								return
							default:
							}
							err := runDeployForProject(a, project)
							tuiShell.App().QueueUpdateDraw(func() {
								if err != nil {
									tuiShell.ShowToast("Deploy échoué: "+err.Error(), shell.ToastError)
								} else {
									tuiShell.ShowToast("Deploy réussi", shell.ToastSuccess)
								}
							})
						}()
					}},
					{Label: "Annuler", Callback: func() {}},
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
		tuiShell.ShowToast("Hub non initialisé", shell.ToastError)
		return
	}

	tuiShell.ShowToast("Sync en cours...", shell.ToastInfo)
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
				tuiShell.ShowToast("Sync échoué: "+err.Error(), shell.ToastError)
			} else {
				tuiShell.ShowToast("Sync réussi", shell.ToastSuccess)
			}
		})
	}()
}

func actionUpgrade() {
	if tuiShell == nil {
		return
	}

	tuiShell.ShowToast("Mise à jour opencode...", shell.ToastInfo)
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
				tuiShell.ShowToast(fmt.Sprintf("Téléchargement opencode... %d%%", pct), shell.ToastInfo)
			})
		})

		err := runUpgradeOpencode(progressFn)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Mise à jour échouée: "+err.Error(), shell.ToastError)
			} else {
				tuiShell.ShowToast("opencode mis à jour", shell.ToastSuccess)
			}
		})
	}()
}

// canLaunchTUI returns true if the environment supports launching the TUI shell.
func canLaunchTUI() bool {
	return common.UseRichTUI()
}
