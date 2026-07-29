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
		return "v simple · g config équipe · l config locale · t tester · w sauvegarder · r refresh · Esc retour"
	}
	return "v détaillé · g config équipe · l config locale · t tester · w sauvegarder · r refresh · Esc retour"
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
	v.shell.ShowToastMsg("Utilisez 'oh team config' en CLI pour éditer la config d'équipe", true)
}

func (v *TeamDetailView) editLocalConfig() {
	if v.shell == nil {
		return
	}
	v.shell.ShowToastMsg("Utilisez 'oh team config' en CLI pour éditer la config locale", true)
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

func (v *TeamDetailView) save() {
	if v.shell == nil {
		return
	}
	// For now, direct hub.toml edits are delegated to the CLI wizard.
	// The view displays current state; edits happen via 'oh team config'.
	v.shell.ShowToastMsg("Utilisez 'oh team config' pour sauvegarder les modifications", true)
	v.dirty = false
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
