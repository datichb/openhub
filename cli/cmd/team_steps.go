package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Shared team step builders (used by team.go CLI wizard and tui_team_actions.go TUI wizard)
// ─────────────────────────────────────────────────────────────────────────────

// teamStepState holds mutable state shared across the 4 team config wizard steps.
// Callers populate input fields before step construction; the builders mutate
// them via closures. Post-wizard code can read the final state from the struct.
type teamStepState struct {
	// Context for git operations (set by caller)
	Ctx context.Context

	// Repo reference (may be nil initially; assigned by step 0)
	Repo *teamstate.Repo

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

func buildTeamIdentityStep(s *teamStepState, opts teamStepOpts) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_identity"),
		Processing: i18n.T("cmd.team.init.processing_identity"),
		Required:   true,
		SkipIf:     func() bool { return opts.shouldSkip(s) },
		Validate: func() string {
			if strings.TrimSpace(s.MemberID) == "" {
				return i18n.T("cmd.team.init.validate.member_id_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			if !s.HasMember {
				form.AddInputField(
					i18n.T("cmd.team.init.identity_id"),
					s.MemberID, 0, nil,
					func(text string) { s.MemberID = text })
			}
			form.AddInputField(
				i18n.T("cmd.team.init.identity_display"),
				s.DisplayName, 0, nil,
				func(text string) { s.DisplayName = text })
			form.AddInputField(
				i18n.T("cmd.team.init.identity_gitlab"),
				s.GitLabUsername, 0, nil,
				func(text string) { s.GitLabUsername = text })
			form.AddInputField(
				i18n.T("cmd.team.init.identity_mattermost"),
				s.MattermostUsername, 0, nil,
				func(text string) { s.MattermostUsername = text })
			form.AddInputField(
				i18n.T("tui.team.identity_tracker_username"),
				s.TrackerUsername, 0, nil,
				func(text string) { s.TrackerUsername = text })
			roles := []string{"lead", "dev", "reviewer"}
			roleLabels := []string{
				i18n.T("cmd.init.wizard_team_role_lead"),
				i18n.T("cmd.init.wizard_team_role_dev"),
				i18n.T("cmd.init.wizard_team_role_reviewer"),
			}
			roleIdx := 0
			for i, r := range roles {
				if r == s.Role {
					roleIdx = i
					break
				}
			}
			form.AddDropDown(
				i18n.T("cmd.team.init.identity_role"),
				roleLabels, roleIdx,
				func(_ string, idx int) { s.Role = roles[idx] })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			member := teamstate.Member{
				ID:                 s.MemberID,
				DisplayName:        s.DisplayName,
				GitLabUsername:     s.GitLabUsername,
				TrackerUsername:    s.TrackerUsername,
				MattermostUsername: s.MattermostUsername,
				Role:               s.Role,
				DefaultMode:        "semi-auto",
			}
			if s.HasMember {
				if err := s.Repo.UpdateMember(s.Ctx, member); err != nil {
					return err
				}
				return s.Repo.CommitAndPush(s.Ctx, fmt.Sprintf("team: update member %s", s.MemberID), "members.toml")
			}
			if err := s.Repo.AddMember(s.Ctx, member); err != nil {
				if err == teamstate.ErrMemberExists {
					return nil
				}
				return err
			}
			return s.Repo.CommitAndPush(s.Ctx, fmt.Sprintf("team: add member %s", s.MemberID), "members.toml")
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
			cfg, err := s.Repo.LoadConfig()
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
			policies := buildRecommendedPolicies(s.SelectedPolicies)
			if s.HasPolicies {
				existing, err := s.Repo.LoadPolicies("")
				if err != nil {
					return fmt.Errorf("loading existing policies: %w", err)
				}
				for _, ep := range existing {
					if _, ok := policies[ep.Name]; !ok {
						policies[ep.Name] = ep
					}
				}
			}
			if err := s.Repo.SavePolicies(s.Ctx, policies); err != nil {
				return err
			}
			commitMsg := "team: init policies"
			if s.HasPolicies {
				commitMsg = "team: update policies"
			}
			return s.Repo.CommitAndPush(s.Ctx, commitMsg, "policies.toml")
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
