package views

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// The keys of the Teams view that are not the ones of the convention: p opens
// the Team view of the team under the cursor, * makes it the default team (★).
// Archive is x and restore is R, rename is e (keys.Archive, keys.Resume,
// keys.Edit).
var (
	teamsOpenKey    = keys.Own("teams.open", 'p', "v54u.teams.key.team_view")
	teamsDefaultKey = keys.Own("teams.default", '*', "v54u.teams.key.default")
)

// newTeamRef is the Reference of the « Nouvelle équipe » row of the list.
const newTeamRef = "\x00new"

// TeamsView displays and manages the user's team memberships (ADR-029).
// It shows all configured teams with their sync status and allows
// add/remove/sync operations.
type TeamsView struct {
	list       *widgets.SectionedList
	page       *widgets.ListDetailPage
	app        *tview.Application
	shell      ShellAccess
	cfg        *config.Config
	onSync     func(teamID string)
	onUpdate   func(fn func(*config.Config) error) error
	onNavigate func(viewID string)
	onSelect   func(teamID string)
	onAdd      func()
	onOpen     func(teamID string)
	onDefault  func(teamID string) error
	onArchive  func(teamID string, archive bool) (note string, err error)
	onRemove   func(teamID string) (detached []string, note string, err error)
	prepare    func(teamID string, removeMember bool) TeamLeave
	projectsOf func(teamID string) []string
	undoStack  *widgets.UndoStack[[]config.TeamConfig]

	// memberNames: the display name of the member of each team, by team ID,
	// read in the team-states (the IDs until they are there).
	memberNames func(ctx context.Context, teams []config.TeamConfig) map[string]string
	shown       map[string]string
	nameLoads   loadSlot
	planLoads   loadSlot
}

// TeamLeave is a departure from a team, ready to run off the event loop.
type TeamLeave struct {
	// Plan says what leaving does, one line per step (it reads the clone and
	// the keychain).
	Plan func(ctx context.Context) []string
	// Leave releases the claims of the member, deletes the clone and the keys
	// of the keychain, removes the team from hub.toml and detaches its
	// projects. It reaches the team-state. It returns what it did, one line
	// per step.
	Leave func(ctx context.Context) (summary []string, err error)
}

// TeamsViewDeps holds the dependencies for constructing a TeamsView.
type TeamsViewDeps struct {
	Config *config.Config
	OnSync func(teamID string) // Called when user requests team-state sync
	// OnUpdate changes hub.toml: fn receives the file as it is on disk and
	// changes only the teams of the action; the host then brings the shared
	// config up to date with the result.
	OnUpdate   func(fn func(*config.Config) error) error
	OnNavigate func(viewID string) // Called to navigate to another view
	// OnSelectTeam tells the host which team the user opened from the list,
	// right before OnNavigate("team.detail"): the views of the team then work
	// on that team (the default team of the hub otherwise).
	OnSelectTeam func(teamID string)
	// OnAdd adds a team: the host opens the assistant that joins it (the
	// repository, its access, the clone, the card of the member). The list
	// shows the new team when the assistant is left. Nil: the view asks the
	// repository, the member and the team ID itself and only writes hub.toml
	// (no clone, no card).
	OnAdd func()
	// OnOpen opens the Team view on the team p was pressed on. Nil: the key
	// does nothing.
	OnOpen func(teamID string)
	// OnSetDefault makes the team the default team of hub.toml (« Définir par
	// défaut »). Nil: the view marks it in its own list and saves it.
	OnSetDefault func(teamID string) error
	// OnArchive archives (or, archive false, restores) the team in hub.toml;
	// the note tells where the default team went when it was that team. Nil:
	// the view changes its own list and saves it.
	OnArchive func(teamID string, archive bool) (note string, err error)
	// OnRemove takes the team out of hub.toml and detaches its projects; it
	// returns the names of the projects it detached and a note on the default
	// team. Nil: the view only removes the team from its list and saves it
	// (the projects stay attached).
	OnRemove func(teamID string) (detached []string, note string, err error)
	// PrepareLeave readies « Quitter l'équipe » for a team: it is called on the
	// event loop (it takes what it needs from the Config the loop owns) and
	// gives the two steps that run off the loop. Nil: the view only offers to
	// remove the team from hub.toml.
	PrepareLeave func(teamID string, removeMember bool) TeamLeave
	// ProjectsOf names the projects attached to a team, for the confirmation
	// of a removal. Nil: none is announced.
	ProjectsOf func(teamID string) []string
	// MemberNames gives the display name of the member of each team (team ID →
	// name) from the team-states. It reads files: called off the event loop.
	// Nil: the member IDs.
	MemberNames func(ctx context.Context, teams []config.TeamConfig) map[string]string
}

var _ View = (*TeamsView)(nil)

// NewTeamsView creates a new TeamsView.
func NewTeamsView(deps TeamsViewDeps) *TeamsView {
	v := &TeamsView{
		list:       widgets.NewSectionedList(),
		cfg:        deps.Config,
		onSync:     deps.OnSync,
		onUpdate:   deps.OnUpdate,
		onNavigate: deps.OnNavigate,
		onSelect:   deps.OnSelectTeam,
		onAdd:      deps.OnAdd,
		onOpen:     deps.OnOpen,
		onDefault:  deps.OnSetDefault,
		onArchive:  deps.OnArchive,
		onRemove:   deps.OnRemove,
		prepare:    deps.PrepareLeave,
		projectsOf: deps.ProjectsOf,
		undoStack:  widgets.NewUndoStack[[]config.TeamConfig](10),

		memberNames: deps.MemberNames,
	}
	v.list.SetItemSelectedFunc(v.handleSelect)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showDetail(it) })
	return v
}

// SetShell provides the shell reference for modal interactions.
func (v *TeamsView) SetShell(s ShellAccess) { v.shell = s }

func (v *TeamsView) ID() string    { return "teams" }
func (v *TeamsView) Title() string { return i18n.T("tui.teams") }

func (v *TeamsView) StatusHints() string {
	return keys.Hints(keys.Open.HintAs(i18n.T("tui.hints.detail")), teamsOpenKey.Hint(), keys.Create.Hint(),
		teamsDefaultKey.Hint(), keys.Edit.HintAs(i18n.T("v54u.teams.key.rename")),
		keys.Archive.HintAs(i18n.T("v54u.teams.key.archive")), keys.Resume.HintAs(i18n.T("v54u.teams.key.restore")),
		keys.Delete.Hint(), cfgSyncKey.Hint(), keys.Refresh.Hint(), cfgUndoKey.Hint())
}

func (v *TeamsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.list.SetApp(app)
	v.page = newListPage(v.Title(), v.list)
	v.rebuild()
	v.loadMemberNames()

	v.page.Mount(content)
	app.SetFocus(v.list)
}

func (v *TeamsView) Unmount() {
	v.nameLoads.Invalidate()
	v.planLoads.Invalidate()
	v.page = nil
}

// loadMemberNames reads the display name of the member of each team off the
// event loop, then draws the list again with them.
func (v *TeamsView) loadMemberNames() {
	if v.memberNames == nil || v.page == nil {
		return
	}
	read, teams := v.memberNames, copyTeams(v.cfg.Teams)
	load(v.app, shellContext(v.shell), &v.nameLoads, func(ctx context.Context) (map[string]string, error) {
		return read(ctx, teams), nil
	}, func(names map[string]string, _ error) {
		if v.page == nil {
			return
		}
		v.shown = names
		v.rebuild()
	})
}

// memberOf is the member of the team as shown: their display name, or the ID
// they gave when the team-state does not tell it.
func (v *TeamsView) memberOf(t config.TeamConfig) string {
	if name := v.shown[t.ID]; name != "" {
		return name
	}
	return t.MemberID
}

// teamName is the name of the team with this ID in the configuration (the ID
// when it is not there).
func (v *TeamsView) teamName(id string) string {
	for _, t := range v.cfg.Teams {
		if t.ID == id {
			return t.DisplayName()
		}
	}
	return id
}

// teamByID is the team of hub.toml with this ID (nil when there is none, or
// for the row of the new team).
func (v *TeamsView) teamByID(id string) *config.TeamConfig {
	if id == "" || id == newTeamRef {
		return nil
	}
	for i := range v.cfg.Teams {
		if v.cfg.Teams[i].ID == id {
			return &v.cfg.Teams[i]
		}
	}
	return nil
}

// currentTeam is the team under the cursor (nil on the row of the new team,
// or without any).
func (v *TeamsView) currentTeam() *config.TeamConfig {
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return nil
	}
	id, _ := it.Reference.(string)
	return v.teamByID(id)
}

// showDetail fills the detail pane with the team of it.
func (v *TeamsView) showDetail(it widgets.SectionItem) {
	if v.page == nil {
		return
	}
	id, _ := it.Reference.(string)
	team := v.teamByID(id)
	if team == nil {
		var d detailText
		if len(v.cfg.Teams) == 0 {
			d.title(i18n.T("tui.teams.no_teams"))
		} else {
			d.title(i18n.T("v54u.home.team_new"))
		}
		d.para(i18n.T("tui.teams.no_teams_hint"))
		d.keys(append(keys.Pairs(keys.Create), cfgUndoKey.Keys(), cfgUndoKey.Label())...)
		v.page.SetDetail(d.String())
		return
	}
	status := theme.Paint(theme.SuccessHex, theme.SymSuccess) + " " + i18n.T("tui.teams.status_active")
	if !team.Enabled {
		status = muted(i18n.T("tui.teams.status_inactive"))
	}
	repo := team.StateRepo
	if team.Solo {
		repo = i18n.T("tui.solo.teams_label")
	}
	var d detailText
	d.title(tview.Escape(team.DisplayName()))
	d.para(i18n.T("tui.detail.team.desc"))
	d.field("ID :", tview.Escape(team.ID))
	d.field(i18n.T("tui.detail.member"), tview.Escape(v.memberOf(*team)))
	d.field(i18n.T("tui.detail.status"), status)
	d.field(i18n.T("v54u.teams.field_default"), v.defaultText(*team))
	d.field(i18n.T("tui.detail.repo"), tview.Escape(repo))
	d.field(i18n.T("tui.detail.path"), tview.Escape(team.StatePath))
	pairs := []string{keys.Open.Keys(), i18n.T("tui.hints.detail"), teamsOpenKey.Keys(), teamsOpenKey.Label()}
	pairs = append(pairs, cfgSyncKey.Keys(), cfgSyncKey.Label(), teamsDefaultKey.Keys(), teamsDefaultKey.Label(),
		keys.Edit.Keys(), i18n.T("v54u.teams.key.rename"), keys.Archive.Keys(), i18n.T("v54u.teams.key.archive"),
		keys.Resume.Keys(), i18n.T("v54u.teams.key.restore"))
	pairs = append(pairs, keys.Pairs(keys.Create, keys.Delete, keys.Refresh)...)
	d.keys(append(pairs, cfgUndoKey.Keys(), cfgUndoKey.Label())...)
	v.page.SetDetail(d.String())
}

// isDefault reports whether t is the default team of hub.toml.
func (v *TeamsView) isDefault(t config.TeamConfig) bool {
	def := v.cfg.DefaultTeam()
	return def != nil && def.ID == t.ID
}

// defaultText says in the detail whether the team is the default team.
func (v *TeamsView) defaultText(t config.TeamConfig) string {
	switch {
	case v.isDefault(t) && t.Default:
		return theme.Paint(theme.SuccessHex, theme.SymDefault) + " " + i18n.T("v54u.teams.default_yes")
	case v.isDefault(t):
		return theme.Paint(theme.SuccessHex, theme.SymDefault) + " " + i18n.Tf("v54u.teams.default_implicit", teamsDefaultKey.Keys())
	case t.Solo:
		return muted(i18n.T("v54u.teams.default_solo"))
	case !t.Enabled:
		return muted(i18n.T("v54u.teams.default_archived"))
	}
	return muted(i18n.Tf("v54u.teams.default_no", teamsDefaultKey.Keys()))
}

func (v *TeamsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Open.Matches(event):
		if _, item, ok := v.list.CurrentItem(); ok {
			v.handleSelect(0, item)
		}
		return nil
	case keys.Create.Matches(event):
		v.handleAdd()
		return nil
	case keys.Delete.Matches(event):
		v.handleDelete()
		return nil
	case keys.Refresh.Matches(event):
		v.rebuild()
		v.loadMemberNames()
		return nil
	case cfgSyncKey.Matches(event):
		v.handleSync()
		return nil
	case cfgUndoKey.Matches(event):
		v.handleUndo()
		return nil
	case teamsOpenKey.Matches(event):
		v.handleOpen()
		return nil
	case teamsDefaultKey.Matches(event):
		v.handleDefault()
		return nil
	case keys.Edit.Matches(event):
		v.handleRename()
		return nil
	case keys.Archive.Matches(event):
		v.handleArchive(true)
		return nil
	case keys.Resume.Matches(event):
		v.handleArchive(false)
		return nil
	}
	return event
}

// ContextCommands implements CommandProvider for omnibar integration.
func (v *TeamsView) ContextCommands() []ContextCommand {
	commands := []ContextCommand{
		{
			ID:          "teams.add",
			Label:       i18n.T("tui.teams.add_team"),
			Aliases:     []string{"add", "new", "ajouter", "creer", "créer", "rejoindre"},
			Description: i18n.T("tui.teams.add_team_desc"),
			Category:    i18n.T("tui.category.team"),
			Action:      v.handleAdd,
		},
		{
			ID:          "teams.open",
			Label:       i18n.T("v54u.teams.open"),
			Aliases:     []string{"open", "ouvrir", "team view", "vue equipe"},
			Description: i18n.T("v54u.teams.open_desc"),
			Category:    i18n.T("tui.category.team"),
			Action:      v.handleOpen,
		},
		{
			ID:          "teams.default",
			Label:       i18n.T("v54u.teams.set_default"),
			Aliases:     []string{"default", "defaut", "par defaut"},
			Description: i18n.T("v54u.teams.set_default_desc"),
			Category:    i18n.T("tui.category.team"),
			Action:      v.handleDefault,
		},
		{
			ID:          "teams.refresh",
			Label:       i18n.T("tui.teams.refresh"),
			Aliases:     []string{"refresh", "reload"},
			Description: i18n.T("tui.teams.refresh_desc"),
			Category:    i18n.T("tui.category.team"),
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
			Category:    i18n.T("tui.category.team"),
			Action:      func() { v.syncTeam(teamID) },
		})
	}

	return commands
}

func (v *TeamsView) rebuild() {
	// The cursor stays on the team it was on (a team that is renamed,
	// archived or made the default does not send it back to the top).
	keep := ""
	if _, it, ok := v.list.CurrentItem(); ok {
		keep, _ = it.Reference.(string)
	}
	items := make([]widgets.SectionItem, 0, len(v.cfg.Teams)+3)

	if len(v.cfg.Teams) > 0 {
		items = append(items, widgets.SectionItem{
			MainText: i18n.T("tui.teams.header"),
			IsHeader: true,
		})

		for _, t := range v.cfg.Teams {
			status := theme.Paint(theme.SuccessHex, theme.SymSuccess) + " " + i18n.T("tui.teams.status_active")
			if !t.Enabled {
				status = i18n.T("tui.teams.status_inactive")
			}
			main := fmt.Sprintf("%s  %s", t.ID, t.DisplayName())
			if v.isDefault(t) {
				main += "  " + theme.Paint(theme.SuccessHex, theme.SymDefault)
			}

			items = append(items, widgets.SectionItem{
				MainText:      main,
				SecondaryText: i18n.Tf("tui.teams.member_info", v.memberOf(t), teamRepoLabel(t), status),
				Reference:     t.ID,
			})
		}
	}

	// The row of the new team closes the list; without any team it is the
	// list, and says what a team is for.
	desc := i18n.T("v54u.home.team_new_desc")
	if len(v.cfg.Teams) == 0 {
		desc = i18n.T("v54u.home.team_empty")
	}
	items = append(items, widgets.SectionItem{
		MainText:      "➕  " + i18n.T("v54u.home.team_new"),
		SecondaryText: desc,
		Reference:     newTeamRef,
	})

	v.list.SetItems(items)
	if keep != "" {
		for i, it := range items {
			if ref, _ := it.Reference.(string); ref == keep {
				v.list.SelectIndex(i)
				break
			}
		}
	}
	_, it, ok := v.list.CurrentItem()
	if !ok && len(items) > 0 {
		it = items[0]
	}
	v.showDetail(it)
}

func (v *TeamsView) handleSelect(index int, item widgets.SectionItem) {
	teamID, ok := item.Reference.(string)
	if !ok || teamID == "" {
		return
	}
	if teamID == newTeamRef {
		v.handleAdd()
		return
	}
	// The detail shows the team chosen here, not the default one.
	if v.onSelect != nil {
		v.onSelect(teamID)
	}
	if v.onNavigate != nil {
		v.onNavigate("team.detail")
	}
}

func (v *TeamsView) handleAdd() {
	if v.onAdd != nil {
		v.onAdd()
		return
	}
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
			// A team of another repository is never replaced, nor is a repository
			// configured twice.
			if other := v.cfg.FindTeam(teamID); other != nil {
				v.shell.ShowToastMsg(i18n.Tf("v533a.teams.duplicate_id", teamID, teamRepoLabel(*other)), false)
				return
			}
			if other := v.cfg.FindTeamByRepo(repo); other != nil {
				v.shell.ShowToastMsg(i18n.Tf("v533a.teams.duplicate_repo", other.ID), false)
				return
			}
			v.undoStack.Push(copyTeams(v.cfg.Teams))
			newTeam := config.TeamConfig{
				ID:        teamID,
				Name:      name,
				Enabled:   true,
				StateRepo: repo,
				StatePath: config.ResolveStatePath(repo, ""),
				MemberID:  memberID,
			}
			prevTeams := copyTeams(v.cfg.Teams)
			v.cfg.Teams = append(v.cfg.Teams, newTeam)
			if v.onUpdate != nil {
				if err := v.onUpdate(teamsPatch(prevTeams, v.cfg.Teams)); err != nil {
					// Rollback in-memory state.
					if prev, ok := v.undoStack.Pop(); ok {
						v.cfg.Teams = prev
					}
					v.rebuild()
					v.shell.ShowToastMsg(i18n.Tf("tui.teams.save_failed", err.Error()), false)
					return
				}
			}
			v.rebuild()
			v.loadMemberNames()
			v.shell.ShowToastMsg(i18n.Tf("tui.teams.team_added", newTeam.DisplayName()), true)
		},
	})
}

// handleDelete is d. A team with a team-state offers two ways out: leave it
// (the claims of the member are released, the clone and the keys are deleted,
// then hub.toml), or only take it out of hub.toml. The other cases ask for the
// second directly.
func (v *TeamsView) handleDelete() {
	team := v.currentTeam()
	if team == nil || v.shell == nil {
		return
	}
	if v.prepare == nil || team.Solo {
		v.confirmRemove(team)
		return
	}
	teamID := team.ID
	v.shell.ShowSelectModal(i18n.Tf("v54l.teams.delete.title", team.DisplayName()), []SelectOption{
		{Label: i18n.T("v54l.teams.delete.leave"), Value: "leave"},
		{Label: i18n.T("v54l.teams.delete.leave_card"), Value: "leave_card"},
		{Label: i18n.T("v54l.teams.delete.remove"), Value: "remove"},
	}, "", func(choice string) {
		t := v.teamByID(teamID)
		if t == nil {
			return
		}
		switch choice {
		case "leave":
			v.confirmLeave(t, false)
		case "leave_card":
			v.confirmLeave(t, true)
		case "remove":
			v.confirmRemove(t)
		}
	})
}

// confirmLeave shows what leaving does (read off the event loop), then leaves
// on the confirmation.
func (v *TeamsView) confirmLeave(team *config.TeamConfig, removeMember bool) {
	name := team.DisplayName()
	leave := v.prepare(team.ID, removeMember)
	plan := func(context.Context) ([]string, error) { return nil, nil }
	if leave.Plan != nil {
		read := leave.Plan
		plan = func(ctx context.Context) ([]string, error) { return read(ctx), nil }
	}
	load(v.app, shellContext(v.shell), &v.planLoads, plan, func(lines []string, _ error) {
		if v.page == nil || v.shell == nil {
			return
		}
		v.shell.Confirm(i18n.Tf("v54l.teams.leave.confirm_title", name), strings.Join(lines, "\n"), true, func() { v.doLeave(name, leave) })
	})
}

// doLeave leaves the team in the background (the team-state is reached) and
// says what it did.
func (v *TeamsView) doLeave(name string, leave TeamLeave) {
	if leave.Leave == nil {
		return
	}
	app, ctx := v.app, shellContext(v.shell)
	v.shell.ShowToastMsg(i18n.Tf("v54l.teams.leaving", name), true)
	safego.Go(func() {
		summary, err := leave.Leave(ctx)
		if app == nil {
			return
		}
		queueIfLive(ctx, app, func() {
			v.rebuild()
			if err != nil {
				v.shell.ShowToastMsg(i18n.Tf("v54l.teams.leave_failed", name, err.Error()), false)
				return
			}
			v.shell.ShowToastMsg(strings.Join(append([]string{i18n.Tf("v54l.teams.left", name)}, summary...), " · "), true)
		})
	})
}

// confirmRemove asks to take the team out of hub.toml (its projects are
// detached) and does it.
func (v *TeamsView) confirmRemove(team *config.TeamConfig) {
	teamID := team.ID
	name := v.teamName(teamID)
	detail := i18n.T("tui.teams.delete_detail")
	if v.projectsOf != nil {
		if projects := v.projectsOf(teamID); len(projects) > 0 {
			detail = i18n.Tf("v54u.teams.delete_detail_projects", len(projects), strings.Join(projects, ", "))
		}
	}
	v.shell.Confirm(i18n.Tf("tui.teams.confirm_delete", name), detail, true, func() {
		// Push undo state before mutation
		v.undoStack.Push(copyTeams(v.cfg.Teams))

		if v.onRemove != nil {
			// The host takes the team out of hub.toml and detaches its projects.
			detached, note, err := v.onRemove(teamID)
			if err != nil && len(detached) == 0 && v.teamByID(teamID) != nil {
				v.undoStack.Pop()
				v.shell.ShowToastMsg(i18n.Tf("tui.teams.save_failed", err.Error()), false)
				return
			}
			v.rebuild()
			msg := i18n.Tf("tui.teams.team_deleted", name)
			if len(detached) > 0 {
				msg = i18n.Tf("v54u.teams.team_deleted_detached", name, len(detached), strings.Join(detached, ", "))
			}
			if note != "" {
				msg += " " + note
			}
			if err != nil {
				v.shell.ShowToastMsg(i18n.Tf("v54u.teams.detach_failed", err.Error()), false)
				return
			}
			v.shell.ShowToastMsg(msg, true)
			return
		}

		// Remove from config
		newTeams := make([]config.TeamConfig, 0, len(v.cfg.Teams)-1)
		for _, t := range v.cfg.Teams {
			if t.ID != teamID {
				newTeams = append(newTeams, t)
			}
		}
		prevTeams := v.cfg.Teams
		v.cfg.Teams = newTeams

		// Auto-save
		if v.onUpdate != nil {
			if err := v.onUpdate(teamsPatch(prevTeams, newTeams)); err != nil {
				// Rollback in-memory state.
				if prev, ok := v.undoStack.Pop(); ok {
					v.cfg.Teams = prev
				}
				v.rebuild()
				v.shell.ShowToastMsg(i18n.Tf("tui.teams.save_failed", err.Error()), false)
				return
			}
		}

		v.rebuild()
		v.shell.ShowToastMsg(i18n.Tf("tui.teams.team_deleted", name), true)
	})
}

func (v *TeamsView) handleSync() {
	if team := v.currentTeam(); team != nil {
		v.syncTeam(team.ID)
	}
}

// handleOpen is p: the Team view of the team under the cursor. A solo space
// or an archived team has none.
func (v *TeamsView) handleOpen() {
	team := v.currentTeam()
	if team == nil || v.onOpen == nil {
		return
	}
	if team.Solo || !team.Enabled {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("v54u.teams.no_team_view", team.DisplayName()), false)
		}
		return
	}
	v.onOpen(team.ID)
}

// handleDefault is *: the team under the cursor becomes the default team.
func (v *TeamsView) handleDefault() {
	team := v.currentTeam()
	if team == nil || v.shell == nil {
		return
	}
	if v.isDefault(*team) {
		v.shell.ShowToastMsg(i18n.Tf("v54u.teams.already_default", team.DisplayName()), true)
		return
	}
	v.undoStack.Push(copyTeams(v.cfg.Teams))
	var err error
	if v.onDefault != nil {
		err = v.onDefault(team.ID)
	} else {
		err = v.saveTeams(func(c *config.Config) error { return c.SetDefaultTeam(team.ID) })
	}
	if err != nil {
		v.undoStack.Pop()
		v.shell.ShowToastMsg(err.Error(), false)
		return
	}
	v.rebuild()
	v.shell.ShowToastMsg(i18n.Tf("v54u.teams.default_set", team.DisplayName()), true)
}

// handleRename is e: the display name of the team under the cursor (empty:
// back to the ID).
func (v *TeamsView) handleRename() {
	team := v.currentTeam()
	if team == nil || v.shell == nil {
		return
	}
	id := team.ID
	v.shell.ShowInputModal(i18n.Tf("v54u.teams.rename_title", id), team.Name, func(name string) {
		name = strings.TrimSpace(name)
		v.undoStack.Push(copyTeams(v.cfg.Teams))
		err := v.saveTeams(func(c *config.Config) error {
			if t := c.FindTeam(id); t != nil {
				t.Name = name
			}
			return nil
		})
		if err != nil {
			v.undoStack.Pop()
			v.shell.ShowToastMsg(i18n.Tf("tui.teams.save_failed", err.Error()), false)
			return
		}
		v.rebuild()
		v.shell.ShowToastMsg(i18n.Tf("v54u.teams.renamed", v.teamName(id)), true)
	})
}

// handleArchive is x (archive) and R (restore): enabled = false or true. It
// never destroys anything and never asks.
func (v *TeamsView) handleArchive(archive bool) {
	team := v.currentTeam()
	if team == nil || v.shell == nil {
		return
	}
	name := team.DisplayName()
	if team.Enabled != archive {
		msg := "v54u.teams.already_active" // R on a team that is not archived
		if archive {
			msg = "v54u.teams.already_archived"
		}
		v.shell.ShowToastMsg(i18n.Tf(msg, name), false)
		return
	}
	v.undoStack.Push(copyTeams(v.cfg.Teams))
	var note string
	var err error
	switch {
	case v.onArchive != nil:
		note, err = v.onArchive(team.ID, archive)
	default:
		id := team.ID
		err = v.saveTeams(func(c *config.Config) error {
			if t := c.FindTeam(id); t != nil {
				t.Enabled = !archive
			}
			if archive {
				c.ClearDefaultTeam(id)
			}
			return nil
		})
	}
	if err != nil {
		v.undoStack.Pop()
		v.shell.ShowToastMsg(i18n.Tf("tui.teams.save_failed", err.Error()), false)
		return
	}
	v.rebuild()
	msg := i18n.Tf("v54u.teams.restored", name)
	if archive {
		msg = i18n.Tf("v54u.teams.archived", name, keys.Resume.Keys())
	}
	if note != "" {
		msg += " " + note
	}
	v.shell.ShowToastMsg(msg, true)
}

// saveTeams applies fn on the list the view shows, then on hub.toml through
// OnUpdate (the host then brings the shared config up to date with the file).
// Nothing changes when fn or the write fails.
func (v *TeamsView) saveTeams(fn func(*config.Config) error) error {
	prev := copyTeams(v.cfg.Teams)
	if err := fn(v.cfg); err != nil {
		v.cfg.Teams = prev
		return err
	}
	if v.onUpdate != nil {
		if err := v.onUpdate(fn); err != nil {
			v.cfg.Teams = prev
			return err
		}
	}
	return nil
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
	if v.onUpdate != nil {
		if err := v.onUpdate(teamsPatch(current, prev)); err != nil {
			// Restore pre-undo state and re-push the snapshot.
			v.cfg.Teams = current
			v.undoStack.Push(prev)
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.Tf("tui.teams.undo_failed", err.Error()), false)
			}
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

// teamRepoLabel is the team-state of a team: its remote, or « solo space »
// (local, no remote).
func teamRepoLabel(t config.TeamConfig) string {
	if t.Solo {
		return i18n.T("tui.solo.teams_label")
	}
	return textfmt.Clip(t.StateRepo, 40)
}

// teamsPatch changes a list of teams from before to after on the list that
// hub.toml holds now: the teams that are gone are removed, the ones that are
// new or changed take their new value, the others stay as they are.
func teamsPatch(before, after []config.TeamConfig) func(*config.Config) error {
	return func(c *config.Config) error {
		kept := make(map[string]bool, len(after))
		for _, t := range after {
			kept[t.ID] = true
		}
		old := make(map[string]config.TeamConfig, len(before))
		for _, t := range before {
			old[t.ID] = t
			if !kept[t.ID] {
				c.Teams = slices.DeleteFunc(c.Teams, func(x config.TeamConfig) bool { return x.ID == t.ID })
			}
		}
		for _, t := range after {
			if prev, had := old[t.ID]; had && reflect.DeepEqual(prev, t) {
				continue
			}
			if i := slices.IndexFunc(c.Teams, func(x config.TeamConfig) bool { return x.ID == t.ID }); i >= 0 {
				c.Teams[i] = t
			} else {
				c.Teams = append(c.Teams, t)
			}
		}
		return nil
	}
}
