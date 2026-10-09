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

// WorktreeGit is the git side of the worktree view; every call runs off the
// event loop (N19).
type WorktreeGit interface {
	List(projectPath string) ([]worktree.Entry, error)
	Create(projectPath, branch string) error
	Remove(projectPath, path string) error
	Prune(projectPath string) error
	// CleanupMerged removes the worktrees whose branch is merged into the
	// base branch, never those for which keep returns true nor those with
	// uncommitted changes.
	CleanupMerged(projectPath string, keep func(path string) bool) (WorktreeCleanup, error)
}

// WorktreeCleanup is the outcome of WorktreeGit.CleanupMerged (branches).
type WorktreeCleanup struct {
	Removed []string
	Dirty   []string // merged, kept: uncommitted changes
	InUse   []string // merged, kept: used by a session
}

// WorktreeViewConfig holds the optional callbacks for the worktree view.
type WorktreeViewConfig struct {
	// OpenSession opens the launch form of a free session (workflow
	// `libre`) located in the worktree at path. Nil disables the action.
	OpenSession func(path string)
	// Git runs the git commands (nil = the git command line).
	Git WorktreeGit
	// InUse reports the worktree paths used by a session that is not
	// stopped (nil = read from the sessions of the active project).
	InUse func(ctx context.Context, projectID string) map[string]bool
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
	gen         int
}

var _ View = (*WorktreeView)(nil)

// NewWorktreeView creates a new worktree view.
func NewWorktreeView(a *app.App, cfg WorktreeViewConfig) *WorktreeView {
	if cfg.Git == nil {
		cfg.Git = gitWorktrees{}
	}
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
	v.gen++
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

// refresh lists the worktrees off the event loop.
func (v *WorktreeView) refresh() {
	if v.list == nil {
		return
	}
	projectPath := v.getProjectPath()
	if projectPath == "" {
		v.items = nil
		v.list.Clear()
		v.list.AddItem("  "+i18n.T("tui.worktree.no_project"), "  "+i18n.T("tui.worktree.no_project_hint"), 0, nil)
		return
	}
	v.gen++
	gen, tvApp, git := v.gen, v.app, v.cfg.Git
	go func() {
		entries, err := git.List(projectPath)
		if tvApp == nil {
			return
		}
		tvApp.QueueUpdateDraw(func() {
			if v.gen != gen || v.list == nil {
				return
			}
			v.render(projectPath, entries, err)
		})
	}()
}

func (v *WorktreeView) render(projectPath string, entries []worktree.Entry, err error) {
	v.list.Clear()
	v.headerCount = 0
	if err != nil {
		v.items = nil
		v.list.AddItem("  "+i18n.T("tui.worktree.no_worktree"), "  "+i18n.T("tui.worktree.no_worktree_hint"), 0, nil)
		return
	}

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

// runGit runs a git operation off the event loop, then shows its outcome
// and refreshes the list on the loop.
func (v *WorktreeView) runGit(op func() error, done func(err error)) {
	tvApp := v.app // capture stable reference before goroutine
	if tvApp == nil {
		return
	}
	go func() {
		err := op()
		tvApp.QueueUpdateDraw(func() {
			if v.shell == nil {
				return
			}
			done(err)
			v.refresh()
		})
	}()
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
		v.runGit(func() error { return v.cfg.Git.Create(projectPath, branch) }, func(err error) {
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
				return
			}
			v.shell.ShowToastMsg(i18n.Tf("tui.worktree.created", branch), true)
		})
	})
}

// selectedItem returns the worktree.Entry currently highlighted in the list,
// accounting for the non-actionable header items prepended before v.items.
// Returns (entry, true) on success, or (zero, false) if the selection is on a
// header row or out of range.
func (v *WorktreeView) selectedItem() (worktree.Entry, bool) {
	if v.list == nil {
		return worktree.Entry{}, false
	}
	listIdx := v.list.GetCurrentItem()
	dataIdx := listIdx - v.headerCount
	if dataIdx < 0 || dataIdx >= len(v.items) {
		return worktree.Entry{}, false
	}
	return v.items[dataIdx], true
}

// removeWorktree is `d`: removes the selected worktree after confirmation
// (the branch is kept; a worktree with uncommitted changes is refused by git).
func (v *WorktreeView) removeWorktree() {
	if v.shell == nil {
		return
	}
	wt, ok := v.selectedItem()
	if !ok {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.select_secondary"), false)
		return
	}
	projectPath := v.getProjectPath()
	if projectPath == "" {
		return
	}
	// Safety guard: never allow removing the main worktree.
	if filepath.Clean(wt.Path) == filepath.Clean(projectPath) {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.cannot_remove_main"), false)
		return
	}
	v.shell.Confirm(i18n.T("tui.confirm.worktree.remove.title"),
		i18n.Tf("tui.confirm.worktree.remove.detail", wt.Branch, wt.Path), true, func() {
			v.runGit(func() error { return v.cfg.Git.Remove(projectPath, wt.Path) }, func(err error) {
				if err != nil {
					v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
					return
				}
				v.shell.ShowToastMsg(i18n.T("tui.worktree.removed"), true)
			})
		})
}

// pruneWorktrees is `p`: git worktree prune, after confirmation.
func (v *WorktreeView) pruneWorktrees() {
	if v.shell == nil {
		return
	}
	projectPath := v.getProjectPath()
	if projectPath == "" {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.no_active_project"), false)
		return
	}
	v.shell.Confirm(i18n.T("tui.confirm.worktree.prune.title"), i18n.T("tui.confirm.worktree.prune.detail"), false, func() {
		v.runGit(func() error { return v.cfg.Git.Prune(projectPath) }, func(err error) {
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.worktree.prune_failed")+err.Error(), false)
				return
			}
			v.shell.ShowToastMsg(i18n.T("tui.worktree.pruned"), true)
		})
	})
}

// cleanupWorktrees is `x`: removes the merged worktrees after confirmation,
// never one used by a session that is not stopped.
func (v *WorktreeView) cleanupWorktrees() {
	if v.shell == nil {
		return
	}
	projectPath := v.getProjectPath()
	if projectPath == "" {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.no_active_project"), false)
		return
	}
	projectID := ""
	if ap := v.shell.ActiveProject(); ap != nil {
		projectID = ap.ID
	}
	ctx := v.shell.Context()
	v.shell.Confirm(i18n.T("tui.confirm.worktree.cleanup.title"), i18n.T("tui.confirm.worktree.cleanup.detail"), true, func() {
		v.shell.ShowToastMsg(i18n.T("tui.worktree.cleaning_merged"), true)
		var result WorktreeCleanup
		v.runGit(func() error {
			used := v.inUse(ctx, projectID)
			var err error
			result, err = v.cfg.Git.CleanupMerged(projectPath, func(path string) bool { return used[filepath.Clean(path)] })
			return err
		}, func(err error) {
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.worktree.error")+err.Error(), false)
				return
			}
			kept := len(result.Dirty) + len(result.InUse)
			switch {
			case len(result.Removed) == 0 && kept == 0:
				v.shell.ShowToastMsg(i18n.T("tui.worktree.no_merged"), true)
			case len(result.InUse) > 0:
				v.shell.ShowToastMsg(i18n.Tf("tui.confirm.worktree.cleanup.in_use", len(result.Removed), len(result.InUse), len(result.Dirty)), len(result.Removed) > 0)
			case len(result.Dirty) > 0:
				v.shell.ShowToastMsg(i18n.Tf("tui.worktree.cleanup_partial", len(result.Removed), len(result.Dirty)), len(result.Removed) > 0)
			default:
				v.shell.ShowToastMsg(i18n.Tf("tui.worktree.cleanup_done", len(result.Removed)), true)
			}
		})
	})
}

// inUse returns the cleaned paths of the worktrees used by a session that
// is not stopped (off the event loop).
func (v *WorktreeView) inUse(ctx context.Context, projectID string) map[string]bool {
	if v.cfg.InUse != nil {
		used := map[string]bool{}
		for p := range v.cfg.InUse(ctx, projectID) {
			used[filepath.Clean(p)] = true
		}
		return used
	}
	used := map[string]bool{}
	if v.appCtx == nil || v.appCtx.Sessions == nil {
		return used
	}
	sessions, err := v.appCtx.Sessions.List(ctx, projectID)
	if err != nil {
		return used
	}
	for _, s := range sessions {
		if s.LaunchPath == "" || s.State == domain.RunStopped || (s.State == "" && s.EndedAt != nil) {
			continue
		}
		used[filepath.Clean(s.LaunchPath)] = true
	}
	return used
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

// gitWorktrees is WorktreeGit on the git command line.
type gitWorktrees struct{}

func (gitWorktrees) List(projectPath string) ([]worktree.Entry, error) {
	return worktree.List(projectPath)
}

func (gitWorktrees) Create(projectPath, branch string) error {
	_, err := worktree.ResolveOrCreate(projectPath, branch)
	return err
}

func (gitWorktrees) Remove(projectPath, path string) error {
	return worktree.Remove(projectPath, path, false)
}

func (gitWorktrees) Prune(projectPath string) error { return worktree.Prune(projectPath) }

func (gitWorktrees) CleanupMerged(projectPath string, keep func(path string) bool) (WorktreeCleanup, error) {
	var result WorktreeCleanup
	entries, err := worktree.List(projectPath)
	if err != nil {
		return result, err
	}
	base := worktree.DetectBaseBranch(projectPath)
	mainPath, _ := filepath.Abs(projectPath)
	for _, e := range entries {
		if e.IsBare || e.Branch == "" || e.Branch == base {
			continue
		}
		if abs, _ := filepath.Abs(e.Path); abs == mainPath {
			continue
		}
		merged, err := worktree.IsMerged(projectPath, e.Branch, base)
		if err != nil || !merged {
			continue
		}
		if keep != nil && keep(e.Path) {
			result.InUse = append(result.InUse, e.Branch)
			continue
		}
		if err := worktree.Remove(projectPath, e.Path, false); err != nil {
			result.Dirty = append(result.Dirty, e.Branch)
			continue
		}
		result.Removed = append(result.Removed, e.Branch)
	}
	_ = worktree.Prune(projectPath)
	return result, nil
}
