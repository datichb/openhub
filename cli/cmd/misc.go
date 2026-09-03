package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
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
		// Intercept "init" subcommand to enforce zero-impact flags.
		// This prevents bd from installing hooks, generating agent files,
		// or modifying .gitignore even when called via the proxy.
		if len(args) > 0 && args[0] == "init" {
			args = beads.EnsureInitFlags(args)
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
		theme.Title.Render("oh service"))

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
		status := theme.ErrorStyle.Render(i18n.T("cmd.service.status_disabled"))
		tokenStatus := ""

		if svc.enabled {
			status = theme.SuccessStyle.Render(i18n.T("cmd.service.status_enabled"))

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
				tokenStatus = fmt.Sprintf(" %s %s", theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.service.token_missing"))
			}
		}

		fmt.Fprintf(a.IO.Out, "  %-15s %s%s\n", svc.label, status, tokenStatus)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.service.setup_hint", theme.Bold.Render("oh service setup")))
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
	}

	// Shared state between wizard steps
	var serviceName string
	var token string
	var envHint string
	var writeEnabled bool

	serviceOptions := []string{
		"Figma — design tokens & composants",
		"GitLab — merge requests & pipelines",
		"Google Slides — présentations",
	}
	serviceValues := []string{"figma", "gitlab", "gslides"}

	steps := []views.WizardStep{
		{
			Label: "Service",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown(i18n.T("cmd.service.select"), serviceOptions, -1, func(option string, index int) {
					if index >= 0 && index < len(serviceValues) {
						serviceName = serviceValues[index]
					}
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				switch serviceName {
				case "figma":
					envHint = "FIGMA_TOKEN"
				case "gitlab":
					envHint = "GITLAB_TOKEN"
				case "gslides":
					envHint = "GOOGLE_ACCESS_TOKEN"
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Service", Value: serviceName},
				}
			},
		},
		{
			Label: "Token",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.service.token_prompt", serviceName),
					"", 0, '*', func(text string) {
						token = text
					})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if token == "" {
					// Empty token — skip storage, user will use env var
					return nil
				}
				if a.Secrets != nil {
					keyName := serviceName + "-token"
					if project != nil {
						keyName = serviceName + "-token-" + project.ID
					}
					if err := a.Secrets.Set(ctx, keyName, token); err != nil {
						return fmt.Errorf("%s", i18n.Tf("cmd.service.keychain_error", err))
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				status := "stored in keychain"
				if token == "" {
					status = fmt.Sprintf("use env %s", envHint)
				}
				return []views.InfoField{
					{Label: "Token", Value: status},
				}
			},
		},
		{
			Label: "Write permissions",
			SkipIf: func() bool {
				return serviceName != "gitlab"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox("Activer le mode écriture (créer MR, commenter, assigner) ?", false, func(checked bool) {
					writeEnabled = checked
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				return nil
			},
			InfoFields: func() []views.InfoField {
				mode := "lecture seule"
				if writeEnabled {
					mode = "lecture + écriture"
				}
				return []views.InfoField{
					{Label: "Mode", Value: mode},
				}
			},
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "service setup",
			StatusHints: "enter confirm · esc skip",
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
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
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.service.project_configured", theme.Bold.Render(serviceName), project.Name))
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
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.service.enabled", theme.Bold.Render(serviceName)))
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
		form := theme.NewForm(
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
		_ = theme.NewForm(huh.NewGroup(huh.NewConfirm().
			Title(i18n.Tf("cmd.service.remove.confirm", serviceName)).
			Value(&confirm))).Run()
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
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.service.disabled", theme.Bold.Render(serviceName)))
	return nil
}

func init() {
	rootCmd.AddCommand(beadsCmd)

	rootCmd.AddCommand(serviceCmd)
	serviceCmd.Flags().StringP("project", "p", "", "Nom du projet")
	_ = serviceCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	serviceCmd.AddCommand(serviceSetupCmd)
	serviceCmd.AddCommand(serviceRemoveCmd)
}
