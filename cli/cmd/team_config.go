package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/glyph"
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
	team, err := ResolveTeam(ctx, a)
	if err != nil {
		return err
	}
	if !team.Enabled {
		fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_config.team_not_configured_run_first", cliout.WarnIcon(), theme.Bold.Render("oh team init")))
		return nil
	}
	ctx = teamstate.WithActor(ctx, team.MemberID)
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		fmt.Fprint(a.IO.Out, i18n.Tf("cmd.team_config.team_state_not_cloned_run_first", cliout.WarnIcon(), theme.Bold.Render("oh team init")))
		return nil
	}

	// The questions need someone to answer them: refused before anything is read.
	p, err := newPrompter(a)
	if err != nil {
		return err
	}

	// A config.toml that cannot be read is not an empty configuration to start
	// from: the assistant would write it over the real one.
	teamCfg, err := repo.LoadConfig(ctx)
	if err != nil {
		return errors.New(i18n.Tf("v533b.team_config.unreadable", err))
	}

	fmt.Fprint(a.IO.Out, i18n.T("v533b.team_config.hint_keep"))

	// Step 1 — Choisir le scope
	scope := askSelect(p, i18n.T("cmd.team_config.what_do_you_want_to_configure"), []string{
		i18n.T("cmd.team_config.team_configuration_shared_through_team_state"),
		i18n.T("cmd.team_config.local_configuration_hub_toml_just_for"),
		i18n.T("cmd.team_config.both"),
	}, -1)

	// Step 2 — Choisir le service
	service := askSelect(p, i18n.T("cmd.team_config.which_service"), []string{
		"GitLab",
		"Jira",
		"Figma",
		i18n.T("cmd.team_config.tracker_sync"),
		i18n.T("cmd.team_config.show_everything"),
	}, -1)
	if err := p.failed(); err != nil {
		return err
	}

	configureTeam := scope == 0 || scope == 2
	configureLocal := scope == 1 || scope == 2

	switch service {
	case 0: // GitLab
		return configureGitLab(ctx, a, p, repo, teamCfg, configureTeam, configureLocal)
	case 1: // Jira
		return configureJira(ctx, a, p, repo, teamCfg, configureTeam, configureLocal)
	case 2: // Figma
		return configureFigma(ctx, a, p, repo, teamCfg, configureTeam, configureLocal)
	case 3: // Tracker sync
		return configureTrackerSync(ctx, a, p, repo, teamCfg, configureTeam, configureLocal)
	case 4: // Tout afficher
		return runTeamConfigStatus(cmd, nil)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Service configuration helpers
// ─────────────────────────────────────────────────────────────────────────────

// saveTeamConfig writes the changes made to teamCfg since base (the config as
// it was read) on top of the latest config.toml. Nothing is written once the
// answers were cut short. A setting a colleague changed meanwhile is not
// overwritten: the assistant says so and stops. format wraps another failure
// (it takes the error as its %w).
func saveTeamConfig(ctx context.Context, p *prompter, repo teamstate.ConfigWriter, base, teamCfg *teamstate.TeamConfig, format string) error {
	if err := p.failed(); err != nil {
		return err
	}
	err := repo.UpdateConfigFrom(ctx, base, teamCfg)
	if c, ok := teamstate.IsConfigConflict(err); ok {
		return errors.New(i18n.Tf("v533b.team_config.conflict_cli", strings.Join(c.Keys, ", ")))
	}
	if err != nil {
		return fmt.Errorf(format, err)
	}
	return nil
}

// recommendation asks whether the team uses a service: yes, no, or no
// recommendation at all (the setting stays absent), starting from the current
// one.
func recommendation(p *prompter, prompt string, current *bool) *bool {
	return askTriState(p, prompt, current, i18n.T("v533b.team_config.no_recommendation"))
}

// inheritLabel words the choice of inheriting a setting of the team, with the
// value it has there.
func inheritLabel(teamValue bool) string {
	return i18n.Tf("v533b.team_config.inherit", boolWord(teamValue))
}

func configureGitLab(ctx context.Context, a *app.App, p *prompter, repo teamstate.ConfigWriter, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_configuration", theme.Title.Render(glyph.Current)))

	// ── Config d'équipe ────────────────────────────────────────────────────────
	if configTeam {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_configuration", theme.Bold.Render("→")))

		base := teamstate.CloneConfig(teamCfg)
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["gitlab"]

		// URL
		url := clearable(askInput(p, i18n.T("cmd.team_config.gitlab_url_leave_empty_for_gitlab"), shared.URL))
		if url == "https://gitlab.com" {
			url = "" // store empty to mean "use default"
		}
		shared.URL = url

		// Enabled: a recommendation of the team, or none
		shared.Enabled = recommendation(p, i18n.T("cmd.team_config.does_the_team_use_gitlab"), shared.Enabled)

		// WriteRecommended
		shared.WriteRecommended = askYN(p, i18n.T("cmd.team_config.recommend_write_enabled_for_the_team"), shared.WriteRecommended)

		teamCfg.MCP["gitlab"] = shared

		if err := saveTeamConfig(ctx, p, repo, base, teamCfg, i18n.T("cmd.team_config.saving_the_team_config")); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_team_config_saved", cliout.SuccessIcon()))
	}

	// ── Config locale ──────────────────────────────────────────────────────────
	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))
		edits := &localEdits{}

		// Test de connexion avec les credentials actuels du GitLab du MCP
		src := mcpGitLabCredentials(ctx, a, teamCfg)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeGitLab); err != nil {
			if askYN(p, i18n.T("cmd.team_config.set_up_a_new_gitlab_token"), true) {
				tokenKey := strings.TrimSpace(askInput(p, i18n.T("cmd.team_config.key_name_in_the_keychain_e"), a.Config.MCP.Gitlab.Token))
				fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
				if tokenKey != "" {
					edits.set("mcp.gitlab.token_key", tokenKey)
					edits.set("mcp.gitlab.enabled", "true")
				}
				// The token is only used for the server declared for the GitLab MCP.
				mcpURL := strings.TrimSpace(askInput(p, i18n.T("cmd.team_config.gitlab_server_of_the_key"), src.URL))
				if tracker.ServerOf(mcpURL) != "" {
					edits.change(func(c *config.Config) error {
						c.MCP.Gitlab.URL = mcpURL
						return nil
					})
				}
			}
		}

		// write_enabled
		writeEnabled := askYN(p, i18n.T("cmd.team_config.enable_write_enabled_mr_creation_label"), a.Config.MCP.Gitlab.WriteEnabled)
		if writeEnabled != a.Config.MCP.Gitlab.WriteEnabled {
			edits.set("mcp.gitlab.write_enabled", strconv.FormatBool(writeEnabled))
		}

		if err := finishLocal(a, p, edits); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.gitlab_local_config_saved", cliout.SuccessIcon()))
	}

	return nil
}

// finishLocal writes the local changes of an assistant, unless its answers were
// cut short.
func finishLocal(a *app.App, p *prompter, edits *localEdits) error {
	if err := p.failed(); err != nil {
		return err
	}
	if err := edits.apply(a); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.team_config.warning", err))
	}
	return nil
}

func configureJira(ctx context.Context, a *app.App, p *prompter, repo teamstate.ConfigWriter, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_configuration", theme.Title.Render(glyph.Current)))

	if configTeam {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_configuration", theme.Bold.Render("→")))
		base := teamstate.CloneConfig(teamCfg)
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["jira"]

		shared.URL = clearable(askInput(p, i18n.T("cmd.team_config.jira_url_e_g_https_yourcompany"), shared.URL))
		shared.Enabled = recommendation(p, i18n.T("cmd.team_config.does_the_team_use_jira"), shared.Enabled)

		teamCfg.MCP["jira"] = shared

		if err := saveTeamConfig(ctx, p, repo, base, teamCfg, i18n.T("cmd.team_config.saving_the_team_config")); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_team_config_saved", cliout.SuccessIcon()))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))
		edits := &localEdits{}
		src := trackerCredentials(ctx, a, teamCfg)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeJira); err != nil {
			if askYN(p, i18n.T("cmd.team_config.set_up_a_new_jira_token"), true) {
				tokenKey := strings.TrimSpace(askInput(p, i18n.T("cmd.team_config.key_name_in_the_keychain_e_2"), a.Config.MCP.Jira.Token))
				fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
				if tokenKey != "" {
					edits.set("mcp.jira.token_key", tokenKey)
					edits.set("mcp.jira.enabled", "true")
				}
			}
		}
		writeEnabled := askYN(p, i18n.T("cmd.team_config.enable_write_enabled_label_push"), a.Config.MCP.Jira.WriteEnabled)
		if writeEnabled != a.Config.MCP.Jira.WriteEnabled {
			edits.change(func(c *config.Config) error {
				c.MCP.Jira.WriteEnabled = writeEnabled
				return nil
			})
			edits.said = append(edits.said, i18n.Tf("cmd.team_config.updated", config.ConfigPath(), "mcp.jira.write_enabled", strconv.FormatBool(writeEnabled)))
		}
		if err := finishLocal(a, p, edits); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.jira_local_config_saved", cliout.SuccessIcon()))
	}

	return nil
}

func configureFigma(ctx context.Context, a *app.App, p *prompter, repo teamstate.ConfigWriter, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_configuration", theme.Title.Render(glyph.Current)))

	if configTeam {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_configuration", theme.Bold.Render("→")))
		base := teamstate.CloneConfig(teamCfg)
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["figma"]
		shared.Enabled = recommendation(p, i18n.T("cmd.team_config.does_the_team_use_figma"), shared.Enabled)
		// No URL: the Figma server always calls api.figma.com. A URL already
		// recorded is left as it is and ignored.
		teamCfg.MCP["figma"] = shared

		if err := saveTeamConfig(ctx, p, repo, base, teamCfg, i18n.T("cmd.team_config.saving_the_team_config")); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_team_config_saved", cliout.SuccessIcon()))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration", theme.Bold.Render("→")))
		edits := &localEdits{}
		tokenKey := strings.TrimSpace(askInput(p, i18n.T("cmd.team_config.key_name_in_the_keychain_e_3"), a.Config.MCP.Figma.Token))
		if tokenKey != "" {
			fmt.Fprint(out, i18n.Tf("cmd.team_config.to_store_the_token", theme.Bold.Render("oh secrets set "+tokenKey)))
			edits.set("mcp.figma.token_key", tokenKey)
			edits.set("mcp.figma.enabled", "true")
		}
		if err := finishLocal(a, p, edits); err != nil {
			return err
		}
		if tokenKey != "" {
			fmt.Fprint(out, i18n.Tf("cmd.team_config.figma_local_config_saved", cliout.SuccessIcon()))
		}
	}

	return nil
}

func configureTrackerSync(ctx context.Context, a *app.App, p *prompter, repo teamstate.ConfigWriter, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out
	teamID := teamIDOf(ctx, a)

	fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_sync_configuration", theme.Title.Render(glyph.Current)))

	if configTeam {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_configuration", theme.Bold.Render("→")))
		base := teamstate.CloneConfig(teamCfg)

		// Type: Enter keeps the one of the team; a team without one has to say.
		trackerTypes := []string{"gitlab", "jira"}
		currentType := -1
		for i, t := range trackerTypes {
			if teamCfg.Tracker.Type == t {
				currentType = i
			}
		}
		typeIdx := askSelect(p, i18n.T("cmd.team_config.tracker_type"), []string{"GitLab", "Jira"}, currentType)
		if err := p.failed(); err != nil {
			return err
		}
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
		trackerURL := clearable(askInput(p, i18n.T("cmd.team_config.instance_url_e_g_https_gitlab"), defaultURL))
		teamCfg.Tracker.TrackerURL = trackerURL

		// Tracker token — where it is (key and server), said by the user:
		// nothing is filled in on their behalf.
		ask := func(prompt, def string) string { return askInput(p, prompt, def) }
		teamCfg.Tracker.TrackerTokenKey, teamCfg.Tracker.TrackerTokenURL = askTrackerTokenDecl(
			out, ask, declTeam, teamCfg.Tracker.Type, teamCfg.Tracker.TrackerTokenKey, teamCfg.Tracker.TrackerTokenURL)
		if err := p.failed(); err != nil {
			return err
		}

		// Check if token exists, prompt if not
		src := trackerCredentials(ctx, a, teamCfg)
		trackerType := tracker.Type(teamCfg.Tracker.Type)
		fmt.Fprint(out, i18n.T("cmd.team_config.testing_the_connection"))
		if err := testAndDisplayConnection(ctx, out, src, trackerType); err != nil {
			if askYN(p, i18n.T("cmd.team_config.set_up_a_token_for_this"), true) {
				token := askInput(p, i18n.Tf("v533b.team_config.access_token", teamCfg.Tracker.Type), "")
				if token != "" && p.failed() == nil {
					tokenKey := teamCfg.Tracker.TrackerTokenKey
					if tokenKey == "" {
						// No key declared: the default key is where GitLab reads it for
						// the tracker's server; Jira only reads a declared key.
						tokenKey = tracker.DefaultTokenKey(teamCfg.Tracker.Type)
						if trackerType == tracker.TypeJira {
							teamCfg.Tracker.TrackerTokenKey = tokenKey
							fmt.Fprint(out, i18n.Tf("cmd.init.wizard_tracker_decl_key_declared", tokenKey))
						}
					}
					if err := a.Secrets.Set(ctx, tokenKey, token); err != nil {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.token_storage_error", cliout.WarnIcon(), err))
					} else {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.token_stored_in_the_keychain", cliout.SuccessIcon(), tokenKey))
						// Re-test connection
						src = trackerCredentials(ctx, a, teamCfg)
						fmt.Fprint(out, i18n.T("cmd.team_config.testing_the_connection_again"))
						_ = testAndDisplayConnection(ctx, out, src, trackerType)
					}
				}
			}
		}

		// Enabled
		teamCfg.Tracker.Enabled = askYN(p, i18n.T("cmd.team_config.enable_the_tracker_sync_for_the"), teamCfg.Tracker.Enabled)

		// Auto-plan
		teamCfg.Tracker.AutoPlanAssigned = askYN(p, i18n.T("cmd.team_config.auto_create_planned_claims_for_assigned"), teamCfg.Tracker.AutoPlanAssigned)

		// Work-in-progress limit (0 = no limit); writing it also replaces the
		// former max_auto_plan_per_member of the file.
		current := teamCfg.Tracker.InProgressLimit()
		maxStr := strings.TrimSpace(askInput(p, i18n.T("cmd.team_config.max_in_progress_per_member"), strconv.Itoa(current)))
		if n, err := strconv.Atoi(maxStr); err != nil || n < 0 {
			fmt.Fprint(out, i18n.Tf("v533b.team_config.invalid_limit", cliout.WarnIcon(), maxStr, current))
		} else if n != current {
			teamCfg.Tracker.MaxInProgressPerMember = &n
		}

		// Push labels
		teamCfg.Tracker.PushLabels = askYN(p, i18n.T("cmd.team_config.recommend_pushing_labels_to_the_tracker"), teamCfg.Tracker.PushLabels)

		// Auto-sync
		teamCfg.Tracker.AutoSync = askYN(p, i18n.T("cmd.team_config.enable_the_automatic_sync_when_views"), teamCfg.Tracker.AutoSync)

		// Projet tracker par défaut
		fmt.Fprint(out, i18n.Tf("cmd.team_config.default_tracker_project", theme.Bold.Render("→")))
		trackerProject := clearable(askInput(p, i18n.T("cmd.team_config.project_id_or_path_on_the"), teamCfg.Tracker.TrackerProject))
		teamCfg.Tracker.TrackerProject = trackerProject
		if err := p.failed(); err != nil {
			return err
		}

		// ── Discovery: fetch labels/statuses and suggest mappings ─────────
		if trackerProject != "" && teamCfg.Tracker.Type != "" {
			if askYN(p, i18n.T("cmd.team_config.run_the_discovery_of_the_tracker"), true) {
				fmt.Fprint(out, i18n.Tf("cmd.team_config.project_discovery", theme.Bold.Render("→")))

				// Build tracker client for discovery.
				src := trackerCredentials(ctx, a, teamCfg)
				trackerType := tracker.Type(teamCfg.Tracker.Type)
				creds, credErr := tracker.ResolveCredentials(ctx, src, trackerType)
				if credErr != nil {
					fmt.Fprint(out, i18n.Tf("cmd.team_config.missing_credentials", cliout.WarnIcon(), credErr))
				} else {
					t, tErr := tracker.New(creds)
					if tErr != nil {
						fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_error", cliout.WarnIcon(), tErr))
					} else {
						fmt.Fprint(out, i18n.T("cmd.team_config.fetching_labels_statuses"))
						info, discErr := t.DiscoverProject(ctx, trackerProject)
						if discErr != nil {
							fmt.Fprint(out, i18n.Tf("cmd.team_config.discovery_error", cliout.WarnIcon(), discErr))
						} else {
							fmt.Fprint(out, i18n.Tf("cmd.team_config.labels_statuses_discovered", cliout.SuccessIcon(), len(info.Labels), len(info.Statuses)))

							// Suggest mappings: the claim status of the columns of the
							// board of the team ("planned" for the initial column of a
							// board without columns of its own, not "todo").
							existingCols := teamCfg.Board.Columns
							mappings := tracker.SuggestMappings(info, trackerType, existingCols)
							if len(existingCols) == 0 {
								existingCols = teamstate.DefaultBoardConfig().Columns
							}

							if len(mappings) > 0 {
								fmt.Fprint(out, i18n.Tf("cmd.team_config.suggested_mappings", len(mappings)))
								for _, m := range mappings {
									fmt.Fprintf(out, "    %s → %s (%s)\n", m.Source, m.ColumnID, m.Confidence)
								}

								var acceptedMappings []tracker.SuggestedMapping
								if askYN(p, i18n.Tf("cmd.team_config.accept_the_suggested_mappings", len(mappings)), true) {
									acceptedMappings = mappings
								} else {
									fmt.Fprint(out, i18n.T("cmd.team_config.individual_confirmation"))
									for _, m := range mappings {
										label := fmt.Sprintf("    %s → %s (%s)", m.Source, m.ColumnID, m.Confidence)
										if askYN(p, label, true) {
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
									fmt.Fprint(out, i18n.Tf("cmd.team_config.mappings_applied", cliout.SuccessIcon(), len(acceptedMappings)))
								}

								// Suggest pool labels.
								poolLabels := tracker.SuggestPoolLabels(acceptedMappings, teamCfg.Board)
								if len(poolLabels) > 0 {
									fmt.Fprint(out, i18n.Tf("cmd.team_config.pool_tickets", theme.Bold.Render("→")))
									fmt.Fprint(out, i18n.Tf("cmd.team_config.labels_detected_as_ready", strings.Join(poolLabels, ", ")))
									if askYN(p, i18n.T("cmd.team_config.show_these_tickets_on_the_board"), true) {
										teamCfg.Tracker.AutoPlanUnassigned = true
										teamCfg.Tracker.UnassignedLabels = poolLabels
										fmt.Fprint(out, i18n.Tf("cmd.team_config.pool_configured", cliout.SuccessIcon(), strings.Join(poolLabels, ", ")))
									}
								}
							}

							// Suggest new columns.
							newCols := tracker.SuggestNewColumns(info, existingCols)
							if len(newCols) > 0 {
								fmt.Fprint(out, i18n.Tf("cmd.team_config.suggested_columns", theme.Bold.Render("→")))
								for _, c := range newCols {
									if askYN(p, i18n.Tf("cmd.team_config.add_the_column", c.Name, c.Role), true) {
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

		if err := saveTeamConfig(ctx, p, repo, base, teamCfg, i18n.T("cmd.team_config.saving_the_tracker_config")); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.team_tracker_config_saved", cliout.SuccessIcon()))
	}

	if configLocal {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.local_configuration_overrides", theme.Bold.Render("→")))
		fmt.Fprint(out, i18n.T("v533b.team_config.local_hint"))
		edits := &localEdits{team: teamID}
		// the settings asked about are the ones of the section of this team
		// ([teams.tracker]); the default [tracker] and the team's own are
		// merged to know what is in force
		local := a.Config.TeamTracker(teamID)
		merged := a.Config.TrackerFor(teamID)

		// enabled override
		edits.tri("tracker.enabled", local.Enabled,
			askTriState(p, i18n.T("v533b.team_config.local_enabled"), local.Enabled, inheritLabel(teamCfg.Tracker.Enabled)))

		// auto_sync override
		edits.tri("tracker.auto_sync", local.AutoSync,
			askTriState(p, i18n.T("v533b.team_config.local_auto_sync"), local.AutoSync, inheritLabel(teamCfg.Tracker.AutoSync)))

		// push_labels override
		sharedPush := teamCfg.Tracker.PushLabels
		effLocal := tracker.ResolveTrackerConfig(&teamCfg.Tracker, merged)
		writeEnabled := effLocal.WriteEnabled
		if !writeEnabled && sharedPush {
			fmt.Fprint(out, i18n.Tf("cmd.team_config.push_labels_is_recommended_by_the", cliout.WarnIcon()))
			fmt.Fprint(out, i18n.Tf("cmd.team_config.enable_write_enabled_in_mcp_to", teamCfg.Tracker.Type))
		} else if writeEnabled {
			edits.tri("tracker.push_labels", local.PushLabels,
				askTriState(p, i18n.T("v533b.team_config.local_push_labels"), local.PushLabels, inheritLabel(sharedPush)))
		}

		// the personal declaration of the token (key and server together)
		curKey, curServer := merged.TokenDecl()
		key, server := askTrackerTokenDecl(out, func(prompt, def string) string { return askInput(p, prompt, def) },
			declLocal, teamCfg.Tracker.Type, curKey, curServer)
		if key != curKey || server != curServer {
			edits.change(func(c *config.Config) error {
				c.TrackerForWrite(teamID).SetTokenDecl(key, server)
				return nil
			})
		}

		// the real GitLab of the team repository, when its remote is an SSH alias
		var gitlabURL string
		team, _ := ResolveTeam(ctx, a)
		tc := team.ResolvedTeamConfig
		if tc.StateRepo != "" && !teamstate.IsHTTPS(tc.StateRepo) {
			gitlabURL = askTeamGitLabURL(out, func(prompt, def string) string { return askInput(p, prompt, def) }, tc.GitLabURL)
			if gitlabURL != tc.GitLabURL {
				edits.change(func(c *config.Config) error {
					t := c.FindTeam(tc.TeamID)
					if t == nil {
						return fmt.Errorf("%s", i18n.Tf("tui.team.detail.team_not_in_hub", tc.TeamID))
					}
					t.GitLabURL = gitlabURL
					return nil
				})
			}
		}

		if err := finishLocal(a, p, edits); err != nil {
			return err
		}
		fmt.Fprint(out, i18n.Tf("cmd.team_config.tracker_local_config_saved", cliout.SuccessIcon()))
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

	// The team ResolveTeam gives (--team, else the project, else the default team):
	// the same one as `oh team config`.
	team, err := ResolveTeam(ctx, a)
	if err != nil {
		return err
	}
	tc := team.ResolvedTeamConfig

	// Load team-state config (optional — degrades gracefully if not configured).
	// This command only reads: it writes nothing, to the team-state nor to hub.toml.
	var teamCfg *teamstate.TeamConfig
	var issues configIssuer
	if tc.Enabled && tc.StatePath != "" {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			issues = repo
			var err error
			if teamCfg, err = repo.LoadConfig(ctx); err != nil {
				fmt.Fprint(out, i18n.Tf("v533b.team_config.status_unreadable", cliout.WarnIcon(), err))
				teamCfg = nil
			}
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
		fmt.Fprint(out, i18n.Tf("cmd.team_config.not_configured", theme.Subtitle.Render(glyph.Absent)))
		fmt.Fprint(out, i18n.Tf("cmd.team_config.run_to_configure", theme.Bold.Render("oh team config")))
		printTeamConfigStatus(ctx, out, issues, teamCfg)
		return nil
	}

	local := trackerLocalOf(ctx, a)
	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, local)

	printTrackerStatus(out, teamCfg, local, eff)

	// ── Connection tests ─────────────────────────────────────────────────────
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Test de connexion"))
	src := buildCredentialSource(ctx, a, eff)
	trackerType := tracker.Type(teamCfg.Tracker.Type)
	connErr := testAndDisplayConnection(ctx, out, src, trackerType)

	// Test par projet mappé: pas la peine quand la connexion elle-même a échoué
	if connErr == nil && len(teamCfg.Tracker.Projects) > 0 { //nolint:staticcheck // backward compat: deprecated field
		trackerCfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err == nil {
			t, err := tracker.New(trackerCfg)
			if err == nil {
				for hubID, trackerID := range teamCfg.Tracker.Projects { //nolint:staticcheck // backward compat: deprecated field
					projectName, err := t.TestProject(ctx, trackerID)
					if err != nil {
						fmt.Fprintf(out, "  %s %s → %s: %v\n",
							theme.ErrorStyle.Render(glyph.Failure), hubID, trackerID, err)
					} else {
						fmt.Fprintf(out, "  %s %s → %s (%s)\n",
							cliout.SuccessIcon(), hubID, trackerID, projectName)
					}
				}
			}
		}
	}

	printTeamConfigStatus(ctx, out, issues, teamCfg)
	fmt.Fprintln(out)

	return nil
}

// printTeamConfigStatus is the rest of the configuration of the team: the
// notifications (the webhook masked), the claims, the governance and what
// oh doctor finds in config.toml. Nothing when the config is not there.
// configIssuer is the clone of a team as printTeamConfigStatus reads it.
type configIssuer interface {
	ConfigIssues(ctx context.Context) ([]teamstate.ConfigIssue, error)
}

func printTeamConfigStatus(ctx context.Context, out io.Writer, repo configIssuer, cfg *teamstate.TeamConfig) {
	if cfg == nil {
		return
	}
	n := cfg.Notification
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render(i18n.T("v54c.status.notifications")))
	target := i18n.T("v54c.status.no_target")
	switch {
	case len(n.Destinations) > 0:
		target = i18n.Tf("v54c.status.destinations", len(n.Destinations))
	case n.EffectiveURL() != "":
		target = n.EffectiveType() + " · " + teamstate.MaskWebhook(n.EffectiveURL())
	}
	fmt.Fprintf(out, "  enabled:       %s\n  target:        %s\n", fmtBool(n.Enabled), target)

	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render(i18n.T("v54c.status.team")))
	fmt.Fprint(out, i18n.Tf("v54c.status.claims", cfg.Claim.LeaseHours, cfg.Takeover.StaleDays, cfg.Claim.DoneRetentionDays))
	fmt.Fprint(out, i18n.Tf("v54c.status.governance", cfg.Governance.PublishPolicy(), cfg.Governance.ConfigPolicy()))
	if repo != nil {
		issues, err := repo.ConfigIssues(ctx)
		switch {
		case err != nil:
			fmt.Fprint(out, i18n.Tf("v533b.team_config.status_unreadable", cliout.WarnIcon(), err))
		case len(issues) > 0:
			for _, i := range issues {
				fmt.Fprintf(out, "  %s %s\n", cliout.WarnIcon(), i.Detail)
			}
			fmt.Fprint(out, i18n.T("v54c.doctor.team_config.fix_hint")+"\n")
		}
	}
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

	// URL (none for a service whose server has a fixed address)
	if mcpresolve.ServiceHasURL(name) {
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
			fmt.Fprintf(out, "  %s %s\n", cliout.WarnIcon(), n.Message())
		}
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

	// max_in_progress_per_member: the work-in-progress limit (0 = no limit)
	limit := strconv.Itoa(eff.MaxInProgressPerMember)
	if eff.MaxInProgressPerMember == 0 {
		limit = i18n.T("cmd.team_config.status_no_limit")
	}
	fmt.Fprintf(out, "%smax_in_progress: %s  %s\n", indent, limit, sourceLabel(local.MaxInProgressPerMember != nil, "local", "équipe"))

	// token: the key and the server it is declared for, as the cascade reads them
	tokenSrc := sourceLabel(local.TrackerTokenKey != "", "local", "équipe")
	tokenKey := eff.TrackerTokenKey
	if tokenKey == "" {
		tokenKey = i18n.T("cmd.team_config.status_token_not_set")
	}
	tokenURL := eff.TrackerTokenURL
	if tokenURL == "" {
		tokenURL = i18n.T("cmd.team_config.status_token_not_declared")
	}
	fmt.Fprintf(out, "%stoken_key:     %s  %s\n", indent, tokenKey, tokenSrc)
	fmt.Fprintf(out, "%stoken_url:     %s  %s\n", indent, tokenURL, tokenSrc)

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
// config: the effective tracker config (the team config merged with the
// overrides of hub.toml, tracker_token_key and its server included) completed
// with the hub team, the servers of its sources and the key of [mcp.gitlab],
// so that the GitLab token is found with the unified cascade like everywhere
// else, and only for the server of the tracker.
func trackerCredentials(ctx context.Context, a *app.App, teamCfg *teamstate.TeamConfig) tracker.CredentialSource {
	return buildCredentialSource(ctx, a, tracker.ResolveTrackerConfig(&teamCfg.Tracker, trackerLocalOf(ctx, a)))
}

// mcpGitLabCredentials is the credential source for the GitLab of the MCP
// (url of the team recommendation or of hub.toml): the token the MCP server
// would be given.
func mcpGitLabCredentials(ctx context.Context, a *app.App, teamCfg *teamstate.TeamConfig) tracker.CredentialSource {
	svc := teamServiceFor(ctx, a)
	var shared *teamstate.SharedMCPConfig
	if s, ok := teamCfg.MCP["gitlab"]; ok {
		shared = &s
	}
	url := tracker.MCPGitLabURL(a.Config.MCP.Gitlab, shared, nil, svc.ResolveTeam(nil).TeamID)
	return svc.CredentialSource(ctx, tracker.CredentialSource{URL: url}, nil)
}

// testAndDisplayConnection builds a tracker, tests the connection, and prints
// the result. Returns the error if connection failed.
func testAndDisplayConnection(ctx context.Context, out interface{ Write([]byte) (int, error) }, src tracker.CredentialSource, trackerType tracker.Type) error {
	cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
	if err != nil {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.credentials_not_available", theme.ErrorStyle.Render(glyph.Failure), trackerType, err))
		return err
	}
	t, err := tracker.New(cfg)
	if err != nil {
		fmt.Fprint(out, i18n.Tf("cmd.team_config.initialization_failed", theme.ErrorStyle.Render(glyph.Failure), trackerType, err))
		return err
	}
	username, err := t.TestConnection(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %s %s (%s): %v\n",
			theme.ErrorStyle.Render(glyph.Failure), trackerType, cfg.BaseURL, err)
		return err
	}
	fmt.Fprint(out, i18n.Tf("cmd.team_config.connected_to_as", cliout.SuccessIcon(), cfg.BaseURL, username))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Misc helpers
// ─────────────────────────────────────────────────────────────────────────────

func fmtBool(b bool) string {
	if b {
		return cliout.SuccessIcon()
	}
	return theme.Subtitle.Render(glyph.Inactive)
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
