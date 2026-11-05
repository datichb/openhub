package cmd

import (
	"github.com/spf13/cobra"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// reconfigure makes oh team init work on the team that is configured, as
// --reconfigure does.
func reconfigure(t *testing.T) {
	t.Helper()
	require.NoError(t, teamInitCmd.Flags().Set("reconfigure", "true"))
	t.Cleanup(func() { _ = teamInitCmd.Flags().Set("reconfigure", "false") })
}

// oh team init --reconfigure, on a team-state that is cloned: the answers of
// the wizard write the team config, the member, the notifications and the
// policies to the team-state.
func TestTeamInitWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	reconfigure(t)
	// Steps: 0 and 1 the SSH server (skipped: the repository is a local path),
	// 2 probe of the access, 3 access (skipped: the repository answers),
	// 4 synchronisation, 5 config (stale days), 6 identity, 7 notifications
	// (webhook, channel, bot name), 8 policies (four check boxes:
	// branch_naming, commit_format, max_ticket_wip, review_required).
	notif := func(f *tview.Form) {
		typeInto(f, 0, "https://mm.example/hook")
		typeInto(f, 1, "dev-team")
		typeInto(f, 2, "Bot")
	}
	policies := func(f *tview.Form) { tick(f, 0, true); tick(f, 3, true) }

	t.Run("the member of the hub is updated", func(t *testing.T) {
		configtest.Home(t)
		require.NoError(t, config.Save(&config.Config{}))
		a, repo, out := setupClaimTestApp(t)
		// Identity of a known member: no id field; display name, GitLab,
		// Mattermost, tracker, role.
		s := scriptWizard(t, map[int]func(f *tview.Form){
			5: func(f *tview.Form) { typeInto(f, 0, "5") },
			6: func(f *tview.Form) {
				typeInto(f, 0, "Test User II")
				typeInto(f, 1, "tuser")
				typeInto(f, 2, "tmm")
				typeInto(f, 3, "ttrack")
				choose(f, 4, 1)
			},
			7: notif,
			8: policies,
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, "team init", s.command, "command of the wizard")
		assert.Equal(t, []int{2, 4, 5, 6, 7, 8}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 5, cfg.Takeover.StaleDays, "stale days")
		assert.Equal(t, "https://mm.example/hook", cfg.Notification.MattermostWebhook, "webhook")
		assert.Equal(t, "dev-team", cfg.Notification.Channel, "channel")
		assert.Equal(t, "Bot", cfg.Notification.BotName, "bot name")
		assert.True(t, cfg.Notification.Enabled, "notifications enabled")
		m, err := repo.GetMember(t.Context(), "testuser")
		require.NoError(t, err)
		assert.Equal(t, "Test User II", m.DisplayName)
		assert.Equal(t, "tuser", m.GitLabUsername)
		assert.Equal(t, "tmm", m.MattermostUsername)
		assert.Equal(t, "ttrack", m.TrackerUsername)
		assert.Equal(t, "dev", m.Role)
		list, err := repo.LoadPolicies(t.Context(), "")
		require.NoError(t, err)
		var names []string
		for _, p := range list {
			names = append(names, p.Name)
		}
		assert.ElementsMatch(t, []string{"branch_naming", "review_required"}, names, "policies")
		assert.Equal(t, "testuser", a.Config.ActiveTeam().MemberID, "member of the hub")
		assert.Contains(t, out.String(), i18n.T("cmd.team.init.done"), "summary")
		assert.Contains(t, out.String(), "mattermost", "summary")
	})

	t.Run("a new member is added and the hub is written", func(t *testing.T) {
		configtest.Home(t)
		require.NoError(t, config.Save(&config.Config{}))
		a, repo, _ := setupClaimTestApp(t)
		a.Config.Teams[0].MemberID = ""
		s := scriptWizard(t, map[int]func(f *tview.Form){
			5: func(f *tview.Form) { typeInto(f, 0, "abc") }, // not a number: 3 days
			6: func(f *tview.Form) {
				typeInto(f, 0, "newbie")
				typeInto(f, 1, "New Bie")
				choose(f, 5, 2)
			},
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{2, 4, 5, 6, 7, 8}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 3, cfg.Takeover.StaleDays, "stale days")
		assert.False(t, cfg.Notification.Enabled, "no webhook: notifications stay off")
		m, err := repo.GetMember(t.Context(), "newbie")
		require.NoError(t, err)
		assert.Equal(t, "New Bie", m.DisplayName)
		assert.Equal(t, "reviewer", m.Role)
		assert.Equal(t, "semi-auto", m.DefaultMode)
		assert.Equal(t, "newbie", a.Config.ActiveTeam().MemberID, "hub.toml in memory")
		config.Reset()
		saved, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "newbie", saved.ActiveTeam().MemberID, "hub.toml")
	})

	t.Run("no member id: the identity step is refused, nothing is written for the member", func(t *testing.T) {
		configtest.Home(t)
		require.NoError(t, config.Save(&config.Config{}))
		a, repo, _ := setupClaimTestApp(t)
		a.Config.Teams[0].MemberID = ""
		s := scriptWizard(t, map[int]func(f *tview.Form){
			6: func(f *tview.Form) { typeInto(f, 1, "No Id") },
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{2, 4, 5, 6}, s.ran, "steps that ran")
		assert.Equal(t, map[int]string{6: i18n.T("cmd.init.wizard_team_member_required")}, s.refused, "steps refused")
		members, err := repo.ListMembers(t.Context())
		require.NoError(t, err)
		assert.Len(t, members, 1, "only the member that existed")
		assert.Empty(t, a.Config.ActiveTeam().MemberID, "member of the hub")
	})

	t.Run("the user quits: nothing is written after the step that ran", func(t *testing.T) {
		configtest.Home(t)
		require.NoError(t, config.Save(&config.Config{}))
		a, repo, out := setupClaimTestApp(t)
		s := scriptWizard(t, map[int]func(f *tview.Form){5: func(f *tview.Form) { typeInto(f, 0, "9") }})
		s.abortAt = 6

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{2, 4, 5}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 9, cfg.Takeover.StaleDays, "the config step had been applied")
		assert.Empty(t, cfg.Notification.MattermostWebhook)
		list, err := repo.LoadPolicies(t.Context(), "")
		require.NoError(t, err)
		assert.Empty(t, list, "policies")
		assert.NotContains(t, out.String(), i18n.T("cmd.team.init.done"), "no summary")
		assert.Equal(t, "testuser", a.Config.ActiveTeam().MemberID)
	})
}

// oh team init at the first launch: no team repo is configured, the first
// steps probe it and clone it, and the steps that follow work on the clone.
func TestTeamInitWizardFirstLaunch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	i18ntest.Set(t, "en")
	configtest.Home(t)
	require.NoError(t, config.Save(&config.Config{}))
	bare, _ := claimRepos(t)
	io, out, _ := testIO()
	a := &app.App{Config: &config.Config{}, Projects: &mockProjectStore{}, IO: io}
	application = a
	t.Cleanup(func() { application = nil })

	// Steps: 0 repository, 1 and 2 the SSH server (skipped), 3 probe,
	// 4 access (skipped), 5 clone, 6 config, 7 identity, 8 notifications,
	// 9 policies.
	s := scriptWizard(t, map[int]func(f *tview.Form){
		0: func(f *tview.Form) { typeInto(f, 0, bare) },
		6: func(f *tview.Form) { typeInto(f, 0, "6") },
		7: func(f *tview.Form) {
			typeInto(f, 0, "newbie")
			typeInto(f, 1, "New Bie")
			choose(f, 5, 1)
		},
		8: func(f *tview.Form) {
			typeInto(f, 0, "https://mm.example/hook")
			typeInto(f, 1, "dev-team")
			typeInto(f, 2, "Bot")
		},
		9: func(f *tview.Form) { tick(f, 0, true); tick(f, 3, true) },
	})

	teamInitCmd.SetContext(t.Context())
	require.NoError(t, runTeamInit(teamInitCmd, nil))

	assert.Equal(t, []int{0, 3, 5, 6, 7, 8, 9}, s.ran, "steps that ran")
	assert.Empty(t, s.refused, "steps refused")
	repo := teamstate.NewRepo(bare, config.TeamStatePath(bare))
	require.True(t, repo.IsCloned(), "the team-state is cloned in the folder every team assistant uses")
	assert.NoDirExists(t, config.DefaultTeamStatePath(), "not in the former ~/.oh/team-state")
	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 6, cfg.Takeover.StaleDays, "stale days")
	assert.Equal(t, "https://mm.example/hook", cfg.Notification.MattermostWebhook, "webhook")
	assert.True(t, cfg.Notification.Enabled, "notifications enabled")
	m, err := repo.GetMember(t.Context(), "newbie")
	require.NoError(t, err)
	assert.Equal(t, "New Bie", m.DisplayName)
	assert.Equal(t, "dev", m.Role)
	list, err := repo.LoadPolicies(t.Context(), "")
	require.NoError(t, err)
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	assert.ElementsMatch(t, []string{"branch_naming", "review_required"}, names, "policies")
	assert.Equal(t, bare, a.Config.ActiveTeam().StateRepo, "team repo of the hub")
	assert.Equal(t, "newbie", a.Config.ActiveTeam().MemberID, "member of the hub")
	assert.Contains(t, out.String(), i18n.T("cmd.team.init.done"), "summary")
}

// A team-state that is available after the state of the steps was built (the
// clone of step 0) gives them its repo and what it already holds.
func TestTeamStepStateUseRepo(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	cfg := &teamstate.TeamConfig{Takeover: teamstate.TakeoverConfig{StaleDays: 8}}
	cfg.Notification.MattermostWebhook = "https://mm.example/hook"
	cfg.Notification.Channel = "dev"
	cfg.Notification.BotName = "Bot"
	require.NoError(t, repo.SaveConfig(t.Context(), cfg))

	s := &teamStepState{Ctx: t.Context(), MemberID: "testuser", StaleDaysStr: "3", BotName: "OpenHub"}
	s.useRepo(repo)

	assert.Same(t, repo, s.Repo)
	assert.True(t, s.HasConfig)
	assert.False(t, s.HasPolicies)
	assert.Equal(t, "8", s.StaleDaysStr)
	assert.Equal(t, "https://mm.example/hook", s.WebhookURL)
	assert.Equal(t, "dev", s.Channel)
	assert.Equal(t, "Bot", s.BotName)
	assert.True(t, s.HasMember)
	assert.Equal(t, "Test User", s.DisplayName)
	assert.Equal(t, "dev", s.Role)

	fresh := &teamStepState{Ctx: t.Context(), StaleDaysStr: "3", BotName: "OpenHub"}
	fresh.useRepo(repo)
	assert.False(t, fresh.HasMember, "no member id: no profile")
	assert.Empty(t, fresh.DisplayName)
}

// --team (the persistent flag of oh team) names the team to reconfigure; an
// unknown one is told.
func TestTeamToReconfigure(t *testing.T) {
	a := &app.App{Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "one", Enabled: true, StateRepo: "git@x:o/one.git"},
		{ID: "two", Enabled: true, StateRepo: "git@x:o/two.git"},
	}}}
	c := &cobra.Command{}
	got, err := teamToReconfigure(c, a)
	require.NoError(t, err)
	assert.Equal(t, "one", got.ID, "no --team: the team the commands work on")

	c.Flags().String("team", "", "")
	require.NoError(t, c.Flags().Set("team", "two"))
	got, err = teamToReconfigure(c, a)
	require.NoError(t, err)
	assert.Equal(t, "two", got.ID)

	require.NoError(t, c.Flags().Set("team", "nope"))
	_, err = teamToReconfigure(c, a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}
