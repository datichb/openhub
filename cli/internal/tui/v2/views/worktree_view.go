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
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// WorktreeViewConfig holds the optional callbacks for the worktree view.
type WorktreeViewConfig struct {
	// OpenSession opens the launch form of a free session (workflow
	// `libre`) located in the worktree at path. Nil disables the action.
	OpenSession func(path string)
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
func (v *WorktreeView) Title() string { return i18n.T("tui.worktree.title") }

// StatusHints returns keybinding hints.
func (v *WorktreeView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Enter %s · a %s · d %s · o %s · p %s · x %s · r %s · Esc %s", i18n.T("tui.hints.nav"), i18n.T("tui.hints.open"), i18n.T("tui.hints.add"), i18n.T("tui.hints.delete"), i18n.T("tui.hints.open"), i18n.T("tui.hints.prune"), i18n.T("tui.hints.cleanup"), i18n.T("tui.hints.refresh"), i18n.T("tui.hints.back"))
}

// Mount builds the worktree list.
func (v *WorktreeView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorder(false)
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
	if event.Key() == tcell.KeyEnter {
		v.openInTerminal()
		return nil
	}
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
	case 'x':
		v.cleanupWorktrees()
		return nil
	}
	return event
}

func (v *WorktreeView) refresh() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		v.items = nil
		v.list.Clear()
		v.list.AddItem("  "+i18n.T("tui.worktree.no_project"), "  "+i18n.T("tui.worktree.no_project_hint"), 0, nil)
		return
	}

	entries, err := worktree.List(projectPath)
	if err != nil {
		v.items = nil
		v.list.Clear()
		v.list.AddItem("  "+i18n.T("tui.worktree.no_worktree"), "  "+i18n.T("tui.worktree.no_worktree_hint"), 0, nil)
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
		v.list.AddItem("  "+i18n.T("tui.worktree.no_secondary"), "  "+i18n.T("tui.worktree.no_secondary_hint"), 0, nil)
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
	v.shell.ShowInputModal(i18n.T("tui.worktree.branch_name"), "", func(branch string) {
		if branch == "" {
			return
		}
		projectPath := v.getProjectPath()
		if projectPath == "" {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.no_active_project"), false)
			return
		}

		v.shell.ShowToastMsg(i18n.T("tui.worktree.creating"), true)
		tvApp := v.app // capture stable reference before goroutine
		go func() {
			_, err := worktree.ResolveOrCreate(projectPath, branch)
			if tvApp == nil {
				return
			}
			tvApp.QueueUpdateDraw(func() {
				if v.shell == nil {
					return
				}
				if err != nil {
					v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
				} else {
					v.shell.ShowToastMsg(i18n.Tf("tui.worktree.created", branch), true)
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
			v.shell.ShowToastMsg(i18n.T("tui.worktree.select_secondary"), false)
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
			v.shell.ShowToastMsg(i18n.T("tui.worktree.cannot_remove_main"), false)
		}
		return
	}

	if err := worktree.Remove(projectPath, wt.Path, false); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.removed"), true)
		}
		v.refresh()
	}
}

func (v *WorktreeView) pruneWorktrees() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.no_active_project"), false)
		}
		return
	}

	if err := worktree.Prune(projectPath); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.prune_failed")+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.pruned"), true)
		}
		v.refresh()
	}
}

func (v *WorktreeView) cleanupWorktrees() {
	projectPath := v.getProjectPath()
	if projectPath == "" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.no_active_project"), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.cleaning_merged"), true)
	}

	baseBranch := worktree.DetectBaseBranch(projectPath)
	tvApp := v.app // capture stable reference before goroutine
	go func() {
		result, err := worktree.CleanupMerged(projectPath, baseBranch, false)
		if tvApp == nil {
			return
		}
		tvApp.QueueUpdateDraw(func() {
			if v.shell == nil {
				return
			}
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
				return
			}
			switch {
			case len(result.Removed) == 0 && len(result.Skipped) == 0:
				v.shell.ShowToastMsg(i18n.T("tui.worktree.no_merged"), true)
			case len(result.Skipped) > 0:
				v.shell.ShowToastMsg(
					i18n.Tf("tui.worktree.cleanup_partial", len(result.Removed), len(result.Skipped)),
					len(result.Removed) > 0,
				)
			default:
				v.shell.ShowToastMsg(i18n.Tf("tui.worktree.cleanup_done", len(result.Removed)), true)
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

// openInTerminal opens a session in the selected worktree: the launch form
// of the free session (workflow `libre`), located in the worktree.
func (v *WorktreeView) openInTerminal() {
	wt, ok := v.selectedItem()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.worktree.select_secondary"), false)
		}
		return
	}
	if v.cfg.OpenSession != nil {
		v.cfg.OpenSession(wt.Path)
	}
}

// resolveFirstProject returns the first available project (helper for views).
func resolveFirstProject(a *app.App) (*domain.Project, error) {
	projects, err := a.Projects.List(context.Background(), "")
	if err != nil || len(projects) == 0 {
		return nil, fmt.Errorf("no projects")
	}
	return &projects[0], nil
}
