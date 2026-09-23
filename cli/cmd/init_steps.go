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
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// initWizardState — shared mutable state for all init wizard step builders
// ─────────────────────────────────────────────────────────────────────────────
//
// This struct replaces the 22+ closure-captured variables that previously lived
// inside buildFirstRunInlineWizard. All step builders receive a pointer to this
// struct and read/write fields through it. The semantics are identical to the
// closure pattern (pointer capture), but the state is explicit and testable.

type initStepState struct {
	// ── Language ──
	SelectedLang string
	LangIdx      int

	// ── Provider ──
	SelectedProvider string
	ProviderIdx      int
	AuthMode         string
	AuthIdx          int
	Token            string
	Region           string
	ProfileName      string
	RegionIdx        int
	CustomRegion     bool
	HasKeychainToken bool
	ProviderSkipped  bool

	// ── Project ──
	ProjectName     string
	ProjectPath     string
	ProjectCreated  bool
	ProjectSkipped  bool
	DeployConfirmed bool

	// ── MCP ──
	FigmaToken   string
	GitlabToken  string
	GitlabWrite  bool
	GslidesToken string
	MCPSkipped   bool

	// ── Shared refs ──
	TeamState *initWizardTeamState
	AppPtr    **app.App    // double pointer: team/deploy steps reload the app
	FocusBtn  *bool        // pointer shared with InlineWizardConfig.FocusButtonAfterRender
	Steps     *[]views.WizardStep // pointer to the assembled steps slice (for Rerender lookup)

	// ── Step index caches (resolved after assembly) ──
	LangStepIdx     int
	ProviderStepIdx int

	// ── Provider options (immutable after init) ──
	ProviderOptions []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Step builders
// ─────────────────────────────────────────────────────────────────────────────

// buildWelcomeStep creates the welcome/splash screen step.
func buildWelcomeStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
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

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_welcome_start")+"  ", onDone)

			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})

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
	}
}

// buildLangStep creates the language selection step with live locale switching.
func buildLangStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:       "lang",
		Label:    i18n.T("cmd.init.wizard_step_lang"),
		Required: true,
		Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
			langOptions := []string{"Français", "English"}

			rerenderLang := func() {
				if fn := (*s.Steps)[s.LangStepIdx].Rerender; fn != nil {
					go func() { tvApp.QueueUpdateDraw(func() { fn() }) }()
				}
			}

			form := tview.NewForm()
			langMounted := false
			form.AddDropDown(i18n.T("cmd.init.wizard_lang_select"), langOptions, s.LangIdx, func(_ string, idx int) {
				if s.LangIdx == idx {
					return
				}
				s.LangIdx = idx
				if idx == 1 {
					s.SelectedLang = "en"
				} else {
					s.SelectedLang = "fr"
				}
				i18n.SetLocale(s.SelectedLang)

				if langMounted {
					*s.FocusBtn = true
					rerenderLang()
				}
			})
			form.AddButton(i18n.T("wizard.hint.submit"), onDone)
			langMounted = true
			return form
		},
		OnDone: func() error {
			i18n.SetLocale(s.SelectedLang)
			return config.Update(func(c *config.Config) error {
				c.CLI.Language = s.SelectedLang
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			label := "Français"
			if s.SelectedLang == "en" {
				label = "English"
			}
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_lang"), Value: label}}
		},
	}
}

// buildProviderStep creates the single dynamic provider + credentials step.
// The form adapts its fields based on the selected provider (and auth mode
// for bedrock). DropDown changes trigger a full step re-render via Rerender.
func buildProviderStep(s *initStepState) views.WizardStep {
	a := *s.AppPtr
	return views.WizardStep{
		ID:    "provider",
		Label: i18n.T("cmd.init.wizard_step_provider_label"),
		SkipIf: func() bool {
			return s.ProviderSkipped
		},
		Validate: func() string {
			switch s.SelectedProvider {
			case "anthropic", "openrouter":
				if s.Token == "" && !s.HasKeychainToken {
					return i18n.T("cmd.init.wizard_token_required")
				}
			case "bedrock":
				if s.AuthMode == "bearer" && s.Token == "" && !s.HasKeychainToken {
					return i18n.T("cmd.init.wizard_token_required")
				}
				if s.Region == "" {
					return i18n.T("cmd.init.wizard_region_required")
				}
			}
			return ""
		},
		Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
			if s.SelectedProvider == "" {
				s.SelectedProvider = s.ProviderOptions[0]
			}
			if s.AuthMode == "" {
				s.AuthMode = "bearer"
			}
			if s.ProfileName == "" {
				s.ProfileName = "default"
			}

			// Credential detection: check keychain for existing token
			s.HasKeychainToken = false
			if s.Token == "" {
				name := providerPkg.Name(s.SelectedProvider)
				if a.Secrets != nil {
					if key := providerPkg.KeychainKey(name, ""); key != "" {
						if existing, err := a.Secrets.Get(context.Background(), key); err == nil && existing != "" {
							s.HasKeychainToken = true
						}
					}
				}
			}

			rerenderSafe := func() {
				if fn := (*s.Steps)[s.ProviderStepIdx].Rerender; fn != nil {
					go func() { tvApp.QueueUpdateDraw(func() { fn() }) }()
				}
			}

			form := tview.NewForm()
			authModes := []string{"bearer", "profile", "env"}

			// Provider dropdown
			form.AddDropDown(
				i18n.T("cmd.init.wizard_provider_select"),
				s.ProviderOptions, s.ProviderIdx,
				func(_ string, idx int) {
					if s.ProviderIdx == idx {
						return
					}
					s.ProviderIdx = idx
					s.SelectedProvider = s.ProviderOptions[idx]
					s.Token = ""
					s.Region = ""
					s.RegionIdx = 0
					s.CustomRegion = false
					s.HasKeychainToken = false
					s.ProfileName = "default"
					s.AuthMode = "bearer"
					s.AuthIdx = 0
					rerenderSafe()
				},
			)

			// Conditional fields based on current provider
			switch s.SelectedProvider {
			case "bedrock":
				form.AddDropDown(
					i18n.T("cmd.init.wizard_auth_mode"),
					authModes, s.AuthIdx,
					func(_ string, idx int) {
						if s.AuthIdx == idx {
							return
						}
						s.AuthIdx = idx
						s.AuthMode = authModes[idx]
						s.Token = ""
						s.ProfileName = "default"
						s.Region = ""
						s.RegionIdx = 0
						s.CustomRegion = false
						s.HasKeychainToken = false
						rerenderSafe()
					},
				)
				switch s.AuthMode {
				case "bearer":
					if s.HasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
					}
					form.AddPasswordField(i18n.T("cmd.init.wizard_bearer_token"), s.Token, 0, '*', func(t string) { s.Token = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_bearer"), 60, 1, true, false)
				case "profile":
					form.AddInputField(i18n.T("cmd.init.wizard_aws_profile"), s.ProfileName, 0, nil, func(t string) { s.ProfileName = t })
					form.AddTextView("", i18n.T("cmd.init.provider_hint_bedrock_profile"), 60, 1, true, false)
				case "env":
					// No extra fields before the region dropdown.
				}

				// Region dropdown (shared by all bedrock auth modes)
				addBedrockRegionDropDown(form, &s.Region, &s.RegionIdx, &s.CustomRegion, rerenderSafe)

			case "anthropic":
				if s.HasKeychainToken {
					form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
				}
				form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_anthropic"), s.Token, 0, '*', func(t string) { s.Token = t })
				form.AddTextView("", i18n.T("cmd.init.provider_hint_anthropic"), 60, 1, true, false)

			case "openrouter":
				if s.HasKeychainToken {
					form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
				}
				form.AddPasswordField(i18n.T("cmd.init.wizard_api_key_openrouter"), s.Token, 0, '*', func(t string) { s.Token = t })
				form.AddTextView("", i18n.T("cmd.init.provider_hint_openrouter"), 60, 1, true, false)

			case "github-copilot":
				form.AddTextView("", i18n.T("cmd.init.wizard_copilot_desc"), 60, 2, true, false)
				form.AddTextView("", "$ gh auth login", 60, 1, true, false)
			}

			form.AddButton(i18n.T("wizard.hint.submit"), onDone)
			return form
		},
		OnDone: func() error {
			a := *s.AppPtr
			switch s.SelectedProvider {
			case "bedrock":
				if s.AuthMode == "bearer" && s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.Bedrock, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			case "anthropic":
				if s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.Anthropic, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			case "openrouter":
				if s.Token != "" && a.Secrets != nil {
					keychainKey := providerPkg.KeychainKey(providerPkg.OpenRouter, "")
					if keychainKey != "" {
						if err := a.Secrets.Set(context.Background(), keychainKey, s.Token); err != nil {
							return fmt.Errorf("keychain: %w", err)
						}
					}
				}
			}

			return config.Update(func(c *config.Config) error {
				c.Opencode.DefaultProvider = s.SelectedProvider
				if s.SelectedProvider == "bedrock" {
					c.Provider.Bedrock.AuthMode = s.AuthMode
					if s.Region != "" {
						c.Provider.Bedrock.AWSRegion = s.Region
					}
					if s.AuthMode == "profile" && s.ProfileName != "" {
						c.Provider.Bedrock.AWSProfile = s.ProfileName
					}
				}
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			a := *s.AppPtr
			fields := []views.InfoField{{Label: "Provider", Value: s.SelectedProvider}}
			if s.SelectedProvider == "bedrock" {
				fields = append(fields, views.InfoField{Label: "Auth", Value: s.AuthMode})
				if s.Region != "" {
					fields = append(fields, views.InfoField{Label: "Region", Value: s.Region})
				}
			}
			if a.Secrets == nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: i18n.T("cmd.init.wizard_no_keyring"),
				})
			}
			if _, err := opencode.FindBinary(); err != nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: i18n.T("cmd.init.wizard_opencode_not_found"),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_credentials"),
	}
}

// buildProjectStep creates the first project creation step.
func buildProjectStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "project",
		Label: i18n.T("cmd.init.wizard_step_project"),
		SkipIf: func() bool {
			return s.ProjectSkipped
		},
		Validate: func() string {
			if s.ProjectName == "" {
				return i18n.T("cmd.init.wizard_project_name_required")
			}
			p := expandPath(s.ProjectPath)
			abs, err := filepath.Abs(p)
			if err != nil {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			info, err := os.Stat(abs)
			if err != nil {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			if !info.IsDir() {
				return i18n.Tf("cmd.init.wizard_project_path_invalid", s.ProjectPath)
			}
			s.ProjectPath = abs
			return ""
		},
		Form: func(tvApp *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			initialName := s.ProjectName
			initialPath := s.ProjectPath
			if initialPath == "" {
				initialPath = "."
			}
			form.AddInputField(i18n.T("cmd.init.wizard_project_name"), initialName, 0, nil, func(t string) { s.ProjectName = t })
			form.AddInputField(i18n.T("cmd.init.wizard_project_path"), initialPath, 0, nil, func(t string) { s.ProjectPath = t })

			attachMounted := false
			if s.TeamState.Configured && s.TeamState.TeamID != "" {
				attachOptions := []string{
					i18n.Tf("cmd.init.wizard_project_attach_yes", s.TeamState.TeamID),
					i18n.T("cmd.init.wizard_project_attach_no"),
				}
				s.TeamState.attachProject = true
				form.AddDropDown(i18n.T("cmd.init.wizard_project_attach_team"), attachOptions, 0, func(_ string, idx int) {
					s.TeamState.attachProject = idx == 0
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
			if (*s.AppPtr).Projects == nil {
				return fmt.Errorf("project store not initialized")
			}
			p := &domain.Project{
				ID:     uuid.New().String()[:8],
				Name:   s.ProjectName,
				Path:   s.ProjectPath,
				Status: domain.ProjectStatusActive,
			}
			if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
				tid := s.TeamState.TeamID
				p.TeamID = &tid
			}
			if err := (*s.AppPtr).Projects.Create(context.Background(), p); err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			s.ProjectCreated = true
			return nil
		},
		InfoFields: func() []views.InfoField {
			fields := []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_project"), Value: i18n.T("cmd.init.wizard_project_added")}}
			if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
				fields = append(fields, views.InfoField{
					Label: i18n.T("cmd.init.wizard_step_team"),
					Value: i18n.Tf("cmd.init.wizard_project_attached", s.TeamState.TeamID),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_project"),
	}
}

// countHubContent returns the number of agent and skill files found
// in the hub content directory. Returns (0, 0) when hubDir is empty or unreadable.
func countHubContent(hubDir string) (agents int, skills int) {
	if hubDir == "" {
		return 0, 0
	}
	agentDir := filepath.Join(hubDir, "agents")
	if entries, err := os.ReadDir(agentDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				agents++
			}
		}
	}
	skillDir := filepath.Join(hubDir, "skills")
	if entries, err := os.ReadDir(skillDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skills++
			}
		}
	}
	return
}

// buildDeployStep creates the deploy agents/skills step (conditional on project).
// The step displays a dynamic description listing the number of agents and skills
// available, and offers two explicit buttons: "Deploy now" / "Skip".
func buildDeployStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "deploy",
		Label: i18n.T("cmd.init.wizard_step_deploy"),
		SkipIf: func() bool {
			return s.ProjectSkipped || !s.ProjectCreated
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			s.DeployConfirmed = false
			form := tview.NewForm()

			// Build dynamic description with agent/skill counts.
			hubDir := findHubDir()
			agentCount, skillCount := countHubContent(hubDir)

			var desc string
			if agentCount > 0 {
				desc = i18n.Tf("cmd.init.wizard_deploy_desc", agentCount, skillCount)
			} else {
				desc = i18n.T("cmd.init.wizard_deploy_desc_fallback")
			}
			form.AddTextView("", desc, 60, 12, true, true)

			form.AddButton(i18n.T("cmd.init.wizard_deploy_now"), func() {
				s.DeployConfirmed = true
				onDone()
			})
			form.AddButton(i18n.T("cmd.init.wizard_deploy_skip_btn"), func() {
				s.DeployConfirmed = false
				onDone()
			})
			return form
		},
		OnDone: func() error {
			if !s.DeployConfirmed || !s.ProjectCreated {
				return nil
			}
			config.Reset()
			newApp, err := ReloadApp()
			if err != nil {
				return fmt.Errorf("reload: %w", err)
			}
			*s.AppPtr = newApp

			hubDir := findHubDir()
			if hubDir == "" {
				slog.Warn("deploy step skipped: hub content directory not found")
				return nil
			}

			plan := buildDeployPlan(*s.AppPtr, s.ProjectPath, "", hubDir, s.SelectedProvider, "", nil, nil, nil, nil)
			_, err = deploy.Execute(plan)
			return err
		},
		InfoFields: func() []views.InfoField {
			status := i18n.T("cmd.init.wizard_deploy_done")
			if !s.DeployConfirmed {
				status = i18n.T("cmd.init.wizard_deploy_skipped")
			}
			return []views.InfoField{{Label: "Deploy", Value: status}}
		},
		Processing: i18n.T("cmd.init.wizard_deploy_processing"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildIntroStep — group introduction page
// ─────────────────────────────────────────────────────────────────────────────

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

			fmt.Fprintf(&b, "%s%s%s\n\n", accent, title, reset)

			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")

			if listTitle != "" && listItems != "" {
				fmt.Fprintf(&b, "%s%s%s\n", muted, listTitle, reset)
				fmt.Fprintf(&b, "%s%s%s\n", accent, listItems, reset)
				b.WriteString("\n")
			}

			if prereqs != "" {
				fmt.Fprintf(&b, "%s┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄%s\n\n", muted, reset)
				for _, line := range strings.Split(prereqs, "\n") {
					if strings.HasPrefix(line, "• ") {
						fmt.Fprintf(&b, "%s•%s %s%s%s\n", accent, reset, muted, line[len("• "):], reset)
					} else {
						fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
					}
				}
				b.WriteString("\n")
			}

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
						skipConfirmed = true
						if btn := buttonForm.GetButton(1); btn != nil {
							btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
						}
						return
					}
					onSkip()
					onDone()
				})
			}

			buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})

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
