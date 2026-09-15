package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
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
		fmt.Fprintf(a.IO.Out, "%s Équipe non configurée. Lance d'abord %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			theme.Bold.Render("oh team init"))
		return nil
	}
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)
	if !repo.IsCloned() {
		fmt.Fprintf(a.IO.Out, "%s Team-state non cloné. Lance d'abord %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			theme.Bold.Render("oh team init"))
		return nil
	}

	teamCfg, _ := repo.LoadConfig()
	if teamCfg == nil {
		teamCfg = &teamstate.TeamConfig{}
	}

	// Step 1 — Choisir le scope
	scope := askSelect(a.IO.Out, "Que souhaitez-vous configurer ?", []string{
		"Configuration d'équipe (partagée via team-state)",
		"Configuration locale (hub.toml, juste pour vous)",
		"Les deux",
	})

	// Step 2 — Choisir le service
	service := askSelect(a.IO.Out, "Quel service ?", []string{
		"GitLab",
		"Jira",
		"Figma",
		"Tracker sync",
		"Tout afficher",
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

	fmt.Fprintf(out, "\n%s Configuration GitLab\n\n", theme.Title.Render("●"))

	// ── Config d'équipe ────────────────────────────────────────────────────────
	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))

		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["gitlab"]

		// URL
		url := askInput(out, "URL GitLab (laisser vide pour gitlab.com)", shared.URL)
		if url == "https://gitlab.com" {
			url = "" // store empty to mean "use default"
		}
		shared.URL = url

		// Enabled
		enabledRec := askYN(out, "L'équipe utilise-t-elle GitLab ?", shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec

		// WriteRecommended
		shared.WriteRecommended = askYN(out, "Recommander write_enabled pour l'équipe ?", shared.WriteRecommended)

		teamCfg.MCP["gitlab"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf("sauvegarde config équipe: %w", err)
		}
		fmt.Fprintf(out, "%s Config d'équipe GitLab sauvegardée\n",
			theme.SuccessStyle.Render(theme.IconSuccess))
	}

	// ── Config locale ──────────────────────────────────────────────────────────
	if configLocal {
		fmt.Fprintf(out, "\n%s Configuration locale\n", theme.Bold.Render("→"))

		// Test de connexion avec les credentials actuels
		src := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeGitLab); err != nil {
			// Proposer de configurer un nouveau token
			if askYN(out, "Configurer un nouveau token GitLab ?", true) {
				tokenKey := askInput(out, "Nom de la clé dans le keychain (ex: gitlab-token)", a.Config.MCP.Gitlab.Token)
				fmt.Fprintf(out, "  Pour stocker le token: %s\n",
					theme.Bold.Render("oh secrets set "+tokenKey))
				writeLocal(config.ConfigPath(), "mcp.gitlab.token_key", tokenKey)
				writeLocal(config.ConfigPath(), "mcp.gitlab.enabled", "true")
			}
		}

		// write_enabled
		writeEnabled := askYN(out, "Activer write_enabled (création MR, push labels) ?", a.Config.MCP.Gitlab.WriteEnabled)
		writeLocal(config.ConfigPath(), "mcp.gitlab.write_enabled", fmt.Sprintf("%v", writeEnabled))

		fmt.Fprintf(out, "%s Config locale GitLab sauvegardée\n",
			theme.SuccessStyle.Render(theme.IconSuccess))
	}

	return nil
}

func configureJira(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprintf(out, "\n%s Configuration Jira\n\n", theme.Title.Render("●"))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["jira"]

		url := askInput(out, "URL Jira (ex: https://yourcompany.atlassian.net)", shared.URL)
		shared.URL = url

		enabledRec := askYN(out, "L'équipe utilise-t-elle Jira ?", shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec

		teamCfg.MCP["jira"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf("sauvegarde config équipe: %w", err)
		}
		fmt.Fprintf(out, "%s Config d'équipe Jira sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
	}

	if configLocal {
		fmt.Fprintf(out, "\n%s Configuration locale\n", theme.Bold.Render("→"))
		src := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
		if err := testAndDisplayConnection(ctx, out, src, tracker.TypeJira); err != nil {
			if askYN(out, "Configurer un nouveau token Jira ?", true) {
				tokenKey := askInput(out, "Nom de la clé dans le keychain (ex: jira-token)", a.Config.MCP.Jira.Token)
				fmt.Fprintf(out, "  Pour stocker le token: %s\n", theme.Bold.Render("oh secrets set "+tokenKey))
				writeLocal(config.ConfigPath(), "mcp.jira.token_key", tokenKey)
				writeLocal(config.ConfigPath(), "mcp.jira.enabled", "true")
			}
		}
		writeEnabled := askYN(out, "Activer write_enabled (push labels) ?", a.Config.MCP.Jira.WriteEnabled)
		writeLocal(config.ConfigPath(), "mcp.jira.write_enabled", fmt.Sprintf("%v", writeEnabled))
		fmt.Fprintf(out, "%s Config locale Jira sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
	}

	return nil
}

func configureFigma(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprintf(out, "\n%s Configuration Figma\n\n", theme.Title.Render("●"))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))
		if teamCfg.MCP == nil {
			teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
		}
		shared := teamCfg.MCP["figma"]
		enabledRec := askYN(out, "L'équipe utilise-t-elle Figma ?", shared.Enabled == nil || *shared.Enabled)
		shared.Enabled = &enabledRec
		// URL optionnelle pour Figma (self-hosted)
		url := askInput(out, "URL Figma (laisser vide pour api.figma.com)", shared.URL)
		shared.URL = url
		teamCfg.MCP["figma"] = shared

		if err := repo.SaveConfig(ctx, teamCfg); err != nil {
			return fmt.Errorf("sauvegarde config équipe: %w", err)
		}
		fmt.Fprintf(out, "%s Config d'équipe Figma sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
	}

	if configLocal {
		fmt.Fprintf(out, "\n%s Configuration locale\n", theme.Bold.Render("→"))
		tokenKey := askInput(out, "Nom de la clé dans le keychain (ex: figma-token)", a.Config.MCP.Figma.Token)
		if tokenKey != "" {
			fmt.Fprintf(out, "  Pour stocker le token: %s\n", theme.Bold.Render("oh secrets set "+tokenKey))
			writeLocal(config.ConfigPath(), "mcp.figma.token_key", tokenKey)
			writeLocal(config.ConfigPath(), "mcp.figma.enabled", "true")
			fmt.Fprintf(out, "%s Config locale Figma sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
		}
	}

	return nil
}

func configureTrackerSync(ctx context.Context, a *app.App, repo *teamstate.Repo, teamCfg *teamstate.TeamConfig, configTeam, configLocal bool) error {
	out := a.IO.Out

	fmt.Fprintf(out, "\n%s Configuration Tracker Sync\n\n", theme.Title.Render("●"))

	if configTeam {
		fmt.Fprintf(out, "%s Configuration d'équipe\n", theme.Bold.Render("→"))

		// Type
		typeIdx := askSelect(out, "Type de tracker", []string{"GitLab", "Jira"})
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
		trackerURL := askInput(out, "URL de l'instance (ex: https://gitlab.example.com)", defaultURL)
		teamCfg.Tracker.TrackerURL = trackerURL

		// Tracker token — determine the keychain key
		trackerTokenKey := teamCfg.Tracker.TrackerTokenKey
		if trackerTokenKey == "" {
			trackerTokenKey = "openhub.tracker." + teamCfg.Tracker.Type + ".token"
		}
		teamCfg.Tracker.TrackerTokenKey = trackerTokenKey

		// Check if token exists, prompt if not
		src := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
		trackerType := tracker.Type(teamCfg.Tracker.Type)
		fmt.Fprintf(out, "\n  Test de connexion...\n")
		if err := testAndDisplayConnection(ctx, out, src, trackerType); err != nil {
			if askYN(out, "Configurer un token pour ce tracker ?", true) {
				token := askInput(out, "Token d'accès "+teamCfg.Tracker.Type, "")
				if token != "" {
					if err := a.Secrets.Set(ctx, trackerTokenKey, token); err != nil {
						fmt.Fprintf(out, "  %s Erreur stockage token: %s\n", theme.WarningStyle.Render(theme.IconWarning), err)
					} else {
						fmt.Fprintf(out, "  %s Token stocké dans le keychain (%s)\n",
							theme.SuccessStyle.Render(theme.IconSuccess), trackerTokenKey)
						// Re-test connection
						src = buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
						fmt.Fprintf(out, "\n  Re-test de connexion...\n")
						_ = testAndDisplayConnection(ctx, out, src, trackerType)
					}
				}
			}
		}

		// Enabled
		teamCfg.Tracker.Enabled = askYN(out, "Activer le tracker sync pour l'équipe ?", teamCfg.Tracker.Enabled)

		// Auto-plan
		teamCfg.Tracker.AutoPlanAssigned = askYN(out, "Auto-créer des claims 'planned' pour les tickets assignés ?", teamCfg.Tracker.AutoPlanAssigned)
		if teamCfg.Tracker.AutoPlanAssigned {
			maxStr := askInput(out, "Limite de claims auto-plan par membre", fmt.Sprintf("%d", max(teamCfg.Tracker.MaxAutoPlanPerMember, 5)))
			if n := parseInt(maxStr, 5); n > 0 {
				teamCfg.Tracker.MaxAutoPlanPerMember = n
			}
		}

		// Push labels
		teamCfg.Tracker.PushLabels = askYN(out, "Recommander le push de labels vers le tracker ?", teamCfg.Tracker.PushLabels)

		// Auto-sync
		teamCfg.Tracker.AutoSync = askYN(out, "Activer la synchronisation automatique à l'ouverture des vues ?", teamCfg.Tracker.AutoSync)

		// Projet tracker par défaut
		fmt.Fprintf(out, "\n%s Projet tracker par défaut\n", theme.Bold.Render("→"))
		trackerProject := askInput(out, "ID ou path du projet sur le tracker (ex: group/project)", teamCfg.Tracker.TrackerProject)
		teamCfg.Tracker.TrackerProject = trackerProject

		// ── Discovery: fetch labels/statuses and suggest mappings ─────────
		if trackerProject != "" && teamCfg.Tracker.Type != "" {
			if askYN(out, "Lancer la discovery du projet tracker ?", true) {
				fmt.Fprintf(out, "\n%s Discovery du projet\n", theme.Bold.Render("→"))

				// Build tracker client for discovery.
				src := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
				trackerType := tracker.Type(teamCfg.Tracker.Type)
				creds, credErr := tracker.ResolveCredentials(ctx, src, trackerType)
				if credErr != nil {
					fmt.Fprintf(out, "  %s Credentials manquants : %s\n", theme.WarningStyle.Render(theme.IconWarning), credErr)
				} else {
					t, tErr := tracker.New(creds)
					if tErr != nil {
						fmt.Fprintf(out, "  %s Erreur tracker : %s\n", theme.WarningStyle.Render(theme.IconWarning), tErr)
					} else {
						fmt.Fprintf(out, "  Récupération des labels/statuts...\n")
						info, discErr := t.DiscoverProject(ctx, trackerProject)
						if discErr != nil {
							fmt.Fprintf(out, "  %s Erreur discovery : %s\n", theme.WarningStyle.Render(theme.IconWarning), discErr)
						} else {
							fmt.Fprintf(out, "  %s %d labels, %d statuts découverts\n",
								theme.SuccessStyle.Render(theme.IconSuccess), len(info.Labels), len(info.Statuses))

							// Suggest mappings.
							existingCols := teamCfg.Board.Columns
							if len(existingCols) == 0 {
								existingCols = teamstate.DefaultBoardConfig().Columns
							}
							mappings := tracker.SuggestMappings(info, trackerType, existingCols)

							if len(mappings) > 0 {
								fmt.Fprintf(out, "\n  Mappings suggérés (%d) :\n", len(mappings))
								for _, m := range mappings {
									fmt.Fprintf(out, "    %s → %s (%s)\n", m.Source, m.ColumnID, m.Confidence)
								}

								var acceptedMappings []tracker.SuggestedMapping
								if askYN(out, fmt.Sprintf("  Accepter les %d mappings suggérés ?", len(mappings)), true) {
									acceptedMappings = mappings
								} else {
									fmt.Fprintf(out, "  Confirmation individuelle :\n")
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
									fmt.Fprintf(out, "  %s %d mappings appliqués\n",
										theme.SuccessStyle.Render(theme.IconSuccess), len(acceptedMappings))
								}

								// Suggest pool labels.
								boardCfg := teamCfg.Board
								if !boardCfg.HasCustomColumns() {
									boardCfg = teamstate.DefaultBoardConfig()
								}
								poolLabels := tracker.SuggestPoolLabels(acceptedMappings, boardCfg)
								if len(poolLabels) > 0 {
									fmt.Fprintf(out, "\n%s Tickets à prendre (pool)\n", theme.Bold.Render("→"))
									fmt.Fprintf(out, "  Labels détectés comme 'prêts' : %s\n", strings.Join(poolLabels, ", "))
									if askYN(out, "  Afficher ces tickets sur le board même sans assigné ?", true) {
										teamCfg.Tracker.AutoPlanUnassigned = true
										teamCfg.Tracker.UnassignedLabels = poolLabels
										fmt.Fprintf(out, "  %s Pool configuré : %s\n",
											theme.SuccessStyle.Render(theme.IconSuccess), strings.Join(poolLabels, ", "))
									}
								}
							}

							// Suggest new columns.
							newCols := tracker.SuggestNewColumns(info, existingCols)
							if len(newCols) > 0 {
								fmt.Fprintf(out, "\n%s Colonnes suggérées\n", theme.Bold.Render("→"))
								for _, c := range newCols {
									if askYN(out, fmt.Sprintf("  Ajouter la colonne '%s' (%s) ?", c.Name, c.Role), true) {
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
			return fmt.Errorf("sauvegarde config tracker: %w", err)
		}
		fmt.Fprintf(out, "%s Config tracker d'équipe sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
	}

	if configLocal {
		fmt.Fprintf(out, "\n%s Configuration locale (overrides)\n", theme.Bold.Render("→"))
		fmt.Fprintf(out, "  Laisser vide = hériter de la config d'équipe\n\n")

		// enabled override
		if askYN(out, "Désactiver le tracker sync localement ?", false) {
			writeLocal(config.ConfigPath(), "tracker.enabled", "false")
		} else {
			writeLocal(config.ConfigPath(), "tracker.enabled", "true")
		}

		// auto_sync override
		sharedAutoSync := teamCfg.Tracker.AutoSync
		if askYN(out, fmt.Sprintf("Sync automatique ? (équipe: %v)", sharedAutoSync), sharedAutoSync) {
			writeLocal(config.ConfigPath(), "tracker.auto_sync", "true")
		} else {
			writeLocal(config.ConfigPath(), "tracker.auto_sync", "false")
		}

		// push_labels override
		sharedPush := teamCfg.Tracker.PushLabels
		writeEnabled := resolveWriteEnabledForTracker(a, teamCfg)
		if !writeEnabled && sharedPush {
			fmt.Fprintf(out, "  %s push_labels recommandé par l'équipe mais write_enabled = false sur votre MCP\n",
				theme.WarningStyle.Render(theme.IconWarning))
			fmt.Fprintf(out, "  Activez write_enabled dans [mcp.%s] pour activer le push\n", teamCfg.Tracker.Type)
		} else if writeEnabled {
			if askYN(out, i18n.Tf("cmd.team_config.push_labels_prompt", sharedPush), sharedPush) {
				writeLocal(config.ConfigPath(), "tracker.push_labels", "true")
			} else {
				writeLocal(config.ConfigPath(), "tracker.push_labels", "false")
			}
		}

		fmt.Fprintf(out, "%s Config locale tracker sauvegardée\n", theme.SuccessStyle.Render(theme.IconSuccess))
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
		eff := tracker.ResolveMCPConfig(shared, svc.local)
		printMCPStatus(out, svc.name, shared, svc.local, eff)
	}

	// ── Tracker sync ────────────────────────────────────────────────────────────
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Tracker Sync"))

	if teamCfg == nil || teamCfg.Tracker.Type == "" {
		fmt.Fprintf(out, "  %s Non configuré\n", theme.Subtitle.Render("○"))
		fmt.Fprintf(out, "  Lance %s pour configurer\n\n", theme.Bold.Render("oh team config"))
		return nil
	}

	writeEnabled := resolveWriteEnabledForTracker(a, teamCfg)
	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker, writeEnabled)

	printTrackerStatus(out, teamCfg, a.Config.Tracker, eff)

	// ── Connection tests ─────────────────────────────────────────────────────
	fmt.Fprintf(out, "\n%s\n", theme.Bold.Render("Test de connexion"))
	src := buildCredentialSource(a, sharedMCP, &teamCfg.Tracker)
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
							theme.SuccessStyle.Render("✓"), hubID, trackerID, projectName)
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
	eff tracker.EffectiveMCPConfig,
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
	} else if shared != nil && shared.URL != "" {
		urlSource = "équipe"
	}
	fmt.Fprintf(out, "  url:           %s  %s\n", urlVal, theme.Subtitle.Render("("+urlSource+")"))

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

// testAndDisplayConnection builds a tracker, tests the connection, and prints
// the result. Returns the error if connection failed.
func testAndDisplayConnection(ctx context.Context, out interface{ Write([]byte) (int, error) }, src tracker.CredentialSource, trackerType tracker.Type) error {
	cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
	if err != nil {
		fmt.Fprintf(out, "  %s %s: credentials non disponibles — %v\n",
			theme.ErrorStyle.Render("✗"), trackerType, err)
		return err
	}
	t, err := tracker.New(cfg)
	if err != nil {
		fmt.Fprintf(out, "  %s %s: initialisation échouée — %v\n",
			theme.ErrorStyle.Render("✗"), trackerType, err)
		return err
	}
	username, err := t.TestConnection(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %s %s (%s): %v\n",
			theme.ErrorStyle.Render("✗"), trackerType, cfg.BaseURL, err)
		return err
	}
	fmt.Fprintf(out, "  %s Connecté à %s en tant que @%s\n",
		theme.SuccessStyle.Render("✓"), cfg.BaseURL, username)
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
		fmt.Fprintf(out, "  %s Membre %q (%s, gitlab: %q) dans le registre mais non associé à une config active\n",
			theme.Subtitle.Render("ℹ"),
			m.ID, m.DisplayName, m.GitLabUsername)
	}

	// Offer cleanup for each orphan
	for _, orphan := range orphans {
		fmt.Fprintf(out, "\n  Que faire de %q ?\n", orphan.ID)
		choice := askSelect(out, "", []string{
			"Supprimer du registre",
			"Fusionner avec un membre actif",
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
				fmt.Fprintf(out, "  %s Fusion annulée: %v\n", theme.WarningStyle.Render(theme.IconWarning), err)
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
		return fmt.Errorf("aucun autre membre disponible pour la fusion")
	}

	// Let the user pick the target
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = fmt.Sprintf("%s (%s)", c.ID, c.DisplayName)
	}
	idx := askSelect(out, "Fusionner avec quel membre ?", names)
	target := candidates[idx]

	// Merge: for each field, if target is empty and orphan is not → copy automatically.
	// If both have different non-empty values → ask which to keep.
	merged := target

	merged.DisplayName = resolveField(out, "display_name", target.DisplayName, orphan.DisplayName)
	merged.GitLabUsername = resolveField(out, "gitlab_username", target.GitLabUsername, orphan.GitLabUsername)
	merged.MattermostUsername = resolveField(out, "mattermost_username", target.MattermostUsername, orphan.MattermostUsername)
	merged.Role = resolveField(out, "role", target.Role, orphan.Role)

	// Preview
	fmt.Fprintf(out, "\n  Résultat de la fusion pour %q:\n", merged.ID)
	fmt.Fprintf(out, "    display_name:        %s\n", merged.DisplayName)
	fmt.Fprintf(out, "    gitlab_username:     %s\n", merged.GitLabUsername)
	fmt.Fprintf(out, "    mattermost_username: %s\n", merged.MattermostUsername)
	fmt.Fprintf(out, "    role:                %s\n", merged.Role)

	if !askYN(out, "Confirmer la fusion ?", true) {
		return fmt.Errorf("fusion annulée par l'utilisateur")
	}

	// Apply: update target member, remove orphan
	if err := repo.UpdateMember(ctx, merged); err != nil {
		return fmt.Errorf("mise à jour du membre: %w", err)
	}
	if err := repo.RemoveMember(ctx, orphan.ID); err != nil {
		return fmt.Errorf("suppression de l'orphelin: %w", err)
	}

	fmt.Fprintf(out, "%s Fusion effectuée\n", theme.SuccessStyle.Render(theme.IconSuccess))

	fmt.Fprintf(out, "  %s Membres fusionnés avec succès\n", theme.SuccessStyle.Render(theme.IconSuccess))
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
		fmt.Fprintf(out, "\n  Conflit sur %q:\n    [1] %s (membre actif)\n    [2] %s (orphelin)\n",
			fieldName, target, source)
		choice := askSelect(out, "Quelle valeur garder ?", []string{target, source})
		if choice == 1 {
			return source
		}
		return target
	}
}

// reading the file, updating the value, and rewriting. This is intentionally
// simple — for complex edits the user can edit hub.toml directly.
func writeLocal(configPath, keyPath, value string) {
	// Read current content
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("  Erreur lecture %s: %v\n", configPath, err)
		return
	}

	// Parse existing TOML into a nested map
	var tree map[string]interface{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &tree); err != nil {
			fmt.Printf("  Erreur parsing %s: %v\n", configPath, err)
			return
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
			fmt.Printf("  Erreur: %q n'est pas une table TOML\n", strings.Join(parts[:i+1], "."))
			return
		}
	}
	current[parts[len(parts)-1]] = value

	// Marshal and write atomically
	out, err := toml.Marshal(tree)
	if err != nil {
		fmt.Printf("  Erreur sérialisation: %v\n", err)
		return
	}
	tmpFile := configPath + ".tmp"
	if err := os.WriteFile(tmpFile, out, 0o600); err != nil {
		fmt.Printf("  Erreur écriture: %v\n", err)
		return
	}
	if err := os.Rename(tmpFile, configPath); err != nil {
		fmt.Printf("  Erreur rename: %v\n", err)
		os.Remove(tmpFile)
		return
	}
	fmt.Printf("  %s mis à jour: %s = %s\n", configPath, keyPath, value)
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
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Fprintf(out, "  %s %s ", prompt, hint)
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes" || input == "oui" || input == "o"
}

func askSelect(out interface{ Write([]byte) (int, error) }, prompt string, options []string) int {
	fmt.Fprintf(out, "\n  %s\n", prompt)
	for i, opt := range options {
		fmt.Fprintf(out, "    [%d] %s\n", i+1, opt)
	}
	fmt.Fprintf(out, "  Choix [1]: ")
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
		return theme.SuccessStyle.Render("✓")
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
