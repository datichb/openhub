package remote

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func ticketJSON(status string, labels ...string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"id": "bd-42", "title": "CSV", "status": status, "labels": labels})
	return data
}

func entries(argvs ...[]string) (journal []beadswire.JournalEntry, seqs []int) {
	for i, a := range argvs {
		journal = append(journal, beadswire.JournalEntry{Seq: i + 1, Argv: a})
		seqs = append(seqs, i+1)
	}
	return journal, seqs
}

var (
	add    = func(l ...string) []string { return append([]string{"label", "add", "bd-42"}, l...) }
	remove = func(l ...string) []string { return append([]string{"label", "remove", "bd-42"}, l...) }
)

// Labels are sets: adding and removing them commutes with what was done to
// the ticket on the machine meanwhile. A ticket whose version changed on the
// machine is a conflict only when the replay would undo a local decision
// (re-add a label removed locally, remove one added locally) or when the
// session wrote something else than labels.
func TestReplayLabelsConflict(t *testing.T) {
	const rfr = "ready-for-review"
	for _, tc := range []struct {
		name     string
		snap     []string // labels when the session was sent
		local    []string // labels on the machine now
		localSt  string   // status on the machine now
		journal  [][]string
		conflict bool
	}{
		{"label added, local status moved", nil, nil, "in_progress", [][]string{add(rfr)}, false},
		{"label added, local label added", nil, []string{"ai"}, "open", [][]string{add(rfr)}, false},
		{"label added, local added the same", nil, []string{rfr}, "open", [][]string{add(rfr)}, false},
		{"label added, local removed another", []string{"wip"}, nil, "open", [][]string{add(rfr)}, false},
		{"label removed, local removed it too", []string{"wip"}, nil, "open", [][]string{remove("wip")}, false},
		{"label removed, local kept it", []string{"wip"}, []string{"wip", "ai"}, "open", [][]string{remove("wip")}, false},
		{"added then removed, nothing local", nil, nil, "in_progress", [][]string{add("tmp"), remove("tmp")}, false},
		{"update --add-label --remove-label with lists", []string{"c"}, []string{"c", "z"}, "open",
			[][]string{{"update", "bd-42", "--add-label", "a,b", "--remove-label=c"}}, false},
		{"re-added a label removed locally", []string{rfr}, nil, "open", [][]string{add(rfr)}, true},
		{"removed a label added locally", nil, []string{"hot"}, "open", [][]string{add("hot"), remove("hot")}, true},
		{"removed a label added locally (never in the snapshot)", nil, []string{"hot"}, "open", [][]string{remove("hot")}, true},
		{"label and status in one entry", nil, nil, "in_progress", [][]string{{"update", "bd-42", "--status", "in_progress", "--add-label", rfr}}, true},
		{"label and note", nil, nil, "in_progress", [][]string{add(rfr), {"note", "bd-42", "done"}}, true},
		{"label and notes in one entry", nil, nil, "in_progress", [][]string{{"update", "bd-42", "--add-label", rfr, "--notes", "done"}}, true},
		{"only a note", nil, nil, "in_progress", [][]string{{"note", "bd-42", "done"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			journal, seqs := entries(tc.journal...)
			snapRaw, curRaw := ticketJSON("in_progress", tc.snap...), ticketJSON(tc.localSt, tc.local...)
			assert.Equal(t, !tc.conflict, labelsCommute(snapRaw, curRaw, journal, seqs))
		})
	}
}

// The conflict shows the labels the session changes, next to the other fields.
func TestConflictOfShowsLabels(t *testing.T) {
	journal, seqs := entries(
		[]string{"update", "bd-42", "--status", "review", "--add-label", "ready-for-review,ai"},
		remove("wip"),
		add("zz"), remove("zz"),
	)
	snapRaw := ticketJSON("in_progress", "wip", "b")
	curRaw := ticketJSON("in_progress", "b", "local")
	c := conflictOf("bd-42", snapRaw, curRaw, journal, seqs)
	assert.Equal(t, []FieldChange{
		{Field: "status", Snapshot: "in_progress", Local: "in_progress", Remote: "review"},
		{Field: "labels", Snapshot: "b,wip", Local: "b,local", Remote: "ai,b,ready-for-review"},
	}, c.Fields)

	// Labels unchanged by the session: not listed.
	journal, seqs = entries(add("b"), []string{"note", "bd-42", "x"})
	c = conflictOf("bd-42", snapRaw, curRaw, journal, seqs)
	assert.Empty(t, c.Fields)
	assert.Equal(t, []string{"x"}, c.Notes)
}

func newLabelsEnv(t *testing.T, journal []byte) *returnEnv {
	t.Helper()
	e := newReturnEnvAllow(t, []string{"show", "update", "create", "dep add", "note", "label"}, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted}, journal)
	e.pipeline.Status = "success"
	return e
}

// Through the whole replay: a label the session adds is not a conflict when
// the ticket moved on the machine, and it is applied as it was recorded.
func TestReplayLabelOnlyIsNotAConflict(t *testing.T) {
	ctx := context.Background()
	e := newLabelsEnv(t, journalLines(
		beadswire.JournalEntry{Seq: 1, Argv: []string{"label", "add", "bd-42", "ready-for-review"}},
		beadswire.JournalEntry{Seq: 2, Argv: []string{"update", "bd-42", "--remove-label", "wip"}},
	))
	// The snapshot (revision r1, labels wip) was taken at Send.
	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	// Meanwhile on the machine: alice moved the ticket and labelled it.
	e.beads.issues["bd-42"]["status"] = "in_progress"
	e.beads.issues["bd-42"]["labels"] = []string{"wip", "alice"}
	e.beads.issues["bd-42"]["revision"] = "r3-local"

	plan, err := e.svc.PlanReplay(ctx, e.sessionID)
	require.NoError(t, err)
	assert.Empty(t, plan.Conflicts, "labels commute with the local changes")
	assert.Equal(t, 2, plan.Pending())

	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	require.NoError(t, err)
	require.Len(t, e.beads.execs, 2)
	assert.Contains(t, e.beads.execs[0], "label add")
	assert.Contains(t, e.beads.execs[0], "ready-for-review")
	ref, _ := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteResolved, ref.Status)
}

// A replay that would undo a local decision is still a conflict, listed with
// the labels of both sides.
func TestReplayLabelUndoingLocalDecisionIsAConflict(t *testing.T) {
	ctx := context.Background()
	e := newLabelsEnv(t, journalLines(
		beadswire.JournalEntry{Seq: 1, Argv: []string{"label", "add", "bd-42", "wip"}}, // already there when sent
	))
	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	e.beads.issues["bd-42"]["labels"] = []string{} // alice removed it
	e.beads.issues["bd-42"]["revision"] = "r3-local"

	plan, err := e.svc.PlanReplay(ctx, e.sessionID)
	require.NoError(t, err)
	require.Len(t, plan.Conflicts, 1)
	assert.Equal(t, []int{1}, plan.Conflicts[0].Entries)
	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	assert.ErrorIs(t, err, ErrUnresolved)
	assert.Empty(t, e.beads.execs)
}
