package cmd

import (
	"context"
	"fmt"
	"path/filepath"

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

// needsFirstRunWizard returns true if no provider is configured.
func needsFirstRunWizard(a *app.App) bool {
	return a.Config.Opencode.DefaultProvider == ""
}

// buildFirstRunInlineWizard creates an InlineWizardView for the first-run setup.
// It runs inside the TUI shell and handles:
//  1. Welcome screen
//  2. Language selection (fr/en)
//  3. AI provider selection
//  4. Bedrock auth mode (conditional)
//  5. Credentials
//  6. First project (optional)
//  7. Deploy agents/skills (conditional on project)
//  8. MCP Figma (optional)
//  9. MCP GitLab (optional)
//  10. MCP Google Slides (optional)
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
	)

	providerOptions := []string{"bedrock", "anthropic", "openrouter", "github-copilot"}

	steps := []views.WizardStep{
		// ══════════════════════════════════════════════════════════════════════
		// STEP 1 — Welcome
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:    i18n.T("cmd.init.wizard_step_welcome"),
			Required: true,
			CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
				accent := theme.ColorTag(theme.AccentHex)
				muted := theme.ColorTag(theme.TextMutedHex)
				reset := theme.TagColor

				tv := tview.NewTextView().
					SetDynamicColors(true).
					SetTextAlign(tview.AlignLeft)
				tv.SetBackgroundColor(theme.BgPanel)
				tv.SetBorderPadding(1, 0, 4, 4)
				tv.SetText(fmt.Sprintf(`%s██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s
%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s
%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s
%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s
%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s
%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s

  %s`+i18n.T("cmd.init.wizard_welcome_title")+`%s

  `+i18n.T("cmd.init.wizard_welcome_desc")+`

  %s1.%s `+i18n.T("cmd.init.wizard_step_lang_desc")+`
  %s2.%s `+i18n.T("cmd.init.wizard_step_provider_desc")+`
  %s3.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
  %s4.%s `+i18n.T("cmd.init.wizard_step_deploy_desc")+`
  %s5.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`

  %s`+i18n.T("cmd.init.wizard_press_enter")+`%s
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset,
					muted, reset,
				))

				tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					if event.Key() == tcell.KeyEnter {
						onDone()
						return nil
					}
					return event
				})

				container.AddItem(tv, 0, 1, true)
				tvApp.SetFocus(tv)
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
		// STEP 3 — Provider selection
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:    "Provider",
			Required: true,
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown(i18n.T("cmd.init.wizard_provider_select"), providerOptions, 0, func(option string, _ int) {
					selectedProvider = option
				})
				selectedProvider = providerOptions[0]
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Provider", Value: selectedProvider}}
			},
			Processing: i18n.T("cmd.init.wizard_processing_provider"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 4 — Bedrock auth mode (conditional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.wizard_auth_mode"),
			SkipIf: func() bool {
				return selectedProvider != "bedrock"
			},
			Required: true,
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				authModes := []string{"bearer", "profile", "env"}
				form := tview.NewForm()
				form.AddDropDown(i18n.T("cmd.init.wizard_auth_mode"), authModes, 0, func(option string, _ int) {
					authMode = option
				})
				authMode = authModes[0]
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth", Value: authMode}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 5 — Credentials
		// ══════════════════════════════════════════════════════════════════════
		{
			Label:    "Credentials",
			Required: true,
			SkipIf: func() bool {
				return selectedProvider == "github-copilot"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()

				switch selectedProvider {
				case "bedrock":
					switch authMode {
					case "bearer":
						form.AddPasswordField("Bearer token", "", 50, '*', func(t string) { token = t })
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						region = "us-east-1"
					case "profile":
						form.AddInputField("AWS Profile", "default", 30, nil, func(t string) { profileName = t })
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						profileName = "default"
						region = "us-east-1"
					case "env":
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						region = "us-east-1"
					}
				case "anthropic":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), "", 50, '*', func(t string) { token = t })
				case "openrouter":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), "", 50, '*', func(t string) { token = t })
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
				if selectedProvider == "bedrock" && region != "" {
					fields = append(fields, views.InfoField{Label: "Region", Value: region})
				}
				return fields
			},
			Processing: i18n.T("cmd.init.wizard_processing_credentials"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 5bis — GitHub Copilot hint (conditional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "GitHub Copilot",
			SkipIf: func() bool {
				return selectedProvider != "github-copilot"
			},
			Required: true,
			CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
				accent := theme.ColorTag(theme.AccentHex)
				muted := theme.ColorTag(theme.TextMutedHex)
				reset := theme.TagColor

				tv := tview.NewTextView().
					SetDynamicColors(true).
					SetTextAlign(tview.AlignLeft)
				tv.SetBackgroundColor(theme.BgPanel)
				tv.SetBorderPadding(1, 0, 4, 4)
				tv.SetText(fmt.Sprintf(`  %sGitHub Copilot%s

  `+i18n.T("cmd.init.wizard_copilot_desc")+`

  %s$ gh auth login%s

  %s`+i18n.T("cmd.init.wizard_press_enter")+`%s
`,
					accent, reset,
					accent, reset,
					muted, reset,
				))
				tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					if event.Key() == tcell.KeyEnter {
						// Save provider config
						vip := viper.New()
						vip.SetConfigName("hub")
						vip.SetConfigType("toml")
						vip.AddConfigPath(config.HubDir())
						vip.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
						_ = vip.ReadInConfig()
						vip.Set("opencode.default_provider", "github-copilot")
						_ = vip.WriteConfigAs(config.ConfigPath())
						onDone()
						return nil
					}
					return event
				})
				container.AddItem(tv, 0, 1, true)
				tvApp.SetFocus(tv)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Provider", Value: "github-copilot"}}
			},
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 6 — First project (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: i18n.T("cmd.init.wizard_step_project"),
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
				return !projectCreated
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
		// STEP 8 — MCP Figma (optional)
		// ══════════════════════════════════════════════════════════════════════
		{
			Label: "MCP Figma",
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

	// Determine summary target: project view if a project was created, else home
	summaryTarget := "home"
	summaryLabel := i18n.T("wizard.summary.go_home")
	if projectCreated {
		summaryTarget = "project.mode"
		summaryLabel = i18n.T("wizard.summary.goto_projects")
	}

	return views.NewInlineWizardView(views.InlineWizardConfig{
		ID:                 "wizard.init",
		Title:              i18n.T("cmd.init.wizard_title"),
		Steps:              steps,
		SummaryTargetView:  summaryTarget,
		SummaryTargetLabel: summaryLabel,
		OnComplete: func(completed bool, err error) {
			if completed && err == nil {
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					_ = newApp
				}
			}
		},
	})
}
