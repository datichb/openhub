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
	c, err := repo.GetClaim("myproject", ticket)
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
