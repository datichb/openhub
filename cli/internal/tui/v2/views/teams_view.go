package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// TeamsView displays and manages the user's team memberships (ADR-029).
// It shows all configured teams with their sync status and allows
// add/remove/sync operations.
type TeamsView struct {
	list       *widgets.SectionedList
	app        *tview.Application
	cfg        *config.Config
	onSync     func(teamID string)
	onSave     func(cfg *config.Config)
	onNavigate func(viewID string)
	undoStack  *widgets.UndoStack[[]config.TeamConfig]
}

// TeamsViewDeps holds the dependencies for constructing a TeamsView.
type TeamsViewDeps struct {
	Config     *config.Config
	OnSync     func(teamID string)       // Called when user requests team-state sync
	OnSave     func(cfg *config.Config)   // Called to persist config changes
	OnNavigate func(viewID string)        // Called to navigate to another view
}

// NewTeamsView creates a new TeamsView.
func NewTeamsView(deps TeamsViewDeps) *TeamsView {
	v := &TeamsView{
		list:       widgets.NewSectionedList(),
		cfg:        deps.Config,
		onSync:     deps.OnSync,
		onSave:     deps.OnSave,
		onNavigate: deps.OnNavigate,
		undoStack:  widgets.NewUndoStack[[]config.TeamConfig](10),
	}
	v.list.SetItemSelectedFunc(v.handleSelect)
	return v
}

func (v *TeamsView) ID() string    { return "teams" }
func (v *TeamsView) Title() string { return i18n.T("tui.teams") }

func (v *TeamsView) StatusHints() string {
	return "Enter:détail  a:ajouter  d:retirer  s:sync  r:refresh  u:undo"
}

func (v *TeamsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.list.SetApp(app)
	v.rebuild()

	content.Clear()
	content.AddItem(v.list, 0, 1, true)
	app.SetFocus(v.list)
}

func (v *TeamsView) Unmount() {}

func (v *TeamsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'a':
			v.handleAdd()
			return nil
		case 'd':
			v.handleDelete()
			return nil
		case 's':
			v.handleSync()
			return nil
		case 'r':
			v.rebuild()
			return nil
		case 'u':
			v.handleUndo()
			return nil
		case '?':
			// TODO: show help modal
			return nil
		}
	}
	return event
}

// ContextCommands implements CommandProvider for omnibar integration.
func (v *TeamsView) ContextCommands() []ContextCommand {
	commands := []ContextCommand{
		{
			ID:          "teams.add",
			Label:       "Ajouter une équipe",
			Aliases:     []string{"add", "new", "ajouter"},
			Description: "Configurer une nouvelle équipe",
			Category:    "Équipes",
			Action:      v.handleAdd,
		},
		{
			ID:          "teams.refresh",
			Label:       "Rafraîchir",
			Aliases:     []string{"refresh", "reload"},
			Description: "Recharger la liste des équipes",
			Category:    "Équipes",
			Action:      v.rebuild,
		},
	}

	// Navigation commands — contextual to team view
	if v.onNavigate != nil {
		commands = append(commands,
			ContextCommand{
				ID:          "teams.board",
				Label:       i18n.T("tui.team.board"),
				Aliases:     []string{"board", "kanban"},
				Description: i18n.T("tui.team.board.desc"),
				Category:    i18n.T("tui.category.team"),
				Action:      func() { v.onNavigate("team.board") },
			},
			ContextCommand{
				ID:          "teams.status",
				Label:       i18n.T("tui.team.status"),
				Aliases:     []string{"status", "statut"},
				Description: i18n.T("tui.team.status.desc"),
				Category:    i18n.T("tui.category.team"),
				Action:      func() { v.onNavigate("team.status") },
			},
			ContextCommand{
				ID:          "teams.activity",
				Label:       i18n.T("tui.team.activity"),
				Aliases:     []string{"activity", "activite"},
				Description: i18n.T("tui.team.activity.desc"),
				Category:    i18n.T("tui.category.team"),
				Action:      func() { v.onNavigate("team.activity") },
			},
		)
	}

	// Add per-team sync commands
	for _, t := range v.cfg.Teams {
		teamID := t.ID
		commands = append(commands, ContextCommand{
			ID:          fmt.Sprintf("teams.sync.%s", teamID),
			Label:       fmt.Sprintf("Sync %s", t.DisplayName()),
			Aliases:     []string{"sync", teamID},
			Description: fmt.Sprintf("Synchroniser le team-state de %s", t.DisplayName()),
			Category:    "Équipes",
			Action:      func() { v.syncTeam(teamID) },
		})
	}

	return commands
}

func (v *TeamsView) rebuild() {
	items := make([]widgets.SectionItem, 0, len(v.cfg.Teams)*3+2)

	if len(v.cfg.Teams) == 0 {
		items = append(items, widgets.SectionItem{
			MainText:      "Aucune équipe configurée",
			SecondaryText: "Appuyez sur 'a' pour ajouter une équipe",
		})
	} else {
		items = append(items, widgets.SectionItem{
			MainText: "Équipes",
			IsHeader: true,
		})

		for _, t := range v.cfg.Teams {
			status := fmt.Sprintf("%s actif", theme.ColorTag(theme.SuccessHex)+"✓"+theme.TagColor)
			if !t.Enabled {
				status = "inactif"
			}

			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%s  %s", t.ID, t.DisplayName()),
				SecondaryText: fmt.Sprintf("membre: %s | repo: %s | %s", t.MemberID, truncateString(t.StateRepo, 40), status),
				Reference:     t.ID,
			})
		}
	}

	v.list.SetItems(items)
}

func (v *TeamsView) handleSelect(index int, item widgets.SectionItem) {
	// TODO: navigate to TeamDetailView for the selected team
	_ = item.Reference
}

func (v *TeamsView) handleAdd() {
	// TODO: open modal chain to add a new team (repo → member-id → id → name)
	// For now this is a placeholder; the full modal flow will be implemented
	// when the modal infrastructure is wired up in Phase 4.
}

func (v *TeamsView) handleDelete() {
	idx, item, ok := v.list.CurrentItem()
	if !ok || item.Reference == nil {
		return
	}
	_ = idx
	teamID, ok := item.Reference.(string)
	if !ok {
		return
	}

	// Push undo state before mutation
	v.undoStack.Push(copyTeams(v.cfg.Teams))

	// Remove from config
	newTeams := make([]config.TeamConfig, 0, len(v.cfg.Teams)-1)
	for _, t := range v.cfg.Teams {
		if t.ID != teamID {
			newTeams = append(newTeams, t)
		}
	}
	v.cfg.Teams = newTeams

	// Auto-save
	if v.onSave != nil {
		v.onSave(v.cfg)
	}

	v.rebuild()
}

func (v *TeamsView) handleSync() {
	_, item, ok := v.list.CurrentItem()
	if !ok || item.Reference == nil {
		return
	}
	teamID, ok := item.Reference.(string)
	if !ok {
		return
	}
	v.syncTeam(teamID)
}

func (v *TeamsView) syncTeam(teamID string) {
	if v.onSync != nil {
		v.onSync(teamID)
	}
}

func (v *TeamsView) handleUndo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		return
	}
	v.cfg.Teams = prev
	if v.onSave != nil {
		v.onSave(v.cfg)
	}
	v.rebuild()
}

// copyTeams creates a shallow copy of a TeamConfig slice (for undo snapshots).
func copyTeams(teams []config.TeamConfig) []config.TeamConfig {
	cp := make([]config.TeamConfig, len(teams))
	copy(cp, teams)
	return cp
}

// truncateString truncates a string for display.
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
