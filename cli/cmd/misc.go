package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var beadsCmd = &cobra.Command{
	Use:                "beads",
	Short:              "Gestion des tickets beads (proxy vers bd)",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("bd"); err != nil {
			return fmt.Errorf("%s", i18n.T("cmd.beads.bd_not_installed"))
		}
		// Delegate to bd command — all args/flags are passed through
		bdCmd := exec.Command("bd", args...)
		bdCmd.Stdin = os.Stdin
		bdCmd.Stdout = os.Stdout
		bdCmd.Stderr = os.Stderr
		return bdCmd.Run()
	},
}

var serviceCmd = &cobra.Command{
	Use:        "service",
	Short:      "Gestion des services MCP",
	Deprecated: "Utilisez 'oh mcp status' à la place.",
	Long: `Gestion des serveurs MCP (Figma, GitLab, Google Slides).

Sans sous-commande, affiche le statut enrichi de tous les services.
Sous-commandes : setup, status, remove.`,
	RunE: runServiceStatus,
}

var serviceSetupCmd = &cobra.Command{
	Use:        "setup",
	Short:      "Configure un service MCP (wizard interactif)",
	Deprecated: "Utilisez 'oh mcp setup' à la place.",
	Long:       "Lance un wizard pour configurer les tokens d'un service MCP dans le keychain.",
	RunE:       runServiceSetup,
}

var serviceRemoveCmd = &cobra.Command{
	Use:        "remove [service-name]",
	Short:      "Désactive un service MCP",
	Deprecated: "Utilisez 'oh mcp disable' à la place.",
	Long:       "Supprime le token d'un service et le désactive dans la configuration.",
	Args:       cobra.MaximumNArgs(1),
	RunE:       runServiceRemove,
}

func init() {
	serviceRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	serviceSetupCmd.Flags().StringP("project", "p", "", "Configure MCP for a specific project (overrides hub-level)")
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	fmt.Fprintf(a.IO.Out, "%s Services MCP\n\n",
		common.Title.Render("oh service"))

	services := []struct {
		name    string
		label   string
		enabled bool
		token   string
		envVar  string
	}{
		{"figma", "Figma", a.Config.MCP.Figma.Enabled, a.Config.MCP.Figma.Token, "FIGMA_TOKEN"},
		{"gitlab", "GitLab", a.Config.MCP.Gitlab.Enabled, a.Config.MCP.Gitlab.Token, "GITLAB_TOKEN"},
		{"gslides", "Google Slides", a.Config.MCP.Gslides.Enabled, a.Config.MCP.Gslides.Token, "GOOGLE_ACCESS_TOKEN"},
	}

	for _, svc := range services {
		status := common.ErrorStyle.Render(i18n.T("cmd.service.status_disabled"))
		tokenStatus := ""

		if svc.enabled {
			status = common.SuccessStyle.Render(i18n.T("cmd.service.status_enabled"))

			// Check if token is available
			hasToken := false
			if svc.envVar != "" && os.Getenv(svc.envVar) != "" {
				hasToken = true
				tokenStatus = fmt.Sprintf(" (env: %s)", svc.envVar)
			} else if svc.token != "" && a.Secrets != nil {
				if t, _ := a.Secrets.Get(ctx, svc.token); t != "" {
					hasToken = true
					tokenStatus = " (keychain)"
				}
			}

			if !hasToken && svc.token != "" {
				tokenStatus = fmt.Sprintf(" %s %s", common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.service.token_missing"))
			}
		}

		fmt.Fprintf(a.IO.Out, "  %-15s %s%s\n", svc.label, status, tokenStatus)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.service.setup_hint", common.Bold.Render("oh service setup")))
	return nil
}

func runServiceSetup(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Check if project-scoped
	projectID, _ := cmd.Flags().GetString("project")
	var project *domain.Project
	if projectID != "" {
		var err error
		project, err = resolveProject(ctx, a, projectID)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.IO.Out, "%s %s (%s)\n\n",
			common.Title.Render("oh service setup"),
			i18n.T("cmd.service.project_scope"), project.Name)
	} else {
		fmt.Fprintf(a.IO.Out, "%s Configuration MCP\n\n",
			common.Title.Render("oh service setup"))
	}

	// Service selection
	var serviceName string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.service.select")).
				Options(
					huh.NewOption("Figma — design tokens & composants", "figma"),
					huh.NewOption("GitLab — merge requests & pipelines", "gitlab"),
					huh.NewOption("Google Slides — présentations", "gslides"),
				).
				Value(&serviceName),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Token input
	var token string
	envHint := ""
	switch serviceName {
	case "figma":
		envHint = "FIGMA_TOKEN"
	case "gitlab":
		envHint = "GITLAB_TOKEN"
	case "gslides":
		envHint = "GOOGLE_ACCESS_TOKEN"
	}

	tokenForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(i18n.Tf("cmd.service.token_prompt", serviceName)).
				Description(i18n.Tf("cmd.service.token_env_hint", envHint)).
				EchoMode(huh.EchoModePassword).
				Value(&token),
		),
	)

	if err := tokenForm.Run(); err != nil {
		return err
	}

	if token == "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.service.token_empty_warning", envHint))
	} else if a.Secrets != nil {
		// Store token in keychain (project-scoped key if --project)
		keyName := serviceName + "-token"
		if project != nil {
			keyName = serviceName + "-token-" + project.ID
		}
		if err := a.Secrets.Set(ctx, keyName, token); err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.service.keychain_error", err))
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.service.token_stored", keyName))
	}

	// GitLab: ask about write permissions
	var writeEnabled bool
	if serviceName == "gitlab" {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, common.Bold.Render("  Droits requis pour le token GitLab :"))
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, "  Mode lecture seule (par défaut) :")
		fmt.Fprintf(a.IO.Out, "    %s read_api\n", common.SuccessStyle.Render(common.IconSuccess))
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, "  Mode lecture + écriture :")
		fmt.Fprintf(a.IO.Out, "    %s api (inclut read + write)\n", common.SuccessStyle.Render(common.IconSuccess))
		fmt.Fprintln(a.IO.Out, "    Permet : créer MR, commenter, assigner, modifier labels/statuts")
		fmt.Fprintln(a.IO.Out)

		_ = huh.NewConfirm().
			Title("Activer le mode écriture (créer MR, commenter, assigner) ?").
			Description("Nécessite un token avec le scope 'api'").
			Value(&writeEnabled).
			Run()

		if writeEnabled {
			fmt.Fprintf(a.IO.Out, "%s Mode écriture activé\n",
				common.SuccessStyle.Render(common.IconSuccess))
		}
	}

	// Persist configuration
	if project != nil {
		// Project-scoped: update project.MCPConfig in DB
		tokenKey := serviceName + "-token-" + project.ID
		svc := domain.ProjectMCPService{
			Name:     serviceName,
			TokenKey: tokenKey,
		}
		if serviceName == "gitlab" {
			svc.WriteEnabled = &writeEnabled
		}

		// Merge with existing MCPConfig
		if project.MCPConfig == nil {
			project.MCPConfig = &domain.ProjectMCPConfig{}
		}
		// Replace or add the service
		found := false
		for i, existing := range project.MCPConfig.Services {
			if existing.Name == serviceName {
				project.MCPConfig.Services[i] = svc
				found = true
				break
			}
		}
		if !found {
			project.MCPConfig.Services = append(project.MCPConfig.Services, svc)
		}

		if err := a.Projects.Update(ctx, project); err != nil {
			return fmt.Errorf("updating project MCP config: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.service.project_configured", common.Bold.Render(serviceName), project.Name))
	} else {
		// Hub-scoped: write to hub.toml
		v := configViper()
		v.Set("mcp."+serviceName+".enabled", true)
		v.Set("mcp."+serviceName+".token_key", serviceName+"-token")
		if serviceName == "gitlab" {
			v.Set("mcp.gitlab.write_enabled", writeEnabled)
		}

		cfgPath := config.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(cfgPath); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.service.enabled", common.Bold.Render(serviceName)))
	}
	return nil
}

func runServiceRemove(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	var serviceName string
	if len(args) > 0 {
		serviceName = args[0]
	} else {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(i18n.T("cmd.service.select_remove")).
					Options(
						huh.NewOption("Figma", "figma"),
						huh.NewOption("GitLab", "gitlab"),
						huh.NewOption("Google Slides", "gslides"),
					).
					Value(&serviceName),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
	}

	// Validate service name
	switch serviceName {
	case "figma", "gitlab", "gslides":
		// valid
	default:
		return fmt.Errorf("%s", i18n.Tf("cmd.service.invalid", serviceName))
	}

	// Confirmation prompt
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		var confirm bool
		_ = huh.NewConfirm().
			Title(i18n.Tf("cmd.service.remove.confirm", serviceName)).
			Value(&confirm).
			Run()
		if !confirm {
			return nil
		}
	}

	// Remove token from keychain
	if a.Secrets != nil {
		keyName := serviceName + "-token"
		_ = a.Secrets.Delete(ctx, keyName) // ignore error if not found
	}

	// Disable in config
	v := configViper()
	v.Set("mcp."+serviceName+".enabled", false)

	cfgPath := config.ConfigPath()
	if err := v.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.service.disabled", common.Bold.Render(serviceName)))
	return nil
}

func init() {
	rootCmd.AddCommand(beadsCmd)

	rootCmd.AddCommand(serviceCmd)
	serviceCmd.Flags().StringP("project", "j", "", "Nom du projet")
	_ = serviceCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	serviceCmd.AddCommand(serviceSetupCmd)
	serviceCmd.AddCommand(serviceRemoveCmd)
}
