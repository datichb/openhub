package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team resolution helpers
// ─────────────────────────────────────────────────────────────────────────────

// teamEnabledForProject reports whether team features are active for a specific project.
// Pass nil as project to check hub-level only.
func teamEnabledForProject(a *app.App, project *domain.Project) bool {
	return resolvedTeamConfig(a, project).Enabled
}

// resolvedTeamConfig returns the effective TeamConfig for a project.
// Pass nil as project to get hub-level only.
func resolvedTeamConfig(a *app.App, project *domain.Project) config.ResolvedTeamConfig {
	if project != nil {
		return config.ResolveTeamForProject(a.Config, project)
	}
	// Hub-level only: use ActiveTeam() directly
	hub := a.Config.ActiveTeam()
	return config.ResolvedTeamConfig{
		Enabled:   hub.Enabled,
		TeamID:    hub.ID,
		StateRepo: hub.StateRepo,
		StatePath: hub.StatePath,
		MemberID:  hub.MemberID,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Wizard step (used in oh project add)
// ─────────────────────────────────────────────────────────────────────────────

// httpsCredState holds mutable state for the HTTPS credential wizard step.
type httpsCredState struct {
	AuthChoice string // "provide", "skip", or "public"
	Username   string // default "oauth2"
	Token      string
}

// buildHTTPSCredStep returns a WizardStep that collects HTTPS credentials
// (auth mode, username, token) and stores them via git credential approve.
// The step auto-skips when the repo URL is not HTTPS.
func buildHTTPSCredStep(ctx context.Context, repoURL *string, state *httpsCredState, extraSkipIf func() bool) views.WizardStep {
	if state.Username == "" {
		state.Username = "oauth2"
	}
	if state.AuthChoice == "" {
		state.AuthChoice = "provide"
	}
	return views.WizardStep{
		Label: i18n.T("cmd.team.init.step_credentials"),
		SkipIf: func() bool {
			if extraSkipIf != nil && extraSkipIf() {
				return true
			}
			return !teamstate.IsHTTPS(*repoURL)
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			authOptions := []string{
				i18n.T("cmd.team.init.cred_provide"),
				i18n.T("cmd.team.init.cred_skip"),
				i18n.T("cmd.team.init.cred_public"),
			}
			form.AddDropDown(i18n.T("cmd.team.init.cred_auth_mode"), authOptions, 0,
				func(_ string, idx int) {
					switch idx {
					case 0:
						state.AuthChoice = "provide"
					case 1:
						state.AuthChoice = "skip"
					case 2:
						state.AuthChoice = "public"
					}
				})
			form.AddInputField("Username", state.Username, 0, nil,
				func(text string) { state.Username = text })
			form.AddPasswordField("Token", state.Token, 0, '*',
				func(text string) { state.Token = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() {
				if state.AuthChoice == "provide" && state.Token == "" {
					return // block submit without token
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			if state.AuthChoice == "provide" && state.Token != "" {
				return teamstate.ConfigureCredential(ctx, *repoURL, state.Username, state.Token)
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Auth", Value: "configured"}}
		},
	}
}

// initWizardTeamState holds mutable state shared across the team wizard steps.
// It is allocated once by buildInitWizardTeamSteps and captured by closures.
type initWizardTeamState struct {
	Skipped       bool   // user chose not to configure team
	Mode          string // "init" or "rejoin" (empty = not chosen yet)
	Repo          string // team-state repo URL
	MemberID      string // member identifier
	DisplayName   string // optional display name
	Configured    bool   // true after successful teamInitCore or teamRejoinCore
	TeamID        string // derived team ID (after init or rejoin)
	attachProject bool   // true if user wants to attach the project to this team
	// Rejoin-specific state
	Members       []teamstate.Member // fetched members for rejoin flow
	// HTTPS credential state (shared by init and rejoin)
	CredUsername   string // HTTPS username (default "oauth2")
	CredToken      string // HTTPS token/password
	CredAuthChoice string // "provide", "skip", or "public"
}

// buildInitWizardTeamSteps returns the two WizardSteps for the "Team" group
// used in the hub initialization wizards (first-run inline + oh init standalone).
//
// The returned steps are:
//  1. Form — repo URL, member ID, display name
//  2. Processing — clone/pull, init structure, register member, write hub.toml
//
// The caller is responsible for prepending an intro step (via buildIntroStep)
// that sets state.Skipped. Both steps use SkipIf → state.Skipped.
//
// After successful completion, state.Configured is true and the app should be
// reloaded (config.Reset + ReloadApp) so subsequent steps can see the team.
func buildInitWizardTeamSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	return []views.WizardStep{
		// ── Team form: repo + identity ──
		{
			Label: i18n.T("cmd.init.wizard_step_team_form"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "init"
			},
			Validate: func() string {
				if state.Repo == "" {
					return i18n.T("cmd.init.wizard_team_repo_required")
				}
				if state.MemberID == "" {
					return i18n.T("cmd.init.wizard_team_member_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_repo"), state.Repo, 60, nil,
					func(t string) { state.Repo = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_member_id"), state.MemberID, 30, nil,
					func(t string) { state.MemberID = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_display_name"), state.DisplayName, 40, nil,
					func(t string) { state.DisplayName = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Repo", Value: state.Repo},
					{Label: "Member", Value: state.MemberID},
				}
			},
		},
		// ── Team processing: clone + init + register ──
		{
			Label: i18n.T("cmd.init.wizard_step_team_sync"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "init"
			},
			Processing: i18n.T("cmd.init.wizard_processing_team"),
			OnDone: func() error {
				displayName := state.DisplayName
				if displayName == "" {
					displayName = state.MemberID
				}
				err := teamInitCore(context.Background(), *a, teamInitParams{
					StateRepo:   state.Repo,
					MemberID:    state.MemberID,
					DisplayName: displayName,
					Role:        "dev",
				})
				if err != nil {
					return err
				}
				state.Configured = true
				state.TeamID = config.RepoNameFromRemote(state.Repo)

				// Reload app so subsequent steps see the team
				config.Reset()
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					*a = newApp
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if state.Configured {
					return []views.InfoField{
						{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_team_connected")},
					}
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_team_skipped")},
				}
			},
		},
	}
}

// buildInitWizardHTTPSCredSteps returns two WizardSteps for HTTPS credential
// configuration. They are used by the rejoin flow when the repo URL uses HTTPS.
//
// The returned steps are:
//  1. Auth mode selection — DropDown (provide/skip/public). Always shown for HTTPS.
//  2. Credentials — Username + Token. Skipped when auth mode is "skip" or "public".
//
// Both steps are conditionally skipped if the repo is not HTTPS.
func buildInitWizardHTTPSCredSteps(state *initWizardTeamState, requiredMode string) []views.WizardStep {
	// Initialize defaults
	if state.CredUsername == "" {
		state.CredUsername = "oauth2"
	}
	if state.CredAuthChoice == "" {
		state.CredAuthChoice = "provide"
	}

	return []views.WizardStep{
		// ── Step 1: Auth mode selection ──
		{
			Label: i18n.T("cmd.init.wizard_step_team_cred_mode"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != requiredMode || !teamstate.IsHTTPS(state.Repo)
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				authOptions := []string{
					i18n.T("cmd.init.wizard_team_cred_provide"),
					i18n.T("cmd.init.wizard_team_cred_skip"),
					i18n.T("cmd.init.wizard_team_cred_public"),
				}
				form.AddDropDown(i18n.T("cmd.init.wizard_team_cred_auth_mode"), authOptions, 0,
					func(_ string, idx int) {
						switch idx {
						case 0:
							state.CredAuthChoice = "provide"
						case 1:
							state.CredAuthChoice = "skip"
						case 2:
							state.CredAuthChoice = "public"
						}
					})
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth", Value: state.CredAuthChoice}}
			},
		},
		// ── Step 2: Username + Token (skipped when not "provide") ──
		{
			Label: i18n.T("cmd.init.wizard_step_team_creds"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != requiredMode || !teamstate.IsHTTPS(state.Repo) || state.CredAuthChoice != "provide"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Username", state.CredUsername, 0, nil,
					func(text string) { state.CredUsername = text })
				form.AddPasswordField("Token", state.CredToken, 0, '*',
					func(text string) { state.CredToken = text })
				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					if state.CredToken == "" {
						return
					}
					onDone()
				})
				return form
			},
			OnDone: func() error {
				if state.CredToken != "" {
					return teamstate.ConfigureCredential(
						context.Background(), state.Repo, state.CredUsername, state.CredToken,
					)
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Auth", Value: i18n.T("cmd.init.wizard_team_cred_configured")}}
			},
		},
	}
}

// buildInitWizardRejoinSteps returns the WizardSteps for the "rejoin" flow
// used in the hub initialization wizards (first-run inline + oh init standalone).
//
// The returned steps are:
//  1. Repo URL — collects the team-state repo and fetches member list
//  2. HTTPS Auth mode — conditional on HTTPS repo URL (provide/skip/public)
//  3. HTTPS Credentials — conditional on auth mode == "provide"
//  4. Member selection — SectionedList to pick the existing member
//  5. Validation — validates GitLab identity, writes hub.toml, retro-tags sessions
//
// All steps have SkipIf → state.Skipped || state.Mode != "rejoin".
// After successful completion, state.Configured is true and the app should be
// reloaded so subsequent steps can see the team.
func buildInitWizardRejoinSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	steps := []views.WizardStep{
		// ── Rejoin step 1: Repo URL + fetch members ──
		{
			Label: i18n.T("cmd.init.wizard_step_rejoin_repo"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "rejoin"
			},
			Validate: func() string {
				if strings.TrimSpace(state.Repo) == "" {
					return i18n.T("cmd.init.wizard_team_repo_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_repo"), state.Repo, 60, nil,
					func(t string) { state.Repo = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				ctx := context.Background()
				_, members, _, err := listTeamMembers(ctx, state.Repo, "")
				if err != nil {
					return err
				}
				if len(members) == 0 {
					return fmt.Errorf("%s", i18n.T("cmd.init.wizard_rejoin_no_members"))
				}
				state.Members = members
				return nil
			},
			Processing: i18n.T("cmd.init.wizard_processing_rejoin_clone"),
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Repo", Value: state.Repo}}
			},
		},
	}

	// ── Rejoin steps 2-3: HTTPS Credentials (auth mode + username/token) ──
	steps = append(steps, buildInitWizardHTTPSCredSteps(state, "rejoin")...)

	// ── Rejoin step 4: Member selection ──
	steps = append(steps, views.WizardStep{
			Label: i18n.T("cmd.init.wizard_step_rejoin_member"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "rejoin"
			},
			Required: true,
			CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
				list := widgets.NewSectionedList()
				list.SetApp(tvApp)

				var items []widgets.SectionItem
				items = append(items, widgets.SectionItem{
					MainText: i18n.T("cmd.init.wizard_rejoin_members_header"),
					IsHeader: true,
				})
				for _, m := range state.Members {
					label := fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)
					secondary := ""
					if m.GitLabUsername != "" {
						secondary = fmt.Sprintf("gitlab: %s", m.GitLabUsername)
					}
					if m.Role != "" {
						if secondary != "" {
							secondary += " — "
						}
						secondary += m.Role
					}
					items = append(items, widgets.SectionItem{
						MainText:      label,
						SecondaryText: secondary,
						Reference:     m.ID,
					})
				}
				list.SetItems(items)

				list.SetItemSelectedFunc(func(_ int, item widgets.SectionItem) {
					if id, ok := item.Reference.(string); ok {
						state.MemberID = id
					}
					onDone()
				})

				container.AddItem(list, 0, 1, true)
				tvApp.SetFocus(list)
			},
			InfoFields: func() []views.InfoField {
				if state.MemberID == "" {
					return nil
				}
				for _, m := range state.Members {
					if m.ID == state.MemberID {
						return []views.InfoField{
							{Label: i18n.T("cmd.init.wizard_rejoin_member_label"), Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)},
						}
					}
				}
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_rejoin_member_label"), Value: state.MemberID}}
			},
		},
	)

	// ── Rejoin step 5: Validate identity + write config ──
	steps = append(steps, views.WizardStep{
			Label:      i18n.T("cmd.init.wizard_step_rejoin_validate"),
			Processing: i18n.T("cmd.init.wizard_processing_rejoin_validate"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "rejoin"
			},
			OnDone: func() error {
				ctx := context.Background()
				result, err := teamRejoinCore(ctx, *a, teamRejoinParams{
					StateRepo: state.Repo,
					MemberID:  state.MemberID,
				})
				if err != nil {
					return err
				}

				state.Configured = true
				state.TeamID = result.TeamID

				// Retro-tag sessions synchronously (wizard shows spinner)
				_, _ = retroTagSessions(ctx, state.MemberID)

				// Reload app so subsequent steps see the team
				config.Reset()
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					*a = newApp
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if state.Configured {
					return []views.InfoField{
						{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_rejoin_reconnected")},
					}
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_team_skipped")},
				}
			},
		},
	)

	return steps
}

// buildTeamModeIntroStep creates a CustomView step that serves as the team
// group gateway in the hub init wizard. It presents three choices:
//   - Create a new team (sets state.Mode = "init")
//   - Rejoin an existing team (sets state.Mode = "rejoin")
//   - Skip (sets state.Skipped = true)
//
// This replaces the simpler Continue/Skip pattern used by buildIntroStep for
// other wizard groups.
func buildTeamModeIntroStep(state *initWizardTeamState) views.WizardStep {
	return views.WizardStep{
		Label:         i18n.T("cmd.init.wizard_step_team"),
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagColor

			title := i18n.T("cmd.init.wizard_intro_team_title")
			desc := i18n.T("cmd.init.wizard_intro_team_desc")
			note := i18n.T("cmd.init.wizard_intro_team_optional")
			prereqs := i18n.T("cmd.init.wizard_team_prereq")

			var b strings.Builder
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s%s\n\n", accent, title, reset)
			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "%s%s%s\n", secondary, line, reset)
			}
			b.WriteString("\n")

			// Prerequisites
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

			// Note
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

			// Button form with 3 choices
			buttonForm := views.NewStyledButtonForm()

			// Button 1: Create a new team
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_init")+"  ", func() {
				state.Mode = "init"
				state.Skipped = false
				onDone()
			})

			// Button 2: Rejoin an existing team
			buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_rejoin")+"  ", func() {
				state.Mode = "rejoin"
				state.Skipped = false
				onDone()
			})

			// Button 3: Skip (with double-click confirmation)
			skipConfirmed := false
			buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
				if !skipConfirmed {
					skipConfirmed = true
					if btn := buttonForm.GetButton(2); btn != nil {
						btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
					}
					return
				}
				state.Skipped = true
				state.Mode = ""
				onDone()
			})

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
			badgeView := views.BuildStepBadge(i18n.T("cmd.init.wizard_step_team"))
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

// buildProjectTeamStep returns a WizardStep that collects team configuration
// for a project during project creation or reconfiguration.
//
// The step sets out to:
//   - &activeTeamID → attach the project to the hub's active team
//   - nil           → no team for this project
func buildProjectTeamStep(a *app.App, out **string) views.WizardStep {
	hubTeam := a.Config.ActiveTeam()

	// Local state for the step
	var attachToTeam bool

	// If hub has a team, default to attaching
	if hubTeam.Enabled && hubTeam.ID != "" {
		attachToTeam = true
	}

	return views.WizardStep{
		Label: i18n.T("form.project.team_label"),
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			if hubTeam.Enabled && hubTeam.ID != "" {
				hubSummary := i18n.Tf("form.project.team_hub_summary", hubTeam.StateRepo, hubTeam.MemberID)
				form.AddTextView(i18n.T("form.project.team_hub_label"), hubSummary, 0, 1, false, false)
				modeOptions := []string{
					i18n.Tf("form.project.team_use_hub", hubTeam.MemberID),
					i18n.T("form.project.team_no_team"),
				}
				form.AddDropDown(i18n.T("form.project.team_dropdown"), modeOptions, 0, func(_ string, idx int) {
					attachToTeam = idx == 0
				})
			} else {
				form.AddTextView(i18n.T("form.project.team_hub_label"), i18n.T("form.project.team_none_configured"), 0, 1, false, false)
			}

			form.AddButton("Next", func() { onDone() })
			return form
		},
		OnDone: func() error {
			if attachToTeam && hubTeam.ID != "" {
				id := hubTeam.ID
				*out = &id
			} else {
				*out = nil
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			if attachToTeam && hubTeam.ID != "" {
				return []views.InfoField{{Label: i18n.T("form.project.team_label"), Value: hubTeam.ID}}
			}
			return []views.InfoField{{Label: i18n.T("form.project.team_label"), Value: i18n.T("form.project.team_value_none")}}
		},
	}
}
