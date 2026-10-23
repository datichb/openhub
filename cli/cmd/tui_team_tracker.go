package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sync Tracker action (omnibar + view touche 's')
// ─────────────────────────────────────────────────────────────────────────────

// actionSyncTracker is the omnibar action for "sync tracker".
func actionSyncTracker() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	ctx := tuiShell.Context()

	tuiShell.ShowToast(i18n.T("tui.team.sync.in_progress"), shell.ToastInfo)

	safego.Go(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := runSyncTrackerForTUI(a, ctx)
		select {
		case <-ctx.Done():
			return
		default:
		}
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.T("tui.team.sync.error_prefix")+err.Error(), shell.ToastError)
				return
			}
			content := formatSyncResultModal(result)
			tuiShell.ShowScrollableModal(i18n.T("tui.team.sync.result_title"), content, []views.ModalAction{
				{Label: "OK", Callback: func() {}},
			})
		})
	})
}

// runSyncTrackerForTUI executes the tracker sync and returns a result for display.
// The rules (team, enabled = false, credentials) are those of the team
// service, like the command line; only the messages differ.
func runSyncTrackerForTUI(a *app.App, ctx context.Context) (*views.SyncTrackerResult, error) {
	syncResult, err := teamService(a).SyncTracker(ctx, teamsvc.SyncOptions{Pull: true})
	if err != nil {
		return nil, syncTrackerTUIError(err)
	}

	// Convert to view-friendly result
	result := &views.SyncTrackerResult{
		ClaimsCreated: syncResult.ClaimsCreated,
		ClaimsUpdated: syncResult.ClaimsUpdated,
		LabelsPushed:  syncResult.LabelsPushed,
	}
	for _, p := range syncResult.Projects {
		result.Projects = append(result.Projects, fmt.Sprintf("%s: %d fetched, %d created, %d updated", p.ProjectID, p.IssuesFetched, p.ClaimsCreated, p.ClaimsUpdated))
	}
	for _, w := range syncResult.Warnings {
		result.Warnings = append(result.Warnings, w.Message)
	}
	for _, e := range syncResult.Errors {
		result.Errors = append(result.Errors, e.Error())
	}

	return result, nil
}

// syncTrackerTUIError translates the typed errors of the team service into
// the messages of the screen.
func syncTrackerTUIError(err error) error {
	var (
		cfgErr  *teamsvc.ConfigError
		credErr *teamsvc.CredentialsError
		initErr *teamsvc.TrackerInitError
	)
	switch {
	case errors.Is(err, teamsvc.ErrTeamNotConfigured):
		return errors.New(i18n.T("tui.team.sync.team_not_configured"))
	case errors.Is(err, teamsvc.ErrNotCloned):
		return errors.New(i18n.T("tui.team.sync.not_cloned"))
	case errors.As(err, &cfgErr):
		return fmt.Errorf("%s: %w", i18n.T("tui.team.sync.config_load_error"), cfgErr.Err)
	case errors.Is(err, teamsvc.ErrTrackerNotConfigured):
		return errors.New(i18n.T("tui.team.sync.tracker_not_configured"))
	case errors.Is(err, teamsvc.ErrTrackerDisabled):
		return errors.New(i18n.T("tui.team.sync.tracker_disabled"))
	case errors.As(err, &credErr):
		msg := i18n.Tf("tui.team.sync.credentials_missing", credErr.TrackerType)
		var missing *tracker.MissingTokenError
		if errors.As(credErr.Err, &missing) {
			msg = i18n.Tf("tui.team.sync.token_missing_server", tracker.ServerOf(missing.Server))
		}
		return &views.TrackerCredentialsError{Msg: msg, Err: credErr.Err}
	case errors.As(err, &initErr):
		return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.tracker_init_error"), initErr.Err)
	case errors.Is(err, teamsvc.ErrNoProjects):
		return errors.New(i18n.T("tui.team.sync.no_projects"))
	}
	return fmt.Errorf("sync: %w", err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tracker Discovery Wizard (C6)
// ─────────────────────────────────────────────────────────────────────────────

// actionTrackerDiscovery launches the 5-step tracker discovery wizard.
// It connects to the configured tracker, discovers project metadata (labels,
// statuses), suggests board columns and label→column mappings, and saves
// the resulting configuration to the team-state repo.
//
// The wizard follows the same inline-wizard pattern as actionTeamInit.
func actionTrackerDiscovery() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// ── Resolve team config (same pattern as runSyncTrackerForTUI) ───
	tc := resolvedTeamConfig(a, nil)
	if !tc.Enabled {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.team_not_configured"), shell.ToastError)
		return
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.not_cloned"), shell.ToastError)
		return
	}

	// Load existing team config for pre-fill
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.config_load_error")+err.Error(), shell.ToastError)
		return
	}

	// ── Shared state (captured by step closures) ────────────────────
	var (
		// Step 1: Connection
		trackerType = teamCfg.Tracker.Type // pre-fill from existing config
		trackerURL  string
		projectID   = teamCfg.Tracker.TrackerProject
		token       string // resolved at runtime, not pre-filled

		// Step 2: Discovery results
		discoveryInfo     *tracker.DiscoveryInfo
		suggestedColumns  []teamstate.BoardColumnConfig
		suggestedMappings []tracker.SuggestedMapping
		unmapped          []string
		poolLabels        []string

		// Step 3: Final column selection
		finalColumns []teamstate.BoardColumnConfig

		// Step 4: Mappings (accepted as-is from suggestions)
		// suggestedMappings is reused directly

		// Step 5: Pool label selection
		selectedPool []string

		// Summary: TOML preview built after save
		tomlPreview string
	)

	// Pre-fill tracker URL from team-state config or MCP config
	if teamCfg.Tracker.TrackerURL != "" {
		trackerURL = teamCfg.Tracker.TrackerURL
	} else if trackerType != "" {
		// Try to get URL from MCP shared config
		if mcpCfg, ok := teamCfg.MCP[trackerType]; ok && mcpCfg.URL != "" {
			trackerURL = mcpCfg.URL
		}
	}

	// ── Step 1: Connection & Project ────────────────────────────────
	connStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_connection"),
		Required:   true,
		Processing: i18n.T("cmd.discovery.processing_connection"),
		Validate: func() string {
			if strings.TrimSpace(trackerType) == "" {
				return i18n.T("cmd.discovery.validate.type_required")
			}
			if strings.TrimSpace(trackerURL) == "" {
				return i18n.T("cmd.discovery.validate.url_required")
			}
			if strings.TrimSpace(projectID) == "" {
				return i18n.T("cmd.discovery.validate.project_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			// Tracker type dropdown
			typeOptions := []string{"gitlab", "jira"}
			typeIdx := 0
			for i, t := range typeOptions {
				if t == trackerType {
					typeIdx = i
					break
				}
			}
			form.AddDropDown(i18n.T("cmd.discovery.field.tracker_type"), typeOptions, typeIdx,
				func(_ string, idx int) { trackerType = typeOptions[idx] })

			// URL
			form.AddInputField(i18n.T("cmd.discovery.field.tracker_url"), trackerURL, 0, nil,
				func(text string) { trackerURL = text })

			// Project ID
			form.AddInputField(i18n.T("cmd.discovery.field.tracker_project"), projectID, 0, nil,
				func(text string) { projectID = text })

			// Token (optional — if empty, will try keychain/env)
			form.AddPasswordField(i18n.T("cmd.discovery.field.token"), token, 0, '*',
				func(text string) { token = text })

			form.AddButton(i18n.T("cmd.discovery.btn.test"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			// Build tracker config for connection test
			cfg := tracker.Config{
				Type:    tracker.Type(trackerType),
				BaseURL: trackerURL,
			}

			// Resolve token: explicit > keychain > env
			if token != "" {
				cfg.Token = token
			} else {
				credSrc := trackerCredentials(a, teamCfg)
				resolved, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(trackerType))
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.credentials_error"), err)
				}
				cfg.Token = resolved.Token
				if cfg.BaseURL == "" {
					cfg.BaseURL = resolved.BaseURL
				}
			}

			// Create tracker instance and test
			t, err := tracker.New(cfg)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.tracker_init_error"), err)
			}

			// Wrap API calls in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Test connection
			username, err := t.TestConnection(apiCtx)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.connection_failed"), err)
			}
			_ = username // connection OK

			// Test project access
			_, err = t.TestProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.project_inaccessible"), err)
			}

			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Type", Value: trackerType},
				{Label: "URL", Value: trackerURL},
				{Label: i18n.T("tui.team.discovery.info_project"), Value: projectID},
			}
		},
	}

	// ── Step 2: Discovery (processing only, no form) ────────────────
	discoveryStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_discovery"),
		Processing: i18n.T("cmd.discovery.processing_discovery"),
		// No Form — this is a processing-only step
		OnDone: func() error {
			// Build tracker instance (same as step 1 but we need it again)
			cfg := tracker.Config{
				Type:    tracker.Type(trackerType),
				BaseURL: trackerURL,
			}
			if token != "" {
				cfg.Token = token
			} else {
				credSrc := trackerCredentials(a, teamCfg)
				resolved, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(trackerType))
				if err != nil {
					return fmt.Errorf("credentials: %w", err)
				}
				cfg.Token = resolved.Token
				if cfg.BaseURL == "" {
					cfg.BaseURL = resolved.BaseURL
				}
			}

			t, err := tracker.New(cfg)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.tracker_init_error"), err)
			}

			// Wrap API call in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Discover project metadata (labels, statuses)
			discoveryInfo, err = t.DiscoverProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.discovery_error"), err)
			}

			// Start from existing board columns or defaults
			baseColumns := teamCfg.Board.Columns
			if len(baseColumns) == 0 {
				baseColumns = teamstate.DefaultBoardConfig().Columns
			}

			// Suggest new columns based on discovered labels/statuses
			newCols := tracker.SuggestNewColumns(discoveryInfo, baseColumns)
			suggestedColumns = make([]teamstate.BoardColumnConfig, len(baseColumns))
			copy(suggestedColumns, baseColumns)
			suggestedColumns = append(suggestedColumns, newCols...)

			// Suggest label→column and status→column mappings
			suggestedMappings = tracker.SuggestMappings(discoveryInfo, tracker.Type(trackerType), suggestedColumns)

			// Identify unmapped items
			unmapped = tracker.UnmappedItems(discoveryInfo, suggestedMappings)

			// Suggest pool labels (labels mapped to the initial column)
			boardCfg := teamstate.BoardConfig{Columns: suggestedColumns}
			poolLabels = tracker.SuggestPoolLabels(suggestedMappings, boardCfg)

			return nil
		},
		InfoFields: func() []views.InfoField {
			if discoveryInfo == nil {
				return nil
			}
			fields := []views.InfoField{
				{Label: "Labels", Value: i18n.Tf("cmd.discovery.info.labels_found", len(discoveryInfo.Labels))},
				{Label: "Mappings", Value: i18n.Tf("cmd.discovery.info.mappings_suggested", len(suggestedMappings))},
			}
			if len(unmapped) > 0 {
				fields = append(fields, views.InfoField{
					Label: i18n.T("tui.team.discovery.info_unmapped"), Value: fmt.Sprintf("%d", len(unmapped)),
				})
			}
			return fields
		},
	}

	// ── Step 3: Columns (interactive editor) ────────────────────────
	columnsStep := buildColumnEditorStep(&suggestedColumns, &finalColumns, nil, nil, "")

	// ── Step 4: Mappings ────────────────────────────────────────────
	// Track which mappings the user accepts (pre-checked by default).
	var acceptedMappingFlags []bool

	mappingStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_mappings"),
		Processing: i18n.T("cmd.discovery.processing_mappings"),
		SkipIf: func() bool {
			return len(suggestedMappings) == 0
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			// Show unmapped labels as info (read-only).
			if len(unmapped) > 0 {
				maxShow := 5
				if len(unmapped) < maxShow {
					maxShow = len(unmapped)
				}
				info := i18n.Tf("tui.team.discovery.unmapped_labels", len(unmapped), strings.Join(unmapped[:maxShow], ", "))
				if len(unmapped) > maxShow {
					info += "..."
				}
				form.AddTextView("Info", info, 0, 2, false, false)
			}

			// Interactive checkboxes for each suggested mapping.
			acceptedMappingFlags = make([]bool, len(suggestedMappings))
			for i, m := range suggestedMappings {
				acceptedMappingFlags[i] = true // pre-checked
				idx := i
				label := fmt.Sprintf("%s → %s (%s)", m.Source, m.ColumnID, m.Confidence)
				form.AddCheckbox(label, true, func(checked bool) {
					acceptedMappingFlags[idx] = checked
				})
			}

			form.AddButton(i18n.T("cmd.discovery.btn.accept"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			// Filter suggestedMappings to only accepted ones.
			var accepted []tracker.SuggestedMapping
			for i, m := range suggestedMappings {
				if i < len(acceptedMappingFlags) && acceptedMappingFlags[i] {
					accepted = append(accepted, m)
				}
			}
			suggestedMappings = accepted
			return nil
		},
		InfoFields: func() []views.InfoField {
			accepted := 0
			for _, f := range acceptedMappingFlags {
				if f {
					accepted++
				}
			}
			return []views.InfoField{
				{Label: "Mappings", Value: i18n.Tf("cmd.discovery.info.mappings_accepted", accepted)},
				{Label: i18n.T("tui.team.discovery.info_unmapped"), Value: fmt.Sprintf("%d", len(unmapped))},
			}
		},
	}

	// ── Step 5: Pool labels ─────────────────────────────────────────
	poolStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_pool"),
		Processing: i18n.T("cmd.discovery.processing_save"),
		SkipIf: func() bool {
			// Skip if no pool labels were suggested
			return len(poolLabels) == 0
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			if len(poolLabels) == 0 {
				form.AddTextView("Info", i18n.T("cmd.discovery.pool.none_detected"), 0, 2, false, false)
				form.AddButton(i18n.T("cmd.discovery.btn.save"), func() { onDone() })
				return form
			}

			form.AddTextView("Info",
				i18n.T("cmd.discovery.pool.explanation"), 0, 4, false, false)

			poolEnabled := make([]bool, len(poolLabels))
			for i := range poolLabels {
				poolEnabled[i] = true
				idx := i
				form.AddCheckbox(poolLabels[i], true, func(checked bool) {
					poolEnabled[idx] = checked
				})
			}

			form.AddButton(i18n.T("cmd.discovery.btn.save"), func() {
				selectedPool = nil
				for i, l := range poolLabels {
					if poolEnabled[i] {
						selectedPool = append(selectedPool, l)
					}
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			// ── Save everything to team-state config ────────────
			cfg, err := repo.LoadConfig()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.sync.config_load_error"), err)
			}

			// Tracker connection settings
			cfg.Tracker.Type = trackerType
			cfg.Tracker.Enabled = true
			if trackerURL != "" {
				cfg.Tracker.TrackerURL = trackerURL
			}
			cfg.Tracker.TrackerProject = projectID

			// Board columns
			cfg.Board.Columns = finalColumns

			// Label/status mappings
			if trackerType == "gitlab" {
				if cfg.Tracker.LabelStatusMapping == nil {
					cfg.Tracker.LabelStatusMapping = make(map[string]string)
				}
				for _, m := range suggestedMappings {
					cfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
				}
			} else {
				// Jira: use both StatusMapping and LabelStatusMapping
				if cfg.Tracker.StatusMapping == nil {
					cfg.Tracker.StatusMapping = make(map[string]string)
				}
				if cfg.Tracker.LabelStatusMapping == nil {
					cfg.Tracker.LabelStatusMapping = make(map[string]string)
				}
				for _, m := range suggestedMappings {
					if m.Origin == "status" {
						cfg.Tracker.StatusMapping[m.Source] = m.ColumnID
					} else {
						cfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
					}
				}
			}

			// Pool labels
			if len(selectedPool) > 0 {
				cfg.Tracker.AutoPlanUnassigned = true
				cfg.Tracker.UnassignedLabels = selectedPool
			}

			// Save config and commit
			if err := repo.SaveConfig(ctx, cfg); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.board_config.save_error_label"), err)
			}

			// Build TOML preview for the summary screen.
			type previewColumn struct {
				ID   string `toml:"id"`
				Name string `toml:"name"`
				Role string `toml:"role,omitempty"`
			}
			type previewBoard struct {
				Columns []previewColumn `toml:"columns"`
			}
			type previewTracker struct {
				LabelStatusMapping map[string]string `toml:"label_status_mapping,omitempty"`
				StatusMapping      map[string]string `toml:"status_mapping,omitempty"`
				AutoPlanUnassigned bool              `toml:"auto_plan_unassigned,omitempty"`
				UnassignedLabels   []string          `toml:"unassigned_labels,omitempty"`
			}
			type previewRoot struct {
				Board   previewBoard   `toml:"board"`
				Tracker previewTracker `toml:"tracker"`
			}

			cols := make([]previewColumn, len(cfg.Board.Columns))
			for i, c := range cfg.Board.Columns {
				cols[i] = previewColumn{ID: c.ID, Name: c.Name, Role: c.Role}
			}
			preview := previewRoot{
				Board: previewBoard{Columns: cols},
				Tracker: previewTracker{
					LabelStatusMapping: cfg.Tracker.LabelStatusMapping,
					StatusMapping:      cfg.Tracker.StatusMapping,
					AutoPlanUnassigned: cfg.Tracker.AutoPlanUnassigned,
					UnassignedLabels:   cfg.Tracker.UnassignedLabels,
				},
			}
			if raw, err := toml.Marshal(preview); err == nil {
				tomlPreview = strings.TrimSpace(string(raw))
			}

			return repo.CommitAndPush(ctx,
				fmt.Sprintf("tracker: discovery config for %s (%s)", projectID, trackerType),
				"config.toml")
		},
		InfoFields: func() []views.InfoField {
			fields := []views.InfoField{
				{Label: "Tracker", Value: fmt.Sprintf("%s @ %s", trackerType, trackerURL)},
				{Label: i18n.T("tui.team.discovery.info_project"), Value: projectID},
				{Label: i18n.T("tui.team.columns.header_label"), Value: fmt.Sprintf("%d", len(finalColumns))},
				{Label: "Mappings", Value: fmt.Sprintf("%d", len(suggestedMappings))},
			}
			if len(selectedPool) > 0 {
				fields = append(fields, views.InfoField{
					Label: "Pool", Value: i18n.Tf("tui.team.discovery.pool_labels_count", len(selectedPool)),
				})
			}
			if tomlPreview != "" {
				fields = append(fields, views.InfoField{
					Label: i18n.T("cmd.tui_team_tracker.config_toml"), Value: tomlPreview,
				})
			}
			return fields
		},
	}

	// ── Build and push the inline wizard ────────────────────────────
	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.tracker.discovery",
		Title: i18n.T("cmd.discovery.title"),
		Steps: []views.WizardStep{
			connStep,
			discoveryStep,
			columnsStep,
			mappingStep,
			poolStep,
		},
		SummaryTargetView:  "team.board",
		SummaryTargetLabel: i18n.T("cmd.discovery.summary.target"),
	})

	tuiShell.PushView(wizard)
}

func formatSyncResultModal(r *views.SyncTrackerResult) string {
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
			fmt.Fprintf(&sb, "  ⚠ %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.errors_header") + "\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "  ✗ %s\n", e)
		}
	}
	return sb.String()
}
