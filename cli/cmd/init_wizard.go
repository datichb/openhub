package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// needsFirstRunWizard returns true if the setup wizard has never been completed.
func needsFirstRunWizard(a *app.App) bool {
	return !a.Config.CLI.SetupDone
}

// buildFirstRunInlineWizard creates an InlineWizardView for the first-run setup.
// It runs inside the TUI shell and handles:
//  1. Welcome screen
//  2. Language selection (fr/en)
//  3. AI provider + credentials (optional, single dynamic form)
//  4. First project (optional)
//  5. Deploy agents/skills (conditional on project)
//  6. MCP Figma (optional)
//  7. MCP GitLab (optional)
//  8. MCP Google Slides (optional)
func buildFirstRunInlineWizard(a *app.App) *views.InlineWizardView {
	var (
		selectedLang     string
		selectedProvider string
		authMode         string
		token            string
		region           string
		profileName      string
		projectName      string
		projectPath      string
		projectCreated   bool
		deployConfirmed  bool
		figmaToken       string
		gitlabToken      string
		gitlabWrite      bool
		gslidesToken     string
		providerIdx      int  // current index in providerOptions DropDown
		authIdx          int  // current index in authModes DropDown (bedrock)
		providerSkipped  bool // true if user clicked "Ignorer" on the Provider intro
		projectSkipped   bool // true if user clicked "Ignorer" on the Project intro
		mcpSkipped       bool // true if user clicked "Ignorer" on the MCP intro
	)

	providerOptions := []string{"bedrock", "anthropic", "openrouter", "github-copilot"}

	// providerStepIdx is the 0-based index of the Provider form step in the
	// steps slice below. Used by the Form callback to access step.Rerender
	// (injected by the wizard engine at render time). Update if steps are
	// reordered. With intro pages: 0=Welcome, 1=Lang, 2=IntroProvider, 3=Provider.
	const providerStepIdx = 3

	var steps []views.WizardStep
	steps = []views.WizardStep{
		// ══════════════════════════════════════════════════════════════════════
		// STEP 0 — Welcome (group: Langue)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:         i18n.T("cmd.init.wizard_step_welcome"),
			Required:      true,
			SidebarHidden: true,
			CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
				accent := theme.ColorTag(theme.AccentHex)
				secondary := theme.ColorTag(theme.TextSecondaryHex)
				muted := theme.ColorTag(theme.TextMutedHex)
				reset := theme.TagColor

				tv := tview.NewTextView().
					SetDynamicColors(true).
					SetTextAlign(tview.AlignCenter)
				tv.SetBackgroundColor(theme.BgPanel)
				tv.SetBorderPadding(1, 0, 2, 2)
				tv.SetText(fmt.Sprintf(`%s██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s
%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s
%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s
%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s
%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s
%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s

%s`+i18n.T("cmd.init.wizard_welcome_title")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_desc")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_detail")+`%s

%s1.%s `+i18n.T("cmd.init.wizard_step_lang_desc")+`
%s2.%s `+i18n.T("cmd.init.wizard_step_provider_desc")+`
%s3.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
%s4.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
				))

				// Button form for "Get started"
				buttonForm := views.NewStyledButtonForm()
				buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_welcome_start")+"  ", onDone)

				// Unified layout: topSpacer + text(flex) + buttonForm(fixed) + bottomSpacer
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				bottomSpacer := tview.NewBox()
				bottomSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 3, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 5, 0, true)
				container.AddItem(bottomSpacer, 3, 0, false)
				tvApp.SetFocus(buttonForm)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Status", Value: i18n.T("cmd.init.wizard_started")}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 2 — Language
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:    i18n.T("cmd.init.wizard_step_lang"),
			Required: true,
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				langOptions := []string{"Français", "English"}
				form := tview.NewForm()
				form.AddDropDown(i18n.T("cmd.init.wizard_lang_select"), langOptions, 0, func(option string, _ int) {
					if option == "English" {
						selectedLang = "en"
					} else {
						selectedLang = "fr"
					}
				})
				selectedLang = "fr" // default
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				// Apply locale immediately so following steps use the right language
				i18n.SetLocale(selectedLang)

				// Persist to hub.toml
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				vip.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
				_ = vip.ReadInConfig()
				vip.Set("cli.language", selectedLang)
				return vip.WriteConfigAs(config.ConfigPath())
			},
			InfoFields: func() []views.InfoField {
				label := "Français"
				if selectedLang == "en" {
					label = "English"
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_lang"), Value: label}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 2 — Intro Provider (group: Provider)
		// ══════════════════════════════════════════════════════════════════════
		buildIntroStep(
			"Provider",
			i18n.T("cmd.init.wizard_intro_provider_title"),
			i18n.T("cmd.init.wizard_intro_provider_desc"),
			i18n.T("cmd.init.wizard_intro_provider_list"),
			"Bedrock · Anthropic · OpenRouter · Copilot",
			"",
			func() { providerSkipped = false },
			func() { providerSkipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 3 — Provider + Credentials (single dynamic form, optional)
		//
		// The form adapts its fields based on the selected provider (and auth
		// mode for bedrock). When a DropDown selection changes, we defer a
		// full step re-render via step.Rerender (injected by the wizard engine)
		// wrapped in go func() { app.QueueUpdateDraw(...) }() to avoid mutating
		// the widget tree from inside a tview handler callback.
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "Provider",
			SkipIf: func() bool {
				return providerSkipped
			},
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				// Initialize defaults on first render (zero values).
				// On subsequent re-renders (after a DropDown change), these
				// are already set by the SetSelectedFunc callback.
				if selectedProvider == "" {
					selectedProvider = providerOptions[0]
				}
				if authMode == "" {
					authMode = "bearer"
				}
				if region == "" {
					region = "us-east-1"
				}
				if profileName == "" {
					profileName = "default"
				}

				// rerenderSafe schedules a full step re-render on the next
				// tview event loop iteration. Safe to call from SetSelectedFunc.
				rerenderSafe := func() {
					if fn := steps[providerStepIdx].Rerender; fn != nil {
						go func() { app.QueueUpdateDraw(func() { fn() }) }()
					}
				}

				form := tview.NewForm()
				authModes := []string{"bearer", "profile", "env"}

				// ── Provider dropdown (always first) ──
				form.AddDropDown(
					i18n.T("cmd.init.wizard_provider_select"),
					providerOptions, providerIdx,
					func(_ string, idx int) {
						if providerIdx == idx {
							return // no change — avoid spurious rerender
						}
						providerIdx = idx
						selectedProvider = providerOptions[idx]
						// Reset credentials on provider change
						token = ""
						region = "us-east-1"
						profileName = "default"
						authMode = "bearer"
						authIdx = 0
						rerenderSafe()
					},
				)

				// ── Conditional fields based on current provider ──
				switch selectedProvider {
				case "bedrock":
					form.AddDropDown(
						i18n.T("cmd.init.wizard_auth_mode"),
						authModes, authIdx,
						func(_ string, idx int) {
							if authIdx == idx {
								return // no change — avoid spurious rerender
							}
							authIdx = idx
							authMode = authModes[idx]
							// Reset token/profile on auth change
							token = ""
							profileName = "default"
							rerenderSafe()
						},
					)
					switch authMode {
					case "bearer":
						form.AddPasswordField("Bearer token", "", 50, '*', func(t string) { token = t })
						form.AddInputField("AWS Region", region, 30, nil, func(t string) { region = t })
					case "profile":
						form.AddInputField("AWS Profile", profileName, 30, nil, func(t string) { profileName = t })
						form.AddInputField("AWS Region", region, 30, nil, func(t string) { region = t })
					case "env":
						form.AddInputField("AWS Region", region, 30, nil, func(t string) { region = t })
					}

				case "anthropic":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), "", 50, '*', func(t string) { token = t })

				case "openrouter":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), "", 50, '*', func(t string) { token = t })

				case "github-copilot":
					form.AddTextView("", i18n.T("cmd.init.wizard_copilot_desc"), 60, 2, true, false)
					form.AddTextView("", "$ gh auth login", 60, 1, true, false)
				}

				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				vip.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
				_ = vip.ReadInConfig()

				vip.Set("opencode.default_provider", selectedProvider)

				switch selectedProvider {
				case "bedrock":
					vip.Set("provider.bedrock.auth_mode", authMode)
					if region != "" {
						vip.Set("provider.bedrock.aws_region", region)
					}
					if authMode == "profile" && profileName != "" {
						vip.Set("provider.bedrock.aws_profile", profileName)
					}
					if authMode == "bearer" && token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.Bedrock, "")
						if keychainKey != "" {
							_ = a.Secrets.Set(context.Background(), keychainKey, token)
						}
					}
				case "anthropic":
					if token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.Anthropic, "")
						if keychainKey != "" {
							_ = a.Secrets.Set(context.Background(), keychainKey, token)
						}
					}
				case "openrouter":
					if token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.OpenRouter, "")
						if keychainKey != "" {
							_ = a.Secrets.Set(context.Background(), keychainKey, token)
						}
					}
				case "github-copilot":
					// No token needed — uses gh auth
				}

				return vip.WriteConfigAs(config.ConfigPath())
			},
			InfoFields: func() []views.InfoField {
				fields := []views.InfoField{{Label: "Provider", Value: selectedProvider}}
				if selectedProvider == "bedrock" {
					fields = append(fields, views.InfoField{Label: "Auth", Value: authMode})
					if region != "" {
						fields = append(fields, views.InfoField{Label: "Region", Value: region})
					}
				}
				return fields
			},
			Processing: i18n.T("cmd.init.wizard_processing_credentials"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 4 — Intro Projet (group: Projet)
		// ══════════════════════════════════════════════════════════════════════
		buildIntroStep(
			"Projet",
			i18n.T("cmd.init.wizard_intro_project_title"),
			i18n.T("cmd.init.wizard_intro_project_desc"),
			"", "",
			i18n.T("cmd.init.wizard_intro_project_optional"),
			func() { projectSkipped = false },
			func() { projectSkipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 5 — First project (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.wizard_step_project"),
			SkipIf: func() bool {
				return projectSkipped
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.init.wizard_project_name"), "", 40, nil, func(t string) { projectName = t })
				form.AddInputField(i18n.T("cmd.init.wizard_project_path"), ".", 50, nil, func(t string) { projectPath = t })
				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					if projectName == "" || projectPath == "" {
						return
					}
					if a.Projects != nil {
						p := &domain.Project{
							ID:     uuid.New().String()[:8],
							Name:   projectName,
							Path:   projectPath,
							Status: domain.ProjectStatusActive,
						}
						_ = a.Projects.Create(context.Background(), p)
						projectCreated = true
					}
					onDone()
				})
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_project"), Value: i18n.T("cmd.init.wizard_project_added")}}
			},
			Processing: i18n.T("cmd.init.wizard_processing_project"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 7 — Deploy agents/skills (conditional on project)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.wizard_step_deploy"),
			SkipIf: func() bool {
				return projectSkipped || !projectCreated
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				deployConfirmed = true // default to yes
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("cmd.init.wizard_deploy_confirm"), true, func(checked bool) {
					deployConfirmed = checked
				})
				// Add description as a text item
				form.AddTextView(
					"",
					i18n.T("cmd.init.wizard_deploy_desc"),
					60, 3, true, false,
				)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if !deployConfirmed || !projectCreated {
					return nil
				}
				// Reload app to pick up the new project + provider config
				newApp, err := ReloadApp()
				if err != nil {
					return fmt.Errorf("reload: %w", err)
				}
				a = newApp

				hubDir := findHubDir()
				if hubDir == "" {
					return nil // hub content not extracted yet, skip silently
				}

				// Build a minimal deploy plan for the new project
				plan := buildDeployPlan(a, projectPath, "", hubDir, selectedProvider, "", nil, nil, nil, nil)
				_, err = deploy.Execute(plan)
				return err
			},
			InfoFields: func() []views.InfoField {
				status := i18n.T("cmd.init.wizard_deploy_done")
				if !deployConfirmed {
					status = i18n.T("cmd.init.wizard_deploy_skipped")
				}
				return []views.InfoField{{Label: "Deploy", Value: status}}
			},
			Processing: i18n.T("cmd.init.wizard_deploy_processing"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 7 — Intro MCP (group: Intégrations)
		// ══════════════════════════════════════════════════════════════════════
		buildIntroStep(
			"MCP",
			i18n.T("cmd.init.wizard_intro_mcp_title"),
			i18n.T("cmd.init.wizard_intro_mcp_desc"),
			i18n.T("cmd.init.wizard_intro_mcp_list"),
			"Figma · GitLab · Google Slides",
			"",
			func() { mcpSkipped = false },
			func() { mcpSkipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — MCP Figma (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "MCP Figma",
			SkipIf: func() bool {
				return mcpSkipped
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "Figma"),
					"", 50, '*',
					func(t string) { figmaToken = t },
				)
				form.AddTextView("", i18n.Tf("cmd.init.mcp_token_hint", "FIGMA_TOKEN"), 60, 1, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if figmaToken == "" {
					return nil
				}
				if a.Secrets != nil {
					_ = a.Secrets.Set(context.Background(), config.DefaultFigmaTokenKey, figmaToken)
				}
				// Enable in config
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				_ = vip.ReadInConfig()
				vip.Set("mcp.figma.enabled", true)
				return vip.WriteConfigAs(config.ConfigPath())
			},
			InfoFields: func() []views.InfoField {
				v := i18n.T("cmd.init.wizard_mcp_configured")
				if figmaToken == "" {
					v = i18n.T("cmd.init.wizard_mcp_skipped")
				}
				return []views.InfoField{{Label: "Figma", Value: v}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 9 — MCP GitLab (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "MCP GitLab",
			SkipIf: func() bool {
				return mcpSkipped
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "GitLab"),
					"", 50, '*',
					func(t string) { gitlabToken = t },
				)
				form.AddTextView("", i18n.Tf("cmd.init.mcp_token_hint", "GITLAB_TOKEN"), 60, 1, true, false)
				form.AddCheckbox(i18n.T("cmd.init.mcp_gitlab_write"), false, func(checked bool) {
					gitlabWrite = checked
				})
				form.AddTextView("", i18n.T("cmd.init.mcp_gitlab_write_desc"), 60, 1, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if gitlabToken == "" {
					return nil
				}
				if a.Secrets != nil {
					_ = a.Secrets.Set(context.Background(), config.DefaultGitLabTokenKey, gitlabToken)
				}
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				_ = vip.ReadInConfig()
				vip.Set("mcp.gitlab.enabled", true)
				if gitlabWrite {
					vip.Set("mcp.gitlab.write", true)
				}
				return vip.WriteConfigAs(config.ConfigPath())
			},
			InfoFields: func() []views.InfoField {
				v := i18n.T("cmd.init.wizard_mcp_configured")
				if gitlabToken == "" {
					v = i18n.T("cmd.init.wizard_mcp_skipped")
				}
				fields := []views.InfoField{{Label: "GitLab", Value: v}}
				if gitlabToken != "" && gitlabWrite {
					fields = append(fields, views.InfoField{Label: "Write", Value: i18n.T("cmd.init.wizard_mcp_enabled")})
				}
				return fields
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 10 — MCP Google Slides (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "MCP Google Slides",
			SkipIf: func() bool {
				return mcpSkipped
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "Google Slides"),
					"", 50, '*',
					func(t string) { gslidesToken = t },
				)
				form.AddTextView("", i18n.Tf("cmd.init.mcp_token_hint", "GOOGLE_ACCESS_TOKEN"), 60, 1, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if gslidesToken == "" {
					return nil
				}
				if a.Secrets != nil {
					_ = a.Secrets.Set(context.Background(), config.DefaultGslidesTokenKey, gslidesToken)
				}
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				_ = vip.ReadInConfig()
				vip.Set("mcp.gslides.enabled", true)
				return vip.WriteConfigAs(config.ConfigPath())
			},
			InfoFields: func() []views.InfoField {
				v := i18n.T("cmd.init.wizard_mcp_configured")
				if gslidesToken == "" {
					v = i18n.T("cmd.init.wizard_mcp_skipped")
				}
				return []views.InfoField{{Label: "Google Slides", Value: v}}
			},
		},
	}

	return views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.init",
		Title: i18n.T("cmd.init.wizard_title"),
		Steps: steps,
		Groups: []views.StepGroup{
			{Label: i18n.T("cmd.init.wizard_group_lang"), StartIdx: 0},
			{Label: i18n.T("cmd.init.wizard_group_provider"), StartIdx: 2},
			{Label: i18n.T("cmd.init.wizard_group_project"), StartIdx: 4},
			{Label: i18n.T("cmd.init.wizard_group_mcp"), StartIdx: 7},
		},
		SummaryTargetViewFunc: func() string {
			if projectCreated {
				return "project.mode"
			}
			return "home"
		},
		SummaryTargetLabelFunc: func() string {
			if projectCreated {
				return i18n.T("wizard.summary.goto_projects")
			}
			return i18n.T("wizard.summary.go_home")
		},
		OnComplete: func(completed bool, err error) {
			if completed && err == nil {
				// Mark setup as done so the wizard doesn't re-launch
				vip := viper.New()
				vip.SetConfigName("hub")
				vip.SetConfigType("toml")
				vip.AddConfigPath(config.HubDir())
				vip.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
				_ = vip.ReadInConfig()
				vip.Set("cli.setup_done", true)
				_ = vip.WriteConfigAs(config.ConfigPath())

				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					_ = newApp
				}
			}
		},
	})
}

// buildIntroStep creates a CustomView step with a rounded badge, title,
// description, optional list of items, and "Continue" / "Skip" buttons.
// Used for group introduction pages in the first-run wizard.
// When onSkip is non-nil, a "Skip" button is added alongside "Continue".
// When onContinue is non-nil, it is called when "Continue" is clicked
// (use to reset a skip flag when the user goes back and re-enters a group).
func buildIntroStep(badge, title, desc, listTitle, listItems, note string, onContinue, onSkip func()) views.WizardStep {
	return views.WizardStep{
		Label:         badge,
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			var b strings.Builder
			b.WriteString("\n")

			// Title
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, title, reset)

			// Description
			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")

			// Optional list
			if listTitle != "" && listItems != "" {
				fmt.Fprintf(&b, "%s%s%s\n", muted, listTitle, reset)
				fmt.Fprintf(&b, "%s%s%s\n", accent, listItems, reset)
				b.WriteString("\n")
			}

			// Optional note
			if note != "" {
				for _, line := range strings.Split(note, "\n") {
					fmt.Fprintf(&b, "%s%s%s\n", muted, line, reset)
				}
				b.WriteString("\n")
			}

			tv := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter)
			tv.SetBackgroundColor(theme.BgPanel)
			tv.SetText(b.String())

			// Button form (same style as wizard engine's buttonForm)
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.intro.continue")+"  ", func() {
				if onContinue != nil {
					onContinue()
				}
				onDone()
			})
			if onSkip != nil {
				buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
					onSkip()
					onDone()
				})
			}

			// Arrow keys navigate between buttons
			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})

			// Unified layout: topSpacer + badge + gap + content + buttonForm + bottomSpacer
			topSpacer := tview.NewBox()
			topSpacer.SetBackgroundColor(theme.BgPanel)
			badgeView := views.BuildStepBadge(badge)
			gapSpacer := tview.NewBox()
			gapSpacer.SetBackgroundColor(theme.BgPanel)
			bottomSpacer := tview.NewBox()
			bottomSpacer.SetBackgroundColor(theme.BgPanel)

			container.AddItem(topSpacer, 3, 0, false)
			container.AddItem(badgeView, 5, 0, false)
			container.AddItem(gapSpacer, 2, 0, false)
			container.AddItem(tv, 0, 1, false)
			container.AddItem(buttonForm, 5, 0, true)
			container.AddItem(bottomSpacer, 3, 0, false)
			tvApp.SetFocus(buttonForm)
		},
	}
}
