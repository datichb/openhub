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
	assert.Equal(t, time.Hour, opts.ClaimsHeartbeatEvery, "heartbeat_minutes of the team (default 60)")

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
	assert.Zero(t, b.every(context.Background()), "no heartbeat loop")
	require.NoError(t, b.beat(context.Background()))
	assert.Equal(t, before, claimsOnDisk(t, repo, "pt-a").LeaseUntil, "no renewal")
	claimsGone(t, repo, "pt-gone")
}

// Two teams: one call per team repository, each with its own claims; the
// loop beats at the shortest period of the two.
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
	assert.Equal(t, 30*time.Minute, b.every(context.Background()))
	require.NoError(t, b.beat(context.Background()))
	lease := func(repo *teamstate.Repo, project, id string) time.Time {
		c, err := repo.GetClaim(project, id)
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
