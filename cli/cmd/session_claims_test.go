package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// lookup answers for the tickets it knows; the others are unknown.
func lookup(tickets map[string]ticketFacts) ticketLookup {
	return func(id string) (ticketFacts, error) {
		if tk, ok := tickets[id]; ok {
			return tk, nil
		}
		return ticketFacts{}, fmt.Errorf("unknown ticket")
	}
}

func staticState(st sessionEndState) func(context.Context) sessionEndState {
	return func(context.Context) sessionEndState { return st }
}

func commitsOf(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(gittest.Run(t, dir, "rev-list", "--count", "HEAD"))
	require.NoError(t, err)
	return n
}

func goOffline(t *testing.T, repo *teamstate.Repo) (online func()) {
	t.Helper()
	remote := gittest.Run(t, repo.Path(), "remote", "get-url", "origin")
	gittest.Run(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/x.git")
	return func() { gittest.Run(t, repo.Path(), "remote", "set-url", "origin", remote) }
}

// A31, v5.1 (B3a): at the end of a session, the claim of a closed ticket is
// completed, one in review stays, one with a checkpoint pending or the commit
// put off with work left stays labelled session-ended, the others are
// released; the claims of other members, of another session and of unknown
// tickets are left as they are.
func TestEndTicketClaims(t *testing.T) {
	ctx := context.Background()
	tickets := map[string]ticketFacts{
		"pt-closed":  {Closed: true},
		"pt-review":  {Labels: []string{"ai-delegated", "ready-for-review"}},
		"pt-open":    {},
		"pt-legacy":  {},
		"pt-bob":     {Closed: true},
		"pt-other":   {},
		"pt-done":    {},
		"pt-already": {},
	}
	setup := func(t *testing.T) *teamstate.Repo {
		_, repo, _ := setupClaimTestApp(t)
		claimsCreate(t, repo, "pt-closed", "testuser", "ses_1")
		claimsCreate(t, repo, "pt-review", "testuser", "ses_1")
		claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
		claimsCreate(t, repo, "pt-legacy", "testuser", "") // taken before the session was recorded
		claimsCreate(t, repo, "pt-bob", "bob", "ses_bob")
		claimsCreate(t, repo, "pt-other", "testuser", "ses_other") // another session of the member
		claimsCreate(t, repo, "pt-already", "testuser", "ses_1")
		require.NoError(t, repo.AddClaimLabel(ctx, claimsProject, "pt-already", teamstate.LabelSessionEnded))
		claimsCreate(t, repo, "pt-done", "testuser", "ses_1")
		require.NoError(t, repo.UpdateClaimStatusFromTracker(ctx, claimsProject, "pt-done", teamstate.ClaimStatusDone))
		return repo
	}
	run := func(t *testing.T, repo *teamstate.Repo, st sessionEndState) claimsEndReport {
		in := sessionClaimsInput{
			Project: claimsProject, Member: "testuser", Session: "ses_1",
			Tickets: []string{"pt-closed", "pt-review", "pt-open", "pt-legacy", "pt-bob", "pt-other", "pt-gone", "pt-done", "pt-already"},
			Ticket:  lookup(tickets), State: staticState(st),
		}
		return endSessionClaims(ctx, repo, in, time.Second, 30*time.Second)
	}
	outcomes := func(r claimsEndReport) map[string]claimEnd {
		m := map[string]claimEnd{}
		for id, o := range r.Outcomes {
			m[id] = o.End
		}
		return m
	}

	t.Run("nothing pending", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, map[string]claimEnd{
			"pt-closed": claimDone, "pt-review": claimKept, "pt-open": claimReleased, "pt-legacy": claimReleased,
			"pt-done": claimKept, "pt-already": claimReleased,
		}, outcomes(r))
		assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "pt-closed").Status)
		assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "pt-review").Status, "in review: kept as it is")
		claimsGone(t, repo, "pt-open")
		claimsGone(t, repo, "pt-legacy")
		claimsGone(t, repo, "pt-already")
		assert.Equal(t, "bob", claimsOnDisk(t, repo, "pt-bob").ClaimedBy)
		assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "pt-bob").Status, "someone else's claim")
		assert.Equal(t, "ses_other", claimsOnDisk(t, repo, "pt-other").Session, "another session's claim")
		assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "pt-done").Status)
		ops, err := repo.ClaimQueue()
		require.NoError(t, err)
		assert.Empty(t, ops, "the queue is empty")
	})

	t.Run("cp-2 put off with work left", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{Deferred: "cp-2", Work: teamstate.ClaimWorkPresent})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, claimMarked, r.Outcomes["pt-open"].End)
		assert.Equal(t, "cp-2", r.Outcomes["pt-open"].Checkpoint)
		assert.Equal(t, teamstate.ClaimEndCheckpointDeferred, r.Outcomes["pt-open"].Reason)
		c := claimsOnDisk(t, repo, "pt-open")
		assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)
		assert.Equal(t, "ses_1", c.Session)
		assert.Equal(t, []string{teamstate.LabelSessionEnded}, c.Labels)
		assert.Equal(t, claimMarked, r.Outcomes["pt-legacy"].End)
		assert.Equal(t, claimDone, r.Outcomes["pt-closed"].End, "a closed ticket is completed whatever the checkpoints")
		assert.Equal(t, claimKept, r.Outcomes["pt-review"].End, "review wins")
		assert.Equal(t, claimMarked, r.Outcomes["pt-already"].End, "already labelled: nothing to add")
		assert.Equal(t, []string{teamstate.LabelSessionEnded}, claimsOnDisk(t, repo, "pt-already").Labels)
	})

	t.Run("cp-2 put off, nothing done", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{Deferred: "cp-2", Work: teamstate.ClaimWorkNone})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, claimReleased, r.Outcomes["pt-open"].End)
		claimsGone(t, repo, "pt-open")
	})

	t.Run("cp-2 put off, work unknown", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{Deferred: "cp-2"})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, claimMarked, r.Outcomes["pt-open"].End)
	})

	t.Run("checkpoint waiting", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{Waiting: "cp-1"})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, claimMarked, r.Outcomes["pt-open"].End)
		assert.Equal(t, "cp-1", r.Outcomes["pt-open"].Checkpoint)
		assert.Contains(t, claimsOnDisk(t, repo, "pt-open").Labels, teamstate.LabelSessionEnded)
	})

	t.Run("checkpoints unreadable", func(t *testing.T) {
		repo := setup(t)
		r := run(t, repo, sessionEndState{Unreadable: true})
		require.NoError(t, r.Flush.Err)
		assert.Equal(t, teamstate.ClaimEndCheckpointUnknown, r.Outcomes["pt-open"].Reason)
		assert.Contains(t, claimsOnDisk(t, repo, "pt-open").Labels, teamstate.LabelSessionEnded)
	})
}

// The state of the session is read once, and only when a claim needs it.
func TestEndTicketClaimsReadsTheStateLazily(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-closed", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-b", "testuser", "ses_1")
	reads := 0
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-closed"},
		Ticket: lookup(map[string]ticketFacts{"pt-closed": {Closed: true}, "pt-a": {}, "pt-b": {}}),
		State:  func(context.Context) sessionEndState { reads++; return sessionEndState{} },
	}
	endTicketClaims(context.Background(), repo, in)
	assert.Zero(t, reads, "only closed tickets: the state is not read")
	in.Tickets = []string{"pt-a", "pt-b"}
	endTicketClaims(context.Background(), repo, in)
	assert.Equal(t, 1, reads)
}

// Offline, the operations stay in the queue and the session still ends; they
// are applied when the remote answers again, and running the end again
// changes nothing.
func TestEndTicketClaimsOfflineThenOnline(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-closed", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-wait", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1",
		Tickets: []string{"pt-closed", "pt-open", "pt-wait"},
		Ticket:  lookup(map[string]ticketFacts{"pt-closed": {Closed: true}, "pt-open": {}, "pt-wait": {}}),
		State:   staticState(sessionEndState{}),
	}
	online := goOffline(t, repo)

	r := endSessionClaims(ctx, repo, in, time.Second, 20*time.Second)
	require.ErrorIs(t, r.Flush.Err, teamstate.ErrOffline)
	assert.Equal(t, 3, r.Flush.Pending, "one operation per ticket")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 3)
	for _, op := range ops {
		assert.Equal(t, "testuser", op.ClaimedBy)
		assert.Equal(t, "ses_1", op.Session)
	}
	assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "pt-closed").Status, "nothing applied")
	claimsOnDisk(t, repo, "pt-open")

	// A second end (the hook may run twice) queues nothing more.
	r = endSessionClaims(ctx, repo, in, time.Second, 20*time.Second)
	require.ErrorIs(t, r.Flush.Err, teamstate.ErrOffline)
	ops, err = repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 3, "same operations: not queued twice")

	// What the person reads.
	i18ntest.Set(t, "en")
	var errOut bytes.Buffer
	reportClaimsEnd(&app.App{IO: &app.IOStreams{ErrOut: &errOut}}, r)
	assert.Contains(t, errOut.String(), "cannot be reached")
	assert.Contains(t, errOut.String(), "kept in the queue")

	online()
	f := flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	assert.Equal(t, 3, f.Applied)
	assert.Zero(t, f.Dropped)
	assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "pt-closed").Status)
	claimsGone(t, repo, "pt-open")
	ops, err = repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)
}

// Idempotence: ending the same session twice online changes nothing the
// second time (no commit, nothing queued).
func TestEndTicketClaimsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-closed", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-wait", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1",
		Tickets: []string{"pt-closed", "pt-open", "pt-wait"},
		Ticket:  lookup(map[string]ticketFacts{"pt-closed": {Closed: true}, "pt-open": {Labels: []string{"review"}}, "pt-wait": {}}),
		State:   staticState(sessionEndState{Waiting: "cp-1"}),
	}
	first := endSessionClaims(ctx, repo, in, time.Second, 30*time.Second)
	require.NoError(t, first.Flush.Err)
	assert.Equal(t, 2, first.Flush.Applied, "completed + labelled; the review one needs nothing")
	commits := commitsOf(t, repo.Path())

	second := endSessionClaims(ctx, repo, in, time.Second, 30*time.Second)
	require.NoError(t, second.Flush.Err)
	assert.Zero(t, second.Flush.Applied)
	assert.Equal(t, commits, commitsOf(t, repo.Path()), "no commit the second time")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)
	assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "pt-closed").Status)
	assert.Equal(t, []string{teamstate.LabelSessionEnded}, claimsOnDisk(t, repo, "pt-wait").Labels, "the label is not added twice")
}

// A claim another session took over before the queue was flushed is not
// touched: the operation conditioned on the session is dropped.
func TestEndTicketClaimsDropsWhatAnotherSessionTookOver(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-open"},
		Ticket: lookup(map[string]ticketFacts{"pt-open": {}}), State: staticState(sessionEndState{}),
	}
	online := goOffline(t, repo)
	r := endSessionClaims(ctx, repo, in, time.Second, 20*time.Second)
	require.ErrorIs(t, r.Flush.Err, teamstate.ErrOffline)
	online()

	adopted, err := repo.AdoptClaim(ctx, claimsProject, "pt-open", "testuser", "ses_2")
	require.NoError(t, err)
	require.True(t, adopted)

	f := flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	assert.Zero(t, f.Applied)
	assert.Equal(t, 1, f.Dropped)
	assert.Equal(t, "ses_2", claimsOnDisk(t, repo, "pt-open").Session, "the claim of the new session is still there")
}

// The flush is bounded: past the limit the operations stay in the queue.
func TestFlushClaimQueueIsBounded(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-open"},
		Ticket: lookup(map[string]ticketFacts{"pt-open": {}}), State: staticState(sessionEndState{}),
	}
	endTicketClaims(ctx, repo, in)

	f := flushClaimQueue(ctx, repo, time.Second, time.Nanosecond)
	require.Error(t, f.Err)
	assert.Equal(t, 1, f.Pending)
	claimsOnDisk(t, repo, "pt-open")

	i18ntest.Set(t, "en")
	assert.Contains(t, claimFlushNote(claimFlush{Err: context.DeadlineExceeded, Pending: 1}), "did not answer in time")
	assert.Empty(t, claimFlushNote(claimFlush{}))

	f = flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	claimsGone(t, repo, "pt-open")
}

// The label read from Beads and the one of the claims are the same.
func TestReviewLabelsAgree(t *testing.T) {
	assert.Equal(t, beads.LabelReadyForReview, teamstate.LabelReadyForReview)
}

func TestSessionWork(t *testing.T) {
	ctx := context.Background()
	dir := gittest.Repo(t)
	start := gittest.Run(t, dir, "rev-parse", "HEAD")
	s := domain.Session{ID: "ses_1", LaunchPath: dir, StartRef: start}

	assert.Equal(t, teamstate.ClaimWorkNone, sessionWork(ctx, s), "clean")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("{}\n"), 0o644))
	assert.Equal(t, teamstate.ClaimWorkNone, sessionWork(ctx, s), "only the Beads data")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package x\n"), 0o644))
	assert.Equal(t, teamstate.ClaimWorkPresent, sessionWork(ctx, s), "untracked file")

	gittest.Run(t, dir, "add", "new.go")
	gittest.Run(t, dir, "commit", "-q", "-m", "work")
	assert.Equal(t, teamstate.ClaimWorkPresent, sessionWork(ctx, s), "commit since the start")

	s.StartRef = gittest.Run(t, dir, "rev-parse", "HEAD")
	assert.Equal(t, teamstate.ClaimWorkNone, sessionWork(ctx, s), "committed before the start")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644))
	assert.Equal(t, teamstate.ClaimWorkPresent, sessionWork(ctx, s), "modified file")

	assert.Equal(t, teamstate.ClaimWorkUnknown, sessionWork(ctx, domain.Session{LaunchPath: filepath.Join(dir, "nope")}), "unreadable location")
	assert.Equal(t, teamstate.ClaimWorkUnknown, sessionWork(ctx, domain.Session{}), "no location")
	remote := s
	remote.Runtime = "remote"
	assert.Equal(t, teamstate.ClaimWorkUnknown, sessionWork(ctx, remote), "remote runtime: the work is not here")
}

func TestSessionEndStateOfWithoutWorkflow(t *testing.T) {
	st := sessionEndStateOf(nil, domain.Session{ID: "ses_1"})(context.Background())
	assert.Equal(t, sessionEndState{}, st, "no bundle: no workflow, nothing pending")
}

func TestDeferredCommitCheckpoint(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}},
		{ID: "cp-2", Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit}, Behaviors: map[string]string{"manuel": "pause", "auto": "skip"}},
		{ID: "cp-3", Unlocks: []string{sessionspec.UnlockPush}, Behaviors: map[string]string{"manuel": "pause"}},
	}}
	at := time.Now()
	passed := func(ids ...string) *domain.CheckpointState {
		cs := &domain.CheckpointState{Passed: map[string]time.Time{}}
		for _, id := range ids {
			cs.Passed[id] = at
		}
		return cs
	}
	assert.Equal(t, "cp-2", deferredCommitCheckpoint(wf, passed()), "nothing passed")
	assert.Equal(t, "cp-2", deferredCommitCheckpoint(wf, passed("cp-1")), "cp-1 passed, cp-2 put off")
	assert.Empty(t, deferredCommitCheckpoint(wf, passed("cp-1", "cp-2")), "cp-2 passed")
	assert.Empty(t, deferredCommitCheckpoint(&sessionspec.WorkflowRuntime{}, passed()), "no commit checkpoint")
	assert.Equal(t, sessionspec.CheckpointPause, wf.Checkpoints[1].Behavior("auto"), "a commit checkpoint is never skipped")
}
