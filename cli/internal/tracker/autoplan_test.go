package tracker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// limitTracker lists the issues assigned to alice, newest first, the way
// GitLab does: it honours updated_after and the maximum number of results.
type limitTracker struct {
	stubTracker
	assigned []IssueState
	asked    []ListOpts
}

func (a *limitTracker) ListAssignedIssues(_ context.Context, _ string, opts ListOpts) ([]IssueState, error) {
	a.asked = append(a.asked, opts)
	var out []IssueState
	for _, is := range a.assigned {
		if !opts.UpdatedAfter.IsZero() && !is.UpdatedAt.After(opts.UpdatedAfter) {
			continue
		}
		out = append(out, is)
		if opts.MaxResults > 0 && len(out) >= opts.MaxResults {
			break
		}
	}
	return out, nil
}

func issueOf(iid int, labels ...string) IssueState {
	return IssueState{IID: iid, State: "open", StatusName: "opened", Title: "issue " + strconv.Itoa(iid),
		Labels: labels, Assignees: []string{"alice"}, UpdatedAt: time.Now().Add(-48 * time.Hour)}
}

type autoplanFixture struct {
	repo     *teamstate.Repo
	tr       *limitTracker
	stateDir string
	cfg      teamstate.TrackerConfig
}

func newAutoplanFixture(t *testing.T, assigned ...IssueState) *autoplanFixture {
	t.Helper()
	i18ntest.Set(t, "en")
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "alice", DisplayName: "Alice", GitLabUsername: "alice"}))
	tr := &limitTracker{stubTracker: stubTracker{issues: map[int]IssueState{}}, assigned: assigned}
	for _, is := range assigned {
		tr.issues[is.IID] = is
	}
	return &autoplanFixture{repo: repo, tr: tr, stateDir: t.TempDir(), cfg: teamstate.TrackerConfig{
		AutoPlanAssigned:   true,
		Projects:           map[string]string{"p": "grp/p"}, //nolint:staticcheck // the engine still reads Projects
		StatusMapping:      map[string]string{"opened": teamstate.ClaimStatusPlanned},
		LabelStatusMapping: map[string]string{"DEV DOING": teamstate.ClaimStatusInProgress},
	}}
}

func (f *autoplanFixture) run(t *testing.T) *SyncResult {
	t.Helper()
	res, err := NewEngine(f.tr, f.repo, f.cfg, f.stateDir).Run(t.Context())
	require.NoError(t, err)
	return res
}

func (f *autoplanFixture) claims(t *testing.T) map[string]string {
	t.Helper()
	list, err := f.repo.ListClaims(t.Context(), "p")
	require.NoError(t, err)
	out := map[string]string{}
	for _, c := range list {
		out[c.TicketID] = c.Status
	}
	return out
}

func warningsAbout(res *SyncResult, sub string) []SyncWarning {
	var out []SyncWarning
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, sub) {
			out = append(out, w)
		}
	}
	return out
}

// The sync reflects the tracker: every open issue assigned to a member comes
// onto the board, with no cap per member, in the column its status maps to
// (#623 in DEV DOING is the oldest and is on the board all the same). Nothing
// is held back, so nothing is said.
func TestAutoPlanPutsEveryAssignedIssueOnTheBoard(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t,
		issueOf(617), issueOf(616), issueOf(615), issueOf(614), issueOf(613), issueOf(612), issueOf(611), issueOf(610),
		issueOf(623, "DEV DOING"))

	res := f.run(t)
	got := f.claims(t)
	assert.Len(t, got, 9, "all the assigned issues, not five of them")
	assert.Equal(t, teamstate.ClaimStatusInProgress, got["623"])
	for _, id := range []string{"617", "616", "615", "614", "613", "612", "611", "610"} {
		assert.Equal(t, teamstate.ClaimStatusPlanned, got[id], id)
	}
	assert.Equal(t, 9, res.ClaimsCreated)
	assert.Empty(t, res.Warnings, "nothing is left off the board, nothing is said")

	// Nothing left to do: nothing created again.
	res = f.run(t)
	assert.Len(t, f.claims(t), 9)
	assert.Zero(t, res.ClaimsCreated)
}

// A closed issue and one that already has a claim are not planned.
func TestAutoPlanSkipsClosedAndClaimedIssues(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	closed := issueOf(612)
	closed.State = "closed"
	f := newAutoplanFixture(t, issueOf(615), issueOf(614), closed)
	_, err := f.repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "614", Project: "p", ClaimedBy: "alice",
		Status: teamstate.ClaimStatusPlanned, ExternalIID: 614})
	require.NoError(t, err)
	f.tr.issues[614] = issueOf(614)

	res := f.run(t)
	got := f.claims(t)
	assert.Equal(t, teamstate.ClaimStatusPlanned, got["615"])
	assert.Contains(t, got, "614", "the claim stays")
	assert.NotContains(t, got, "612", "a closed issue is not planned")
	assert.Equal(t, 1, res.ClaimsCreated)
}

// A claim released on purpose is not planned again by the runs that follow.
func TestAutoPlanKeepsAClaimReleasedOnPurposeReleased(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, issueOf(617), issueOf(616), issueOf(615))
	f.run(t)
	assert.Len(t, f.claims(t), 3)
	require.NoError(t, f.repo.ReleaseClaim(t.Context(), "p", "617"))
	f.run(t)
	got := f.claims(t)
	assert.NotContains(t, got, "617", "the claim released on purpose stays released")
	assert.Contains(t, got, "615")
	assert.Contains(t, got, "616")
}

// The real case: the board is full of planned claims, a project already synced
// (so the runs only look at the issues updated since), and #623 assigned to
// the member, in DEV DOING, untouched for weeks. The first complete scan finds
// it; the next runs only look at what was updated.
func TestAutoPlanFindsAnOldIssueInProgressOfAProjectAlreadySynced(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, issueOf(615), issueOf(614), issueOf(623, "DEV DOING"))
	for _, id := range []int{611, 612, 613} {
		_, err := f.repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: strconv.Itoa(id), Project: "p", ClaimedBy: "alice",
			Status: teamstate.ClaimStatusPlanned, ExternalIID: id})
		require.NoError(t, err)
		f.tr.issues[id] = issueOf(id)
	}
	// A previous sync (before the first complete scan) saw the project.
	state := &SyncState{LastSyncAt: map[string]time.Time{}}
	state.SetLastSync(TypeGitLab, "grp/p", time.Now().Add(-time.Hour))
	require.NoError(t, state.Save(f.stateDir))
	f.cfg.Type = string(TypeGitLab)

	res := f.run(t)
	got := f.claims(t)
	assert.Equal(t, teamstate.ClaimStatusInProgress, got["623"], "found by the first complete scan")
	assert.Equal(t, teamstate.ClaimStatusPlanned, got["615"])
	assert.Equal(t, teamstate.ClaimStatusPlanned, got["614"])
	assert.Equal(t, 3, res.ClaimsCreated)

	// The scan is made once: the next run only looks at what was updated.
	f.tr.asked = nil
	f.run(t)
	require.NotEmpty(t, f.tr.asked)
	assert.False(t, f.tr.asked[0].UpdatedAfter.IsZero(), "an incremental fetch from then on")
}

// oh 5.1.0 kept the issues its per-member cap left off the board in the sync
// state. They are untouched on the tracker since: an incremental fetch would
// never see them. Reading such a state makes the next run scan in full, which
// puts them on the board.
func TestAutoPlanScansInFullAgainAfterA510State(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, issueOf(615), issueOf(614))
	f.cfg.Type = string(TypeGitLab)
	last := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	scanned := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	legacy := `{"last_sync_at":{"gitlab/grp/p":"` + last + `"},"autoplan_scanned":{"gitlab/grp/p":"` + scanned + `"},` +
		`"autoplan_held":{"p":{"limit":1,"issues":[{"ticket_id":"614","external_iid":614,"member_id":"alice"}]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(f.stateDir, "sync-state.json"), []byte(legacy), 0o600))

	res := f.run(t)
	got := f.claims(t)
	assert.Contains(t, got, "615")
	assert.Contains(t, got, "614", "the issue the old cap held comes onto the board")
	assert.Equal(t, 2, res.ClaimsCreated)

	data, err := os.ReadFile(filepath.Join(f.stateDir, "sync-state.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "autoplan_held", "the old field is dropped at the next write")
	assert.Contains(t, string(data), "autoplan_scanned", "and the full scan is recorded again")
}
