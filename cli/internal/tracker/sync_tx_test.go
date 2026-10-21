package tracker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// stubTracker answers FetchIssue only.
type stubTracker struct {
	issues map[int]IssueState
}

func (s *stubTracker) FetchIssue(_ context.Context, _ string, iid int) (*IssueState, error) {
	if is, ok := s.issues[iid]; ok {
		return &is, nil
	}
	return nil, ErrIssueNotFound
}
func (s *stubTracker) ListAssignedIssues(context.Context, string, ListOpts) ([]IssueState, error) {
	return nil, nil
}
func (s *stubTracker) ListUnassignedIssues(context.Context, string, ListUnassignedOpts) ([]IssueState, error) {
	return nil, nil
}
func (s *stubTracker) ListIssuesByLabels(context.Context, string, ListByLabelsOpts) ([]IssueState, error) {
	return nil, nil
}
func (s *stubTracker) AssignIssue(context.Context, string, int, string) error    { return nil }
func (s *stubTracker) AddLabels(context.Context, string, int, []string) error    { return nil }
func (s *stubTracker) RemoveLabels(context.Context, string, int, []string) error { return nil }
func (s *stubTracker) CreateIssue(context.Context, CreateIssueOpts) (*CreatedIssue, error) {
	return nil, nil
}
func (s *stubTracker) TestConnection(context.Context) (string, error)      { return "me", nil }
func (s *stubTracker) TestProject(context.Context, string) (string, error) { return "p", nil }
func (s *stubTracker) DiscoverProject(context.Context, string) (*DiscoveryInfo, error) {
	return &DiscoveryInfo{}, nil
}

// N15 step B: a tracker synchronization is one transaction: every claim
// change of the run in one commit, pushed once.
func TestSyncIsOneTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(ctx, teamstate.Member{ID: "alice", DisplayName: "Alice"}))
	for _, id := range []string{"1", "2"} {
		_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: id, Project: "p", ClaimedBy: "alice", ExternalIID: mustAtoi(id)})
		require.NoError(t, err)
	}
	// Someone else's file in the clone: never committed by the sync.
	require.NoError(t, os.WriteFile(filepath.Join(clone, "scratch.txt"), []byte("x"), 0o644))
	before := mustAtoi(gittest.Run(t, clone, "rev-list", "--count", "HEAD"))

	tr := &stubTracker{issues: map[int]IssueState{
		1: {IID: 1, State: "open", StatusName: "opened", Title: "first", Labels: []string{"bug"}},
		2: {IID: 2, State: "open", StatusName: "opened", Title: "second"},
	}}
	e := NewEngine(tr, repo, teamstate.TrackerConfig{Projects: map[string]string{"p": "grp/p"}}, t.TempDir()) //nolint:staticcheck // the engine still reads Projects
	res, err := e.Run(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	after := mustAtoi(gittest.Run(t, clone, "rev-list", "--count", "HEAD"))
	assert.Equal(t, before+1, after, "one commit for the run")
	assert.Equal(t, gittest.Run(t, clone, "rev-parse", "HEAD"), gittest.Run(t, bare, "rev-parse", "HEAD"), "pushed")
	assert.NotContains(t, gittest.Run(t, clone, "show", "--name-only", "--format=", "HEAD"), "scratch.txt")
	c, err := repo.GetClaim("p", "1")
	require.NoError(t, err)
	assert.Equal(t, "first", c.Title)
	assert.Contains(t, c.Labels, "bug")
}

func mustAtoi(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }
