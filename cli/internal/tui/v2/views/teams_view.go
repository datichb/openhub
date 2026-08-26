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
	shell      ShellAccess
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

// SetShell provides the shell reference for modal interactions.
func (v *TeamsView) SetShell(s ShellAccess) { v.shell = s }

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
			v.showHelp()
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
	teamID, ok := item.Reference.(string)
	if !ok || teamID == "" {
		return
	}
	// Navigate to team detail view
	if v.onNavigate != nil {
		v.onNavigate("team.detail")
	}
}

func (v *TeamsView) handleAdd() {
	if v.shell == nil {
		return
	}
	// Step 1: Repo URL
	v.shell.ShowInputModal("URL du repo team-state", "", func(repo string) {
		if repo == "" {
			return
		}
		// Step 2: Member ID
		v.shell.ShowInputModal("Votre member-id", "", func(memberID string) {
			if memberID == "" {
				return
			}
			// Step 3: Team ID (short identifier)
			v.shell.ShowInputModal("ID court de l'équipe", "", func(teamID string) {
				if teamID == "" {
					return
				}
				// Step 4: Display name (optional)
				v.shell.ShowInputModal("Nom d'affichage (optionnel)", "", func(name string) {
					// Push undo state before mutation
					v.undoStack.Push(copyTeams(v.cfg.Teams))

					newTeam := config.TeamConfig{
						ID:        teamID,
						Name:      name,
						Enabled:   true,
						StateRepo: repo,
						MemberID:  memberID,
					}
					v.cfg.Teams = append(v.cfg.Teams, newTeam)

					if v.onSave != nil {
						v.onSave(v.cfg)
					}
					v.rebuild()
					v.shell.ShowToastMsg("Équipe ajoutée: "+newTeam.DisplayName(), true)
				})
			})
		})
	})
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

	if v.shell == nil {
		return
	}

	v.shell.ShowSelectModal(fmt.Sprintf("Supprimer l'équipe %q ?", teamID), []SelectOption{
		{Label: "Confirmer la suppression", Value: "yes"},
		{Label: "Annuler", Value: ""},
	}, "", func(choice string) {
		if choice != "yes" {
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
		v.shell.ShowToastMsg("Équipe "+teamID+" supprimée (u pour annuler)", true)
	})
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

func (v *TeamsView) showHelp() {
	if v.shell == nil {
		return
	}
	helpText := "Raccourcis clavier :\n\n" +
		"  Enter   Ouvrir le détail de l'équipe\n" +
		"  a       Ajouter une équipe\n" +
		"  d       Retirer l'équipe sélectionnée\n" +
		"  s       Synchroniser le team-state\n" +
		"  r       Rafraîchir la liste\n" +
		"  u       Annuler la dernière action\n" +
		"  ?       Afficher cette aide\n" +
		"  :       Ouvrir l'omnibar"
	v.shell.ShowScrollableModal("Aide — Équipes", helpText, []ModalAction{
		{Label: "OK", Callback: func() {}},
	})
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
