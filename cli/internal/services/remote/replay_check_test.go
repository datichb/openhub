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
	closing := func(st string) bool { return st != "review" }
	for _, tc := range []struct {
		argv   []string
		refuse string // "" = pending
	}{
		{[]string{"close", "my-app-zz9"}, "not allowed"},
		{[]string{"update", "my-app-zz9", "--status", "in_progress"}, "my-app-zz9 is not a ticket of the session"},
		{[]string{"update", "my-app-x9", "--notes", "ok"}, ""},
		{[]string{"update", "bd-42", "--status", "closed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "--status=Closed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "-sclosed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "-s=closed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "-qsclosed"}, "closing a ticket"},
		{[]string{"update", "bd-42", "--status", "done"}, "closing a ticket"},
		{[]string{"update", "bd-42", "--status", "shipped"}, "closing a ticket"}, // done category of the project
		{[]string{"update", "bd-42", "--status", "unknown"}, "closing a ticket"}, // unknown status: closing
		{[]string{"update", "bd-42", "--status", "review"}, ""},                  // wip category
		{[]string{"done", "bd-42"}, "not allowed"},                               // alias of close
		{[]string{"update", "bd-42", "--DB=/tmp/x"}, "refused"},
		{[]string{"update", "bd-42", "-qC/tmp/other"}, "option --directory refused"},
		{[]string{"update", "--db", "/x", "bd-42"}, "option --db refused"},
		{[]string{"create", "Follow-up", "--repo=/other"}, "option --repo refused"},
		{[]string{"update", "bd-42", "--set-metadata", "k=v"}, "not replayed"},
		{[]string{"update", "bd-42", "--body-file", "/etc/hosts"}, "names a file of the job"},
		{[]string{"update", "bd-42", "--body-file=-"}, ""},
		{[]string{"note", "bd-42", "-f/etc/hosts"}, "unknown option"}, // note has no -f
		{[]string{"comments", "add", "bd-42", "-f/etc/hosts"}, "names a file of the job"},
		{[]string{"label", "add", "bd-42", "my-app-zz9", "needs-review"}, "my-app-zz9 is not a ticket"},
		{[]string{"dep", "rm", "bd-42", "my-app-zz9"}, "not allowed"}, // dep remove not granted
		{[]string{"--json", "update", "my-app-zz9", "--status", "open"}, "my-app-zz9 is not a ticket of the session"},
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
		it := classify(beadswire.JournalEntry{Seq: 1, Argv: tc.argv}, allow, snap, ph, closing)
		if tc.refuse == "" {
			assert.Equal(t, ItemPending, it.State, "%v: %s", tc.argv, it.Reason)
			continue
		}
		assert.Equal(t, ItemRefused, it.State, "%v", tc.argv)
		assert.Contains(t, it.Reason, tc.refuse, "%v", tc.argv)
	}
	// close granted: a closing status is a close.
	for _, argv := range [][]string{{"update", "bd-42", "--status", "closed"}, {"update", "bd-42", "-sshipped"}, {"done", "bd-42"}} {
		it := classify(beadswire.JournalEntry{Argv: argv}, append(allow, "close"), snap, ph, closing)
		assert.Equal(t, ItemPending, it.State, "%v: %s", argv, it.Reason)
	}
	// bd statuses unreadable on the machine: every custom status closes.
	it := classify(beadswire.JournalEntry{Argv: []string{"update", "bd-42", "--status", "review"}}, allow, snap, ph, nil)
	assert.Equal(t, ItemRefused, it.State)
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

// Placeholders are replaced in the arguments and in the options that hold
// ids; bd runs the canonical form of the checked entry.
func TestReplaySubstitutesInIDFlags(t *testing.T) {
	c, why := parseEntry([]string{"create", "Child", "--parent=pending-1", "--deps", "blocks:pending-2,bd-42"})
	require.Empty(t, why)
	got := substitute(c, map[string]string{"pending-1": "bd-n1", "pending-2": "bd-n2"})
	assert.Equal(t, []string{"create", "--parent=bd-n1", "--deps=blocks:bd-n2,bd-42", "--", "Child"}, got.Argv())
	c, _ = parseEntry([]string{"dep", "add", "pending-1", "bd-42"})
	assert.Equal(t, []string{"dep", "add", "--", "bd-n1", "bd-42"}, substitute(c, map[string]string{"pending-1": "bd-n1"}).Argv())
}
