package cmd

import (
	"github.com/datichb/openhub/cli/internal/deploy"
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
				tuiShell.ShowToast("Erreur diff: "+truncateErr(err), shell.ToastError)
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
									tuiShell.ShowToast("Deploy échoué: "+truncateErr(err), shell.ToastError)
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
				tuiShell.ShowToast("Sync échoué: "+truncateErr(err), shell.ToastError)
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
		err := runUpgradeOpencode()
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Mise à jour échouée: "+truncateErr(err), shell.ToastError)
			} else {
				tuiShell.ShowToast("opencode mis à jour", shell.ToastSuccess)
			}
		})
	}()
}

// truncateErr returns a short error message suitable for a toast.
func truncateErr(err error) string {
	msg := err.Error()
	if len(msg) > 40 {
		return msg[:37] + "..."
	}
	return msg
}

// canLaunchTUI returns true if the environment supports launching the TUI shell.
func canLaunchTUI() bool {
	return common.UseRichTUI()
}
