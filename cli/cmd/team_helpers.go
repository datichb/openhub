package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

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
//
// When secrets is non-nil and the repo host is a GitLab instance, the token
// is also stored in the keychain for GitLab API use (identity verification, etc.).
func buildHTTPSCredStep(ctx context.Context, repoURL *string, state *httpsCredState, extraSkipIf func() bool, secrets domain.SecretStore) views.WizardStep {
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
			form.AddInputField(i18n.T("cmd.init.wizard_team_cred_username"), state.Username, 0, nil,
				func(text string) { state.Username = text })
			form.AddPasswordField(i18n.T("cmd.init.wizard_team_cred_token"), state.Token, 0, '*',
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
				if err := teamstate.ConfigureCredential(ctx, *repoURL, state.Username, state.Token); err != nil {
					return err
				}
				// When the host is GitLab, also store in the keychain for API use.
				if teamstate.IsGitLabHost(*repoURL) && secrets != nil {
					teamID := config.RepoNameFromRemote(*repoURL)
					if err := secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), state.Token); err != nil {
						slog.Warn("failed to store GitLab token in keychain", "err", err)
					}
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_auth"), Value: i18n.T("cmd.init.wizard_team_info_configured")}}
		},
	}
}

// initWizardTeamState holds mutable state shared across the team wizard steps.
// It is allocated once by buildInitWizardTeamSteps and captured by closures.
type initWizardTeamState struct {
	Skipped     bool   // user chose not to configure team
	Mode        string // "init", "rejoin", or "keep" (empty = not chosen yet)
	Repo        string // team-state repo URL
	MemberID    string // member identifier
	DisplayName string // optional display name
	Configured  bool   // true after successful teamInitCore or teamRejoinCore
	TeamID      string // derived team ID (after init or rejoin)
	// Existing team detection (re-init)
	ExistingTeam *config.TeamConfig // non-nil if a team is already configured
	TeamChoice   string             // "keep", "reconfigure", "skip" (only when ExistingTeam != nil)
	// SoloSpace: the user chose a solo workflow space (P2-T16); the
	// wizard's project is attached to it.
	SoloSpace     bool
	attachProject bool            // true if user wants to attach the project to this team
	Ctx           context.Context // propagated to OnDone closures (set by caller)
	// Rejoin-specific state
	Members              []teamstate.Member // fetched members for rejoin flow
	TeamRepo             *teamstate.Repo    // repo handle (set after clone in rejoin_repo)
	AddingNewMember      bool               // true when user chose "add new member" in rejoin flow
	NewMemberDisplayName string             // display name for new member being added
	NewMemberGitLab      string             // gitlab username for new member being added
	NewMemberRole        string             // role for new member being added
	// HTTPS credential state (shared by init and rejoin)
	CredUsername   string // HTTPS username (default "oauth2")
	CredToken      string // HTTPS token/password
	CredAuthChoice string // "provide", "skip", or "public"
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

// gitlabTokenAvailable checks whether a GitLab API token is already
// gitlabTokenSource returns a human-readable label describing the first
// available GitLab token source, or "" if no token is resolvable.
// The teamID is used to check for a team-specific keychain key.
func gitlabTokenSource(ctx context.Context, a *app.App, teamID string) string {
	if os.Getenv("GITLAB_TOKEN") != "" {
		return i18n.T("cmd.init.wizard_rejoin_token_source_env")
	}
	if a != nil && a.Secrets != nil {
		if teamID != "" {
			if val, err := a.Secrets.Get(ctx, config.TeamGitLabTokenKey(teamID)); err == nil && val != "" {
				return i18n.Tf("cmd.init.wizard_rejoin_token_source_team", teamID)
			}
		}
		if val, err := a.Secrets.Get(ctx, config.DefaultGitLabTokenKey); err == nil && val != "" {
			return i18n.T("cmd.init.wizard_rejoin_token_source_mcp")
		}
		if val, err := a.Secrets.Get(ctx, "openhub.tracker.gitlab.token"); err == nil && val != "" {
			return i18n.T("cmd.init.wizard_rejoin_token_source_tracker")
		}
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
func buildTrackerSetupSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	return []views.WizardStep{
		// ── Step 1: Tracker choice ──
		{
			ID:    "tracker_setup",
			Label: i18n.T("cmd.init.wizard_step_tracker_setup"),
			SkipIf: func() bool {
				return state.Skipped || !state.Configured
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()

				// Load team-state config to check if a tracker is already configured.
				statePath := config.TeamStatePath(state.Repo)
				repo := teamstate.NewRepo(state.Repo, statePath)
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
		// Shown only when user chose "keep" and no token is available.
		{
			ID:    "tracker_token",
			Label: i18n.Tf("cmd.init.wizard_tracker_token_label", ""),
			SkipIf: func() bool {
				if state.Skipped || !state.Configured || state.TrackerChoice != "keep" {
					return true
				}
				// Check if a token already exists.
				statePath := config.TeamStatePath(state.Repo)
				repo := teamstate.NewRepo(state.Repo, statePath)
				teamCfg, err := repo.LoadConfig()
				if err != nil || teamCfg.Tracker.Type == "" {
					return true
				}
				tokenKey := teamCfg.Tracker.TrackerTokenKey
				if tokenKey == "" {
					tokenKey = "openhub.tracker." + teamCfg.Tracker.Type + ".token"
				}
				// Check env fallback
				envKey := "GITLAB_TOKEN"
				if teamCfg.Tracker.Type == "jira" {
					envKey = "JIRA_TOKEN"
				}
				if os.Getenv(envKey) != "" {
					return true
				}
				// Check keychain
				if *a != nil && (*a).Secrets != nil {
					if val, e := (*a).Secrets.Get(state.Ctx, tokenKey); e == nil && val != "" {
						return true
					}
				}
				return false
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				// Resolve tracker type for display.
				trackerType := ""
				statePath := config.TeamStatePath(state.Repo)
				repo := teamstate.NewRepo(state.Repo, statePath)
				if teamCfg, err := repo.LoadConfig(); err == nil {
					trackerType = teamCfg.Tracker.Type
				}
				form.AddTextView("",
					i18n.Tf("cmd.init.wizard_tracker_token_missing", strings.ToUpper(trackerType)),
					60, 2, true, false)
				form.AddPasswordField(
					i18n.Tf("cmd.init.wizard_tracker_token_label", strings.ToUpper(trackerType)),
					"", 0, '*',
					func(t string) { state.TrackerToken = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				if state.TrackerToken != "" && *a != nil && (*a).Secrets != nil {
					statePath := config.TeamStatePath(state.Repo)
					repo := teamstate.NewRepo(state.Repo, statePath)
					if teamCfg, err := repo.LoadConfig(); err == nil && teamCfg.Tracker.Type != "" {
						tokenKey := teamCfg.Tracker.TrackerTokenKey
						if tokenKey == "" {
							tokenKey = "openhub.tracker." + teamCfg.Tracker.Type + ".token"
						}
						if err := (*a).Secrets.Set(state.Ctx, tokenKey, state.TrackerToken); err != nil {
							return fmt.Errorf("storing tracker token: %w", err)
						}
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if state.TrackerToken != "" {
					return []views.InfoField{
						{Label: i18n.T("cmd.init.wizard_team_info_tracker"), Value: i18n.T("cmd.init.wizard_tracker_token_stored")},
					}
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_info_tracker"), Value: i18n.T("cmd.init.wizard_team_skipped")},
				}
			},
		},
	}
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
	steps := []views.WizardStep{
		// ── Team form: repo + identity ──
		{
			ID:    "team_form",
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
					i18n.T("cmd.init.wizard_team_repo"), state.Repo, 0, nil,
					func(t string) { state.Repo = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_member_id"), state.MemberID, 0, nil,
					func(t string) { state.MemberID = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_display_name"), state.DisplayName, 0, nil,
					func(t string) { state.DisplayName = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_team_info_repo"), Value: state.Repo},
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
			Processing: i18n.T("cmd.init.wizard_processing_team"),
			OnDone: func() error {
				displayName := state.DisplayName
				if displayName == "" {
					displayName = state.MemberID
				}
				err := teamInitCore(state.Ctx, *a, teamInitParams{
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
				if newApp, reloadErr := ReloadApp(); reloadErr != nil {
					slog.Warn("ReloadApp failed after team connect", "err", reloadErr)
				} else {
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
		// ── Init: Tracker setup (conditional) ──
	}
	steps = append(steps, buildTrackerSetupSteps(a, state)...)
	return steps
}

// buildInitWizardHTTPSCredSteps returns two WizardSteps for HTTPS credential
// configuration. They are used by the init and rejoin flows when the repo URL
// uses HTTPS.
//
// The returned steps are:
//  1. Auth mode selection — DropDown (provide/skip/public). Always shown for HTTPS.
//  2. Credentials — Username + Token. Skipped when auth mode is "skip" or "public".
//
// Both steps are conditionally skipped if the repo is not HTTPS.
//
// When the repo host is a GitLab instance, the token is also stored in the
// keychain (openhub.team.<teamID>.gitlab.token) so it can be reused for
// GitLab API calls (identity verification, tracker sync, etc.).
func buildInitWizardHTTPSCredSteps(a **app.App, state *initWizardTeamState, requiredMode string) []views.WizardStep {
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
			ID:    "team_cred_mode",
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
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_auth"), Value: state.CredAuthChoice}}
			},
		},
		// ── Step 2: Username + Token (skipped when not "provide") ──
		{
			ID:    "team_creds",
			Label: i18n.T("cmd.init.wizard_step_team_creds"),
			SkipIf: func() bool {
				return state.Skipped || state.Mode != requiredMode || !teamstate.IsHTTPS(state.Repo) || state.CredAuthChoice != "provide"
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.init.wizard_team_cred_username"), state.CredUsername, 0, nil,
					func(text string) { state.CredUsername = text })
				form.AddPasswordField(i18n.T("cmd.init.wizard_team_cred_token"), state.CredToken, 0, '*',
					func(text string) { state.CredToken = text })
				// Show a hint when the repo is on a GitLab host: this single token
				// will be used for both git access and GitLab API calls.
				if teamstate.IsGitLabHost(state.Repo) {
					form.AddTextView("", i18n.T("cmd.init.wizard_team_cred_gitlab_hint"), 60, 2, true, false)
				}
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
					// Store in git credential helper for clone/pull.
					if err := teamstate.ConfigureCredential(
						state.Ctx, state.Repo, state.CredUsername, state.CredToken,
					); err != nil {
						return err
					}
					// When the host is GitLab, also store in the keychain for API use.
					if teamstate.IsGitLabHost(state.Repo) && *a != nil && (*a).Secrets != nil {
						teamID := config.RepoNameFromRemote(state.Repo)
						if err := (*a).Secrets.Set(state.Ctx, config.TeamGitLabTokenKey(teamID), state.CredToken); err != nil {
							slog.Warn("failed to store GitLab token in keychain", "err", err)
						}
					}
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
	var steps []views.WizardStep
	steps = []views.WizardStep{
		// ── Rejoin step 1: Repo URL + fetch members ──
		{
			ID:    "rejoin_repo",
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
					i18n.T("cmd.init.wizard_team_repo"), state.Repo, 0, nil,
					func(t string) { state.Repo = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				repo, members, _, err := listTeamMembers(state.Ctx, state.Repo, "")
				if err != nil {
					return err
				}
				if len(members) == 0 {
					return fmt.Errorf("%s", i18n.T("cmd.init.wizard_rejoin_no_members"))
				}
				state.Members = members
				state.TeamRepo = repo
				return nil
			},
			Processing: i18n.T("cmd.init.wizard_processing_rejoin_clone"),
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_repo"), Value: state.Repo}}
			},
		},
	}

	// ── Rejoin steps 2-3: HTTPS Credentials (auth mode + username/token) ──
	steps = append(steps, buildInitWizardHTTPSCredSteps(a, state, "rejoin")...)

	// ── Rejoin step 4: Member selection ──
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_member",
		Label: i18n.T("cmd.init.wizard_step_rejoin_member"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin"
		},
		Required: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			// Build filterable items: "add new" action + existing members.
			items := []widgets.FilterItem{
				{
					MainText:  i18n.T("cmd.init.wizard_rejoin_add_member"),
					Reference: "__add_new__",
				},
			}
			for _, m := range state.Members {
				label := fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)
				secondary := ""
				if m.GitLabUsername != "" {
					secondary = i18n.Tf("cmd.init.wizard_rejoin_member_gitlab", m.GitLabUsername)
				}
				if m.Role != "" {
					if secondary != "" {
						secondary += " — "
					}
					secondary += m.Role
				}
				items = append(items, widgets.FilterItem{
					MainText:      label,
					SecondaryText: secondary,
					Reference:     m.ID,
				})
			}

			fl := widgets.NewFilterableList(items, func(item widgets.FilterItem) {
				if item.Reference == "__add_new__" {
					state.AddingNewMember = true
					state.MemberID = ""
				} else if id, ok := item.Reference.(string); ok {
					state.MemberID = id
					state.AddingNewMember = false
				}
				onDone()
			})
			fl.SetApp(tvApp)

			container.AddItem(fl, 0, 1, true)
			tvApp.SetFocus(fl)
		},
		InfoFields: func() []views.InfoField {
			if state.MemberID == "" && !state.AddingNewMember {
				return nil
			}
			if state.AddingNewMember {
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_rejoin_member_label"), Value: i18n.T("cmd.init.wizard_rejoin_add_member")},
				}
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

	// ── Rejoin step 4a: New member form (conditional) ──
	// Only shown when the user selected "Add a new member" in the previous step.
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_new_member",
		Label: i18n.T("cmd.init.wizard_step_rejoin_new_member"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin" || !state.AddingNewMember
		},
		Validate: func() string {
			if strings.TrimSpace(state.MemberID) == "" {
				return i18n.T("cmd.init.wizard_rejoin_new_member_id_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.init.wizard_rejoin_new_member_id"), state.MemberID, 0, nil,
				func(t string) { state.MemberID = t },
			)
			form.AddInputField(
				i18n.T("cmd.init.wizard_rejoin_new_member_display_name"), state.NewMemberDisplayName, 0, nil,
				func(t string) { state.NewMemberDisplayName = t },
			)
			form.AddInputField(
				i18n.T("cmd.init.wizard_rejoin_new_member_gitlab"), state.NewMemberGitLab, 0, nil,
				func(t string) { state.NewMemberGitLab = t },
			)
			roles := []string{"dev", "lead", "reviewer"}
			roleIdx := 0
			for idx, r := range roles {
				if r == state.NewMemberRole {
					roleIdx = idx
					break
				}
			}
			form.AddDropDown(
				i18n.T("cmd.init.wizard_rejoin_new_member_role"), roles, roleIdx,
				func(_ string, idx int) {
					if idx >= 0 && idx < len(roles) {
						state.NewMemberRole = roles[idx]
					}
				},
			)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			displayName := state.NewMemberDisplayName
			if displayName == "" {
				displayName = state.MemberID
			}
			role := state.NewMemberRole
			if role == "" {
				role = "dev"
			}
			member := teamstate.Member{
				ID:             state.MemberID,
				DisplayName:    displayName,
				GitLabUsername: state.NewMemberGitLab,
				Role:           role,
			}
			if state.TeamRepo == nil {
				return fmt.Errorf("team-state repository not available")
			}
			if err := state.TeamRepo.AddMember(state.Ctx, member); err != nil {
				return fmt.Errorf("adding member: %w", err)
			}
			// Update members list with the new member.
			state.Members = append(state.Members, member)
			state.AddingNewMember = false
			return nil
		},
		Processing: i18n.T("cmd.init.wizard_rejoin_new_member_processing"),
		InfoFields: func() []views.InfoField {
			if state.MemberID == "" {
				return nil
			}
			display := state.NewMemberDisplayName
			if display == "" {
				display = state.MemberID
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_rejoin_member_label"), Value: fmt.Sprintf("%s (%s)", display, state.MemberID)},
			}
		},
	})

	// ── Rejoin step 4b: GitLab token for SSH repos (conditional) ──
	// When the repo uses HTTPS, the token was already stored in the keychain
	// by buildInitWizardHTTPSCredSteps. This step only appears for SSH repos
	// where the member has a gitlab_username and identity verification is needed.
	steps = append(steps, views.WizardStep{
		ID:    "rejoin_gitlab_token",
		Label: i18n.T("cmd.init.wizard_step_rejoin_gitlab_token"),
		SkipIf: func() bool {
			return state.Skipped || state.Mode != "rejoin" ||
				selectedMemberGitLabUsername(state) == "" ||
				teamstate.IsHTTPS(state.Repo) // HTTPS flow already stores the token
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
			teamID := config.RepoNameFromRemote(state.Repo)
			source := gitlabTokenSource(state.Ctx, *a, teamID)

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
				teamID := config.RepoNameFromRemote(state.Repo)
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
		OnDone: func() error {
			result, err := teamRejoinCore(state.Ctx, *a, teamRejoinParams{
				StateRepo: state.Repo,
				MemberID:  state.MemberID,
			})
			if err != nil {
				return err
			}

			// Store identity mismatch for the next conditional step.
			state.IdentityMismatch = result.IdentityMismatch

			state.Configured = true
			state.TeamID = result.TeamID

			// Retro-tag sessions synchronously (wizard shows spinner)
			if tagged, tagErr := retroTagSessions(state.Ctx, state.MemberID); tagErr != nil {
				slog.Warn("retro-tagging sessions failed", "err", tagErr)
			} else if tagged > 0 {
				slog.Info("retro-tagged sessions", "count", tagged)
			}

			// Reload app so subsequent steps see the team
			config.Reset()
			if newApp, reloadErr := ReloadApp(); reloadErr != nil {
				slog.Warn("ReloadApp failed after team rejoin", "err", reloadErr)
			} else {
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
				teamID := config.RepoNameFromRemote(state.Repo)
				if err := (*a).Secrets.Set(state.Ctx, config.TeamGitLabTokenKey(teamID), state.MismatchPAT); err != nil {
					return fmt.Errorf("storing token: %w", err)
				}
			}
			// Re-run identity validation with the new token
			statePath := config.TeamStatePath(state.Repo)
			repo := teamstate.NewRepo(state.Repo, statePath)
			member, err := repo.GetMember(state.MemberID)
			if err != nil {
				return fmt.Errorf("member lookup: %w", err)
			}
			if err := validateGitLabIdentity(state.Ctx, *a, repo, member); err != nil {
				return fmt.Errorf("identity verification: %w", err)
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
	steps = append(steps, buildTrackerSetupSteps(a, state)...)

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
			} else {
				// ── Prerequisites (only when no existing team) ──
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
				if state.TeamChoice == "keep" {
					defaultIdx = 1
				} else if state.TeamChoice == "reconfigure" {
					defaultIdx = 2
				} else if state.TeamChoice == "skip" {
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
						go func() { tvApp.QueueUpdateDraw(func() { rebuildButtons() }) }()
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

// buildProjectTeamStep returns a WizardStep that collects team configuration
// for a project during project creation or reconfiguration.
//
// The step sets out to:
//   - &activeTeamID → attach the project to the hub's active team
//   - nil           → no team for this project
func buildProjectTeamStep(a *app.App, out **string, solo *bool) views.WizardStep {
	hubTeam := a.Config.ActiveTeam()
	hasTeam := hubTeam.Enabled && hubTeam.ID != ""

	// Choices: hub team (default when there is one), solo space (P2-T16),
	// no team.
	const (
		choiceTeam = "team"
		choiceSolo = "solo"
		choiceNone = "none"
	)
	var choices []string
	if hasTeam {
		choices = append(choices, choiceTeam)
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

	return views.WizardStep{
		Label: i18n.T("form.project.team_label"),
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			if hasTeam {
				hubSummary := i18n.Tf("form.project.team_hub_summary", hubTeam.StateRepo, hubTeam.MemberID)
				form.AddTextView(i18n.T("form.project.team_hub_label"), hubSummary, 0, 1, false, false)
			} else {
				form.AddTextView(i18n.T("form.project.team_hub_label"), i18n.T("form.project.team_none_configured"), 0, 1, false, false)
			}
			var labels []string
			initial := 0
			for i, c := range choices {
				switch c {
				case choiceTeam:
					labels = append(labels, i18n.Tf("form.project.team_use_hub", hubTeam.MemberID))
				case choiceSolo:
					labels = append(labels, soloLabel)
				default:
					labels = append(labels, i18n.T("form.project.team_no_team"))
				}
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
			if solo != nil {
				*solo = choice == choiceSolo
			}
			if choice == choiceTeam {
				id := hubTeam.ID
				*out = &id
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			value := i18n.T("form.project.team_value_none")
			switch choice {
			case choiceTeam:
				value = hubTeam.ID
			case choiceSolo:
				value = soloLabel
			}
			return []views.InfoField{{Label: i18n.T("form.project.team_label"), Value: value}}
		},
	}
}
