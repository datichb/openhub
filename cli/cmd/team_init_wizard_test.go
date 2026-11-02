package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh team init, on a team-state that is cloned: the answers of the wizard
// write the team config, the member, the notifications and the policies to
// the team-state.
func TestTeamInitWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	// Steps: 0 repo (skipped: the team has a repo), 1 config (stale days),
	// 2 identity, 3 notifications (webhook, channel, bot name),
	// 4 policies (four check boxes: branch_naming, commit_format,
	// max_ticket_wip, review_required).
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
			1: func(f *tview.Form) { typeInto(f, 0, "5") },
			2: func(f *tview.Form) {
				typeInto(f, 0, "Test User II")
				typeInto(f, 1, "tuser")
				typeInto(f, 2, "tmm")
				typeInto(f, 3, "ttrack")
				choose(f, 4, 1)
			},
			3: notif,
			4: policies,
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, "team init", s.command, "command of the wizard")
		assert.Equal(t, []int{1, 2, 3, 4}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 5, cfg.Takeover.StaleDays, "stale days")
		assert.Equal(t, "https://mm.example/hook", cfg.Notification.MattermostWebhook, "webhook")
		assert.Equal(t, "dev-team", cfg.Notification.Channel, "channel")
		assert.Equal(t, "Bot", cfg.Notification.BotName, "bot name")
		assert.True(t, cfg.Notification.Enabled, "notifications enabled")
		m, err := repo.GetMember("testuser")
		require.NoError(t, err)
		assert.Equal(t, "Test User II", m.DisplayName)
		assert.Equal(t, "tuser", m.GitLabUsername)
		assert.Equal(t, "tmm", m.MattermostUsername)
		assert.Equal(t, "ttrack", m.TrackerUsername)
		assert.Equal(t, "dev", m.Role)
		list, err := repo.LoadPolicies("")
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
			1: func(f *tview.Form) { typeInto(f, 0, "abc") }, // not a number: 3 days
			2: func(f *tview.Form) {
				typeInto(f, 0, "newbie")
				typeInto(f, 1, "New Bie")
				choose(f, 5, 2)
			},
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{1, 2, 3, 4}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 3, cfg.Takeover.StaleDays, "stale days")
		assert.False(t, cfg.Notification.Enabled, "no webhook: notifications stay off")
		m, err := repo.GetMember("newbie")
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
			2: func(f *tview.Form) { typeInto(f, 1, "No Id") },
		})

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{1, 2}, s.ran, "steps that ran")
		assert.Equal(t, map[int]string{2: i18n.T("cmd.team.init.validate.member_id_required")}, s.refused, "steps refused")
		members, err := repo.ListMembers()
		require.NoError(t, err)
		assert.Len(t, members, 1, "only the member that existed")
		assert.Empty(t, a.Config.ActiveTeam().MemberID, "member of the hub")
	})

	t.Run("the user quits: nothing is written after the step that ran", func(t *testing.T) {
		configtest.Home(t)
		require.NoError(t, config.Save(&config.Config{}))
		a, repo, out := setupClaimTestApp(t)
		s := scriptWizard(t, map[int]func(f *tview.Form){1: func(f *tview.Form) { typeInto(f, 0, "9") }})
		s.abortAt = 2

		teamInitCmd.SetContext(t.Context())
		require.NoError(t, runTeamInit(teamInitCmd, nil))

		assert.Equal(t, []int{1}, s.ran, "steps that ran")
		cfg, err := repo.LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 9, cfg.Takeover.StaleDays, "the config step had been applied")
		assert.Empty(t, cfg.Notification.MattermostWebhook)
		list, err := repo.LoadPolicies("")
		require.NoError(t, err)
		assert.Empty(t, list, "policies")
		assert.NotContains(t, out.String(), i18n.T("cmd.team.init.done"), "no summary")
		assert.Equal(t, "testuser", a.Config.ActiveTeam().MemberID)
	})
}
