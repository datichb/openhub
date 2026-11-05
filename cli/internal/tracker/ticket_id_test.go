package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestTicketIDFollowsThePattern(t *testing.T) {
	tests := []struct {
		name    string
		issue   IssueState
		pattern string
		want    string
	}{
		{"no pattern", IssueState{IID: 42}, "", "42"},
		{"prefix", IssueState{IID: 42}, `^SRU-(\d+)$`, "SRU-42"},
		{"prefix without anchors", IssueState{IID: 7}, `SRU-(\d+)`, "SRU-7"},
		{"prefix and suffix", IssueState{IID: 7}, `^T(\d+)x$`, "T7x"},
		{"digits class", IssueState{IID: 7}, `^SRU-([0-9]+)$`, "SRU-7"},
		{"bare capture", IssueState{IID: 7}, `^(\d+)$`, "7"},
		{"wildcard cannot be written back", IssueState{IID: 7}, `.*-(\d+)`, "7"},
		{"alternative cannot be written back", IssueState{IID: 7}, `^(?:A|B)-(\d+)$`, "7"},
		{"invalid pattern", IssueState{IID: 7}, `(`, "7"},
		{"the key of the tracker wins", IssueState{IID: 42, Key: "SRU-42"}, `^PRJ-(\d+)$`, "SRU-42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ticketIDFromIssue(tt.issue, tt.pattern)
			assert.Equal(t, tt.want, got)
			if tt.issue.Key == "" && tt.pattern != "" {
				if iid, ok := resolveIID(got, 0, tt.pattern); ok {
					assert.Equal(t, tt.issue.IID, iid, "the ID reads back as the IID")
				}
			}
		})
	}
}

// Two hub projects linked to the same tracker project do not both take the
// issues of the tracker, and an issue that has a claim "42" does not get a
// second one "SRU-42".
func TestSyncCreatesOneClaimPerIssueOfTheTracker(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, issueOf(42), issueOf(43))
	f.cfg.Projects = map[string]string{"b": "grp/p", "a": "grp/p"}                   //nolint:staticcheck // the engine still reads Projects
	f.cfg.TicketPatterns = map[string]string{"a": `^SRU-(\d+)$`, "b": `^SRU-(\d+)$`} //nolint:staticcheck // the engine still reads TicketPatterns
	ctx := t.Context()
	// The claim of #42 exists already, under its bare number, in b.
	_, err := f.repo.CreateClaim(ctx, teamstate.Claim{TicketID: "42", Project: "b", ClaimedBy: "alice", ExternalIID: 42})
	require.NoError(t, err)

	res := f.run(t)
	assert.Equal(t, 1, res.ClaimsCreated, "only #43 is new")

	inA, err := f.repo.ListClaims(ctx, "a")
	require.NoError(t, err)
	require.Len(t, inA, 1)
	assert.Equal(t, "SRU-43", inA[0].TicketID, "the ID is built with the pattern; the first project (a) takes it")
	inB, err := f.repo.ListClaims(ctx, "b")
	require.NoError(t, err)
	require.Len(t, inB, 1)
	assert.Equal(t, "42", inB[0].TicketID)

	res = f.run(t)
	assert.Zero(t, res.ClaimsCreated, "nothing is created twice")
	inB, err = f.repo.ListClaims(ctx, "b")
	require.NoError(t, err)
	assert.Len(t, inB, 1)
}

// The claim of a ticket that an earlier run named "SRU-43" is found again by
// its number.
func TestSyncKnowsTheIssueOfAClaimByItsPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newAutoplanFixture(t, issueOf(43))
	f.cfg.TicketPatterns = map[string]string{"p": `^SRU-(\d+)$`} //nolint:staticcheck // the engine still reads TicketPatterns
	_, err := f.repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "SRU-43", Project: "p", ClaimedBy: "alice"})
	require.NoError(t, err)

	res := f.run(t)
	assert.Zero(t, res.ClaimsCreated)
	assert.Len(t, f.claims(t), 1)
	assert.Contains(t, f.claims(t), "SRU-43")
}
