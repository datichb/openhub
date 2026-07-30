package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TeamDetailViewConfig holds the external dependencies for TeamDetailView.
type TeamDetailViewConfig struct {
	// GetMCPConfig returns the local MCP config from hub.toml.
	GetMCPConfig func() config.MCPConfig
	// GetTrackerLocalConfig returns the local tracker override from hub.toml.
	GetTrackerLocalConfig func() config.TrackerLocalConfig
	// ResolveTeam returns the effective team config for the active project.
	ResolveTeam ResolveTeamFunc
	// SaveTeamConfig persists changes to team-state config.toml and pushes.
	SaveTeamConfig func(ctx context.Context, cfg *teamstate.TeamConfig) error
	// SaveLocalMCP persists a local MCP setting to hub.toml.
	// key is a TOML key path (e.g. "mcp.gitlab.write_enabled"), value is the new value.
	SaveLocalMCP func(key, value string) error
	// SaveLocalTracker persists a local tracker override to hub.toml.
	SaveLocalTracker func(key, value string) error
	// GetSecrets returns the secret store for connection tests.
	GetSecrets func() tracker.SecretGetter
	// GetHubConfig returns the full hub config (for local tracker overrides save).
	GetHubConfig func() *config.Config
	// ListProjects returns all registered hub projects (for tracker mapping UI).
	ListProjects func(ctx context.Context) []ProjectInfo
	// SyncTracker runs the tracker sync and returns a formatted result.
	SyncTracker func(ctx context.Context) (*SyncTrackerResult, error)
}

// ProjectInfo is a minimal project representation for the tracker mapping UI.
type ProjectInfo struct {
	ID   string
	Name string
}

// SyncTrackerResult holds the outcome of a tracker sync for display in a modal.
type SyncTrackerResult struct {
	ClaimsCreated int
	ClaimsUpdated int
	LabelsPushed  int
	Projects      []string // per-project summary lines
	Warnings      []string
	Errors        []string
}

// TeamDetailView displays and edits MCP + tracker configuration
// with two display modes:
//   - Simple: only the effective (resolved) values
//   - Detailed: three columns — equipe | local | effectif
type TeamDetailView struct {
	app    *tview.Application
	tv     *tview.TextView
	shell  ShellAccess
	cfg    TeamDetailViewConfig

	// detailedMode toggles between simple (false) and detailed (true) display.
	detailedMode bool
	// dirty tracks whether there are unsaved local changes.
	dirty bool

	// cached data loaded on Mount/refresh
	teamCfg  *teamstate.TeamConfig
	localMCP config.MCPConfig
	localTrk config.TrackerLocalConfig
}

var _ View = (*TeamDetailView)(nil)

// NewTeamDetailView creates the MCP & tracker config view.
func NewTeamDetailView(cfg TeamDetailViewConfig) *TeamDetailView {
	return &TeamDetailView{cfg: cfg}
}

// SetShell provides the shell reference for toast/modal interactions.
func (v *TeamDetailView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier used by the omnibar router.
func (v *TeamDetailView) ID() string { return "team.detail" }

// Title returns the display title.
func (v *TeamDetailView) Title() string { return "Équipe - Détail" }

// StatusHints returns keybinding hints shown in the status bar.
func (v *TeamDetailView) StatusHints() string {
	if v.detailedMode {
		return "v simple · g config équipe · l config locale · s sync · t tester · r refresh"
	}
	return "v détaillé · g config équipe · l config locale · s sync · t tester · r refresh"
}

// Mount builds and displays the view content.
func (v *TeamDetailView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// Async pull then render
	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		syncAsync(v.app, repo, v.shell, func(_ error) {
			v.loadAndRender()
		})
	} else {
		v.loadAndRender()
	}

	content.AddItem(v.tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *TeamDetailView) Unmount() {
	if v.dirty && v.shell != nil {
		// Inform user of unsaved changes — they navigate away
		v.shell.ShowToastMsg("⚠ Modifications non sauvegardées — utilisez 'w' pour sauvegarder", false)
	}
	v.app = nil
	v.tv = nil
}

// HandleKey processes view key events.
func (v *TeamDetailView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		v.detailedMode = !v.detailedMode
		v.render()
		return nil
	case 'g':
		v.editTeamConfig()
		return nil
	case 'l':
		v.editLocalConfig()
		return nil
	case 't':
		v.testConnection()
		return nil
	case 's':
		v.syncTracker()
		return nil
	case 'w':
		v.save()
		return nil
	case 'r':
		tc := v.cfg.ResolveTeam()
		if tc.Enabled {
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			syncAsync(v.app, repo, v.shell, func(_ error) {
				v.loadAndRender()
			})
		} else {
			v.loadAndRender()
		}
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) loadAndRender() {
	v.localMCP = v.cfg.GetMCPConfig()
	v.localTrk = v.cfg.GetTrackerLocalConfig()

	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			v.teamCfg, _ = repo.LoadConfig()
		}
	}

	v.render()
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder

	title := "Services & Tracker"
	if v.detailedMode {
		title += "  " + theme.ColorTag(theme.TextMutedHex) + "[vue détaillée — 'v' pour vue simple]" + theme.TagColor
	} else {
		title += "  " + theme.ColorTag(theme.TextMutedHex) + "['v' pour vue détaillée équipe|local|effectif]" + theme.TagColor
	}
	sb.WriteString(fmt.Sprintf("\n  [::b]%s%s\n\n", title, theme.TagReset))

	// ── MCP Services ──────────────────────────────────────────────────────────
	var sharedMCP map[string]teamstate.SharedMCPConfig
	if v.teamCfg != nil {
		sharedMCP = v.teamCfg.MCP
	}

	services := []struct {
		name  string
		local config.MCPServerConfig
	}{
		{"gitlab", v.localMCP.Gitlab},
		{"jira", v.localMCP.Jira},
		{"figma", v.localMCP.Figma},
		{"gslides", v.localMCP.Gslides},
	}

	for _, svc := range services {
		var shared *teamstate.SharedMCPConfig
		if sharedMCP != nil {
			if s, ok := sharedMCP[svc.name]; ok {
				shared = &s
			}
		}
		eff := tracker.ResolveMCPConfig(shared, svc.local)
		v.renderMCPSection(&sb, svc.name, shared, svc.local, eff)
	}

	// ── Tracker Sync ──────────────────────────────────────────────────────────
	sb.WriteString(fmt.Sprintf("\n  %s%s%s\n", theme.ColorTag(theme.AccentHex), "─── Tracker Sync ───", theme.TagColor))

	if v.teamCfg == nil || v.teamCfg.Tracker.Type == "" {
		sb.WriteString(fmt.Sprintf("  %sNon configuré — lance %s\n",
			theme.ColorTag(theme.TextSecondaryHex),
			theme.TagColor))
		sb.WriteString(fmt.Sprintf("  %soh team config%s  pour configurer\n",
			theme.ColorTag(theme.AccentHex), theme.TagColor))
	} else {
		writeEnabled := v.resolveWriteEnabled()
		eff := tracker.ResolveTrackerConfig(&v.teamCfg.Tracker, v.localTrk, writeEnabled)
		v.renderTrackerSection(&sb, eff)
	}

	v.tv.SetText(sb.String())
}

func (v *TeamDetailView) renderMCPSection(sb *strings.Builder, name string, shared *teamstate.SharedMCPConfig, local config.MCPServerConfig, eff tracker.EffectiveMCPConfig) {
	displayName := strings.ToUpper(name[:1]) + name[1:]
	sb.WriteString(fmt.Sprintf("  %s%s%s\n", theme.ColorTag(theme.AccentHex), "─── "+displayName+" ───", theme.TagColor))

	if v.detailedMode {
		// Header
		sb.WriteString(fmt.Sprintf("  %-20s %-20s %-20s\n",
			theme.ColorTag(theme.TextMutedHex)+"équipe"+theme.TagColor,
			theme.ColorTag(theme.TextMutedHex)+"local"+theme.TagColor,
			theme.ColorTag(theme.TextMutedHex)+"effectif"+theme.TagColor))

		// Enabled
		teamEnabledStr := "—"
		if shared != nil && shared.Enabled != nil {
			teamEnabledStr = fmtBoolColor(*shared.Enabled)
		}
		localEnabledStr := fmtBoolColor(local.Enabled)
		sb.WriteString(fmt.Sprintf("  enabled:     %-20s %-20s %s\n",
			teamEnabledStr, localEnabledStr, fmtBoolColor(eff.Enabled)))

		// URL
		teamURL := "—"
		if shared != nil && shared.URL != "" {
			teamURL = shared.URL
		}
		localURL := "—"
		if local.URL != "" {
			localURL = local.URL
		}
		effURL := eff.URL
		if effURL == "" {
			effURL = "(défaut)"
		}
		sb.WriteString(fmt.Sprintf("  url:         %-20s %-20s %s\n",
			truncate(teamURL, 18), truncate(localURL, 18), truncate(effURL, 18)))

		// WriteEnabled
		teamWriteStr := "—"
		if shared != nil {
			teamWriteStr = fmt.Sprintf("rec:%v", shared.WriteRecommended)
		}
		sb.WriteString(fmt.Sprintf("  write:       %-20s %-20s %s\n",
			teamWriteStr, fmtBoolColor(local.WriteEnabled), fmtBoolColor(eff.WriteEnabled)))
	} else {
		// Simple view — effective only
		sb.WriteString(fmt.Sprintf("  enabled: %s", fmtBoolColor(eff.Enabled)))
		if eff.URL != "" {
			sb.WriteString(fmt.Sprintf("  url: %s", truncate(eff.URL, 30)))
		}
		sb.WriteString(fmt.Sprintf("  write: %s", fmtBoolColor(eff.WriteEnabled)))
		if eff.WriteRecommended && !eff.WriteEnabled {
			sb.WriteString(fmt.Sprintf("  %s(équipe recommande: true)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor))
		}
		sb.WriteString("\n")
		// Token
		tokenStr := theme.ColorTag(theme.TextSecondaryHex) + "(non configuré)" + theme.TagColor
		if eff.TokenKey != "" {
			tokenStr = theme.ColorTag(theme.TextMutedHex) + "****" + last4TUI(eff.TokenKey) + theme.TagColor
		}
		sb.WriteString(fmt.Sprintf("  token: %s\n", tokenStr))
	}
}

func (v *TeamDetailView) renderTrackerSection(sb *strings.Builder, eff tracker.EffectiveTrackerConfig) {
	if v.detailedMode {
		shared := &v.teamCfg.Tracker
		local := v.localTrk

		sb.WriteString(fmt.Sprintf("  %-20s %-20s %-20s\n",
			theme.ColorTag(theme.TextMutedHex)+"équipe"+theme.TagColor,
			theme.ColorTag(theme.TextMutedHex)+"local"+theme.TagColor,
			theme.ColorTag(theme.TextMutedHex)+"effectif"+theme.TagColor))

		fields := []struct {
			label    string
			team     string
			localStr string
			eff      string
			insight  string
		}{
			{"type", shared.Type, "—", eff.Type, ""},
			{"enabled", fmtBoolColor(shared.Enabled), fmtBoolPtrColor(local.Enabled), fmtBoolColor(eff.Enabled), ""},
			{"auto_sync", fmtBoolColor(shared.AutoSync), fmtBoolPtrColor(local.AutoSync), fmtBoolColor(eff.AutoSync), ""},
			{"push_labels", fmtBoolColor(shared.PushLabels), fmtBoolPtrColor(local.PushLabels), fmtBoolColor(eff.PushLabels),
				insightPushLabels(eff)},
			{"auto_plan", fmtBoolColor(shared.AutoPlanAssigned), fmtBoolPtrColor(local.AutoPlanAssigned), fmtBoolColor(eff.AutoPlanAssigned), ""},
		}
		for _, f := range fields {
			insight := ""
			if f.insight != "" {
				insight = "  " + theme.ColorTag(theme.TextMutedHex) + "ℹ " + f.insight + theme.TagColor
			}
			sb.WriteString(fmt.Sprintf("  %-14s %-20s %-20s %s%s\n",
				f.label+":", f.team, f.localStr, f.eff, insight))
		}
	} else {
		sb.WriteString(fmt.Sprintf("  type: %s  enabled: %s  auto_sync: %s  push_labels: %s\n",
			eff.Type, fmtBoolColor(eff.Enabled), fmtBoolColor(eff.AutoSync), fmtBoolColor(eff.PushLabels)))
		if eff.LocalOverrides.PushLabels {
			sb.WriteString(fmt.Sprintf("  %sℹ push_labels surchargé localement (équipe recommande: %v)%s\n",
				theme.ColorTag(theme.TextMutedHex), eff.SharedPushLabels, theme.TagColor))
		}
		if len(v.teamCfg.Tracker.Projects) > 0 {
			sb.WriteString("  mappings:")
			for hubID, trackerID := range v.teamCfg.Tracker.Projects {
				sb.WriteString(fmt.Sprintf("  %s→%s", hubID, trackerID))
			}
			sb.WriteString("\n")
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) editTeamConfig() {
	if v.shell == nil {
		return
	}
	if v.teamCfg == nil {
		v.shell.ShowToastMsg("Config équipe non disponible (team-state non cloné ?)", false)
		return
	}

	// Step 1: Edit tracker type + flags
	trkType := v.teamCfg.Tracker.Type
	if trkType == "" {
		trkType = "gitlab"
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Configuration tracker (équipe)",
		Fields: []FormField{
			{Key: "type", Label: "Type", Type: FieldSelect,
				Options: []SelectOption{{Label: "GitLab", Value: "gitlab"}, {Label: "Jira", Value: "jira"}},
				Default: trkType},
			{Key: "enabled", Label: "Activé", Type: FieldBool,
				Default: boolToStr(v.teamCfg.Tracker.Enabled)},
			{Key: "auto_sync", Label: "Auto-sync", Type: FieldBool,
				Default: boolToStr(v.teamCfg.Tracker.AutoSync)},
			{Key: "push_labels", Label: "Push labels", Type: FieldBool,
				Default: boolToStr(v.teamCfg.Tracker.PushLabels)},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			v.teamCfg.Tracker.Type = values["type"]
			v.teamCfg.Tracker.Enabled = values["enabled"] == "true"
			v.teamCfg.Tracker.AutoSync = values["auto_sync"] == "true"
			v.teamCfg.Tracker.PushLabels = values["push_labels"] == "true"

			// Step 2: Edit project mappings
			v.editProjectMappings()
		},
	})
}

func (v *TeamDetailView) editProjectMappings() {
	if v.shell == nil || v.cfg.ListProjects == nil {
		// No project listing available — save directly
		v.saveTeamConfigAndRender()
		return
	}

	ctx := context.Background()
	projects := v.cfg.ListProjects(ctx)

	if len(projects) == 0 {
		// No projects registered — save directly
		v.saveTeamConfigAndRender()
		return
	}

	// Build form fields: one text field per hub project
	fields := make([]FormField, len(projects))
	for i, p := range projects {
		currentVal := ""
		if v.teamCfg.Tracker.Projects != nil {
			currentVal = v.teamCfg.Tracker.Projects[p.ID]
		}
		fields[i] = FormField{
			Key:     p.ID,
			Label:   fmt.Sprintf("%s → ID tracker", p.Name),
			Type:    FieldText,
			Default: currentVal,
			Hint:    "Numéro du projet ou path (vide = pas de mapping)",
		}
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  "Mappings projets → tracker",
		Fields: fields,
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			if v.teamCfg.Tracker.Projects == nil {
				v.teamCfg.Tracker.Projects = make(map[string]string)
			}
			for _, p := range projects {
				if val := values[p.ID]; val != "" {
					v.teamCfg.Tracker.Projects[p.ID] = val
				} else {
					delete(v.teamCfg.Tracker.Projects, p.ID)
				}
			}
			v.saveTeamConfigAndRender()
		},
		OnCancel: func() {
			// Still save the tracker type/flags from step 1
			v.saveTeamConfigAndRender()
		},
	})
}

func (v *TeamDetailView) saveTeamConfigAndRender() {
	if v.cfg.SaveTeamConfig == nil || v.teamCfg == nil {
		return
	}
	ctx := context.Background()
	if err := v.cfg.SaveTeamConfig(ctx, v.teamCfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur sauvegarde: "+err.Error(), false)
		}
		return
	}
	if v.shell != nil {
		v.shell.ShowToastMsg("✓ Config tracker sauvegardée et poussée", true)
	}
	v.loadAndRender()
}

func (v *TeamDetailView) editLocalConfig() {
	if v.shell == nil {
		return
	}

	triOptions := []SelectOption{
		{Label: "(hériter de l'équipe)", Value: "inherit"},
		{Label: "Oui", Value: "true"},
		{Label: "Non", Value: "false"},
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Overrides tracker (local)",
		Fields: []FormField{
			{Key: "enabled", Label: "Activé", Type: FieldSelect,
				Options: triOptions, Default: ptrBoolToTriState(v.localTrk.Enabled)},
			{Key: "auto_sync", Label: "Auto-sync", Type: FieldSelect,
				Options: triOptions, Default: ptrBoolToTriState(v.localTrk.AutoSync)},
			{Key: "push_labels", Label: "Push labels", Type: FieldSelect,
				Options: triOptions, Default: ptrBoolToTriState(v.localTrk.PushLabels)},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			hubCfg := v.cfg.GetHubConfig()
			if hubCfg == nil {
				return
			}
			hubCfg.Tracker.Enabled = triStateToPtrBool(values["enabled"])
			hubCfg.Tracker.AutoSync = triStateToPtrBool(values["auto_sync"])
			hubCfg.Tracker.PushLabels = triStateToPtrBool(values["push_labels"])

			if err := config.Save(hubCfg); err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg("Erreur sauvegarde: "+err.Error(), false)
				}
				return
			}
			v.localTrk = hubCfg.Tracker
			if v.shell != nil {
				v.shell.ShowToastMsg("✓ Overrides locaux sauvegardés", true)
			}
			v.render()
		},
	})
}

func (v *TeamDetailView) syncTracker() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.cfg.SyncTracker == nil {
		v.shell.ShowToastMsg("Sync tracker non disponible", false)
		return
	}

	v.shell.ShowToastMsg("Synchronisation en cours...", true)

	go func() {
		ctx := context.Background()
		result, err := v.cfg.SyncTracker(ctx)

		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.shell.ShowToastMsg("✗ Sync échouée: "+err.Error(), false)
				return
			}
			// Show result modal
			content := formatSyncTrackerResult(result)
			v.shell.ShowScrollableModal("Résultat sync tracker", content, []ModalAction{
				{Label: "OK", Callback: func() {
					v.loadAndRender()
				}},
			})
		})
	}()
}

func (v *TeamDetailView) save() {
	if v.shell == nil {
		return
	}
	v.shell.ShowToastMsg("✓ Modifications sauvegardées via les modaux g/l", true)
	v.dirty = false
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func ptrBoolToTriState(p *bool) string {
	if p == nil {
		return "inherit"
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
		return nil
	}
}

func formatSyncTrackerResult(r *SyncTrackerResult) string {
	if r == nil {
		return "Aucun résultat"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Claims créés:      %d\n", r.ClaimsCreated))
	sb.WriteString(fmt.Sprintf("Claims mis à jour: %d\n", r.ClaimsUpdated))
	sb.WriteString(fmt.Sprintf("Labels poussés:    %d\n", r.LabelsPushed))

	if len(r.Projects) > 0 {
		sb.WriteString("\nProjets:\n")
		for _, p := range r.Projects {
			sb.WriteString(fmt.Sprintf("  %s\n", p))
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", w))
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\nErreurs:\n")
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", e))
		}
	}
	return sb.String()
}

func (v *TeamDetailView) testConnection() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.teamCfg == nil || v.teamCfg.Tracker.Type == "" {
		v.shell.ShowToastMsg("Tracker non configuré", false)
		return
	}

	v.shell.ShowToastMsg("Test de connexion...", true)

	go func() {
		ctx := context.Background()
		var sharedMCP map[string]teamstate.SharedMCPConfig
		if v.teamCfg != nil {
			sharedMCP = v.teamCfg.MCP
		}

		// Build credential source from current config
		mcp := v.cfg.GetMCPConfig()
		var sharedGitLab, sharedJira *teamstate.SharedMCPConfig
		if sharedMCP != nil {
			if g, ok := sharedMCP["gitlab"]; ok {
				sharedGitLab = &g
			}
			if j, ok := sharedMCP["jira"]; ok {
				sharedJira = &j
			}
		}
		effGL := tracker.ResolveMCPConfig(sharedGitLab, mcp.Gitlab)
		effJira := tracker.ResolveMCPConfig(sharedJira, mcp.Jira)
		src := tracker.CredentialSource{
			GitLabEnabled:      effGL.Enabled,
			GitLabTokenKey:     effGL.TokenKey,
			GitLabWriteEnabled: effGL.WriteEnabled,
			GitLabURL:          effGL.URL,
			JiraEnabled:        effJira.Enabled,
			JiraTokenKey:       effJira.TokenKey,
			JiraWriteEnabled:   effJira.WriteEnabled,
			JiraURL:            effJira.URL,
			Secrets:            v.cfg.GetSecrets(),
		}

		trackerType := tracker.Type(v.teamCfg.Tracker.Type)
		cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.shell.ShowToastMsg("✗ Credentials non disponibles: "+err.Error(), false)
			})
			return
		}

		t, err := tracker.New(cfg)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.shell.ShowToastMsg("✗ Initialisation échouée: "+err.Error(), false)
			})
			return
		}

		username, err := t.TestConnection(ctx)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.shell.ShowToastMsg("✗ Connexion échouée: "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg("✓ Connecté en tant que @"+username, true)
			}
		})
	}()
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) resolveWriteEnabled() bool {
	if v.teamCfg == nil {
		return false
	}
	switch tracker.Type(v.teamCfg.Tracker.Type) {
	case tracker.TypeGitLab:
		return v.localMCP.Gitlab.WriteEnabled
	case tracker.TypeJira:
		return v.localMCP.Jira.WriteEnabled
	}
	return false
}

func fmtBoolColor(b bool) string {
	if b {
		return "[green]✓[-]"
	}
	return "[gray]✗[-]"
}

func fmtBoolPtrColor(b *bool) string {
	if b == nil {
		return "[gray]—[-]"
	}
	return fmtBoolColor(*b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func last4TUI(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

func insightPushLabels(eff tracker.EffectiveTrackerConfig) string {
	if eff.LocalOverrides.PushLabels {
		return fmt.Sprintf("équipe recommande: %v", eff.SharedPushLabels)
	}
	return ""
}
