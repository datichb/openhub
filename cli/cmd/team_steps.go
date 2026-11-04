package cmd

import (
	"context"
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
	MemberID           string
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
	s.HasConfig = repo.HasConfig()
	s.HasPolicies = repo.HasPolicies()
	s.ExistingCfg, _ = repo.LoadConfig(s.Ctx)
	if s.HasConfig && s.ExistingCfg != nil {
		s.StaleDaysStr = strconv.Itoa(s.ExistingCfg.Takeover.StaleDays)
		s.WebhookURL = s.ExistingCfg.Notification.MattermostWebhook
		s.Channel = s.ExistingCfg.Notification.Channel
		s.BotName = s.ExistingCfg.Notification.BotName
	}
	s.HasMember = s.MemberID != "" && repo.HasMember(s.Ctx, s.MemberID)
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
			if s.HasConfig && s.ExistingCfg != nil {
				if days == s.ExistingCfg.Takeover.StaleDays {
					return nil
				}
				s.ExistingCfg.Takeover.StaleDays = days
				return s.Repo.SaveConfig(s.Ctx, s.ExistingCfg)
			}
			cfg := &teamstate.TeamConfig{
				Notification: teamstate.NotificationConfig{
					Enabled: false,
					BotName: "OpenHub",
				},
				Takeover: teamstate.TakeoverConfig{StaleDays: days},
				Parallel: teamstate.ParallelConfig{
					MaxSessions:            5,
					MaxBudgetMinutes:       180,
					DefaultTicketWeightMin: 60,
					PortRangeStart:         4100,
					AutoMergeBeads:         true,
					MaxRetries:             2,
					RetryDelaySeconds:      5,
				},
			}
			return s.Repo.SaveConfig(s.Ctx, cfg)
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
		Validate: func() string {
			var members []teamstate.Member
			if s.Repo != nil {
				members, _ = s.Repo.ListMembers(s.Ctx)
				if members == nil {
					members = []teamstate.Member{}
				}
			}
			return s.identityFields().problem(members, s.ownID(), "v532m.join.id_taken_other")
		},
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
			_, err := s.Repo.JoinMember(s.Ctx, m, teamstate.JoinModeFor(s.ownID(), m.ID, false))
			return joinRefusalError(err, "v532m.join.id_taken_other")
		},
		ErrorActions: func(err error) []views.ErrorAction {
			if !isJoinRefusal(err) {
				return nil
			}
			return []views.ErrorAction{
				{Label: i18n.T("v532w.error.retry"), Retry: true},
				{Label: i18n.T("v532m.join.action_change"), GoToStep: "team_identity"},
			}
		},
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
			form.AddInputField(
				i18n.T("cmd.team.init.notif_webhook"),
				s.WebhookURL, 0, nil,
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
			cfg, err := s.Repo.LoadConfig(s.Ctx)
			if err != nil {
				return err
			}
			if cfg.Notification.MattermostWebhook == s.WebhookURL &&
				cfg.Notification.Channel == s.Channel &&
				cfg.Notification.BotName == s.BotName {
				return nil
			}
			cfg.Notification.MattermostWebhook = s.WebhookURL
			cfg.Notification.Channel = s.Channel
			cfg.Notification.BotName = s.BotName
			cfg.Notification.Enabled = true
			return s.Repo.SaveConfig(s.Ctx, cfg)
		},
		InfoFields: func() []views.InfoField {
			if s.WebhookURL == "" {
				return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_notifs"), Value: i18n.T("cmd.init.wizard_team_info_skipped")}}
			}
			return []views.InfoField{
				{Label: i18n.T("cmd.init.wizard_team_info_webhook"), Value: s.WebhookURL},
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
