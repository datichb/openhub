package views

import (
	"context"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// leaveShell records the windows with actions the view opens.
type leaveShell struct {
	askShell
	actions [][]ModalAction
	bodies  []string
}

func (s *leaveShell) ShowScrollableModal(_, body string, actions []ModalAction) {
	s.bodies = append(s.bodies, body)
	s.actions = append(s.actions, actions)
}

func (s *leaveShell) press(t *testing.T, label string) {
	t.Helper()
	require.NotEmpty(t, s.actions)
	last := s.actions[len(s.actions)-1]
	for _, a := range last {
		if a.Label == label {
			a.Callback()
			return
		}
	}
	t.Fatalf("no button %q among %v", label, last)
}

// selectField puts the cursor of the mounted view on the field with the key.
func (v *TeamDetailView) selectField(t *testing.T, key string) {
	t.Helper()
	for i, item := range v.list.GetItems() {
		if ref, ok := item.Reference.(int); ok && v.fields[ref].Key == key {
			v.list.SelectIndex(i)
			return
		}
	}
	t.Fatalf("no field %q on screen", key)
}

// The notifications have their switch, a hidden webhook, and a type that follows the webhook.
func TestTeamDetailNotificationsHaveTheirSwitchAndAHiddenWebhook(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.Mount(tview.NewFlex(), nil)
	on := e.field(t, "notif_enabled")
	assert.Equal(t, ScopeTeamShared, on.Scope)
	on.Set("true")
	assert.True(t, e.v.teamCfg.Notification.Enabled)
	assert.True(t, e.v.dirtyTeam)

	hook := e.field(t, fieldWebhookURL)
	assert.Equal(t, CfgFieldPassword, hook.Kind, "typed in a window that hides it")
	assert.Empty(t, hook.Source())

	e.v.selectField(t, fieldWebhookURL)
	e.v.editSelected()
	require.Len(t, e.sh.modals, 1)
	assert.Equal(t, "password", e.sh.modals[0].kind)
	e.sh.modals[0].fn("https://hooks.slack.com/services/T0/B0/verysecretvalue")
	assert.Equal(t, "https://hooks.slack.com/services/T0/B0/verysecretvalue", e.v.teamCfg.Notification.WebhookURL)
	assert.Equal(t, "slack", e.v.teamCfg.Notification.Type, "the type follows the webhook")

	hook = e.field(t, fieldWebhookURL)
	assert.NotContains(t, hook.Source(), "verysecretvalue")
	assert.Contains(t, hook.Source(), "hooks.slack.com")
	for _, item := range configItems(e.v.fields, 28) {
		assert.NotContains(t, item.MainText+item.SecondaryText, "verysecretvalue", "never on screen")
	}

	e.sh.modals = nil
	e.v.selectField(t, fieldWebhookURL)
	e.v.editSelected()
	e.sh.modals[0].fn("not a url")
	assert.Contains(t, e.sh.lastToast(), "https://")
	e.sh.modals = nil
	e.v.selectField(t, fieldWebhookURL)
	e.v.editSelected()
	e.sh.modals[0].fn("")
	assert.Equal(t, "slack", e.v.teamCfg.Notification.Type, "an empty answer keeps the webhook")
}

// A former mattermost_webhook is the webhook in force.
func TestTeamDetailShowsTheFormerWebhookKey(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.teamCfg.Notification.MattermostWebhook = "https://chat.example.com/hooks/abcdef"
	e.v.buildFields()
	kind := e.field(t, "notif_type")
	assert.Equal(t, "mattermost", kind.Get())
	assert.Contains(t, e.field(t, fieldWebhookURL).Source(), "chat.example.com")
}

// An empty entry puts the default back (and not 0, which the bounds refuse).
func TestTeamDetailEmptyEntryPutsTheDefaultBack(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	stale := e.field(t, "stale_days")
	stale.Set("9")
	assert.Equal(t, 9, e.v.teamCfg.Takeover.StaleDays)
	stale.Set("")
	assert.Equal(t, 3, e.v.teamCfg.Takeover.StaleDays)
	assert.Equal(t, "3", stale.Get())
	assert.Error(t, stale.Validator.Validate("31"))
	assert.Error(t, stale.Validator.Validate("0"))
}

func TestTeamDetailLimitsAndTicketPattern(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.field(t, "recommended_session_budget_usd").Set("2.5")
	e.field(t, "recommended_models").Set("eu.*, us.*")
	assert.Equal(t, 2.5, e.v.teamCfg.Limits.Recommended.SessionBudgetUSD)
	assert.Equal(t, []string{"eu.*", "us.*"}, e.v.teamCfg.Limits.Recommended.Models)
	budget := e.field(t, "recommended_daily_budget_usd")
	assert.Error(t, budget.Validator.Validate("-1"))
	assert.NoError(t, budget.Validator.Validate(""))

	pattern := e.field(t, "ticket_pattern")
	assert.NoError(t, pattern.Validator.Validate(`SRU-(\d+)`))
	assert.Error(t, pattern.Validator.Validate(`SRU-\d+`), "one capture group")
	assert.Error(t, pattern.Validator.Validate(`SRU-(`))
	pattern.Set(`SRU-(\d+)`)
	assert.Equal(t, `SRU-(\d+)`, e.v.teamCfg.Tracker.TicketPattern)
}

// The enforced settings are for the leads: the others see them locked, with the reason.
func TestTeamDetailEnforcedSettingsAreLockedForAMemberWhoIsNotALead(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.Mount(tview.NewFlex(), nil)
	enforced := e.field(t, "enforced_session_budget_usd")
	pushEnforced := e.field(t, "push_labels_enforced")
	cfgPolicy := e.field(t, "governance.config")
	e.v.canEnforce = true
	assert.False(t, enforced.Locked())
	e.v.canEnforce = false
	for _, f := range []configField{enforced, pushEnforced, cfgPolicy} {
		assert.True(t, f.Locked(), f.Key)
	}
	e.v.selectField(t, "push_labels_enforced")
	e.v.editSelected()
	assert.Contains(t, e.sh.lastToast(), "lead")
	assert.False(t, e.v.teamCfg.Tracker.IsPushLabelsEnforced())
	// a recommendation is free
	assert.False(t, e.field(t, "recommended_session_budget_usd").Locked != nil)
}

// A personal edit is written to the section of this team by itself, and the
// default [tracker] of hub.toml is not touched.
func TestTeamDetailPersonalEditsAreAutoSavedInTheTeamSection(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	teamTracker := config.TrackerLocalConfig{}
	e.v.cfg.GetTeamTrackerConfig = func() config.TrackerLocalConfig { return teamTracker }
	e.v.loadData()
	e.v.buildFields()

	e.field(t, "my_auto_plan_assigned").Set("true")
	assert.True(t, e.v.dirtyLocal)
	assert.False(t, e.v.dirtyTeam, "it is not part of the draft")
	e.v.write(false)
	got, err := config.Read()
	require.NoError(t, err)
	require.NotNil(t, got.TeamTracker("core").AutoPlanAssigned)
	assert.True(t, *got.TeamTracker("core").AutoPlanAssigned)
	assert.Nil(t, got.Tracker.AutoPlanAssigned, "the default of every team is untouched")
}

// What is shared is a draft kept when the view is left for a link, put back
// on the fresh config, dropped when a colleague changed the same setting.
func TestTeamDetailKeepsTheDraftAcrossAVisitToAnotherView(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.Mount(tview.NewFlex(), nil)
	e.field(t, "tracker_project").Set("acme/app")
	require.True(t, e.v.dirtyTeam)
	e.v.Unmount()
	require.NotNil(t, e.v.kept)
	assert.Contains(t, e.sh.lastToast(), "gardées")

	e.v.SetShell(e.sh)
	e.v.Mount(tview.NewFlex(), nil)
	assert.True(t, e.v.dirtyTeam, "the draft is back")
	assert.Equal(t, "acme/app", e.v.teamCfg.Tracker.TrackerProject)
	assert.Nil(t, e.v.kept)

	// a colleague changed the same setting meanwhile: the draft is dropped, said
	e.v.Unmount()
	require.NotNil(t, e.v.kept)
	tc := e.v.cfg.ResolveTeam()
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error { c.Tracker.TrackerProject = "other/app"; return nil }))
	e.v.Mount(tview.NewFlex(), nil)
	assert.False(t, e.v.dirtyTeam)
	assert.Equal(t, "other/app", e.v.teamCfg.Tracker.TrackerProject)
	assert.Contains(t, e.sh.lastToast(), "tracker.tracker_project")

	// a draft of another team is not carried over
	e.v.kept = &keptDraft{teamID: "elsewhere", base: &teamstate.TeamConfig{}, draft: &teamstate.TeamConfig{}}
	e.v.Mount(tview.NewFlex(), nil)
	assert.Nil(t, e.v.kept)
	assert.False(t, e.v.dirtyTeam)
}

// Esc with a draft asks: publish and leave, drop and leave, stay.
func TestTeamDetailEscAsksBeforeLeavingWithADraft(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	ms := &leaveShell{askShell: *e.sh}
	e.v.SetShell(ms)
	left := 0
	proceed := func() { left++ }

	assert.False(t, e.v.GuardLeave(proceed), "nothing to ask without a draft")

	e.field(t, "tracker_project").Set("acme/app")
	require.True(t, e.v.GuardLeave(proceed))
	assert.Contains(t, ms.bodies[0], "tracker.tracker_project")
	ms.press(t, i18n.T("v54c.leave.stay"))
	assert.Zero(t, left)
	assert.True(t, e.v.dirtyTeam)

	require.True(t, e.v.GuardLeave(proceed))
	ms.press(t, i18n.T("v54c.leave.drop"))
	assert.Equal(t, 1, left)
	assert.False(t, e.v.dirtyTeam)

	e.field(t, "tracker_project").Set("acme/app")
	require.True(t, e.v.GuardLeave(proceed))
	ms.press(t, i18n.T("v54c.leave.publish"))
	require.NotNil(t, e.saved, "published")
	assert.Equal(t, "acme/app", e.saved.Tracker.TrackerProject)
	assert.Equal(t, 2, left, "and left once published")
}

func TestTeamDetailPublishFailureDoesNotLeave(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	ms := &leaveShell{askShell: *e.sh}
	e.v.SetShell(ms)
	e.v.cfg.SaveTeamConfig = func() TeamConfigWriter {
		return func(context.Context, *teamstate.TeamConfig, *teamstate.TeamConfig) error {
			return &teamstate.ErrNotLead{Member: "dan", Keys: []string{"tracker.push_labels_enforced"}}
		}
	}
	left := 0
	e.field(t, "tracker_project").Set("acme/app")
	require.True(t, e.v.GuardLeave(func() { left++ }))
	ms.press(t, i18n.T("v54c.leave.publish"))
	assert.Zero(t, left)
	assert.True(t, e.v.dirtyTeam)
	assert.Contains(t, ms.lastToast(), "lead")
}

func TestTeamDetailOpenSectionAndEmptyStateNewTeam(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.Mount(tview.NewFlex(), nil)
	e.v.OpenSection(TeamSectionNotifications)
	_, item, ok := e.v.list.CurrentItem()
	require.True(t, ok)
	f := e.v.fields[item.Reference.(int)]
	assert.Equal(t, "notif_enabled", f.Key, "the cursor is on the first line of the section")
	e.v.OpenSection("nope")
	_, item, _ = e.v.list.CurrentItem()
	assert.Equal(t, "notif_enabled", e.v.fields[item.Reference.(int)].Key, "an unknown section changes nothing")

	opened := 0
	v := teamDetailFor(TeamResolution{}, nil)
	v.cfg.OnNewTeam = func() { opened++ }
	v.SetShell(&formShell{})
	v.Mount(tview.NewFlex(), nil)
	assert.NotNil(t, v.HandleKey(runeKey('x')))
	assert.Nil(t, v.HandleKey(runeKey('n')))
	assert.Equal(t, 1, opened)
	assert.Contains(t, v.StatusHints(), "n")
}

// The two sections the team landing opens the detail on exist and are not the
// same place: the card of the member, then the shared configuration.
func TestTeamDetailOpensOnTheCardAndOnTheSharedConfig(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	resolve := e.v.cfg.ResolveTeam
	e.v.cfg.ResolveTeam = func() TeamResolution { tc := resolve(); tc.MemberID = "alice"; return tc }
	e.v.cfg.GetMember = func(context.Context, string) *teamstate.Member { return &teamstate.Member{ID: "alice"} }
	e.v.cfg.SaveMemberField = func(context.Context, string, string, string) error { return nil }
	e.v.Mount(tview.NewFlex(), nil)
	current := func() int {
		idx, _, ok := e.v.list.CurrentItem()
		require.True(t, ok)
		return idx
	}
	e.v.OpenSection(TeamSectionTracker)
	tracker := current()
	e.v.OpenSection(TeamSectionMember)
	member := current()
	assert.NotEqual(t, tracker, member)
	assert.Less(t, member, tracker, "the card comes before the shared configuration")
}
