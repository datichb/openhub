package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: i18n.T("cmd.init.short"),
	Long:  i18n.T("cmd.init.long"),
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// ─────────────────────────────────────────────────────────────────────────────
// Main init flow
// ─────────────────────────────────────────────────────────────────────────────

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// ── Guard: warn if hub is already initialized ────────────────────────────
	if existing, err := config.Load(); err == nil && existing.CLI.SetupDone {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.init.already_initialized"))
		fmt.Fprintf(os.Stderr, "%s ", i18n.T("cmd.init.confirm_reinit"))
		var answer string
		if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil || (answer != "y" && answer != "Y") {
			return nil
		}
		// Backup existing config before overwriting.
		cfgPath := config.ConfigPath()
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = os.WriteFile(cfgPath+".bak", data, 0o600)
		}
	}

	// ── Lockfile: prevent concurrent wizard runs ─────────────────────────────
	lockPath := filepath.Join(config.HubDir(), "init.lock")
	if err := acquireInitLock(lockPath); err != nil {
		return fmt.Errorf("another wizard is already running: %w", err)
	}
	defer releaseInitLock(lockPath)

	// ── Pre-flight: check git availability ───────────────────────────────────
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "  %s %s\n", "⚠", i18n.T("cmd.init.git_not_found"))
	}

	// ── Shared state across wizard steps ──────────────────────────────────────
	var (
		language    string
		opencodeVer string
		provider    string

		// Provider credential state
		useExisting    bool
		configureNow   bool
		authMode       string
		bedrockToken   string
		bedrockRegion  string
		bedrockProfile string
		apiKey         string

		// MCP state
		configureMCP bool
		mcpFigma     bool
		mcpGitlab    bool
		mcpGslides   bool
		mcpServices  []string
		figmaToken   string
		gitlabToken  string
		gslidesToken string
		gitlabWrite  bool

		// Team state
		configureTeam bool

		// Project state
		addProject bool

		// App ref (set after config is written)
		a *app.App
	)

	// Team wizard state — shared with buildInitWizardTeamSteps closures.
	teamState := &initWizardTeamState{}
	appPtr := &a

	steps := []views.WizardStep{
		// ══════════════════════════════════════════════════════════════════════
		// STEP 1 — Language & OpenCode version
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:    i18n.T("cmd.init.section_general"),
			Required: true,
			Validate: func() string {
				if language == "" {
					return i18n.T("cmd.init.wizard_lang_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				langOptions := []string{"Français", "English"}
				form.AddDropDown(i18n.T("cmd.init.language_select"), langOptions, -1, func(_ string, index int) {
					switch index {
					case 0:
						language = "fr"
					case 1:
						language = "en"
					}
				})
				form.AddInputField(i18n.T("cmd.init.opencode_version"), "", 0, nil, func(text string) {
					opencodeVer = text
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if opencodeVer == "" {
					opencodeVer = "latest"
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Language", Value: language},
					{Label: "OpenCode", Value: opencodeVer},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 2 — Provider selection
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.section_provider"),
			Validate: func() string {
				if provider == "" {
					return i18n.T("cmd.init.wizard_provider_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				providerOptions := []string{
					"Amazon Bedrock",
					"Anthropic (direct API)",
					"OpenRouter",
					"GitHub Copilot",
				}
				providerValues := []string{"bedrock", "anthropic", "openrouter", "github-copilot"}
				form.AddDropDown(i18n.T("cmd.init.provider_select"), providerOptions, -1, func(_ string, index int) {
					if index >= 0 && index < len(providerValues) {
						provider = providerValues[index]
					}
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				// Write initial hub.toml (provider set, MCP all disabled) to allow initApp
				cfgDir := config.HubDir()
				if err := os.MkdirAll(cfgDir, 0o755); err != nil {
					return fmt.Errorf("creating config directory: %w", err)
				}

				initialCfg := buildInitialConfig(language, opencodeVer, provider, nil, detectBranchPatternHeuristic("."))
				if err := config.Save(initialCfg); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}

				// Initialize app so we can access secrets/keychain for provider credentials
				config.Reset()
				if err := initApp(); err != nil {
					return err
				}

				a = MustApp()
				if err := ensureOpencode(a); err != nil {
					return err
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Provider", Value: provider},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 3 — Provider: detect existing credentials
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Detect credentials",
			SkipIf: func() bool {
				// GitHub Copilot just detects, no wizard needed
				return provider == "github-copilot"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				name := providerPkg.Name(provider)
				det := providerPkg.Detect(name)

				// Also check keychain for existing token (Detect only checks env/system)
				if !det.Available && a != nil && a.Secrets != nil {
					if key := providerPkg.KeychainKey(name, ""); key != "" {
						if existing, err := a.Secrets.Get(ctx, key); err == nil && existing != "" {
							det.Available = true
							det.Source = "keychain"
							if len(existing) > 8 {
								det.Details = existing[:4] + "..." + existing[len(existing)-4:]
							} else {
								det.Details = "***"
							}
						}
					}
				}

				form := tview.NewForm()
				if det.Available {
					form.AddCheckbox(
						fmt.Sprintf("%s (%s: %s)", i18n.T("cmd.provider.use_existing"), det.Source, det.Details),
						true,
						func(checked bool) { useExisting = checked },
					)
				} else {
					// No existing credentials — ask if user wants to configure now
					useExisting = false
					form.AddCheckbox(
						i18n.T("cmd.init.provider_configure_now"),
						true,
						func(checked bool) { configureNow = checked },
					)
				}
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if useExisting {
					name := providerPkg.Name(provider)
					det := providerPkg.Detect(name)
					// For bedrock with aws-profile source: persist profile/region in hub.toml
					if name == providerPkg.Bedrock && det.Source == "aws-profile" {
						if err := config.Update(func(c *config.Config) error {
							c.Provider.Bedrock.AuthMode = "profile"
							if awsProfile := os.Getenv("AWS_PROFILE"); awsProfile != "" {
								c.Provider.Bedrock.AWSProfile = awsProfile
							} else {
								c.Provider.Bedrock.AWSProfile = "default"
							}
							if region := os.Getenv("AWS_REGION"); region != "" {
								c.Provider.Bedrock.AWSRegion = region
							} else if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
								c.Provider.Bedrock.AWSRegion = region
							}
							return nil
						}); err != nil {
							return fmt.Errorf("write config: %w", err)
						}
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if useExisting {
					return []views.InfoField{{Label: "Credentials", Value: "existing (detected)"}}
				}
				if configureNow {
					return []views.InfoField{{Label: "Credentials", Value: "configure now"}}
				}
				return []views.InfoField{{Label: "Credentials", Value: "skipped"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 4 — Bedrock: auth mode selection
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Bedrock auth mode",
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				authOptions := []string{
					"Bearer Token (SSO/STS)",
					"AWS Profile (~/.aws/credentials)",
					"Environment Variables (AWS_ACCESS_KEY_ID)",
				}
				form.AddDropDown(i18n.T("cmd.provider.bedrock.auth_mode"), authOptions, -1, func(_ string, index int) {
					switch index {
					case 0:
						authMode = "bearer"
					case 1:
						authMode = "profile"
					case 2:
						authMode = "env"
					}
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AuthMode = authMode
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth mode", Value: authMode}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 5 — Bedrock: bearer token + region
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Bedrock bearer token",
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "bearer"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.T("cmd.provider.bedrock.bearer_token"),
					"", 0, '*',
					func(text string) { bedrockToken = text },
				)
				form.AddInputField(i18n.T("cmd.provider.bedrock.region"), "", 0, nil, func(text string) {
					bedrockRegion = text
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if bedrockRegion == "" {
					bedrockRegion = "us-east-1"
				}
				if err := config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AWSRegion = bedrockRegion
					return nil
				}); err != nil {
					return fmt.Errorf("write config: %w", err)
				}

				if bedrockToken != "" && a != nil && a.Secrets != nil {
					keyName := providerPkg.KeychainKey(providerPkg.Bedrock, "")
					if err := a.Secrets.Set(ctx, keyName, bedrockToken); err != nil {
						return fmt.Errorf("storing bedrock token: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				stored := "stored"
				if bedrockToken == "" {
					stored = "skipped"
				}
				return []views.InfoField{
					{Label: "Token", Value: stored},
					{Label: "Region", Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 6 — Bedrock: profile + region
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Bedrock AWS profile",
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "profile"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.provider.bedrock.profile"), "", 0, nil, func(text string) {
					bedrockProfile = text
				})
				form.AddInputField(i18n.T("cmd.provider.bedrock.region"), "", 0, nil, func(text string) {
					bedrockRegion = text
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if bedrockProfile == "" {
					bedrockProfile = "default"
				}
				if bedrockRegion == "" {
					bedrockRegion = "us-east-1"
				}
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AWSProfile = bedrockProfile
					c.Provider.Bedrock.AWSRegion = bedrockRegion
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Profile", Value: bedrockProfile},
					{Label: "Region", Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 7 — Bedrock: env mode (region only)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Bedrock env config",
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "env"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.provider.bedrock.region"), "", 0, nil, func(text string) {
					bedrockRegion = text
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if bedrockRegion == "" {
					bedrockRegion = "us-east-1"
				}
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AuthMode = "env"
					c.Provider.Bedrock.AWSRegion = bedrockRegion
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Auth", Value: "env"},
					{Label: "Region", Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — Anthropic / OpenRouter: API key
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "API Key",
			SkipIf: func() bool {
				return (provider != "anthropic" && provider != "openrouter") || useExisting || !configureNow
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				name := providerPkg.Name(provider)
				envVar := providerPkg.EnvVar(name)
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
			OnDone: func() error {
				if apiKey == "" {
					return nil
				}
				if a != nil && a.Secrets != nil {
					name := providerPkg.Name(provider)
					keyName := providerPkg.KeychainKey(name, "")
					if err := a.Secrets.Set(ctx, keyName, apiKey); err != nil {
						return fmt.Errorf("storing API key: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if apiKey == "" {
					return []views.InfoField{{Label: "API Key", Value: "skipped"}}
				}
				masked := "***"
				if len(apiKey) > 8 {
					masked = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
				}
				return []views.InfoField{{Label: "API Key", Value: masked}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 9 — MCP: configure?
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.section_mcp"),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("cmd.init.mcp_configure_prompt"), true, func(checked bool) {
					configureMCP = checked
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if configureMCP {
					return []views.InfoField{{Label: "MCP", Value: "configure"}}
				}
				return []views.InfoField{{Label: "MCP", Value: "skipped"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 10 — MCP: service selection (checkboxes)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "MCP services",
			SkipIf: func() bool {
				return !configureMCP
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox("Figma (personal access token, scope file:read)", false, func(checked bool) {
					mcpFigma = checked
				})
				form.AddCheckbox("GitLab (personal access token, scope api)", false, func(checked bool) {
					mcpGitlab = checked
				})
				form.AddCheckbox("Google Slides (OAuth access token)", false, func(checked bool) {
					mcpGslides = checked
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				mcpServices = nil
				if mcpFigma {
					mcpServices = append(mcpServices, "figma")
				}
				if mcpGitlab {
					mcpServices = append(mcpServices, "gitlab")
				}
				if mcpGslides {
					mcpServices = append(mcpServices, "gslides")
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if len(mcpServices) == 0 {
					return []views.InfoField{{Label: "Services", Value: "none"}}
				}
				return []views.InfoField{{Label: "Services", Value: strings.Join(mcpServices, ", ")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 11 — MCP: Figma token
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Figma token",
			SkipIf: func() bool {
				return !configureMCP || !mcpFigma
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "figma"),
					"", 0, '*',
					func(text string) { figmaToken = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if figmaToken == "" {
					return nil
				}
				if a != nil && a.Secrets != nil {
					if err := a.Secrets.Set(ctx, config.DefaultFigmaTokenKey, figmaToken); err != nil {
						return fmt.Errorf("storing figma token: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if figmaToken == "" {
					return []views.InfoField{{Label: "Figma", Value: "use env FIGMA_TOKEN"}}
				}
				return []views.InfoField{{Label: "Figma", Value: "stored in keychain"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 12 — MCP: GitLab token
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "GitLab token",
			SkipIf: func() bool {
				return !configureMCP || !mcpGitlab
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "gitlab"),
					"", 0, '*',
					func(text string) { gitlabToken = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if gitlabToken == "" {
					return nil
				}
				if a != nil && a.Secrets != nil {
					if err := a.Secrets.Set(ctx, config.DefaultGitLabTokenKey, gitlabToken); err != nil {
						return fmt.Errorf("storing gitlab token: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if gitlabToken == "" {
					return []views.InfoField{{Label: "GitLab", Value: "use env GITLAB_TOKEN"}}
				}
				return []views.InfoField{{Label: "GitLab", Value: "stored in keychain"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 13 — MCP: GitLab write permissions
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "GitLab write mode",
			SkipIf: func() bool {
				return !configureMCP || !mcpGitlab || gitlabToken == ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("cmd.init.mcp_gitlab_write"), false, func(checked bool) {
					gitlabWrite = checked
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if gitlabWrite {
					return []views.InfoField{{Label: "GitLab write", Value: "enabled"}}
				}
				return []views.InfoField{{Label: "GitLab write", Value: "read-only"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 14 — MCP: Google Slides token
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Google Slides token",
			SkipIf: func() bool {
				return !configureMCP || !mcpGslides
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "gslides"),
					"", 0, '*',
					func(text string) { gslidesToken = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if gslidesToken == "" {
					return nil
				}
				if a != nil && a.Secrets != nil {
					if err := a.Secrets.Set(ctx, config.DefaultGslidesTokenKey, gslidesToken); err != nil {
						return fmt.Errorf("storing gslides token: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if gslidesToken == "" {
					return []views.InfoField{{Label: "Google Slides", Value: "use env GOOGLE_ACCESS_TOKEN"}}
				}
				return []views.InfoField{{Label: "Google Slides", Value: "stored in keychain"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 15 — Team: create / rejoin / skip
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.section_team"),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				modeOptions := []string{
					i18n.T("cmd.init.wizard_team_mode_init"),
					i18n.T("cmd.init.wizard_team_mode_rejoin"),
					i18n.T("wizard.intro.skip"),
				}
				// Default to "skip" (index 2)
				form.AddDropDown(i18n.T("cmd.init.wizard_team_mode_prompt"), modeOptions, 2,
					func(_ string, idx int) {
						switch idx {
						case 0:
							teamState.Mode = "init"
							teamState.Skipped = false
							configureTeam = true
						case 1:
							teamState.Mode = "rejoin"
							teamState.Skipped = false
							configureTeam = true
						case 2:
							teamState.Mode = ""
							teamState.Skipped = true
							configureTeam = false
						}
					})
				form.AddTextView("", i18n.T("cmd.init.team_configure_hint"), 60, 3, true, false)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if !configureTeam {
					teamState.Skipped = true
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				switch teamState.Mode {
				case "init":
					return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_team"), Value: i18n.T("cmd.init.wizard_team_mode_init")}}
				case "rejoin":
					return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_team"), Value: i18n.T("cmd.init.wizard_team_mode_rejoin")}}
				default:
					return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_team"), Value: "skipped"}}
				}
			},
		},
	}

	// Inject team init wizard steps (form + processing), skipped if mode != "init".
	steps = append(steps, buildInitWizardTeamSteps(appPtr, teamState)...)

	// Inject team rejoin wizard steps (repo + creds + member + validate), skipped if mode != "rejoin".
	steps = append(steps, buildInitWizardRejoinSteps(appPtr, teamState)...)

	// Continue with finalize + project steps.
	steps = append(steps,

		// ══════════════════════════════════════════════════════════════════════
		// STEP 16 — Update config + extract hub content
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			Label:      "Finalize config",
			Processing: "Writing configuration...",
			OnDone: func() error {
				// Update hub.toml with MCP enabled flags
				if len(mcpServices) > 0 {
					if err := updateConfigMCP(mcpServices); err != nil {
						return fmt.Errorf("updating MCP config: %w", err)
					}
				}

				// Extract hub content
				hubContentDir := hubcontent.HubContentDir()
				if err := hubcontent.Extract(hubContentDir); err != nil {
					return fmt.Errorf("extracting hub content: %w", err)
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Status", Value: "config saved"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 17 — Add first project?
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			Label: i18n.T("cmd.init.section_project"),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("cmd.init.add_project_prompt"), false, func(checked bool) {
					addProject = checked
				})
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if addProject {
					return []views.InfoField{{Label: "Project", Value: "will add"}}
				}
				return []views.InfoField{{Label: "Project", Value: "skip"}}
			},
		},
	)

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: "oh",
			Command:     "init",
			StatusHints: i18n.T("wizard.hints.default"),
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
	}

	// Mark setup as done so the TUI wizard doesn't re-trigger on next launch.
	_ = config.Update(func(c *config.Config) error {
		c.CLI.SetupDone = true
		return nil
	})

	// Post-wizard: add project if requested
	if addProject {
		if a == nil {
			config.Reset()
			if err := initApp(); err != nil {
				return err
			}
			a = MustApp()
		}
		return runProjectAddInteractive(ctx, a)
	}

	// Final summary card
	fields := []summary.Field{
		{Label: "Provider", Value: provider},
	}
	if language != "" {
		fields = append(fields, summary.Field{Label: "Language", Value: language})
	}
	if len(mcpServices) > 0 {
		fields = append(fields, summary.Field{Label: "MCP", Value: strings.Join(mcpServices, ", ")})
	}
	fmt.Fprint(os.Stdout, summary.Render(summary.Config{
		Title:     i18n.T("cmd.init.done_no_project"),
		Icon:      theme.IconSuccess,
		IconColor: theme.LipSuccess,
		Fields:    fields,
		Footer:    i18n.Tf("cmd.init.done_hint", "oh project add"),
	}))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP helpers
// ─────────────────────────────────────────────────────────────────────────────

// updateConfigMCP enables the selected MCP services in hub.toml.
func updateConfigMCP(mcpServices []string) error {
	return config.Update(func(c *config.Config) error {
		for _, svc := range mcpServices {
			if s := c.MCPServer(svc); s != nil {
				s.Enabled = true
			}
		}
		return nil
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Config generation
// ─────────────────────────────────────────────────────────────────────────────

// buildInitialConfig creates the initial hub.toml Config struct.
// branchPattern is optional: if non-empty it is written into [worktree].branch_pattern.
func buildInitialConfig(language, opencodeVer, provider string, mcpServices []string, branchPattern string) *config.Config {
	c := &config.Config{
		CLI: config.CLIConfig{Language: language},
		Opencode: config.OpencodeConfig{
			Version:         opencodeVer,
			DefaultProvider: provider,
			Channel:         "stable",
			AutoUpdate:      false,
			InstallDir:      filepath.Join(config.HubDir(), "bin"),
		},
		Worktree: config.WorktreeConfig{
			AutoCleanup:   true,
			BranchPattern: branchPattern,
		},
		MCP: config.MCPConfig{
			Figma:   config.MCPServerConfig{Token: config.DefaultFigmaTokenKey},
			Gitlab:  config.MCPServerConfig{Token: config.DefaultGitLabTokenKey},
			Gslides: config.MCPServerConfig{Token: config.DefaultGslidesTokenKey},
		},
	}
	for _, svc := range mcpServices {
		if s := c.MCPServer(svc); s != nil {
			s.Enabled = true
		}
	}
	return c
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

// detectBranchPatternHeuristic scans local and remote branches in dir to infer
// the project's branch naming convention and returns a fmt.Sprintf-style pattern
// (e.g. "feat/%s") suitable for [worktree].branch_pattern.
//
// It counts the frequency of common prefixes in all branch names, picks the
// dominant one (>50% of non-main/master branches), and returns "<prefix>/%s".
// Returns an empty string when no dominant pattern is found or on git error.
func detectBranchPatternHeuristic(dir string) string {
	cmd := exec.Command("git", "branch", "-a", "--format=%(refname:short)")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Known prefixes to look for (ordered by priority)
	knownPrefixes := []string{
		"feat", "feature", "fix", "bugfix", "hotfix",
		"chore", "refactor", "docs", "test", "ci",
	}

	// Skip main/master/HEAD lines
	skip := map[string]bool{
		"main": true, "master": true, "develop": true, "HEAD": true,
	}

	counts := make(map[string]int)
	total := 0

	for _, raw := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(raw)
		// Strip "origin/" prefix for remote branches
		branch = strings.TrimPrefix(branch, "origin/")
		if branch == "" || skip[branch] {
			continue
		}
		total++
		for _, prefix := range knownPrefixes {
			if strings.HasPrefix(branch, prefix+"/") {
				counts[prefix]++
				break
			}
		}
	}

	if total == 0 {
		return ""
	}

	// Find the prefix with the highest count
	type kv struct {
		prefix string
		count  int
	}
	var ranked []kv
	for _, prefix := range knownPrefixes {
		if counts[prefix] > 0 {
			ranked = append(ranked, kv{prefix, counts[prefix]})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })

	if len(ranked) == 0 {
		return ""
	}

	// Only suggest a pattern if the dominant prefix represents more than 50% of branches
	dominant := ranked[0]
	if dominant.count*2 > total {
		return dominant.prefix + "/%s"
	}

	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Lockfile helpers
// ─────────────────────────────────────────────────────────────────────────────

// acquireInitLock creates a lockfile to prevent concurrent wizard runs.
// The lock is considered stale after 1 hour.
func acquireInitLock(path string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	// Check for stale lock (older than 1 hour)
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) > time.Hour {
			_ = os.Remove(path) // stale, remove it
		} else {
			return fmt.Errorf("lockfile %s exists (created %s ago)", path, time.Since(info.ModTime()).Truncate(time.Second))
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return f.Close()
}

// releaseInitLock removes the lockfile.
func releaseInitLock(path string) {
	_ = os.Remove(path)
}
