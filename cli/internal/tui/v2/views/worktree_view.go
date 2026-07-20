package views

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// WorktreeView displays and manages git worktrees for the active project.
type WorktreeView struct {
	app    *tview.Application
	appCtx *app.App
	list   *tview.List
	shell  ShellAccess
	items  []worktreeItem
}

type worktreeItem struct {
	Path   string
	Branch string
}

var _ View = (*WorktreeView)(nil)

// NewWorktreeView creates a new worktree view.
func NewWorktreeView(a *app.App) *WorktreeView {
	return &WorktreeView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *WorktreeView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *WorktreeView) ID() string { return "worktrees" }

// Title returns the display title.
func (v *WorktreeView) Title() string { return "Worktrees" }

// StatusHints returns keybinding hints.
func (v *WorktreeView) StatusHints() string {
	return "j/k nav · a ajouter · d supprimer · p prune · C cleanup · r refresh · Esc retour"
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
	case 'p':
		v.pruneWorktrees()
		return nil
	case 'C':
		v.cleanupWorktrees()
		return nil
	}
	return event
}

func (v *WorktreeView) refresh() {
	v.items = v.listWorktrees()
	v.list.Clear()

	if len(v.items) == 0 {
		v.list.AddItem("  Aucun worktree détecté", "  Vérifiez que le projet actif est un repo git", 0, nil)
		return
	}

	for _, wt := range v.items {
		icon := theme.IconDot
		if strings.Contains(wt.Branch, "main") || strings.Contains(wt.Branch, "master") {
			icon = theme.IconActive
		}
		v.list.AddItem(
			fmt.Sprintf("  %s %s", icon, wt.Branch),
			"    "+wt.Path,
			0, nil)
	}
}

func (v *WorktreeView) listWorktrees() []worktreeItem {
	// Try to get project path
	projectPath := v.getProjectPath()
	if projectPath == "" {
		return nil
	}

	out, err := exec.Command("git", "-C", projectPath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}

	var items []worktreeItem
	var current worktreeItem
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				items = append(items, current)
			}
			current = worktreeItem{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			current.Branch = branch
		}
	}
	if current.Path != "" {
		items = append(items, current)
	}
	return items
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
		wtPath := projectPath + "-" + branch
		cmd := exec.Command("git", "-C", projectPath, "worktree", "add", wtPath, "-b", branch)
		if err := cmd.Run(); err != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		} else {
			v.shell.ShowToastMsg("Worktree créé: "+branch, true)
			v.refresh()
		}
	})
}

func (v *WorktreeView) removeWorktree() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.items) {
		return
	}
	wt := v.items[idx]

	projectPath := v.getProjectPath()
	if projectPath == "" {
		return
	}

	cmd := exec.Command("git", "-C", projectPath, "worktree", "remove", wt.Path)
	if err := cmd.Run(); err != nil {
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

func (v *WorktreeView) pruneWorktrees() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet actif", false)
		}
		return
	}

	cmd := exec.Command("git", "-C", projectPath, "worktree", "prune")
	if err := cmd.Run(); err != nil {
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

	// Get merged branches
	out, err := exec.Command("git", "-C", projectPath, "branch", "--merged", "HEAD").Output()
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
		return
	}

	merged := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(line)
		branch = strings.TrimPrefix(branch, "* ")
		if branch != "" && branch != "main" && branch != "master" && branch != "develop" {
			merged[branch] = true
		}
	}

	// Remove worktrees whose branch is merged
	removed := 0
	for _, wt := range v.items {
		if merged[wt.Branch] {
			cmd := exec.Command("git", "-C", projectPath, "worktree", "remove", wt.Path)
			if cmd.Run() == nil {
				removed++
			}
		}
	}

	if v.shell != nil {
		if removed == 0 {
			v.shell.ShowToastMsg("Aucun worktree mergé à nettoyer", true)
		} else {
			v.shell.ShowToastMsg(fmt.Sprintf("%d worktree(s) nettoyé(s)", removed), true)
		}
	}
	v.refresh()
}

func (v *WorktreeView) getProjectPath() string {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return ""
	}
	project, err := resolveFirstProject(v.appCtx)
	if err != nil {
		return ""
	}
	return project.Path
}

// resolveFirstProject returns the first available project (helper for views).
func resolveFirstProject(a *app.App) (*domain.Project, error) {
	projects, err := a.Projects.List(nil, "")
	if err != nil || len(projects) == 0 {
		return nil, fmt.Errorf("no projects")
	}
	return &projects[0], nil
}
