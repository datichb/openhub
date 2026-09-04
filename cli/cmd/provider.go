package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Gestion des credentials provider LLM",
	Long:  "Configure les credentials du provider LLM (clé API, profil AWS, bearer token) au niveau hub ou projet.",
}

var providerSetupCmd = &cobra.Command{
	Use:   "setup [provider-name]",
	Short: "Configure un provider LLM (wizard interactif)",
	Long:  "Lance un wizard pour configurer les credentials d'un provider. Sans argument, propose un sélecteur.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProviderSetup,
}

func init() {
	rootCmd.AddCommand(providerCmd)
	providerCmd.AddCommand(providerSetupCmd)
	providerSetupCmd.Flags().StringP("project", "p", "", "Configure provider for a specific project")
}

func runProviderSetup(cmd *cobra.Command, args []string) error {
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
			theme.Title.Render("oh provider setup"),
			i18n.T("cmd.provider.project_scope"), project.Name)
	} else {
		fmt.Fprintf(a.IO.Out, "%s\n\n", theme.Title.Render("oh provider setup"))
	}

	// Provider selection
	var providerName string
	if len(args) > 0 {
		providerName = args[0]
	} else {
		// Detect what's available and show indicators
		options := buildProviderOptions()
		form := theme.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(i18n.T("cmd.provider.select")).
					Options(options...).
					Value(&providerName),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
	}

	// Run provider-specific wizard
	switch provider.Name(providerName) {
	case provider.Bedrock:
		return setupBedrock(ctx, a, project)
	case provider.Anthropic:
		return setupAPIKey(ctx, a, project, provider.Anthropic)
	case provider.OpenRouter:
		return setupAPIKey(ctx, a, project, provider.OpenRouter)
	case provider.GithubCopilot:
		return setupGithubCopilot(a)
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}
}

func buildProviderOptions() []huh.Option[string] {
	results := provider.DetectAll()
	var options []huh.Option[string]
	for _, r := range results {
		label := string(r.Provider)
		switch r.Provider {
		case provider.Bedrock:
			label = "Amazon Bedrock"
		case provider.Anthropic:
			label = "Anthropic (direct API)"
		case provider.OpenRouter:
			label = "OpenRouter"
		case provider.GithubCopilot:
			label = "GitHub Copilot"
		}
		if r.Available {
			label += fmt.Sprintf(" %s (%s)", theme.SuccessStyle.Render(theme.IconSuccess), r.Source)
		}
		options = append(options, huh.NewOption(label, string(r.Provider)))
	}
	return options
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider-specific setup wizards
// ─────────────────────────────────────────────────────────────────────────────

func setupBedrock(ctx context.Context, a *app.App, project *domain.Project) error {
	// Detect existing config
	detection := provider.Detect(provider.Bedrock)

	// Shared state across wizard steps
	var (
		useExisting bool
		authMode    string
		token       string
		awsProfile  string
		awsRegion   string
	)

	steps := []views.WizardStep{
		// Step 1: Use existing detected config?
		{
			Label: "Detect existing",
			SkipIf: func() bool {
				return !detection.Available
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(
					fmt.Sprintf("%s (%s)", i18n.T("cmd.provider.use_existing"), detection.Details),
					true,
					func(checked bool) { useExisting = checked },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if useExisting {
					return persistProviderConfig(ctx, a, project, provider.Bedrock, "", detection)
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if useExisting {
					return []views.InfoField{
						{Label: "Source", Value: detection.Source},
						{Label: "Details", Value: detection.Details},
					}
				}
				return nil
			},
		},
		// Step 2: Auth mode selection
		{
			Label: "Auth mode",
			SkipIf: func() bool {
				return useExisting
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				options := []string{"Bearer Token (SSO/STS)", "AWS Profile (~/.aws/credentials)"}
				form.AddDropDown(
					i18n.T("cmd.provider.bedrock.auth_mode"),
					options,
					0,
					func(option string, index int) {
						if index == 0 {
							authMode = "bearer"
						} else {
							authMode = "profile"
						}
					},
				)
				// Set default
				authMode = "bearer"
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth", Value: authMode}}
			},
		},
		// Step 3: Bearer token + region (conditional on authMode == "bearer")
		{
			Label: "Bearer credentials",
			SkipIf: func() bool {
				return useExisting || authMode != "bearer"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.T("cmd.provider.bedrock.bearer_token"),
					"", 0, '*',
					func(text string) { token = text },
				)
				form.AddInputField(
					i18n.T("cmd.provider.bedrock.region"),
					"us-east-1", 0, nil,
					func(text string) { awsRegion = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				masked := "***"
				if len(token) > 4 {
					masked = token[:2] + "..." + token[len(token)-2:]
				}
				return []views.InfoField{
					{Label: "Token", Value: masked},
					{Label: "Region", Value: awsRegion},
				}
			},
		},
		// Step 4: AWS profile + region (conditional on authMode == "profile")
		{
			Label: "Profile credentials",
			SkipIf: func() bool {
				return useExisting || authMode != "profile"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(
					i18n.T("cmd.provider.bedrock.profile"),
					"default", 0, nil,
					func(text string) { awsProfile = text },
				)
				form.AddInputField(
					i18n.T("cmd.provider.bedrock.region"),
					"us-east-1", 0, nil,
					func(text string) { awsRegion = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Profile", Value: awsProfile},
					{Label: "Region", Value: awsRegion},
				}
			},
		},
		// Step 5: Persist configuration
		{
			Label:      "Save config",
			Processing: "Saving credentials...",
			SkipIf: func() bool {
				return useExisting
			},
			OnDone: func() error {
				if awsProfile == "" && authMode == "profile" {
					awsProfile = "default"
				}
				if awsRegion == "" {
					awsRegion = "us-east-1"
				}

				// Store token in keychain if provided
				if token != "" && a.Secrets != nil {
					keyName := provider.KeychainKey(provider.Bedrock, projectIDOrEmpty(project))
					if err := a.Secrets.Set(ctx, keyName, token); err != nil {
						return fmt.Errorf("storing bedrock token: %w", err)
					}
				}

				det := provider.DetectionResult{
					Source:  authMode,
					Details: fmt.Sprintf("profile %s, region %s", awsProfile, awsRegion),
				}
				return persistProviderConfig(ctx, a, project, provider.Bedrock, awsProfile+"|"+awsRegion+"|"+authMode, det)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Status", Value: "saved"}}
			},
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "provider setup",
			StatusHints: i18n.T("wizard.hints.default"),
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	return wizResult.Err
}

func setupAPIKey(ctx context.Context, a *app.App, project *domain.Project, name provider.Name) error {
	detection := provider.Detect(name)

	// Shared state across wizard steps
	var (
		useExisting bool
		apiKey      string
	)

	envVar := provider.EnvVar(name)

	steps := []views.WizardStep{
		// Step 1: Use existing detected key?
		{
			Label: "Detect existing",
			SkipIf: func() bool {
				return !detection.Available
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(
					fmt.Sprintf("%s (%s)", i18n.T("cmd.provider.use_existing"), detection.Details),
					true,
					func(checked bool) { useExisting = checked },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if useExisting {
					return persistProviderConfig(ctx, a, project, name, "", detection)
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if useExisting {
					return []views.InfoField{
						{Label: "Source", Value: detection.Source},
						{Label: "Details", Value: detection.Details},
					}
				}
				return nil
			},
		},
		// Step 2: Enter API key
		{
			Label: "API Key",
			SkipIf: func() bool {
				return useExisting
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.provider.api_key_prompt", string(name)),
					"", 0, '*',
					func(text string) { apiKey = text },
				)
				form.AddInputField(
					i18n.Tf("cmd.provider.api_key_hint", envVar),
					"", 0, nil, nil,
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				masked := "***"
				if len(apiKey) > 8 {
					masked = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
				}
				return []views.InfoField{{Label: "Key", Value: masked}}
			},
		},
		// Step 3: Store key and persist
		{
			Label:      "Save config",
			Processing: "Saving credentials...",
			SkipIf: func() bool {
				return useExisting
			},
			OnDone: func() error {
				if apiKey == "" {
					fmt.Fprintf(a.IO.Out, "%s %s\n",
						theme.WarningStyle.Render(theme.IconWarning),
						i18n.T("cmd.provider.no_key"))
					return nil
				}

				// Store in keychain
				if a.Secrets != nil {
					keyName := provider.KeychainKey(name, projectIDOrEmpty(project))
					if err := a.Secrets.Set(ctx, keyName, apiKey); err != nil {
						return fmt.Errorf("storing API key: %w", err)
					}
				}

				return persistProviderConfig(ctx, a, project, name, "", detection)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Status", Value: "saved"}}
			},
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "provider setup",
			StatusHints: i18n.T("wizard.hints.default"),
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	return wizResult.Err
}

func setupGithubCopilot(a *app.App) error {
	detection := provider.Detect(provider.GithubCopilot)
	if detection.Available {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.provider.detected", detection.Source, detection.Details))
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.T("cmd.provider.copilot_ready"))
		return nil
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.WarningStyle.Render(theme.IconWarning),
		i18n.T("cmd.provider.copilot_not_found"))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func persistProviderConfig(ctx context.Context, a *app.App, project *domain.Project, name provider.Name, rawConfig string, _ provider.DetectionResult) error {
	if project != nil {
		// Project-scoped: update project.ProviderConfig
		if project.ProviderConfig == nil {
			project.ProviderConfig = &domain.ProjectProviderConfig{}
		}
		if name == provider.Bedrock && rawConfig != "" {
			parts := splitProviderRawConfig(rawConfig)
			if len(parts) == 3 {
				project.ProviderConfig.AWSProfile = parts[0]
				project.ProviderConfig.AWSRegion = parts[1]
				project.ProviderConfig.AuthMode = parts[2]
			}
		}
		project.ProviderConfig.TokenKey = string(provider.KeychainKey(name, project.ID))
		if err := a.Projects.Update(ctx, project); err != nil {
			return fmt.Errorf("updating project provider config: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.provider.project_configured", string(name), project.Name))
	} else {
		// Hub-scoped: write to hub.toml
		v := configViper()
		if name == provider.Bedrock && rawConfig != "" {
			parts := splitProviderRawConfig(rawConfig)
			if len(parts) == 3 {
				v.Set("provider.bedrock.aws_profile", parts[0])
				v.Set("provider.bedrock.aws_region", parts[1])
				v.Set("provider.bedrock.auth_mode", parts[2])
			}
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
			i18n.Tf("cmd.provider.hub_configured", string(name)))
	}
	return nil
}

func splitProviderRawConfig(raw string) []string {
	// Simple split by "|" — internal format for bedrock: "profile|region|authMode"
	parts := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '|' {
			parts = append(parts, raw[start:i])
			start = i + 1
		}
	}
	parts = append(parts, raw[start:])
	return parts
}

func projectIDOrEmpty(p *domain.Project) string {
	if p == nil {
		return ""
	}
	return p.ID
}
