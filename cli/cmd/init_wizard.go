package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
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
		regionIdx        int  // current index in region DropDown (bedrock); 0 = placeholder
		customRegion     bool // true when user selects "Custom..." in the region dropdown
		hasKeychainToken bool // true when a token for the current provider exists in the keychain
		providerSkipped  bool // true if user clicked "Ignorer" on the Provider intro
		projectSkipped   bool // true if user clicked "Ignorer" on the Project intro
		mcpSkipped       bool // true if user clicked "Ignorer" on the MCP intro
	)

	// Team wizard state — shared with buildInitWizardTeamSteps closures.
	teamState := &initWizardTeamState{Ctx: context.Background()}

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
			ID:            "welcome",
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

				// Arrow keys navigate between buttons (consistency with other intro steps)
				buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					switch event.Key() {
					case tcell.KeyLeft:
						return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
					case tcell.KeyRight:
						return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
					}
					return event
				})

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
			ID:       "lang",
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
			ID:    "provider",
			Label: i18n.T("cmd.init.wizard_step_provider_label"),
			SkipIf: func() bool {
				return providerSkipped
			},
			Validate: func() string {
				switch selectedProvider {
				case "anthropic", "openrouter":
					if token == "" && !hasKeychainToken {
						return i18n.T("cmd.init.wizard_token_required")
					}
				case "bedrock":
					if authMode == "bearer" && token == "" && !hasKeychainToken {
						return i18n.T("cmd.init.wizard_token_required")
					}
					if region == "" {
						return i18n.T("cmd.init.wizard_region_required")
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
			if profileName == "" {
				profileName = "default"
			}

			// ── Credential detection: check keychain for existing token ──
			hasKeychainToken = false
			if token == "" {
				name := providerPkg.Name(selectedProvider)
				if a.Secrets != nil {
					if key := providerPkg.KeychainKey(name, ""); key != "" {
						if existing, err := a.Secrets.Get(context.Background(), key); err == nil && existing != "" {
							hasKeychainToken = true
							// Do NOT pre-fill token — show hint instead (Option A)
						}
					}
				}
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
						region = ""
						regionIdx = 0
						customRegion = false
						hasKeychainToken = false
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
							// Reset token/profile/region on auth change
							token = ""
							profileName = "default"
							region = ""
							regionIdx = 0
							customRegion = false
							hasKeychainToken = false
							rerenderSafe()
						},
					)
					switch authMode {
					case "bearer":
						if hasKeychainToken {
							form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
						}
						form.AddPasswordField(i18n.T("cmd.init.wizard_bearer_token"), token, 0, '*', func(t string) { token = t })
						form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_bearer"), 60, 1, true, false)
					case "profile":
						form.AddInputField(i18n.T("cmd.init.wizard_aws_profile"), profileName, 0, nil, func(t string) { profileName = t })
						form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_profile"), 60, 1, true, false)
					case "env":
						// No extra fields before the region dropdown.
					}

					// ── Region dropdown (shared by all bedrock auth modes) ──
					regionLabels := append(
						[]string{i18n.T("cmd.init.wizard_region_placeholder")},
						providerPkg.BedrockRegionLabels()...,
					)
					regionLabels = append(regionLabels, i18n.T("cmd.init.wizard_custom_region"))
					customIdx := len(regionLabels) - 1
					form.AddDropDown(
						i18n.T("cmd.init.wizard_aws_region"),
						regionLabels, regionIdx,
						func(_ string, idx int) {
							if regionIdx == idx {
								return // no change — avoid spurious rerender
							}
							wasCustom := customRegion
							if idx == 0 {
								// Placeholder selected — clear region.
								customRegion = false
								region = ""
							} else if idx == customIdx {
								customRegion = true
								region = ""
							} else if idx > 0 {
								customRegion = false
								region = providerPkg.BedrockRegions[idx-1].Code // -1 for placeholder offset
							}
							regionIdx = idx
							// Only rerender when the custom input field needs to appear/disappear.
							if customRegion != wasCustom {
								rerenderSafe()
							}
						},
					)
					if customRegion {
						form.AddInputField(i18n.T("cmd.init.wizard_custom_region_input"), region, 0, nil, func(t string) { region = t })
					}

				case "anthropic":
					if hasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
					}
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), token, 0, '*', func(t string) { token = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_anthropic"), 60, 1, true, false)

				case "openrouter":
					if hasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
					}
					form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), token, 0, '*', func(t string) { token = t })
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
						keychainKey := providerPkg.KeychainKey(providerPkg.Bedrock, "")
						if keychainKey != "" {
							if err := a.Secrets.Set(context.Background(), keychainKey, token); err != nil {
								return fmt.Errorf("keychain: %w", err)
							}
						}
					}
				case "anthropic":
					if token != "" && a.Secrets != nil {
						keychainKey := providerPkg.KeychainKey(providerPkg.Anthropic, "")
						if keychainKey != "" {
							if err := a.Secrets.Set(context.Background(), keychainKey, token); err != nil {
								return fmt.Errorf("keychain: %w", err)
							}
						}
					}
				case "openrouter":
					if token != "" && a.Secrets != nil {
						keychainKey := providerPkg.KeychainKey(providerPkg.OpenRouter, "")
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
				if a.Secrets == nil {
					fields = append(fields, views.InfoField{
						Label: "Warning",
						Value: i18n.T("cmd.init.wizard_no_keyring"),
					})
				}
				// Check opencode binary availability
				if _, err := opencode.FindBinary(); err != nil {
					fields = append(fields, views.InfoField{
						Label: "Warning",
						Value: i18n.T("cmd.init.wizard_opencode_not_found"),
					})
				}
				return fields
			},
			Processing: i18n.T("cmd.init.wizard_processing_credentials"),
		},

		// ══════════════════════════════════════════════════════════════════════
		// STEP 4 — Team mode choice (group: Équipe)
		// Tri-choice: Create / Rejoin / Skip
		// ══════════════════════════════════════════════════════════════════════
		buildTeamModeIntroStep(teamState),

		// ══════════════════════════════════════════════════════════════════════
		// STEPS 5-6 — Team init form + processing (appended below)
		// STEPS 7-10 — Team rejoin steps (appended below)
		// ══════════════════════════════════════════════════════════════════════
	}

	// Inject the team wizard steps (form + processing).
	steps = append(steps, buildInitWizardTeamSteps(appPtr, teamState)...)

	// Inject the team rejoin steps (repo + HTTPS creds + member selection + validate).
	steps = append(steps, buildInitWizardRejoinSteps(appPtr, teamState)...)

	// Record the start index for the project group (after all team steps).
	projectGroupStart := len(steps)

	// Continue with remaining steps: Projet, Deploy.
	steps = append(steps,

		// ══════════════════════════════════════════════════════════════════════
		// Intro Projet (group: Projet)
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
		// STEP 5 — First project (optional)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			ID:    "project",
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
				// Use outer vars as initial values for retry-resilience.
				initialName := projectName
				initialPath := projectPath
				if initialPath == "" {
					initialPath = "."
				}
				form.AddInputField(i18n.T("cmd.init.wizard_project_name"), initialName, 0, nil, func(t string) { projectName = t })
				form.AddInputField(i18n.T("cmd.init.wizard_project_path"), initialPath, 0, nil, func(t string) { projectPath = t })

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

				form.AddButton(i18n.T("wizard.hint.submit"), onDone)
				attachMounted = true
				return form
			},
			OnDone: func() error {
				if (*appPtr).Projects == nil {
					return fmt.Errorf("project store not initialized")
				}
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
				if err := (*appPtr).Projects.Create(context.Background(), p); err != nil {
					return fmt.Errorf("create project: %w", err)
				}
				projectCreated = true
				return nil
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
		// STEP 6 — Deploy agents/skills (conditional on project)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			ID:    "deploy",
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
	)

	// Record the start index for the MCP group (after project steps).
	mcpGroupStart := len(steps)

	// Continue with MCP steps.
	steps = append(steps,

		// ══════════════════════════════════════════════════════════════════════
		// Intro MCP (group: Intégrations)
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
		// STEP 7 — MCP Figma (optional)
		// ══════════════════════════════════════════════════════════════════════
		buildMCPTokenStep(mcpTokenStepOpts{
			ID:           "mcp_figma",
			LabelI18nKey: "cmd.init.wizard_step_mcp_figma",
			DisplayName:  "Figma",
			TokenKey:     config.DefaultFigmaTokenKey,
			HintI18nKey:  "cmd.init.mcp_hint_figma",
			TokenVar:     &figmaToken,
			SkipIf:       func() bool { return mcpSkipped },
			Secrets:      a.Secrets,
			AfterStore: func() error {
				return config.Update(func(c *config.Config) error {
					c.MCP.Figma.Enabled = true
					return nil
				})
			},
		}),

		// ══════════════════════════════════════════════════════════════════════
		// STEP 8 — MCP GitLab (optional)
		// ══════════════════════════════════════════════════════════════════════
		views.WizardStep{
			ID:    "mcp_gitlab",
			Label: i18n.T("cmd.init.wizard_step_mcp_gitlab"),
			SkipIf: func() bool {
				return mcpSkipped
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				// Check keychain for existing token (don't pre-fill)
				hasKeychainGitlab := false
				if gitlabToken == "" && a.Secrets != nil {
					if existing, err := a.Secrets.Get(context.Background(), config.DefaultGitLabTokenKey); err == nil && existing != "" {
						hasKeychainGitlab = true
					}
				}
				form := tview.NewForm()
				if hasKeychainGitlab {
					form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
				}
				form.AddPasswordField(
					i18n.Tf("cmd.init.mcp_token_prompt", "GitLab"),
					gitlabToken, 0, '*',
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
		// STEP 9 — MCP Google Slides (optional)
		// ══════════════════════════════════════════════════════════════════════
		buildMCPTokenStep(mcpTokenStepOpts{
			ID:           "mcp_gslides",
			LabelI18nKey: "cmd.init.wizard_step_mcp_gslides",
			DisplayName:  "Google Slides",
			TokenKey:     config.DefaultGslidesTokenKey,
			HintI18nKey:  "cmd.init.mcp_hint_gslides",
			TokenVar:     &gslidesToken,
			SkipIf:       func() bool { return mcpSkipped },
			Secrets:      a.Secrets,
			AfterStore: func() error {
				return config.Update(func(c *config.Config) error {
					c.MCP.Gslides.Enabled = true
					return nil
				})
			},
		}),
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
			{Label: i18n.T("cmd.init.wizard_group_project"), StartIdx: projectGroupStart},
			{Label: i18n.T("cmd.init.wizard_group_mcp"), StartIdx: mcpGroupStart},
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
				// ── Extract hub content (agents, skills, permissions) ────
				// Without this, oh start would lack agent/skill definitions.
				hubContentDir := hubcontent.HubContentDir()
				if extractErr := hubcontent.Extract(hubContentDir); extractErr != nil {
					slog.Error("failed to extract hub content", "error", extractErr)
				}

				// ── Detect branch pattern heuristic ─────────────────────
				// Best-effort: only applies if CWD is a git repo.
				if pattern := detectBranchPatternHeuristic("."); pattern != "" {
					_ = config.Update(func(c *config.Config) error {
						if c.Worktree.BranchPattern == "" {
							c.Worktree.BranchPattern = pattern
						}
						return nil
					})
				}

				// Mark setup as done so the wizard doesn't re-launch.
				// Retry once on failure to avoid a wizard-loop on next startup.
				for attempt := 0; attempt < 2; attempt++ {
					if err := config.Update(func(c *config.Config) error {
						c.CLI.SetupDone = true
						return nil
					}); err == nil {
						break
					} else if attempt == 1 {
						slog.Error("failed to persist setup_done flag after retry — wizard may re-launch", "error", err)
					} else {
						slog.Warn("retrying setup_done persistence", "error", err)
						config.Reset()
					}
				}

				config.Reset() // clear cached singleton so ReloadApp re-reads hub.toml
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					_ = newApp
				}

				// Launch the Tracker Discovery Wizard if the user chose
				// "Configure now" during the team setup step.
				if teamState.LaunchTrackerDiscovery {
					go func() {
						time.Sleep(200 * time.Millisecond)
						if tuiShell != nil {
							tuiShell.App().QueueUpdateDraw(func() {
								actionTrackerDiscovery()
							})
						}
					}()
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

		// Step labels — resolved by ID (no fragile positional indices)
		refresh := func(id, labelKey string) {
			if s := views.StepByID(steps, id); s != nil {
				s.Label = i18n.T(labelKey)
			}
		}
		refreshP := func(id, labelKey, procKey string) {
			if s := views.StepByID(steps, id); s != nil {
				s.Label = i18n.T(labelKey)
				s.Processing = i18n.T(procKey)
			}
		}

		refresh("welcome", "cmd.init.wizard_step_welcome")
		refresh("lang", "cmd.init.wizard_step_lang")
		refreshP("provider", "cmd.init.wizard_step_provider_label", "cmd.init.wizard_processing_credentials")
		refresh("team_mode", "cmd.init.wizard_step_team")

		// Team init steps
		refresh("team_form", "cmd.init.wizard_step_team_form")
		refresh("team_sync", "cmd.init.wizard_step_team_sync")

		// Team rejoin steps
		refreshP("rejoin_repo", "cmd.init.wizard_step_rejoin_repo", "cmd.init.wizard_processing_rejoin_clone")
		refresh("team_cred_mode", "cmd.init.wizard_step_team_cred_mode")
		refresh("team_creds", "cmd.init.wizard_step_team_creds")
		refresh("rejoin_member", "cmd.init.wizard_step_rejoin_member")
		refreshP("rejoin_validate", "cmd.init.wizard_step_rejoin_validate", "cmd.init.wizard_processing_rejoin_validate")

		// Project steps
		refreshP("project", "cmd.init.wizard_step_project", "cmd.init.wizard_processing_project")
		refreshP("deploy", "cmd.init.wizard_step_deploy", "cmd.init.wizard_deploy_processing")

		// MCP steps
		refresh("mcp_figma", "cmd.init.wizard_step_mcp_figma")
		refresh("mcp_gitlab", "cmd.init.wizard_step_mcp_gitlab")
		refresh("mcp_gslides", "cmd.init.wizard_step_mcp_gslides")
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

			// Adaptive layout: reduce chrome when terminal is small.
			_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
			if termH <= 0 {
				termH = 50
			}
			availH := termH - 7

			if availH >= 35 {
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)
				badgeView := views.BuildStepBadge(badge, false)
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
			} else if availH >= 20 {
				badgeView := views.BuildStepBadge(badge, true)
				topSpacer := tview.NewBox()
				topSpacer.SetBackgroundColor(theme.BgPanel)

				container.AddItem(topSpacer, 1, 0, false)
				container.AddItem(badgeView, 3, 0, false)
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			} else {
				container.AddItem(tv, 0, 1, false)
				container.AddItem(buttonForm, 3, 0, true)
			}
			tvApp.SetFocus(buttonForm)
		},
	}
}
