package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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
	// ── Setup type (selected on the welcome screen) ──
	// "solo"  = skip team group entirely (default)
	// "team"  = skip provider (inherited from team config)
	// "full"  = show all steps
	SetupMode string

	// ── Language ──
	SelectedLang string
	LangIdx      int
	LangChanged  bool // the welcome screen was redrawn for a new language

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
	ProjectChoice   string          // "keep" or "reconfigure" (only when ExistingProject != nil)
	ExistingProject *domain.Project // non-nil if a project was found at CWD

	// ── Shared refs ──
	TeamState *initWizardTeamState
	AppPtr    **app.App           // double pointer: team/deploy steps reload the app
	Steps     *[]views.WizardStep // pointer to the assembled steps slice (for Rerender lookup)

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

// Heights (rows available to a wizard page) from which the welcome screen
// shows more: the banner, then every setup type with its description. Below
// that, the description of the highlighted setup type still shows, on one line.
const (
	welcomeBannerMinHeight = 38
	welcomeDescMinHeight   = 30
)

var welcomeBanner = []string{
	"██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗",
	"██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗",
	"██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝",
	"██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗",
	"╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝",
	" ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝",
}

// welcomeBannerFits reports whether the banner fits the width of the page
// (it would wrap into noise otherwise); true when the width is unknown.
func welcomeBannerFits() bool {
	innerW := views.StepContentInnerWidth()
	if innerW == 0 {
		return true
	}
	banner := 0
	for _, line := range welcomeBanner {
		banner = max(banner, views.MaxVisibleWidth(line))
	}
	return innerW >= banner+4 // the intro keeps 2 cells of padding on each side
}

// initLangCodes and initLangLabels are the languages of the welcome screen
// (same order).
var (
	initLangCodes  = []string{"fr", "en"}
	initLangLabels = []string{"Français", "English"}
)

// initSetupModes are the setup types, in the order of the welcome screen.
var initSetupModes = []string{"solo", "team", "full"}

// availableWizardHeight is the number of rows a wizard page can use:
// the terminal minus the omnibar (5), the border (1) and the hints (1).
// It is a variable so tests can pretend a small terminal.
var availableWizardHeight = func() int {
	_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
	if termH <= 0 {
		termH = 50
	}
	return termH - 7
}

// defaultSetupMode is the setup type preselected on the welcome screen: solo,
// or the complete one when a team is already configured (a re-init must not
// skip it silently).
func (s *initStepState) defaultSetupMode() string {
	if s.TeamState != nil && s.TeamState.ExistingTeam != nil {
		return "full"
	}
	return "solo"
}

// setupModeLabel is the name of a setup type in the current language.
func setupModeLabel(mode string) string {
	if !slices.Contains(initSetupModes, mode) {
		return ""
	}
	return i18n.T("cmd.init.wizard_mode_" + mode)
}

// rerenderStep draws the step of that ID again (a language change, a
// dropdown that changes the fields). It never runs inside the tview handler
// that asks for it: the update is queued.
func rerenderStep(tvApp *tview.Application, s *initStepState, id string) {
	if s.Steps == nil {
		return
	}
	step := views.StepByID(*s.Steps, id)
	if step == nil || step.Rerender == nil {
		return
	}
	rerender := step.Rerender
	go func() { tvApp.QueueUpdateDraw(rerender) }()
}

// wizardIntroText is the header of a wizard page: a title, a description and
// an optional note, centered by the page.
func wizardIntroText(title, desc, note string) string {
	accent := theme.ColorTag(theme.ActiveMode.AccentHex)
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder
	b.WriteString("\n")
	fmt.Fprintf(&b, "%s%s%s\n\n", accent, title, reset)
	for _, line := range strings.Split(desc, "\n") {
		fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
	}
	if note != "" {
		for _, line := range strings.Split(note, "\n") {
			fmt.Fprintf(&b, "%s%s%s\n", muted, line, reset)
		}
	}
	return b.String()
}

// welcomeIntroText is the header of the welcome screen: the banner when the
// terminal is tall enough, else a title line, then what OpenHub is.
func welcomeIntroText(banner bool) string {
	accent := theme.ColorTag(theme.ActiveMode.AccentHex)
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	reset := theme.TagColor

	var b strings.Builder
	if banner {
		for _, line := range welcomeBanner {
			fmt.Fprintf(&b, "%s%s%s\n", accent, line, reset)
		}
		fmt.Fprintf(&b, "\n%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_welcome_title"), reset)
	} else {
		fmt.Fprintf(&b, "%s%s%s\n\n", accent, i18n.T("cmd.init.wizard_welcome_title_compact"), reset)
	}
	fmt.Fprintf(&b, "%s%s%s\n", secondary, i18n.T("cmd.init.wizard_welcome_desc"), reset)
	return b.String()
}

// buildWelcomeStep creates the first screen: the language and the setup type
// on the same page. Changing the language redraws the page in that language.
func buildWelcomeStep(s *initStepState) views.WizardStep {
	return views.WizardStep{
		ID:       "welcome",
		Label:    i18n.T("cmd.init.wizard_step_welcome"),
		Required: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			availH := availableWizardHeight()
			intro := welcomeIntroText(availH >= welcomeBannerMinHeight && welcomeBannerFits())
			blockWidth := views.MaxVisibleWidth(intro)

			if s.SetupMode == "" {
				s.SetupMode = s.defaultSetupMode()
			}

			langSelect := widgets.NewInlineSelect(
				i18n.T("cmd.init.wizard_lang_select"),
				initLangLabels,
				s.LangIdx,
				func(_ string, idx int) {
					if idx < 0 || idx >= len(initLangCodes) || s.LangIdx == idx {
						return
					}
					s.LangIdx = idx
					s.SelectedLang = initLangCodes[idx]
					i18n.SetLocale(s.SelectedLang)
					s.LangChanged = true
					rerenderStep(tvApp, s, "welcome")
				},
			)
			langSelect.SetCentered(true).SetContentWidth(blockWidth)

			modeOptions := make([]string, len(initSetupModes))
			modeDescs := make([]string, len(initSetupModes))
			defaultMode := 0
			for i, mode := range initSetupModes {
				modeOptions[i] = setupModeLabel(mode)
				modeDescs[i] = i18n.T("cmd.init.wizard_mode_" + mode + "_desc")
				if mode == s.SetupMode {
					defaultMode = i
				}
			}
			modeSelect := widgets.NewInlineSelect(
				i18n.T("cmd.init.wizard_section_mode"),
				modeOptions,
				defaultMode,
				func(_ string, idx int) {
					if idx >= 0 && idx < len(initSetupModes) {
						s.SetupMode = initSetupModes[idx]
					}
				},
			)
			modeSelect.SetCentered(true).SetContentWidth(blockWidth)
			// The description of the setup type is always visible: for each
			// option when there is room, else on one line under the selector.
			modeSelect.SetDescriptions(modeDescs)
			modeSelect.SetCompactDescription(availH < welcomeDescMinHeight)

			form := tview.NewForm()
			form.SetBackgroundColor(theme.BgPanel)
			form.SetLabelColor(theme.FgPrimary)
			form.SetFieldBackgroundColor(theme.BgPanel)
			form.SetFieldTextColor(theme.FgPrimary)
			form.SetBorder(false)
			form.AddFormItem(langSelect)
			form.AddFormItem(modeSelect)
			if s.LangChanged {
				// The page was redrawn for a new language: stay on the form,
				// on the next field.
				s.LangChanged = false
				form.SetFocus(1)
			}

			buttonForm := views.NewStyledButtonForm()
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_welcome_start")+"  ", onDone)

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:       intro,
				Content:     form,
				Buttons:     buttonForm,
				FocusTarget: form,
			})
			views.SetupCrossSectionNav(views.CrossSectionNavConfig{
				App:     tvApp,
				Content: form,
				Buttons: buttonForm,
				IsContentAtEnd: func() bool {
					idx, _ := form.GetFocusedItemIndex()
					return idx >= form.GetFormItemCount()-1
				},
			})
		},
		OnDone: func() error {
			i18n.SetLocale(s.SelectedLang)
			return updateHubConfig(*s.AppPtr, func(c *config.Config) error {
				c.CLI.Language = s.SelectedLang
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			lang := initLangLabels[0]
			if s.SelectedLang == "en" {
				lang = initLangLabels[1]
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_step_lang"), Value: lang},
				{Label: i18n.T("cmd.init.wizard_section_mode"), Value: setupModeLabel(s.SetupMode)},
			}
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
				if msg := bedrockRegionStepProblem(s.Region); msg != "" {
					return msg
				}
			}
			return ""
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// ── Auto-detection: pre-select the first available provider ──
			detectedSource := ""
			if s.SelectedProvider == "" {
				// Run provider detection to find available credentials.
				detections := providerPkg.DetectAll()
				for _, d := range detections {
					if !d.Available {
						continue
					}
					name := string(d.Provider)
					for idx, opt := range s.ProviderOptions {
						if opt != name {
							continue
						}
						s.SelectedProvider = name
						s.ProviderIdx = idx
						detectedSource = d.Source
						if d.Details != "" {
							detectedSource += " (" + d.Details + ")"
						}
						break
					}
					if s.SelectedProvider != "" {
						break
					}
				}
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

			rerenderSafe := func() { rerenderStep(tvApp, s, "provider") }

			reset := theme.TagColor

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

			// Show detection source hint when provider was auto-detected.
			if detectedSource != "" {
				info := theme.ColorTag(theme.InfoHex)
				form.AddTextView("", fmt.Sprintf("%s%s%s", info, i18n.Tf("cmd.init.wizard_detected_from", detectedSource), reset), 60, 1, true, false)
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

			styleWizardForm(form)

			views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro: wizardIntroText(
					i18n.T("cmd.init.wizard_intro_provider_title"),
					i18n.T("cmd.init.wizard_intro_provider_desc"), ""),
				Content:         form,
				ContentMaxWidth: views.ComputeFormMaxWidth(form),
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

			return updateHubConfig(*s.AppPtr, func(c *config.Config) error {
				c.LLM.DefaultProvider = s.SelectedProvider
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
			if s.ProviderSkipped {
				return []views.InfoField{{
					Label: i18n.T("cmd.init.wizard_step_provider_label"),
					Value: infoMuted(i18n.T("cmd.init.wizard_not_configured")),
				}}
			}
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
			if err := requireV2(context.Background()); err != nil {
				fields = append(fields, views.InfoField{
					Label: "Warning",
					Value: infoWarning(err.Error()),
				})
			}
			return fields
		},
		Processing: i18n.T("cmd.init.wizard_processing_credentials"),
	}
}

// prefillProjectFromCwd fills the name and the path of the first project from
// the current folder: the path is the folder, the name is the repository name
// of its git remote, else the folder name. What the user (or an existing
// project) already set is kept. It returns where the name comes from
// ("git remote", the current folder, or "" when nothing was filled).
func prefillProjectFromCwd(s *initStepState) string {
	if s.ProjectPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			s.ProjectPath = cwd
		}
	}
	if s.ProjectName != "" {
		return ""
	}
	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		if remote := strings.TrimSpace(string(out)); remote != "" {
			if name := config.RepoNameFromRemote(remote); name != "" {
				s.ProjectName = name
				return "git remote"
			}
		}
	}
	if base := filepath.Base(s.ProjectPath); s.ProjectPath != "" && base != "." && base != string(filepath.Separator) {
		s.ProjectName = base
		return i18n.T("cmd.init.wizard_detected_cwd")
	}
	return ""
}

// projectIntroText is the header of the project page: what the project is,
// that it is optional, and the project already found in the current folder.
func projectIntroText(s *initStepState) string {
	text := wizardIntroText(
		i18n.T("cmd.init.wizard_intro_project_title"),
		i18n.T("cmd.init.wizard_intro_project_desc"),
		i18n.T("cmd.init.wizard_intro_project_optional"),
	)
	if s.ExistingProject == nil {
		return text
	}
	infoColor := theme.ColorTag(theme.InfoHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n")
	fmt.Fprintf(&b, "%s%s %s%s\n", infoColor, theme.IconInfo, i18n.T("cmd.init.wizard_project_existing"), reset)
	fmt.Fprintf(&b, "%s  %s%s\n", muted,
		i18n.Tf("cmd.init.wizard_project_existing_info", s.ExistingProject.Name, s.ExistingProject.Path), reset)
	if s.ExistingProject.TeamID != nil && *s.ExistingProject.TeamID != "" {
		fmt.Fprintf(&b, "%s  %s%s\n", muted,
			i18n.Tf("cmd.init.wizard_project_existing_team", *s.ExistingProject.TeamID), reset)
	}
	return b.String()
}

// buildProjectStep creates the first project creation step: an optional page
// whose fields are filled from the current folder.
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
			reset := theme.TagColor
			infoColor := theme.ColorTag(theme.InfoHex)

			detectedProject := prefillProjectFromCwd(s)

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
					switch s.ProjectChoice {
					case "keep":
						defaultIdx = 1
					case "reconfigure":
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

					switch s.ProjectChoice {
					case "keep":
						// Keep mode: only show team attachment option.
						addTeamAttachment()
					case "reconfigure":
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

			styleWizardForm(form)

			pageResult := views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
				Intro:           projectIntroText(s),
				Content:         form,
				ContentMaxWidth: views.ComputeFormMaxWidth(form),
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
				return errors.New(i18n.T("cmd.init_steps.project_store_not_initialized"))
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
						return fmt.Errorf(i18n.T("cmd.init_steps.update_project"), err)
					}
				}
				s.ProjectID = existing.ID
				s.ProjectName = existing.Name
				s.ProjectPath = existing.Path
				s.ProjectCreated = true
				initWizardSoloSpace(ctx, s, existing)
				return nil
			}

			// "reconfigure" or new project — upsert logic.
			p := &domain.Project{
				ID:       uuid.New().String()[:8],
				Name:     s.ProjectName,
				Path:     s.ProjectPath,
				Status:   domain.ProjectStatusActive,
				Provider: s.SelectedProvider,
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
			offerBeadsClean(TryApp(), result.Path)
			initWizardSoloSpace(ctx, s, result)
			return nil
		},
		InfoFields: func() []views.InfoField {
			if s.ProjectSkipped {
				return []views.InfoField{{
					Label: i18n.T("cmd.init.wizard_step_project"),
					Value: infoMuted(i18n.T("cmd.init.wizard_not_configured")),
				}}
			}
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
