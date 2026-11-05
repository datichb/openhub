package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func auditSession(outputs map[string]any) domain.Session {
	start := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Minute)
	return domain.Session{ID: "ses_audit", ProjectID: "api", WorkflowID: "audit", Status: domain.SessionStatusCompleted,
		StartedAt: start, EndedAt: &end, Outputs: outputs}
}

func eventTypes(events []teamstate.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

// The end of an audit session that deposited its report writes an
// audit.finding event next to session.complete; any other session, or an
// audit without a report, writes session.complete only.
func TestSessionTeamEventsOfAnAudit(t *testing.T) {
	events := sessionTeamEvents("alice", auditSession(map[string]any{"findings": "rp-2", "type": "security"}))
	require.Equal(t, []string{teamstate.EventSessionComplete, teamstate.EventAuditFinding}, eventTypes(events))
	f := events[1]
	assert.Equal(t, "alice", f.Actor)
	assert.Equal(t, "api", f.Project)
	assert.Equal(t, map[string]any{"session_id": "ses_audit", "report": "rp-2", "audit_type": "security"}, f.Data)

	assert.Equal(t, []string{teamstate.EventSessionComplete}, eventTypes(sessionTeamEvents("alice", auditSession(nil))), "no report")
	other := auditSession(map[string]any{"findings": "rp-2"})
	other.WorkflowID = "ticket"
	assert.Equal(t, []string{teamstate.EventSessionComplete}, eventTypes(sessionTeamEvents("alice", other)), "not an audit")
}

// The whole chain, with a fake webhook only: the end of an audit session →
// the events of the session → the team-state (committed on a local bare
// repository) → the event sink → the text sent to the webhook of the team.
func TestAuditFindingReachesTheWebhook(t *testing.T) {
	repo, rec := sinkTeam(t, "events = [\"audit.finding\"]\n")
	ctx := t.Context()
	for _, e := range sessionTeamEvents("alice", auditSession(map[string]any{"findings": "rp-2", "type": "security"})) {
		require.NoError(t, repo.AppendEvent(ctx, e))
	}
	require.True(t, teamstate.WaitEventSinks(10*time.Second))
	got := rec.list()
	require.Len(t, got, 1, "the destination asked for audit.finding only, not for session.complete")
	assert.Contains(t, got[0], "constat d'audit")
	assert.Contains(t, got[0], "Alice M.")
	assert.Contains(t, got[0], "api")
}

// A destination without an events list does not receive the findings: the
// default set is the review and the wiki, the sessions are asked for by name.
func TestAuditFindingIsNotInTheDefaultEvents(t *testing.T) {
	repo, rec := sinkTeam(t, "")
	for _, e := range sessionTeamEvents("alice", auditSession(map[string]any{"findings": "rp-2"})) {
		require.NoError(t, repo.AppendEvent(t.Context(), e))
	}
	require.True(t, teamstate.WaitEventSinks(10*time.Second))
	assert.Empty(t, rec.list())
}
