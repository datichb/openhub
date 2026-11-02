package views

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

// TeamMCPViewConfig holds external dependencies for TeamMCPView.
type TeamMCPViewConfig struct {
	// ResolveTeam returns the effective team resolution (state repo path, etc.).
	ResolveTeam ResolveTeamFunc
	// GetMCPConfig returns the local hub.toml MCP configuration.
	GetMCPConfig func() config.MCPConfig
	// SaveTeamConfig is called on the event loop and gives the function that
	// writes the team-state config to the team-state repo, which the view
	// runs off the loop. Nil: nothing to save with.
	SaveTeamConfig func() TeamConfigWriter
	// SaveLocalMCP persists a local hub.toml MCP override key/value.
	SaveLocalMCP func(key, value string) error
	// UpdateHubConfig changes hub.toml: fn receives the file as it is on disk
	// and changes only the settings the view edited (the host then brings the
	// shared config up to date). The view never keeps nor saves a whole config.
	UpdateHubConfig func(fn func(*config.Config) error) error
	// CheckSecret tests whether a keychain key has a stored value.
	// Returns (true, masked) if present, (false, "") if absent.
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a token value in the system keychain.
	SetSecret func(ctx context.Context, key, value string) error
	// TestMCP prepares the connection test of an MCP service. It runs on the
	// event loop and takes what it needs from the shared configuration; the
	// function it returns makes the calls to the server (off the loop) and
	// returns what the server answered, or an error that names the server,
	// the source and the key of the token. It returns nil for a service with
	// no connection test.
	TestMCP func(service string) func(ctx context.Context) (string, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// TeamMCPView provides a standalone sub-page for editing team-level and local
// MCP service configuration, extracted from TeamDetailView.
type TeamMCPView struct {
	app   *tview.Application
	list  *widgets.SectionedList
	page  *widgets.ListDetailPage
	shell ShellAccess
	cfg   TeamMCPViewConfig
	loads loadSlot // the reads of the team config: only the latest is applied
	tests loadSlot // connection tests: only the latest one is shown

	teamCfg    *teamstate.TeamConfig
	localMCP   config.MCPConfig
	baseMCP    config.MCPConfig // localMCP as loaded or last saved: the save writes the difference
	fields     []configField
	dirtyTeam  bool
	dirtyLocal bool
	badge      saveBadge
	// saving is true while a save runs off the event loop; saveAgain, that an
	// edit asked for another one meanwhile.
	saving, saveAgain bool
	undoStack         *widgets.UndoStack[teamMCPSnapshot]
}

// teamMCPSnapshot holds combined state for undo.
type teamMCPSnapshot struct {
	teamCfg  teamstate.TeamConfig
	localMCP config.MCPConfig
}

var _ View = (*TeamMCPView)(nil)
var _ CommandProvider = (*TeamMCPView)(nil)

// NewTeamMCPView creates the MCP services configuration view.
func NewTeamMCPView(cfg TeamMCPViewConfig) *TeamMCPView {
	return &TeamMCPView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[teamMCPSnapshot](10),
	}
}

// SetShell provides shell access for modals/toasts.
func (v *TeamMCPView) SetShell(s ShellAccess) { v.shell = s }

func (v *TeamMCPView) ID() string    { return "team.mcp" }
func (v *TeamMCPView) Title() string { return i18n.T("tui.config.section.mcp_services") }
func (v *TeamMCPView) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), "{/} "+i18n.T("tui.hints.sections"),
		toggleHint(),
		keys.Open.HintAs(i18n.T("tui.hints.edit")), keys.Save.Hint(),
		cfgUndoKey.Hint(), cfgTestKey.Hint(), keys.Refresh.Hint())
}

// ─────────────────────────────────────────────────────────────────────────────
// Mount / Unmount
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.dirtyTeam = false
	v.dirtyLocal = false

	// Show loading placeholder immediately
	loading := stateView(stateLoading, i18n.T("tui.settings.loading"))
	content.AddItem(loading, 0, 1, true)

	// Pull the team-state, read its config off the event loop, then build the
	// list on it.
	v.open(func() {
		v.list = widgets.NewSectionedList()
		v.list.SetApp(app)
		v.page = newListPage(v.Title(), v.list)
		v.badge.attach(v.page, v.Title())

		v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
			v.onItemSelected(index, item)
		})
		v.list.SetItemChangedFunc(func(_ int, item widgets.SectionItem) {
			showFieldItemDetail(v.page, v.fields, item, teamMCPExtraKeys()...)
		})

		v.buildFields()
		v.renderFields()

		v.page.Mount(content)
		app.SetFocus(v.list)
	})
}

// open reads what the view shows: the personal MCP settings (taken on the
// event loop, which owns the config) and the team config, pulled and read off
// the loop. then runs on the loop once they are in; the latest request wins,
// Unmount drops the one in flight.
func (v *TeamMCPView) open(then func()) {
	local := v.cfg.GetMCPConfig()
	tc := v.cfg.ResolveTeam()
	before := v.edited()
	fetch := func(context.Context) (*teamstate.TeamConfig, error) {
		if !tc.Enabled {
			return nil, nil // no team: an empty config
		}
		return readTeamConfig(tc), nil
	}
	take := func(cfg *teamstate.TeamConfig) {
		v.localMCP, v.baseMCP = local, local
		v.setTeamConfig(cfg)
		then()
	}
	apply := func(cfg *teamstate.TeamConfig, _ error) {
		if v.app == nil {
			return
		}
		if reflect.DeepEqual(v.edited(), before) {
			take(cfg)
			return
		}
		// An edit was made while the read ran (a reload is asked, the user
		// edits before it ends): taking the file would drop it unseen.
		if v.shell == nil {
			return
		}
		v.shell.Confirm(i18n.T("tui.config.discard_title"), i18n.T("tui.config.discard_detail"), true, func() { take(cfg) })
	}
	if tc.Enabled {
		syncLoad(v.app, shellContext(v.shell), &v.loads, teamstate.NewRepo(tc.StateRepo, tc.StatePath), v.shell, fetch, apply)
		return
	}
	load(v.app, shellContext(v.shell), &v.loads, fetch, apply)
}

// edited is the content the user edits (the team config and the personal MCP
// settings), to tell whether an edit was made between two moments.
func (v *TeamMCPView) edited() teamMCPSnapshot {
	snap := teamMCPSnapshot{localMCP: v.localMCP}
	if v.teamCfg != nil {
		snap.teamCfg = deepCopyTeamConfig(v.teamCfg)
	}
	return snap
}

func (v *TeamMCPView) Unmount() {
	v.loads.Invalidate() // drop the load in flight
	v.tests.Invalidate()
	if (v.dirtyTeam || v.dirtyLocal) && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.save.unpublished_left"), false)
	}
	v.app = nil
	v.list = nil
	v.page = nil
}

// teamMCPExtraKeys are the page keys listed in the detail pane.
func teamMCPExtraKeys() []string {
	return []string{cfgTestKey.Keys(), cfgTestKey.Label(), keys.Save.Keys(), keys.Save.Label(), cfgUndoKey.Keys(), cfgUndoKey.Label(), keys.Refresh.Keys(), keys.Refresh.Label()}
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}
	switch {
	case keys.Open.Matches(event):
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.onItemSelected(idx, item)
		}
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
	case cfgUndoKey.Matches(event):
		v.undo()
		return nil
	case cfgTestKey.Matches(event):
		v.testConnection()
		return nil
	}
	return event
}

// reload pulls the team-state and reloads the fields; unsaved changes are
// dropped (r asks first, confirmDiscard).
func (v *TeamMCPView) reload() {
	tc := v.cfg.ResolveTeam()
	if !tc.Enabled {
		return
	}
	v.refreshData()
}

// refreshData reads the data again and shows it; the changes not saved are
// dropped.
func (v *TeamMCPView) refreshData() {
	v.open(func() {
		v.buildFields()
		v.renderFields()
		v.dirtyTeam = false
		v.dirtyLocal = false
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Context commands (omnibar)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "team.mcp.save", Label: i18n.T("tui.hints.save"),
			Aliases:     []string{"save", "write", "sauvegarder"},
			Description: i18n.T("tui.team_mcp.cmd_save_desc"), Category: "MCP",
			Action: v.save,
		},
		{
			ID: "team.mcp.reload", Label: i18n.T("tui.hints.refresh"),
			Aliases:     []string{"refresh", "reload", "recharger"},
			Description: i18n.T("tui.team_mcp.cmd_reload_desc"), Category: "MCP",
			Action: v.refreshData,
		},
		{
			ID: "team.mcp.test", Label: i18n.T("tui.team_mcp.cmd_test_label"),
			Aliases:     []string{"test", "ping", "check"},
			Description: i18n.T("tui.team_mcp.cmd_test_desc"), Category: "MCP",
			Action: v.testConnection,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

// setTeamConfig takes the config that was read (nil: it has none, the config
// already shown stays, else an empty one).
func (v *TeamMCPView) setTeamConfig(cfg *teamstate.TeamConfig) {
	if cfg != nil {
		v.teamCfg = cfg
	}
	// Ensure non-nil teamCfg for editing
	if v.teamCfg == nil {
		v.teamCfg = &teamstate.TeamConfig{}
	}
	if v.teamCfg.MCP == nil {
		v.teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Build config lines
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) buildFields() {
	v.fields = nil

	for _, svc := range []string{"gitlab", "jira", "figma", "gslides"} {
		svc := svc // capture
		svcTitle := cases.Title(language.Und).String(svc)

		// ── Section header: service name ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: "MCP " + svcTitle})

		// ── Sub-section: Team (shared config.toml) ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSubHeader, Label: i18n.T("tui.config.section.team_shared")})
		v.fields = append(v.fields, configField{
			Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].Enabled) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.Enabled = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.fields = append(v.fields, configField{
			Key: "enabled_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enforced.label"),
			Description: i18n.T("tui.config.field.mcp_enforced.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].EnabledEnforced) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.EnabledEnforced = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		if mcpresolve.ServiceHasURL(svc) {
			v.fields = append(v.fields, configField{
				Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
				Description: i18n.T("tui.config.field.mcp_url.desc"), Scope: ScopeTeamShared, Section: svc,
				Placeholder: i18n.T("tui.config.field.mcp_url.placeholder"),
				Get:         func() string { return v.teamCfg.MCP[svc].URL },
				Set: func(val string) {
					s := v.teamCfg.MCP[svc]
					s.URL = val
					v.teamCfg.MCP[svc] = s
					v.dirtyTeam = true
				},
			})
			v.fields = append(v.fields, configField{
				Key: "url_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_url_enforced.label"),
				Description: i18n.T("tui.config.field.mcp_url_enforced.desc"), Scope: ScopeTeamShared, Section: svc,
				Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].URLEnforced) },
				Set: func(val string) {
					s := v.teamCfg.MCP[svc]
					b := val == "true"
					s.URLEnforced = &b
					v.teamCfg.MCP[svc] = s
					v.dirtyTeam = true
				},
			})
		}
		v.fields = append(v.fields, configField{
			Key: "write_recommended", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_write_recommended.label"),
			Description: i18n.T("tui.config.field.mcp_write_recommended.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolStr(v.teamCfg.MCP[svc].WriteRecommended) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				s.WriteRecommended = val == "true"
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})

		// ── Sub-section: Personal (local hub.toml) ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSubHeader, Label: i18n.T("tui.config.section.personal")})
		v.fields = append(v.fields, configField{
			Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Locked: func() bool { return v.teamCfg.MCP[svc].IsEnabledEnforced() },
			Get:    v.getMCPLocalEnabled(svc),
			Set:    v.setMCPLocalEnabled(svc),
		})
		if mcpresolve.ServiceHasURL(svc) {
			v.fields = append(v.fields, configField{
				Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
				Description: i18n.T("tui.config.field.mcp_url.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
				Locked: func() bool { return v.teamCfg.MCP[svc].IsURLEnforced() },
				Get:    v.getMCPLocalURL(svc),
				Set:    v.setMCPLocalURL(svc),
			})
		}
		v.fields = append(v.fields, configField{
			Key: "token", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Get: v.getMCPLocalToken(svc),
			Set: v.setMCPLocalToken(svc),
		})
		v.fields = append(v.fields, configField{
			Key: "write_enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_write.label"),
			Description: i18n.T("tui.config.field.mcp_write.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Get: v.getMCPLocalWrite(svc),
			Set: v.setMCPLocalWrite(svc),
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

// syncBadge shows whether the changes are published to the team-state.
func (v *TeamMCPView) syncBadge() {
	if v.saving {
		v.badge.set(saveStatusPending) // the write runs: not published yet
		return
	}
	v.badge.settle(v.dirtyTeam || v.dirtyLocal)
}

func (v *TeamMCPView) renderFields() {
	v.syncBadge()
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	v.list.SetItems(configItems(v.fields, 28))
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
	showCurrentFieldDetail(v.page, v.list, v.fields, teamMCPExtraKeys()...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing (delegates to shared editConfigField / toggleConfigField)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) onItemSelected(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if !isEditable(f.Kind) || f.Get == nil || v.shell == nil {
		return
	}
	if f.Locked != nil && f.Locked() {
		v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		return
	}

	// Password fields: token setup flow
	if f.Kind == CfgFieldPassword {
		tokenKey := f.Get()
		if tokenKey == "" {
			// Auto-assign default key name based on service section
			svc := f.Section
			if idx := strings.Index(svc, "."); idx >= 0 {
				svc = svc[:idx]
			}
			tokenKey = config.DefaultTokenKeyForService(svc)
			f.Set(tokenKey)
		}
		v.shell.ShowPasswordModal(i18n.T("tui.config.field.mcp_token.label")+" ("+tokenKey+")", func(val string) {
			if val == "" {
				return
			}
			persist(v.app, func(ctx context.Context) error {
				if v.cfg.SetSecret == nil {
					return nil
				}
				return v.cfg.SetSecret(ctx, tokenKey, val)
			}, func(err error) {
				if err != nil {
					v.shell.ShowToastMsg(errorText(err), false)
					return
				}
				v.renderFields()
				v.shell.ShowToastMsg(i18n.T("tui.settings.secret_updated"), true)
			})
		})
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.renderFields()
	})
}

func (v *TeamMCPView) toggleSelected() {
	if v.list == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if f.Locked != nil && f.Locked() {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		}
		return
	}
	if !isToggleable(f.Kind) {
		return
	}
	v.pushUndo()
	toggleConfigField(f)
	v.renderFields()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test connection
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) testConnection() {
	if v.list == nil || v.shell == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}

	// Determine service from field's Section (e.g. "gitlab", "gitlab.perso")
	svc := v.fields[ref].Section
	if idx := strings.Index(svc, "."); idx >= 0 {
		svc = svc[:idx]
	}
	if svc == "" {
		v.shell.ShowToastMsg(i18n.T("tui.team_mcp.select_field"), false)
		return
	}

	// The test reads the saved configuration: unsaved changes would be
	// tested under the wrong name.
	if v.dirtyTeam || v.dirtyLocal {
		v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.test_unsaved", svc), false)
		return
	}
	var run func(ctx context.Context) (string, error)
	if v.cfg.TestMCP != nil {
		run = v.cfg.TestMCP(svc)
	}
	if run == nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.test_unavailable", svc), false)
		return
	}

	v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.testing", svc), true)
	sh := v.shell
	load(v.app, sh.Context(), &v.tests, func(ctx context.Context) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, mcpTestTimeout)
		defer cancel()
		return run(ctx)
	}, func(detail string, err error) {
		if err != nil {
			sh.ShowScrollableModal(i18n.Tf("tui.team_mcp.test_failed_title", svc), tview.Escape(err.Error()), nil)
			return
		}
		sh.ShowToastMsg(i18n.Tf("tui.team_mcp.test_ok", svc, detail), true)
	})
}

// mcpTestTimeout bounds the connection test of a service.
const mcpTestTimeout = 20 * time.Second

// ─────────────────────────────────────────────────────────────────────────────
// Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) pushUndo() {
	if v.teamCfg == nil {
		return
	}
	v.undoStack.Push(teamMCPSnapshot{
		teamCfg:  deepCopyTeamConfig(v.teamCfg),
		localMCP: v.localMCP,
	})
}

func (v *TeamMCPView) undo() {
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
	v.dirtyTeam = v.undoStack.Len() > 0
	v.dirtyLocal = v.undoStack.Len() > 0
	v.buildFields()
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Persistence
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) save() {
	if v.shell == nil {
		return
	}
	if v.saving {
		v.saveAgain = true
		return
	}
	// What the write needs from the view and the app is taken here, on the
	// event loop; the write runs off it.
	var (
		writeTeam TeamConfigWriter
		teamCopy  teamstate.TeamConfig
		patch     func(*config.MCPConfig)
		newMCP    = v.localMCP
	)
	saveTeam := v.dirtyTeam && v.cfg.SaveTeamConfig != nil
	if saveTeam {
		if writeTeam = v.cfg.SaveTeamConfig(); writeTeam == nil {
			saveTeam = false
		} else {
			teamCopy = deepCopyTeamConfig(v.teamCfg)
		}
	}
	saveLocal := v.dirtyLocal
	writeLocal := saveLocal && v.cfg.UpdateHubConfig != nil
	if writeLocal {
		patch = config.Patch(v.baseMCP, v.localMCP)
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
			teamErr = writeTeam(ctx, &teamCopy)
			if teamErr != nil {
				return teamErr
			}
			teamDone = true
		}
		if writeLocal {
			localErr = v.cfg.UpdateHubConfig(func(c *config.Config) error {
				patch(&c.MCP)
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
		again := v.saveAgain
		v.saveAgain = false
		switch {
		case teamErr != nil:
			v.dirtyTeam = true
			v.dirtyLocal = v.dirtyLocal || saveLocal // not tried
			v.shell.ShowToastMsg(i18n.T("tui.team_mcp.save_error_team")+teamErr.Error(), false)
			v.badge.set(saveStatusFailed)
			return
		case localErr != nil:
			v.dirtyLocal = true
			v.shell.ShowToastMsg(i18n.T("tui.team_mcp.save_error_local")+localErr.Error(), false)
			v.badge.set(saveStatusFailed)
			return
		}
		var saved []string
		if teamDone {
			saved = append(saved, i18n.T("tui.team_mcp.scope_team"))
		}
		if saveLocal {
			if localDone {
				v.baseMCP = newMCP
			}
			saved = append(saved, i18n.T("tui.team_mcp.scope_local"))
		}
		if len(saved) == 0 {
			v.syncBadge()
			v.shell.ShowToastMsg(i18n.T("tui.team_mcp.no_changes"), true)
			return
		}
		v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.saved", strings.Join(saved, " + ")), true)
		v.renderFields()
		if again {
			v.save()
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP Local (hub.toml) accessor helpers
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) getMCPLocalEnabled(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return mcpBoolStr(v.localMCP.Gitlab.Enabled)
		case "jira":
			return mcpBoolStr(v.localMCP.Jira.Enabled)
		case "figma":
			return mcpBoolStr(v.localMCP.Figma.Enabled)
		case "gslides":
			return mcpBoolStr(v.localMCP.Gslides.Enabled)
		}
		return "false"
	}
}

func (v *TeamMCPView) setMCPLocalEnabled(svc string) func(string) {
	return func(val string) {
		b := val == "true"
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.Enabled = b
		case "jira":
			v.localMCP.Jira.Enabled = b
		case "figma":
			v.localMCP.Figma.Enabled = b
		case "gslides":
			v.localMCP.Gslides.Enabled = b
		}
		v.dirtyLocal = true
	}
}

func (v *TeamMCPView) getMCPLocalURL(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return v.localMCP.Gitlab.URL
		case "jira":
			return v.localMCP.Jira.URL
		case "figma":
			return v.localMCP.Figma.URL
		case "gslides":
			return v.localMCP.Gslides.URL
		}
		return ""
	}
}

func (v *TeamMCPView) setMCPLocalURL(svc string) func(string) {
	return func(val string) {
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.URL = val
		case "jira":
			v.localMCP.Jira.URL = val
		case "figma":
			v.localMCP.Figma.URL = val
		case "gslides":
			v.localMCP.Gslides.URL = val
		}
		v.dirtyLocal = true
	}
}

func (v *TeamMCPView) getMCPLocalToken(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return v.localMCP.Gitlab.Token
		case "jira":
			return v.localMCP.Jira.Token
		case "figma":
			return v.localMCP.Figma.Token
		case "gslides":
			return v.localMCP.Gslides.Token
		}
		return ""
	}
}

func (v *TeamMCPView) setMCPLocalToken(svc string) func(string) {
	return func(val string) {
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.Token = val
		case "jira":
			v.localMCP.Jira.Token = val
		case "figma":
			v.localMCP.Figma.Token = val
		case "gslides":
			v.localMCP.Gslides.Token = val
		}
		v.dirtyLocal = true
	}
}

func (v *TeamMCPView) getMCPLocalWrite(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return mcpBoolStr(v.localMCP.Gitlab.WriteEnabled)
		case "jira":
			return mcpBoolStr(v.localMCP.Jira.WriteEnabled)
		case "figma":
			return mcpBoolStr(v.localMCP.Figma.WriteEnabled)
		case "gslides":
			return mcpBoolStr(v.localMCP.Gslides.WriteEnabled)
		}
		return "false"
	}
}

func (v *TeamMCPView) setMCPLocalWrite(svc string) func(string) {
	return func(val string) {
		b := val == "true"
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.WriteEnabled = b
		case "jira":
			v.localMCP.Jira.WriteEnabled = b
		case "figma":
			v.localMCP.Figma.WriteEnabled = b
		case "gslides":
			v.localMCP.Gslides.WriteEnabled = b
		}
		v.dirtyLocal = true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// mcpBoolPtrToStr converts a *bool to "true"/"false" (nil → "" = not configured).
func mcpBoolPtrToStr(b *bool) string {
	if b == nil {
		return "" // not configured
	}
	if *b {
		return "true"
	}
	return "false"
}

// mcpBoolStr converts a plain bool to "true"/"false".
func mcpBoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
