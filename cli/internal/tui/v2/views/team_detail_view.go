package views

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// TrackerCredentialsError is the error of a tracker sync that has no
// credentials: the team detail then offers to type the token.
type TrackerCredentialsError struct {
	// Msg is the translated message (« credentials manquants… »).
	Msg string
	Err error
}

func (e *TrackerCredentialsError) Error() string {
	if e.Err == nil {
		return e.Msg
	}
	return e.Msg + ": " + e.Err.Error()
}

func (e *TrackerCredentialsError) Unwrap() error { return e.Err }

// discoverKey launches the tracker discovery wizard (T: tracker; y is
// accept in the key convention).
var discoverKey = keys.Own("team.discover", 'T', "tui.team.detail.hint_discover")

// newTeamKey opens the creation of a team from the detail that has none.
var newTeamKey = keys.Create

// teamLLMKey stores the LLM credential of the team for a provider.
var teamLLMKey = keys.Own("team.llm_key", 'K', "tui.team.llm_key.hint")

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

// TeamDetailViewConfig holds the external dependencies for TeamDetailView.
type TeamDetailViewConfig struct {
	GetMCPConfig func() config.MCPConfig
	// GetTrackerLocalConfig is the [tracker] of hub.toml: the default of every
	// team (and, without GetTeamTrackerConfig, the only personal settings).
	GetTrackerLocalConfig func() config.TrackerLocalConfig
	// GetTeamTrackerConfig is the section of this team in hub.toml
	// ([teams.tracker]): what the member sets for this team only. Nil: the
	// personal settings are the ones of GetTrackerLocalConfig.
	GetTeamTrackerConfig func() config.TrackerLocalConfig
	ResolveTeam          ResolveTeamFunc
	// CanEnforce says whether the member may change the settings that bind the
	// others (*_enforced, [limits.enforced], [governance]: for the leads). Nil:
	// they may; the team-state refuses the others anyway.
	CanEnforce func(ctx context.Context, teamID string) bool
	// GetTeamInfo is the entry of the team in hub.toml, for its identity lines.
	GetTeamInfo func(teamID string) TeamInfo
	// SetTeamField changes a field of the team in hub.toml (TeamField* keys):
	// the display name, the member id, the default mark. Written at once.
	SetTeamField func(teamID, field, value string) error
	// OnReconfigure opens the assistant on the team (« Reconfigurer »).
	OnReconfigure func(teamID string)
	// OnEditBoard opens the editor of the columns and of the mappings.
	OnEditBoard func()
	// OnNewTeam opens the window that creates or joins a team: offered by the
	// detail that has no team to show (the n key).
	OnNewTeam func()
	// SaveTeamConfig is called on the event loop and gives the function that
	// writes the config of the team to its team-state (a commit and a push),
	// which the view runs off the loop. Nil: nothing to save with.
	SaveTeamConfig func() TeamConfigWriter
	GetSecrets     func() tracker.SecretGetter
	GetHubConfig   func() *config.Config
	// UpdateHubConfig changes hub.toml: fn receives the file as it is on disk
	// and changes only the settings the view edited (the host then brings the
	// shared config up to date). The view never saves a whole config.
	UpdateHubConfig func(fn func(*config.Config) error) error
	ListProjects    func(ctx context.Context) []ProjectInfo
	SyncTracker     func(ctx context.Context) (*SyncTrackerResult, error)
	// CheckSecret tests whether a keychain key has a value stored.
	// Returns (present, maskedValue) — masked shows last 4 chars like "****7a3f".
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a secret value in the keychain.
	SetSecret func(ctx context.Context, key, value string) error
	// TeamProviderKey returns the keychain key of the team-level LLM
	// credential for a provider ("" when no team is active). Used by 'k'.
	TeamProviderKey func(provider string) string
	// GetMember returns the profile of the current member in the team-state
	// (nil when there is none). With SaveMemberField it shows the section
	// « Ma fiche de membre ».
	GetMember func(ctx context.Context, teamID string) *teamstate.Member
	// SaveMemberField writes one field of the profile of the current member
	// (a Member* key) to the team-state, in one commit.
	SaveMemberField func(ctx context.Context, teamID, field, value string) error
	// OnDiscoverTracker launches the tracker discovery wizard.
	// Called when the user presses T in the team detail view.
	OnDiscoverTracker func()
	// PromoteSolo turns the solo space teamID into a team pushed to remote
	// (« Passer en équipe »).
	PromoteSolo func(teamID, remote string)
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// TeamDetailView provides an interactive list for editing team-state config
// (tracker, notifications, collaboration) and local overrides.
// MCP and Models are accessible via sub-page links.
type TeamDetailView struct {
	app    *tview.Application
	list   *widgets.SectionedList
	page   *widgets.ListDetailPage
	shell  ShellAccess
	cfg    TeamDetailViewConfig
	loads  loadSlot // the reads of the team config: only the latest is applied
	syncs  loadSlot // the tracker sync: only the latest one is shown
	checks loadSlot // the connection test: only the latest one is shown

	teamCfg *teamstate.TeamConfig
	// baseTeam is teamCfg as it was read or last saved: the save writes the
	// difference, on top of the latest version of the team-state.
	baseTeam *teamstate.TeamConfig
	// cfgErr is why the team config could not be read: the view shows it and
	// nothing can be saved until a read succeeds.
	cfgErr   error
	member   teamstate.Member // the profile of the current member, as read or last saved
	localMCP config.MCPConfig
	localTrk config.TrackerLocalConfig // the personal tracker settings of this team ([teams.tracker])
	hubTrk   config.TrackerLocalConfig // the [tracker] default of hub.toml, under localTrk
	baseMCP  config.MCPConfig          // localMCP as loaded or last saved
	baseTrk  config.TrackerLocalConfig // localTrk as loaded or last saved
	fields   []configField
	// dirtyTeam: the shared config has edits that are not published (Ctrl+S);
	// dirtyLocal: hub.toml has edits that the autosave has not written yet.
	dirtyTeam  bool
	dirtyLocal bool
	// fileKeys are the settings config.toml writes (the others are defaults);
	// canEnforce says whether the member may change the settings that bind the
	// others; info is the entry of the team in hub.toml.
	fileKeys   map[string]bool
	canEnforce bool
	info       TeamInfo
	// autosave writes the personal settings (hub.toml) a moment after an edit.
	autosave *AutoSaver
	// section is the section the next render puts the cursor on (OpenSection).
	section string
	// leadKeys are the keys of the fields only a lead may change.
	leadKeys map[string]bool
	// kept is the unpublished draft of a visit that left without publishing
	// (a link to MCP, the models, the workflows): it is put back on the fresh
	// config at the next Mount.
	kept *keptDraft
	// localAgain: a personal edit came while a write ran; afterWrite runs when
	// the write is over (publish and leave).
	localAgain bool
	afterWrite func(ok bool)
	// noTeam is true when no team (nor solo space) could be resolved: the
	// view shows an empty state and nothing can be edited or saved.
	noTeam bool
	badge  saveBadge
	// saving is true while a save runs off the event loop; saveAgain, that an
	// edit asked for another one meanwhile.
	saving, saveAgain bool
	undoStack         *widgets.UndoStack[teamDetailSnapshot]
}

// teamDetailSnapshot holds the combined state for undo (team + local).
type teamDetailSnapshot struct {
	teamCfg  teamstate.TeamConfig
	localMCP config.MCPConfig
	localTrk config.TrackerLocalConfig
}

var _ View = (*TeamDetailView)(nil)

func NewTeamDetailView(cfg TeamDetailViewConfig) *TeamDetailView {
	return &TeamDetailView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[teamDetailSnapshot](10),
	}
}

func (v *TeamDetailView) SetShell(s ShellAccess) { v.shell = s }
func (v *TeamDetailView) ID() string             { return "team.detail" }
func (v *TeamDetailView) Title() string          { return i18n.T("tui.team.detail") }

func (v *TeamDetailView) StatusHints() string {
	if v.noTeam {
		if v.cfg.OnNewTeam != nil {
			return keys.Hints(newTeamKey.HintAs(i18n.T("v54c.hint.new_team")), keys.Back.Hint())
		}
		return keys.Back.Hint()
	}
	if v.cfgErr != nil {
		return keys.Hints(keys.Refresh.Hint(), keys.Back.Hint())
	}
	if v.solo() {
		return keys.Hints(keys.Navigate.Hint(), keys.Open.HintAs(i18n.T("tui.hints.edit")),
			keys.Save.HintAs(i18n.T("v54c.hint.publish")), cfgUndoKey.Hint())
	}
	return keys.Hints(keys.Navigate.Hint(), toggleHint(),
		keys.Open.HintAs(i18n.T("tui.hints.edit")), keys.Save.HintAs(i18n.T("v54c.hint.publish")),
		cfgSyncKey.Hint(), cfgTestKey.Hint(), cfgUndoKey.Hint(),
		keys.Refresh.Hint(), discoverKey.Hint(), teamLLMKey.Hint())
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Nothing carries over from a previous visit but the draft that was left
	// unpublished for the same team (v.kept, put back by setData): another
	// team's draft, or a stale flag, would show « unpublished changes » on
	// every exit.
	v.dirtyTeam, v.dirtyLocal = false, false
	v.undoStack.Clear()
	v.teamCfg, v.baseTeam, v.cfgErr, v.member, v.fields = nil, nil, nil, teamstate.Member{}, nil
	var team TeamResolution
	if v.cfg.ResolveTeam != nil {
		team = v.cfg.ResolveTeam()
	}
	if v.kept != nil && v.kept.teamID != team.TeamID {
		v.kept = nil
	}
	v.noTeam = !team.hasState()
	v.autosave = NewAutoSaver(SaveDelay, app, func() { v.write(false) })
	title := v.Title()
	if team.TeamID != "" {
		title += " · " + team.TeamID
	}

	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.page = widgets.NewListDetailPage(title, i18n.T("tui.catalog.detail"))
	v.badge.attach(v.page, title)
	v.page.SetMain(v.list)

	v.list.SetItemSelectedFunc(func(_ int, item widgets.SectionItem) {
		v.editSelected()
	})
	v.list.SetItemChangedFunc(func(_ int, item widgets.SectionItem) {
		showFieldItemDetail(v.page, v.fields, item, v.extraKeys()...)
	})

	if v.noTeam {
		v.page.Mount(content)
		v.showNoTeam()
		return
	}

	// Pull the team-state, read it off the event loop, then show the fields.
	v.page.Mount(content)
	v.page.SetDetail(stateText(stateLoading, ""))
	v.open(func() {
		v.buildFields()
		v.renderFields()
	})
}

// showNoTeam is the state of a view with no team to show.
func (v *TeamDetailView) showNoTeam() {
	v.list.SetItems([]widgets.SectionItem{{MainText: stateText(stateEmpty, i18n.T("v532k.team_detail.empty"))}})
	var d detailText
	d.title(i18n.T("v532k.team_detail.empty"))
	d.para(i18n.T("v532k.team_detail.empty_hint"))
	if v.cfg.OnNewTeam != nil {
		d.keys(keys.Pairs(newTeamKey, keys.Back)...)
	} else {
		d.keys(keys.Pairs(keys.Back)...)
	}
	v.page.SetDetail(d.String())
}

// extraKeys are the page keys listed in the detail pane.
func (v *TeamDetailView) extraKeys() []string {
	pairs := []string{keys.Save.Keys(), keys.Save.Label(), cfgUndoKey.Keys(), cfgUndoKey.Label()}
	if v.solo() {
		return pairs
	}
	return append(pairs,
		cfgSyncKey.Keys(), cfgSyncKey.Label(), cfgTestKey.Keys(), cfgTestKey.Label(),
		discoverKey.Keys(), discoverKey.Label(),
		teamLLMKey.Keys(), teamLLMKey.Label(), keys.Refresh.Keys(), keys.Refresh.Label())
}

func (v *TeamDetailView) Unmount() {
	v.loads.Invalidate() // drop the loads in flight
	v.syncs.Invalidate()
	v.checks.Invalidate()
	// what is personal is written now; what is shared stays a draft of this
	// view, put back on the next visit: leaving for MCP, the models or the
	// workflows does not throw the edits away.
	if v.autosave != nil {
		v.autosave.Flush()
		v.autosave = nil
	}
	v.kept = nil
	if v.dirtyTeam && v.teamCfg != nil && v.baseTeam != nil && v.cfg.ResolveTeam != nil {
		v.kept = &keptDraft{teamID: v.cfg.ResolveTeam().TeamID, base: v.baseTeam, draft: teamstate.CloneConfig(v.teamCfg)}
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("v54c.save.draft_kept", keys.Save.Keys()), false)
		}
	}
	v.app = nil
	v.list = nil
	v.page = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling — navigation is handled by SectionedList
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list != nil && v.noTeam && v.cfg.OnNewTeam != nil && newTeamKey.Matches(event) {
		v.cfg.OnNewTeam()
		return nil
	}
	if v.list == nil || v.noTeam {
		return event
	}
	if v.cfgErr != nil { // nothing to edit: only a new read gets out of it
		switch {
		case keys.Refresh.Matches(event):
			v.reload()
			return nil
		case keys.Save.Matches(event):
			v.save() // says why it is refused
			return nil
		}
		return event
	}
	switch {
	case keys.Open.Matches(event), keys.Edit.Matches(event):
		v.editSelected()
		return nil
	case keys.Refresh.Matches(event):
		confirmDiscard(v.shell, v.dirtyTeam || v.dirtyLocal, v.reload)
		return nil
	case cfgToggleKey.Matches(event):
		v.toggleSelected()
		return nil
	case keys.Save.Matches(event):
		v.save()
		return nil
	case cfgSyncKey.Matches(event):
		if !v.solo() {
			v.syncTracker()
		}
		return nil
	case cfgTestKey.Matches(event):
		if !v.solo() {
			v.testConnection()
		}
		return nil
	case cfgUndoKey.Matches(event):
		v.undo()
		return nil
	case teamLLMKey.Matches(event):
		v.setTeamProviderKey()
		return nil
	case discoverKey.Matches(event):
		if v.cfg.OnDiscoverTracker != nil && !v.solo() {
			v.cfg.OnDiscoverTracker()
		}
		return nil
	}
	return event
}

// reload pulls the team-state and reloads the fields; unsaved changes are
// dropped (r asks first, confirmDiscard).
func (v *TeamDetailView) reload() {
	if v.noTeam || !v.cfg.ResolveTeam().hasState() {
		return
	}
	v.kept = nil
	v.open(func() {
		v.buildFields()
		v.renderFields()
		v.dirtyTeam = false
		v.dirtyLocal = false
	})
}

// showConfigError is the state of a view whose team config could not be read:
// the reason, no field, nothing to save.
func (v *TeamDetailView) showConfigError() {
	v.fields = nil
	v.dirtyTeam, v.dirtyLocal = false, false
	v.undoStack.Clear()
	if v.list == nil {
		return
	}
	v.list.SetItems([]widgets.SectionItem{{MainText: stateText(stateError, unreadableConfigText(v.cfgErr))}})
	v.page.SetDetail(unreadableDetail(v.cfgErr))
	v.badge.set(saveStatusFailed)
	refreshHints(v.shell)
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

// teamDetailRead is what the view reads of the team-state, off the event loop.
type teamDetailRead struct {
	teamCfg    *teamstate.TeamConfig
	cfgErr     error // why teamCfg could not be read
	member     *teamstate.Member
	keys       map[string]bool // the settings config.toml writes
	canEnforce bool
}

// open reads what the view shows: the personal settings (taken on the event
// loop, which owns the config), the team config and the profile of the member
// (pulled and read off the loop). then runs on the loop once they are in; the
// latest request wins, Unmount drops the one in flight.
func (v *TeamDetailView) open(then func()) {
	localMCP, localTrk, hubTrk := v.cfg.GetMCPConfig(), v.cfg.GetTrackerLocalConfig(), config.TrackerLocalConfig{}
	if v.cfg.GetTeamTrackerConfig != nil {
		hubTrk, localTrk = localTrk, v.cfg.GetTeamTrackerConfig()
	}
	tc := v.cfg.ResolveTeam()
	var info TeamInfo
	if v.cfg.GetTeamInfo != nil {
		info = v.cfg.GetTeamInfo(tc.TeamID)
	}
	fetch := func(ctx context.Context) (teamDetailRead, error) {
		var r teamDetailRead
		r.teamCfg, r.cfgErr = readTeamConfig(ctx, tc)
		if v.cfg.GetMember != nil {
			r.member = v.cfg.GetMember(ctx, tc.TeamID)
		}
		if r.cfgErr == nil && tc.hasState() {
			r.keys, _ = teamstate.NewRepo(tc.StateRepo, tc.StatePath).ConfigKeysSet()
		}
		r.canEnforce = v.cfg.CanEnforce == nil || v.cfg.CanEnforce(ctx, tc.TeamID)
		return r, nil
	}
	apply := func(r teamDetailRead, err error) {
		if err != nil && r.cfgErr == nil {
			r.cfgErr = err
		}
		v.localMCP, v.localTrk, v.hubTrk, v.info = localMCP, localTrk, hubTrk, info
		v.baseMCP, v.baseTrk = localMCP, localTrk
		v.fileKeys, v.canEnforce = r.keys, r.canEnforce
		v.setData(r)
		if v.cfgErr != nil {
			v.showConfigError()
			return
		}
		then()
	}
	if tc.hasState() {
		syncLoad(v.app, shellContext(v.shell), &v.loads, teamstate.NewRepo(tc.StateRepo, tc.StatePath), v.shell, fetch, apply)
		return
	}
	load(v.app, shellContext(v.shell), &v.loads, fetch, apply)
}

// loadData reads the data in place, on the calling goroutine (tests only: the
// view reads it with open).
func (v *TeamDetailView) loadData() { v.open(func() {}) }

// setData takes what was read: the team config and the profile of the member.
// A team config that could not be read is not replaced by an empty one: the
// view keeps the error (cfgErr) and nothing of it, so nothing can be saved.
func (v *TeamDetailView) setData(r teamDetailRead) {
	v.member = teamstate.Member{}
	if r.member != nil {
		v.member = *r.member
	}
	v.cfgErr = r.cfgErr
	if v.cfgErr != nil {
		v.teamCfg, v.baseTeam = nil, nil
		return
	}
	if r.teamCfg != nil {
		v.teamCfg = r.teamCfg
	}
	// Ensure non-nil teamCfg for editing
	if v.teamCfg == nil {
		v.teamCfg = &teamstate.TeamConfig{}
	}
	if v.teamCfg.MCP == nil {
		v.teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
	}
	if v.teamCfg.Tracker.Projects == nil { //nolint:staticcheck // backward compat: deprecated field
		v.teamCfg.Tracker.Projects = make(map[string]string) //nolint:staticcheck // backward compat: deprecated field
	}
	if v.teamCfg.Models.Families == nil {
		v.teamCfg.Models.Families = make(map[string]string)
	}
	if v.teamCfg.Models.Agents == nil {
		v.teamCfg.Models.Agents = make(map[string]string)
	}
	v.baseTeam = teamstate.CloneConfig(v.teamCfg)
	v.putBackDraft()
}

// ─────────────────────────────────────────────────────────────────────────────
// Build config lines
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

// syncBadge shows whether the changes are published to the team-state.
func (v *TeamDetailView) syncBadge() {
	if v.saving {
		v.badge.set(saveStatusPending) // the write runs: not published yet
		return
	}
	v.badge.settle(v.dirtyTeam)
}

func (v *TeamDetailView) renderFields() {
	v.syncBadge()
	if v.list == nil {
		return
	}
	savedItem, _, hasCursor := v.list.CurrentItem() // the index of the item, not the row (spacers)

	v.list.SetItems(configItems(v.fields, 28))
	switch idx := v.sectionItem(); {
	case idx >= 0:
		v.list.SelectIndex(idx)
		v.section = ""
	case hasCursor:
		v.list.SelectIndex(savedItem)
	}
	showCurrentFieldDetail(v.page, v.list, v.fields, v.extraKeys()...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing (delegates to shared editConfigField / toggleConfigField)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) selectedField() (*configField, int, bool) {
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return nil, -1, false
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return nil, -1, false
	}
	f := &v.fields[ref]
	if !isSelectable(f.Kind) || f.Get == nil {
		return nil, -1, false
	}
	return f, ref, true
}

func (v *TeamDetailView) toggleSelected() {
	f, _, ok := v.selectedField()
	if !ok {
		return
	}
	if f.Locked != nil && f.Locked() {
		v.lockedToast(f)
		return
	}
	if !isToggleable(f.Kind) {
		return
	}
	if f.Immediate {
		toggleConfigField(f)
		return
	}
	v.pushUndo()
	toggleConfigField(f)
	v.renderFields()
}

// lockedToast says why a field cannot be edited: reserved to the leads, or
// imposed by the team.
func (v *TeamDetailView) lockedToast(f *configField) {
	if v.shell == nil {
		return
	}
	if v.leadKeys[f.Key] {
		v.shell.ShowToastMsg(i18n.T("v54c.governance.lead_only"), false)
		return
	}
	v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
}

func (v *TeamDetailView) editSelected() {
	f, _, ok := v.selectedField()
	if !ok || v.shell == nil {
		return
	}
	if f.Locked != nil && f.Locked() {
		v.lockedToast(f)
		return
	}
	if !isEditable(f.Kind) {
		return
	}
	if f.Kind == CfgFieldAction {
		if f.Set != nil {
			f.Set("")
		}
		return
	}

	// The token of the tracker: key, server and token in turn
	if f.Key == fieldTrackerToken {
		v.promptTokenSetup()
		return
	}
	// Written to hub.toml (or to the profile in the team-state) at once,
	// outside the save/undo of the other fields
	if f.Key == fieldTeamGitLabURL || isMemberField(f.Key) || f.Immediate {
		editConfigField(v.shell, f, v.renderFields)
		return
	}
	// The webhook is a secret: typed in a window that hides it, shown masked.
	if f.Key == fieldWebhookURL {
		v.promptWebhook(f)
		return
	}

	// Special handling for password fields (token setup flow)
	if f.Kind == CfgFieldPassword {
		v.promptTokenSetup()
		return
	}

	// Special handling for links
	if f.Kind == CfgFieldLink {
		editConfigField(v.shell, f, nil)
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.renderFields()
		// a personal setting is written by itself a moment later
		if f.Scope == ScopeTeamLocal {
			v.touchLocal()
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) pushUndo() {
	if v.teamCfg == nil {
		return
	}
	v.undoStack.Push(teamDetailSnapshot{
		teamCfg:  deepCopyTeamConfig(v.teamCfg),
		localMCP: v.localMCP,
		localTrk: v.localTrk,
	})
}

func (v *TeamDetailView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	if v.teamCfg != nil {
		*v.teamCfg = prev.teamCfg
	}
	v.localMCP = prev.localMCP
	v.localTrk = prev.localTrk
	// the key of the tracker is written to hub.toml as soon as it is set: an
	// older snapshot must not bring back the declaration it replaced
	v.localTrk.SetTokenDecl(v.baseTrk.TokenDecl())
	v.dirtyTeam = v.teamCfg != nil && v.baseTeam != nil && !teamstate.DiffConfig(v.baseTeam, v.teamCfg).Empty()
	v.touchLocal() // the personal settings are put back by the autosave
	v.buildFields()
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Persistence
// ─────────────────────────────────────────────────────────────────────────────

// TeamConfigWriter writes the changes between base (the config as the view
// read it) and edited to the config of the team, on top of the latest version
// of its team-state: what a colleague changed meanwhile is kept, and a setting
// both changed is refused with a teamstate.ErrConfigConflict.
type TeamConfigWriter func(ctx context.Context, base, edited *teamstate.TeamConfig) error

// save writes what was edited: the team config to the team-state and the
// local settings to hub.toml, off the event loop, one save at a time (the
// edits made meanwhile wait for the next one). What the write needs from the
// view and the app is taken here, on the loop.
func (v *TeamDetailView) save() { v.write(true) }

// write is save; without includeTeam only the personal settings are written
// (the autosave of hub.toml): the draft of the shared config is published by
// Ctrl+S alone.
func (v *TeamDetailView) write(includeTeam bool) {
	if v.shell == nil || v.noTeam {
		return
	}
	if v.cfgErr != nil || v.teamCfg == nil { // never write what was not read
		if includeTeam {
			v.shell.ShowToastMsg(saveRefusedText(v.cfgErr), false)
		}
		return
	}
	if v.saving {
		if includeTeam {
			v.saveAgain = true
		} else {
			v.localAgain = true
		}
		return
	}
	teamID := ""
	if v.cfg.ResolveTeam != nil {
		teamID = v.cfg.ResolveTeam().TeamID
	}
	if v.autosave != nil && includeTeam {
		v.autosave.Cancel() // written now
	}
	var (
		writeTeam TeamConfigWriter
		teamCopy  teamstate.TeamConfig
		baseCopy  *teamstate.TeamConfig
		patchMCP  func(*config.MCPConfig)
		patchTrk  func(*config.TrackerLocalConfig)
		newMCP    = v.localMCP
		newTrk    = v.localTrk
	)
	saveTeam := includeTeam && v.dirtyTeam && v.cfg.SaveTeamConfig != nil
	var publishedKeys []string
	if saveTeam {
		if writeTeam = v.cfg.SaveTeamConfig(); writeTeam == nil {
			saveTeam = false
		} else {
			teamCopy = deepCopyTeamConfig(v.teamCfg)
			baseCopy = teamstate.CloneConfig(v.baseTeam)
			publishedKeys = teamstate.DiffConfig(baseCopy, &teamCopy).Keys()
		}
	}
	saveLocal := v.dirtyLocal
	writeLocal := saveLocal && v.cfg.UpdateHubConfig != nil
	if writeLocal {
		patchMCP, patchTrk = config.Patch(v.baseMCP, v.localMCP), config.Patch(v.baseTrk, v.localTrk)
	}
	// the flags go down now: an edit made during the write sets them again
	if saveTeam {
		v.dirtyTeam = false
	}
	if saveLocal {
		v.dirtyLocal = false
	}

	var teamDone, localDone bool
	var teamErr, localErr error
	v.saving = true
	v.badge.set(saveStatusPending) // the write really runs: saved is said when it ends
	persist(v.app, func(ctx context.Context) error {
		if saveTeam {
			teamErr = writeTeam(ctx, baseCopy, &teamCopy)
			if teamErr != nil {
				return teamErr
			}
			teamDone = true
		}
		if writeLocal {
			localErr = v.cfg.UpdateHubConfig(func(c *config.Config) error {
				patchMCP(&c.MCP)
				patchTrk(c.TrackerForWrite(teamID))
				return nil
			})
			if localErr != nil {
				return localErr
			}
			localDone = true
		}
		return nil
	}, func(error) {
		v.saving = false
		again, localAgain, after := v.saveAgain, v.localAgain, v.afterWrite
		v.saveAgain, v.localAgain, v.afterWrite = false, false, nil
		var saved []string
		switch {
		case teamErr != nil:
			v.dirtyTeam = true
			v.dirtyLocal = v.dirtyLocal || saveLocal // not tried
			v.shell.ShowToastMsg(teamSaveErrorText(teamErr), false)
			v.badge.set(saveStatusFailed)
			confirmReloadOnConflict(v.shell, teamErr, v.reload)
			if after != nil {
				after(false)
			}
			return
		case localErr != nil:
			v.dirtyLocal = true
			v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.save_local_error", localErr.Error()), false)
			v.badge.set(saveStatusFailed)
			if after != nil {
				after(false)
			}
			return
		}
		if teamDone {
			v.baseTeam = teamstate.CloneConfig(&teamCopy)
			v.undoStack.Clear() // what was published is no longer an edit to take back
			if v.fileKeys == nil {
				v.fileKeys = map[string]bool{}
			}
			for _, k := range publishedKeys {
				v.fileKeys[k] = true
			}
			saved = append(saved, i18n.T("tui.team.detail.saved_team"))
		}
		if saveLocal {
			if localDone {
				v.baseMCP, v.baseTrk = newMCP, newTrk
			}
			saved = append(saved, i18n.T("tui.team.detail.saved_local"))
		}
		switch {
		case !includeTeam: // the autosave of hub.toml is silent
		case len(saved) == 0:
			v.shell.ShowToastMsg(i18n.T("tui.team.detail.nothing_to_save"), false)
		default:
			v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.saved", strings.Join(saved, " + ")), true)
		}
		v.syncBadge()
		if again {
			v.save()
		} else if localAgain {
			v.write(false)
		}
		if after != nil {
			after(true)
		}
		v.renderFields()
	})
}

// teamSaveErrorText is the toast of a publication that failed: a refusal for
// want of the lead role says so, the rest is the error.
func teamSaveErrorText(err error) string {
	if nl, ok := teamstate.IsNotLead(err); ok {
		return i18n.Tf("v54c.save.not_lead", strings.Join(nl.Keys, ", "))
	}
	return i18n.Tf("tui.team.detail.save_team_error", err.Error())
}

// touchLocal records an edit of the personal settings: written by itself a
// moment later.
func (v *TeamDetailView) touchLocal() {
	v.dirtyLocal = true
	if v.autosave != nil {
		v.autosave.Schedule()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions (sync + test + token setup)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) syncTracker() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.cfg.SyncTracker == nil {
		v.shell.ShowToastMsg(i18n.T("tui.team.detail.sync_unavailable"), false)
		return
	}

	v.shell.ShowToastMsg(i18n.T("tui.team.sync.in_progress"), true)

	load(v.app, v.shell.Context(), &v.syncs, v.cfg.SyncTracker, func(result *SyncTrackerResult, err error) {
		if v.shell == nil {
			return
		}
		if err != nil {
			var credErr *TrackerCredentialsError
			if errors.As(err, &credErr) {
				body := i18n.T("tui.team.detail.token_missing_body")
				if credErr.Err != nil {
					// the server, the sources read and how to declare the key
					body = credErr.Err.Error() + "\n\n" + body
				}
				v.shell.ShowScrollableModal(i18n.T("tui.team.detail.token_missing_title"), body,
					[]ModalAction{
						{Label: i18n.T("tui.team.detail.token_configure"), Callback: func() {
							v.promptTokenSetup()
						}},
						{Label: i18n.T("tui.team.detail.later"), Callback: func() {}},
					})
			} else {
				v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.sync_failed", err.Error()), false)
			}
			return
		}
		content := formatSyncTrackerResultView(result)
		v.shell.ShowScrollableModal(i18n.T("tui.team.sync.result_title"), content, []ModalAction{
			{Label: i18n.T("tui.team.detail.ok"), Callback: func() {
				v.open(func() {
					v.buildFields()
					v.renderFields()
				})
			}},
		})
	})
}

// connError says at which step the test of the connection failed.
type connError struct {
	key string // i18n key of the message
	err error
}

func (e *connError) Error() string { return e.err.Error() }
func (e *connError) Unwrap() error { return e.err }

func (v *TeamDetailView) testConnection() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.teamCfg == nil || v.teamCfg.Tracker.Type == "" {
		v.shell.ShowToastMsg(i18n.T("tui.team.detail.tracker_not_configured"), false)
		return
	}

	v.shell.ShowToastMsg(i18n.T("tui.team.detail.testing"), true)

	// What the test needs of the view is taken here, on the event loop: the
	// effective tracker config and the sources of the credentials.
	effTracker := v.effectiveTracker()
	src := tracker.NewCredentialSource(effTracker, v.cfg.GetSecrets())
	// Same GitLab token cascade as the sync: the team and the MCP key count,
	// each for its own server.
	teamID := ""
	if v.cfg.ResolveTeam != nil {
		tc := v.cfg.ResolveTeam()
		teamID = tc.TeamID
		src.TeamID = tc.TeamID
		src.TeamServer = tracker.ServerOf(firstNonEmpty(tc.GitLabURL, tc.StateRepo))
	}
	if hub := v.cfg.GetHubConfig(); hub != nil {
		src.MCPTokenKey = hub.MCP.Gitlab.Token
		var shared *teamstate.SharedMCPConfig
		if s, ok := v.teamCfg.MCP["gitlab"]; ok {
			shared = &s
		}
		src.MCPServer, src.MCPImplicit = tracker.MCPServerOf(tracker.MCPGitLabURL(hub.MCP.Gitlab, shared, nil, teamID))
	}
	trackerType := tracker.Type(v.teamCfg.Tracker.Type)

	load(v.app, v.shell.Context(), &v.checks, func(ctx context.Context) (string, error) {
		cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err != nil {
			return "", &connError{"tui.team.detail.credentials_unavailable", err}
		}
		t, err := tracker.New(cfg)
		if err != nil {
			return "", &connError{"tui.team.detail.init_failed", err}
		}
		username, err := t.TestConnection(ctx)
		if err != nil {
			return "", &connError{"tui.team.detail.connection_failed", err}
		}
		return username, nil
	}, func(username string, err error) {
		if v.shell == nil {
			return
		}
		if err != nil {
			key := "tui.team.detail.connection_failed"
			var ce *connError
			if errors.As(err, &ce) {
				key = ce.key
			}
			v.shell.ShowToastMsg(i18n.Tf(key, err.Error()), false)
			return
		}
		v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.connected_as", username), true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func tdBoolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func ptrBoolToTriState(p *bool) string {
	if p == nil {
		return "" // not configured
	}
	if *p {
		return "true"
	}
	return "false"
}

func triStateToPtrBool(val string) *bool {
	switch val {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil // not configured
	}
}

func formatSyncTrackerResultView(r *SyncTrackerResult) string {
	if r == nil {
		return i18n.T("tui.team.sync.no_result")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.claims_created", r.ClaimsCreated))
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.claims_updated", r.ClaimsUpdated))
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.labels_pushed", r.LabelsPushed))
	if len(r.Projects) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.projects_header") + "\n")
		for _, p := range r.Projects {
			fmt.Fprintf(&sb, "  %s\n", p)
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.warnings_header") + "\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "  %s %s\n", theme.SymWarning, w)
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.errors_header") + "\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "  %s %s\n", theme.SymFailure, e)
		}
	}
	return sb.String()
}

// setTeamProviderKey stores the team-level LLM credential of a provider in
// the keychain (cascade: project key → team key → hub key).
func (v *TeamDetailView) setTeamProviderKey() {
	if v.shell == nil || v.cfg.TeamProviderKey == nil || v.cfg.SetSecret == nil {
		return
	}
	options := []SelectOption{
		{Label: i18n.T("tui.team.llm_key.bedrock"), Value: "bedrock"},
		{Label: "Anthropic", Value: "anthropic"},
		{Label: "OpenRouter", Value: "openrouter"},
	}
	v.shell.ShowSelectModal(i18n.T("tui.team.llm_key.choose_provider"), options, "bedrock", func(prov string) {
		key := v.cfg.TeamProviderKey(prov)
		if key == "" {
			v.shell.ShowToastMsg(i18n.T("tui.team.llm_key.no_team"), false)
			return
		}
		v.shell.ShowPasswordModal(i18n.Tf("tui.team.llm_key.prompt", prov), func(value string) {
			if value == "" {
				return
			}
			persist(v.app, func(ctx context.Context) error { return v.cfg.SetSecret(ctx, key, value) }, func(err error) {
				if err != nil {
					v.shell.ShowToastMsg(err.Error(), false)
					return
				}
				v.shell.ShowToastMsg(i18n.Tf("tui.team.llm_key.saved", prov), true)
			})
		})
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Workflows: governance and solo space
// ─────────────────────────────────────────────────────────────────────────────

// GovernanceLabel describes a workflow publication policy.
func GovernanceLabel(policy string) string {
	if policy == "" || policy == teamstate.GovernancePublishAnyMember {
		return i18n.T("tui.team.governance.any_member")
	}
	return i18n.Tf("tui.team.governance.unsupported", policy)
}

// governanceValue is the policy alone (the field label says « publication »).
func governanceValue(policy string) string {
	if policy == "" || policy == teamstate.GovernancePublishAnyMember {
		return i18n.T("tui.team.governance.value_any_member")
	}
	return i18n.Tf("tui.team.governance.value_unsupported", policy)
}

func (v *TeamDetailView) buildWorkflowFields() {
	tc := v.cfg.ResolveTeam()
	if !tc.hasState() {
		return
	}
	v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.team.governance.section")})
	v.fields = append(v.fields, configField{
		Key: "governance.publish", Kind: CfgFieldReadonly, Label: i18n.T("tui.team.governance.label"),
		Description: i18n.T("tui.team.governance.desc"), Scope: ScopeTeamShared,
		Get: func() string { return governanceValue(v.teamCfg.Governance.PublishPolicy()) },
	})
	if !tc.Solo {
		return
	}
	v.fields = append(v.fields, configField{
		Key: "solo", Kind: CfgFieldReadonly, Label: i18n.T("tui.solo.detail.label"),
		Description: i18n.T("tui.solo.detail.desc"), Scope: ScopeTeamShared,
		Get: func() string { return i18n.Tf("tui.solo.detail.value", tc.StatePath) },
	})
	if v.cfg.PromoteSolo == nil {
		return
	}
	v.fields = append(v.fields, configField{
		Key: "solo.promote", Kind: CfgFieldAction, Label: i18n.T("tui.solo.promote.label"),
		Description: i18n.T("tui.solo.promote.desc"),
		Get:         func() string { return "→" },
		Set:         func(string) { v.askPromote(tc.TeamID) },
	})
}

// askPromote asks for the remote of the new team, then confirms.
func (v *TeamDetailView) askPromote(teamID string) {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal(i18n.T("tui.solo.promote.remote"), "", func(remote string) {
		remote = strings.TrimSpace(remote)
		if remote == "" {
			return
		}
		v.shell.ShowScrollableModal(i18n.T("tui.solo.promote.label"), i18n.Tf("tui.solo.promote.confirm", teamID, remote), []ModalAction{
			{Label: i18n.T("tui.solo.promote.do"), Callback: func() { v.cfg.PromoteSolo(teamID, remote) }},
			{Label: i18n.T("tui.catalog.edit.cancel"), Callback: func() {}, Separator: true},
		})
	})
}
