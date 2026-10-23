package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/fsutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

var teamConfigCmd = &cobra.Command{
	Use:   "config",
	Short: i18n.T("cmd.team.config.short"),
	Long:  i18n.T("cmd.team.config.long"),
	RunE:  runTeamConfigWizard,
}

var teamConfigStatusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("cmd.team.config.status.short"),
	RunE:  runTeamConfigStatus,
}

func init() {
	teamCmd.AddCommand(teamConfigCmd)
	teamConfigCmd.AddCommand(teamConfigStatusCmd)
}

// ─────────────────────────────────────────────────────────────────────────────
// Wizard
// ─────────────────────────────────────────────────────────────────────────────

func runTeamConfigWizard(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Step 0 — Vérification du team-state
	if !a.Config.ActiveTeam().Enabled {
		fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_config.team_not_configured_run_first", theme.WarningStyle.Render(theme.IconWarning), theme.Bold.Render("oh team init")))
		return nil
	}
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)
	if !repo.IsCloned() {
		fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_config.team_state_not_cloned_run_first", theme.WarningStyle.Render(theme.IconWarning), theme.Bold.Render("oh team init")))
		return nil
	}

	teamCfg, _ := repo.LoadConfig()
	if teamCfg == nil {
		teamCfg = &teamstate.TeamConfig{}
	}

	// Step 1 — Choisir le scope
	scope := askSelect(a.IO.Out, i18n.T("cmd.team_config.what_do_you_want_to_configure"), []string{
		i18n.T("cmd.team_config.team_configuration_shared_through_team_state"),
		i18n.T("cmd.team_config.local_configuration_hub_toml_just_for"),
		i18n.T("cmd.team_config.both"),
	})

	// Step 2 — Choisir le service
	service := askSelect(a.IO.Out, i18n.T("cmd.team_config.which_service"), []string{
		"GitLab",
		"Jira",
		"Figma",
		i18n.T("cmd.team_config.tracker_sync"),
		i18n.T("cmd.team_config.show_everything"),
	})

	configureTeam := scope == 0 || scope == 2
	configureLocal := scope == 1 || scope == 2

	switch service {
	case 0: // GitLab
		if err := configureGitLab(ctx, a, repo, teamCfg, configureTeam, configureLocal); err != nil {
			return err
		}
	case 1: // Jira
		if err := configureJira(ctx, a, repo, teamCfg, configureTeam, configureLocal); err != nil {
			return err
		}
	case 2: // Figma
		if err := configureFigma(ctx, a, repo, teamCfg, configureTeam, configureLocal); err != nil {
			return err
		}
	case 3: // Tracker sync
		if err := configureTrackerSync(ctx, a, repo, teamCfg, configureTeam, configureLocal); err != nil {
			return err
		}
	case 4: // Tout afficher
		return runTeamConfigStatus(cmd, nil)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Service configuration helpers
// ─────────────────────────────────────────────────────────────────────────────

func configureGitLab(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_configuration", theme.Title.Render("●")))

	// ── Config d'équipe ────────────────────────────────────────────────────────
	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))

		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["gitlab"]

		// URL
		url := askInput(out, i18n.T("cmd.team_config.gitlab_url_leave_empty_for_gitlab"), shared.URL)
		if url == "https://gitlab.com" {
			url = "" // store empty to mean "use default"
		}
		shared.URL = url

		// Enabled
		enabledRec := askYN(out, i18n.T("cmd.team_config.does_the_team_use_gitlab"), shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec

		// WriteRecommended
		shared.WriteRecommended = askYN(out, i18n.T("cmd.team_config.recommend_write_enabled_for_the_team"), shared.WriteRecommended)

		teamCfg.MCP["gitlab"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_config.saving_the_team_config"), err)
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_team_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	// ── Config locale ──────────────────────────────────────────────────────────
	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))

		// Test de connexion avec les credentials actuels
		src := trackerCredentials(a, teamCfg)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeGitLab); err != nil {
			if askYN(out, i18n.T("cmd.team_config.set_up_a_new_gitlab_token"), true) {
				tokenKey := askInput(out, i18n.T("cmd.team_config.key_name_in_the_keychain_e"), a.Config.MCP.Gitlab.Token)
				fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
				writeLocalLog(config.ConfigPath(), "mcp.gitlab.token_key", tokenKey)
				writeLocalLog(config.ConfigPath(), "mcp.gitlab.enabled", "true")
			}
		}

		// write_enabled
		writeEnabled := askYN(out, i18n.T("cmd.team_config.enable_write_enabled_mr_creation_label"), a.Config.MCP.Gitlab.WriteEnabled)
		writeLocalLog(config.ConfigPath(), "mcp.gitlab.write_enabled", fmt.Sprintf("%v", writeEnabled))

		fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_local_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	return nil
}

func configureJira(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_configuration", theme.Title.Render("●")))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["jira"]

		url := askInput(out, i18n.T("cmd.team_config.jira_url_e_g_https_yourcompany"), shared.URL)
		shared.URL = url

		enabledRec := askYN(out, i18n.T("cmd.team_config.does_the_team_use_jira"), shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec

		teamCfg.MCP["jira"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_config.saving_the_team_config"), err)
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_team_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))
		src := trackerCredentials(a, teamCfg)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeJira); err != nil {
			if askYN(out, i18n.T("cmd.team_config.set_up_a_new_jira_token"), true) {
				tokenKey := askInput(out, i18n.T("cmd.team_config.key_name_in_the_keychain_e_2"), a.Config.MCP.Jira.Token)
				fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
				writeLocalLog(config.ConfigPath(), "mcp.jira.token_key", tokenKey)
				writeLocalLog(config.ConfigPath(), "mcp.jira.enabled", "true")
			}
		}
		writeEnabled := askYN(out, i18n.T("cmd.team_config.enable_write_enabled_label_push"), a.Config.MCP.Jira.WriteEnabled)
		writeLocalLog(config.ConfigPath(), "mcp.jira.write_enabled", fmt.Sprintf("%v", writeEnabled))
		fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_local_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	return nil
}

func configureFigma(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_configuration", theme.Title.Render("●")))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["figma"]
		enabledRec := askYN(out, i18n.T("cmd.team_config.does_the_team_use_figma"), shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec
		// URL optionnelle pour Figma (self-hosted)
		url := askInput(out, i18n.T("cmd.team_config.figma_url_leave_empty_for_api"), shared.URL)
		shared.URL = url
		teamCfg.MCP["figma"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_config.saving_the_team_config"), err)
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_team_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))
		tokenKey := askInput(out, i18n.T("cmd.team_config.key_name_in_the_keychain_e_3"), a.Config.MCP.Figma.Token)
		if tokenKey != "" {
			fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
			writeLocalLog(config.ConfigPath(), "mcp.figma.token_key", tokenKey)
			writeLocalLog(config.ConfigPath(), "mcp.figma.enabled", "true")
			fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_local_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
		}
	}

	return nil
}

func configureTrackerSync(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_sync_configuration", theme.Title.Render("●")))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))

		// Type
		typeIdx := askSelect(out, i18n.T("cmd.team_config.tracker_type"), []string{"GitLab", "Jira"})
		trackerTypes := []string{"gitlab", "jira"}
		teamCfg.Tracker.Type = trackerTypes[typeIdx]

		// Tracker URL
		defaultURL := teamCfg.Tracker.TrackerURL
		if defaultURL == "" {
			// Suggest MCP URL as default if available
			if teamCfg.Tracker.Type == "gitlab" && a.Config.MCP.Gitlab.URL != "" {
				defaultURL = a.Config.MCP.Gitlab.URL
			} else if teamCfg.Tracker.Type == "jira" && a.Config.MCP.Jira.URL != "" {
				defaultURL = a.Config.MCP.Jira.URL
			}
		}
		trackerURL := askInput(out, i18n.T("cmd.team_config.instance_url_e_g_https_gitlab"), defaultURL)
		teamCfg.Tracker.TrackerURL = trackerURL

		// Tracker token — determine the keychain key
		trackerTokenKey := teamCfg.Tracker.TrackerTokenKey
		if trackerTokenKey == "" {
			trackerTokenKey = "openhub.tracker." + teamCfg.Tracker.Type + ".token"
		}
		teamCfg.Tracker.TrackerTokenKey = trackerTokenKey

		// Check if token exists, prompt if not
		src := trackerCredentials(a, teamCfg)
		trackerType := tracker.Type(teamCfg.Tracker.Type)
		fmt.Fprint(out, i18n.T("cmd.team_config.testing_the_connection"))
		if err := testAndDisplayConnection(ctx, out, src, trackerType); err != nil {
			if askYN(out, i18n.T("cmd.team_config.set_up_a_token_for_this"), true) {
				token := askInput(out, "Token d'accès "+teamCfg.Tracker.Type, "")
				if token != "" {
					if err := a.Secrets.Set(ctx, trackerTokenKey, token); err != nil {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.token_storage_error", theme.WarningStyle.Render(theme.IconWarning), err))
					} else {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.token_stored_in_the_keychain", theme.SuccessStyle.Render(theme.IconSuccess), trackerTokenKey))
						// Re-test connection
						src = trackerCredentials(a, teamCfg)
						fmt.Fprint(out, i18n.T("cmd.team_config.testing_the_connection_again"))
						_ = testAndDisplayConnection(ctx, out, src, trackerType)
					}
				}
			}
		}

		// Enabled
		teamCfg.Tracker.Enabled = askYN(out, i18n.T("cmd.team_config.enable_the_tracker_sync_for_the"), teamCfg.Tracker.Enabled)

		// Auto-plan
		teamCfg.Tracker.AutoPlanAssigned = askYN(out, i18n.T("cmd.team_config.auto_create_planned_claims_for_assigned"), teamCfg.Tracker.AutoPlanAssigned)
		if teamCfg.Tracker.AutoPlanAssigned {
			maxStr := askInput(out, i18n.T("cmd.team_config.auto_plan_claims_limit_per_member"), fmt.Sprintf("%d", max(teamCfg.Tracker.MaxAutoPlanPerMember, 5)))
			if n := parseInt(maxStr, 5); n > 0 {
				teamCfg.Tracker.MaxAutoPlanPerMember = n
			}
		}

		// Push labels
		teamCfg.Tracker.PushLabels = askYN(out, i18n.T("cmd.team_config.recommend_pushing_labels_to_the_tracker"), teamCfg.Tracker.PushLabels)

		// Auto-sync
		teamCfg.Tracker.AutoSync = askYN(out, i18n.T("cmd.team_config.enable_the_automatic_sync_when_views"), teamCfg.Tracker.AutoSync)

		// Projet tracker par défaut
		fmt.Fprint(out, i18n.Tf("cmd.team_config.default_tracker_project", theme.Bold.Render("→")))
		trackerProject := askInput(out, i18n.T("cmd.team_config.project_id_or_path_on_the"), teamCfg.Tracker.TrackerProject)
		teamCfg.Tracker.TrackerProject = trackerProject

		// ── Discovery: fetch labels/statuses and suggest mappings ─────────
		if trackerProject != "" && teamCfg.Tracker.Type != "" {
			if askYN(out, i18n.T("cmd.team_config.run_the_discovery_of_the_tracker"), true) {
				fmt.Fprint(out, i18n.Tf("cmd.team_config.project_discovery", theme.Bold.Render("→")))

				// Build tracker client for discovery.
				src := trackerCredentials(a, teamCfg)
				trackerType := tracker.Type(teamCfg.Tracker.Type)
				creds, credErr := tracker.ResolveCredentials(ctx, src, trackerType)
				if credErr != nil {
					fmt.Fprint(out, i18n.Tf("cmd.team_config.missing_credentials", theme.WarningStyle.Render(theme.IconWarning), credErr))
				} else {
					t, tErr := tracker.New(creds)
					if tErr != nil {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_error", theme.WarningStyle.Render(theme.IconWarning), tErr))
					} else {
						fmt.Fprint(out, i18n.T("cmd.team_config.fetching_labels_statuses"))
						info, discErr := t.DiscoverProject(ctx, trackerProject)
						if discErr != nil {
							fmt.Fprint(out, i18n.Tf("cmd.team_config.discovery_error", theme.WarningStyle.Render(theme.IconWarning), discErr))
						} else {
							fmt.Fprint(out, i18n.Tf("cmd.team_config.labels_statuses_discovered", theme.SuccessStyle.Render(theme.IconSuccess), len(info.Labels), len(info.Statuses)))

							// Suggest mappings.
							existingCols := teamCfg.Board.Columns
							if len(existingCols) == 0 {
								existingCols = teamstate.DefaultBoardConfig().Columns
							}
							mappings := tracker.SuggestMappings(info, trackerType, existingCols)

							if len(mappings) > 0 {
								fmt.Fprint(out, i18n.Tf("cmd.team_config.suggested_mappings", len(mappings)))
								for _, m := range mappings {
									fmt.Fprintf(out, "    %s → %s (%s)\n", m.Source, m.ColumnID, m.Confidence)
								}

								var acceptedMappings []tracker.SuggestedMapping
								if askYN(out, i18n.Tf("cmd.team_config.accept_the_suggested_mappings", len(mappings)), true) {
									acceptedMappings = mappings
								} else {
									fmt.Fprint(out, i18n.T("cmd.team_config.individual_confirmation"))
									for _, m := range mappings {
										label := fmt.Sprintf("    %s → %s (%s)", m.Source, m.ColumnID, m.Confidence)
										if askYN(out, label, true) {
											acceptedMappings = append(acceptedMappings, m)
										}
									}
								}

								// Apply accepted mappings.
								if len(acceptedMappings) > 0 {
									if trackerType == tracker.TypeGitLab {
										if teamCfg.Tracker.LabelStatusMapping == nil {
											teamCfg.Tracker.LabelStatusMapping = make(map[string]string)
										}
										for _, m := range acceptedMappings {
											teamCfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
										}
									} else {
										if teamCfg.Tracker.StatusMapping == nil {
											teamCfg.Tracker.StatusMapping = make(map[string]string)
										}
										for _, m := range acceptedMappings {
											if m.Origin == "status" {
												teamCfg.Tracker.StatusMapping[m.Source] = m.ColumnID
											} else {
												if teamCfg.Tracker.LabelStatusMapping == nil {
													teamCfg.Tracker.LabelStatusMapping = make(map[string]string)
												}
												teamCfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
											}
										}
									}
									fmt.Fprint(out, i18n.Tf("cmd.team_config.mappings_applied", theme.SuccessStyle.Render(theme.IconSuccess), len(acceptedMappings)))
								}

								// Suggest pool labels.
								boardCfg := teamCfg.Board
								if !boardCfg.HasCustomColumns() {
									boardCfg = teamstate.DefaultBoardConfig()
								}
								poolLabels := tracker.SuggestPoolLabels(acceptedMappings, boardCfg)
								if len(poolLabels) > 0 {
									fmt.Fprintf(out, "\n%s Tickets à prendre (pool)\n", theme.Bold.Render("→"))
									fmt.Fprint(out, i18n.Tf("cmd.team_config.labels_detected_as_ready", strings.Join(poolLabels, ", ")))
									if askYN(out, i18n.T("cmd.team_config.show_these_tickets_on_the_board"), true) {
										teamCfg.Tracker.AutoPlanUnassigned = true
										teamCfg.Tracker.UnassignedLabels = poolLabels
										fmt.Fprint(out, i18n.Tf("cmd.team_config.pool_configured", theme.SuccessStyle.Render(theme.IconSuccess), strings.Join(poolLabels, ", ")))
									}
								}
							}

							// Suggest new columns.
							newCols := tracker.SuggestNewColumns(info, existingCols)
							if len(newCols) > 0 {
								fmt.Fprint(out, i18n.Tf("cmd.team_config.suggested_columns", theme.Bold.Render("→")))
								for _, c := range newCols {
									if askYN(out, i18n.Tf("cmd.team_config.add_the_column", c.Name, c.Role), true) {
										existingCols = append(existingCols, c)
									}
								}
								teamCfg.Board.Columns = existingCols
							}
						}
					}
				}
			}
		}

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_config.saving_the_tracker_config"), err)
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_tracker_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration_overrides", theme.Bold.Render("→")))
		fmt.Fprint(out, i18n.T("cmd.team_config.leave_empty_inherit_the_team_config"))

		// enabled override
		if askYN(out, i18n.T("cmd.team_config.disable_the_tracker_sync_locally"), false) {
			writeLocalLog(config.ConfigPath(), "tracker.enabled", "false")
		} else {
			writeLocalLog(config.ConfigPath(), "tracker.enabled", "true")
		}

		// auto_sync override
		sharedAutoSync := teamCfg.Tracker.AutoSync
		if askYN(out, i18n.Tf("cmd.team_config.automatic_sync_team", sharedAutoSync), sharedAutoSync) {
			writeLocalLog(config.ConfigPath(), "tracker.auto_sync", "true")
		} else {
			writeLocalLog(config.ConfigPath(), "tracker.auto_sync", "false")
		}

		// push_labels override
		sharedPush := teamCfg.Tracker.PushLabels
		effLocal := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker)
		writeEnabled := effLocal.WriteEnabled
		if !writeEnabled && sharedPush {
			fmt.Fprint(out, i18n.Tf("cmd.team_config.push_labels_is_recommended_by_the", theme.WarningStyle.Render(theme.IconWarning)))
			fmt.Fprint(out, i18n.Tf("cmd.team_config.enable_write_enabled_in_mcp_to", teamCfg.Tracker.Type))
		} else if writeEnabled {
			if askYN(out, i18n.Tf("cmd.team_config.push_labels_prompt", sharedPush), sharedPush) {
				writeLocalLog(config.ConfigPath(), "tracker.push_labels", "true")
			} else {
				writeLocalLog(config.ConfigPath(), "tracker.push_labels", "false")
			}
		}

		fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_local_config_saved", theme.SuccessStyle.Render(theme.IconSuccess)))
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Status command
// ─────────────────────────────────────────────────────────────────────────────

func runTeamConfigStatus(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()
	out := a.IO.Out

	fmt.Fprintf(out, "\n%s\n\n", theme.Title.Render("Configuration MCP & Tracker"))

	// Resolve team config via active project (project-level config takes priority).
	project, _ := resolveActiveProject(a)
	tc := resolvedTeamConfig(a, project)

	// Load team-state config (optional — degrades gracefully if not configured).
	var teamCfg *teamstate.TeamConfig
	var repo *teamstate.Repo
	if tc.Enabled && tc.StatePath != "" {
		repo = teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			teamCfg, _ = repo.LoadConfig()
		}
	}

	var sharedMCP map[string]teamstate.SharedMCPConfig
	if teamCfg != nil {
		sharedMCP = teamCfg.MCP
	}

	// ── MCP Services ────────────────────────────────────────────────────────────
	mcpServices := []struct {
		name  string
		local config.MCPServerConfig
	}{
		{"gitlab", a.Config.MCP.Gitlab},
		{"jira", a.Config.MCP.Jira},
		{"figma", a.Config.MCP.Figma},
		{"gslides", a.Config.MCP.Gslides},
	}

	for _, svc := range mcpServices {
		var shared *teamstate.SharedMCPConfig
		if sharedMCP != nil {
			if s, ok := sharedMCP[svc.name]; ok {
				shared = &s
			}
		}
		eff := mcpresolve.Resolve(shared, svc.local)
		printMCPStatus(out, svc.name, shared, svc.local, eff)
	}

	// ── Tracker sync ────────────────────────────────────────────────────────────
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Tracker Sync"))

	if teamCfg == nil || teamCfg.Tracker.Type == "" {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.not_configured", theme.Subtitle.Render("○")))
		fmt.Fprint(out, i18n.Tf("cmd.team_config.run_to_configure", theme.Bold.Render("oh team config")))
		return nil
	}

	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker)

	printTrackerStatus(out, teamCfg, a.Config.Tracker, eff)

	// ── Connection tests ─────────────────────────────────────────────────────
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Test de connexion"))
	src := buildCredentialSource(a, eff)
	trackerType := tracker.Type(teamCfg.Tracker.Type)
	_ = testAndDisplayConnection(ctx, out, src, trackerType)

	// Test par projet mappé
	if len(teamCfg.Tracker.Projects) > 0 { //nolint:staticcheck // backward compat: deprecated field
		trackerCfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err == nil {
			t, err := tracker.New(trackerCfg)
			if err == nil {
				for hubID, trackerID := range teamCfg.Tracker.Projects { //nolint:staticcheck // backward compat: deprecated field
					projectName, err := t.TestProject(ctx, trackerID)
					if err != nil {
						fmt.Fprintf(out, "  %s %s → %s: %v\n",
							theme.ErrorStyle.Render("✗"), hubID, trackerID, err)
					} else {
						fmt.Fprintf(out, "  %s %s → %s (%s)\n",
							theme.SuccessStyle.Render(theme.IconSuccess), hubID, trackerID, projectName)
					}
				}
			}
		}
	}

	fmt.Fprintln(out)

	// ── Orphan member check ──────────────────────────────────────────────────
	if repo != nil && repo.IsCloned() {
		checkAndCleanOrphanMembers(ctx, a, out, repo)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Display helpers
// ─────────────────────────────────────────────────────────────────────────────

func printMCPStatus(out interface{ Write([]byte) (int, error) },
	name string,
	shared *teamstate.SharedMCPConfig,
	local config.MCPServerConfig,
	eff mcpresolve.EffectiveConfig,
) {
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("MCP "+strings.ToUpper(name[:1])+name[1:]))

	// Enabled
	enabledStr := fmtBool(eff.Enabled)
	insight := ""
	if shared != nil && shared.Enabled != nil && *shared.Enabled != local.Enabled && local.Token != "" {
		insight = fmt.Sprintf(" %s (équipe: %v)", theme.Subtitle.Render("ℹ"), *shared.Enabled)
	}
	fmt.Fprintf(out, "  enabled:       %s%s\n", enabledStr, insight)

	// URL
	urlSource := "défaut"
	urlVal := eff.URL
	if urlVal == "" {
		urlVal = "(défaut SaaS)"
	}
	if local.URL != "" {
		urlSource = "local"
	} else if shared != nil && shared.URL != "" && eff.EnforcedURLPending == "" {
		urlSource = "équipe"
	}
	fmt.Fprintf(out, "  url:           %s  %s\n", urlVal, theme.Subtitle.Render("("+urlSource+")"))
	if eff.EnforcedURLPending != "" {
		n := enforcedURLNotice{Service: name, URL: eff.EnforcedURLPending, Pinned: local.TrustedURL, Reason: eff.EnforcedURLReason}
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), n.Message())
	}

	// Token
	tokenDisplay := "(non configuré)"
	if local.Token != "" {
		tokenDisplay = "****" + last4(local.Token) + "  (keychain: " + local.Token + ")"
	}
	fmt.Fprintf(out, "  token:         %s\n", tokenDisplay)

	// Write
	writeStr := fmtBool(eff.WriteEnabled)
	writeInsight := ""
	if eff.WriteRecommended && !eff.WriteEnabled {
		writeInsight = i18n.Tf("cmd.team_config.team_recommends_true", theme.Subtitle.Render("ℹ"))
	}
	fmt.Fprintf(out, "  write_enabled: %s%s\n", writeStr, writeInsight)
}

func printTrackerStatus(out interface{ Write([]byte) (int, error) },
	teamCfg *teamstate.TeamConfig,
	local config.TrackerLocalConfig,
	eff tracker.EffectiveTrackerConfig,
) {
	indent := "  "

	fmt.Fprintf(out, "%stype:          %s  %s\n", indent, eff.Type, theme.Subtitle.Render("(équipe)"))

	// enabled
	enabledSrc := sourceLabel(local.Enabled != nil, "local", "équipe")
	fmt.Fprintf(out, "%senabled:       %s  %s\n", indent, fmtBool(eff.Enabled), enabledSrc)

	// auto_sync
	autoSyncSrc := sourceLabel(local.AutoSync != nil, "local", "équipe")
	fmt.Fprintf(out, "%sauto_sync:     %s  %s\n", indent, fmtBool(eff.AutoSync), autoSyncSrc)

	// push_labels
	pushSrc := sourceLabel(local.PushLabels != nil, "local", "équipe")
	pushInsight := ""
	if eff.LocalOverrides.PushLabels {
		pushInsight = i18n.Tf("cmd.team_config.team_recommends_val", theme.Subtitle.Render("ℹ"), eff.SharedPushLabels)
	}
	fmt.Fprintf(out, "%spush_labels:   %s  %s%s\n", indent, fmtBool(eff.PushLabels), pushSrc, pushInsight)

	// auto_plan
	autoPlanSrc := sourceLabel(local.AutoPlanAssigned != nil, "local", "équipe")
	fmt.Fprintf(out, "%sauto_plan:     %s  %s\n", indent, fmtBool(eff.AutoPlanAssigned), autoPlanSrc)

	// Mappings
	if len(teamCfg.Tracker.Projects) > 0 { //nolint:staticcheck // backward compat: deprecated field
		fmt.Fprintf(out, "%smappings:\n", indent)
		for hubID, trackerID := range teamCfg.Tracker.Projects { //nolint:staticcheck // backward compat: deprecated field
			pattern := teamCfg.Tracker.TicketPatterns[hubID] //nolint:staticcheck // backward compat: deprecated field
			fmt.Fprintf(out, "%s  %s → %s  (pattern: %s)\n", indent, hubID, trackerID, pattern)
		}
	}
}

// trackerCredentials is the credential source of the tracker of the team
// config: besides the keychain it knows the hub team and the key of
// [mcp.gitlab], so that the GitLab token is found with the unified cascade
// (the team step included) like everywhere else.
func trackerCredentials(a *app.App, teamCfg *teamstate.TeamConfig) tracker.CredentialSource {
	return teamService(a).CredentialSource(tracker.CredentialSource{
		URL:      teamCfg.Tracker.TrackerURL,
		TokenKey: teamCfg.Tracker.TrackerTokenKey,
	}, nil)
}

// testAndDisplayConnection builds a tracker, tests the connection, and prints
// the result. Returns the error if connection failed.
func testAndDisplayConnection(ctx context.Context, out interface{ Write([]byte) (int, error) }, src tracker.CredentialSource, trackerType tracker.Type) error {
	cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
	if err != nil {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.credentials_not_available", theme.ErrorStyle.Render("✗"), trackerType, err))
		return err
	}
	t, err := tracker.New(cfg)
	if err != nil {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.initialization_failed", theme.ErrorStyle.Render("✗"), trackerType, err))
		return err
	}
	username, err := t.TestConnection(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %s %s (%s): %v\n",
			theme.ErrorStyle.Render("✗"), trackerType, cfg.BaseURL, err)
		return err
	}
	fmt.Fprint(out, i18n.Tf("cmd.team_config.connected_to_as", theme.SuccessStyle.Render(theme.IconSuccess), cfg.BaseURL, username))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Orphan member detection + cleanup
// ─────────────────────────────────────────────────────────────────────────────

// checkAndCleanOrphanMembers detects members in members.toml that are not
// referenced as member_id in any active configuration (hub or project), displays
// an insight for each, and offers to delete or merge them interactively.
func checkAndCleanOrphanMembers(ctx context.Context, a *app.App, out interface{ Write([]byte) (int, error) }, repo *teamstate.Repo) {
	members, err := repo.ListMembers()
	if err != nil || len(members) == 0 {
		return
	}

	used := collectUsedMemberIDs(ctx, a)

	var orphans []teamstate.Member
	for _, m := range members {
		if !used[m.ID] {
			orphans = append(orphans, m)
		}
	}
	if len(orphans) == 0 {
		return
	}

	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Membres orphelins"))
	for _, m := range orphans {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.member_gitlab_is_in_the_registry", theme.Subtitle.Render("ℹ"), m.ID, m.DisplayName, m.GitLabUsername))
	}

	// Offer cleanup for each orphan
	for _, orphan := range orphans {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.what_to_do_with", orphan.ID))
		choice := askSelect(out, "", []string{
			i18n.T("cmd.team_config.remove_from_the_registry"),
			i18n.T("cmd.team_config.merge_with_an_active_member"),
			"Ignorer",
		})

		switch choice {
		case 0: // Supprimer
			if err := repo.RemoveMember(ctx, orphan.ID); err != nil {
				fmt.Fprintf(out, "  %s Erreur: %v\n", theme.ErrorStyle.Render("✗"), err)
				continue
			}
			fmt.Fprintf(out, "  %s Membre %q supprimé\n", theme.SuccessStyle.Render(theme.IconSuccess), orphan.ID)

		case 1: // Fusionner
			if err := mergeOrphanMember(ctx, out, repo, orphan, members); err != nil {
				fmt.Fprint(out, i18n.Tf("cmd.team_config.merge_cancelled", theme.WarningStyle.Render(theme.IconWarning), err))
			}

		case 2: // Ignorer
			fmt.Fprintf(out, "  Ignoré\n")
		}
	}
}

// mergeOrphanMember guides the user through merging an orphan member into an
// active member. Non-empty fields of the orphan are offered to fill gaps in the
// active member; when both have conflicting values the user chooses which to keep.
func mergeOrphanMember(ctx context.Context, out interface{ Write([]byte) (int, error) }, repo *teamstate.Repo, orphan teamstate.Member, allMembers []teamstate.Member) error {
	// Build list of candidates (active members, i.e. not the orphan itself)
	var candidates []teamstate.Member
	for _, m := range allMembers {
		if m.ID != orphan.ID {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return errors.New(i18n.T("cmd.team_config.no_other_member_available_for_the"))
	}

	// Let the user pick the target
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = fmt.Sprintf("%s (%s)", c.ID, c.DisplayName)
	}
	idx := askSelect(out, i18n.T("cmd.team_config.merge_with_which_member"), names)
	target := candidates[idx]

	// Merge: for each field, if target is empty and orphan is not → copy automatically.
	// If both have different non-empty values → ask which to keep.
	merged := target

	merged.DisplayName = resolveField(out, "display_name", target.DisplayName, orphan.DisplayName)
	merged.GitLabUsername = resolveField(out, "gitlab_username", target.GitLabUsername, orphan.GitLabUsername)
	merged.MattermostUsername = resolveField(out, "mattermost_username", target.MattermostUsername, orphan.MattermostUsername)
	merged.Role = resolveField(out, "role", target.Role, orphan.Role)

	// Preview
	fmt.Fprint(out, i18n.Tf("cmd.team_config.merge_result_for", merged.ID))
	fmt.Fprintf(out, "    display_name:        %s\n", merged.DisplayName)
	fmt.Fprintf(out, "    gitlab_username:     %s\n", merged.GitLabUsername)
	fmt.Fprintf(out, "    mattermost_username: %s\n", merged.MattermostUsername)
	fmt.Fprintf(out, "    role:                %s\n", merged.Role)

	if !askYN(out, i18n.T("cmd.team_config.confirm_the_merge"), true) {
		return errors.New(i18n.T("cmd.team_config.merge_cancelled_by_the_user"))
	}

	// Apply: update target member, remove orphan
	if err := repo.UpdateMember(ctx, merged); err != nil {
		return fmt.Errorf(i18n.T("cmd.team_config.updating_the_member"), err)
	}
	if err := repo.RemoveMember(ctx, orphan.ID); err != nil {
		return fmt.Errorf(i18n.T("cmd.team_config.removing_the_orphan"), err)
	}

	fmt.Fprint(out, i18n.Tf("cmd.team_config.merge_done", theme.SuccessStyle.Render(theme.IconSuccess)))

	fmt.Fprint(out, i18n.Tf("cmd.team_config.members_merged", theme.SuccessStyle.Render(theme.IconSuccess)))
	return nil
}

// resolveField returns the value to use for a member field after merge.
// If only one side has a non-empty value → use it automatically.
// If both differ → ask the user to choose.
func resolveField(out interface{ Write([]byte) (int, error) }, fieldName, target, source string) string {
	target = strings.TrimSpace(target)
	source = strings.TrimSpace(source)

	switch {
	case target == "" && source == "":
		return ""
	case target == "" && source != "":
		return source // auto-fill from orphan
	case target != "" && source == "":
		return target // target already has a value
	case target == source:
		return target // same value, no conflict
	default:
		// Conflict — ask the user
		fmt.Fprint(out, i18n.Tf("cmd.team_config.conflict_on_1_active_member_2", fieldName, target, source))
		choice := askSelect(out, i18n.T("cmd.team_config.which_value_to_keep"), []string{target, source})
		if choice == 1 {
			return source
		}
		return target
	}
}

// coerceValue converts string representations of booleans and integers
// to their native Go types so that TOML marshaling produces the correct
// syntax (e.g., enabled = true instead of enabled = "true").
func coerceValue(s string) interface{} {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return s
}

// writeLocalLog calls writeLocal and logs any error to stderr.
// Used by interactive config commands where write failures are non-fatal
// but should be visible to the user.
func writeLocalLog(configPath, keyPath, value string) {
	if err := writeLocal(configPath, keyPath, value); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
	}
}

// reading the file, updating the value, and rewriting. This is intentionally
// simple — for complex edits the user can edit hub.toml directly.
func writeLocal(configPath, keyPath, value string) error {
	// Read current content
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", configPath, err)
	}

	// Parse existing TOML into a nested map
	var tree map[string]interface{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &tree); err != nil {
			return fmt.Errorf("parsing %s: %w", configPath, err)
		}
	}
	if tree == nil {
		tree = make(map[string]interface{})
	}

	// Navigate the key path and set the value
	parts := strings.Split(keyPath, ".")
	current := tree
	for i := 0; i < len(parts)-1; i++ {
		child, ok := current[parts[i]]
		if !ok {
			// Create intermediate table
			newTable := make(map[string]interface{})
			current[parts[i]] = newTable
			current = newTable
		} else if childMap, ok := child.(map[string]interface{}); ok {
			current = childMap
		} else {
			return fmt.Errorf(i18n.T("cmd.team_config.is_not_a_toml_table"), strings.Join(parts[:i+1], "."))
		}
	}
	current[parts[len(parts)-1]] = coerceValue(value)

	// Marshal and write atomically
	out, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team_config.marshaling_toml"), err)
	}
	if err := fsutil.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}
	fmt.Printf("  %s mis à jour: %s = %s\n", configPath, keyPath, value)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Interactive prompt helpers (stdin)
// ─────────────────────────────────────────────────────────────────────────────

func askInput(out interface{ Write([]byte) (int, error) }, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(out, "  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Fprintf(out, "  %s: ", prompt)
	}
	var input string
	_, _ = fmt.Scanln(&input)
	if input == "" {
		return defaultVal
	}
	return input
}

func askYN(out interface{ Write([]byte) (int, error) }, prompt string, defaultYes bool) bool {
	fmt.Fprintf(out, "  %s %s ", prompt, yesNoHint(defaultYes))
	var input string
	_, _ = fmt.Scanln(&input)
	return confirmed(input, defaultYes)
}

func askSelect(out interface{ Write([]byte) (int, error) }, prompt string, options []string) int {
	fmt.Fprintf(out, "\n  %s\n", prompt)
	for i, opt := range options {
		fmt.Fprintf(out, "    [%d] %s\n", i+1, opt)
	}
	fmt.Fprint(out, i18n.T("cmd.prompt.choice"))
	var input string
	_, _ = fmt.Scanln(&input)
	if input == "" {
		return 0
	}
	n := parseInt(input, 1)
	if n < 1 || n > len(options) {
		return 0
	}
	return n - 1
}

// ─────────────────────────────────────────────────────────────────────────────
// Misc helpers
// ─────────────────────────────────────────────────────────────────────────────

func fmtBool(b bool) string {
	if b {
		return theme.SuccessStyle.Render(theme.IconSuccess)
	}
	return theme.Subtitle.Render("✗")
}

func sourceLabel(isLocal bool, localLabel, teamLabel string) string {
	if isLocal {
		return theme.Subtitle.Render("(" + localLabel + ")")
	}
	return theme.Subtitle.Render("(" + teamLabel + ")")
}

func last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

func parseInt(s string, dflt int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return dflt
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return dflt
	}
	return n
}
