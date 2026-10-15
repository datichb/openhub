package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// The bricks of the TUI: origin, replacement, version and publication,
// not sealed, findings (the replacement itself is not a problem).
func TestBrickRows(t *testing.T) {
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	lock := &teamstate.BricksLock{}
	lock.Set("skill", "acme/review-protocol", teamstate.BrickLockEntry{Version: 3, PublishedBy: "Grace", PublishedAt: at})
	ws := &workflowsvc.BrickWorkspace{
		Bricks: []workflowsvc.BrickEntry{
			{Kind: workflowsvc.BrickAgent, ID: "developer", Origin: workflowsvc.OriginHub, Tokens: 10, Workflows: []string{"quick"}},
			{Kind: workflowsvc.BrickSkill, ID: "review/review-protocol", Origin: workflowsvc.OriginTeam, TeamID: "acme/review-protocol",
				Shadows: "hub:review/review-protocol", Version: 3, Source: "catalog/skills/acme/review-protocol.md"},
			{Kind: workflowsvc.BrickSkill, ID: "legacy", Origin: workflowsvc.OriginTeam, TeamID: "legacy"},
		},
		Unsealed: []string{"skill:legacy"},
		Problems: workflowsvc.BrickProblems{
			"skill:acme/review-protocol": wf.Diagnostics{{Code: workflowsvc.DiagBrickShadows, Message: "remplace"},
				{Severity: wf.SeverityWarning, Message: "description longue"}},
			"skill:legacy": wf.Diagnostics{{Severity: wf.SeverityError, Message: "frontmatter illisible"}},
		},
	}
	rows := brickRows(ws, lock)
	require.Len(t, rows, 3)
	assert.Equal(t, "hub", rows[0].Origin)
	assert.Empty(t, rows[0].Problems)
	assert.Equal(t, []string{"quick"}, rows[0].Workflows)
	r := rows[1]
	assert.Equal(t, "hub:review/review-protocol", r.Shadows)
	assert.Equal(t, 3, r.Version)
	assert.Equal(t, "Grace", r.PublishedBy)
	assert.Equal(t, at, r.PublishedAt)
	assert.False(t, r.Unsealed)
	require.Len(t, r.Problems, 1, "the replacement is the origin, not a problem")
	assert.False(t, r.Problems[0].Error)
	assert.True(t, rows[2].Unsealed)
	require.Len(t, rows[2].Problems, 1)
	assert.True(t, rows[2].Problems[0].Error)
	assert.Len(t, brickRows(&workflowsvc.BrickWorkspace{Bricks: ws.Bricks[:1]}, nil), 1, "without catalog.lock")
}

// The queue of the project: the open and stale proposals, with the
// workflow of their session.
func TestQueueWiki(t *testing.T) {
	list := []domain.WikiProposal{
		{Seq: 1, SessionID: "ses_a", Status: domain.WikiPending, Page: "index.md", Content: "x"},
		{Seq: 2, SessionID: "ses_a", Status: domain.WikiAccepted, Original: "y", Content: "z"},
		{Seq: 3, Status: domain.WikiRefused},
		{Seq: 4, Status: domain.WikiApplied},
		{Seq: 5, Status: domain.WikiStale, Replaces: "old", Error: "gone"},
	}
	out := queueWiki(context.Background(), nil, list)
	require.Len(t, out, 3)
	assert.Equal(t, []string{"wp-1", "wp-2", "wp-5"}, []string{out[0].ID, out[1].ID, out[2].ID})
	assert.True(t, out[1].Edited)
	assert.Equal(t, "gone", out[2].Error)
	assert.Equal(t, "old", out[2].Replaces)
}
