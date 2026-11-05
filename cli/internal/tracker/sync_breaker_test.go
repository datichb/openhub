package tracker

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// refusingTracker answers FetchIssue of a tracker project with a status code:
// the others with the issues it knows.
type refusingTracker struct {
	stubTracker
	refuse map[string]int
}

func (r *refusingTracker) FetchIssue(ctx context.Context, project string, iid int) (*IssueState, error) {
	if status, ok := r.refuse[project]; ok {
		return nil, &TokenRejectedError{Origin: TokenOrigin{Server: "https://gitlab.example"}, Status: status}
	}
	return r.stubTracker.FetchIssue(ctx, project, iid)
}

func newRefusalFixture(t *testing.T, refuse map[string]int) (*teamstate.Repo, *refusingTracker, teamstate.TrackerConfig) {
	t.Helper()
	i18ntest.Set(t, "en")
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "alice", DisplayName: "Alice"}))
	for _, project := range []string{"a", "b"} {
		_, err := repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "1", Project: project, ClaimedBy: "alice", ExternalIID: 1})
		require.NoError(t, err)
	}
	tr := &refusingTracker{stubTracker: stubTracker{issues: map[int]IssueState{1: {IID: 1, State: "open", StatusName: "opened", Title: "first"}}}, refuse: refuse}
	cfg := teamstate.TrackerConfig{Type: "gitlab", Projects: map[string]string{"a": "grp/denied", "b": "grp/ok"}} //nolint:staticcheck // the engine still reads Projects
	return repo, tr, cfg
}

// A 403 on one tracker project is an error of that project: the others are
// synchronized and the run goes through. A 401 stops the run.
func TestSyncForbiddenProjectDoesNotStopTheOthers(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	repo, tr, cfg := newRefusalFixture(t, map[string]int{"grp/denied": http.StatusForbidden})
	stateDir := t.TempDir()
	e := NewEngine(tr, repo, cfg, stateDir)
	e.SetIdentity(Identity{TeamID: "core", TokenFingerprint: TokenFingerprint("tok")})

	res, err := e.Run(t.Context())
	require.NoError(t, err, "a refusal of access is not a refused token")
	require.Len(t, res.Errors, 1)
	assert.Equal(t, "a", res.Errors[0].ProjectID)
	assert.True(t, IsAccessDenied(res.Errors[0].Err))
	got, err := repo.GetClaim(t.Context(), "b", "1")
	require.NoError(t, err)
	assert.Equal(t, "first", got.Title, "the other project was synchronized")

	state, err := LoadSyncState(stateDir)
	require.NoError(t, err)
	assert.Empty(t, state.TokenFailures, "a 403 does not count against the token")
}

// Three refusals of the same token in a row mute the automatic sync, whatever
// the engine: the count is in the sync state. Another token, or a run that
// gets through, lifts it.
func TestSyncCircuitBreakerIsPersisted(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	repo, tr, cfg := newRefusalFixture(t, map[string]int{"grp/denied": http.StatusUnauthorized, "grp/ok": http.StatusUnauthorized})
	stateDir := t.TempDir()
	engine := func(token string) *Engine {
		e := NewEngine(tr, repo, cfg, stateDir)
		e.SetIdentity(Identity{TeamID: "core", TokenFingerprint: TokenFingerprint(token)})
		return e
	}

	for i := range 3 {
		assert.True(t, engine("old").ShouldAutoSync(), "run %d", i)
		_, err := engine("old").Run(t.Context())
		require.ErrorIs(t, err, ErrTokenInvalid)
	}
	assert.False(t, engine("old").ShouldAutoSync(), "a new engine (a new process) knows the token was refused three times")
	assert.True(t, engine("new").ShouldAutoSync(), "another token is tried")

	// A manual sync tries anyway; one that gets through lifts the stop.
	tr.refuse = nil
	_, err := engine("old").Run(t.Context())
	require.NoError(t, err)
	assert.True(t, engine("old").ShouldAutoSync())
	state, err := LoadSyncState(stateDir)
	require.NoError(t, err)
	assert.Empty(t, state.TokenFailures)
}

func TestTokenFailuresStartOverWithAnotherToken(t *testing.T) {
	s := &SyncState{}
	now := time.Now()
	s.NoteTokenFailure("core/gitlab", "a", now)
	s.NoteTokenFailure("core/gitlab", "a", now)
	s.NoteTokenFailure("core/gitlab", "b", now)
	assert.Equal(t, 1, s.TokenFailures["core/gitlab"].Count)
	assert.False(t, s.TokenMuted("core/gitlab", "b"))
	assert.Empty(t, TokenFingerprint(""))
	assert.NotContains(t, TokenFingerprint("secret"), "secret")
}

// A different assignee on the tracker than the owner of the claim is said, once
// per run, instead of being counted as an update.
func TestSyncSaysWhenTheTrackerAssigneeIsNotTheOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t)
	ctx := t.Context()
	require.NoError(t, f.repo.AddMember(ctx, teamstate.Member{ID: "bob", DisplayName: "Bob", GitLabUsername: "bob"}))
	_, err := f.repo.CreateClaim(ctx, teamstate.Claim{TicketID: "5", Project: "p", ClaimedBy: "alice", ExternalIID: 5})
	require.NoError(t, err)
	is := issueOf(5)
	is.Assignees = []string{"bob"}
	f.tr.issues[5] = is
	f.cfg.AutoPlanAssigned = false

	res := f.run(t)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0].Message, "assigned to bob")
	assert.Equal(t, "p", res.Warnings[0].ProjectID)
	assert.Zero(t, res.Projects[0].ClaimsUpdated, "not an update")
}
