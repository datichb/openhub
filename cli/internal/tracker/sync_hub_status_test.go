package tracker

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// fakeIssues is a GitLab that serves issues by IID, with a state and labels,
// and applies add_labels / remove_labels of a PUT, recording each write.
type fakeIssues struct {
	mu     sync.Mutex
	issues map[int]*fakeIssue
	writes []string // "add 1 A,B" / "remove 1 C"
}

type fakeIssue struct {
	state  string // "opened" | "closed"
	labels []string
}

var issuePath = regexp.MustCompile(`/issues/(\d+)$`)

func (f *fakeIssues) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	m := issuePath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	iid, _ := strconv.Atoi(m[1])
	is, ok := f.issues[iid]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPut {
		var body map[string]string
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if v := body["add_labels"]; v != "" {
			f.writes = append(f.writes, "add "+m[1]+" "+v)
			for _, l := range strings.Split(v, ",") {
				if !slices.Contains(is.labels, l) {
					is.labels = append(is.labels, l)
				}
			}
		}
		if v := body["remove_labels"]; v != "" {
			f.writes = append(f.writes, "remove "+m[1]+" "+v)
			for _, l := range strings.Split(v, ",") {
				is.labels = slices.DeleteFunc(is.labels, func(x string) bool { return x == l })
			}
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"iid": iid, "state": is.state, "title": "t" + m[1],
		"labels": is.labels, "updated_at": "2026-01-01T00:00:00Z", "assignees": []any{}})
}

func (f *fakeIssues) set(iid int, state string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issues[iid] = &fakeIssue{state: state, labels: labels}
}

func (f *fakeIssues) labelsOf(iid int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.issues[iid].labels)
}

func (f *fakeIssues) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.writes)
}

type hubSyncFixture struct {
	repo     *teamstate.Repo
	bare     string
	gl       *fakeIssues
	cfg      teamstate.TrackerConfig
	stateDir string
	push     bool
}

// newHubSyncFixture is a team on the default board whose tracker has labels
// attached to the columns (in_progress, review, blocked) and one issue (1)
// open without label.
func newHubSyncFixture(t *testing.T) *hubSyncFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	i18ntest.Set(t, "en")
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "alice", DisplayName: "Alice"}))
	gl := &fakeIssues{issues: map[int]*fakeIssue{}}
	gl.set(1, "opened")
	srv := httptest.NewServer(gl)
	t.Cleanup(srv.Close)
	cfg := teamstate.TrackerConfig{
		TrackerURL: srv.URL,
		Projects:   map[string]string{"p": "grp/p"}, //nolint:staticcheck // the engine still reads Projects
		LabelStatusMapping: map[string]string{
			"DEV DOING": "in_progress", "TO REVIEW": "review", "Blocked": "blocked",
		},
	}
	return &hubSyncFixture{repo: repo, bare: bare, gl: gl, cfg: cfg, stateDir: t.TempDir()}
}

func (f *hubSyncFixture) claim(t *testing.T, c teamstate.Claim) {
	t.Helper()
	c.Project, c.ClaimedBy, c.ExternalIID = "p", "alice", mustAtoi(c.TicketID)
	_, err := f.repo.CreateClaim(t.Context(), c)
	require.NoError(t, err)
}

func (f *hubSyncFixture) run(t *testing.T) *SyncResult {
	t.Helper()
	cfg := f.cfg
	cfg.PushLabels = f.push
	tr := newGitLab(Config{Type: TypeGitLab, BaseURL: cfg.TrackerURL, Token: "tok", WriteEnabled: true})
	res, err := NewEngine(tr, f.repo, cfg, f.stateDir).Run(t.Context())
	require.NoError(t, err)
	return res
}

func (f *hubSyncFixture) get(t *testing.T) teamstate.Claim {
	t.Helper()
	c, err := f.repo.GetClaim(t.Context(), "p", "1")
	require.NoError(t, err)
	return *c
}

// The status put on the board, by an MR, a verdict or the end of a session
// stays when the issue stays open without a label of status: the sync of any
// member, however many times, does not bring it back to in_progress.
func TestSyncKeepsTheStatusOfTheHub(t *testing.T) {
	for _, tc := range []struct {
		name string
		seen bool // the claim already remembers the issue
	}{{"a claim no sync has seen", false}, {"a claim the sync remembers", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHubSyncFixture(t)
			c := teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusReview}
			if tc.seen {
				c.TrackerState, c.TrackerStatus = "open", "opened"
			}
			f.claim(t, c)
			f.run(t)
			f.run(t)
			assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status, "still in review after two syncs")
			assert.Equal(t, "open", f.get(t).TrackerState, "the issue is remembered")

			require.NoError(t, f.repo.UpdateClaimStatus(t.Context(), "p", "1", teamstate.ClaimStatusValidation))
			f.run(t)
			assert.Equal(t, teamstate.ClaimStatusValidation, f.get(t).Status)
		})
	}
}

// A label attached to a column that appears on the issue is the tracker
// moving the card: the status follows; blocked first, then the most advanced.
func TestSyncFollowsALabelAttachedToAColumn(t *testing.T) {
	f := newHubSyncFixture(t)
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusInProgress, TrackerState: "open", TrackerStatus: "opened"})
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusInProgress, f.get(t).Status)

	f.gl.set(1, "opened", "TO REVIEW", "bug")
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status, "the label moved the card")
	assert.ElementsMatch(t, []string{"TO REVIEW", "bug"}, f.get(t).Labels)

	// The hub moves it on; a label without a column changes nothing.
	require.NoError(t, f.repo.UpdateClaimStatus(t.Context(), "p", "1", teamstate.ClaimStatusValidation))
	f.gl.set(1, "opened", "TO REVIEW", "bug", "other")
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusValidation, f.get(t).Status)

	f.gl.set(1, "opened", "TO REVIEW", "bug", "other", "Blocked", "DEV DOING")
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusBlocked, f.get(t).Status, "blocked first")
}

// A closed issue ends the claim (an event, even for a claim never seen); its
// reopening brings it back to work.
func TestSyncClosedIssueIsTerminalAndReopeningIsWork(t *testing.T) {
	f := newHubSyncFixture(t)
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusReview})
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status)

	f.gl.set(1, "closed")
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusDone, f.get(t).Status)
	assert.Equal(t, "closed", f.get(t).TrackerState)
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusDone, f.get(t).Status)

	f.gl.set(1, "opened")
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusInProgress, f.get(t).Status, "reopened: back to work")
}

// A claim never seen, done in the hub while the issue is open (and was
// recorded closed by a previous version), is reopened by the sync; one that
// was never closed stays done.
func TestSyncUnseenClaimDoneStaysDoneUnlessTheIssueWasClosed(t *testing.T) {
	f := newHubSyncFixture(t)
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusDone, TrackerStatus: "opened"})
	f.run(t)
	assert.Equal(t, teamstate.ClaimStatusDone, f.get(t).Status)

	f.gl.set(2, "opened")
	f.claim(t, teamstate.Claim{TicketID: "2", Status: teamstate.ClaimStatusDone, TrackerStatus: "closed"})
	f.run(t)
	c, err := f.repo.GetClaim(t.Context(), "p", "2")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status, "the issue was closed, it is open: reopened")
}

// With push_labels, the status the hub gave to the card goes to the tracker as
// the label of its column, the label of the previous column taken off.
func TestSyncPushesTheStatusAsALabel(t *testing.T) {
	f := newHubSyncFixture(t)
	f.push = true
	f.gl.set(1, "opened", "DEV DOING", "bug")
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusReview, TrackerState: "open", TrackerStatus: "opened",
		TrackerLabels: []string{"DEV DOING", "bug"}, Labels: []string{"DEV DOING", "bug"}})
	res := f.run(t)

	assert.ElementsMatch(t, []string{"TO REVIEW", "bug"}, f.gl.labelsOf(1))
	assert.Equal(t, []string{"add 1 TO REVIEW", "remove 1 DEV DOING"}, f.gl.written())
	assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status, "the hub keeps its status")
	assert.Equal(t, 1, res.LabelsPushed)
	assert.ElementsMatch(t, []string{"TO REVIEW", "bug"}, f.get(t).TrackerLabels)

	// Nothing left to push; the label that came back does not move the card.
	f.run(t)
	assert.Len(t, f.gl.written(), 2)
	assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status)
	assert.NotContains(t, f.get(t).Labels, "DEV DOING", "the label taken off the issue is taken off the claim")
}

// Without push_labels nothing is written to the tracker.
func TestSyncDoesNotPushWithoutPushLabels(t *testing.T) {
	f := newHubSyncFixture(t)
	f.gl.set(1, "opened", "DEV DOING")
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusReview, TrackerState: "open", TrackerStatus: "opened",
		TrackerLabels: []string{"DEV DOING"}})
	f.run(t)
	f.run(t)
	assert.Empty(t, f.gl.written())
	assert.Equal(t, teamstate.ClaimStatusReview, f.get(t).Status)
}

// A hub status with no label attached to its column cannot be pushed: nothing
// is written.
func TestSyncPushWithoutALabelForTheColumn(t *testing.T) {
	f := newHubSyncFixture(t)
	f.push = true
	f.gl.set(1, "opened", "DEV DOING")
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusValidation, TrackerState: "open", TrackerStatus: "opened",
		TrackerLabels: []string{"DEV DOING"}})
	f.run(t)
	assert.Empty(t, f.gl.written())
	assert.Equal(t, teamstate.ClaimStatusValidation, f.get(t).Status)
}

// The labels the hub puts on a claim (session-ended, review, ready-for-review)
// are not the tracker's: the sync does not take them off. A label the sync saw
// on the issue and that left it is taken off.
func TestSyncKeepsTheLabelsOfTheHub(t *testing.T) {
	f := newHubSyncFixture(t)
	f.gl.set(1, "opened", "bug", "wontfix")
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusInProgress,
		Labels: []string{"bug", teamstate.LabelSessionEnded, teamstate.LabelReview, teamstate.LabelReadyForReview}})
	f.run(t)
	f.run(t)
	got := f.get(t).Labels
	for _, l := range []string{teamstate.LabelSessionEnded, teamstate.LabelReview, teamstate.LabelReadyForReview, "bug", "wontfix"} {
		assert.Contains(t, got, l)
	}
	assert.ElementsMatch(t, []string{"bug", "wontfix"}, f.get(t).TrackerLabels)

	f.gl.set(1, "opened", "bug")
	f.run(t)
	got = f.get(t).Labels
	assert.NotContains(t, got, "wontfix", "taken off the issue, taken off the claim")
	for _, l := range []string{teamstate.LabelSessionEnded, teamstate.LabelReview, teamstate.LabelReadyForReview, "bug"} {
		assert.Contains(t, got, l)
	}
}
