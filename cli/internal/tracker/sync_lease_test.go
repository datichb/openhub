package tracker

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// leaseTeam is a team-state shared by two members, each with a clone of their
// own, an engine and a sync state of their own (their machine).
type leaseTeam struct {
	bare   string
	tr     *stubTracker
	cfg    teamstate.TrackerConfig
	clones map[string]*teamstate.Repo
	paths  map[string]string
	states map[string]string
}

func newLeaseTeam(t *testing.T) *leaseTeam {
	t.Helper()
	bare, aliceClone := gittest.BareClone(t)
	alice := teamstate.NewRepo(bare, aliceClone)
	require.NoError(t, alice.AddMember(t.Context(), teamstate.Member{ID: "alice", DisplayName: "Alice"}))
	require.NoError(t, alice.AddMember(t.Context(), teamstate.Member{ID: "bob", DisplayName: "Bob"}))
	_, err := alice.CreateClaim(t.Context(), teamstate.Claim{TicketID: "1", Project: "p", ClaimedBy: "alice", ExternalIID: 1})
	require.NoError(t, err)
	bobClone := filepath.Join(t.TempDir(), "bob")
	gittest.Run(t, filepath.Dir(bobClone), "clone", "-q", bare, bobClone)
	return &leaseTeam{
		bare: bare,
		tr:   &stubTracker{issues: map[int]IssueState{1: {IID: 1, State: "open", StatusName: "opened", Title: "first"}}},
		cfg: teamstate.TrackerConfig{Type: "gitlab", AutoSync: true, SyncIntervalMinutes: 5,
			Projects: map[string]string{"p": "grp/p"}}, //nolint:staticcheck // the engine still reads Projects
		clones: map[string]*teamstate.Repo{"alice": alice, "bob": teamstate.NewRepo(bare, bobClone)},
		paths:  map[string]string{"alice": aliceClone, "bob": bobClone},
		states: map[string]string{"alice": t.TempDir(), "bob": t.TempDir()},
	}
}

func (l *leaseTeam) engine(member string) *Engine {
	e := NewEngine(l.tr, l.clones[member], l.cfg, l.states[member])
	e.SetIdentity(Identity{TeamID: "core", MemberID: member, TokenFingerprint: TokenFingerprint("tok")})
	return e
}

func (l *leaseTeam) lock(t *testing.T) *teamstate.SyncLock {
	t.Helper()
	require.NoError(t, l.clones["alice"].Pull(t.Context()))
	lock, err := l.clones["alice"].ReadSyncLock()
	require.NoError(t, err)
	return lock
}

// One member synchronizes per interval: the first automatic sync takes the
// lease (member and expiry, in its own commit), the other finds it taken and
// changes nothing; once it has expired the other one takes it over.
func TestAutoSyncHasOneMemberPerInterval(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	l := newLeaseTeam(t)

	res, err := l.engine("alice").RunAuto(t.Context())
	require.NoError(t, err)
	require.NotNil(t, res)
	lock := l.lock(t)
	require.NotNil(t, lock)
	assert.Equal(t, "alice", lock.Member)
	assert.True(t, lock.Held(time.Now()))
	assert.InDelta(t, 5*time.Minute, lock.ExpiresAt.Time().Sub(lock.AcquiredAt.Time()), float64(2*time.Second))

	before := gittest.Run(t, l.bare, "rev-parse", "HEAD")
	_, err = l.engine("bob").RunAuto(t.Context())
	var held *LeaseHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, "alice", held.Member)
	assert.Equal(t, before, gittest.Run(t, l.bare, "rev-parse", "HEAD"), "the other member pushed nothing")

	// The lease of alice is over: bob takes it.
	past := teamstate.NewSyncLock("alice", time.Now().Add(-10*time.Minute), 5*time.Minute)
	require.NoError(t, os.WriteFile(filepath.Join(l.paths["alice"], teamstate.SyncLockFile),
		[]byte("member = 'alice'\nacquired_at = '"+string(past.AcquiredAt)+"'\nexpires_at = '"+string(past.ExpiresAt)+"'\n"), 0o644))
	gittest.Run(t, l.paths["alice"], "commit", "-qam", "lease over")
	gittest.Run(t, l.paths["alice"], "push", "-q")
	require.NoError(t, l.clones["bob"].Pull(t.Context()))
	_, err = l.engine("bob").RunAuto(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "bob", l.lock(t).Member)
}

// Two members whose daemons start together: the push tells them apart, exactly
// one synchronizes.
func TestAutoSyncOfTwoMembersAtOnceHasOneWinner(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	l := newLeaseTeam(t)
	errs := map[string]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, m := range []string{"alice", "bob"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := l.engine(m).RunAuto(t.Context())
			mu.Lock()
			errs[m] = err
			mu.Unlock()
		}()
	}
	wg.Wait()

	winners := 0
	for m, err := range errs {
		var held *LeaseHeldError
		if err == nil {
			winners++
			assert.Equal(t, m, l.lock(t).Member)
			continue
		}
		require.ErrorAs(t, err, &held, m)
	}
	assert.Equal(t, 1, winners)
}

// A manual sync does not wait for the lease and takes it: the automatic syncs
// of the others leave the interval to it.
func TestManualSyncTakesTheLease(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	l := newLeaseTeam(t)
	_, err := l.engine("alice").RunAuto(t.Context())
	require.NoError(t, err)

	_, err = l.engine("bob").Run(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "bob", l.lock(t).Member)

	_, err = l.engine("alice").RunAuto(t.Context())
	var held *LeaseHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, "bob", held.Member)
}

// A team without auto_sync has no lease: nothing is written, an automatic run
// is refused.
func TestSyncWithoutAutoSyncWritesNoLease(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	l := newLeaseTeam(t)
	l.cfg.AutoSync = false
	_, err := l.engine("alice").Run(t.Context())
	require.NoError(t, err)
	assert.Nil(t, l.lock(t))
	_, err = l.engine("alice").RunAuto(t.Context())
	assert.ErrorIs(t, err, ErrAutoSyncDisabled)
}

// A sync the server stopped (refused token) takes no lease: a member with a bad
// token does not keep the others from synchronizing. Three refusals in a row
// mute the automatic runs of that token.
func TestAutoSyncStoppedByTheTokenTakesNoLeaseAndIsMuted(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	l := newLeaseTeam(t)
	bad := &refusingTracker{stubTracker: *l.tr, refuse: map[string]int{"grp/p": http.StatusUnauthorized}}
	l.tr = &bad.stubTracker
	engine := func() *Engine {
		e := NewEngine(bad, l.clones["bob"], l.cfg, l.states["bob"])
		e.SetIdentity(Identity{TeamID: "core", MemberID: "bob", TokenFingerprint: TokenFingerprint("tok")})
		return e
	}
	for range 3 {
		_, err := engine().RunAuto(t.Context())
		require.ErrorIs(t, err, ErrTokenInvalid)
	}
	assert.Nil(t, l.lock(t), "no lease taken")
	_, err := engine().RunAuto(t.Context())
	assert.ErrorIs(t, err, ErrAutoSyncMuted)
	// Alice, with a good token, is not held back.
	_, err = l.engine("alice").RunAuto(t.Context())
	require.NoError(t, err)
}
