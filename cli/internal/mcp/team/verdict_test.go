package team

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedClaim writes a claim in the state clone and pushes it.
func seedClaim(t *testing.T, stateDir, ticket, status string) {
	t.Helper()
	dir := filepath.Join(stateDir, "projects", "myproject", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := fmt.Sprintf("claimed_by = \"benjamin\"\nclaimed_at = 2026-10-22T08:00:00Z\nstatus = %q\n", status)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ticket+".toml"), []byte(content), 0o644))
	gittest.Run(t, stateDir, "add", ".")
	gittest.Run(t, stateDir, "commit", "-m", "seed "+ticket)
	gittest.Run(t, stateDir, "push")
}

func readClaim(t *testing.T, stateDir, ticket string) *teamstate.Claim {
	t.Helper()
	repo := teamstate.NewRepo(stateDir, stateDir)
	c, err := repo.GetClaim(t.Context(), "myproject", ticket)
	require.NoError(t, err)
	return c
}

func verdict(t *testing.T, ticket, v string) (string, bool) {
	t.Helper()
	res, err := handleTeamReviewVerdict(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"project":"myproject","ticket_id":%q,"verdict":%q}`, ticket, v)))
	require.NoError(t, err)
	require.Len(t, res.Content, 1)
	return res.Content[0].Text, res.IsError
}

// A vote moves the claim: approved → done (through review when the claim is
// still in progress) with the agent-reviewed label, rejected → in_progress.
func TestReviewVerdictStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	tests := []struct {
		name       string
		from       string
		verdict    string
		wantStatus string
		wantLabel  bool
	}{
		{"approved from in_progress goes through review to done", "in_progress", "approved", "done", true},
		{"approved from review", "review", "approved", "done", true},
		{"approved from validation", "validation", "approved", "done", true},
		{"rejected from review", "review", "rejected", "in_progress", false},
		{"rejected from in_progress keeps the claim", "in_progress", "rejected", "in_progress", false},
		{"approved twice keeps done", "done", "approved", "done", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := setupHandlerWriteTest(t)
			seedClaim(t, stateDir, "SRU-1", tt.from)

			text, isErr := verdict(t, "SRU-1", tt.verdict)
			require.False(t, isErr, text)
			assert.Contains(t, text, "Claim status → "+tt.wantStatus)

			c := readClaim(t, stateDir, "SRU-1")
			assert.Equal(t, tt.wantStatus, c.Status)
			assert.Equal(t, tt.wantLabel, slices.Contains(c.Labels, teamstate.LabelAgentReviewed), "labels %v", c.Labels)
		})
	}
}

// in_progress → done stays closed to direct transitions.
func TestValidTransitionsKeepInProgressToDoneClosed(t *testing.T) {
	assert.False(t, teamstate.IsValidTransition(teamstate.ClaimStatusInProgress, teamstate.ClaimStatusDone))
}

// A transition that cannot happen is reported, and the vote is neither
// labelled nor recorded.
func TestReviewVerdictTransitionFailureIsReported(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	seedClaim(t, stateDir, "SRU-2", "blocked")

	text, isErr := verdict(t, "SRU-2", "approved")
	require.True(t, isErr)
	assert.Contains(t, text, "not recorded")
	assert.Contains(t, text, "blocked")

	c := readClaim(t, stateDir, "SRU-2")
	assert.Equal(t, "blocked", c.Status)
	assert.Empty(t, c.Labels)
}

func TestReviewVerdictUnknownClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	setupHandlerWriteTest(t)
	text, isErr := verdict(t, "NOPE-1", "approved")
	require.True(t, isErr)
	assert.Contains(t, text, "Claim not found")
}

// The verdict is one transaction: status, label and event in one commit, and
// the actor of the event is the member of the session, not a fixed name.
func TestReviewVerdictIsOneCommitSignedByTheMember(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	seedClaim(t, stateDir, "SRU-3", "in_progress")
	before := gittest.Run(t, stateDir, "rev-list", "--count", "HEAD")

	text, isErr := verdict(t, "SRU-3", "approved")
	require.False(t, isErr, text)

	after := gittest.Run(t, stateDir, "rev-list", "--count", "HEAD")
	assert.Equal(t, mustInt(before)+1, mustInt(after), "one commit for the status (through review), the label and the event")
	assert.Equal(t, gittest.Run(t, stateDir, "rev-parse", "HEAD"), gittest.Run(t, stateDir, "rev-parse", "origin/main"), "pushed")
	assert.Contains(t, gittest.Run(t, stateDir, "log", "-1", "--format=%s"), "verdict: approved myproject/SRU-3")

	events, err := os.ReadDir(filepath.Join(stateDir, "projects", "myproject", "events"))
	require.NoError(t, err)
	require.Len(t, events, 1)
	data, err := os.ReadFile(filepath.Join(stateDir, "projects", "myproject", "events", events[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"actor":"benjamin"`)
	assert.NotContains(t, string(data), "orchestrator-dev")
}

// A failed transition leaves nothing: no label, no event, no commit.
func TestReviewVerdictFailureLeavesNoEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	seedClaim(t, stateDir, "SRU-4", "blocked")
	head := gittest.Run(t, stateDir, "rev-parse", "HEAD")

	_, isErr := verdict(t, "SRU-4", "approved")
	require.True(t, isErr)
	assert.Equal(t, head, gittest.Run(t, stateDir, "rev-parse", "HEAD"))
	_, err := os.Stat(filepath.Join(stateDir, "projects", "myproject", "events"))
	assert.True(t, os.IsNotExist(err), "no event")
}

// On a board of the team's own the verdict uses its columns: an approval ends
// in the terminal column, a rejection goes back to the first column of work.
func TestReviewVerdictUsesTheColumnsOfTheBoard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	board := `
[notification]
enabled = false

[[board.columns]]
id = "backlog"
name = "Backlog"
role = "initial"

[[board.columns]]
id = "dev"
name = "Dev"
role = "active"

[[board.columns]]
id = "shipped"
name = "Shipped"
role = "terminal"
`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte(board), 0o644))
	gittest.Run(t, stateDir, "add", ".")
	gittest.Run(t, stateDir, "commit", "-m", "board")
	gittest.Run(t, stateDir, "push")
	seedClaim(t, stateDir, "SRU-5", "dev")
	seedClaim(t, stateDir, "SRU-6", "shipped")

	text, isErr := verdict(t, "SRU-5", "approved")
	require.False(t, isErr, text)
	assert.Equal(t, "shipped", readClaim(t, stateDir, "SRU-5").Status)
	assert.Contains(t, text, "Claim status → shipped")

	text, isErr = verdict(t, "SRU-6", "rejected")
	require.False(t, isErr, text)
	assert.Equal(t, "dev", readClaim(t, stateDir, "SRU-6").Status)
}

func mustInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// The arguments of the verdict are checked before the repository is opened.
func TestReviewVerdictRefusesBadArguments(t *testing.T) {
	_, err := handleTeamReviewVerdict(t.Context(), json.RawMessage(`{"project":`))
	require.Error(t, err)

	res, err := handleTeamReviewVerdict(t.Context(), json.RawMessage(`{"project":"p","ticket_id":"t","verdict":"maybe"}`))
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, res.Content[0].Text, `Invalid verdict "maybe"`)
}

// The reason, the verdict of the AI reviewer and the cycle go in the event.
func TestReviewVerdictEventCarriesTheAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	seedClaim(t, stateDir, "SRU-7", "review")

	res, err := handleTeamReviewVerdict(t.Context(), json.RawMessage(
		`{"project":"myproject","ticket_id":"SRU-7","verdict":"rejected","reason":"tests missing","reviewer_verdict":"changes","cycle":2}`))
	require.NoError(t, err)
	require.False(t, res.IsError, res.Content[0].Text)
	assert.Equal(t, "in_progress", readClaim(t, stateDir, "SRU-7").Status)

	events, err := os.ReadDir(filepath.Join(stateDir, "projects", "myproject", "events"))
	require.NoError(t, err)
	require.Len(t, events, 1)
	data, err := os.ReadFile(filepath.Join(stateDir, "projects", "myproject", "events", events[0].Name()))
	require.NoError(t, err)
	for _, want := range []string{`"reason":"tests missing"`, `"reviewer_verdict":"changes"`, `"cycle":2`, `"author":"benjamin"`} {
		assert.Contains(t, string(data), want)
	}
}
