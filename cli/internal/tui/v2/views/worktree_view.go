package views

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/termlaunch"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// WorktreeViewConfig holds the optional callbacks for the worktree view.
type WorktreeViewConfig struct {
	// DeployProject triggers a full hub deploy into projectPath.
	// Called automatically when EnsureWorktreeConfig detects the project has
	// not been deployed yet. If nil, auto-deploy is skipped and the user sees
	// an error toast instead.
	DeployProject func(projectPath string) error
}

// WorktreeView displays and manages git worktrees for the active project.
type WorktreeView struct {
	app         *tview.Application
	appCtx      *app.App
	cfg         WorktreeViewConfig
	list        *tview.List
	shell       ShellAccess
	items       []worktree.Entry
	headerCount int // number of non-actionable items prepended to v.list before v.items
}

var _ View = (*WorktreeView)(nil)

// NewWorktreeView creates a new worktree view.
func NewWorktreeView(a *app.App, cfg WorktreeViewConfig) *WorktreeView {
	return &WorktreeView{appCtx: a, cfg: cfg}
}

// SetShell provides the shell reference for modal interactions.
func (v *WorktreeView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *WorktreeView) ID() string { return "worktrees" }

// Title returns the display title.
func (v *WorktreeView) Title() string { return "Worktrees" }

// StatusHints returns keybinding hints.
func (v *WorktreeView) StatusHints() string {
	return fmt.Sprintf("j/k %s · a %s · d %s · o %s · s %s · p %s · C %s · r %s · Esc %s", i18n.T("tui.hints.nav"), i18n.T("tui.hints.add"), i18n.T("tui.hints.delete"), i18n.T("tui.hints.open"), i18n.T("tui.hints.sync"), i18n.T("tui.hints.prune"), i18n.T("tui.hints.cleanup"), i18n.T("tui.hints.refresh"), i18n.T("tui.hints.back"))
}

// Mount builds the worktree list.
func (v *WorktreeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorder(true)
	v.list.SetBorderColor(theme.BorderFocus)
	v.list.SetTitle(" Git Worktrees ")
	v.list.SetTitleColor(theme.FgPrimary)
	v.list.SetBorderPadding(1, 0, 1, 1)

	v.refresh()
	content.AddItem(v.list, 0, 1, true)
}

// Unmount cleans up resources.
func (v *WorktreeView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes worktree view key events.
func (v *WorktreeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'a':
		v.addWorktree()
		return nil
	case 'd':
		v.removeWorktree()
		return nil
	case 'o':
		v.openInTerminal()
		return nil
	case 'p':
		v.pruneWorktrees()
		return nil
	case 'C':
		v.cleanupWorktrees()
		return nil
	case 's':
		v.syncWorktree()
		return nil
	}
	return event
}

func (v *WorktreeView) refresh() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		v.items = nil
		v.list.Clear()
		v.list.AddItem("  Aucun projet actif", "  Sélectionnez un projet", 0, nil)
		return
	}

	entries, err := worktree.List(projectPath)
	if err != nil {
		v.items = nil
		v.list.Clear()
		v.list.AddItem("  Aucun worktree détecté", "  Vérifiez que le projet actif est un repo git", 0, nil)
		return
	}

	v.list.Clear()
	v.headerCount = 0

	// Separate main worktree from secondary ones.
	// The main worktree (path == projectPath) is displayed as a read-only header;
	// only secondary worktrees are stored in v.items and are actionable.
	var secondary []worktree.Entry
	for _, e := range entries {
		if filepath.Clean(e.Path) == filepath.Clean(projectPath) {
			// Display as a non-actionable informational header.
			label := e.Branch
			if label == "" {
				label = "(detached)"
			}
			v.list.AddItem(
				fmt.Sprintf("  %s %s  [principal]", theme.IconActive, label),
				"    "+e.Path,
				0, nil)
			v.headerCount++
		} else {
			secondary = append(secondary, e)
		}
	}

	v.items = secondary
	if len(secondary) == 0 {
		v.list.AddItem("  Aucun worktree secondaire", "  Utilisez 'a' pour en créer un", 0, nil)
		return
	}

	for _, wt := range secondary {
		v.list.AddItem(
			fmt.Sprintf("  %s %s", theme.IconDot, wt.Branch),
			"    "+wt.Path,
			0, nil)
	}
}

func (v *WorktreeView) addWorktree() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal("Nom de la branche", "", func(branch string) {
		if branch == "" {
			return
		}
		projectPath := v.getProjectPath()
		if projectPath == "" {
			v.shell.ShowToastMsg("Aucun projet actif", false)
			return
		}

		v.shell.ShowToastMsg("Création du worktree en cours...", true)
		app := v.app // capture stable reference before goroutine
		go func() {
			_, err := worktree.ResolveOrCreate(projectPath, branch)
			if app == nil {
				return
			}
			app.QueueUpdateDraw(func() {
				if v.shell == nil {
					return
				}
				if err != nil {
					v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
				} else {
					v.shell.ShowToastMsg("Worktree créé: "+branch, true)
					v.refresh()
				}
			})
		}()
	})
}

// selectedItem returns the worktree.Entry currently highlighted in the list,
// accounting for the non-actionable header items prepended before v.items.
// Returns (entry, true) on success, or (zero, false) if the selection is on a
// header row or out of range.
func (v *WorktreeView) selectedItem() (worktree.Entry, bool) {
	listIdx := v.list.GetCurrentItem()
	dataIdx := listIdx - v.headerCount
	if dataIdx < 0 || dataIdx >= len(v.items) {
		return worktree.Entry{}, false
	}
	return v.items[dataIdx], true
}

func (v *WorktreeView) removeWorktree() {
	wt, ok := v.selectedItem()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg("Sélectionnez un worktree secondaire", false)
		}
		return
	}

	projectPath := v.getProjectPath()
	if projectPath == "" {
		return
	}

	// Safety guard: never allow removing the main worktree.
	if filepath.Clean(wt.Path) == filepath.Clean(projectPath) {
		if v.shell != nil {
			v.shell.ShowToastMsg("Impossible de supprimer le worktree principal", false)
		}
		return
	}

	if err := worktree.Remove(projectPath, wt.Path, false); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg("Worktree supprimé", true)
		}
		v.refresh()
	}
}

// syncWorktree forces a resync of the .opencode/ configuration for the
// selected worktree. It migrates legacy full-directory symlinks to the current
// layout (real .opencode/ dir + internal symlinks) and refreshes copied files
// from the main project.
func (v *WorktreeView) syncWorktree() {
	wt, ok := v.selectedItem()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg("Sélectionnez un worktree secondaire", false)
		}
		return
	}

	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet actif", false)
		}
		return
	}

	if err := worktree.ResyncConfig(wt.Path, projectPath); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur sync: "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg("Worktree synchronisé", true)
	}
}

func (v *WorktreeView) pruneWorktrees() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet actif", false)
		}
		return
	}

	if err := worktree.Prune(projectPath); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Prune échoué: "+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg("Worktrees prunés", true)
		}
		v.refresh()
	}
}

func (v *WorktreeView) cleanupWorktrees() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet actif", false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg("Nettoyage des worktrees mergés...", true)
	}

	baseBranch := worktree.DetectBaseBranch(projectPath)
	app := v.app // capture stable reference before goroutine
	go func() {
		result, err := worktree.CleanupMerged(projectPath, baseBranch, false)
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			if v.shell == nil {
				return
			}
			if err != nil {
				v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
				return
			}
			switch {
			case len(result.Removed) == 0 && len(result.Skipped) == 0:
				v.shell.ShowToastMsg("Aucun worktree mergé à nettoyer", true)
			case len(result.Skipped) > 0:
				v.shell.ShowToastMsg(
					fmt.Sprintf("%d nettoyé(s), %d ignoré(s) (modifications non commitées)", len(result.Removed), len(result.Skipped)),
					len(result.Removed) > 0,
				)
			default:
				v.shell.ShowToastMsg(fmt.Sprintf("%d worktree(s) nettoyé(s)", len(result.Removed)), true)
			}
			v.refresh()
		})
	}()
}

func (v *WorktreeView) getProjectPath() string {
	// Prefer the shell's active project (set when entering project mode)
	if v.shell != nil {
		if ap := v.shell.ActiveProject(); ap != nil {
			return ap.Path
		}
	}
	// Fallback: first available project
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return ""
	}
	project, err := resolveFirstProject(v.appCtx)
	if err != nil {
		return ""
	}
	return project.Path
}

// openInTerminal opens the selected worktree in a new terminal window running opencode.
// It ensures hub config symlinks exist in the worktree first, triggering an
// automatic deploy into the main project if needed.
func (v *WorktreeView) openInTerminal() {
	wt, ok := v.selectedItem()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg("Sélectionnez un worktree secondaire", false)
		}
		return
	}

	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet actif", false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg("Préparation du worktree...", true)
	}

	go func() {
		// Step 1: ensure worktree has config symlinks.
		err := worktree.EnsureWorktreeConfig(wt.Path, projectPath)

		if err == worktree.ErrProjectNotDeployed {
			// Main project not deployed yet — trigger auto-deploy.
			if v.cfg.DeployProject == nil {
				v.app.QueueUpdateDraw(func() {
					if v.shell != nil {
						v.shell.ShowToastMsg("Projet non déployé — lancez 'oh deploy' d'abord", false)
					}
				})
				return
			}
			// Deploy into the main project, then retry.
			if deployErr := v.cfg.DeployProject(projectPath); deployErr != nil {
				v.app.QueueUpdateDraw(func() {
					if v.shell != nil {
						v.shell.ShowToastMsg("Déploiement échoué: "+deployErr.Error(), false)
					}
				})
				return
			}
			// Retry symlinks now that deploy is done.
			err = worktree.EnsureWorktreeConfig(wt.Path, projectPath)
		}

		if err != nil {
			v.app.QueueUpdateDraw(func() {
				if v.shell != nil {
					v.shell.ShowToastMsg("Erreur config worktree: "+err.Error(), false)
				}
			})
			return
		}

		// Step 2: resolve opencode binary path.
		ocBin, binErr := resolveOpencodeBinary()
		if binErr != nil {
			v.app.QueueUpdateDraw(func() {
				if v.shell != nil {
					v.shell.ShowToastMsg("opencode introuvable: "+binErr.Error(), false)
				}
			})
			return
		}

		// Step 3: open new terminal.
		termErr := termlaunch.OpenInNewTerminal(wt.Path, ocBin)
		v.app.QueueUpdateDraw(func() {
			if termErr != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg("Impossible d'ouvrir le terminal: "+termErr.Error(), false)
				}
				return
			}
			if v.shell != nil {
				v.shell.ShowToastMsg(
					fmt.Sprintf("opencode ouvert dans %s — %s", termlaunch.Detect(), wt.Branch),
					true,
				)
			}
		})
	}()
}

// resolveFirstProject returns the first available project (helper for views).
func resolveFirstProject(a *app.App) (*domain.Project, error) {
	projects, err := a.Projects.List(context.Background(), "")
	if err != nil || len(projects) == 0 {
		return nil, fmt.Errorf("no projects")
	}
	return &projects[0], nil
}

// resolveOpencodeBinary returns the absolute path to the opencode binary.
func resolveOpencodeBinary() (string, error) {
	return opencode.FindBinary()
}
