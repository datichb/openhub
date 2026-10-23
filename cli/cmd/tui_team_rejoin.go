package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team Rejoin action (reconnect to existing team after reinstall)
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamRejoin() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// ── Shared state (captured by step closures) ────────────────────
	var (
		stateRepo        string
		gitlabURL        string // GitLab of the repository when its remote is an SSH alias
		statePath        string
		members          []teamstate.Member
		memberID         string
		gitlabToken      string
		tokenChoice      string // "reuse", "new", or "skip"
		tokenChoiceIdx   int    // current dropdown index (0 = placeholder)
		steps            []views.WizardStep
		identityMismatch *identityMismatchError
		mismatchPAT      string
		mismatchChoice   string
		rejoinCredState  = &httpsCredState{Username: "oauth2", AuthChoice: "provide"}
	)

	// ── Step 0: Repository URL ──────────────────────────────────────
	repoStep := views.WizardStep{
		Label:    i18n.T("cmd.team.init.step_repo"),
		Required: true,
		Validate: func() string {
			if strings.TrimSpace(stateRepo) == "" {
				return i18n.T("cmd.team.init.validate.repo_required")
			}
			return teamGitLabURLProblem(gitlabURL)
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.repo_url_title"),
				stateRepo, 0, nil,
				func(text string) { stateRepo = text })
			form.AddInputField(
				i18n.T("cmd.init.wizard_team_gitlab_url"),
				gitlabURL, 0, nil,
				func(text string) { gitlabURL = strings.TrimSpace(text) })
			form.AddTextView("", i18n.T("cmd.init.wizard_team_gitlab_hint_short"), 60, 2, true, false)
			form.AddButton(i18n.T("tui.team.rejoin.next"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			statePath = config.TeamStatePath(stateRepo)
			var err error
			_, members, _, err = listTeamMembers(ctx, stateRepo, statePath)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				return errors.New(i18n.T("tui.team.rejoin.no_members"))
			}
			rejoinCredState.Probe(ctx, stateRepo)
			return nil
		},
		Processing: i18n.T("cmd.team.init.processing_repo"),
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Repo", Value: stateRepo}}
		},
	}

	// ── Step 0b: HTTPS Credentials (conditional) ────────────────────
	httpsCredStep := buildHTTPSCredStep(ctx, &stateRepo, rejoinCredState, nil, a.Secrets)

	// ── Step 1: Member selection ────────────────────────────────────
	memberStep := views.WizardStep{
		Label:    i18n.T("cmd.team.rejoin.select_member"),
		Required: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			list := widgets.NewSectionedList()
			list.SetApp(tvApp)

			var items []widgets.SectionItem
			items = append(items, widgets.SectionItem{
				MainText: i18n.T("tui.team.rejoin.members_header"),
				IsHeader: true,
			})
			for _, m := range members {
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
					memberID = id
				}
				onDone()
			})

			container.AddItem(list, 0, 1, true)
			tvApp.SetFocus(list)
		},
		InfoFields: func() []views.InfoField {
			if memberID == "" {
				return nil
			}
			for _, m := range members {
				if m.ID == memberID {
					return []views.InfoField{
						{Label: i18n.T("tui.team.rejoin.member_label"), Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)},
					}
				}
			}
			return []views.InfoField{{Label: i18n.T("tui.team.rejoin.member_label"), Value: memberID}}
		},
	}

	// ── Step 1b: GitLab token for SSH repos (conditional) ──────────
	// When HTTPS, the token is already stored by httpsCredStep.
	gitlabTokenStep := views.WizardStep{
		ID:    "rejoin_gitlab_token",
		Label: i18n.T("cmd.init.wizard_step_rejoin_gitlab_token"),
		SkipIf: func() bool {
			glUser := ""
			for _, m := range members {
				if m.ID == memberID {
					glUser = m.GitLabUsername
					break
				}
			}
			// the HTTPS step stores the token, unless it was skipped
			return glUser == "" || (teamstate.IsHTTPS(stateRepo) && !rejoinCredState.Open)
		},
		Validate: func() string {
			if tokenChoiceIdx == 0 {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if tokenChoice == "new" && strings.TrimSpace(gitlabToken) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			teamID := config.RepoNameFromRemote(stateRepo)
			source := gitlabTokenSourceAt(ctx, a, teamID, stateRepo, gitlabURL)

			options := []string{i18n.T("cmd.init.wizard_select_placeholder")}
			optionKeys := []string{"placeholder"}
			if source != "" {
				options = append(options, i18n.Tf("cmd.init.wizard_rejoin_token_reuse", source))
				optionKeys = append(optionKeys, "reuse")
			}
			options = append(options, i18n.T("cmd.init.wizard_rejoin_token_new"))
			optionKeys = append(optionKeys, "new")
			options = append(options, i18n.T("cmd.init.wizard_rejoin_token_skip"))
			optionKeys = append(optionKeys, "skip")

			form := tview.NewForm()
			form.AddDropDown(
				i18n.T("cmd.init.wizard_rejoin_token_choice"),
				options, tokenChoiceIdx,
				func(_ string, idx int) {
					if tokenChoiceIdx == idx {
						return
					}
					if idx >= 0 && idx < len(optionKeys) {
						tokenChoice = optionKeys[idx]
					}
					tokenChoiceIdx = idx
				},
			)
			form.AddTextView("", i18n.T("cmd.init.wizard_rejoin_token_new_hint"), 60, 2, true, false)
			form.AddPasswordField(
				i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
				gitlabToken, 0, '*',
				func(t string) { gitlabToken = t },
			)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			if tokenChoice == "new" && gitlabToken != "" && a.Secrets != nil {
				teamID := config.RepoNameFromRemote(stateRepo)
				return a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), gitlabToken)
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			var status string
			switch tokenChoice {
			case "reuse":
				status = i18n.T("cmd.init.wizard_rejoin_token_reuse_short")
			case "new":
				status = i18n.T("tui.team.wizard.token_stored")
			default:
				status = i18n.T("cmd.init.wizard_team_skipped")
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.tui_team_rejoin.gitlab_token"), Value: status},
			}
		},
	}

	// ── Step 2: GitLab identity validation + config write ───────────
	validateStep := views.WizardStep{
		Label:      i18n.T("cmd.team.rejoin.validating"),
		Processing: i18n.T("cmd.team.rejoin.validating"),
		OnDone: func() error {
			result, err := teamRejoinCore(ctx, a, teamRejoinParams{
				StateRepo: stateRepo,
				StatePath: statePath,
				MemberID:  memberID,
				GitLabURL: gitlabURL,
			})
			if err != nil {
				return err
			}

			// Store identity mismatch for the next conditional step.
			identityMismatch = result.IdentityMismatch

			// Retro-tag sessions in background
			safego.Go(func() {
				bgCtx := context.Background()
				count, err := retroTagSessions(bgCtx, memberID)
				if err == nil && count > 0 && tuiShell != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(
							i18n.Tf("cmd.team.rejoin.retro_tagged", count),
							shell.ToastSuccess,
						)
					})
				}
			})

			// Show result summary via toast
			if tuiShell != nil {
				msg := i18n.Tf("cmd.team.rejoin.success", result.Member.DisplayName, result.Member.ID, result.TeamName)
				if result.EventCount > 0 {
					msg += " — " + i18n.Tf("cmd.team.rejoin.events_found", result.EventCount)
				}
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(msg, shell.ToastSuccess)
				})
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: i18n.T("tui.team.rejoin.status_label"), Value: theme.SuccessStyle.Render(i18n.T("tui.team.rejoin.reconnected"))},
			}
		},
	}

	// ── Step 2b: Identity mismatch recovery (conditional) ──────────
	identityMismatchStep := views.WizardStep{
		ID:    "rejoin_identity_mismatch",
		Label: i18n.T("cmd.init.wizard_step_identity_mismatch"),
		SkipIf: func() bool {
			return identityMismatch == nil
		},
		Validate: func() string {
			if mismatchChoice == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if mismatchChoice == "verify" && strings.TrimSpace(mismatchPAT) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			var msg string
			if identityMismatch.IsBot {
				msg = i18n.Tf("cmd.init.wizard_identity_bot_detected", identityMismatch.AuthenticatedAs)
			} else {
				msg = i18n.Tf("cmd.init.wizard_identity_user_mismatch",
					identityMismatch.AuthenticatedAs,
					identityMismatch.MemberID,
					identityMismatch.ExpectedUser,
				)
			}
			form.AddTextView("", msg, 60, 3, true, false)

			options := []string{
				i18n.T("cmd.init.wizard_select_placeholder"),
				i18n.T("cmd.init.wizard_identity_verify"),
				i18n.T("cmd.init.wizard_identity_skip"),
			}
			form.AddDropDown(
				i18n.T("cmd.init.wizard_step_identity_mismatch"),
				options, 0,
				func(_ string, idx int) {
					switch idx {
					case 1:
						mismatchChoice = "verify"
					case 2:
						mismatchChoice = "skip"
					default:
						mismatchChoice = ""
					}
				},
			)

			form.AddTextView("", i18n.T("cmd.init.wizard_identity_pat_hint"), 60, 2, true, false)
			form.AddPasswordField(
				i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
				mismatchPAT, 0, '*',
				func(t string) { mismatchPAT = t },
			)

			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			if mismatchChoice == "skip" {
				identityMismatch = nil
				return nil
			}
			// "verify" — store the PAT and re-validate
			if mismatchPAT != "" && a.Secrets != nil {
				teamID := config.RepoNameFromRemote(stateRepo)
				if err := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), mismatchPAT); err != nil {
					return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.storing_token"), err)
				}
			}
			sp := statePath
			if sp == "" {
				sp = config.TeamStatePath(stateRepo)
			}
			repo := teamstate.NewRepo(stateRepo, sp)
			member, err := repo.GetMember(memberID)
			if err != nil {
				return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.member_lookup"), err)
			}
			if err := validateGitLabIdentity(ctx, a, repo, member); err != nil {
				return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.identity_verification"), err)
			}
			identityMismatch = nil
			return nil
		},
		InfoFields: func() []views.InfoField {
			if mismatchChoice == "skip" {
				return []views.InfoField{{Label: i18n.T("tui.team.wizard.identity"), Value: i18n.T("cmd.init.wizard_identity_skip")}}
			}
			return []views.InfoField{{Label: i18n.T("tui.team.wizard.identity"), Value: i18n.T("tui.team.wizard.identity_verified")}}
		},
	}

	// ── Step 3: Tracker setup (conditional) ─────────────────────────
	var launchDiscoveryAfterRejoin bool
	var tuiTrackerChoice string
	trackerStepRejoin := views.WizardStep{
		ID:    "rejoin_tracker_setup",
		Label: i18n.T("cmd.init.wizard_step_tracker_setup"),
		SkipIf: func() bool {
			return stateRepo == ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			sp := statePath
			if sp == "" {
				sp = config.TeamStatePath(stateRepo)
			}
			repo := teamstate.NewRepo(stateRepo, sp)
			trackerConfigured := false
			if teamCfg, err := repo.LoadConfig(); err == nil && teamCfg.Tracker.Type != "" {
				trackerConfigured = true
				hint := i18n.T("cmd.init.wizard_tracker_configured") + "\n" +
					i18n.Tf("cmd.init.wizard_tracker_info",
						teamCfg.Tracker.Type,
						teamCfg.Tracker.TrackerURL,
						teamCfg.Tracker.TrackerProject,
					)
				form.AddTextView("", hint, 60, 3, true, false)
			} else {
				form.AddTextView("", i18n.T("cmd.init.wizard_tracker_not_configured"), 60, 2, true, false)
			}

			options := []string{i18n.T("cmd.init.wizard_select_placeholder")}
			if trackerConfigured {
				options = append(options,
					i18n.T("cmd.init.wizard_tracker_keep_existing"),
					i18n.T("cmd.init.wizard_tracker_configure_now"),
					i18n.T("cmd.init.wizard_tracker_configure_later"),
				)
				form.AddDropDown(i18n.T("cmd.init.wizard_step_tracker_setup"), options, 0, func(_ string, idx int) {
					switch idx {
					case 1:
						tuiTrackerChoice = "keep"
						launchDiscoveryAfterRejoin = false
					case 2:
						tuiTrackerChoice = "reconfigure"
						launchDiscoveryAfterRejoin = true
					case 3:
						tuiTrackerChoice = "later"
						launchDiscoveryAfterRejoin = false
					default:
						tuiTrackerChoice = ""
						launchDiscoveryAfterRejoin = false
					}
				})
			} else {
				options = append(options,
					i18n.T("cmd.init.wizard_tracker_configure_now"),
					i18n.T("cmd.init.wizard_tracker_configure_later"),
				)
				form.AddDropDown(i18n.T("cmd.init.wizard_step_tracker_setup"), options, 0, func(_ string, idx int) {
					switch idx {
					case 1:
						tuiTrackerChoice = "reconfigure"
						launchDiscoveryAfterRejoin = true
					case 2:
						tuiTrackerChoice = "later"
						launchDiscoveryAfterRejoin = false
					default:
						tuiTrackerChoice = ""
						launchDiscoveryAfterRejoin = false
					}
				})
			}
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error { return nil },
		InfoFields: func() []views.InfoField {
			val := i18n.T("cmd.init.wizard_tracker_configure_later")
			if launchDiscoveryAfterRejoin {
				val = i18n.T("cmd.init.wizard_tracker_configure_now")
			} else if tuiTrackerChoice == "keep" {
				val = i18n.T("cmd.init.wizard_tracker_keep_existing")
			}
			return []views.InfoField{{Label: "Tracker", Value: val}}
		},
	}

	// ── Step 3b: Tracker token (conditional on "keep" + no token for the server) ──
	trackerTokenStepRejoin := buildTrackerTokenStep(trackerTokenStep{
		ID:        "rejoin_tracker_token",
		Label:     i18n.Tf("cmd.init.wizard_tracker_token_label", ""),
		InfoLabel: i18n.T("cmd.tui_team_rejoin.tracker_token"),
		App:       func() *app.App { return a },
		Ctx:       func() context.Context { return ctx },
		Repo:      func() string { return stateRepo },
		StatePath: func() string { return statePath },
		Keep:      func() bool { return stateRepo != "" && tuiTrackerChoice == "keep" },
	})

	steps = []views.WizardStep{
		repoStep,
		httpsCredStep,
		memberStep,
		gitlabTokenStep,
		validateStep,
		identityMismatchStep,
		trackerStepRejoin,
		trackerTokenStepRejoin,
	}

	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.team.rejoin",
		Title: i18n.T("tui.team.rejoin"),
		Steps: steps,
		OnComplete: func(completed bool, err error) {
			if completed && err == nil && launchDiscoveryAfterRejoin {
				go func() {
					time.Sleep(200 * time.Millisecond)
					if tuiShell != nil {
						tuiShell.App().QueueUpdateDraw(func() {
							actionTrackerDiscovery()
						})
					}
				}()
			}
		},
	})

	tuiShell.PushView(wizard)
}
