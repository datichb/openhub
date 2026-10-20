package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

type oneSessionStore struct {
	domain.SessionStore
	s *domain.Session
}

func (o oneSessionStore) Get(context.Context, string) (*domain.Session, error) { return o.s, nil }

// The relaunch from the TUI says it is the user's; the daemon's keeps its
// own option.
func TestRelaunchFanoutFromTUIAsksForUserOrigin(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	var gotID string
	var gotArgs []string
	prev := spawnDriver
	spawnDriver = func(id string, extra ...string) (string, error) {
		gotID, gotArgs = id, extra
		return "fanout.log", nil
	}
	t.Cleanup(func() { spawnDriver = prev })
	parent := &domain.Session{ID: "ses_p", State: domain.RunActive,
		Fanout: &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutRunning}}
	a := &app.App{Config: &config.Config{Fanout: config.FanoutConfig{Experimental: true}}, Sessions: oneSessionStore{s: parent}}
	_, err := (&tuiSessions{a: a}).RelaunchFanout(context.Background(), "ses_p")
	require.NoError(t, err)
	assert.Equal(t, "ses_p", gotID)
	assert.Equal(t, []string{"--relaunch-by=user"}, gotArgs)
	assert.Equal(t, "--relaunch", relaunchByDaemonArg)
}

func TestRelaunchOrigin(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		by         string
		relaunched bool
		bad        bool
	}{
		{nil, "daemon", false, false},
		{[]string{"--relaunch"}, "daemon", true, false},
		{[]string{"--relaunch-by=user"}, "user", true, false},
		{[]string{"--relaunch-by=daemon"}, "daemon", true, false},
		{[]string{"--relaunch-by=robot"}, "", false, true},
	} {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("relaunch", false, "")
		cmd.Flags().String("relaunch-by", "", "")
		require.NoError(t, cmd.ParseFlags(tc.args))
		by, relaunched, err := relaunchOrigin(cmd)
		if tc.bad {
			assert.Error(t, err, tc.args)
			continue
		}
		require.NoError(t, err, tc.args)
		assert.Equal(t, tc.by, by, tc.args)
		assert.Equal(t, tc.relaunched, relaunched, tc.args)
	}
}
