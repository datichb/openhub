package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise oh pour la première fois",
	Long: `Lance un wizard de configuration pour initialiser le hub et enregistrer le premier projet.

Le wizard configure :
  - La langue de l'interface et le provider LLM
  - Les serveurs MCP (Figma, GitLab, Google Slides)
  - Le projet (optionnel)`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// ─────────────────────────────────────────────────────────────────────────────
// Styles for init wizard
// ─────────────────────────────────────────────────────────────────────────────

var (
	initSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(common.Primary).
				MarginTop(1).
				MarginBottom(0)

	initStepIndicator = lipgloss.NewStyle().
				Foreground(common.Subtle).
				Bold(true)
)

// ─────────────────────────────────────────────────────────────────────────────
// Main init flow
// ─────────────────────────────────────────────────────────────────────────────

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// ══════════════════════════════════════════════════════════════════════════
	// PREAMBLE
	// ══════════════════════════════════════════════════════════════════════════
	preamble := i18n.T("cmd.init.preamble")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, common.Box.Render(preamble))
	fmt.Fprintln(os.Stdout)

	// Wait for user to press Enter
	fmt.Fprintf(os.Stdout, "  %s ", common.Subtitle.Render(i18n.T("cmd.init.press_enter")))
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	// ══════════════════════════════════════════════════════════════════════════
	// PART 1 — Hub Configuration
	// ══════════════════════════════════════════════════════════════════════════
	fmt.Fprintf(os.Stdout, "\n%s %s\n\n",
		initStepIndicator.Render("[1/3]"),
		initSectionStyle.Render(i18n.T("cmd.init.section_hub")))

	var (
		language    string
		opencodeVer string
		provider    string
	)

	hubForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.language_select")).
				Options(
					huh.NewOption("Français", "fr"),
					huh.NewOption("English", "en"),
				).
				Value(&language),

			huh.NewInput().
				Title(i18n.T("cmd.init.opencode_version")).
				Description(i18n.T("cmd.init.opencode_version_desc")).
				Placeholder("latest").
				Value(&opencodeVer),

			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.provider_select")).
				Description(i18n.T("cmd.init.provider_select_desc")).
				Options(
					huh.NewOption("Amazon Bedrock", "bedrock"),
					huh.NewOption("Anthropic (direct API)", "anthropic"),
					huh.NewOption("OpenRouter", "openrouter"),
					huh.NewOption("GitHub Copilot", "github-copilot"),
				).
				Value(&provider),
		).Title(i18n.T("cmd.init.global_config")),
	)
	if err := hubForm.Run(); err != nil {
		return err
	}

	if opencodeVer == "" {
		opencodeVer = "latest"
	}

	// ══════════════════════════════════════════════════════════════════════════
	// PART 2 — MCP Servers (optional)
	// ══════════════════════════════════════════════════════════════════════════
	fmt.Fprintf(os.Stdout, "\n%s %s\n\n",
		initStepIndicator.Render("[2/3]"),
		initSectionStyle.Render(i18n.T("cmd.init.section_mcp")))

	var configureMCP bool
	if err := huh.NewConfirm().
		Title(i18n.T("cmd.init.mcp_configure_prompt")).
		Value(&configureMCP).
		Run(); err != nil {
		return err
	}

	var mcpServices []string
	if configureMCP {
		mcpServices = runInitMCPWizard()
	}

	// ══════════════════════════════════════════════════════════════════════════
	// Write hub.toml + Initialize app
	// ══════════════════════════════════════════════════════════════════════════
	cfgDir := config.HubDir()
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	tomlContent := buildInitConfig(language, opencodeVer, provider, mcpServices)
	cfgPath := config.ConfigPath()
	if err := os.WriteFile(cfgPath, []byte(tomlContent), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "\n%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.init.config_written", cfgPath))

	// Ensure opencode
	config.Reset()
	if err := initApp(); err != nil {
		return err
	}

	a := MustApp()
	if err := ensureOpencode(a); err != nil {
		return err
	}

	// ── Provider credentials detection + setup ──
	initProviderCredentials(providerPkg.Name(provider), a, ctx)

	// Store MCP tokens in keychain (for services that were configured)
	if configureMCP && a.Secrets != nil {
		storeMCPTokens(a, ctx, mcpServices)
	}

	// Extract hub content
	hubContentDir := hubcontent.HubContentDir()
	if err := hubcontent.Extract(hubContentDir); err != nil {
		return fmt.Errorf("extracting hub content: %w", err)
	}
	fmt.Fprintf(os.Stdout, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.init.hub_extracted", hubContentDir))

	// ══════════════════════════════════════════════════════════════════════════
	// PART 3 — First Project (optional)
	// ══════════════════════════════════════════════════════════════════════════
	fmt.Fprintf(os.Stdout, "\n%s %s\n\n",
		initStepIndicator.Render("[3/3]"),
		initSectionStyle.Render(i18n.T("cmd.init.section_project")))

	var addProject bool
	if err := huh.NewConfirm().
		Title(i18n.T("cmd.init.add_project_prompt")).
		Value(&addProject).
		Run(); err != nil {
		return err
	}

	if addProject {
		fmt.Fprintf(os.Stdout, "\n%s %s\n\n",
			common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.init.first_project"))
		return runProjectAddInteractive(ctx, a)
	}

	fmt.Fprintf(os.Stdout, "\n%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.T("cmd.init.done_no_project"))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP Wizard (inline in init)
// ─────────────────────────────────────────────────────────────────────────────

// runInitMCPWizard runs the MCP service selection and token configuration.
// Returns the list of successfully configured services (with tokens stored).
func runInitMCPWizard() []string {
	var selected []string
	mcpForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(i18n.T("cmd.init.mcp_select")).
				Description(i18n.T("cmd.init.mcp_select_desc")).
				Options(
					huh.NewOption("Figma (personal access token, scope file:read)", "figma"),
					huh.NewOption("GitLab (personal access token, scope api)", "gitlab"),
					huh.NewOption("Google Slides (OAuth access token)", "gslides"),
				).
				Value(&selected),
		),
	)
	if err := mcpForm.Run(); err != nil || len(selected) == 0 {
		return nil
	}

	return selected
}

// storeMCPTokens prompts for each selected MCP service token and stores in keychain.
// Services where the user skips the token are removed from the configured list.
func storeMCPTokens(a *app.App, ctx context.Context, services []string) {
	for _, svc := range services {
		var token string
		envHint := mcpEnvHint(svc)

		tokenForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.Tf("cmd.init.mcp_token_prompt", svc)).
					Description(i18n.Tf("cmd.init.mcp_token_hint", envHint)).
					EchoMode(huh.EchoModePassword).
					Value(&token),
			),
		)
		if err := tokenForm.Run(); err != nil {
			continue
		}

		if token == "" {
			fmt.Fprintf(os.Stdout, "%s %s\n",
				common.WarningStyle.Render(common.IconWarning),
				i18n.Tf("cmd.init.mcp_token_skipped", svc))
			continue
		}

		keyName := svc + "-token"
		if err := a.Secrets.Set(ctx, keyName, token); err != nil {
			fmt.Fprintf(os.Stdout, "%s %s\n",
				common.ErrorStyle.Render(common.IconError),
				i18n.Tf("cmd.init.mcp_token_error", svc, err))
			continue
		}

		fmt.Fprintf(os.Stdout, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.init.mcp_token_stored", svc, keyName))

		// GitLab: ask about write permissions
		if svc == "gitlab" {
			var writeEnabled bool
			_ = huh.NewConfirm().
				Title(i18n.T("cmd.init.mcp_gitlab_write")).
				Description(i18n.T("cmd.init.mcp_gitlab_write_desc")).
				Value(&writeEnabled).
				Run()

			if writeEnabled {
				fmt.Fprintf(os.Stdout, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess),
					i18n.T("cmd.init.mcp_gitlab_write_enabled"))
			}
		}
	}
}

func mcpEnvHint(svc string) string {
	switch svc {
	case "figma":
		return "FIGMA_TOKEN"
	case "gitlab":
		return "GITLAB_TOKEN"
	case "gslides":
		return "GOOGLE_ACCESS_TOKEN"
	default:
		return ""
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider credential detection (inline in init)
// ─────────────────────────────────────────────────────────────────────────────

// initProviderCredentials detects existing credentials and offers to use them or configure new ones.
func initProviderCredentials(name providerPkg.Name, a *app.App, ctx context.Context) {
	fmt.Fprintf(os.Stdout, "\n  %s %s\n",
		common.Bold.Render(string(name)),
		common.Subtitle.Render("— "+providerPkg.Description(name)))

	// GitHub Copilot: just detect, no secret to store
	if name == providerPkg.GithubCopilot {
		det := providerPkg.Detect(name)
		if det.Available {
			fmt.Fprintf(os.Stdout, "  %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.provider.detected", det.Source, det.Details))
		} else {
			fmt.Fprintf(os.Stdout, "  %s %s\n",
				common.WarningStyle.Render(common.IconWarning),
				i18n.T("cmd.provider.copilot_not_found"))
		}
		return
	}

	det := providerPkg.Detect(name)
	if det.Available {
		fmt.Fprintf(os.Stdout, "  %s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.provider.detected", det.Source, det.Details))

		var useExisting bool
		if err := huh.NewConfirm().
			Title(i18n.T("cmd.provider.use_existing")).
			Value(&useExisting).
			Run(); err != nil {
			return
		}

		if useExisting {
			// For bedrock with aws-profile source: persist profile/region in hub.toml
			if name == providerPkg.Bedrock && det.Source == "aws-profile" {
				v := configViper()
				// Extract profile and region from Details ("profile default, region eu-west-1")
				v.Set("provider.bedrock.auth_mode", "profile")
				if awsProfile := os.Getenv("AWS_PROFILE"); awsProfile != "" {
					v.Set("provider.bedrock.aws_profile", awsProfile)
				} else {
					v.Set("provider.bedrock.aws_profile", "default")
				}
				if region := os.Getenv("AWS_REGION"); region != "" {
					v.Set("provider.bedrock.aws_region", region)
				} else if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
					v.Set("provider.bedrock.aws_region", region)
				}
				cfgPath := config.ConfigPath()
				_ = v.WriteConfigAs(cfgPath)
			}
			fmt.Fprintf(os.Stdout, "  %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.T("cmd.init.provider_existing_used"))
			return
		}
	} else {
		fmt.Fprintf(os.Stdout, "  %s %s\n",
			common.WarningStyle.Render(common.IconWarning),
			i18n.Tf("cmd.init.provider_not_detected", string(name)))
	}

	// Offer inline wizard
	var configureNow bool
	if err := huh.NewConfirm().
		Title(i18n.T("cmd.init.provider_configure_now")).
		Value(&configureNow).
		Run(); err != nil {
		return
	}

	if !configureNow {
		fmt.Fprintf(os.Stdout, "  %s %s\n",
			common.Subtitle.Render(common.IconArrow),
			i18n.T("cmd.init.provider_configure_later"))
		return
	}

	// Inline wizard — simplified version based on provider type
	switch name {
	case providerPkg.Bedrock:
		initBedrockWizard(a, ctx)
	case providerPkg.Anthropic, providerPkg.OpenRouter:
		initAPIKeyWizard(a, ctx, name)
	}
}

func initBedrockWizard(a *app.App, ctx context.Context) {
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
		return
	}

	v := configViper()
	v.Set("provider.bedrock.auth_mode", authMode)

	if authMode == "bearer" {
		var token, region string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.bearer_token")).
					EchoMode(huh.EchoModePassword).
					Value(&token),
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.region")).
					Placeholder("us-east-1").
					Value(&region),
			),
		).Run(); err != nil {
			return
		}
		if region == "" {
			region = "us-east-1"
		}
		v.Set("provider.bedrock.aws_region", region)

		if token != "" && a.Secrets != nil {
			keyName := providerPkg.KeychainKey(providerPkg.Bedrock, "")
			if err := a.Secrets.Set(ctx, keyName, token); err == nil {
				fmt.Fprintf(os.Stdout, "  %s %s\n",
					common.SuccessStyle.Render(common.IconSuccess),
					i18n.Tf("cmd.provider.token_stored", keyName))
			}
		}
	} else {
		var profile, region string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.profile")).
					Placeholder("default").
					Value(&profile),
				huh.NewInput().
					Title(i18n.T("cmd.provider.bedrock.region")).
					Placeholder("us-east-1").
					Value(&region),
			),
		).Run(); err != nil {
			return
		}
		if profile == "" {
			profile = "default"
		}
		if region == "" {
			region = "us-east-1"
		}
		v.Set("provider.bedrock.aws_profile", profile)
		v.Set("provider.bedrock.aws_region", region)
	}

	cfgPath := config.ConfigPath()
	_ = v.WriteConfigAs(cfgPath)
	fmt.Fprintf(os.Stdout, "  %s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.provider.hub_configured", "bedrock"))
}

func initAPIKeyWizard(a *app.App, ctx context.Context, name providerPkg.Name) {
	var apiKey string
	envVar := providerPkg.EnvVar(name)

	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(i18n.Tf("cmd.provider.api_key_prompt", string(name))).
				Description(i18n.Tf("cmd.provider.api_key_hint", envVar)).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	).Run(); err != nil {
		return
	}

	if apiKey == "" {
		fmt.Fprintf(os.Stdout, "  %s %s\n",
			common.WarningStyle.Render(common.IconWarning),
			i18n.T("cmd.provider.no_key"))
		return
	}

	if a.Secrets != nil {
		keyName := providerPkg.KeychainKey(name, "")
		if err := a.Secrets.Set(ctx, keyName, apiKey); err == nil {
			fmt.Fprintf(os.Stdout, "  %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.provider.token_stored", keyName))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Config generation
// ─────────────────────────────────────────────────────────────────────────────

// buildInitConfig generates the hub.toml content.
func buildInitConfig(language, opencodeVer, provider string, mcpServices []string) string {
	var sb strings.Builder
	sb.WriteString(`# oh — OpenHub CLI configuration
# Generated by oh init

[cli]
`)
	fmt.Fprintf(&sb, "language = %q\n", language)
	sb.WriteString(`
[opencode]
`)
	fmt.Fprintf(&sb, "version = %q\n", opencodeVer)
	fmt.Fprintf(&sb, "default_provider = %q\n", provider)
	sb.WriteString(`channel = "stable"
auto_update = false
install_dir = "~/.oh/bin"

[worktree]
auto_cleanup = true
base_branch = ""
`)

	mcpSet := make(map[string]bool)
	for _, s := range mcpServices {
		mcpSet[s] = true
	}

	sb.WriteString(`
[mcp.figma]
`)
	fmt.Fprintf(&sb, "enabled = %v\n", mcpSet["figma"])
	sb.WriteString(`token_key = "figma-token"
`)

	sb.WriteString(`
[mcp.gitlab]
`)
	fmt.Fprintf(&sb, "enabled = %v\n", mcpSet["gitlab"])
	sb.WriteString(`token_key = "gitlab-token"
`)

	sb.WriteString(`
[mcp.gslides]
`)
	fmt.Fprintf(&sb, "enabled = %v\n", mcpSet["gslides"])
	sb.WriteString(`token_key = "gslides-token"
`)

	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Git helpers
// ─────────────────────────────────────────────────────────────────────────────

// addGitExcludes adds opencode and oh artifacts to .git/info/exclude.
func addGitExcludes(projectPath string) {
	excludeFile := filepath.Join(projectPath, ".git", "info", "exclude")

	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err != nil {
		return
	}

	existing, _ := os.ReadFile(excludeFile)
	content := string(existing)

	patterns := []string{".opencode/", "opencode.json"}

	var toAdd []string
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			toAdd = append(toAdd, p)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	_ = os.MkdirAll(filepath.Dir(excludeFile), 0o755)

	f, err := os.OpenFile(excludeFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(content, "\n") {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString("\n# oh — OpenHub CLI artifacts\n")
	for _, p := range toAdd {
		_, _ = f.WriteString(p + "\n")
	}
}
