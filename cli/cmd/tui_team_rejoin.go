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
	tuiShell.PushView(views.NewInlineWizardView(buildRejoinWizard(tuiShell.Context(), MustApp())))
}

// rejoinWizard is the state the steps of the rejoin share.
type rejoinWizard struct {
	ctx context.Context
	a   *app.App

	stateRepo        string
	gitlabURL        string // GitLab of the repository when its remote is an SSH alias
	statePath        string
	members          []teamstate.Member
	memberID         string
	gitlabToken      string
	tokenChoice      string // "reuse", "new", or "skip"
	tokenChoiceIdx   int    // current dropdown index (0 = placeholder)
	identityMismatch *identityMismatchError
	mismatchPAT      string
	mismatchChoice   string
	cred             *httpsCredState
	trackerWanted    bool   // the tracker is on another server than the team repository
	trackerName      string // typed in the tracker name step

	launchDiscovery bool   // « configure now » was chosen for the tracker
	trackerChoice   string // "keep", "reconfigure", "later" or ""
}

// buildRejoinWizard is the wizard of the rejoin: its steps and what it does
// when it ends.
func buildRejoinWizard(ctx context.Context, a *app.App) views.InlineWizardConfig {
	w := &rejoinWizard{ctx: ctx, a: a, cred: &httpsCredState{Username: "oauth2", AuthChoice: "provide"}}
	return views.InlineWizardConfig{
		ID:    "wizard.team.rejoin",
		Title: i18n.T("tui.team.rejoin"),
		Steps: []views.WizardStep{
			w.repoStep(),
			buildHTTPSCredStep(ctx, &w.stateRepo, w.cred, nil, a.Secrets),
			w.memberStep(),
			w.trackerNameStep(),
			w.gitlabTokenStep(),
			w.validateStep(),
			w.identityMismatchStep(),
			w.trackerSetupStep(),
			w.trackerTokenStep(),
		},
		OnComplete: w.onComplete,
	}
}

// onComplete launches the tracker discovery when « configure now » was chosen.
func (w *rejoinWizard) onComplete(completed bool, err error) {
	if completed && err == nil && w.launchDiscovery {
		safego.Go(func() {
			time.Sleep(200 * time.Millisecond)
			if tuiShell != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					actionTrackerDiscovery()
				})
			}
		})
	}
}

// repoStep asks for the repository of the team-state (and the GitLab of an
// SSH alias), then clones it and lists its members.
func (w *rejoinWizard) repoStep() views.WizardStep {
	ctx := w.ctx
	return views.WizardStep{
		Label:    i18n.T("cmd.team.init.step_repo"),
		Required: true,
		Validate: func() string {
			if strings.TrimSpace(w.stateRepo) == "" {
				return i18n.T("cmd.team.init.validate.repo_required")
			}
			return teamGitLabURLProblem(w.gitlabURL)
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.repo_url_title"),
				w.stateRepo, 0, nil,
				func(text string) { w.stateRepo = text })
			form.AddInputField(
				i18n.T("cmd.init.wizard_team_gitlab_url"),
				w.gitlabURL, 0, nil,
				func(text string) { w.gitlabURL = strings.TrimSpace(text) })
			form.AddTextView("", i18n.T("cmd.init.wizard_team_gitlab_hint_short"), 60, 2, true, false)
			form.AddButton(i18n.T("tui.team.rejoin.next"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			w.statePath = config.TeamStatePath(w.stateRepo)
			var (
				handle *teamstate.Repo
				err    error
			)
			handle, w.members, _, err = listTeamMembers(ctx, w.stateRepo, w.statePath)
			if err != nil {
				return err
			}
			w.trackerWanted = trackerUsernameWanted(handle, w.stateRepo, w.gitlabURL)
			if len(w.members) == 0 {
				return errors.New(i18n.T("tui.team.rejoin.no_members"))
			}
			w.cred.Probe(ctx, w.stateRepo)
			return nil
		},
		Processing: i18n.T("cmd.team.init.processing_repo"),
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Repo", Value: w.stateRepo}}
		},
	}
}

// memberStep picks the member to reconnect as.
func (w *rejoinWizard) memberStep() views.WizardStep {
	return views.WizardStep{
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
			for _, m := range w.members {
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
					w.memberID = id
				}
				onDone()
			})

			container.AddItem(list, 0, 1, true)
			tvApp.SetFocus(list)
		},
		InfoFields: func() []views.InfoField {
			if w.memberID == "" {
				return nil
			}
			for _, m := range w.members {
				if m.ID == w.memberID {
					return []views.InfoField{
						{Label: i18n.T("tui.team.rejoin.member_label"), Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)},
					}
				}
			}
			return []views.InfoField{{Label: i18n.T("tui.team.rejoin.member_label"), Value: w.memberID}}
		},
	}
}

// trackerNameStep asks the name of the member on the tracker. It is asked
// when the tracker is on another server than the team repository and the
// profile has none: without it no issue of the tracker is attributed to the
// member.
func (w *rejoinWizard) trackerNameStep() views.WizardStep {
	return views.WizardStep{
		ID:    "rejoin_tracker_username",
		Label: i18n.T("tui.team.rejoin.tracker_step"),
		SkipIf: func() bool {
			return !w.trackerWanted || memberTrackerUsername(w.members, w.memberID) != ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddTextView("", i18n.T("cmd.team.rejoin.tracker_hint"), 60, 3, true, false)
			form.AddInputField(i18n.T("tui.team.rejoin.tracker_label"), w.trackerName, 0, nil,
				func(text string) { w.trackerName = strings.TrimSpace(text) })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("tui.team.rejoin.tracker_step"), Value: firstNonEmpty(w.trackerName, i18n.T("tui.team.rejoin.tracker_none"))}}
		},
	}
}

// skipTrackerName tells whether the tracker name step is left out.
func (w *rejoinWizard) skipTrackerName() bool {
	return !w.trackerWanted || memberTrackerUsername(w.members, w.memberID) != ""
}

// gitlabTokenStep asks what to do with the GitLab token of an SSH repository
// (with HTTPS the credentials step already stored it).
func (w *rejoinWizard) gitlabTokenStep() views.WizardStep {
	return views.WizardStep{
		ID:    "rejoin_gitlab_token",
		Label: i18n.T("cmd.init.wizard_step_rejoin_gitlab_token"),
		SkipIf: func() bool {
			glUser := ""
			for _, m := range w.members {
				if m.ID == w.memberID {
					glUser = m.GitLabUsername
					break
				}
			}
			// the HTTPS step stores the token, unless it was skipped
			return glUser == "" || (teamstate.IsHTTPS(w.stateRepo) && !w.cred.Open)
		},
		Validate: func() string {
			if w.tokenChoiceIdx == 0 {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if w.tokenChoice == "new" && strings.TrimSpace(w.gitlabToken) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		Form:   w.tokenForm,
		OnDone: w.storeToken,
		InfoFields: func() []views.InfoField {
			var status string
			switch w.tokenChoice {
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
}

// validateStep rejoins the team as the member and checks the GitLab identity.
func (w *rejoinWizard) validateStep() views.WizardStep {
	a, ctx := w.a, w.ctx
	return views.WizardStep{
		Label:      i18n.T("cmd.team.rejoin.validating"),
		Processing: i18n.T("cmd.team.rejoin.validating"),
		OnDone: func() error {
			params := teamRejoinParams{
				StateRepo: w.stateRepo,
				StatePath: w.statePath,
				MemberID:  w.memberID,
				GitLabURL: w.gitlabURL,
			}
			if !w.skipTrackerName() && w.trackerName != "" {
				params.TrackerUsername = &w.trackerName
			}
			result, err := teamRejoinCore(ctx, a, params)
			if err != nil {
				return err
			}
			if result.ProfileErr != nil && tuiShell != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.Tf("cmd.team.rejoin.tracker_failed", result.ProfileErr), shell.ToastError)
				})
			}

			// Store identity mismatch for the next conditional step.
			w.identityMismatch = result.IdentityMismatch

			// Retro-tag sessions in background
			safego.Go(func() {
				bgCtx := context.Background()
				count, err := retroTagSessions(bgCtx, w.memberID)
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
}

// identityMismatchStep recovers from an identity that is not the one of the member.
func (w *rejoinWizard) identityMismatchStep() views.WizardStep {
	return views.WizardStep{
		ID:    "rejoin_identity_mismatch",
		Label: i18n.T("cmd.init.wizard_step_identity_mismatch"),
		SkipIf: func() bool {
			return w.identityMismatch == nil
		},
		Validate: func() string {
			if w.mismatchChoice == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if w.mismatchChoice == "verify" && strings.TrimSpace(w.mismatchPAT) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		Form:   w.mismatchForm,
		OnDone: w.recoverIdentity,
		InfoFields: func() []views.InfoField {
			if w.mismatchChoice == "skip" {
				return []views.InfoField{{Label: i18n.T("tui.team.wizard.identity"), Value: i18n.T("cmd.init.wizard_identity_skip")}}
			}
			return []views.InfoField{{Label: i18n.T("tui.team.wizard.identity"), Value: i18n.T("tui.team.wizard.identity_verified")}}
		},
	}
}

// trackerSetupStep offers to keep, reconfigure or leave the tracker of the team.
func (w *rejoinWizard) trackerSetupStep() views.WizardStep {
	return views.WizardStep{
		ID:    "rejoin_tracker_setup",
		Label: i18n.T("cmd.init.wizard_step_tracker_setup"),
		SkipIf: func() bool {
			return w.stateRepo == ""
		},
		Form:   w.trackerSetupForm,
		OnDone: func() error { return nil },
		InfoFields: func() []views.InfoField {
			val := i18n.T("cmd.init.wizard_tracker_configure_later")
			if w.launchDiscovery {
				val = i18n.T("cmd.init.wizard_tracker_configure_now")
			} else if w.trackerChoice == "keep" {
				val = i18n.T("cmd.init.wizard_tracker_keep_existing")
			}
			return []views.InfoField{{Label: "Tracker", Value: val}}
		},
	}
}

// trackerTokenStep asks the tracker token when the tracker is kept and no
// token is known for its server.
func (w *rejoinWizard) trackerTokenStep() views.WizardStep {
	return buildTrackerTokenStep(trackerTokenStep{
		ID:        "rejoin_tracker_token",
		Label:     i18n.Tf("cmd.init.wizard_tracker_token_label", ""),
		InfoLabel: i18n.T("cmd.tui_team_rejoin.tracker_token"),
		App:       func() *app.App { return w.a },
		Ctx:       func() context.Context { return w.ctx },
		Repo:      func() string { return w.stateRepo },
		StatePath: func() string { return w.statePath },
		Keep:      func() bool { return w.stateRepo != "" && w.trackerChoice == "keep" },
	})
}

// tokenForm asks what to do with the GitLab token: reuse the one that is
// known, give a new one, or leave it.
func (w *rejoinWizard) tokenForm(_ *tview.Application, onDone func()) *tview.Form {
	a, ctx := w.a, w.ctx
	teamID := config.RepoNameFromRemote(w.stateRepo)
	source := gitlabTokenSourceAt(ctx, a, teamID, w.stateRepo, w.gitlabURL)

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
		options, w.tokenChoiceIdx,
		func(_ string, idx int) {
			if w.tokenChoiceIdx == idx {
				return
			}
			if idx >= 0 && idx < len(optionKeys) {
				w.tokenChoice = optionKeys[idx]
			}
			w.tokenChoiceIdx = idx
		},
	)
	form.AddTextView("", i18n.T("cmd.init.wizard_rejoin_token_new_hint"), 60, 2, true, false)
	form.AddPasswordField(
		i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
		w.gitlabToken, 0, '*',
		func(t string) { w.gitlabToken = t },
	)
	form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
	return form
}

// storeToken puts the new GitLab token of the team in the keychain.
func (w *rejoinWizard) storeToken() error {
	a, ctx := w.a, w.ctx
	if w.tokenChoice == "new" && w.gitlabToken != "" && a.Secrets != nil {
		teamID := config.RepoNameFromRemote(w.stateRepo)
		return a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), w.gitlabToken)
	}
	return nil
}

// mismatchForm explains the identity that does not match and asks to verify
// it with a token or to skip.
func (w *rejoinWizard) mismatchForm(_ *tview.Application, onDone func()) *tview.Form {
	form := tview.NewForm()

	var msg string
	if w.identityMismatch.IsBot {
		msg = i18n.Tf("cmd.init.wizard_identity_bot_detected", w.identityMismatch.AuthenticatedAs)
	} else {
		msg = i18n.Tf("cmd.init.wizard_identity_user_mismatch",
			w.identityMismatch.AuthenticatedAs,
			w.identityMismatch.MemberID,
			w.identityMismatch.ExpectedUser,
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
				w.mismatchChoice = "verify"
			case 2:
				w.mismatchChoice = "skip"
			default:
				w.mismatchChoice = ""
			}
		},
	)

	form.AddTextView("", i18n.T("cmd.init.wizard_identity_pat_hint"), 60, 2, true, false)
	form.AddPasswordField(
		i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
		w.mismatchPAT, 0, '*',
		func(t string) { w.mismatchPAT = t },
	)

	form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
	return form
}

// recoverIdentity stores the token given and verifies the identity again, unless the user skipped.
func (w *rejoinWizard) recoverIdentity() error {
	a, ctx := w.a, w.ctx
	if w.mismatchChoice == "skip" {
		w.identityMismatch = nil
		return nil
	}
	// "verify" — store the PAT and re-validate
	if w.mismatchPAT != "" && a.Secrets != nil {
		teamID := config.RepoNameFromRemote(w.stateRepo)
		if err := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), w.mismatchPAT); err != nil {
			return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.storing_token"), err)
		}
	}
	sp := w.statePath
	if sp == "" {
		sp = config.TeamStatePath(w.stateRepo)
	}
	repo := teamstate.NewRepo(w.stateRepo, sp)
	member, err := repo.GetMember(w.memberID)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.member_lookup"), err)
	}
	if err := validateGitLabIdentity(ctx, a, repo, member); err != nil {
		return fmt.Errorf(i18n.T("cmd.tui_team_rejoin.identity_verification"), err)
	}
	w.identityMismatch = nil
	return nil
}

// trackerSetupForm shows the tracker of the team-state and asks what to do with it.
func (w *rejoinWizard) trackerSetupForm(_ *tview.Application, onDone func()) *tview.Form {
	form := tview.NewForm()

	sp := w.statePath
	if sp == "" {
		sp = config.TeamStatePath(w.stateRepo)
	}
	repo := teamstate.NewRepo(w.stateRepo, sp)
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
				w.trackerChoice = "keep"
				w.launchDiscovery = false
			case 2:
				w.trackerChoice = "reconfigure"
				w.launchDiscovery = true
			case 3:
				w.trackerChoice = "later"
				w.launchDiscovery = false
			default:
				w.trackerChoice = ""
				w.launchDiscovery = false
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
				w.trackerChoice = "reconfigure"
				w.launchDiscovery = true
			case 2:
				w.trackerChoice = "later"
				w.launchDiscovery = false
			default:
				w.trackerChoice = ""
				w.launchDiscovery = false
			}
		})
	}
	form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
	return form
}

// memberTrackerUsername is the tracker name of member id in members ("" when
// it has none or is not there).
func memberTrackerUsername(members []teamstate.Member, id string) string {
	for _, m := range members {
		if m.ID == id {
			return m.TrackerUsername
		}
	}
	return ""
}
