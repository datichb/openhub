package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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
	// ── Setup mode (selected on welcome screen) ──
	// "solo"  = skip team group entirely
	// "team"  = skip provider (inherited from team config)
	// "full"  = show all steps (default)
	SetupMode string

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
	ProjectID       string
	ProjectCreated  bool
	ProjectSkipped  bool
	DeployConfirmed bool
	ProjectChoice   string          // "keep" or "reconfigure" (only when ExistingProject != nil)
	ExistingProject *domain.Project // non-nil if a project was found at CWD

	// ── MCP ──
	FigmaToken   string
	GitlabToken  string
	GitlabWrite  bool
	GslidesToken string
	MCPSkipped   bool

	// ── Deploy ──
	SelectedAgents []string
	DeploySkipped  bool

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
// Layout helpers
// ─────────────────────────────────────────────────────────────────────────────

// styleWizardForm applies the standard wizard form theming.
func styleWizardForm(form *tview.Form) {
	form.SetBackgroundColor(theme.BgPanel)
	form.SetFieldBackgroundColor(theme.BgElement)
	form.SetFieldTextColor(theme.FgPrimary)
	form.SetLabelColor(theme.FgPrimary)
	form.SetBorder(false)
	form.SetBorderPadding(1, 1, 2, 2)
}

// ─────────────────────────────────────────────────────────────────────────────
// Step builders
// ─────────────────────────────────────────────────────────────────────────────

// infoColor wraps a value string in a tview color tag for the sidebar InfoFields.
func infoSuccess(v string) string { return theme.ColorTag(theme.SuccessHex) + v + theme.TagReset }
func infoMuted(v string) string   { return theme.ColorTag(theme.TextMutedHex) + v + theme.TagReset }
func infoWarning(v string) string { return theme.ColorTag(theme.WarningHex) + v + theme.TagReset }

// buildWelcomeStep creates the welcome/splash screen step.
func buildWelcomeStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:            "welcome",
		Label:         i18n.T("cmd.init.wizard_step_welcome"),
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			// Detect terminal height for adaptive text (full vs compact).
			_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
			if termH <= 0 {
				termH = 50
			}
			availH := termH - 7

			// Build welcome text — full (with ASCII art) or compact (title only).
			var tvText string
			if availH >= 35 {
				// Full layout: ASCII art banner
				tvText = fmt.Sprintf(`%s██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s
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

%s`+i18n.T("cmd.init.wizard_checklist_title")+`%s
%s☐%s `+i18n.T("cmd.init.wizard_checklist_provider")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_team")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_mcp")+`
`,
					accent, reset, accent, reset, accent, reset,
					accent, reset, accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
				accent, reset,
				accent, reset,
				accent, reset,
			)
		} else {
			// Compact layout: no ASCII art, title line only
			tvText = fmt.Sprintf(`%s`+i18n.T("cmd.init.wizard_welcome_title_compact")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_desc")+`%s

%s`+i18n.T("cmd.init.wizard_welcome_detail")+`%s

%s1.%s `+i18n.T("cmd.init.wizard_step_lang_desc")+`
%s2.%s `+i18n.T("cmd.init.wizard_step_provider_desc")+`
%s3.%s `+i18n.T("cmd.init.wizard_step_team_desc_welcome")+`
%s4.%s `+i18n.T("cmd.init.wizard_step_project_desc")+`
%s5.%s `+i18n.T("cmd.init.wizard_step_mcp_desc")+`

%s`+i18n.T("cmd.init.wizard_checklist_title")+`%s
%s☐%s `+i18n.T("cmd.init.wizard_checklist_provider")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_team")+`
%s☐%s `+i18n.T("cmd.init.wizard_checklist_mcp")+`
`,
					accent, reset,
					secondary, reset,
					muted, reset,
					accent, reset, accent, reset,
					accent, reset, accent, reset,
					accent, reset,
					secondary, reset,
				accent, reset,
				accent, reset,
				accent, reset,
			)
		}

		// Compute the width of the longest visible line for the mode selector.
		maxTextWidth := views.MaxVisibleWidth(tvText)

		// ── Setup mode selector ──
			modeOptions := []string{
				i18n.T("cmd.init.wizard_mode_solo"),
				i18n.T("cmd.init.wizard_mode_team"),
				i18n.T("cmd.init.wizard_mode_full"),
			}
			modeKeys := []string{"solo", "team", "full"}
			defaultMode := 2 // "full" by default — all steps visible
			if s.SetupMode == "solo" {
				defaultMode = 0
			} else if s.SetupMode == "team" {
				defaultMode = 1
			}
			if s.SetupMode == "" {
				s.SetupMode = "full"
			}

			modeSelect := widgets.NewInlineSelect(
				i18n.T("cmd.init.wizard_mode_label"),
				modeOptions,
				defaultMode,
				func(_ string, idx int) {
					if idx >= 0 && idx < len(modeKeys) {
						s.SetupMode = modeKeys[idx]
					}
				},
			)
			modeSelect.SetCentered(true).SetContentWidth(maxTextWidth)

			// Show descriptions only when there's enough vertical space.
			if availH >= 30 {
				modeSelect.SetDescriptions([]string{
					i18n.T("cmd.init.wizard_mode_solo_desc"),
					i18n.T("cmd.init.wizard_mode_team_desc"),
					i18n.T("cmd.init.wizard_mode_full_desc"),
				})
			}

			modeForm := tview.NewForm()
			modeForm.SetBackgroundColor(theme.BgPanel)
			modeForm.SetLabelColor(theme.FgPrimary)
			modeForm.SetFieldBackgroundColor(theme.BgPanel)
			modeForm.SetFieldTextColor(theme.FgPrimary)
			modeForm.SetBorder(false)
			modeForm.AddFormItem(modeSelect)

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_welcome_start")+"  ", onDone)

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:        tvText,
				SectionTitle: i18n.T("cmd.init.wizard_section_mode"),
				Content:      modeForm,
				Buttons:      buttonForm,
				FocusTarget:  modeForm,
			})
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:            tvApp,
				Content:        modeForm,
				Buttons:        buttonForm,
				IsContentAtEnd: func() bool { return true },
			})
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
// The intro text (title, description, prerequisites) is embedded at the top
// of the form, eliminating the need for a separate intro page.
func buildProviderStep(s *initStepState) views.WizardStep {
	a := *s.AppPtr
	return views.WizardStep{
		ID:    "provider",
		Label: i18n.T("cmd.init.wizard_step_provider_label"),
		SkipIf: func() bool {
			return s.ProviderSkipped
		},
		Validate: func() string {
			if s.ProviderSkipped {
				return "" // Skip mode: bypass validation.
			}
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
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Auto-detection: pre-select the first available provider ──
			detectedSource := ""
			reusedFromConfig := false
			if s.SelectedProvider == "" {
				// Run provider detection to find available credentials.
				detections := providerPkg.DetectAll()
				for _, d := range detections {
					if d.Available {
						name := string(d.Provider)
						for idx, opt := range s.ProviderOptions {
							if opt == name {
								s.SelectedProvider = name
								s.ProviderIdx = idx
								detectedSource = d.Source
								if d.Details != "" {
									detectedSource += " (" + d.Details + ")"
								}
								break
							}
						}
						if s.SelectedProvider != "" {
							break
						}
					}
				}
			} else {
				// Provider was already set (from config pre-fill) — mark as reused.
				reusedFromConfig = true
			}
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

			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			reset := theme.TagColor

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_provider_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_provider_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			intro.WriteString("\n")
			fmt.Fprintf(&intro, "%s%s%s  %s%s%s\n", muted, i18n.T("cmd.init.wizard_intro_provider_list"), reset, accent, i18n.T("cmd.init.wizard_provider_list_items"), reset)
			intro.WriteString("\n")
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_provider_prereq"), "\n") {
				if strings.HasPrefix(line, "• ") {
					fmt.Fprintf(&intro, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
				} else {
					fmt.Fprintf(&intro, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
				}
			}

			// ── Form with interactive fields only ──
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

			// Show detection/reuse source hint when provider was pre-selected.
			if detectedSource != "" {
				info := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", info, i18n.Tf("cmd.init.wizard_detected_from", detectedSource), reset), 60, 1, true, false)
			} else if reusedFromConfig {
				info := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", info, i18n.T("cmd.init.wizard_reused_from"), reset), 60, 1, true, false)
			}

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

			views.FixFormDropDownStyles(form)
			views.FixFormLabelFocus(form)

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.ProviderSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.ProviderSkipped = true
				s.Token = ""
				onDone()
			})

			introText := intro.String()
			maxW := views.MaxVisibleWidth(introText)
			styleWizardForm(form)

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:           introText,
				SectionTitle:    i18n.T("cmd.init.wizard_section_config"),
				Content:         form,
				ContentMaxWidth: maxW,
				Buttons:         buttonForm,
				FocusTarget:     form,
			})
			views.SetupFormNavigation(form)
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Content: form,
				Buttons: buttonForm,
			})
		},
		OnDone: func() error {
			if s.ProviderSkipped {
				return nil // Skip mode: nothing to persist.
			}
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
					Value: infoWarning(i18n.T("cmd.init.wizard_no_keyring")),
				})
			}
			if _, err := opencode.FindBinary(); err != nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: infoWarning(i18n.T("cmd.init.wizard_opencode_not_found")),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_credentials"),
	}
}

// buildProjectStep creates the first project creation step.
// The intro text (title, description, optional note) is embedded at the top
// of the form, eliminating the need for a separate intro page.
func buildProjectStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "project",
		Label: i18n.T("cmd.init.wizard_step_project"),
		SkipIf: func() bool {
			return s.ProjectSkipped
		},
		Validate: func() string {
			if s.ProjectSkipped {
				return "" // Skip mode: bypass validation.
			}
			if s.ExistingProject != nil && s.ProjectChoice == "" {
				return i18n.T("cmd.init.wizard_project_choice_required")
			}
			if s.ExistingProject != nil && s.ProjectChoice == "keep" {
				return "" // Keeping existing project — no field validation needed.
			}
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

			// Reject if another project (different path) already uses this name.
			if store := (*s.AppPtr).Projects; store != nil {
				if existing, err := store.GetByName(context.Background(), s.ProjectName); err == nil && existing != nil {
					if existing.Path != abs {
						return i18n.Tf("cmd.init.wizard_project_name_duplicate", s.ProjectName)
					}
					// Same path → will be updated in OnDone, not a conflict.
				}
			}
			return ""
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor
			infoColor := theme.ColorTag(theme.InfoHex)

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_project_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_project_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			note := i18n.T("cmd.init.wizard_intro_project_optional")
			if note != "" {
				fmt.Fprintf(&intro, "%s%s%s\n", muted, note, reset)
			}

			// ── Existing project recap ──
			if s.ExistingProject != nil {
				intro.WriteString("\n")
				fmt.Fprintf(&intro, "%s%s %s%s\n", infoColor, theme.IconInfo,
					i18n.T("cmd.init.wizard_project_existing"), reset)
				fmt.Fprintf(&intro, "%s  %s%s\n", muted,
					i18n.Tf("cmd.init.wizard_project_existing_info", s.ExistingProject.Name, s.ExistingProject.Path), reset)
				if s.ExistingProject.TeamID != nil && *s.ExistingProject.TeamID != "" {
					fmt.Fprintf(&intro, "%s  %s%s\n", muted,
						i18n.Tf("cmd.init.wizard_project_existing_team", *s.ExistingProject.TeamID), reset)
				}
			}

			// ── Auto-detection: pre-fill project name from git remote ──
			detectedProject := ""
			if s.ProjectName == "" {
				if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
					remote := strings.TrimSpace(string(out))
					if remote != "" {
						s.ProjectName = config.RepoNameFromRemote(remote)
						detectedProject = "git remote"
					}
				}
			}
			if s.ProjectPath == "" {
				s.ProjectPath = "."
			}

			// ── Form rebuilt dynamically based on existing project + choice ──
			form := tview.NewForm()

			// addTeamAttachment adds the team dropdown if a team is configured.
			addTeamAttachment := func() {
				if s.TeamState.Configured && s.TeamState.TeamID != "" {
					attachOptions := []string{
						i18n.Tf("cmd.init.wizard_project_attach_yes", s.TeamState.TeamID),
						i18n.T("cmd.init.wizard_project_attach_no"),
					}
					s.TeamState.attachProject = true
					attachMounted := false
					form.AddDropDown(i18n.T("cmd.init.wizard_project_attach_team"), attachOptions, 0, func(_ string, idx int) {
						s.TeamState.attachProject = idx == 0
						if attachMounted {
							views.AutoAdvanceFromDropDown(tvApp, form, 2)
						}
					})
					defer func() { attachMounted = true }()
				}
			}

			var onFormRebuilt func()

			var rebuildForm func(focusIdx int)
			rebuildForm = func(focusIdx int) {
				form.Clear(true)

				if s.ExistingProject != nil {
					// ── Existing project detected: show choice dropdown ──
					options := []string{
						i18n.T("cmd.init.wizard_select_placeholder"),
						i18n.T("cmd.init.wizard_project_choice_keep"),
						i18n.T("cmd.init.wizard_project_choice_reconfigure"),
					}
					defaultIdx := 0
					if s.ProjectChoice == "keep" {
						defaultIdx = 1
					} else if s.ProjectChoice == "reconfigure" {
						defaultIdx = 2
					}
					choiceMounted := false
					form.AddDropDown(i18n.T("cmd.init.wizard_project_choice_label"), options, defaultIdx, func(_ string, idx int) {
						switch idx {
						case 1:
							s.ProjectChoice = "keep"
						case 2:
							s.ProjectChoice = "reconfigure"
						default:
							s.ProjectChoice = ""
						}
						if choiceMounted {
							go func() { tvApp.QueueUpdateDraw(func() { rebuildForm(0) }) }()
						}
					})
					choiceMounted = true

					if s.ProjectChoice == "keep" {
						// Keep mode: only show team attachment option.
						addTeamAttachment()
					} else if s.ProjectChoice == "reconfigure" {
						// Reconfigure mode: show editable fields (pre-filled).
						form.AddInputField(i18n.T("cmd.init.wizard_project_name"), s.ProjectName, 0, nil, func(t string) { s.ProjectName = t })
						form.AddInputField(i18n.T("cmd.init.wizard_project_path"), s.ProjectPath, 0, nil, func(t string) { s.ProjectPath = t })
						addTeamAttachment()
					}
					// else: placeholder selected, show nothing extra.
				} else {
					// ── No existing project: standard creation form ──
					form.AddInputField(i18n.T("cmd.init.wizard_project_name"), s.ProjectName, 0, nil, func(t string) { s.ProjectName = t })
					if detectedProject != "" {
						form.AddTextView("", fmt.Sprintf("%s%s%s", infoColor, i18n.Tf("cmd.init.wizard_detected_from", detectedProject), reset), 60, 1, true, false)
					}
					form.AddInputField(i18n.T("cmd.init.wizard_project_path"), s.ProjectPath, 0, nil, func(t string) { s.ProjectPath = t })
					addTeamAttachment()
				}

				views.FixFormDropDownStyles(form)
				if focusIdx >= 0 {
					form.SetFocus(focusIdx)
				}
				views.FixFormLabelFocus(form)
				tvApp.SetFocus(form)
				if onFormRebuilt != nil {
					onFormRebuilt()
				}
			}
			rebuildForm(-1)

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.ProjectSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.ProjectSkipped = true
				s.ProjectName = ""
				s.ProjectPath = ""
				onDone()
			})

			introText := intro.String()
			maxW := views.MaxVisibleWidth(introText)
			styleWizardForm(form)

			pageResult := views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:           introText,
				SectionTitle:    i18n.T("cmd.init.wizard_section_project"),
				Content:         form,
				ContentMaxWidth: maxW,
				Buttons:         buttonForm,
				FocusTarget:     form,
			})
			onFormRebuilt = func() {
				if pageResult.ResizeContent != nil {
					pageResult.ResizeContent(form)
				}
			}
			views.SetupFormNavigation(form)
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Content: form,
				Buttons: buttonForm,
			})
		},
		OnDone: func() error {
			if s.ProjectSkipped {
				return nil // Skip mode: nothing to persist.
			}
			store := (*s.AppPtr).Projects
			if store == nil {
				return fmt.Errorf("project store not initialized")
			}
			ctx := context.Background()

			if s.ExistingProject != nil && s.ProjectChoice == "keep" {
				// Keep existing project, but update team attachment if changed.
				existing := s.ExistingProject
				needUpdate := false
				if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
					tid := s.TeamState.TeamID
					if existing.TeamID == nil || *existing.TeamID != tid {
						existing.TeamID = &tid
						needUpdate = true
					}
				}
				if needUpdate {
					if err := store.Update(ctx, existing); err != nil {
						return fmt.Errorf("update project: %w", err)
					}
				}
				s.ProjectID = existing.ID
				s.ProjectName = existing.Name
				s.ProjectPath = existing.Path
				s.ProjectCreated = true
				return nil
			}

			// "reconfigure" or new project — upsert logic.
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

			result, _, err := upsertProject(ctx, store, p)
			if err != nil {
				return err
			}
			s.ProjectID = result.ID
			s.ProjectCreated = true
			return nil
		},
		InfoFields: func() []views.InfoField {
			var status string
			if s.ExistingProject != nil && s.ProjectChoice == "keep" {
				status = infoSuccess(i18n.T("cmd.init.wizard_project_kept"))
			} else {
				status = infoSuccess(i18n.T("cmd.init.wizard_project_added"))
			}
			fields := []views.InfoField{{Label: i18n.T("cmd.init.wizard_step_project"), Value: status}}
			if s.TeamState.Configured && s.TeamState.attachProject && s.TeamState.TeamID != "" {
				fields = append(fields, views.InfoField{
					Label: i18n.T("cmd.init.wizard_step_team"),
					Value: infoSuccess(i18n.Tf("cmd.init.wizard_project_attached", s.TeamState.TeamID)),
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

// ─────────────────────────────────────────────────────────────────────────────
// buildAgentSelectionStep — checkbox form for agent selection
// ─────────────────────────────────────────────────────────────────────────────

// buildAgentSelectionStep creates a form step with a checkbox per hub agent.
// All agents are selected by default. The result is stored in s.SelectedAgents.
// The collection of selected agents happens in Validate (called by the wizard
// engine before advancing) rather than in a form button callback, because in
// grouped mode the engine strips form buttons and replaces them with its own.
// The deploy intro text is embedded at the top of the form.
func buildAgentSelectionStep(s *initStepState) views.WizardStep {
	// Shared between CustomView and Validate closures.
	var available []string
	var selected map[string]bool

	return views.WizardStep{
		ID:    "agents",
		Label: i18n.T("cmd.init.wizard_step_agents"),
		SkipIf: func() bool {
			return s.DeploySkipped || s.ProjectSkipped || !s.ProjectCreated
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Intro text ──
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			var intro strings.Builder
			fmt.Fprintf(&intro, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_intro_deploy_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_deploy_desc"), "\n") {
				fmt.Fprintf(&intro, "%s%s%s\n", secondary, line, reset)
			}
			intro.WriteString("\n")
			listTitle := i18n.T("cmd.init.wizard_intro_deploy_list")
			listItems := i18n.T("cmd.init.wizard_deploy_list_items")
			if listTitle != "" && listItems != "" {
				fmt.Fprintf(&intro, "%s%s%s  %s%s%s\n", muted, listTitle, reset, accent, listItems, reset)
			}
			note := i18n.T("cmd.init.wizard_intro_deploy_note")
			if note != "" {
				fmt.Fprintf(&intro, "\n%s%s%s", muted, note, reset)
			}

			// ── Form with checkboxes only ──
			form := tview.NewForm()
			available = discoverAgents()
			selected = make(map[string]bool, len(available))
			for _, ag := range available {
				selected[ag] = true
			}
			for _, ag := range available {
				agName := ag
				form.AddCheckbox(agName, true, func(checked bool) {
					selected[agName] = checked
				})
			}

			// ── Button bar with Submit + Skip (double-click confirm) ──
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.DeploySkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.DeploySkipped = true
				s.SelectedAgents = nil
				onDone()
			})

			introText := intro.String()
			maxW := views.MaxVisibleWidth(introText)
			styleWizardForm(form)

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:           introText,
				SectionTitle:    i18n.T("cmd.init.wizard_section_agents"),
				Content:         form,
				ContentMaxWidth: maxW,
				Buttons:         buttonForm,
				FocusTarget:     form,
			})
			views.SetupFormNavigation(form)
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Content: form,
				Buttons: buttonForm,
			})
		},
		Validate: func() string {
			if s.DeploySkipped {
				return "" // Skip mode: bypass validation.
			}
			// Collect selected agents into shared state before advancing.
			s.SelectedAgents = nil
			for _, ag := range available {
				if selected[ag] {
					s.SelectedAgents = append(s.SelectedAgents, ag)
				}
			}
			return "" // no validation error
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{
				Label: i18n.T("cmd.init.wizard_step_agents"),
				Value: i18n.Tf("cmd.init.wizard_deploy_recap_agents", len(s.SelectedAgents)),
			}}
		},
	}
}

// buildDeployStep creates the deploy confirmation step with a recap of what
// will be deployed, and two buttons: "Deploy now" / "Deploy later".
// The step displays a summary of selected agents, detected skills, configured
// MCP servers, and the chosen provider.
func buildDeployStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:    "deploy",
		Label: i18n.T("cmd.init.wizard_step_deploy_confirm"),
		SkipIf: func() bool {
			return s.DeploySkipped || s.ProjectSkipped || !s.ProjectCreated
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			s.DeployConfirmed = false

			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			hubDir := findHubDir()
			_, skillCount := countHubContent(hubDir)

			// Build recap text.
			var b strings.Builder
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_deploy_recap_title"), reset)

			// ── Agents recap ──
			agentCount := len(s.SelectedAgents)
			fmt.Fprintf(&b, "  %s%s%s\n", secondary,
				i18n.Tf("cmd.init.wizard_deploy_recap_agents", agentCount), reset)
			if agentCount > 0 {
				const maxShow = 6
				shown := s.SelectedAgents
				if len(shown) > maxShow {
					list := strings.Join(shown[:maxShow], ", ")
					fmt.Fprintf(&b, "  %s%s%s\n", muted,
						i18n.Tf("cmd.init.wizard_deploy_recap_agents_more", list, agentCount-maxShow), reset)
				} else {
					fmt.Fprintf(&b, "  %s%s%s\n", muted,
						i18n.Tf("cmd.init.wizard_deploy_recap_agents_list", strings.Join(shown, ", ")), reset)
				}
			}
			b.WriteString("\n")

			// ── Skills recap ──
			fmt.Fprintf(&b, "  %s%s%s\n\n", secondary,
				i18n.Tf("cmd.init.wizard_deploy_recap_skills", skillCount), reset)

			// ── MCP recap ──
			var mcpList []string
			if s.FigmaToken != "" {
				mcpList = append(mcpList, "Figma")
			}
			if s.GitlabToken != "" {
				mcpList = append(mcpList, "GitLab")
			}
			if s.GslidesToken != "" {
				mcpList = append(mcpList, "Google Slides")
			}
			if len(mcpList) > 0 {
				fmt.Fprintf(&b, "  %s%s%s\n\n", secondary,
					i18n.Tf("cmd.init.wizard_deploy_recap_mcp", strings.Join(mcpList, ", ")), reset)
			} else {
				fmt.Fprintf(&b, "  %s%s%s\n\n", muted,
					i18n.T("cmd.init.wizard_deploy_recap_mcp_none"), reset)
			}

			// ── Provider recap ──
			if s.SelectedProvider != "" {
				fmt.Fprintf(&b, "  %s%s%s\n", secondary,
					i18n.Tf("cmd.init.wizard_deploy_recap_provider", s.SelectedProvider), reset)
			}

			tv := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignCenter)
			tv.SetBackgroundColor(theme.BgPanel)
			tv.SetText(b.String())

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_deploy_now")+"  ", func() {
				s.DeployConfirmed = true
				onDone()
			})
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_deploy_skip_btn")+"  ", func() {
				s.DeployConfirmed = false
				onDone()
			})

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:       b.String(),
				Buttons:     buttonForm,
				FocusTarget: buttonForm,
			})
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Buttons: buttonForm,
			})
		},
		OnDone: func() error {
			ctx := context.Background()
			a := *s.AppPtr

			// Always persist the agent selection to the project in DB,
			// whether deploying now or later. A future 'oh deploy' will
			// pick up project.Agents automatically.
			if s.ProjectCreated && s.ProjectID != "" && a.Projects != nil {
				project, err := a.Projects.Get(ctx, s.ProjectID)
				if err != nil {
					slog.Warn("deploy step: could not fetch project for agent update", "err", err)
				} else {
					project.Agents = s.SelectedAgents
					project.Provider = s.SelectedProvider
					if err := a.Projects.Update(ctx, project); err != nil {
						slog.Warn("deploy step: could not update project agents", "err", err)
					}
				}
			}

			if !s.DeployConfirmed {
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

			// Fetch the updated project so buildDeployPlan gets the full
			// context (Agents, MCPConfig, ModelOverrides, WorkflowConfig).
			a = *s.AppPtr
			var proj *domain.Project
			if a.Projects != nil && s.ProjectID != "" {
				proj, _ = a.Projects.Get(ctx, s.ProjectID)
			}

			plan := buildDeployPlan(a, DeployRequest{
				Project:        proj,
				ProjectPath:    s.ProjectPath,
				HubDir:         hubDir,
				Provider:       s.SelectedProvider,
				SelectedAgents: s.SelectedAgents,
			})
			_, err = deploy.Execute(ctx, plan)
			return err
		},
		InfoFields: func() []views.InfoField {
			if s.DeploySkipped {
				return []views.InfoField{{Label: "Deploy", Value: infoMuted(i18n.T("cmd.init.wizard_deploy_section_skipped"))}}
			}
			if s.DeployConfirmed {
				return []views.InfoField{{Label: "Deploy", Value: infoSuccess(i18n.T("cmd.init.wizard_deploy_done"))}}
			}
			return []views.InfoField{{Label: "Deploy", Value: infoMuted(i18n.T("cmd.init.wizard_deploy_skipped"))}}
		},
		Processing: i18n.T("cmd.init.wizard_deploy_processing"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP Consolidated Step — combines intro + 3 token steps into a single page
// ─────────────────────────────────────────────────────────────────────────────

// buildMCPConsolidatedStep returns a single WizardStep that replaces the MCP
// intro page and the 3 individual token steps (Figma, GitLab, Google Slides).
// Each integration has a checkbox to enable/disable it and a password field
// that appears only when the checkbox is checked.
func buildMCPConsolidatedStep(s *initStepState, a *app.App) views.WizardStep {
	type mcpEntry struct {
		name          string
		tokenVar      *string
		tokenKey      string
		hintKey       string
		checkboxVar   *bool
		checkboxLabel string
		checkboxDesc  string
		afterStore    func() error
	}

	entries := []mcpEntry{
		{
			name: "Figma", tokenVar: &s.FigmaToken,
			tokenKey: config.DefaultFigmaTokenKey, hintKey: "cmd.init.mcp_hint_figma",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error { c.MCP.Figma.Enabled = true; return nil })
			},
		},
		{
			name: "GitLab", tokenVar: &s.GitlabToken,
			tokenKey: config.DefaultGitLabTokenKey, hintKey: "cmd.init.mcp_hint_gitlab",
			checkboxVar: &s.GitlabWrite, checkboxLabel: i18n.T("cmd.init.mcp_gitlab_write_short"),
			checkboxDesc: "cmd.init.mcp_gitlab_write_desc",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error {
					c.MCP.Gitlab.Enabled = true
					if s.GitlabWrite {
						c.MCP.Gitlab.WriteEnabled = true
					}
					return nil
				})
			},
		},
		{
			name: "Google Slides", tokenVar: &s.GslidesToken,
			tokenKey: config.DefaultGslidesTokenKey, hintKey: "cmd.init.mcp_hint_gslides",
			afterStore: func() error {
				return config.Update(func(c *config.Config) error { c.MCP.Gslides.Enabled = true; return nil })
			},
		},
	}

	return views.WizardStep{
		ID: "mcp_consolidated", Label: "MCP",
		SkipIf: func() bool { return s.MCPSkipped },
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			reset := theme.TagColor

			// Render intro text above the form.
			var b strings.Builder
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_intro_mcp_title"), reset)
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_intro_mcp_desc"), "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n", muted, i18n.T("cmd.init.wizard_intro_mcp_list"), reset)
			fmt.Fprintf(&b, "%s%s%s\n", accent, i18n.T("cmd.init.wizard_mcp_list_items"), reset)
			b.WriteString("\n")
			for _, line := range strings.Split(i18n.T("cmd.init.wizard_mcp_prereq"), "\n") {
				if strings.HasPrefix(line, "• ") {
					fmt.Fprintf(&b, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
				} else {
					fmt.Fprintf(&b, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
				}
			}

			// Build the form with checkbox + token per integration.
			form := tview.NewForm()

			enabled := make([]bool, len(entries))
			for i := range entries {
				enabled[i] = true
			}

			// rebuildForm reconstructs the form after a checkbox toggle.
			// focusEntry is the index of the entry whose checkbox was toggled
			// (-1 on the initial build). After rebuild, focus is restored to
			// that checkbox so keyboard navigation keeps working.
			var onFormRebuilt func()
			var rebuildForm func(focusEntry int)
			rebuildForm = func(focusEntry int) {
				form.Clear(true)
				targetFormIdx := 0
				for idx := range entries {
					ci := idx
					e := entries[idx]
					if ci == focusEntry {
						targetFormIdx = form.GetFormItemCount()
					}
					form.AddCheckbox(e.name, enabled[ci], func(checked bool) {
						enabled[ci] = checked
						go func() { tvApp.QueueUpdateDraw(func() { rebuildForm(ci) }) }()
					})
					if !enabled[ci] {
						continue
					}
					hasKeychainToken := false
					if a.Secrets != nil && *e.tokenVar == "" {
						if existing, err := a.Secrets.Get(context.Background(), e.tokenKey); err == nil && existing != "" {
							hasKeychainToken = true
						}
					}
					if hasKeychainToken {
						form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 1, true, false)
					}
					form.AddPasswordField(
						i18n.Tf("cmd.init.mcp_token_prompt", e.name), *e.tokenVar, 0, '*',
						func(t string) { *entries[ci].tokenVar = t },
					)
					if e.hintKey != "" {
						form.AddTextView("", i18n.T(e.hintKey), 60, 2, true, false)
					}
					if e.checkboxVar != nil {
						if e.checkboxDesc != "" {
							form.AddTextView("", i18n.T(e.checkboxDesc), 60, 2, true, false)
						}
						cbVar := e.checkboxVar
						form.AddCheckbox(e.checkboxLabel, *cbVar, func(checked bool) { *cbVar = checked })
					}
				}
				views.FixFormDropDownStyles(form)
				form.SetFocus(targetFormIdx)
				views.FixFormLabelFocus(form)
				tvApp.SetFocus(form)
				if onFormRebuilt != nil {
					onFormRebuilt()
				}
			}
			rebuildForm(-1)

			// Button bar with Continue + Skip (double-click confirm).
			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
				s.MCPSkipped = false
				onDone()
			})
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(1); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				s.MCPSkipped = true
				s.FigmaToken = ""
				s.GitlabToken = ""
				s.GslidesToken = ""
				onDone()
			})

			introText := b.String()
			maxW := views.MaxVisibleWidth(introText)
			styleWizardForm(form)

			pageResult := views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:           introText,
				SectionTitle:    i18n.T("cmd.init.wizard_section_mcp"),
				Content:         form,
				ContentMaxWidth: maxW,
				Buttons:         buttonForm,
				FocusTarget:     form,
			})
			onFormRebuilt = func() {
				if pageResult.ResizeContent != nil {
					pageResult.ResizeContent(form)
				}
			}
			views.SetupFormNavigation(form)
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Content: form,
				Buttons: buttonForm,
			})
		},
		OnDone: func() error {
			for _, entry := range entries {
				if *entry.tokenVar == "" {
					continue
				}
				if a.Secrets != nil {
					if err := a.Secrets.Set(context.Background(), entry.tokenKey, *entry.tokenVar); err != nil {
						return fmt.Errorf("keychain %s: %w", entry.name, err)
					}
				}
				if entry.afterStore != nil {
					if err := entry.afterStore(); err != nil {
						return err
					}
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			var fields []views.InfoField
			for _, entry := range entries {
				if *entry.tokenVar == "" {
					fields = append(fields, views.InfoField{Label: entry.name, Value: infoMuted(i18n.T("cmd.init.wizard_mcp_skipped"))})
				} else {
					fields = append(fields, views.InfoField{Label: entry.name, Value: infoSuccess(i18n.T("cmd.init.wizard_mcp_configured"))})
				}
			}
			if s.GitlabToken != "" && s.GitlabWrite {
				fields = append(fields, views.InfoField{Label: "Write", Value: infoSuccess(i18n.T("cmd.init.wizard_mcp_enabled"))})
			}
			return fields
		},
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
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
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
						fmt.Fprintf(&b, "%s•%s %s%s%s\n", warning, reset, secondary, line[len("• "):], reset)
					} else {
						fmt.Fprintf(&b, "%s%s %s%s\n", warning, theme.IconWarning, line, reset)
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

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Badge:       badge,
				Intro:       b.String(),
				Buttons:     buttonForm,
				FocusTarget: buttonForm,
			})
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Buttons: buttonForm,
			})
		},
	}
}
