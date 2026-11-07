package cmd

import (
	"context"
	"errors"
	"strconv"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	policysvc "github.com/datichb/openhub/cli/internal/services/policy"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Shared team step builders (used by team.go CLI wizard and tui_team_actions.go TUI wizard)
// ─────────────────────────────────────────────────────────────────────────────

// teamStepStore is what the steps of the team wizard ask of the team-state:
// the configuration, the members, the policies and a commit of their own.
type teamStepStore interface {
	teamstate.RepoInfo
	teamstate.RepoSync
	teamstate.ConfigReader
	teamstate.ConfigWriter
	teamstate.MemberReader
	teamstate.MemberWriter
	teamstate.MemberJoiner
	teamstate.MemberPatcher
	teamstate.PolicyReader
	teamstate.PolicyWriter
}

// teamStepState holds mutable state shared across the 4 team config wizard steps.
// Callers populate input fields before step construction; the builders mutate
// them via closures. Post-wizard code can read the final state from the struct.
type teamStepState struct {
	// Context for git operations (set by caller)
	Ctx context.Context

	// Repo reference (may be nil initially; assigned by step 0)
	Repo teamStepStore

	// Pre-loaded state
	HasConfig   bool
	HasPolicies bool
	ExistingCfg *teamstate.TeamConfig
	HasMember   bool

	// Config step
	StaleDaysStr string

	// Identity step
	MemberID string
	// SuggestedID: MemberID is only a suggestion (who the person is in
	// another team), not the card they already have here: that one is OwnID,
	// the member_id hub.toml has for this repository. A creation never takes
	// the card of a colleague for the person's own.
	SuggestedID bool
	OwnID       string
	// Claim is « C'est moi »: the card with the ID exists and is the
	// person's (identityErrorActions).
	Claim bool
	// OnJoined runs once the card is in the team-state: the assistant writes
	// hub.toml there, so that an interrupted wizard does not leave a card in
	// a team hub.toml does not know.
	OnJoined func(memberID string) error
	// AccessActions are the buttons of the access (teamAccess.ErrorActions)
	// the error screens of the identity step add to their own; nil: none.
	AccessActions      func(error) []views.ErrorAction
	DisplayName        string
	GitLabUsername     string
	TrackerUsername    string
	MattermostUsername string
	Role               string

	// Notifications step
	WebhookURL string
	Channel    string
	BotName    string

	// Policies step
	SelectedPolicies []string
}

// useRepo points the steps at a team-state that is available only now (the
// clone of step 0) and pre-fills them from what it already holds: the config,
// the policies and the profile of the member.
func (s *teamStepState) useRepo(repo teamStepStore) {
	s.Repo = repo
	s.HasConfig = repo.HasConfig(s.Ctx)
	s.HasPolicies = repo.HasPolicies(s.Ctx)
	s.ExistingCfg, _ = repo.LoadConfig(s.Ctx)
	if s.HasConfig && s.ExistingCfg != nil {
		s.StaleDaysStr = strconv.Itoa(s.ExistingCfg.Takeover.StaleDays)
		s.WebhookURL = s.ExistingCfg.Notification.EffectiveURL()
		s.Channel = s.ExistingCfg.Notification.Channel
		s.BotName = s.ExistingCfg.Notification.BotName
	}
	own := s.MemberID
	if s.SuggestedID {
		own = s.OwnID
	}
	s.HasMember = own != "" && own == s.MemberID && repo.HasMember(s.Ctx, s.MemberID)
	if s.HasMember {
		if m, err := repo.GetMember(s.Ctx, s.MemberID); err == nil && m != nil {
			s.DisplayName = m.DisplayName
			s.GitLabUsername = m.GitLabUsername
			s.TrackerUsername = m.TrackerUsername
			s.MattermostUsername = m.MattermostUsername
			s.Role = m.Role
		}
	}
}

// teamStepOpts configures optional per-caller overrides for the shared builders.
type teamStepOpts struct {
	// ExtraSkipIf is prepended to the default SkipIf (repo == nil).
	// Return true to skip the step.
	ExtraSkipIf func() bool
}

// shouldSkip returns true if the step should be skipped (repo nil or extra condition).
func (o teamStepOpts) shouldSkip(s *teamStepState) bool {
	if o.ExtraSkipIf != nil && o.ExtraSkipIf() {
		return true
	}
	return s.Repo == nil
}

// ─── Config step ─────────────────────────────────────────────────────────────

func buildTeamConfigStep(s *teamStepState, opts teamStepOpts) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_config"),
		Processing: i18n.T("cmd.team.init.processing_config"),
		SkipIf:     func() bool { return opts.shouldSkip(s) },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.config_stale_days"),
				s.StaleDaysStr, 0, nil,
				func(text string) { s.StaleDaysStr = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			days, err := strconv.Atoi(s.StaleDaysStr)
			if err != nil || days <= 0 {
				days = 3
			}
			if s.HasConfig && s.ExistingCfg != nil && days == s.ExistingCfg.Takeover.StaleDays {
				return nil
			}
			// The one setting of the step, on top of the latest config.toml:
			// what the others changed in it meanwhile is kept. A team without
			// config.toml gets the defaults of a new team around it.
			return s.Repo.UpdateConfig(teamstate.WithActor(s.Ctx, s.MemberID), func(cfg *teamstate.TeamConfig) error {
				cfg.Takeover.StaleDays = days
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_stale_days"), Value: s.StaleDaysStr}}
		},
	}
}

// ─── Identity step (unified: new + existing member) ──────────────────────────

// identityFields are the fields of the identity form of the configuration of
// the team (oh team init, « Initialiser l'équipe »): the form of every
// assistant, plus the Mattermost and tracker names. The person configuring the
// team creates it: lead by default.
func (s *teamStepState) identityFields() identityFields {
	return identityFields{ID: &s.MemberID, DisplayName: &s.DisplayName, GitLab: &s.GitLabUsername, Role: &s.Role,
		Mattermost: &s.MattermostUsername, Tracker: &s.TrackerUsername, LockID: s.HasMember, DefaultRole: creatorRole}
}

// identityProblem is why the identity form cannot be submitted, "" when it
// can: the format of the fields, and a GitLab handle that is a colleague's. A
// taken ID is not refused here: the step refuses it with the buttons that
// recover from it (« C'est moi » attaches the card that exists).
func (s *teamStepState) identityProblem() string {
	f := s.identityFields()
	if msg := f.problem(nil, "", ""); msg != "" || s.Repo == nil || s.Claim {
		return msg
	}
	members, _ := s.Repo.ListMembers(s.Ctx)
	if err := teamstate.CheckJoin(members, f.member(), teamstate.JoinModeFor(s.ownID(), f.member().ID, false)); err != nil && !errors.Is(err, teamstate.ErrMemberExists) {
		return joinRefusalText(err, "v532m.join.id_taken_hint")
	}
	return ""
}

// accessActions are the buttons of the access, when the assistant has them.
func (s *teamStepState) accessActions(err error) []views.ErrorAction {
	if s.AccessActions == nil {
		return nil
	}
	return s.AccessActions(err)
}

// ownID is the card the person already has in this team: the member_id
// hub.toml has, when the team-state holds that card.
func (s *teamStepState) ownID() string {
	if s.HasMember {
		return s.MemberID
	}
	return ""
}

func buildTeamIdentityStep(s *teamStepState, opts teamStepOpts) views.WizardStep {
	return views.WizardStep{
		ID:         "team_identity",
		Label:      i18n.T("cmd.team.init.step_identity"),
		Processing: i18n.T("cmd.team.init.processing_identity"),
		Required:   true,
		SkipIf:     func() bool { return opts.shouldSkip(s) },
		Validate:   s.identityProblem,
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			s.identityFields().addTo(form)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			// One transaction on top of the remote: the ID and the handle are
			// checked against the latest members, the card of a colleague is
			// never replaced, and the card of the person is changed where the
			// form shows it (what it does not show - default_mode, fields
			// added later - is kept).
			f := s.identityFields()
			m := f.member()
			s.MemberID = m.ID
			_, err := s.Repo.JoinMember(s.Ctx, m, teamstate.JoinModeFor(s.ownID(), m.ID, s.Claim))
			if err != nil {
				return joinRefusalError(err, "v532m.join.id_taken_hint")
			}
			s.Claim = false
			if s.OnJoined != nil {
				return s.OnJoined(m.ID)
			}
			return nil
		},
		ErrorActions: identityErrorActions(&s.Claim, "team_identity", "team_identity", s.accessActions),
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_team_info_member_id"), Value: s.MemberID},
				{Label: i18n.T("cmd.init.wizard_team_info_name"), Value: s.DisplayName},
				{Label: i18n.T("cmd.init.wizard_team_info_role"), Value: s.Role},
			}
		},
	}
}

// ─── Notifications step ──────────────────────────────────────────────────────

func buildTeamNotifStep(s *teamStepState, opts teamStepOpts) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_notifications"),
		Processing: i18n.T("cmd.team.init.processing_notifications"),
		SkipIf:     func() bool { return opts.shouldSkip(s) },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddPasswordField(
				i18n.T("cmd.team.init.notif_webhook"),
				s.WebhookURL, 0, '•',
				func(text string) { s.WebhookURL = text })
			form.AddInputField(
				i18n.T("cmd.team.init.notif_channel"),
				s.Channel, 0, nil,
				func(text string) { s.Channel = text })
			form.AddInputField(
				i18n.T("cmd.team.init.notif_bot_name"),
				s.BotName, 0, nil,
				func(text string) { s.BotName = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			if s.WebhookURL == "" {
				return nil
			}
			return s.Repo.UpdateConfig(teamstate.WithActor(s.Ctx, s.MemberID), func(cfg *teamstate.TeamConfig) error {
				cfg.Notification.WebhookURL = s.WebhookURL
				cfg.Notification.Type = teamstate.InferNotifierType(s.WebhookURL)
				cfg.Notification.MattermostWebhook = ""
				cfg.Notification.Channel = s.Channel
				cfg.Notification.BotName = s.BotName
				cfg.Notification.Enabled = true
				return nil
			})
		},
		InfoFields: func() []views.InfoField {
			if s.WebhookURL == "" {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_notifs"), Value: i18n.T("cmd.init.wizard_team_info_skipped")}}
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_team_info_webhook"), Value: teamstate.MaskWebhook(s.WebhookURL)},
				{Label: i18n.T("cmd.init.wizard_team_info_channel"), Value: s.Channel},
				{Label: i18n.T("cmd.init.wizard_team_info_bot"), Value: s.BotName},
			}
		},
	}
}

// ─── Policies step ───────────────────────────────────────────────────────────

func buildTeamPoliciesStep(s *teamStepState, opts teamStepOpts) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_policies"),
		Processing: i18n.T("cmd.team.init.processing_policies"),
		SkipIf:     func() bool { return opts.shouldSkip(s) },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			branchNaming := false
			commitFormat := false
			maxWip := false
			reviewRequired := false
			form.AddCheckbox(i18n.T("cmd.team.init.policies_branch_naming"), false,
				func(checked bool) { branchNaming = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_commit_format"), false,
				func(checked bool) { commitFormat = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_max_wip"), false,
				func(checked bool) { maxWip = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_review_required"), false,
				func(checked bool) { reviewRequired = checked })
			form.AddButton(i18n.T("wizard.hint.submit"), func() {
				s.SelectedPolicies = nil
				if branchNaming {
					s.SelectedPolicies = append(s.SelectedPolicies, "branch_naming")
				}
				if commitFormat {
					s.SelectedPolicies = append(s.SelectedPolicies, "commit_format")
				}
				if maxWip {
					s.SelectedPolicies = append(s.SelectedPolicies, "max_ticket_wip")
				}
				if reviewRequired {
					s.SelectedPolicies = append(s.SelectedPolicies, "review_required")
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			if len(s.SelectedPolicies) == 0 {
				return nil
			}
			// The service merges them with the existing policies (the
			// recommended ones win on a name clash) and commits once.
			return policysvc.New(s.Repo).Upsert(s.Ctx, buildRecommendedPolicies(s.SelectedPolicies))
		},
		InfoFields: func() []views.InfoField {
			if len(s.SelectedPolicies) == 0 {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_policies"), Value: i18n.T("cmd.init.wizard_team_info_policies_none")}}
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_team_info_policies"), Value: i18n.Tf("cmd.init.wizard_team_info_policies_active", len(s.SelectedPolicies))},
			}
		},
	}
}
