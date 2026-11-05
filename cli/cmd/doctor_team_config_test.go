package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

const doctorOldTeamConfig = `[parallel]
max_sessions = 5

[notification]
enabled = true
mattermost_webhook = "https://chat.example.com/hooks/very-secret-token"

[takeover]
stale_days = 99
`

func seedTeamConfig(t *testing.T, id, text string) string {
	t.Helper()
	a := TryApp()
	team := a.Config.FindTeam(id)
	require.NotNil(t, team)
	require.NoError(t, os.WriteFile(filepath.Join(team.StatePath, "config.toml"), []byte(text), 0o644))
	return team.StatePath
}

// The doctor reads the config.toml of the clone of each shared team and says what is out
// of date, with the repair; a webhook in a public repository is a warning of its own.
func TestDoctorTeamConfigReportsAndRepairs(t *testing.T) {
	doctorTrackerHub(t, "http://127.0.0.1:1", "git@gitlab.example.com:acme/core.git", nil)
	state := seedTeamConfig(t, "core", doctorOldTeamConfig)
	prevOffline, prevProbe := doctorOffline, teamPublicProbe
	t.Cleanup(func() { doctorOffline, teamPublicProbe = prevOffline, prevProbe })
	doctorOffline = func() bool { return false }
	var probed string
	teamPublicProbe = func(_ context.Context, remote string) (bool, error) { probed = remote; return true, nil }

	res := doctorCheckResults(t, doctorIDTeamConfig)
	require.Len(t, res, 2, "%+v", res)
	cfg := res[0]
	assert.True(t, cfg.OK)
	assert.True(t, cfg.Warn)
	assert.Contains(t, cfg.Detail, "parallel")
	assert.Contains(t, cfg.Detail, "mattermost_webhook")
	assert.Contains(t, cfg.Detail, "stale_days")
	assert.NotContains(t, cfg.Detail, "very-secret-token", "a webhook is never shown")
	require.NotNil(t, cfg.Fix, "the repair is offered")
	plan, err := cfg.Fix.Plan(t.Context())
	require.NoError(t, err)
	assert.Contains(t, plan, "stale_days")

	pub := res[1]
	assert.True(t, pub.Warn)
	assert.Contains(t, pub.Name, "Public")
	assert.Contains(t, pub.Detail, "read by anybody")
	assert.Equal(t, "git@gitlab.example.com:acme/core.git", probed)
	assert.NotContains(t, pub.Detail, "token")

	report, err := cfg.Fix.Apply(t.Context())
	require.NoError(t, err)
	assert.Contains(t, report, "repaired")
	raw, err := os.ReadFile(filepath.Join(state, "config.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "parallel")
	assert.NotContains(t, string(raw), "mattermost_webhook")
	assert.Contains(t, string(raw), "webhook_url")

	res = doctorCheckResults(t, doctorIDTeamConfig)
	assert.False(t, res[0].Warn, "%+v", res[0])
	assert.Nil(t, res[0].Fix)
}

// Without a webhook there is nothing a public repository gives away; offline, nothing is asked.
func TestDoctorTeamConfigPublicProbeOnlyWithAWebhookAndOnline(t *testing.T) {
	doctorTrackerHub(t, "http://127.0.0.1:1", "git@gitlab.example.com:acme/core.git", nil)
	prevOffline, prevProbe := doctorOffline, teamPublicProbe
	t.Cleanup(func() { doctorOffline, teamPublicProbe = prevOffline, prevProbe })
	calls := 0
	teamPublicProbe = func(context.Context, string) (bool, error) { calls++; return true, nil }
	doctorOffline = func() bool { return false }

	seedTeamConfig(t, "core", "[tracker]\ntype = \"gitlab\"\n")
	res := doctorCheckResults(t, doctorIDTeamConfig)
	require.Len(t, res, 1)
	assert.False(t, res[0].Warn)
	assert.Zero(t, calls)

	seedTeamConfig(t, "core", "[notification]\nenabled = true\ntype = \"slack\"\nwebhook_url = \"https://hooks.slack.com/services/T/B/x\"\n")
	doctorOffline = func() bool { return true }
	assert.Len(t, doctorCheckResults(t, doctorIDTeamConfig), 1, "offline: no probe")
	assert.Zero(t, calls)

	doctorOffline = func() bool { return false }
	teamPublicProbe = func(context.Context, string) (bool, error) { return false, nil }
	assert.Len(t, doctorCheckResults(t, doctorIDTeamConfig), 1, "private: nothing to say")
	teamPublicProbe = func(context.Context, string) (bool, error) { return false, errors.New("no network") }
	assert.Len(t, doctorCheckResults(t, doctorIDTeamConfig), 1, "no answer: nothing to say")
}

func TestDoctorTeamConfigUnreadableAndBoardAreFailures(t *testing.T) {
	doctorTrackerHub(t, "http://127.0.0.1:1", "git@gitlab.example.com:acme/core.git", nil)
	prev := doctorOffline
	t.Cleanup(func() { doctorOffline = prev })
	doctorOffline = func() bool { return true }
	seedTeamConfig(t, "core", "[[board.columns]]\nid = \"a\"\nname = \"A\"\n")
	res := doctorCheckResults(t, doctorIDTeamConfig)
	require.Len(t, res, 1)
	assert.False(t, res[0].OK, "a board with one column is a failure")
	assert.Contains(t, res[0].Detail, "at least 2 columns")
	assert.Nil(t, res[0].Fix, "nothing to repair by itself")
}

// The tracker connection is tested once for the two checks that look at it.
func TestDoctorConnectionAndIdentityShareOneRequest(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)
	prev := doctorOffline
	t.Cleanup(func() { doctorOffline = prev })
	doctorOffline = func() bool { return false }

	reg, err := doctor.NewRegistry(doctorRegistry()...)
	require.NoError(t, err)
	run := withDoctorRun(t.Context())
	for _, id := range []string{doctorIDTrackerConnection, doctorIDTrackerIdentity} {
		c, ok := reg.Get(id)
		require.True(t, ok)
		c.Results(run)
	}
	assert.EqualValues(t, 1, tr.hits.Load(), "one TestConnection per tracker and run")

	tr.hits.Store(0)
	for _, id := range []string{doctorIDTrackerConnection, doctorIDTrackerIdentity} {
		c, _ := reg.Get(id)
		c.Results(t.Context())
	}
	assert.EqualValues(t, 2, tr.hits.Load(), "outside a run nothing is shared")
}

// Two teams marked default in hub.toml: the doctor says which one wins and --fix writes it.
func TestDoctorHubTeamsTwoDefaults(t *testing.T) {
	a := doctorTrackerHub(t, "http://127.0.0.1:1", "git@gitlab.example.com:acme/core.git", nil)
	a.Config.Teams = append(a.Config.Teams, config.TeamConfig{ID: "b", Enabled: true, StateRepo: "git@h:a/b.git", MemberID: "alice"})
	a.Config.Teams[0].Default, a.Config.Teams[1].Default = true, true
	data := "[[teams]]\nid = 'core'\nenabled = true\nstate_repo = 'git@gitlab.example.com:acme/core.git'\nmember_id = 'alice'\ndefault = true\n\n" +
		"[[teams]]\nid = 'b'\nenabled = true\nstate_repo = 'git@h:a/b.git'\nmember_id = 'alice'\ndefault = true\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(config.ConfigPath()), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(data), 0o644))

	res := doctorCheckResults(t, doctorIDHubTeams)
	require.Len(t, res, 1)
	assert.True(t, res[0].Warn)
	assert.Contains(t, res[0].Detail, "core, b")
	require.NotNil(t, res[0].Fix)
	_, err := res[0].Fix.Apply(t.Context())
	require.NoError(t, err)
	raw, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(raw), "default = true"))
	assert.Empty(t, doctorCheckResults(t, doctorIDHubTeams))
}
