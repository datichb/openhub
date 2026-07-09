package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/common"
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
			common.Title.Render("oh provider setup"),
			i18n.T("cmd.provider.project_scope"), project.Name)
	} else {
		fmt.Fprintf(a.IO.Out, "%s\n\n", common.Title.Render("oh provider setup"))
	}

	// Provider selection
	var providerName string
	if len(args) > 0 {
		providerName = args[0]
	} else {
		// Detect what's available and show indicators
		options := buildProviderOptions()
		form := huh.NewForm(
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
			label += fmt.Sprintf(" %s (%s)", common.SuccessStyle.Render(common.IconSuccess), r.Source)
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
	if detection.Available {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.detected", detection.Source, detection.Details))

		var useExisting bool
		if err := huh.NewConfirm().
			Title(i18n.T("cmd.provider.use_existing")).
			Value(&useExisting).
			Run(); err != nil {
			return err
		}

		if useExisting {
			// Store the detected config
			return persistProviderConfig(ctx, a, project, provider.Bedrock, "", detection)
		}
	}

	// Manual configuration
	var authMode string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.provider.bedrock.auth_mode")).
				Options(
					huh.NewOption("Bearer Token (SSO/STS)", "bearer"),
					huh.NewOption("AWS Profile (~/.aws/credentials)", "profile"),
				).
				Value(&authMode),
		),
	).Run(); err != nil {
		return err
	}

	var token, awsProfile, awsRegion string

	if authMode == "bearer" {
		tokenForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.bearer_token")).
					EchoMode(huh.EchoModePassword).
					Value(&token),
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.region")).
					Placeholder("us-east-1").
					Value(&awsRegion),
			),
		)
		if err := tokenForm.Run(); err != nil {
			return err
		}
	} else {
		profileForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.profile")).
					Placeholder("default").
					Value(&awsProfile),
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.region")).
					Placeholder("us-east-1").
					Value(&awsRegion),
			),
		)
		if err := profileForm.Run(); err != nil {
			return err
		}
		if awsProfile == "" {
			awsProfile = "default"
		}
	}

	// Store token in keychain if provided
	if token != "" && a.Secrets != nil {
		keyName := provider.KeychainKey(provider.Bedrock, projectIDOrEmpty(project))
		if err := a.Secrets.Set(ctx, keyName, token); err != nil {
			return fmt.Errorf("storing bedrock token: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.token_stored", keyName))
	}

	if awsRegion == "" {
		awsRegion = "us-east-1"
	}

	// Persist config
	det := provider.DetectionResult{Source: authMode, Details: fmt.Sprintf("profile %s, region %s", awsProfile, awsRegion)}
	return persistProviderConfig(ctx, a, project, provider.Bedrock, awsProfile+"|"+awsRegion+"|"+authMode, det)
}

func setupAPIKey(ctx context.Context, a *app.App, project *domain.Project, name provider.Name) error {
	detection := provider.Detect(name)
	if detection.Available {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.detected", detection.Source, detection.Details))

		var useExisting bool
		if err := huh.NewConfirm().
			Title(i18n.T("cmd.provider.use_existing")).
			Value(&useExisting).
			Run(); err != nil {
			return err
		}

		if useExisting {
			return persistProviderConfig(ctx, a, project, name, "", detection)
		}
	}

	var apiKey string
	envVar := provider.EnvVar(name)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(i18n.Tf("cmd.provider.api_key_prompt", string(name))).
				Description(i18n.Tf("cmd.provider.api_key_hint", envVar)).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	if apiKey == "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning),
			i18n.T("cmd.provider.no_key"))
		return nil
	}

	// Store in keychain
	if a.Secrets != nil {
		keyName := provider.KeychainKey(name, projectIDOrEmpty(project))
		if err := a.Secrets.Set(ctx, keyName, apiKey); err != nil {
			return fmt.Errorf("storing API key: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.token_stored", keyName))
	}

	return persistProviderConfig(ctx, a, project, name, "", detection)
}

func setupGithubCopilot(a *app.App) error {
	detection := provider.Detect(provider.GithubCopilot)
	if detection.Available {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.detected", detection.Source, detection.Details))
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.T("cmd.provider.copilot_ready"))
		return nil
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.WarningStyle.Render(common.IconWarning),
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
			common.SuccessStyle.Render(common.IconSuccess),
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
			common.SuccessStyle.Render(common.IconSuccess),
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
