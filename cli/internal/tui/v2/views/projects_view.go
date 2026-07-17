package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProjectItem represents a project entry in the list.
type ProjectItem struct {
	ID, Name, Path string
}

// ProjectsViewConfig configures the projects list view.
type ProjectsViewConfig struct {
	Projects []ProjectItem
}

// ProjectsView displays the list of registered projects.
type ProjectsView struct {
	cfg     ProjectsViewConfig
	list    *tview.List
	detail  *tview.TextView
	app     *tview.Application
	shell   ShellAccess
	onAdd   func(name, path string) // callback to add a project
	onRemove func(id string)        // callback to remove a project
}

var _ View = (*ProjectsView)(nil)

// NewProjectsView creates a new projects list view.
func NewProjectsView(cfg ProjectsViewConfig) *ProjectsView {
	return &ProjectsView{cfg: cfg}
}

// SetShell provides the shell reference for modal interactions.
func (v *ProjectsView) SetShell(s ShellAccess) { v.shell = s }

// SetOnAdd sets the callback for adding a project.
func (v *ProjectsView) SetOnAdd(fn func(name, path string)) { v.onAdd = fn }

// SetOnRemove sets the callback for removing a project.
func (v *ProjectsView) SetOnRemove(fn func(id string)) { v.onRemove = fn }

// ID returns the view identifier.
func (v *ProjectsView) ID() string { return "projects.list" }

// Title returns the display title.
func (v *ProjectsView) Title() string { return "Projets" }

// StatusHints returns keybinding hints.
func (v *ProjectsView) StatusHints() string {
	return "j/k naviguer · a ajouter · d supprimer · Esc retour"
}

// Mount builds the projects list with footer detail.
func (v *ProjectsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.detail = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.detail.SetBackgroundColor(theme.BgPanel)
	v.detail.SetBorderPadding(0, 0, 2, 2)

	for _, p := range v.cfg.Projects {
		v.list.AddItem(p.Name, "    "+p.Path, 0, nil)
	}

	projects := v.cfg.Projects
	v.list.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index >= 0 && index < len(projects) {
			v.showDetail(projects[index])
		}
	})

	if len(projects) > 0 {
		v.showDetail(projects[0])
	}

	// Vertical layout: list on top, footer detail at bottom
	content.AddItem(v.list, 0, 3, true)
	content.AddItem(v.detail, 4, 0, false)
}

// Unmount cleans up resources.
func (v *ProjectsView) Unmount() {
	v.app = nil
	v.list = nil
	v.detail = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'a':
		v.addProject()
		return nil
	case 'd':
		v.removeProject()
		return nil
	}
	return event
}

func (v *ProjectsView) addProject() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal("Nom du projet", "", func(name string) {
		if name == "" {
			return
		}
		v.shell.ShowInputModal("Chemin du projet", "", func(path string) {
			if path == "" {
				return
			}
			if v.onAdd != nil {
				v.onAdd(name, path)
			}
			// Add to local list for immediate feedback
			v.cfg.Projects = append(v.cfg.Projects, ProjectItem{Name: name, Path: path})
			v.list.AddItem(name, "  "+path, 0, nil)
			v.shell.ShowToastMsg("Projet ajouté: "+name, true)
		})
	})
}

func (v *ProjectsView) removeProject() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := v.cfg.Projects[idx]
	if v.onRemove != nil {
		v.onRemove(project.ID)
	}
	// Remove from local list
	v.cfg.Projects = append(v.cfg.Projects[:idx], v.cfg.Projects[idx+1:]...)
	v.list.RemoveItem(idx)
	if v.shell != nil {
		v.shell.ShowToastMsg("Projet supprimé: "+project.Name, true)
	}
}

func (v *ProjectsView) showDetail(p ProjectItem) {
	if v.detail == nil {
		return
	}
	sep := theme.ColorTag(theme.TextMutedHex) + "─────────────────────────────────────────────" + theme.TagColor
	v.detail.SetText(fmt.Sprintf("%s\n  %sID:%s %s  %s·%s  %sChemin:%s %s",
		sep,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, p.ID,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, p.Path,
	))
}
