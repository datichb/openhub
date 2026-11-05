package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team resolution helpers
// ─────────────────────────────────────────────────────────────────────────────

// teamEnabledForProject reports whether team features are active for a specific project.
// Pass nil as project to check hub-level only.
func teamEnabledForProject(a *app.App, project *domain.Project) bool {
	return resolvedTeamConfig(a, project).Enabled
}

// resolvedTeamConfig returns the effective team configuration for a project,
// or, for a nil project, the one of the team ResolveTeam gives (the team chosen
// in the TUI, the team of the active project, the default team). The commands
// that read --team use resolvedTeamConfigCtx.
func resolvedTeamConfig(a *app.App, project *domain.Project) config.ResolvedTeamConfig {
	return resolvedTeamConfigCtx(context.Background(), a, project)
}

// ─────────────────────────────────────────────────────────────────────────────
// Wizard step (used in oh project add)
// ─────────────────────────────────────────────────────────────────────────────

// initWizardTeamState holds mutable state shared across the team wizard steps.
// It is allocated once by buildInitWizardTeamSteps and captured by closures.
type initWizardTeamState struct {
	Skipped     bool   // user chose not to configure team
	Mode        string // "init", "rejoin", or "keep" (empty = not chosen yet)
	Repo        string // team-state repo URL
	GitLabURL   string // GitLab of the repo when its remote is an SSH alias (optional)
	MemberID    string // member identifier
	DisplayName string // optional display name
	Configured  bool   // true after successful teamInitCore or teamRejoinCore
	// StructureWarning tells that the commit of the structure of the
	// team-state failed (teamInitCore); the join went on.
	StructureWarning string
	TeamID           string // derived team ID (after init or rejoin)
	// Existing team detection (re-init)
	ExistingTeam *config.TeamConfig // non-nil if a team is already configured
	TeamChoice   string             // "keep", "reconfigure", "skip" (only when ExistingTeam != nil)
	// SoloSpace: the user chose a solo workflow space; the
	// wizard's project is attached to it.
	SoloSpace     bool
	attachProject bool            // true if user wants to attach the project to this team
	Ctx           context.Context // propagated to OnDone closures (set by caller)
	// Rejoin-specific state
	Members         []teamstate.Member // fetched members for rejoin flow
	TeamRepo        rejoinRepo         // repo handle (set after clone in rejoin_repo)
	AddingNewMember bool               // true when the user chose to create their own card in the rejoin flow
	// StaleCause is why the list of members could not be brought up to date
	// ("" when it is).
	StaleCause string
	// Identity of the member (« Créer » asks it, « Rejoindre » when the person
	// creates their card): the GitLab handle is optional; the role of who
	// creates the team and the one of who joins it start differently
	// (creatorRole, joinerRole).
	GitLabUser string
	Role       string
	JoinRole   string
	// Claim: « C'est moi » after a refused ID in « Créer » (see teamInitParams).
	Claim  bool
	joiner *memberJoin
	// Name of the member on the tracker, asked when the tracker is on another
	// server than the team repository (TrackerWanted).
	TrackerWanted   bool
	TrackerUsername string
	// Abandoned: the user gave the team up (« Passer l'équipe », or a step of
	// the group skipped): the group is skipped whole and the summary says
	// that the team is not configured.
	Abandoned bool
	// Access is the state of the repository and access steps (see
	// buildInitWizardRepoSteps), shared by « Créer » and « Rejoindre ».
	Access *teamAccess
	// GitLab identity verification
	GitLabToken      string                 // token entered in the optional GitLab token prompt
	TokenChoice      string                 // "reuse", "new", or "skip"
	TokenChoiceIdx   int                    // current dropdown index (0 = placeholder)
	IdentityMismatch *identityMismatchError // non-nil when GitLab identity mismatch detected
	MismatchPAT      string                 // PAT entered in the mismatch recovery step
	MismatchChoice   string                 // "verify" or "skip"
	// Tracker setup
	LaunchTrackerDiscovery bool   // true if user chose "Configure now" for the tracker
	TrackerChoice          string // "keep", "reconfigure", or "later"
	TrackerToken           string // token entered in the tracker token prompt (if missing from keychain)
}

// abandon gives the team up: every step of the team group skips (Skipped,
// no Mode), the summary says so (Abandoned). A team that was configured
// before the wizard stays as it is.
func (s *initWizardTeamState) abandon() {
	s.Skipped, s.Mode, s.Abandoned = true, "", true
}

// access returns the state of the repository and access steps, made on first
// use.
func (s *initWizardTeamState) access(a **app.App) *teamAccess {
	if s.Access == nil {
		s.Access = &teamAccess{
			Ctx:         func() context.Context { return s.Ctx },
			App:         func() *app.App { return *a },
			Repo:        &s.Repo,
			GitLabURL:   &s.GitLabURL,
			Active:      func() bool { return !s.Skipped && (s.Mode == "init" || s.Mode == "rejoin") },
			Write:       func() bool { return s.Mode == "init" }, // « Créer » writes first: the right to write is probed
			Abandon:     s.abandon,
			RequiredKey: "cmd.init.wizard_team_repo_required",
		}
	}
	return s.Access
}

// join returns the member steps of « Rejoindre », made on first use.
func (s *initWizardTeamState) join(a **app.App) *memberJoin {
	if s.joiner == nil {
		s.joiner = &memberJoin{
			Members:  &s.Members,
			MemberID: &s.MemberID,
			Adding:   &s.AddingNewMember,
			Fields: identityFields{ID: &s.MemberID, DisplayName: &s.DisplayName, GitLab: &s.GitLabUser,
				Role: &s.JoinRole, DefaultRole: joinerRole},
			Repo: func() teamstate.MemberJoiner {
				if s.TeamRepo == nil {
					return nil
				}
				return s.TeamRepo
			},
			Ctx:      func() context.Context { return s.Ctx },
			Own:      func() string { return teamService(*a).MemberIDOf(s.Repo) },
			Stale:    func() string { return s.StaleCause },
			Active:   func() bool { return !s.Skipped && s.Mode == "rejoin" },
			OnSkip:   s.abandon,
			Errors:   s.access(a).ErrorActions,
			Defaults: func() (string, string, string) { return identityDefaults(*a)() },
			Verify: func(ctx context.Context, m teamstate.Member) error {
				return precheckIdentity(ctx, *a, s.Repo, s.GitLabURL, m)
			},
			PickerID: "rejoin_member", CardID: "rejoin_new_member", HandleID: "rejoin_handle",
		}
	}
	return s.joiner
}

// buildInitWizardRepoSteps returns the steps both « Créer » and « Rejoindre »
// start with: the repository (and its GitLab), the confirmation of an unknown
// SSH server, the probe of the access, and the access (a token or the SSH key) when the probe was refused. They come before
// the clone and before the identity of the member.
func buildInitWizardRepoSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	access := state.access(a)
	steps := []views.WizardStep{access.RepoStep()}
	steps = append(steps, access.HostSteps()...)
	return append(steps, access.ProbeStep(), access.AccessStep())
}

// selectedMemberGitLabUsername returns the GitLab username of the currently
// selected member, or "" if the member has no GitLab username configured.
func selectedMemberGitLabUsername(state *initWizardTeamState) string {
	for _, m := range state.Members {
		if m.ID == state.MemberID {
			return m.GitLabUsername
		}
	}
	return ""
}

// gitlabTokenSource returns a human-readable label describing the source of
// the GitLab token that resolves for the team teamID on the server of its
// team-state repository repo (the unique cascade of the team service), or ""
// if no token serves that server.
func gitlabTokenSource(ctx context.Context, a *app.App, teamID, repo string) string {
	return gitlabTokenSourceAt(ctx, a, teamID, repo, registeredGitLabURL(a, repo))
}

// gitlabTokenSourceAt is gitlabTokenSource for a team not in hub.toml yet,
// with the GitLab its repository is on when the remote is an SSH alias.
func gitlabTokenSourceAt(ctx context.Context, a *app.App, teamID, repo, gitlabURL string) string {
	switch gitlabTeamTokenAt(ctx, a, teamID, repo, gitlabURL).Source {
	case tracker.TokenSourceEnv:
		return i18n.T("cmd.init.wizard_rejoin_token_source_env")
	case tracker.TokenSourceTeam:
		return i18n.Tf("cmd.init.wizard_rejoin_token_source_team", teamID)
	case tracker.TokenSourceMCP:
		return i18n.T("cmd.init.wizard_rejoin_token_source_mcp")
	case tracker.TokenSourceTracker, tracker.TokenSourceTrackerKey, tracker.TokenSourceProject:
		return i18n.T("cmd.init.wizard_rejoin_token_source_tracker")
	}
	return ""
}

// buildTrackerSetupSteps returns two WizardSteps for tracker configuration
// after a team init or rejoin:
//
//  1. Tracker choice — displays tracker info and offers "Keep" / "Reconfigure" / "Later".
//  2. Tracker token — conditional on "Keep" + no token in keychain. Prompts for a
//     personal token so that ticket sync works immediately.
//
// When "Reconfigure" is chosen, state.LaunchTrackerDiscovery is set to true;
// the caller's OnComplete is responsible for actually launching the Discovery Wizard.
//
// Both « Créer » and « Rejoindre » append these steps, each for its modes
// (modes): without that the question came twice after a creation.
func buildTrackerSetupSteps(a **app.App, state *initWizardTeamState, modes ...string) []views.WizardStep {
	inModes := func() bool {
		for _, m := range modes {
			if state.Mode == m {
				return true
			}
		}
		return false
	}
	return []views.WizardStep{
		// ── Step 1: Tracker choice ──
		{
			ID:    "tracker_setup",
			Label: i18n.T("cmd.init.wizard_step_tracker_setup"),
			SkipIf: func() bool {
				return state.Skipped || !state.Configured || !inModes()
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()

				// Load team-state config to check if a tracker is already configured.
				repo := teamstate.NewRepo(state.Repo, teamStatePathFor(*a, state.Repo))
				trackerConfigured := false
				if teamCfg, err := repo.LoadConfig(state.Ctx); err == nil && teamCfg.Tracker.Type != "" {
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

				// Build dropdown: options differ depending on whether a tracker exists.
				options := []string{i18n.T("cmd.init.wizard_select_placeholder")}
				if trackerConfigured {
					options = append(options,
						i18n.T("cmd.init.wizard_tracker_keep_existing"),
						i18n.T("cmd.init.wizard_tracker_configure_now"),
						i18n.T("cmd.init.wizard_tracker_configure_later"),
					)
					form.AddDropDown(
						i18n.T("cmd.init.wizard_step_tracker_setup"),
						options, 0,
						func(_ string, idx int) {
							switch idx {
							case 1:
								state.TrackerChoice = "keep"
								state.LaunchTrackerDiscovery = false
							case 2:
								state.TrackerChoice = "reconfigure"
								state.LaunchTrackerDiscovery = true
							case 3:
								state.TrackerChoice = "later"
								state.LaunchTrackerDiscovery = false
							default:
								state.TrackerChoice = ""
								state.LaunchTrackerDiscovery = false
							}
						},
					)
				} else {
					// "Configure now" / "Later"
					options = append(options,
						i18n.T("cmd.init.wizard_tracker_configure_now"),
						i18n.T("cmd.init.wizard_tracker_configure_later"),
					)
					form.AddDropDown(
						i18n.T("cmd.init.wizard_step_tracker_setup"),
						options, 0,
						func(_ string, idx int) {
							switch idx {
							case 1:
								state.TrackerChoice = "reconfigure"
								state.LaunchTrackerDiscovery = true
							case 2:
								state.TrackerChoice = "later"
								state.LaunchTrackerDiscovery = false
							default:
								state.TrackerChoice = ""
								state.LaunchTrackerDiscovery = false
							}
						},
					)
				}
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				status := i18n.T("cmd.init.wizard_tracker_configure_later")
				if state.LaunchTrackerDiscovery {
					status = i18n.T("cmd.init.wizard_tracker_configure_now")
				} else if state.TrackerChoice == "keep" {
					status = i18n.T("cmd.init.wizard_tracker_keep_existing")
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_info_tracker"), Value: status},
				}
			},
		},
		// ── Step 2: Tracker token (conditional) ──
		// Shown when the user kept the tracker and no token serves its server:
		// where the token is (key, server) and the token itself.
		buildTrackerTokenStep(trackerTokenStep{
			ID:        "tracker_token",
			Label:     i18n.Tf("cmd.init.wizard_tracker_token_label", ""),
			InfoLabel: i18n.T("cmd.init.wizard_team_info_tracker"),
			App:       func() *app.App { return *a },
			Ctx: func() context.Context {
				if state.Ctx == nil {
					return context.Background()
				}
				return state.Ctx
			},
			Repo: func() string { return state.Repo },
			Keep: func() bool {
				return !state.Skipped && state.Configured && inModes() && state.TrackerChoice == "keep"
			},
		}),
	}
}

// buildInitWizardTeamSteps returns the WizardSteps of « Créer une équipe » in
// the hub initialization wizards (first-run inline + oh init standalone).
//
// The returned steps are:
//  1. Form — member ID, display name
//  2. Processing — clone/pull, init structure, register member, write hub.toml
//  3. the tracker steps
//
// The repository and its access come first (buildInitWizardRepoSteps), the
// caller prepends the intro step (buildTeamModeIntroStep) that sets
// state.Skipped. The steps use SkipIf → state.Skipped, and OnSkip →
// state.abandon: skipping one skips the group.
//
// After successful completion, state.Configured is true and the app should be
// refreshed (refreshSharedConfig) so subsequent steps can see the team.
func buildInitWizardTeamSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	steps := []views.WizardStep{
		// ── Team form: identity of the member (the repository and the access
		// came before: buildInitWizardRepoSteps) ──
		{
			ID:     "team_form",
			Label:  i18n.T("cmd.init.wizard_step_team_form"),
			OnSkip: state.abandon,
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "init"
			},
			Validate: func() string { return teamCreatorFields(state).problem(nil, "", "v532m.join.id_taken_hint") },
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				teamCreatorFields(state).addTo(form)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_info_member"), Value: state.MemberID},
				}
			},
		},
		// ── Team processing: clone + init + register ──
		{
			ID:    "team_sync",
			Label: i18n.T("cmd.init.wizard_step_team_sync"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "init"
			},
			OnSkip:       state.abandon,
			BackSkip:     true,
			ErrorActions: identityErrorActions(&state.Claim, "team_form", "team_sync", state.access(a).ErrorActions),
			Processing:   i18n.T("cmd.init.wizard_processing_team"),
			OnDone: func() error {
				displayName := state.DisplayName
				if displayName == "" {
					displayName = state.MemberID
				}
				warn, err := teamInitCore(state.Ctx, *a, teamInitParams{
					StateRepo:      state.Repo,
					MemberID:       strings.TrimSpace(state.MemberID),
					DisplayName:    strings.TrimSpace(displayName),
					GitLabUsername: state.GitLabUser,
					Role:           firstNonEmpty(state.Role, creatorRole),
					Claim:          state.Claim,
					GitLabURL:      state.GitLabURL,
				})
				if err != nil {
					return err
				}
				state.StructureWarning = warn
				state.Configured = true
				state.TeamID = teamIDFor(*a, state.Repo)

				// Bring the shared config up to date so subsequent steps see the team
				if reloadErr := refreshSharedConfig(*a); reloadErr != nil {
					slog.Warn("config refresh failed after team connect", "err", reloadErr)
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if state.Configured {
					fields := []views.InfoField{
						{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_team_connected")},
					}
					if state.StructureWarning != "" {
						fields = append(fields, views.InfoField{Label: theme.IconWarning + " " + i18n.T("cmd.team.init.structure_label"), Value: state.StructureWarning})
					}
					return fields
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_status_label"), Value: i18n.T("cmd.init.wizard_team_skipped")},
				}
			},
		},
		// ── Init: Tracker setup (conditional) ──
	}
	steps = append(steps, buildTrackerSetupSteps(a, state, "init", "keep")...)
	return steps
}

// buildInitWizardRejoinSteps returns the WizardSteps for the "rejoin" flow
// used in the hub initialization wizards (first-run inline + oh init standalone).
//
// The returned steps are:
//  1. Clone — clones the team-state repo and fetches the member list (the URL
//     and the access came before: buildInitWizardRepoSteps)
//  2. Member selection — SectionedList to pick the existing member
//  3. Validation — validates GitLab identity, writes hub.toml, retro-tags sessions
//
// All steps have SkipIf → state.Skipped || state.Mode != "rejoin".
// After successful completion, state.Configured is true and the app should be
// reloaded so subsequent steps can see the team.
func buildInitWizardRejoinSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	var steps []views.WizardStep
	steps = []views.WizardStep{
		// ── Rejoin step 1: clone the repository and list its members (the URL
		// and the access came before: buildInitWizardRepoSteps) ──
		{
			ID:    "rejoin_clone",
			Label: i18n.T("v532w.clone.label"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != "rejoin"
			},
			OnSkip:       state.abandon,
			BackSkip:     true,
			ErrorActions: state.access(a).ErrorActions,
			OnDone: func() error {
				repo, members, stale, err := listTeamMembers(state.Ctx, state.Repo, teamStatePathFor(*a, state.Repo))
				if err != nil {
					return err
				}
				state.StaleCause = staleCause(stale)
				state.Members = members
				state.TeamRepo = repo
				state.TrackerWanted = trackerUsernameWanted(state.Ctx, repo, state.Repo, state.GitLabURL)
				return nil
			},
			Processing: i18n.T("cmd.init.wizard_processing_rejoin_clone"),
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_repo"), Value: state.Repo}}
			},
		},
	}

	// ── Rejoin step 4: the member (pick one's card, or create it), the form of
	// the card, the handle of a card that has none ──
	steps = append(steps, state.join(a).Steps()...)

	// ── Rejoin step 4a': name of the member on the tracker (conditional) ──
	// Asked when the tracker is on another server than the team repository and
	// the profile of the member (existing, or just created above) has none.
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_tracker_username",
		Label: i18n.T("tui.team.rejoin.tracker_step"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin" || !state.TrackerWanted ||
				memberTrackerUsername(state.Members, state.MemberID) != ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddTextView("", i18n.T("cmd.team.rejoin.tracker_hint"), 60, 3, true, false)
			form.AddInputField(i18n.T("tui.team.rejoin.tracker_label"), state.TrackerUsername, 0, nil,
				func(t string) { state.TrackerUsername = strings.TrimSpace(t) })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("tui.team.rejoin.tracker_step"),
				Value: firstNonEmpty(state.TrackerUsername, i18n.T("tui.team.rejoin.tracker_none"))}}
		},
	})

	// ── Rejoin step 4b: GitLab API token (conditional) ──
	// The token of the access step is the git one (read_repository,
	// write_repository): the GitLab API refuses it. This step asks the API
	// token (identity check, tracker) whatever the repo URL, when the member
	// has a gitlab_username; it can be skipped.
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_gitlab_token",
		Label: i18n.T("cmd.init.wizard_step_rejoin_gitlab_token"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin" ||
				selectedMemberGitLabUsername(state) == ""
		},
		Validate: func() string {
			if state.TokenChoiceIdx == 0 {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if state.TokenChoice == "new" && strings.TrimSpace(state.GitLabToken) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			teamID := teamIDFor(*a, state.Repo)
			source := gitlabTokenSourceAt(state.Ctx, *a, teamID, state.Repo, state.GitLabURL)

			// Build dropdown options: placeholder + [reuse if available] + new + skip
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
				options, state.TokenChoiceIdx,
				func(_ string, idx int) {
					if state.TokenChoiceIdx == idx {
						return
					}
					if idx >= 0 && idx < len(optionKeys) {
						state.TokenChoice = optionKeys[idx]
					}
					state.TokenChoiceIdx = idx
				},
			)
			form.AddTextView("", i18n.T("cmd.init.wizard_rejoin_token_new_hint"), 60, 2, true, false)
			form.AddPasswordField(
				i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
				state.GitLabToken, 0, '*',
				func(t string) { state.GitLabToken = t },
			)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			if state.TokenChoice == "new" && state.GitLabToken != "" && (*a).Secrets != nil {
				teamID := teamIDFor(*a, state.Repo)
				return (*a).Secrets.Set(state.Ctx, config.TeamGitLabTokenKey(teamID), state.GitLabToken)
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			var status string
			switch state.TokenChoice {
			case "reuse":
				status = i18n.T("cmd.init.wizard_rejoin_token_reuse_short")
			case "new":
				status = i18n.T("cmd.init.wizard_rejoin_token_stored")
			default:
				status = i18n.T("cmd.init.wizard_team_skipped")
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"), Value: status},
			}
		},
	})

	// ── Rejoin step 5: Validate identity + write config ──
	steps = append(steps, views.WizardStep{
		ID:         "rejoin_validate",
		Label:      i18n.T("cmd.init.wizard_step_rejoin_validate"),
		Processing: i18n.T("cmd.init.wizard_processing_rejoin_validate"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin"
		},
		OnSkip:       state.abandon,
		ErrorActions: state.access(a).ErrorActions,
		OnDone: func() error {
			params := teamRejoinParams{
				StateRepo: state.Repo,
				MemberID:  state.MemberID,
				GitLabURL: state.GitLabURL,
			}
			if state.TrackerWanted && state.TrackerUsername != "" &&
				memberTrackerUsername(state.Members, state.MemberID) == "" {
				params.TrackerUsername = &state.TrackerUsername
			}
			result, err := teamRejoinCore(state.Ctx, *a, params)
			if err != nil {
				return err
			}
			if result.ProfileErr != nil {
				slog.Warn("rejoin: tracker name not written to the profile", "error", result.ProfileErr)
			}

			// Store identity mismatch for the next conditional step.
			state.IdentityMismatch = result.IdentityMismatch
			if result.IdentityUnverified != nil && tuiShell != nil {
				warning := result.IdentityUnverified.Error()
				queueOnShell(tuiShell, func() { tuiShell.ShowToast(warning, shell.ToastWarning) })
			}

			state.Configured = true
			state.TeamID = result.TeamID

			// Retro-tag sessions synchronously (wizard shows spinner)
			if tagged, tagErr := retroTagSessions(state.Ctx, state.MemberID); tagErr != nil {
				slog.Warn("retro-tagging sessions failed", "err", tagErr)
			} else if tagged > 0 {
				slog.Info("retro-tagged sessions", "count", tagged)
			}

			// Bring the shared config up to date so subsequent steps see the team
			if reloadErr := refreshSharedConfig(*a); reloadErr != nil {
				slog.Warn("config refresh failed after team rejoin", "err", reloadErr)
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

	// ── Rejoin step 5b: Identity mismatch recovery (conditional) ──
	// Shown only when validateGitLabIdentity detected a bot token or username
	// mismatch. Offers the user a chance to provide a PAT or skip verification.
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_identity_mismatch",
		Label: i18n.T("cmd.init.wizard_step_identity_mismatch"),
		SkipIf: func() bool {
			return state.IdentityMismatch == nil
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			// Build the error message based on mismatch type.
			var msg string
			if state.IdentityMismatch.IsBot {
				msg = i18n.Tf("cmd.init.wizard_identity_bot_detected", state.IdentityMismatch.AuthenticatedAs)
			} else {
				msg = i18n.Tf("cmd.init.wizard_identity_user_mismatch",
					state.IdentityMismatch.AuthenticatedAs,
					state.IdentityMismatch.MemberID,
					state.IdentityMismatch.ExpectedUser,
				)
			}
			form.AddTextView("", msg, 60, 3, true, false)

			// Dropdown: verify with PAT / skip
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
						state.MismatchChoice = "verify"
					case 2:
						state.MismatchChoice = "skip"
					default:
						state.MismatchChoice = ""
					}
				},
			)

			// PAT input + hint
			form.AddTextView("", i18n.T("cmd.init.wizard_identity_pat_hint"), 60, 2, true, false)
			form.AddPasswordField(
				i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"),
				state.MismatchPAT, 0, '*',
				func(t string) { state.MismatchPAT = t },
			)

			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		Validate: func() string {
			if state.MismatchChoice == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_choice_required")
			}
			if state.MismatchChoice == "verify" && strings.TrimSpace(state.MismatchPAT) == "" {
				return i18n.T("cmd.init.wizard_rejoin_token_new_required")
			}
			return ""
		},
		OnDone: func() error {
			if state.MismatchChoice == "skip" {
				state.IdentityMismatch = nil
				return nil
			}
			// "verify" — store the PAT and re-validate
			if state.MismatchPAT != "" && (*a).Secrets != nil {
				teamID := teamIDFor(*a, state.Repo)
				if err := (*a).Secrets.Set(state.Ctx, config.TeamGitLabTokenKey(teamID), state.MismatchPAT); err != nil {
					return fmt.Errorf(i18n.T("cmd.team_helpers.storing_token"), err)
				}
			}
			// Re-run identity validation with the new token
			repo := teamstate.NewRepo(state.Repo, teamStatePathFor(*a, state.Repo))
			member, err := repo.GetMember(state.Ctx, state.MemberID)
			if err != nil {
				return fmt.Errorf(i18n.T("cmd.team_helpers.member_lookup"), err)
			}
			if err := validateGitLabIdentity(state.Ctx, *a, repo, member); err != nil {
				return fmt.Errorf(i18n.T("cmd.team_helpers.identity_verification"), err)
			}
			state.IdentityMismatch = nil
			return nil
		},
		InfoFields: func() []views.InfoField {
			if state.MismatchChoice == "skip" {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_identity"), Value: i18n.T("cmd.init.wizard_identity_skip")}}
			}
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_identity"), Value: i18n.T("cmd.init.wizard_identity_verified")}}
		},
	})

	// ── Rejoin: Tracker setup (conditional) ──
	steps = append(steps, buildTrackerSetupSteps(a, state, "rejoin")...)

	return steps
}

// askInlineQuery asks a line of text in an input under the list of a wizard
// step (there is no shell to show a window): Enter validates, Esc closes it.
func askInlineQuery(tvApp *tview.Application, box *tview.Flex, back tview.Primitive, title, current string, done func(string)) {
	input := tview.NewInputField().
		SetLabel(" " + title + " ").
		SetLabelColor(theme.ActiveMode.Secondary).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetText(current)
	input.SetBackgroundColor(theme.BgElement)
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter && key != tcell.KeyEscape {
			return
		}
		text := input.GetText()
		box.RemoveItem(input)
		tvApp.SetFocus(back)
		if key == tcell.KeyEnter {
			done(text)
		}
	})
	box.AddItem(input, 1, 0, true)
	tvApp.SetFocus(input)
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
		ID:            "team_mode",
		Label:         i18n.T("cmd.init.wizard_step_team"),
		Required:      true,
		SidebarHidden: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			accent := theme.ColorTag(theme.ActiveMode.AccentHex)
			secondary := theme.ColorTag(theme.TextSecondaryHex)
			muted := theme.ColorTag(theme.TextMutedHex)
			warning := theme.ColorTag(theme.WarningHex)
			infoColor := theme.ColorTag(theme.InfoHex)
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

			if state.ExistingTeam != nil {
				// ── Existing team recap ──
				fmt.Fprintf(&b, "%s%s %s%s\n", infoColor, theme.IconInfo,
					i18n.T("cmd.init.wizard_team_existing"), reset)
				fmt.Fprintf(&b, "%s  %s%s\n\n", muted,
					i18n.Tf("cmd.init.wizard_team_existing_info",
						state.ExistingTeam.ID,
						state.ExistingTeam.StateRepo,
						state.ExistingTeam.MemberID), reset)
			} else if prereqs != "" {
				// ── Prerequisites (only when no existing team) ──
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

			// Note
			if note != "" {
				for _, line := range strings.Split(note, "\n") {
					fmt.Fprintf(&b, "%s%s%s\n", muted, line, reset)
				}
				b.WriteString("\n")
			}

			introText := b.String()

			if state.ExistingTeam != nil {
				// ── Existing team detected: dropdown + buttons ──
				form := tview.NewForm()
				options := []string{
					i18n.T("cmd.init.wizard_select_placeholder"),
					i18n.T("cmd.init.wizard_team_choice_keep"),
					i18n.T("cmd.init.wizard_team_choice_reconfigure"),
					i18n.T("wizard.intro.skip"),
				}
				defaultIdx := 0
				switch state.TeamChoice {
				case "keep":
					defaultIdx = 1
				case "reconfigure":
					defaultIdx = 2
				case "skip":
					defaultIdx = 3
				}

				var rebuildButtons func()
				buttonForm := views.NewStyledButtonForm()

				choiceMounted := false
				form.AddDropDown(i18n.T("cmd.init.wizard_team_choice_label"), options, defaultIdx, func(_ string, idx int) {
					switch idx {
					case 1:
						state.TeamChoice = "keep"
					case 2:
						state.TeamChoice = "reconfigure"
					case 3:
						state.TeamChoice = "skip"
					default:
						state.TeamChoice = ""
					}
					if choiceMounted {
						safego.Go(func() { tvApp.QueueUpdateDraw(func() { rebuildButtons() }) })
					}
				})
				choiceMounted = true
				views.FixFormDropDownStyles(form)
				styleWizardForm(form)

				rebuildButtons = func() {
					buttonForm.Clear(true)
					switch state.TeamChoice {
					case "keep":
						buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
							state.Mode = "keep"
							state.Configured = true
							state.TeamID = state.ExistingTeam.ID
							state.Skipped = false
							onDone()
						})
					case "reconfigure":
						buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_init")+"  ", func() {
							state.Mode = "init"
							state.Skipped = false
							onDone()
						})
						buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_rejoin")+"  ", func() {
							state.Mode = "rejoin"
							state.Skipped = false
							onDone()
						})
					case "skip":
						buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
							state.Skipped = true
							state.Mode = ""
							onDone()
						})
					default:
						buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", func() {
							onDone()
						})
					}
				}
				rebuildButtons()

				maxW := views.MaxVisibleWidth(introText)
				views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
					Badge:           i18n.T("cmd.init.wizard_step_team"),
					Intro:           introText,
					SectionTitle:    i18n.T("cmd.init.wizard_section_team"),
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
			} else {
				// ── No existing team: 3 buttons (original layout) ──
				buttonForm := views.NewStyledButtonForm()

				buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_init")+"  ", func() {
					state.Mode = "init"
					state.Skipped = false
					state.SoloSpace = false
					onDone()
				})

				buttonForm.AddButton("  "+i18n.T("cmd.init.wizard_team_mode_rejoin")+"  ", func() {
					state.Mode = "rejoin"
					state.Skipped = false
					state.SoloSpace = false
					onDone()
				})

				buttonForm.AddButton("  "+i18n.T("tui.solo.wizard.choice")+"  ", func() {
					state.Mode = ""
					state.Skipped = true
					state.SoloSpace = true
					onDone()
				})

				skipConfirmed := false
				buttonForm.AddButton("  "+i18n.T("wizard.intro.skip")+"  ", func() {
					if !skipConfirmed {
						skipConfirmed = true
						if btn := buttonForm.GetButton(3); btn != nil {
							btn.SetLabel("  " + i18n.T("wizard.intro.skip_confirm") + "  ")
						}
						return
					}
					state.Skipped = true
					state.Mode = ""
					onDone()
				})

				views.BuildWizardPage(tvApp, container, views.WizardPageLayout{
					Badge:       i18n.T("cmd.init.wizard_step_team"),
					Intro:       introText,
					Buttons:     buttonForm,
					FocusTarget: buttonForm,
				})
				views.SetupCrossSectionNav(views.CrossSectionNavConfig{
					App:     tvApp,
					Buttons: buttonForm,
				})
			}
		},
		Validate: func() string {
			if state.ExistingTeam != nil && state.TeamChoice == "" {
				return i18n.T("cmd.init.wizard_project_choice_required") // reuse: "Please select an option"
			}
			return ""
		},
		InfoFields: func() []views.InfoField {
			if state.Mode == "keep" && state.ExistingTeam != nil {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_step_team"), Value: infoSuccess(i18n.T("cmd.init.wizard_team_kept"))},
				}
			}
			return nil // for init/rejoin/skip, sub-steps provide InfoFields.
		},
	}
}

// attachableTeams are the teams a project can be attached to: the enabled ones
// that are not a solo space, the default team first.
func attachableTeams(cfg *config.Config) []config.TeamConfig {
	var out []config.TeamConfig
	def := cfg.DefaultTeam()
	if def != nil {
		out = append(out, *def)
	}
	for _, t := range cfg.Teams {
		if t.Enabled && !t.Solo && (def == nil || t.ID != def.ID) {
			out = append(out, t)
		}
	}
	return out
}

// teamChoiceLabel is how a team shows in a list where a team is chosen.
func teamChoiceLabel(cfg *config.Config, t config.TeamConfig) string {
	label := i18n.Tf("v54r.team.choice", t.DisplayName(), t.MemberID)
	if def := cfg.DefaultTeam(); def != nil && def.ID == t.ID {
		label += " " + i18n.T("v54r.team.default_mark")
	}
	return label
}

// buildProjectTeamStep returns a WizardStep that collects team configuration
// for a project during project creation or reconfiguration.
//
// The member chooses among all the teams (the default one first and chosen
// unless the member picks another), a solo space or no team. The step sets
// out to:
//   - &teamID → attach the project to the team chosen
//   - nil     → no team for this project
func buildProjectTeamStep(a *app.App, out **string, solo *bool) views.WizardStep {
	teams := attachableTeams(a.Config)
	hasTeam := len(teams) > 0

	// Choices: one per team, solo space, no team.
	const (
		choiceSolo = "solo"
		choiceNone = "none"
	)
	var choices []string
	for _, t := range teams {
		choices = append(choices, t.ID)
	}
	choices = append(choices, choiceSolo, choiceNone)
	choice := choices[0]
	if !hasTeam {
		choice = choiceNone
	}
	soloLabel := i18n.T("tui.solo.project.new")
	for _, t := range a.Config.Teams {
		if t.Solo && t.Enabled {
			soloLabel = i18n.Tf("tui.solo.project.existing", t.ID)
			break
		}
	}
	labelOf := func(c string) string {
		switch c {
		case choiceSolo:
			return soloLabel
		case choiceNone:
			return i18n.T("form.project.team_no_team")
		}
		if t := a.Config.FindTeam(c); t != nil {
			return teamChoiceLabel(a.Config, *t)
		}
		return c
	}

	return views.WizardStep{
		Label: i18n.T("form.project.team_label"),
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			if hasTeam {
				summary := i18n.Tf("v54r.team.wizard_summary", len(teams), teams[0].ID, teams[0].MemberID)
				form.AddTextView(i18n.T("form.project.team_hub_label"), summary, 0, 1, false, false)
			} else {
				form.AddTextView(i18n.T("form.project.team_hub_label"), i18n.T("form.project.team_none_configured"), 0, 1, false, false)
			}
			var labels []string
			initial := 0
			for i, c := range choices {
				labels = append(labels, labelOf(c))
				if c == choice {
					initial = i
				}
			}
			form.AddDropDown(i18n.T("form.project.team_dropdown"), labels, initial, func(_ string, idx int) {
				choice = choices[idx]
			})
			form.AddTextView("", i18n.T("tui.solo.project.hint"), 0, 2, false, false)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			*out = nil
			isTeam := choice != choiceSolo && choice != choiceNone
			if solo != nil {
				*solo = choice == choiceSolo
			}
			if isTeam {
				id := choice
				*out = &id
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			value := i18n.T("form.project.team_value_none")
			switch choice {
			case choiceNone:
			case choiceSolo:
				value = soloLabel
			default:
				value = choice
			}
			return []views.InfoField{{Label: i18n.T("form.project.team_label"), Value: value}}
		},
	}
}
