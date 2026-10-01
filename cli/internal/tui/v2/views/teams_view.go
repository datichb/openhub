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
	onSave     func(cfg *config.Config) error
	onNavigate func(viewID string)
	undoStack  *widgets.UndoStack[[]config.TeamConfig]
}

// TeamsViewDeps holds the dependencies for constructing a TeamsView.
type TeamsViewDeps struct {
	Config     *config.Config
	OnSync     func(teamID string)            // Called when user requests team-state sync
	OnSave     func(cfg *config.Config) error // Called to persist config changes
	OnNavigate func(viewID string)            // Called to navigate to another view
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
	return fmt.Sprintf("Enter %s · a %s · d %s · s %s · r %s · u %s",
		i18n.T("tui.hints.detail"),
		i18n.T("tui.hints.add"),
		i18n.T("tui.hints.delete"),
		i18n.T("tui.hints.sync"),
		i18n.T("tui.hints.refresh"),
		i18n.T("tui.hints.undo"),
	)
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
	case tcell.KeyEnter:
		if _, item, ok := v.list.CurrentItem(); ok {
			v.handleSelect(0, item)
		}
		return nil
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
		}
	}
	return event
}

// ContextCommands implements CommandProvider for omnibar integration.
func (v *TeamsView) ContextCommands() []ContextCommand {
	commands := []ContextCommand{
		{
			ID:          "teams.add",
			Label:       i18n.T("tui.teams.add_team"),
			Aliases:     []string{"add", "new", "ajouter"},
			Description: i18n.T("tui.teams.add_team_desc"),
			Category:    i18n.T("tui.teams.category"),
			Action:      v.handleAdd,
		},
		{
			ID:          "teams.refresh",
			Label:       i18n.T("tui.teams.refresh"),
			Aliases:     []string{"refresh", "reload"},
			Description: i18n.T("tui.teams.refresh_desc"),
			Category:    i18n.T("tui.teams.category"),
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
				Aliases:     []string{"activity", "activite"}, //nolint:misspell // French search alias
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
			Label:       i18n.Tf("tui.teams.sync_team", t.DisplayName()),
			Aliases:     []string{"sync", teamID},
			Description: i18n.Tf("tui.teams.sync_team_desc", t.DisplayName()),
			Category:    i18n.T("tui.teams.category"),
			Action:      func() { v.syncTeam(teamID) },
		})
	}

	return commands
}

func (v *TeamsView) rebuild() {
	items := make([]widgets.SectionItem, 0, len(v.cfg.Teams)*3+2)

	if len(v.cfg.Teams) == 0 {
		items = append(items, widgets.SectionItem{
			MainText:      i18n.T("tui.teams.no_teams"),
			SecondaryText: i18n.T("tui.teams.no_teams_hint"),
		})
	} else {
		items = append(items, widgets.SectionItem{
			MainText: i18n.T("tui.teams.header"),
			IsHeader: true,
		})

		for _, t := range v.cfg.Teams {
			status := fmt.Sprintf("%s %s", theme.ColorTag(theme.SuccessHex)+"✓"+theme.TagColor, i18n.T("tui.teams.status_active"))
			if !t.Enabled {
				status = i18n.T("tui.teams.status_inactive")
			}

			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%s  %s", t.ID, t.DisplayName()),
				SecondaryText: i18n.Tf("tui.teams.member_info", t.MemberID, truncateString(t.StateRepo, 40), status),
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
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.T("tui.teams.add_team"),
		Fields: []FormField{
			{Label: i18n.T("tui.teams.field_repo_url"), Key: "repo", Type: FieldText, Required: true},
			{Label: i18n.T("tui.teams.field_member_id"), Key: "member_id", Type: FieldText, Required: true},
			{Label: i18n.T("tui.teams.field_team_id"), Key: "team_id", Type: FieldText, Required: true},
			{Label: i18n.T("tui.teams.field_display_name"), Key: "name", Type: FieldText},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			repo := values["repo"]
			memberID := values["member_id"]
			teamID := values["team_id"]
			name := values["name"]
			if repo == "" || memberID == "" || teamID == "" {
				v.shell.ShowToastMsg(i18n.T("tui.teams.fields_required"), false)
				return
			}
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
				if err := v.onSave(v.cfg); err != nil {
					// Rollback in-memory state.
					if prev, ok := v.undoStack.Pop(); ok {
						v.cfg.Teams = prev
					}
					v.rebuild()
					v.shell.ShowToastMsg("save failed: "+err.Error(), false)
					return
				}
			}
			v.rebuild()
			v.shell.ShowToastMsg(i18n.Tf("tui.teams.team_added", newTeam.DisplayName()), true)
		},
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

	v.shell.ShowSelectModal(i18n.Tf("tui.teams.confirm_delete", teamID), []SelectOption{
		{Label: i18n.T("tui.teams.cancel"), Value: ""},
		{Label: i18n.T("tui.teams.confirm_delete_btn"), Value: "yes"},
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
			if err := v.onSave(v.cfg); err != nil {
				// Rollback in-memory state.
				if prev, ok := v.undoStack.Pop(); ok {
					v.cfg.Teams = prev
				}
				v.rebuild()
				v.shell.ShowToastMsg("save failed: "+err.Error(), false)
				return
			}
		}

		v.rebuild()
		v.shell.ShowToastMsg(i18n.Tf("tui.teams.team_deleted", teamID), true)
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
	current := copyTeams(v.cfg.Teams)
	v.cfg.Teams = prev
	if v.onSave != nil {
		if err := v.onSave(v.cfg); err != nil {
			// Restore pre-undo state and re-push the snapshot.
			v.cfg.Teams = current
			v.undoStack.Push(prev)
			v.shell.ShowToastMsg("undo failed: "+err.Error(), false)
			return
		}
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
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
