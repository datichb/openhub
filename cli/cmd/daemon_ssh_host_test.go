package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A team-state whose SSH server known_hosts lacks (a hub.toml edited by hand, a
// server removed from known_hosts) is not connected to by the jobs of the
// daemon: one alert whatever the number of beats, even dismissed, no attempt,
// and the repository is taken up as soon as the server is known.
func TestUnknownSSHHostStopsTheDaemonJobsAndAlertsOnce(t *testing.T) {
	knownHosts := sshHostSandbox(t)
	const remote = "git@git.example.test:team/state.git"
	s1 := domain.Session{ID: "ses_1", StartedAt: time.Now().Add(-2 * time.Hour)}
	s2 := domain.Session{ID: "ses_2", StartedAt: time.Now().Add(-time.Hour)}
	w := newAccessWatch()
	log := &alertLog{}
	ctx := daemon.WithAlerts(t.Context(), log)

	for range 3 {
		require.True(t, w.UnknownHost(ctx, remote, []domain.Session{s1, s2}), "no connection to a server that known_hosts lacks")
	}
	require.Len(t, log.all(), 1, "one alert for the repository")
	alert := log.all()[0]
	assert.Equal(t, "ses_2", alert.SessionID, "under the most recent session")
	assert.Equal(t, alertTeamHost, alert.Kind)
	assert.Contains(t, alert.Message, "git.example.test")
	assert.Contains(t, alert.Message, remote)
	assert.Contains(t, alert.Message, "ssh -T git@git.example.test", "the command to run")

	assert.True(t, w.UnknownHost(ctx, remote, nil), "without a session: still no connection, nothing to raise")
	assert.Len(t, log.all(), 1)

	require.NoError(t, os.MkdirAll(filepath.Dir(knownHosts), 0o700))
	require.NoError(t, os.WriteFile(knownHosts, []byte("git.example.test ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMe2oLQFSPbU1ZXsHBUxEcjB6QExxb9fTzDW8/Z/Plf7\n"), 0o600))
	assert.False(t, w.UnknownHost(ctx, remote, []domain.Session{s2}), "the server is known: the job goes on")
	assert.Len(t, log.all(), 1)

	assert.False(t, w.UnknownHost(ctx, "https://git.example.test/team/state.git", []domain.Session{s2}), "HTTPS has no SSH server")
	assert.False(t, w.UnknownHost(ctx, "/srv/state.git", []domain.Session{s2}), "a local path neither")
}

// The heartbeat and the claims-queue task do not touch the team-state of an
// unknown server (no commit, no git), and renew the claims once it is known.
func TestClaimsBeatSkipsATeamWhoseSSHServerIsUnknown(t *testing.T) {
	a, repo := setupBeat(t, openSession("ses_1", "pt-a"))
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	before := claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time()
	commits := commitsOf(t, repo.Path())
	later := time.Now().Add(13 * time.Hour)

	unknown := true
	asked := 0
	b := beaterOf(a, later)
	b.access = &accessWatch{refused: map[string]accessRefusal{}, fingerprint: func(context.Context, string) string { return "" },
		unknownHost: func(_ context.Context, remote string) *teamstate.SSHHost {
			asked++
			assert.Equal(t, repo.Remote(), remote, "the host asked about is the one of the clone")
			if unknown {
				return &teamstate.SSHHost{Host: "git.example.test", Name: "git.example.test", User: "git", WriteTo: "/tmp/known_hosts"}
			}
			return nil
		}}
	log := &alertLog{}
	ctx := daemon.WithAlerts(t.Context(), log)

	require.NoError(t, b.beat(ctx))
	b.flush(ctx)
	assert.Equal(t, 2, asked, "asked by the beat and by the queue task")
	assert.Equal(t, commits, commitsOf(t, repo.Path()), "nothing was committed")
	assert.Equal(t, before, claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time(), "the claim was not renewed")
	require.Len(t, log.all(), 1)
	assert.Equal(t, "ses_1", log.all()[0].SessionID)

	unknown = false
	require.NoError(t, b.beat(ctx))
	assert.WithinDuration(t, later.Add(24*time.Hour), claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time(), 2*time.Second, "renewed once the server is known")
	assert.Len(t, log.all(), 1)
}

// The automatic sync does not synchronize a team whose SSH server is unknown,
// and looks at it again at the next interval.
func TestTrackerSyncTaskSkipsATeamWhoseSSHServerIsUnknown(t *testing.T) {
	svc := &fakeSyncService{intervals: map[string]time.Duration{"core": 5 * time.Minute}}
	s, now := newTestSyncer(svc, syncTeam("core"))
	unknown := true
	var asked []string
	s.access = &accessWatch{refused: map[string]accessRefusal{}, unknownHost: func(_ context.Context, remote string) *teamstate.SSHHost {
		asked = append(asked, remote)
		if unknown {
			return &teamstate.SSHHost{Host: "h", Name: "h", User: "git"}
		}
		return nil
	}}

	s.run(t.Context())
	assert.Empty(t, svc.calls, "no synchronization")
	assert.Equal(t, []string{"git@h:acme/core.git"}, asked)

	*now = now.Add(time.Minute)
	s.run(t.Context())
	assert.Len(t, asked, 1, "not asked again before the interval")

	unknown = false
	*now = now.Add(5 * time.Minute)
	s.run(t.Context())
	assert.Equal(t, []string{"core"}, svc.calls, "synchronized once the server is known")
}
