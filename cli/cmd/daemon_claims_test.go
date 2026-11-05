package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// runningSessions answers ListRunning with a fixed list.
type runningSessions struct {
	domain.SessionStore
	list []domain.Session
}

func (r runningSessions) ListRunning(context.Context, string) ([]domain.Session, error) {
	return r.list, nil
}

func strp(s string) *string { return &s }

// beaterOf is the heartbeat of the claims of a, fixed at now.
func beaterOf(a *app.App, now time.Time) *claimsBeater {
	b := newClaimsBeater(a)
	b.config = func() *config.Config { return a.Config }
	b.now = func() time.Time { return now }
	return b
}

// teamProject links a project to the team of the test app.
func teamProject(id string) domain.Project {
	return domain.Project{ID: id, Name: id, Path: "/p/" + id, TeamID: strp("test-team")}
}

func setupBeat(t *testing.T, sessions ...domain.Session) (*app.App, *teamstate.Repo) {
	t.Helper()
	a, repo, _ := setupClaimTestApp(t)
	a.Projects = &mockProjectStore{projects: []domain.Project{teamProject(claimsProject)}}
	a.Sessions = runningSessions{list: sessions}
	return a, repo
}

func openSession(id string, tickets ...string) domain.Session {
	return domain.Session{ID: id, ProjectID: claimsProject, Status: domain.SessionStatusRunning, State: domain.RunActive, Tickets: tickets}
}

// The beat renews the lease of the claims of the open sessions, in one commit,
// and only those.
func TestClaimsBeatRenewsTheLeasesOfOpenSessions(t *testing.T) {
	ended := openSession("ses_old", "pt-ended")
	ended.State = domain.RunStopped
	a, repo := setupBeat(t, openSession("ses_1", "pt-a", "pt-b"), ended)
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-b", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-ended", "testuser", "ses_old")
	claimsCreate(t, repo, "pt-nosession", "testuser", "")
	before, endedLease := claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time(), claimsOnDisk(t, repo, "pt-ended").LeaseUntil
	require.False(t, before.IsZero())
	commits := commitsOf(t, repo.Path())

	later := time.Now().Add(13 * time.Hour) // more than half of the lease is gone
	require.NoError(t, beaterOf(a, later).beat(context.Background()))

	for _, id := range []string{"pt-a", "pt-b"} {
		assert.WithinDuration(t, later.Add(24*time.Hour), claimsOnDisk(t, repo, id).LeaseUntil.Time(), 2*time.Second, id)
	}
	assert.Equal(t, endedLease, claimsOnDisk(t, repo, "pt-ended").LeaseUntil, "a session that is over renews nothing")
	assert.Equal(t, commits+1, commitsOf(t, repo.Path()), "one commit for the two claims")
}

// The beat empties the queue even when no session is open: what a session
// left when it ended offline is applied at the next beat.
func TestClaimsBeatEmptiesTheQueueWithoutOpenSession(t *testing.T) {
	a, repo := setupBeat(t)
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-open"},
		Ticket: lookup(map[string]ticketFacts{"pt-open": {}}), State: staticState(sessionEndState{}),
	}
	online := goOffline(t, repo)
	r := endSessionClaims(context.Background(), repo, in, time.Second, 20*time.Second)
	require.ErrorIs(t, r.Flush.Err, teamstate.ErrOffline)

	b := beaterOf(a, time.Now())
	require.NoError(t, b.beat(context.Background()), "a remote that does not answer is not an error of the beat")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 1)

	online()
	require.NoError(t, b.beat(context.Background()))
	claimsGone(t, repo, "pt-open")
	ops, err = repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)
}

// The claims-queue task does the same on its own, at the start of the daemon.
func TestClaimsQueueTaskEmptiesTheQueue(t *testing.T) {
	a, repo := setupBeat(t)
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	require.NoError(t, repo.EnqueueClaimOp(teamstate.ClaimOp{Kind: teamstate.ClaimOpRelease, Project: claimsProject, TicketID: "pt-open", ClaimedBy: "testuser", Session: "ses_1"}))

	readHubConfig = func() (*config.Config, error) { return a.Config, nil }
	t.Cleanup(func() { readHubConfig = config.Read })
	var opts daemon.Options
	claimsDaemonOptions(context.Background(), a, &opts)
	require.Len(t, opts.Tasks, 1)
	assert.Equal(t, "claims-queue", opts.Tasks[0].Name)
	assert.NotNil(t, opts.OnClaimsHeartbeat)
	assert.Equal(t, claimsBeatStep, opts.ClaimsHeartbeatEvery, "a fixed step: the period of each team is read at every beat (due)")

	opts.Tasks[0].Run(context.Background())
	claimsGone(t, repo, "pt-open")
}

// A touch that could not be pushed is queued and applied by a later beat.
func TestClaimsBeatOfflineQueuesTheRenewal(t *testing.T) {
	a, repo := setupBeat(t, openSession("ses_1", "pt-a"))
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	later := time.Now().Add(13 * time.Hour)
	online := goOffline(t, repo)

	b := beaterOf(a, later)
	require.NoError(t, b.beat(context.Background()))
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, teamstate.ClaimOpTouch, ops[0].Kind)

	online()
	b.last = map[string]time.Time{}
	require.NoError(t, b.beat(context.Background()))
	assert.WithinDuration(t, later.Add(24*time.Hour), claimsOnDisk(t, repo, "pt-a").LeaseUntil.Time(), 2*time.Second)
}

// A team with heartbeat_minutes = 0 is never touched, but its queue is emptied.
func TestClaimsBeatHeartbeatOff(t *testing.T) {
	a, repo := setupBeat(t, openSession("ses_1", "pt-a"))
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte("[claim]\nheartbeat_minutes = 0\n"), 0o644))
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-gone", "testuser", "ses_old")
	require.NoError(t, repo.EnqueueClaimOp(teamstate.ClaimOp{Kind: teamstate.ClaimOpRelease, Project: claimsProject, TicketID: "pt-gone", ClaimedBy: "testuser", Session: "ses_old"}))
	before := claimsOnDisk(t, repo, "pt-a").LeaseUntil

	b := beaterOf(a, time.Now().Add(13*time.Hour))
	require.NoError(t, b.beat(context.Background()))
	assert.Equal(t, before, claimsOnDisk(t, repo, "pt-a").LeaseUntil, "no renewal")
	claimsGone(t, repo, "pt-gone")
}

// Two teams: one call per team repository, each with its own claims.
func TestClaimsBeatSeveralTeams(t *testing.T) {
	a, repo1 := setupBeat(t, openSession("ses_1", "pt-a"))
	bare2, clone2 := claimRepos(t)
	repo2 := teamstate.NewRepo(bare2, clone2)
	require.NoError(t, os.WriteFile(filepath.Join(clone2, "config.toml"), []byte("[claim]\nheartbeat_minutes = 30\n"), 0o644))
	a.Config.Teams = append(a.Config.Teams, config.TeamConfig{ID: "team-2", Enabled: true, StateRepo: bare2, StatePath: clone2, MemberID: "testuser"})
	other := domain.Project{ID: "otherproj", Name: "otherproj", Path: "/p/other", TeamID: strp("team-2")}
	a.Projects = &mockProjectStore{projects: []domain.Project{teamProject(claimsProject), other}}
	s2 := openSession("ses_2", "pt-b")
	s2.ProjectID = "otherproj"
	a.Sessions = runningSessions{list: []domain.Session{openSession("ses_1", "pt-a"), s2}}
	claimsCreate(t, repo1, "pt-a", "testuser", "ses_1")
	_, err := repo2.CreateClaim(context.Background(), teamstate.Claim{TicketID: "pt-b", Project: "otherproj", ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress, Session: "ses_2"})
	require.NoError(t, err)

	start := time.Now().Add(13 * time.Hour)
	b := beaterOf(a, start)
	require.NoError(t, b.beat(context.Background()))
	lease := func(repo *teamstate.Repo, project, id string) time.Time {
		c, err := repo.GetClaim(t.Context(), project, id)
		require.NoError(t, err)
		return c.LeaseUntil.Time()
	}
	assert.WithinDuration(t, start.Add(24*time.Hour), lease(repo1, claimsProject, "pt-a"), 2*time.Second)
	assert.WithinDuration(t, start.Add(24*time.Hour), lease(repo2, "otherproj", "pt-b"), 2*time.Second)
}

func TestClaimsBeatDue(t *testing.T) {
	b := newClaimsBeater(nil)
	now := time.Now()
	assert.True(t, b.due("a", time.Hour, now), "never beaten")
	b.last["a"] = now
	assert.False(t, b.due("a", time.Hour, now.Add(30*time.Minute)), "half of a period")
	assert.True(t, b.due("a", 30*time.Minute, now.Add(30*time.Minute)))
	assert.True(t, b.due("a", time.Hour, now.Add(59*time.Minute)), "timer jitter")
	assert.False(t, b.due("a", 0, now.Add(24*time.Hour)), "heartbeat off")
}

// The renewal has a ceiling: a session asleep for more than three days and a
// remote session launched more than seven days ago stop renewing their claims,
// which then expire normally. The ceilings are settings of the team.
func TestClaimsBeatStopsRenewingPastTheCeilings(t *testing.T) {
	now := time.Now()
	sleeping := func(id string, since time.Duration, tickets ...string) domain.Session {
		s := openSession(id, tickets...)
		s.State = domain.RunSleeping
		at := now.Add(-since)
		s.StateChangedAt = &at
		return s
	}
	remote := func(id string, age time.Duration, tickets ...string) domain.Session {
		s := openSession(id, tickets...)
		s.Runtime = "remote"
		s.StartedAt = now.Add(-age)
		return s
	}
	day := 24 * time.Hour
	sessions := []domain.Session{
		sleeping("ses_s2", 2*day, "t-sleep-2d"), sleeping("ses_s4", 4*day, "t-sleep-4d"),
		remote("ses_r6", 6*day, "t-remote-6d"), remote("ses_r8", 8*day, "t-remote-8d"),
		openSession("ses_l", "t-local"),
	}
	tickets := []string{"t-sleep-2d", "t-sleep-4d", "t-remote-6d", "t-remote-8d", "t-local"}
	owner := map[string]string{"t-sleep-2d": "ses_s2", "t-sleep-4d": "ses_s4", "t-remote-6d": "ses_r6", "t-remote-8d": "ses_r8", "t-local": "ses_l"}

	run := func(t *testing.T, claim teamstate.ClaimConfig) map[string]bool {
		t.Helper()
		a, repo := setupBeat(t, sessions...)
		for _, id := range tickets {
			claimsCreate(t, repo, id, "testuser", owner[id])
		}
		require.NoError(t, repo.SaveConfig(context.Background(), &teamstate.TeamConfig{Claim: claim}))
		later := now.Add(13 * time.Hour) // more than half of the lease is gone
		require.NoError(t, beaterOf(a, later).beat(context.Background()))
		renewed := map[string]bool{}
		for _, id := range tickets {
			renewed[id] = claimsOnDisk(t, repo, id).LeaseUntil.Time().After(now.Add(30 * time.Hour))
		}
		return renewed
	}

	t.Run("defaults: 3 days asleep, 7 days remote", func(t *testing.T) {
		assert.Equal(t, map[string]bool{
			"t-sleep-2d": true, "t-sleep-4d": false, "t-remote-6d": true, "t-remote-8d": false, "t-local": true,
		}, run(t, teamstate.ClaimConfig{LeaseHours: 24, HeartbeatMinutes: 60}))
	})
	t.Run("settings of the team", func(t *testing.T) {
		assert.Equal(t, map[string]bool{
			"t-sleep-2d": false, "t-sleep-4d": false, "t-remote-6d": true, "t-remote-8d": true, "t-local": true,
		}, run(t, teamstate.ClaimConfig{LeaseHours: 24, HeartbeatMinutes: 60, SleepingRenewalDays: 1, RemoteRenewalDays: 10}))
	})
}

func TestRenewableSession(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	day := 24 * time.Hour
	cfg := teamstate.ClaimConfig{}
	tests := []struct {
		name string
		s    domain.Session
		want bool
	}{
		{"active", domain.Session{State: domain.RunActive, StartedAt: now.Add(-30 * day)}, true},
		{"asleep for two days", domain.Session{State: domain.RunSleeping, StateChangedAt: at(2 * day)}, true},
		{"asleep for exactly three days", domain.Session{State: domain.RunSleeping, StateChangedAt: at(3 * day)}, true},
		{"asleep for more than three days", domain.Session{State: domain.RunSleeping, StateChangedAt: at(3*day + time.Minute)}, false},
		{"asleep, date unknown: the start of the session", domain.Session{State: domain.RunSleeping, StartedAt: now.Add(-4 * day)}, false},
		{"asleep, no date at all: kept", domain.Session{State: domain.RunSleeping}, true},
		{"remote, six days", domain.Session{Runtime: "remote", State: domain.RunActive, StartedAt: now.Add(-6 * day)}, true},
		{"remote, eight days", domain.Session{Runtime: "remote", State: domain.RunActive, StartedAt: now.Add(-8 * day)}, false},
		{"remote asleep for a day, launched eight days ago", domain.Session{Runtime: "remote", State: domain.RunSleeping, StateChangedAt: at(day), StartedAt: now.Add(-8 * day)}, false},
		{"local for ten days", domain.Session{Runtime: "local", State: domain.RunActive, StartedAt: now.Add(-10 * day)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, renewable(tt.s, cfg, now))
		})
	}
}
