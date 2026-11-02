package remote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Merging notes keeps the notes and comments of the session, and also the
// ready-for-review label (in v5 a ticket is in review when it is in progress
// with that label): a conflict on the status must not hide that the work
// is ready. Nothing else of the entry is applied.
func TestNotesOnlyKeepsReadyForReview(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
		want []string // canonical argv; nil = nothing applied
	}{
		{"notes and label of an update", []string{"update", "bd-42", "--status", "review", "--add-label", "ready-for-review", "--notes", "done"},
			[]string{"update", "--add-label=ready-for-review", "--notes=done", "--", "bd-42"}},
		{"status and notes", []string{"update", "bd-42", "--status", "in_progress", "--notes", "done"},
			[]string{"update", "--notes=done", "--", "bd-42"}},
		{"label alone", []string{"update", "bd-42", "--add-label", "ready-for-review"},
			[]string{"update", "--add-label=ready-for-review", "--", "bd-42"}},
		{"label among others", []string{"update", "bd-42", "--add-label", "ai,ready-for-review,wip", "--assignee", "bob"},
			[]string{"update", "--add-label=ready-for-review", "--", "bd-42"}},
		{"label kept in the list of a repeated option", []string{"update", "bd-42", "--add-label", "ai", "--add-label", "Ready-For-Review"},
			[]string{"update", "--add-label=Ready-For-Review", "--", "bd-42"}},
		{"label removal, with another one added", []string{"update", "bd-42", "--remove-label", "ready-for-review", "--add-label", "wip"},
			[]string{"update", "--remove-label=ready-for-review", "--", "bd-42"}},
		{"other labels only", []string{"update", "bd-42", "--add-label", "ai", "--remove-label", "wip"}, nil},
		{"status only", []string{"update", "bd-42", "--status", "review"}, nil},
		{"label add", []string{"label", "add", "bd-42", "ready-for-review"}, []string{"label", "add", "--", "bd-42", "ready-for-review"}},
		{"label add on two tickets", []string{"label", "add", "bd-42", "bd-43", "ready-for-review"}, []string{"label", "add", "--", "bd-42", "bd-43", "ready-for-review"}},
		{"label remove", []string{"label", "remove", "bd-42", "ready-for-review"}, []string{"label", "remove", "--", "bd-42", "ready-for-review"}},
		{"another label", []string{"label", "add", "bd-42", "ai"}, nil},
		{"note", []string{"note", "bd-42", "checked"}, []string{"note", "--", "bd-42", "checked"}},
		{"comment", []string{"comments", "add", "bd-42", "checked"}, []string{"comments", "add", "--", "bd-42", "checked"}},
		{"dependency", []string{"dep", "add", "bd-42", "bd-43"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, why := parseEntry(tc.argv)
			require.Empty(t, why)
			got := notesOnly(c)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tc.want, got.Argv())
		})
	}
}

// Through the replay: a conflict resolved by merging notes applies the notes
// and the ready-for-review label of the session, and keeps the rest local.
func TestReplayMergeNotesKeepsReadyForReview(t *testing.T) {
	ctx := context.Background()
	e := newLabelsEnv(t, journalLines(
		beadswire.JournalEntry{Seq: 1, Argv: []string{"update", "bd-42", "--status", "review", "--add-label", "ready-for-review", "--notes", "done"}},
		beadswire.JournalEntry{Seq: 2, Argv: []string{"label", "add", "bd-42", "ai"}},
	))
	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	e.beads.issues["bd-42"]["status"] = "in_progress"
	e.beads.issues["bd-42"]["revision"] = "r3-local"

	plan, err := e.svc.PlanReplay(ctx, e.sessionID)
	require.NoError(t, err)
	require.Len(t, plan.Conflicts, 1, "the status moved on both sides")
	assert.Contains(t, plan.Conflicts[0].Fields, FieldChange{Field: "labels", Snapshot: "wip", Local: "wip", Remote: "ai,ready-for-review,wip"})

	plan, err = e.svc.ApplyReplay(ctx, e.sessionID, map[string]Resolution{"bd-42": MergeNotes})
	require.NoError(t, err)
	require.Len(t, e.beads.execs, 1)
	assert.Equal(t, "update --add-label=ready-for-review --notes=done --actor=Ada via oh/session (remote) -- bd-42", e.beads.execs[0])
	assert.Equal(t, ItemApplied, plan.Items[0].State)
	assert.Equal(t, ItemSkipped, plan.Items[1].State, "another label: kept local")
	ref, _ := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteResolved, ref.Status)
}
