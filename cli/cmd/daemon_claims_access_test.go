package cmd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// alertLog is a daemon that records the alerts its jobs raise.
type alertLog struct {
	mu     sync.Mutex
	alerts []daemon.Alert
	seen   map[string]bool
}

func (l *alertLog) RaiseAlert(_ context.Context, a daemon.Alert) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := a.SessionID + "|" + a.Kind + "|" + a.Ref
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if l.seen[key] {
		return false
	}
	l.seen[key] = true
	l.alerts = append(l.alerts, a)
	return true
}

func (l *alertLog) all() []daemon.Alert {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]daemon.Alert(nil), l.alerts...)
}

// A team over HTTPS whose token expires after the team was joined: the clone
// and a claim are made with the good token, then the keychain holds a token the
// server refuses.
func expiredTokenTeam(t *testing.T) (a *app.App, repo *teamstate.Repo, set func(token string), refused func() int) {
	t.Helper()
	isolateGitAuth(t)
	bare, clone := claimRepos(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "oauth2", "good")
	gittest.Run(t, clone, "remote", "set-url", "origin", srv.URL)

	var mu sync.Mutex
	token := "good"
	restore := teamstate.SetCredentialSource(func(context.Context, string) teamstate.GitCredentials {
		mu.Lock()
		defer mu.Unlock()
		return teamstate.GitCredentials{Username: "oauth2", Token: token}
	})
	t.Cleanup(restore)
	set = func(v string) { mu.Lock(); token = v; mu.Unlock() }

	repo = teamstate.NewRepo(srv.URL, clone)
	io, _, _ := testIO()
	a = &app.App{
		Config: &config.Config{Teams: []config.TeamConfig{{
			ID: "test-team", Enabled: true, StateRepo: srv.URL, StatePath: clone, MemberID: "testuser",
		}}},
		Projects: &mockProjectStore{projects: []domain.Project{teamProject(claimsProject)}},
		IO:       io,
	}
	application = a
	t.Cleanup(func() { application = nil })
	return a, repo, set, srv.Refused
}

// The token of the team-state expired: the first beat raises one alert under the
// session, the next ones make no attempt on the network at all, and a new token
// is tried again.
func TestClaimsBeatStopsOnAnExpiredTokenAndAlertsOnce(t *testing.T) {
	a, repo, setToken, refused := expiredTokenTeam(t)
	s1 := openSession("ses_1", "pt-a")
	s1.StartedAt = time.Now().Add(-2 * time.Hour)
	s2 := openSession("ses_2", "pt-b")
	s2.StartedAt = time.Now().Add(-1 * time.Hour)
	a.Sessions = runningSessions{list: []domain.Session{s1, s2}}
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-b", "testuser", "ses_2")

	fingerprint := "fp-good"
	b := beaterOf(a, time.Now().Add(13*time.Hour))
	b.access = &accessWatch{refused: map[string]accessRefusal{}, fingerprint: func(context.Context, string) string { return fingerprint }}
	log := &alertLog{}
	ctx := daemon.WithAlerts(t.Context(), log)

	setToken("expired")
	fingerprint = "fp-expired"
	require.NoError(t, b.beat(ctx), "a refused access is told, it is not an error of the beat")
	require.Len(t, log.all(), 1, "one alert")
	alert := log.all()[0]
	assert.Equal(t, "ses_2", alert.SessionID, "under the most recent session of the team")
	assert.Equal(t, alertTeamAccess, alert.Kind)
	assert.Contains(t, alert.Message, repo.Remote())
	assert.Contains(t, alert.Message, "oh team rejoin")
	attempts := refused()
	assert.Positive(t, attempts, "the server refused the expired token")
	assert.Equal(t, 1, attempts, "one attempt on the network for the beat, not one for the renewal and one for the queue")

	b.last = map[string]time.Time{}
	for range 3 {
		require.NoError(t, b.beat(ctx))
		require.NoError(t, b.beat(ctx))
		b.flush(ctx)
	}
	assert.Equal(t, attempts, refused(), "no attempt until the token changes")
	assert.Len(t, log.all(), 1)

	// renewed (and refused again): another chance, another alert
	setToken("expired-too")
	fingerprint = "fp-expired-too"
	require.NoError(t, b.beat(ctx))
	assert.Greater(t, refused(), attempts, "tried again with the new token")
	assert.Len(t, log.all(), 2, "the new token is refused: a new alert")

	// the good token: the claims are renewed, the memory forgets the refusal
	setToken("good")
	fingerprint = "fp-good"
	b.last = map[string]time.Time{}
	before := claimsOnDisk(t, repo, "pt-a").LeaseUntil
	require.NoError(t, b.beat(ctx))
	require.NoError(t, repo.Pull(t.Context()))
	assert.True(t, claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time().After(before.Time()), "renewed again")
	assert.False(t, b.access.Blocked(ctx, repo, b.now()))
	assert.Len(t, log.all(), 2)
}

// Without a token to watch (an SSH key), the refusal is trusted for a while only.
func TestAccessWatchWithoutATokenRetriesAfterAWhile(t *testing.T) {
	w := &accessWatch{refused: map[string]accessRefusal{}, fingerprint: func(context.Context, string) string { return "" }}
	repo := teamstate.NewRepo("git@gitlab.example.com:o/t.git", t.TempDir())
	now := time.Now()
	authErr := &teamstate.AuthError{Message: "m", Remote: repo.Remote()}

	assert.False(t, w.Report(t.Context(), repo, nil, assert.AnError, now), "not a refusal of the access: not handled")
	assert.True(t, w.Report(t.Context(), repo, nil, authErr, now))
	assert.True(t, w.Blocked(t.Context(), repo, now.Add(time.Minute)))
	assert.False(t, w.Blocked(t.Context(), repo, now.Add(accessRetryAfter+time.Minute)), "the key may have been loaded since")
}

// A team joined after the daemon started is beaten at the next step: the loop
// no longer depends on what the teams were at the start.
func TestClaimsBeatFollowsTheTeamsJoinedAfterTheStart(t *testing.T) {
	a, repo := setupBeat(t, openSession("ses_1", "pt-a"))
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	before := claimsOnDisk(t, repo, "pt-a").LeaseUntil
	real := a.Config
	readHubConfig = func() (*config.Config, error) { return &config.Config{}, nil } // no team yet
	t.Cleanup(func() { readHubConfig = config.Read })

	var opts daemon.Options
	claimsDaemonOptions(t.Context(), a, &opts)
	assert.Equal(t, claimsBeatStep, opts.ClaimsHeartbeatEvery, "the loop runs even with no team at the start")
	b := newClaimsBeater(a)
	b.now = func() time.Time { return time.Now().Add(13 * time.Hour) }
	require.NoError(t, b.beat(t.Context()))
	assert.Equal(t, before, claimsOnDisk(t, repo, "pt-a").LeaseUntil, "no team: nothing renewed")

	readHubConfig = func() (*config.Config, error) { return real, nil } // the team is joined
	require.NoError(t, b.beat(t.Context()))
	assert.True(t, claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time().After(before.Time()), "renewed at the next step")
}
