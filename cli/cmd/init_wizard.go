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
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// needsFirstRunWizard returns true if no provider is configured.
func needsFirstRunWizard(a *app.App) bool {
	return a.Config.Opencode.DefaultProvider == ""
}

// runFirstRunWizard launches the init wizard before the TUI shell.
// It returns true if the wizard completed, false if aborted.
func runFirstRunWizard(a *app.App) bool {
	var selectedProvider string
	var authMode string
	var token string
	var region string
	var profile string

	providerOptions := []string{"bedrock", "anthropic", "openrouter"}

	steps := []views.WizardStep{
		// ── Step 1: Welcome ──
		{
			Label:    "Bienvenue",
			Required: true,
			CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
				accent := theme.ColorTag(theme.AccentHex)
				muted := theme.ColorTag(theme.TextMutedHex)
				reset := theme.TagColor

				tv := tview.NewTextView().
					SetDynamicColors(true).
					SetTextAlign(tview.AlignLeft)
				tv.SetBackgroundColor(theme.BgPanel)
				tv.SetBorderPadding(2, 0, 4, 4)
				tv.SetText(fmt.Sprintf(`%s██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s
%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s
%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s
%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s
%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s
%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s

  %sBienvenue dans OpenHub !%s

  `+i18n.T("cmd.init.wizard_intro")+`

  %s1.%s Choisir un fournisseur IA (provider)
  %s2.%s Configurer les credentials
  %s3.%s Ajouter un premier projet (optionnel)

  %sAppuyez sur Enter pour commencer.%s
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					accent, reset, accent, reset, accent, reset,
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
				app.SetFocus(tv)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Statut", Value: "Démarré"}}
			},
		},

		// ── Step 2: Provider selection ──
		{
			Label:    "Provider",
			Required: true,
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown("Provider IA", providerOptions, 0, func(option string, _ int) {
					selectedProvider = option
				})
				selectedProvider = providerOptions[0] // default
				form.AddButton("Suivant", onDone)
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Provider", Value: selectedProvider}}
			},
			Processing: "Configuration du provider...",
		},

		// ── Step 3: Bedrock auth mode (conditional) ──
		{
			Label: "Authentification",
			SkipIf: func() bool {
				return selectedProvider != "bedrock"
			},
			Required: true,
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				authModes := []string{"bearer", "profile", "env"}
				form := tview.NewForm()
				form.AddDropDown("Mode d'authentification", authModes, 0, func(option string, _ int) {
					authMode = option
				})
				authMode = authModes[0] // default
				form.AddButton("Suivant", onDone)
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth mode", Value: authMode}}
			},
		},

		// ── Step 4: Credentials ──
		{
			Label:    "Credentials",
			Required: true,
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()

				switch selectedProvider {
				case "bedrock":
					switch authMode {
					case "bearer":
						form.AddPasswordField("Bearer token", "", 50, '*', func(t string) { token = t })
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						region = "us-east-1"
					case "profile":
						form.AddInputField("AWS Profile", "default", 30, nil, func(t string) { profile = t })
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						profile = "default"
						region = "us-east-1"
					case "env":
						// No fields needed — env vars are already set
						form.AddInputField("AWS Region", "us-east-1", 30, nil, func(t string) { region = t })
						region = "us-east-1"
					}
				case "anthropic":
					form.AddPasswordField("Clé API Anthropic", "", 50, '*', func(t string) { token = t })
				case "openrouter":
					form.AddPasswordField("Clé API OpenRouter", "", 50, '*', func(t string) { token = t })
				}

				form.AddButton("Configurer", onDone)
				return form
			},
			OnDone: func() error {
				// Save provider configuration
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
					if authMode == "profile" && profile != "" {
						vip.Set("provider.bedrock.aws_profile", profile)
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
			Processing: "Enregistrement des credentials...",
		},

		// ── Step 5: Add first project (optional) ──
		{
			Label: "Premier projet",
			Skip:  false,
			Form: func(app *tview.Application, onDone func()) *tview.Form {
				var projectName, projectPath string

				form := tview.NewForm()
				form.AddInputField("Nom du projet", "", 40, nil, func(t string) { projectName = t })
				form.AddInputField("Chemin du projet", ".", 50, nil, func(t string) { projectPath = t })
				form.AddButton("Ajouter", func() {
					if projectName == "" || projectPath == "" {
						return
					}
					// Save project
					if a.Projects != nil {
						p := &domain.Project{
							ID:     uuid.New().String()[:8],
							Name:   projectName,
							Path:   projectPath,
							Status: domain.ProjectStatusActive,
						}
						_ = a.Projects.Create(context.Background(), p)
					}
					onDone()
				})
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Projet", Value: "Ajouté"}}
			},
			Processing: "Ajout du projet...",
		},
	}

	result := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: "OpenHub",
			Command:     "Configuration initiale",
			StatusHints: i18n.T("wizard.hints.default"),
			InfoPanel:   true,
		},
		Steps: steps,
	})

	return result.Completed
}

// buildFirstRunInlineWizard creates an InlineWizardView for the first-run setup.
// It has the same steps as runFirstRunWizard but runs inside the TUI shell.
func buildFirstRunInlineWizard(a *app.App) *views.InlineWizardView {
	var selectedProvider string
	var authMode string
	var token string
	var region string
	var profileName string

	providerOptions := []string{"bedrock", "anthropic", "openrouter"}

	steps := []views.WizardStep{
		// ── Step 1: Welcome ──
		{
			Label:    i18n.T("cmd.init.wizard_step_welcome"),
			Required: true,
			CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
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

  %sBienvenue dans OpenHub !%s

  `+i18n.T("cmd.init.wizard_intro")+`

  %s1.%s `+i18n.T("cmd.init.wizard_step1_desc")+`
  %s2.%s `+i18n.T("cmd.init.wizard_step2_desc")+`
  %s3.%s `+i18n.T("cmd.init.wizard_step3_desc")+`

  %s`+i18n.T("cmd.init.wizard_press_enter")+`%s
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					accent, reset, accent, reset, accent, reset,
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
				app.SetFocus(tv)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Statut", Value: i18n.T("cmd.init.wizard_started")}}
			},
		},

		// ── Step 2: Provider selection ──
		{
			Label:    "Provider",
			Required: true,
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown("Provider IA", providerOptions, 0, func(option string, _ int) {
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

		// ── Step 3: Bedrock auth mode (conditional) ──
		{
			Label: i18n.T("cmd.team.init.step_credentials"),
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
				return []views.InfoField{{Label: "Auth mode", Value: authMode}}
			},
		},

		// ── Step 4: Credentials ──
		{
			Label:    "Credentials",
			Required: true,
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

		// ── Step 5: First project (optional) ──
		{
			Label: i18n.T("cmd.init.wizard_step_project"),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				var projectName, projectPath string

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
					}
					onDone()
				})
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Projet", Value: i18n.T("cmd.init.wizard_project_added")}}
			},
			Processing: i18n.T("cmd.init.wizard_processing_project"),
		},
	}

	return views.NewInlineWizardView(views.InlineWizardConfig{
		ID:                 "wizard.init",
		Title:              i18n.T("cmd.init.wizard_title"),
		Steps:              steps,
		SummaryTargetView:  "settings",
		SummaryTargetLabel: i18n.T("wizard.summary.goto_settings"),
		OnComplete: func(completed bool, err error) {
			if completed && err == nil {
				// Reload the app to pick up the new configuration
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					_ = newApp
				}
			}
		},
	})
}
