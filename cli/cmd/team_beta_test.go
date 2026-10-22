package cmd

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

const teamBetaMarker = "Team mode is in beta"

func resetTeamBeta(t *testing.T) {
	t.Helper()
	teamBetaShown.Store(false)
	t.Cleanup(func() { teamBetaShown.Store(false) })
}

// The team commands warn on stderr, once per process.
func TestTeamPreRunWarnsOnceOnStderr(t *testing.T) {
	i18ntest.Set(t, "en")
	resetTeamBeta(t)

	run := func() (stdout, stderr string) {
		var out, errOut bytes.Buffer
		c := &cobra.Command{Use: "init"} // skips the repo resolution
		c.SetOut(&out)
		c.SetErr(&errOut)
		require.NoError(t, teamPreRunE(c, nil))
		return out.String(), errOut.String()
	}

	stdout, stderr := run()
	assert.Contains(t, stderr, teamBetaMarker)
	assert.Contains(t, stderr, "docs/guides/team-setup.en.md")
	assert.Empty(t, stdout, "the warning stays off stdout")

	_, stderr = run()
	assert.Empty(t, stderr, "second command of the process: no second warning")
}

func TestTeamBetaWarningFrench(t *testing.T) {
	i18ntest.Set(t, "fr")
	resetTeamBeta(t)
	var w bytes.Buffer
	printTeamBetaWarning(&w)
	assert.Contains(t, w.String(), "Le mode équipe est en bêta")
}

// The first claim taken at launch carries the warning, once, and it is not
// repeated after the team commands already showed it.
func TestStartTicketClaimsWarnsOnce(t *testing.T) {
	i18ntest.Set(t, "en")
	resetTeamBeta(t)
	a, _, _ := setupClaimTestApp(t)
	teamID := "test-team"
	project := &domain.Project{ID: "myproject", Name: "myproject", TeamID: &teamID}
	hook := startTicketClaims(a, project)
	require.NotNil(t, hook)

	notes := hook(context.Background(), "myproject", []string{"BETA-1"})
	require.Len(t, notes, 2)
	assert.Contains(t, notes[0], "BETA-1")
	assert.Contains(t, notes[1], teamBetaMarker)

	notes = hook(context.Background(), "myproject", []string{"BETA-2"})
	require.Len(t, notes, 1, "the warning is not repeated")
	assert.NotContains(t, notes[0], teamBetaMarker)

	resetTeamBeta(t)
	var w bytes.Buffer
	printTeamBetaWarning(&w)
	notes = hook(context.Background(), "myproject", []string{"BETA-3"})
	require.Len(t, notes, 1, "already shown by a team command")
}

// The doctor lists the beta state as a warning, only when a team is used.
func TestTeamBetaDoctorCheck(t *testing.T) {
	i18ntest.Set(t, "en")
	off := &app.App{Config: &config.Config{}}
	assert.Empty(t, teamBetaDoctorCheck(off, nil))
	assert.Empty(t, teamBetaDoctorCheck(nil, nil))

	solo := &app.App{Config: &config.Config{Teams: []config.TeamConfig{{ID: "me", Enabled: true, Solo: true}}}}
	assert.Empty(t, teamBetaDoctorCheck(solo, nil), "a solo space is not the team mode")

	on := &app.App{Config: &config.Config{Teams: []config.TeamConfig{{ID: "t", Enabled: true, StateRepo: "git@example.com:t/state.git"}}}}
	got := teamBetaDoctorCheck(on, nil)
	require.Len(t, got, 1)
	assert.True(t, got[0].OK, "a warning never fails the doctor")
	assert.True(t, got[0].Warn)
	assert.Contains(t, got[0].Name, "beta")
	assert.True(t, strings.Contains(got[0].Detail, "oh team release"), got[0].Detail)

	teamID := "t"
	other := &app.App{Config: on.Config}
	got = teamBetaDoctorCheck(other, []domain.Project{{ID: "p", TeamID: &teamID}})
	assert.Len(t, got, 1)
}

func TestTeamBetaDoctorCheckRegistered(t *testing.T) {
	found := false
	for _, fn := range doctorChecks {
		if strings.Contains(runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(), "teamBetaDoctorChecks") {
			found = true
		}
	}
	assert.True(t, found, "teamBetaDoctorChecks is registered with the doctor")
}
