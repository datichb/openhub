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
	"github.com/datichb/openhub/cli/internal/domain"
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
		useExisting         bool
		configureNow        bool
		authMode            string
		bedrockToken        string
		bedrockRegion       string
		bedrockProfile      string
		bedrockRegionIdx    int  // current index in region DropDown; -1 = no selection
		bedrockCustomRegion bool // true when user selects "Custom..." in the region dropdown
		apiKey              string

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

	// bedrockRegionIdx starts at 0 (placeholder option) — the user must
	// actively select a region for validation to pass.

	// Team wizard state — shared with buildInitWizardTeamSteps closures.
	teamState := &initWizardTeamState{Ctx: ctx}
	appPtr := &a

	var steps []views.WizardStep
	steps = []views.WizardStep{
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
				langOptions := []string{i18n.T("cmd.init.wizard_lang_placeholder"), "Français", "English"}
				form.AddDropDown(i18n.T("cmd.init.language_select"), langOptions, 0, func(_ string, index int) {
					switch index {
					case 1:
						language = "fr"
					case 2:
						language = "en"
					default:
						language = ""
					}
				})
				form.AddInputField(i18n.T("cmd.init.opencode_version"), opencodeVer, 0, nil, func(text string) {
					opencodeVer = text
				})
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					{Label: i18n.T("cmd.init.info.language"), Value: language},
					{Label: i18n.T("cmd.init.info.opencode"), Value: opencodeVer},
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
					i18n.T("cmd.init.wizard_provider_placeholder"),
					"Amazon Bedrock",
					"Anthropic (direct API)",
					"OpenRouter",
					"GitHub Copilot",
				}
				providerValues := []string{"", "bedrock", "anthropic", "openrouter", "github-copilot"}
				form.AddDropDown(i18n.T("cmd.init.provider_select"), providerOptions, 0, func(_ string, index int) {
					if index >= 0 && index < len(providerValues) {
						provider = providerValues[index]
					}
				})
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					{Label: i18n.T("cmd.init.info.provider"), Value: provider},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 3 — Provider: detect existing credentials
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_detect_creds"),
			SkipIf: func() bool {
				// GitHub Copilot just detects, no wizard needed
				return provider == "github-copilot"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				name := providerPkg.Name(provider)
				det := providerPkg.Detect(name)

				if !det.Available && a != nil && a.Secrets != nil {
					if key := providerPkg.KeychainKey(name, ""); key != "" {
						if existing, err := a.Secrets.Get(ctx, key); err == nil && existing != "" {
							det.Available = true
							det.Source = "keychain"
							det.Details = "***"
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					return []views.InfoField{{Label: i18n.T("cmd.init.info.credentials"), Value: i18n.T("cmd.init.info.creds_detected")}}
				}
				if configureNow {
					return []views.InfoField{{Label: i18n.T("cmd.init.info.credentials"), Value: i18n.T("cmd.init.info.creds_configure")}}
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.info.credentials"), Value: i18n.T("cmd.init.info.skipped")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 4 — Bedrock: auth mode selection
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_bedrock_auth"),
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow
			},
			Validate: func() string {
				if authMode == "" {
					return i18n.T("cmd.init.wizard_auth_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				authOptions := []string{
					i18n.T("cmd.init.wizard_auth_placeholder"),
					"Bearer Token (SSO/STS)",
					"AWS Profile (~/.aws/credentials)",
					"Environment Variables (AWS_ACCESS_KEY_ID)",
				}
				form.AddDropDown(i18n.T("cmd.provider.bedrock.auth_mode"), authOptions, 0, func(_ string, index int) {
					switch index {
					case 1:
						authMode = "bearer"
					case 2:
						authMode = "profile"
					case 3:
						authMode = "env"
					default:
						authMode = ""
					}
				})
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AuthMode = authMode
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: i18n.T("cmd.init.info.auth_mode"), Value: authMode}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 5 — Bedrock: bearer token + region
		// ══════════════════════════════════════════════════════════════════════
		{
			ID:    "bedrock-bearer",
			Label: i18n.T("cmd.init.step_bedrock_bearer"),
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "bearer"
			},
			Validate: func() string {
				if bedrockRegion == "" {
					return i18n.T("cmd.init.wizard_region_required")
				}
				return ""
			},
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				stepIdx := bedrockStepIndex(&steps, "bedrock-bearer")
				rerenderSafe := func() {
					if stepIdx >= 0 {
						if fn := steps[stepIdx].Rerender; fn != nil {
							go func() { app.QueueUpdateDraw(func() { fn() }) }()
						}
					}
				}

				form := tview.NewForm()
				form.AddPasswordField(
					i18n.T("cmd.provider.bedrock.bearer_token"),
					"", 0, '*',
					func(text string) { bedrockToken = text },
				)
				addBedrockRegionDropDown(form, &bedrockRegion, &bedrockRegionIdx, &bedrockCustomRegion, rerenderSafe)
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
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
					{Label: i18n.T("cmd.init.info.region"), Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 6 — Bedrock: profile + region
		// ══════════════════════════════════════════════════════════════════════
		{
			ID:    "bedrock-profile",
			Label: i18n.T("cmd.init.step_bedrock_profile"),
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "profile"
			},
			Validate: func() string {
				if bedrockRegion == "" {
					return i18n.T("cmd.init.wizard_region_required")
				}
				return ""
			},
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				stepIdx := bedrockStepIndex(&steps, "bedrock-profile")
				rerenderSafe := func() {
					if stepIdx >= 0 {
						if fn := steps[stepIdx].Rerender; fn != nil {
							go func() { app.QueueUpdateDraw(func() { fn() }) }()
						}
					}
				}

				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.provider.bedrock.profile"), bedrockProfile, 0, nil, func(text string) {
					bedrockProfile = text
				})
				addBedrockRegionDropDown(form, &bedrockRegion, &bedrockRegionIdx, &bedrockCustomRegion, rerenderSafe)
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				if bedrockProfile == "" {
					bedrockProfile = "default"
				}
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AWSProfile = bedrockProfile
					c.Provider.Bedrock.AWSRegion = bedrockRegion
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.info.profile"), Value: bedrockProfile},
					{Label: "Region", Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 7 — Bedrock: env mode (region only)
		// ══════════════════════════════════════════════════════════════════════
		{
			ID:    "bedrock-env",
			Label: i18n.T("cmd.init.step_bedrock_env"),
			SkipIf: func() bool {
				return provider != "bedrock" || useExisting || !configureNow || authMode != "env"
			},
			Validate: func() string {
				if bedrockRegion == "" {
					return i18n.T("cmd.init.wizard_region_required")
				}
				return ""
			},
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				stepIdx := bedrockStepIndex(&steps, "bedrock-env")
				rerenderSafe := func() {
					if stepIdx >= 0 {
						if fn := steps[stepIdx].Rerender; fn != nil {
							go func() { app.QueueUpdateDraw(func() { fn() }) }()
						}
					}
				}

				form := tview.NewForm()
				addBedrockRegionDropDown(form, &bedrockRegion, &bedrockRegionIdx, &bedrockCustomRegion, rerenderSafe)
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				return config.Update(func(c *config.Config) error {
					c.Provider.Bedrock.AuthMode = "env"
					c.Provider.Bedrock.AWSRegion = bedrockRegion
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.info.auth_mode"), Value: "env"},
					{Label: "Region", Value: bedrockRegion},
				}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — Anthropic / OpenRouter: API key
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_api_key"),
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					return []views.InfoField{{Label: i18n.T("cmd.init.info.api_key"), Value: i18n.T("cmd.init.info.skipped")}}
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if configureMCP {
					return []views.InfoField{{Label: "MCP", Value: i18n.T("cmd.init.info.configure")}}
				}
				return []views.InfoField{{Label: "MCP", Value: i18n.T("cmd.init.info.skipped")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 10 — MCP: service selection (checkboxes)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_mcp_services"),
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					return []views.InfoField{{Label: i18n.T("cmd.init.info.services"), Value: i18n.T("cmd.init.info.none")}}
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.info.services"), Value: strings.Join(mcpServices, ", ")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 11 — MCP: Figma token
		// ══════════════════════════════════════════════════════════════════════
		buildMCPTokenStep(mcpTokenStepOpts{
			LabelI18nKey: "cmd.init.step_figma_token",
			DisplayName:  "Figma",
			TokenKey:     config.DefaultFigmaTokenKey,
			TokenVar:     &figmaToken,
			SkipIf:       func() bool { return !configureMCP || !mcpFigma },
			SecretsFunc:  func() domain.SecretStore { if a != nil { return a.Secrets }; return nil },
		}),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 12 — MCP: GitLab token
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_gitlab_token"),
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
					return []views.InfoField{{Label: "GitLab", Value: i18n.T("cmd.init.info.use_env_gitlab")}}
				}
				return []views.InfoField{{Label: "GitLab", Value: i18n.T("cmd.init.info.stored_keychain")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 13 — MCP: GitLab write permissions
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.step_gitlab_write"),
			SkipIf: func() bool {
				return !configureMCP || !mcpGitlab || gitlabToken == ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("cmd.init.mcp_gitlab_write"), false, func(checked bool) {
					gitlabWrite = checked
				})
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if gitlabWrite {
					return []views.InfoField{{Label: i18n.T("cmd.init.info.gitlab_write"), Value: i18n.T("cmd.init.info.enabled")}}
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.info.gitlab_write"), Value: i18n.T("cmd.init.info.read_only")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 14 — MCP: Google Slides token
		// ══════════════════════════════════════════════════════════════════════
		buildMCPTokenStep(mcpTokenStepOpts{
			LabelI18nKey: "cmd.init.step_gslides_token",
			DisplayName:  "Google Slides",
			TokenKey:     config.DefaultGslidesTokenKey,
			TokenVar:     &gslidesToken,
			SkipIf:       func() bool { return !configureMCP || !mcpGslides },
			SecretsFunc:  func() domain.SecretStore { if a != nil { return a.Secrets }; return nil },
		}),

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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
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
			Label:      i18n.T("cmd.init.step_finalize"),
			Processing: i18n.T("cmd.init.step_finalize_processing"),
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
				return []views.InfoField{{Label: i18n.T("cmd.init.info.status"), Value: i18n.T("cmd.init.info.config_saved")}}
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
				form.AddButton(i18n.T("wizard.hint.next"), func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if addProject {
					return []views.InfoField{{Label: i18n.T("cmd.init.info.project"), Value: i18n.T("cmd.init.info.will_add")}}
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.info.project"), Value: i18n.T("cmd.init.info.skipped")}}
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
		{Label: i18n.T("cmd.init.info.provider"), Value: provider},
	}
	if language != "" {
		fields = append(fields, summary.Field{Label: i18n.T("cmd.init.info.language"), Value: language})
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

// bedrockStepIndex finds a WizardStep by ID and returns its index, or -1.
func bedrockStepIndex(steps *[]views.WizardStep, id string) int {
	for i, s := range *steps {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// addBedrockRegionDropDown appends a region DropDown (with "Custom..." option)
// to the form. It mutates the shared state via pointers and calls rerenderSafe
// when the selection changes so the form rebuilds with/without the custom input.
func addBedrockRegionDropDown(form *tview.Form, region *string, regionIdx *int, custom *bool, rerenderSafe func()) {
	regionLabels := append(
		[]string{i18n.T("cmd.init.wizard_region_placeholder")},
		providerPkg.BedrockRegionLabels()...,
	)
	regionLabels = append(regionLabels, i18n.T("cmd.init.wizard_custom_region"))
	customIdx := len(regionLabels) - 1

	form.AddDropDown(
		i18n.T("cmd.init.wizard_aws_region"),
		regionLabels, *regionIdx,
		func(_ string, idx int) {
			if *regionIdx == idx {
				return // no change — avoid spurious rerender
			}
			wasCustom := *custom
			if idx == 0 {
				// Placeholder selected — clear region.
				*custom = false
				*region = ""
			} else if idx == customIdx {
				*custom = true
				*region = ""
			} else if idx > 0 {
				*custom = false
				*region = providerPkg.BedrockRegions[idx-1].Code // -1 for placeholder offset
			}
			*regionIdx = idx
			// Only rerender when the custom input field needs to appear/disappear.
			if *custom != wasCustom {
				rerenderSafe()
			}
		},
	)
	if *custom {
		form.AddInputField(i18n.T("cmd.init.wizard_custom_region_input"), *region, 0, nil, func(t string) { *region = t })
	}
}

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
// Returns an error if any file operation fails (other than a missing .git directory).
func addGitExcludes(projectPath string) error {
	excludeFile := filepath.Join(projectPath, ".git", "info", "exclude")

	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err != nil {
		return nil // not a git repo — nothing to do
	}

	existing, err := os.ReadFile(excludeFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", excludeFile, err)
	}
	content := string(existing)

	patterns := []string{".opencode/", "opencode.json"}

	var toAdd []string
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			toAdd = append(toAdd, p)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(excludeFile), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(excludeFile), err)
	}

	f, err := os.OpenFile(excludeFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", excludeFile, err)
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := f.WriteString("\n# oh — OpenHub CLI artifacts\n"); err != nil {
		return err
	}
	for _, p := range toAdd {
		if _, err := f.WriteString(p + "\n"); err != nil {
			return err
		}
	}
	return nil
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
