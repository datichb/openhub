package remote

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
)

// PoC S-04 (audit v5): an id with a dash in its prefix, ids in --parent and
// --deps, a closing status, combined short options. Only the tickets of the
// session, the options a replay knows, and what beads.allow grants pass.
func TestReplayClassifyPoC(t *testing.T) {
	snap := &remote.Snapshot{Issues: map[string]json.RawMessage{"bd-42": nil, "my-app-x9": nil}}
	ph := map[string]bool{"pending-1": true}
	allow := []string{"update", "create", "note", "comments", "dep add", "label"}
	for _, tc := range []struct {
		argv   []string
		refuse string // "" = pending
	}{
		{[]string{"close", "my-app-zz9"}, "not allowed"},
		{[]string{"update", "my-app-zz9", "--status", "in_progress"}, "my-app-zz9 is not a ticket of the session"},
		{[]string{"update", "my-app-x9", "--notes", "ok"}, ""},
		{[]string{"update", "bd-42", "--status", "closed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "--status=Closed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "-sclosed"}, "combined short options"},
		{[]string{"update", "bd-42", "--DB=/tmp/x"}, "refused"},
		{[]string{"update", "bd-42", "--parent", "my-app-zz9"}, "my-app-zz9 is not a ticket of the session"},
		{[]string{"update", "bd-42", "--parent=pending-1"}, ""},
		{[]string{"create", "Follow-up", "--parent", "other-epic"}, "other-epic is not a ticket of the session"},
		{[]string{"create", "Follow-up", "--deps", "blocks:bd-42,discovered-from:my-app-zz9"}, "my-app-zz9 is not a ticket"},
		{[]string{"create", "Follow-up", "--deps", "blocks:bd-42", "--json"}, ""},
		{[]string{"create", "Follow-up", "extra-ticket"}, "unexpected arguments"},
		{[]string{"create", "Follow-up", "--id", "bd-1"}, "--id refused"},
		{[]string{"create", "Follow-up", "--wat", "x"}, "unknown option --wat"},
		{[]string{"note", "bd-42", "follow-up", "with-dashes"}, ""},
		{[]string{"note", "my-app-zz9", "text"}, "not a ticket of the session"},
		{[]string{"comments", "add", "bd-42", "see-also my-app-zz9"}, ""},
		{[]string{"dep", "add", "pending-1", "my-app-zz9"}, "not a ticket of the session"},
		{[]string{"label", "add", "bd-42", "needs-review"}, ""},
		{[]string{"update", "--", "my-app-zz9"}, "not a ticket of the session"},
	} {
		it := classify(beadswire.JournalEntry{Seq: 1, Argv: tc.argv}, allow, snap, ph)
		if tc.refuse == "" {
			assert.Equal(t, ItemPending, it.State, "%v: %s", tc.argv, it.Reason)
			continue
		}
		assert.Equal(t, ItemRefused, it.State, "%v", tc.argv)
		assert.Contains(t, it.Reason, tc.refuse, "%v", tc.argv)
	}
	// close granted: a closing status is a close.
	it := classify(beadswire.JournalEntry{Argv: []string{"update", "bd-42", "--status", "closed"}}, append(allow, "close"), snap, ph)
	assert.Equal(t, ItemPending, it.State, it.Reason)
}

// The journal and the envelope kept on the machine are sealed with the
// session's journal key: a modified journal, a widened allow-list, an added
// ticket or a missing key refuse the whole replay.
func TestReplayRefusesUnauthenticatedJournal(t *testing.T) {
	ctx := context.Background()
	journal := journalLines(beadswire.JournalEntry{Seq: 1, Argv: []string{"note", "bd-42", "checked"}})
	newFetched := func(t *testing.T) *returnEnv {
		e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted}, journal)
		e.pipeline.Status = "success"
		_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
		require.NoError(t, err)
		_, err = e.svc.PlanReplay(ctx, e.sessionID)
		require.NoError(t, err, "sealed journal accepted")
		return e
	}
	edit := func(t *testing.T, e *returnEnv, name, from, to string) {
		p := filepath.Join(e.svc.RemoteDir(e.sessionID), name)
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		require.Contains(t, string(data), from)
		require.NoError(t, os.WriteFile(p, []byte(strings.Replace(string(data), from, to, 1)), 0o600))
	}

	e := newFetched(t)
	edit(t, e, remote.JournalFile, `"note","bd-42","checked"`, `"close","bd-42"`)
	_, err := e.svc.PlanReplay(ctx, e.sessionID)
	assert.ErrorIs(t, err, ErrJournalUnauthenticated, "modified journal")
	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	assert.ErrorIs(t, err, ErrJournalUnauthenticated)
	assert.Empty(t, e.beads.execs, "nothing replayed")

	e = newFetched(t)
	edit(t, e, remote.ManifestFile, `"note"`, `"note",  "close", "delete"`)
	_, err = e.svc.PlanReplay(ctx, e.sessionID)
	assert.ErrorIs(t, err, ErrJournalUnauthenticated, "widened allow-list")

	e = newFetched(t)
	edit(t, e, remote.SnapshotFile, `"bd-42": {`, `"my-app-zz9": {}, "bd-42": {`)
	_, err = e.svc.PlanReplay(ctx, e.sessionID)
	assert.ErrorIs(t, err, ErrJournalUnauthenticated, "ticket added to the snapshot")

	e = newFetched(t)
	require.NoError(t, e.svc.Secrets.Delete(ctx, remote.JournalKeySecret(e.sessionID)))
	_, err = e.svc.PlanReplay(ctx, e.sessionID)
	assert.ErrorIs(t, err, ErrJournalUnauthenticated, "no key on the machine")

	// The local copy of the manifest never holds the key.
	e = newFetched(t)
	data, err := os.ReadFile(filepath.Join(e.svc.RemoteDir(e.sessionID), remote.ManifestFile))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "journal_key")
	key, _ := e.svc.Secrets.Get(ctx, remote.JournalKeySecret(e.sessionID))
	assert.NotContains(t, string(data), key)
}
