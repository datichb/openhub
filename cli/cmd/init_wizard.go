package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"

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

	// Team wizard state — shared with buildInitWizardTeamSteps closures.
	teamState := &initWizardTeamState{}

	// appPtr lets team steps reload the app after hub.toml is updated.
	appPtr := &a

	providerOptions := []string{"bedrock", "anthropic", "openrouter", "github-copilot"}

	// providerStepIdx is resolved dynamically after the steps slice is built
	// (see below). It locates the Provider form step so the DropDown callback
	// can call step.Rerender to rebuild the form on provider change.
	var providerStepIdx int

	// langStepIdx and langIdx track the Language step position and current
	// selection. Used for live locale switching via Rerender.
	var langStepIdx int
	var langIdx int

	// Detect system locale EARLY so the Welcome page and all construction-time
	// labels render in the right language from the start.
	sysLang := os.Getenv("LANG")
	if sysLang == "" {
		sysLang = os.Getenv("LC_ALL")
	}
	if sysLang != "" && strings.Contains(strings.ToLower(sysLang), "fr") {
		selectedLang = "fr"
		langIdx = 0
	} else {
		selectedLang = "en"
		langIdx = 1
	}
	i18n.SetLocale(selectedLang)

	// focusBtn is set to true by the Language DropDown callback before triggering
	// a rerender — the engine reads it to focus the buttonForm instead of the form.
	focusBtn := false

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
%s3.%s `+i18n.T("cmd.init.wizard_step_team_desc_welcome")+`
%s4.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
%s5.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`

%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s

%s`+i18n.T("cmd.init.wizard_checklist_title")+`%s
%s☐%s `+i18n.T("cmd.init.wizard_checklist_provider")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_team")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_mcp")+`

%s`+i18n.T("cmd.init.wizard_checklist_note")+`%s
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
					accent, reset,
					muted, reset,
					secondary, reset,
					accent, reset,
					accent, reset,
					accent, reset,
					muted, reset,
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
		// STEP 1 — Language
		// ══════════════════════════════════════════════════════════════════════
	{
			Label:    i18n.T("cmd.init.wizard_step_lang"),
			Required: true,
			Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
				langOptions := []string{"Français", "English"}
				// selectedLang and langIdx are already initialized by the
				// early locale detection at the top of buildFirstRunInlineWizard.

				rerenderLang := func() {
					if fn := steps[langStepIdx].Rerender; fn != nil {
						go func() { tvApp.QueueUpdateDraw(func() { fn() }) }()
					}
				}

				form := tview.NewForm()
				langMounted := false
				form.AddDropDown(i18n.T("cmd.init.wizard_lang_select"), langOptions, langIdx, func(_ string, idx int) {
					if langIdx == idx {
						return // no change — avoid spurious rerender
					}
					langIdx = idx
					if idx == 1 {
						selectedLang = "en"
					} else {
						selectedLang = "fr"
					}
					i18n.SetLocale(selectedLang)
					if langMounted {
						focusBtn = true
						rerenderLang()
					}
				})
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				langMounted = true
				return form
			},
			OnDone: func() error {
				// Apply locale immediately so following steps use the right language
				i18n.SetLocale(selectedLang)

				// Persist to hub.toml
				return config.Update(func(c *config.Config) error {
					c.CLI.Language = selectedLang
					return nil
				})
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
			"cmd.init.wizard_intro_provider_title",
			"cmd.init.wizard_intro_provider_desc",
			"cmd.init.wizard_intro_provider_list",
			"cmd.init.wizard_provider_list_items",
			"cmd.init.wizard_provider_prereq",
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
			Label: i18n.T("cmd.init.wizard_step_provider_label"),
			SkipIf: func() bool {
				return providerSkipped
			},
			Validate: func() string {
				switch selectedProvider {
				case "anthropic", "openrouter":
					if token == "" {
						return i18n.T("cmd.init.wizard_token_required")
					}
				case "bedrock":
					if authMode == "bearer" && token == "" {
						return i18n.T("cmd.init.wizard_token_required")
					}
				}
				return ""
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
						form.AddPasswordField(i18n.T("cmd.init.wizard_bearer_token"), "", 50, '*', func(t string) { token = t })
						form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_bearer"), 60, 1, true, false)
						form.AddInputField(i18n.T("cmd.init.wizard_aws_region"), region, 30, nil, func(t string) { region = t })
					case "profile":
						form.AddInputField(i18n.T("cmd.init.wizard_aws_profile"), profileName, 30, nil, func(t string) { profileName = t })
						form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_profile"), 60, 1, true, false)
						form.AddInputField(i18n.T("cmd.init.wizard_aws_region"), region, 30, nil, func(t string) { region = t })
					case "env":
						form.AddInputField(i18n.T("cmd.init.wizard_aws_region"), region, 30, nil, func(t string) { region = t })
					}

				case "anthropic":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), "", 50, '*', func(t string) { token = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_anthropic"), 60, 1, true, false)

				case "openrouter":
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), "", 50, '*', func(t string) { token = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_openrouter"), 60, 1, true, false)

				case "github-copilot":
					form.AddTextView("", i18n.T("cmd.init.wizard_copilot_desc"), 60, 2, true, false)
					form.AddTextView("", "$ gh auth login", 60, 1, true, false)
				}

				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				// Store secrets in keychain (non-config data)
				switch selectedProvider {
				case "bedrock":
					if authMode == "bearer" && token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.Bedrock, "")
						if keychainKey != "" {
							if err := a.Secrets.Set(context.Background(), keychainKey, token); err != nil {
								return fmt.Errorf("keychain: %w", err)
							}
						}
					}
				case "anthropic":
					if token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.Anthropic, "")
						if keychainKey != "" {
							if err := a.Secrets.Set(context.Background(), keychainKey, token); err != nil {
								return fmt.Errorf("keychain: %w", err)
							}
						}
					}
				case "openrouter":
					if token != "" && a.Secrets != nil {
						keychainKey := provider.KeychainKey(provider.OpenRouter, "")
						if keychainKey != "" {
							if err := a.Secrets.Set(context.Background(), keychainKey, token); err != nil {
								return fmt.Errorf("keychain: %w", err)
							}
						}
					}
				case "github-copilot":
					// No token needed — uses gh auth
				}

				return config.Update(func(c *config.Config) error {
					c.Opencode.DefaultProvider = selectedProvider
					if selectedProvider == "bedrock" {
						c.Provider.Bedrock.AuthMode = authMode
						if region != "" {
							c.Provider.Bedrock.AWSRegion = region
						}
						if authMode == "profile" && profileName != "" {
							c.Provider.Bedrock.AWSProfile = profileName
						}
					}
					return nil
				})
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
		// STEP 4 — Intro Équipe (group: Équipe)
		// ══════════════════════════════════════════════════════════════════════
		buildIntroStep(
			i18n.T("cmd.init.wizard_step_team"),
			"cmd.init.wizard_intro_team_title",
			"cmd.init.wizard_intro_team_desc",
			"", "",
			"cmd.init.wizard_team_prereq",
			"cmd.init.wizard_intro_team_optional",
			func() { teamState.Skipped = false },
			func() { teamState.Skipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEPS 5-6 — Team form + processing (appended below)
		// ══════════════════════════════════════════════════════════════════════
	}

	// Inject the team wizard steps (form + processing).
	steps = append(steps, buildInitWizardTeamSteps(appPtr, teamState)...)

	// Continue with remaining steps: Projet, Deploy, MCP.
	steps = append(steps,

		// ══════════════════════════════════════════════════════════════════════
		// STEP 7 — Intro Projet (group: Projet)
		// ══════════════════════════════════════════════════════════════════════
		buildIntroStep(
			"Projet",
			"cmd.init.wizard_intro_project_title",
			"cmd.init.wizard_intro_project_desc",
			"", "",
			"",
			"cmd.init.wizard_intro_project_optional",
			func() { projectSkipped = false },
			func() { projectSkipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — First project (optional)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			Label: i18n.T("cmd.init.wizard_step_project"),
			SkipIf: func() bool {
				return projectSkipped
			},
			Validate: func() string {
				if projectName == "" {
					return i18n.T("cmd.init.wizard_project_name_required")
				}
				p := expandPath(projectPath)
				abs, err := filepath.Abs(p)
				if err != nil {
					return i18n.Tf("cmd.init.wizard_project_path_invalid", projectPath)
				}
				info, err := os.Stat(abs)
				if err != nil {
					return i18n.Tf("cmd.init.wizard_project_path_invalid", projectPath)
				}
				if !info.IsDir() {
					return i18n.Tf("cmd.init.wizard_project_path_invalid", projectPath)
				}
				// Normalize for persistence
				projectPath = abs
				return ""
			},
			Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.init.wizard_project_name"), "", 40, nil, func(t string) { projectName = t })
				form.AddInputField(i18n.T("cmd.init.wizard_project_path"), ".", 50, nil, func(t string) { projectPath = t })

				// If a team was configured, offer to attach the project
				attachMounted := false
				if teamState.Configured && teamState.TeamID != "" {
					attachOptions := []string{
						i18n.Tf("cmd.init.wizard_project_attach_yes", teamState.TeamID),
						i18n.T("cmd.init.wizard_project_attach_no"),
					}
					// Default to attaching
					teamState.attachProject = true
					form.AddDropDown(i18n.T("cmd.init.wizard_project_attach_team"), attachOptions, 0, func(_ string, idx int) {
						teamState.attachProject = idx == 0
						if attachMounted {
							views.AutoAdvanceFromDropDown(tvApp, form, 2)
						}
					})
				}

				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					if (*appPtr).Projects != nil {
						p := &domain.Project{
							ID:     uuid.New().String()[:8],
							Name:   projectName,
							Path:   projectPath,
							Status: domain.ProjectStatusActive,
						}
						// Attach to team if user chose to
						if teamState.Configured && teamState.attachProject && teamState.TeamID != "" {
							tid := teamState.TeamID
							p.TeamID = &tid
						}
						if err := (*appPtr).Projects.Create(context.Background(), p); err == nil {
							projectCreated = true
						}
					}
					onDone()
				})
				attachMounted = true
				return form
			},
			InfoFields: func() []views.InfoField {
				fields := []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_project"), Value: i18n.T("cmd.init.wizard_project_added")}}
				if teamState.Configured && teamState.attachProject && teamState.TeamID != "" {
					fields = append(fields, views.InfoField{
						Label: i18n.T("cmd.init.wizard_step_team"),
						Value: i18n.Tf("cmd.init.wizard_project_attached", teamState.TeamID),
					})
				}
				return fields
			},
			Processing: i18n.T("cmd.init.wizard_processing_project"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 9 — Deploy agents/skills (conditional on project)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
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
				config.Reset() // clear cached singleton so ReloadApp re-reads hub.toml
				newApp, err := ReloadApp()
				if err != nil {
					return fmt.Errorf("reload: %w", err)
				}
				*appPtr = newApp

				hubDir := findHubDir()
				if hubDir == "" {
					return nil // hub content not extracted yet, skip silently
				}

				// Build a minimal deploy plan for the new project
				plan := buildDeployPlan(*appPtr, projectPath, "", hubDir, selectedProvider, "", nil, nil, nil, nil)
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
			"cmd.init.wizard_intro_mcp_title",
			"cmd.init.wizard_intro_mcp_desc",
			"cmd.init.wizard_intro_mcp_list",
			"cmd.init.wizard_mcp_list_items",
			"cmd.init.wizard_mcp_prereq",
			"",
			func() { mcpSkipped = false },
			func() { mcpSkipped = true },
		),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — MCP Figma (optional)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			Label: i18n.T("cmd.init.wizard_step_mcp_figma"),
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
				form.AddTextView("", i18n.T("cmd.init.mcp_hint_figma"), 60, 2, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if figmaToken == "" {
					return nil
				}
				if a.Secrets != nil {
					if err := a.Secrets.Set(context.Background(), config.DefaultFigmaTokenKey, figmaToken); err != nil {
						return fmt.Errorf("keychain: %w", err)
					}
				}
				// Enable in config
				return config.Update(func(c *config.Config) error {
					c.MCP.Figma.Enabled = true
					return nil
				})
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
		views.WizardStep{
			Label: i18n.T("cmd.init.wizard_step_mcp_gitlab"),
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
				form.AddTextView("", i18n.T("cmd.init.mcp_hint_gitlab"), 60, 2, true, false)
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
					if err := a.Secrets.Set(context.Background(), config.DefaultGitLabTokenKey, gitlabToken); err != nil {
						return fmt.Errorf("keychain: %w", err)
					}
				}
				return config.Update(func(c *config.Config) error {
					c.MCP.Gitlab.Enabled = true
					if gitlabWrite {
						c.MCP.Gitlab.WriteEnabled = true
					}
					return nil
				})
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
		views.WizardStep{
			Label: i18n.T("cmd.init.wizard_step_mcp_gslides"),
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
				form.AddTextView("", i18n.T("cmd.init.mcp_hint_gslides"), 60, 2, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				return form
			},
			OnDone: func() error {
				if gslidesToken == "" {
					return nil
				}
				if a.Secrets != nil {
					if err := a.Secrets.Set(context.Background(), config.DefaultGslidesTokenKey, gslidesToken); err != nil {
						return fmt.Errorf("keychain: %w", err)
					}
				}
				return config.Update(func(c *config.Config) error {
					c.MCP.Gslides.Enabled = true
					return nil
				})
			},
			InfoFields: func() []views.InfoField {
				v := i18n.T("cmd.init.wizard_mcp_configured")
				if gslidesToken == "" {
					v = i18n.T("cmd.init.wizard_mcp_skipped")
				}
				return []views.InfoField{{Label: "Google Slides", Value: v}}
			},
		},
	)

	// Resolve langStepIdx: Language step is always step 1.
	langStepIdx = 1

	// Resolve providerStepIdx dynamically: find the Provider form step.
	// It's the first step that has both Form and SkipIf (the provider
	// credential form that adapts dynamically).
	for i, s := range steps {
		if s.Form != nil && s.SkipIf != nil && s.Validate != nil {
			providerStepIdx = i
			break
		}
	}

	cfg := views.InlineWizardConfig{
		ID:    "wizard.init",
		Title: i18n.T("cmd.init.wizard_title"),
		Steps: steps,
		Groups: []views.StepGroup{
			{Label: i18n.T("cmd.init.wizard_group_lang"), StartIdx: 0},
			{Label: i18n.T("cmd.init.wizard_group_provider"), StartIdx: 2},
			{Label: i18n.T("cmd.init.wizard_group_team"), StartIdx: 4},
			{Label: i18n.T("cmd.init.wizard_group_project"), StartIdx: 7},
			{Label: i18n.T("cmd.init.wizard_group_mcp"), StartIdx: 10},
		},
		FocusButtonAfterRender: &focusBtn,
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
				if err := config.Update(func(c *config.Config) error {
					c.CLI.SetupDone = true
					return nil
				}); err != nil {
					slog.Warn("failed to persist setup_done flag", "error", err)
				}

				config.Reset() // clear cached singleton so ReloadApp re-reads hub.toml
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					_ = newApp
				}
			}
		},
	}

	// RefreshLabels re-evaluates all construction-time i18n labels.
	// Assigned after cfg is created because the closure references cfg.
	cfg.RefreshLabels = func() {
		cfg.Title = i18n.T("cmd.init.wizard_title")

		// Group labels
		cfg.Groups[0].Label = i18n.T("cmd.init.wizard_group_lang")
		cfg.Groups[1].Label = i18n.T("cmd.init.wizard_group_provider")
		cfg.Groups[2].Label = i18n.T("cmd.init.wizard_group_team")
		cfg.Groups[3].Label = i18n.T("cmd.init.wizard_group_project")
		cfg.Groups[4].Label = i18n.T("cmd.init.wizard_group_mcp")

		// Step labels
		steps[0].Label = i18n.T("cmd.init.wizard_step_welcome")
		steps[1].Label = i18n.T("cmd.init.wizard_step_lang")
		steps[3].Label = i18n.T("cmd.init.wizard_step_provider_label")
		steps[3].Processing = i18n.T("cmd.init.wizard_processing_credentials")
		steps[4].Label = i18n.T("cmd.init.wizard_step_team")
		steps[5].Label = i18n.T("cmd.init.wizard_step_team_form")
		steps[5].Processing = i18n.T("cmd.init.wizard_processing_team")
		steps[6].Label = i18n.T("cmd.init.wizard_step_team_sync")
		steps[8].Label = i18n.T("cmd.init.wizard_step_project")
		steps[8].Processing = i18n.T("cmd.init.wizard_processing_project")
		steps[9].Label = i18n.T("cmd.init.wizard_step_deploy")
		steps[9].Processing = i18n.T("cmd.init.wizard_deploy_processing")
		steps[11].Label = i18n.T("cmd.init.wizard_step_mcp_figma")
		steps[12].Label = i18n.T("cmd.init.wizard_step_mcp_gitlab")
		steps[13].Label = i18n.T("cmd.init.wizard_step_mcp_gslides")
	}

	return views.NewInlineWizardView(cfg)
}

// buildIntroStep creates a CustomView step that serves as a group introduction.
// Parameters are i18n KEYS (not resolved values) so the content is freshly
// translated each time the step is rendered — essential for live locale switching.
// When onSkip is non-nil, a "Skip" button is added alongside "Continue".
// When onContinue is non-nil, it is called when "Continue" is clicked
// (use to reset a skip flag when the user goes back and re-enters a group).
func buildIntroStep(badge, titleKey, descKey, listTitleKey, listItemsKey, prereqsKey, noteKey string, onContinue, onSkip func()) views.WizardStep {
	return views.WizardStep{
		Label:         badge,
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			// Resolve i18n keys at render time (not construction time)
			title := i18n.T(titleKey)
			desc := i18n.T(descKey)
			listTitle := ""
			listItems := ""
			if listTitleKey != "" {
				listTitle = i18n.T(listTitleKey)
			}
			if listItemsKey != "" {
				listItems = i18n.T(listItemsKey)
			}
			prereqs := ""
			if prereqsKey != "" {
				prereqs = i18n.T(prereqsKey)
			}
			note := ""
			if noteKey != "" {
				note = i18n.T(noteKey)
			}

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

			// Optional prerequisites (with visual separator)
			if prereqs != "" {
				fmt.Fprintf(&b, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s\n\n", muted, reset)
				for _, line := range strings.Split(prereqs, "\n") {
					if strings.HasPrefix(line, "• ") {
						// Bullet items: bullet in accent, rest in muted
						fmt.Fprintf(&b, "%s•%s %s%s%s\n", accent, reset, muted, line[len("• "):], reset)
					} else {
						// Section title
						fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
					}
				}
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
				skipConfirmed := false
				buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
					if !skipConfirmed {
						// First click: ask for confirmation
						skipConfirmed = true
						if btn := buttonForm.GetButton(1); btn != nil {
							btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
						}
						return
					}
					// Second click: confirmed, skip the section
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
