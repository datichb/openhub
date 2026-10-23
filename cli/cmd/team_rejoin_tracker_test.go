package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// rejoinTeam runs oh team rejoin on the team-state of the claim tests, whose
// config.toml says the tracker is at trackerURL ("" = no tracker), with the
// answers of the user on its input. flags are given as --name=value.
func rejoinTeam(t *testing.T, trackerURL, answers string, flags ...string) (out string, member *teamstate.Member) {
	t.Helper()
	a, repo, stdout := setupClaimTestApp(t)
	configtest.Home(t)
	i18ntest.Set(t, "en")
	require.NoError(t, config.Save(&config.Config{}))
	a.Config.Teams = nil
	a.IO.In = strings.NewReader(answers)
	if trackerURL != "" {
		toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"" + trackerURL + "\"\ntracker_project = \"acme/app\"\n"
		gittest.Commit(t, repo.Path(), "config.toml", toml, "tracker")
		gittest.Run(t, repo.Path(), "push", "-q")
	}

	c := &cobra.Command{Use: "rejoin", RunE: runTeamRejoin}
	addTeamRejoinFlags(c)
	c.SetContext(t.Context())
	args := append([]string{"--repo=" + repo.Remote(), "--member-id=testuser", "--no-retro-tag"}, flags...)
	require.NoError(t, c.ParseFlags(args))
	require.NoError(t, runTeamRejoin(c, nil))
	require.NoError(t, repo.Pull(t.Context()))
	m, err := repo.GetMember("testuser")
	require.NoError(t, err)
	return stdout.String(), m
}

// Bug 12: oh team rejoin has an option for the tracker name of the member.
func TestTeamRejoinTrackerUsernameFlag(t *testing.T) {
	out, m := rejoinTeam(t, "https://gitlab.other.example", "", "--tracker-username=@benjamin.datiche")
	assert.Equal(t, "benjamin.datiche", m.TrackerUsername)
	assert.Contains(t, out, "benjamin.datiche")
	assert.NotContains(t, out, "Your user name on the tracker", "given: not asked")
}

// And a question, when the tracker is on another server than the team repository.
func TestTeamRejoinAsksTheTrackerUsernameForATrackerOnAnotherServer(t *testing.T) {
	out, m := rejoinTeam(t, "https://gitlab.other.example", "benjamin.datiche\n")
	assert.Contains(t, out, "Your user name on the tracker")
	assert.Equal(t, "benjamin.datiche", m.TrackerUsername)

	out, m = rejoinTeam(t, "https://gitlab.other.example", "\n")
	assert.Contains(t, out, "Your user name on the tracker")
	assert.Empty(t, m.TrackerUsername, "Enter leaves it empty")
}

// No question when it would be useless: the tracker is on the server of the
// repository (declared with --gitlab-url, the remote being a local path here),
// or there is no tracker.
func TestTeamRejoinDoesNotAskWithoutNeed(t *testing.T) {
	out, m := rejoinTeam(t, "", "ignored\n")
	assert.NotContains(t, out, "Your user name on the tracker", "no tracker")
	assert.Empty(t, m.TrackerUsername)

	out, m = rejoinTeam(t, "https://gitlab.same.example", "ignored\n", "--gitlab-url=https://gitlab.same.example")
	assert.NotContains(t, out, "Your user name on the tracker", "same server")
	assert.Empty(t, m.TrackerUsername)
}

// Bug 12: the rejoin of the wizard of oh init has a step for the tracker name,
// shown when the tracker is elsewhere and the member has none.
func TestInitWizardRejoinHasATrackerUsernameStep(t *testing.T) {
	i18ntest.Set(t, "en")
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	steps := buildInitWizardRejoinSteps(&a, &initWizardTeamState{})
	step := views.StepByID(steps, "rejoin_tracker_username")
	require.NotNil(t, step, "the step exists")

	state := &initWizardTeamState{
		Mode: "rejoin", MemberID: "ben", TrackerWanted: true,
		Members: []teamstate.Member{{ID: "ben"}, {ID: "ana", TrackerUsername: "ana.t"}},
	}
	steps = buildInitWizardRejoinSteps(&a, state)
	step = views.StepByID(steps, "rejoin_tracker_username")
	assert.False(t, step.SkipIf(), "the tracker is elsewhere and ben has no tracker name")

	state.MemberID = "ana"
	assert.True(t, step.SkipIf(), "ana has one")
	state.MemberID = "ben"
	state.TrackerWanted = false
	assert.True(t, step.SkipIf(), "the tracker is on the server of the repository (or there is none)")
	state.TrackerWanted, state.Skipped = true, true
	assert.True(t, step.SkipIf(), "the team part of the wizard was skipped")
}

func TestTrackerUsernameWanted(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	for _, tc := range []struct {
		name, toml, remote, gitlabURL string
		want                          bool
	}{
		{"no tracker", "", "git@gitlab.com:a/core.git", "", false},
		{"same server", "tracker_url = \"https://gitlab.corp.example\"", "git@gitlab.corp.example:a/core.git", "", false},
		{"another server", "tracker_url = \"https://gitlab.other.example\"", "git@gitlab.corp.example:a/core.git", "", true},
		{"gitlab.com by default", "", "git@gitlab.com:a/core.git", "", false},
		{"gitlab.com by default, repository elsewhere", "", "git@gitlab.corp.example:a/core.git", "", true},
		{"alias SSH read through its GitLab", "tracker_url = \"https://gitlab.corp.example\"", "git@alias:a/core.git", "https://gitlab.corp.example", false},
		{"jira is never the server of the repository", "type = \"jira\"\ntracker_url = \"https://jira.corp.example\"", "git@gitlab.corp.example:a/core.git", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			repo, err := teamstate.InitSolo(t.Context(), filepath.Join(dir, "s"), teamstate.Member{ID: "a"})
			require.NoError(t, err)
			cfg := ""
			if tc.name != "no tracker" {
				typ := "type = \"gitlab\"\n"
				if strings.Contains(tc.toml, "type =") {
					typ = ""
				}
				cfg = "[tracker]\n" + typ + "enabled = true\n" + tc.toml + "\n"
			}
			require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte(cfg), 0o644))
			assert.Equal(t, tc.want, trackerUsernameWanted(repo, tc.remote, tc.gitlabURL))
		})
	}
}
