package cmd

import (
	"context"
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
	teamRepo         rejoinRepo // the cloned team-state, to register the card of the member
	memberID         string
	adding           bool   // the person creates their own card
	displayName      string // the card being created
	gitlabUser       string
	role             string
	staleCause       string // why the list of members may be out of date ("" = it is up to date)
	join             *memberJoin
	gitlabToken      string
	tokenChoice      string // "reuse", "new", or "skip"
	tokenChoiceIdx   int    // current dropdown index (0 = placeholder)
	identityMismatch *identityMismatchError
	mismatchPAT      string
	mismatchChoice   string
	access           *teamAccess // the repository and its access (team_access.go)
	trackerWanted    bool        // the tracker is on another server than the team repository
	trackerName      string      // typed in the tracker name step

	launchDiscovery bool   // « configure now » was chosen for the tracker
	trackerChoice   string // "keep", "reconfigure", "later" or ""
}

// buildRejoinWizard is the wizard of the rejoin: its steps and what it does
// when it ends.
func buildRejoinWizard(ctx context.Context, a *app.App) views.InlineWizardConfig {
	w := makeRejoinWizard(ctx, a)
	steps := []views.WizardStep{
		w.access.RepoStep(),
		w.access.ProbeStep(),
		w.access.AccessStep(),
		w.cloneStep(),
	}
	steps = append(steps, w.join.Steps()...)
	steps = append(steps,
		w.trackerNameStep(),
		w.gitlabTokenStep(),
		w.validateStep(),
		w.identityMismatchStep(),
		w.trackerSetupStep(),
		w.trackerTokenStep(),
	)
	return views.InlineWizardConfig{
		ID:    "wizard.team.rejoin",
		Title: i18n.T("tui.team.rejoin"),
		Steps: steps,
		// the member step says how many members the team has, once it is cloned
		RefreshLabels: func() {
			if step := views.StepByID(steps, rejoinMemberStepID); step != nil {
				step.Label = w.join.Label()
			}
		},
		OnComplete: w.onComplete,
	}
}

// rejoinMemberStepID is the ID of the step that picks the member, in the
// assistants of « Rejoindre ».
const rejoinMemberStepID = "rejoin_member"

// makeRejoinWizard is the state of the rejoin, with the repository and access
// steps it starts with.
func makeRejoinWizard(ctx context.Context, a *app.App) *rejoinWizard {
	w := &rejoinWizard{ctx: ctx, a: a}
	w.access = &teamAccess{
		Ctx:         func() context.Context { return w.ctx },
		App:         func() *app.App { return w.a },
		Repo:        &w.stateRepo,
		GitLabURL:   &w.gitlabURL,
		Quit:        true, // the wizard is only the team: giving it up is leaving it
		RequiredKey: "cmd.team.init.validate.repo_required",
		NextLabel:   i18n.T("tui.team.rejoin.next"),
	}
	w.join = &memberJoin{
		Members:  &w.members,
		MemberID: &w.memberID,
		Adding:   &w.adding,
		Fields: identityFields{ID: &w.memberID, DisplayName: &w.displayName, GitLab: &w.gitlabUser,
			Role: &w.role, DefaultRole: joinerRole},
		Repo: func() teamstate.MemberJoiner {
			if w.teamRepo == nil {
				return nil
			}
			return w.teamRepo
		},
		Ctx:      func() context.Context { return w.ctx },
		Own:      func() string { return teamService(w.a).MemberIDOf(w.stateRepo) },
		Stale:    func() string { return w.staleCause },
		Errors:   w.access.ErrorActions,
		PickerID: rejoinMemberStepID, CardID: "rejoin_new_member", HandleID: "rejoin_handle",
	}
	return w
}

// onComplete launches the tracker discovery when « configure now » was chosen.
func (w *rejoinWizard) onComplete(completed bool, err error) {
	if completed && err == nil && w.launchDiscovery {
		safego.Go(func() {
			time.Sleep(200 * time.Millisecond)
			if tuiShell != nil {
				queueOnShell(tuiShell, func() {
					actionTrackerDiscovery()
				})
			}
		})
	}
}

// cloneStep clones the repository (the URL and the access came before) and
// lists its members.
func (w *rejoinWizard) cloneStep() views.WizardStep {
	ctx := w.ctx
	return views.WizardStep{
		ID:           "rejoin_clone",
		Label:        i18n.T("v532w.clone.label"),
		Required:     true,
		BackSkip:     true,
		ErrorActions: w.access.ErrorActions,
		OnDone: func() error {
			w.statePath = config.TeamStatePath(w.stateRepo)
			handle, members, stale, err := listTeamMembers(ctx, w.stateRepo, w.statePath)
			if err != nil {
				return err
			}
			w.teamRepo, w.members, w.staleCause = handle, members, staleCause(stale)
			w.trackerWanted = trackerUsernameWanted(ctx, handle, w.stateRepo, w.gitlabURL)
			return nil
		},
		Processing: i18n.T("cmd.team.init.processing_repo"),
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Repo", Value: w.stateRepo}}
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
			// the access step stores the token of an HTTPS repository that asked for one
			return glUser == "" || (teamstate.IsHTTPS(w.stateRepo) && w.access.TokenStored)
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
		Label:        i18n.T("cmd.team.rejoin.validating"),
		Processing:   i18n.T("cmd.team.rejoin.validating"),
		ErrorActions: w.access.ErrorActions,
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
				queueOnShell(tuiShell, func() {
					tuiShell.ShowToast(i18n.Tf("cmd.team.rejoin.tracker_failed", result.ProfileErr), shell.ToastError)
				})
			}

			// Store identity mismatch for the next conditional step.
			w.identityMismatch = result.IdentityMismatch

			// Retro-tag sessions in background
			safego.Go(func() {
				bgCtx := context.Background()
				count, err := retroTagSessionsOf(bgCtx, a, w.memberID)
				if err == nil && count > 0 && tuiShell != nil {
					queueOnShell(tuiShell, func() {
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
				queueOnShell(tuiShell, func() {
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
	member, err := repo.GetMember(ctx, w.memberID)
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
	if teamCfg, err := repo.LoadConfig(w.ctx); err == nil && teamCfg.Tracker.Type != "" {
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
