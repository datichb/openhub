package tracker

import (
	"context"
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

func newAutoplanFixture(t *testing.T, limit int, assigned ...IssueState) *autoplanFixture {
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
		AutoPlanAssigned: true, MaxAutoPlanPerMember: limit,
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
	list, err := f.repo.ListClaims("p")
	require.NoError(t, err)
	out := map[string]string{}
	for _, c := range list {
		out[c.TicketID] = c.Status
	}
	return out
}

func (f *autoplanFixture) heldCount(t *testing.T) (int, int) {
	t.Helper()
	return AutoPlanHeldFor(f.stateDir, []string{"p"})
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

// A2: with more assigned issues than max_auto_plan_per_member, the issues left
// off the board are counted and said; the issues already in progress come
// first, whatever their age (#623 in DEV DOING stayed off the board: the
// fetch stopped at the five newest issues).
func TestAutoPlanSaysWhatTheLimitLeavesOffTheBoardAndStartedIssuesComeFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, 5,
		issueOf(617), issueOf(616), issueOf(615), issueOf(614), issueOf(613), issueOf(612), issueOf(611), issueOf(610),
		issueOf(623, "DEV DOING")) // the oldest, the only one in progress

	res := f.run(t)
	got := f.claims(t)
	assert.Equal(t, teamstate.ClaimStatusInProgress, got["623"], "the issue in progress is on the board though it is the oldest")
	assert.Len(t, got, 5, "the board holds the limit: the started issue and four others")
	for _, id := range []string{"617", "616", "615", "614"} {
		assert.Equal(t, teamstate.ClaimStatusPlanned, got[id], id)
	}
	assert.Equal(t, 4, res.Projects[0].AutoPlanHeld)
	w := warningsAbout(res, "4 issue(s)")
	require.Len(t, w, 1, "the run says it")
	assert.Equal(t, "p", w[0].ProjectID)
	assert.Contains(t, w[0].Message, "5", "with the limit")
	held, limit := f.heldCount(t)
	assert.Equal(t, 4, held)
	assert.Equal(t, 5, limit)

	// Nothing changes while the board is full, and nothing is created twice.
	res = f.run(t)
	assert.Len(t, f.claims(t), 5)
	assert.Zero(t, res.ClaimsCreated)
	assert.Len(t, warningsAbout(res, "4 issue(s)"), 1, "still said")

	// A planned claim released: the next issue held comes in.
	require.NoError(t, f.repo.ReleaseClaim(t.Context(), "p", "617"))
	res = f.run(t)
	got = f.claims(t)
	assert.Equal(t, teamstate.ClaimStatusPlanned, got["613"], "the oldest held issue takes the room")
	assert.NotContains(t, got, "617", "the claim released on purpose stays released")
	assert.Equal(t, 3, res.Projects[0].AutoPlanHeld)

	// Room for everyone: nothing held, nothing said.
	f.cfg.MaxAutoPlanPerMember = 20
	res = f.run(t)
	assert.Zero(t, res.Projects[0].AutoPlanHeld)
	assert.Empty(t, warningsAbout(res, "issue(s)"))
	held, _ = f.heldCount(t)
	assert.Zero(t, held)
	assert.Contains(t, f.claims(t), "610")
}

// The real case: the board is full of planned claims, a project already synced
// (so the runs only look at the issues updated since), and #623 assigned to
// the member, in DEV DOING, untouched for weeks.
func TestAutoPlanFindsAnOldIssueInProgressOfAProjectAlreadySynced(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, 3, issueOf(615), issueOf(614), issueOf(623, "DEV DOING"))
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
	assert.Equal(t, teamstate.ClaimStatusInProgress, got["623"], "found by the first complete scan, and let in though the board is full")
	assert.NotContains(t, got, "615")
	assert.NotContains(t, got, "614")
	assert.Equal(t, 2, res.Projects[0].AutoPlanHeld)

	// The scan is made once: the next run only looks at what was updated.
	f.tr.asked = nil
	f.run(t)
	require.NotEmpty(t, f.tr.asked)
	assert.False(t, f.tr.asked[0].UpdatedAfter.IsZero(), "an incremental fetch from then on")
}
