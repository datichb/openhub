package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProjectModeConfig holds the callbacks used by the project mode view.
type ProjectModeConfig struct {
	// OnLaunchSession is called when the user triggers a session action.
	// agent is the opencode agent name, extraArgs are additional CLI flags.
	OnLaunchSession func(project *ActiveProject, agent string, extraArgs ...string)
	// OnNavigate is called when the user wants to navigate to a sub-view.
	// viewID identifies the target view.
	OnNavigate func(viewID string)
	// OnExitProjectMode is called when the user toggles back to hub mode.
	OnExitProjectMode func()

	// ── Deploy callbacks ────────────────────────────────────────────────
	// OnDeploy triggers a deploy on the active project (shows diff preview + apply modal).
	OnDeploy func(projectPath string)
	// OnViewDiff shows a read-only diff modal comparing hub vs project.
	OnViewDiff func(projectPath string)
	// CheckDeployStatus returns a lightweight deploy status read from .deploy-state.
	// Cheap (~1ms) — safe for synchronous use in Mount().
	CheckDeployStatus func(projectPath string) *DeployStatusResult
	// ComputeDeployDiff computes the full diff between hub and project.
	// Expensive (~60-70 file reads) — must only be called from a goroutine.
	ComputeDeployDiff func(projectPath string) (*DeployDiffResult, error)
}

// projectModeItem represents a navigable item in the project mode view.
type projectModeItem struct {
	Icon   string
	Label  string
	Desc   string
	Action func()
}

// ProjectModeView is the simplified project-scoped TUI view.
// It occupies the full content area and shows the active project's context.
// All actions are accessible via the omnibar (filtered to project scope).
type ProjectModeView struct {
	cfg         ProjectModeConfig
	project     *ActiveProject
	shell       ShellAccess
	app         *tview.Application
	list        *widgets.SectionedList
	dual        *homeDualLayout
	items       []projectModeItem
	resolveTeam ResolveTeamFunc // resolves effective team config for the active project

	// ── Deploy status state ─────────────────────────────────────────────
	mountGen         uint64              // guards stale goroutines (standard pattern)
	deployStatus     *DeployStatusResult // lightweight sync result (ReadDeployState)
	deployDiff       *DeployDiffResult   // full async result (ComputeDiff)
	deployToastShown bool                // prevents duplicate toasts per mount cycle
	headerTV         *tview.TextView     // reference for async header updates
}

var _ View = (*ProjectModeView)(nil)
var _ CommandProvider = (*ProjectModeView)(nil)

// NewProjectModeView creates a new project mode view.
func NewProjectModeView(cfg ProjectModeConfig) *ProjectModeView {
	return &ProjectModeView{cfg: cfg}
}

// SetShell provides the shell reference.
func (v *ProjectModeView) SetShell(s ShellAccess) { v.shell = s }

// SetResolveTeam injects the team resolution function used to conditionally
// show team-related context commands when the active project has team enabled.
func (v *ProjectModeView) SetResolveTeam(fn ResolveTeamFunc) { v.resolveTeam = fn }

// ID returns the view identifier.
func (v *ProjectModeView) ID() string { return "project.mode" }

// Title returns the display title.
func (v *ProjectModeView) Title() string {
	if v.project != nil {
		return fmt.Sprintf("Projet · %s", v.project.Name)
	}
	return "Mode Projet"
}

// StatusHints returns keybinding hints for the omnibar (passive mode).
func (v *ProjectModeView) StatusHints() string {
	if v.project == nil {
		return fmt.Sprintf("Aucun projet actif · Ctrl+T %s", i18n.T("tui.hints.hub_mode"))
	}
	return fmt.Sprintf(
		"%s · j/k %s · Enter %s · r refresh · Ctrl+P %s · Ctrl+T %s",
		v.project.Name,
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.open"),
		i18n.T("tui.hints.commands"),
		i18n.T("tui.hints.hub_mode"),
	)
}

// Mount builds the project mode view.
func (v *ProjectModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.deployToastShown = false
	v.deployDiff = nil

	// Refresh project from shell each time the view is mounted
	if v.shell != nil {
		v.project = v.shell.ActiveProject()
	}

	if v.project == nil {
		// No project active — show empty state
		tv := tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(false)
		tv.SetBackgroundColor(theme.BgPanel)
		tv.SetBorderPadding(1, 0, 2, 2)
		tv.SetText(fmt.Sprintf(
			"\n  [red]Aucun projet actif[-]\n\n  Utilisez %sCtrl+T%s pour revenir au mode hub.",
			theme.ColorTag(theme.AccentHex), theme.TagColor,
		))
		content.AddItem(tv, 0, 1, true)
		return
	}

	// ── Synchronous deploy status (~1ms) ────────────────────────────────
	if v.cfg.CheckDeployStatus != nil {
		v.deployStatus = v.cfg.CheckDeployStatus(v.project.Path)
	}

	v.items = v.buildItems()

	// ── Header ──────────────────────────────────────────────────────────
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	header.SetBackgroundColor(theme.BgPanel)
	v.headerTV = header

	v.renderHeader()
	headerHeight := bannerHeight(v.project.Name, 100) + 6

	// ── Footer ──────────────────────────────────────────────────────────
	muted := theme.ColorTag(theme.TextMutedHex)
	accent := theme.ColorTag(theme.AccentHex)
	reset := theme.TagColor
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(fmt.Sprintf("\n%s%sCtrl+T%s mode hub  %sCtrl+P%s commandes  %sr%s refresh  %s?%s aide",
		muted, accent, reset, accent, reset, accent, reset, accent, reset,
	))

	// ── Split items for dual-column: left = Sessions+Board, right = Config+Deploy+Team+Hub ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	buildFn := func(width int) homeFlexResult {
		return buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       header,
			HeaderHeight: headerHeight,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
		})
	}

	initial := adaptiveHomeMount(app, content, buildFn)

	if initial.Dual != nil {
		v.dual = initial.Dual
		v.list = initial.Dual.left
	} else {
		v.dual = nil
		v.list = initial.SingleList
	}

	// ── Async deploy diff (background goroutine) ────────────────────────
	if v.cfg.ComputeDeployDiff != nil {
		projectPath := v.project.Path
		go func() {
			result, err := v.cfg.ComputeDeployDiff(projectPath)
			app.QueueUpdateDraw(func() {
				if v.app == nil || v.mountGen != gen {
					return // view was unmounted or re-mounted
				}
				if err != nil {
					return // silently ignore diff errors
				}
				v.deployDiff = result
				v.renderHeader()
				v.maybeShowDeployToast()
			})
		}()
	}
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.app = nil
	v.list = nil
	v.dual = nil
	v.headerTV = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}

	// Dual-column navigation (h/l/Tab)
	if v.dual != nil {
		if consumed := v.dual.HandleKey(event); consumed == nil {
			return nil
		}
	}

	if event.Key() == tcell.KeyEnter {
		active := v.list
		if v.dual != nil {
			active = v.dual.activeList()
		}
		if _, item, ok := active.CurrentItem(); ok {
			if idx, refOk := item.Reference.(int); refOk {
				v.executeItem(idx)
			}
		}
		return nil
	}

	// Refresh deploy status via 'r' key
	if event.Rune() == 'r' {
		v.refreshDeployStatus()
		return nil
	}

	return event
}

// RefreshDeployStatus re-checks deploy freshness asynchronously.
// Exposed publicly so the post-deploy callback can trigger a refresh.
func (v *ProjectModeView) RefreshDeployStatus() {
	v.refreshDeployStatus()
}

func (v *ProjectModeView) refreshDeployStatus() {
	if v.app == nil || v.project == nil {
		return
	}
	gen := v.mountGen
	app := v.app
	projectPath := v.project.Path

	// Quick sync refresh of timestamp
	if v.cfg.CheckDeployStatus != nil {
		v.deployStatus = v.cfg.CheckDeployStatus(projectPath)
		v.renderHeader()
	}

	// Full async diff
	if v.cfg.ComputeDeployDiff != nil {
		go func() {
			result, err := v.cfg.ComputeDeployDiff(projectPath)
			app.QueueUpdateDraw(func() {
				if v.app == nil || v.mountGen != gen {
					return
				}
				if err == nil {
					v.deployDiff = result
					v.renderHeader()
				}
			})
		}()
	}
}

func (v *ProjectModeView) executeItem(idx int) {
	if idx < 0 || idx >= len(v.items) {
		return
	}
	item := v.items[idx]
	if item.Action != nil {
		item.Action()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Header rendering with deploy badge
// ─────────────────────────────────────────────────────────────────────────────

// renderHeader sets (or re-sets) the header text including the deploy badge.
func (v *ProjectModeView) renderHeader() {
	if v.headerTV == nil || v.project == nil {
		return
	}
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	banner := renderBanner(v.project.Name, 100)
	badge := v.buildDeployBadge()
	v.headerTV.SetText(fmt.Sprintf("\n%s\n  %s◆ Mode Projet%s\n  %s%s%s\n  %s",
		banner,
		secondary, reset,
		muted, v.project.Path, reset,
		badge,
	))
}

// buildDeployBadge returns a tview-colored string showing the deploy status.
// It uses the sync result (deployStatus) for the timestamp and the async
// result (deployDiff) for the exact change count when available.
func (v *ProjectModeView) buildDeployBadge() string {
	success := theme.ColorTag(theme.SuccessHex)
	warning := theme.ColorTag(theme.WarningHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	if v.deployStatus == nil || !v.deployStatus.Deployed {
		return fmt.Sprintf("%s○ Jamais déployé%s", warning, reset)
	}

	age := time.Since(v.deployStatus.DeployedAt)
	ageStr := formatDeployAge(age)

	// If async diff is available, use it for precise status
	if v.deployDiff != nil {
		if v.deployDiff.HasChanges {
			return fmt.Sprintf("%s○ %d changement(s) en attente%s  %s· Déployé %s%s",
				warning, v.deployDiff.ChangeCount, reset, muted, ageStr, reset)
		}
		return fmt.Sprintf("%s● À jour%s  %s· Déployé %s%s", success, reset, muted, ageStr, reset)
	}

	// Diff not yet computed — show timestamp only
	return fmt.Sprintf("%sDéployé %s%s", muted, ageStr, reset)
}

// maybeShowDeployToast fires a one-shot warning toast when deploy changes are detected.
func (v *ProjectModeView) maybeShowDeployToast() {
	if v.deployToastShown || v.shell == nil || v.deployDiff == nil {
		return
	}
	if v.deployDiff.HasChanges {
		v.deployToastShown = true
		v.shell.ShowToastMsg(
			fmt.Sprintf("%d fichier(s) modifié(s) depuis le dernier deploy — Ctrl+P > deploy", v.deployDiff.ChangeCount),
			false,
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Items
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModeView) buildItems() []projectModeItem {
	if v.project == nil {
		return nil
	}

	p := v.project
	navigate := func(viewID string) func() {
		return func() {
			if v.cfg.OnNavigate != nil {
				v.cfg.OnNavigate(viewID)
			}
		}
	}
	launch := func(agent string, args ...string) func() {
		return func() {
			if v.cfg.OnLaunchSession != nil {
				v.cfg.OnLaunchSession(p, agent, args...)
			}
		}
	}

	items := []projectModeItem{
		// ── Sessions section ──
		{Icon: "─", Label: "Sessions"},
		{Icon: "▶", Label: "Start Dev", Desc: "Lancer une session de développement", Action: launch("", "--dev")},
		{Icon: "◉", Label: "Audit", Desc: "Analyser le code (sécurité, perf, archi)", Action: launch("auditor")},
		{Icon: "◎", Label: "Review", Desc: "Code review du projet", Action: launch("reviewer")},
		{Icon: "◈", Label: "Debug", Desc: "Session de debug", Action: launch("")},
		// ── Board section ──
		{Icon: "─", Label: "Board"},
		{Icon: "⊞", Label: "Board", Desc: "Kanban du projet", Action: navigate("board")},
		{Icon: "⊟", Label: "Métriques", Desc: "Statistiques d'utilisation", Action: navigate("metrics")},
		{Icon: "⊝", Label: "Statut", Desc: "Santé et informations du projet", Action: navigate("status")},
		// ── Configuration section ──
		{Icon: "─", Label: "Configuration"},
		{Icon: "⊛", Label: "Config Projet", Desc: "Modifier la configuration", Action: navigate("project.config")},
		{Icon: "⊜", Label: "Worktrees", Desc: "Gérer les git worktrees", Action: navigate("worktrees")},
		// ── Deploy section ──
		{Icon: "─", Label: "Deploy"},
		{Icon: "⊘", Label: "Déployer", Desc: v.deployItemDesc(), Action: func() {
			if v.cfg.OnDeploy != nil {
				v.cfg.OnDeploy(p.Path)
			}
		}},
		{Icon: "⊙", Label: "Voir les changements", Desc: "Comparer hub vs projet", Action: func() {
			if v.cfg.OnViewDiff != nil {
				v.cfg.OnViewDiff(p.Path)
			}
		}},
	}

	// ── Team items (conditional) ────────────────────────────────────────
	if v.resolveTeam != nil {
		if tc := v.resolveTeam(); tc.Enabled {
			items = append(items,
				projectModeItem{Icon: "─", Label: "Équipe"},
				projectModeItem{Icon: "◫", Label: "Team Status", Desc: "Statut de l'équipe", Action: navigate("team.status")},
				projectModeItem{Icon: "◫", Label: "Team Board", Desc: "Kanban d'équipe", Action: navigate("team.board")},
				projectModeItem{Icon: "◫", Label: "Team Activity", Desc: "Activité récente", Action: navigate("team.activity")},
			)
		}
	}

	// ── Mode hub (standalone — no section) ──────────────────────────────
	items = append(items, projectModeItem{
		Icon: "↩", Label: "Mode Hub", Desc: "Revenir au TUI complet",
		Action: func() {
			if v.cfg.OnExitProjectMode != nil {
				v.cfg.OnExitProjectMode()
			}
		},
	})

	return items
}

// deployItemDesc returns the description for the "Déployer" menu item,
// enriched with change count when the async diff is available.
func (v *ProjectModeView) deployItemDesc() string {
	if v.deployDiff != nil && v.deployDiff.HasChanges {
		return fmt.Sprintf("Agents, skills et config (%d changement(s))", v.deployDiff.ChangeCount)
	}
	if v.deployStatus != nil && v.deployStatus.Deployed {
		return "Agents, skills et configuration"
	}
	return "Premier déploiement"
}

// splitItems distributes project mode items into left/right columns for dual mode.
// Left: Sessions + Board. Right: Configuration + Deploy + Équipe + Mode Hub.
func (v *ProjectModeView) splitItems() (left, right []widgets.SectionItem) {
	// Find the "Configuration" section boundary
	configIdx := -1
	for i, it := range v.items {
		if it.Icon == "─" && it.Label == "Configuration" {
			configIdx = i
			break
		}
	}

	for i, it := range v.items {
		si := projectItemToSectionItem(it, i)
		if configIdx >= 0 && i >= configIdx {
			right = append(right, si)
		} else {
			left = append(left, si)
		}
	}
	return
}

func projectItemToSectionItem(it projectModeItem, idx int) widgets.SectionItem {
	if it.Icon == "─" {
		return widgets.SectionItem{
			MainText: it.Label,
			IsHeader: true,
		}
	}
	return widgets.SectionItem{
		MainText:      fmt.Sprintf("%s  %s", it.Icon, it.Label),
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// formatDeployAge returns a human-readable relative time string.
func formatDeployAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "à l'instant"
	case d < time.Hour:
		return fmt.Sprintf("il y a %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("il y a %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("il y a %dj", int(d.Hours()/24))
	}
}

// ContextCommands returns contextual commands for the omnibar.
// Since ADR-032 Phase 3, commands are filtered by mode at the registry level.
// The global command registry with Modes annotations replaces the need for
// per-view contextual command duplication.
func (v *ProjectModeView) ContextCommands() []ContextCommand {
	return nil
}
