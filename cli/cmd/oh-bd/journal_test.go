package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
)

func journalEnv(t *testing.T) (map[string]string, string) {
	t.Helper()
	dir := t.TempDir()
	snap := remote.Snapshot{Schema: 1, Requested: []string{"bd-42"}, Issues: map[string]json.RawMessage{
		"bd-42": json.RawMessage(`{"id":"bd-42","title":"CSV export","status":"in_progress","description":"Export as CSV","dependencies":[{"id":"bd-7","dependency_type":"blocks"}]}`),
		"bd-7":  json.RawMessage(`{"id":"bd-7","title":"Data model","status":"closed"}`),
		"bd-43": json.RawMessage(`{"id":"bd-43","title":"Excel export","status":"open","dependencies":[{"id":"bd-42","dependency_type":"blocks"}]}`),
		"bd-44": json.RawMessage(`{"id":"bd-44","title":"Docs","status":"open"}`),
	}, Revisions: map[string]string{"bd-42": "r1"}, Children: map[string][]string{"bd-42": {"bd-43"}}}
	data, _ := json.Marshal(snap)
	p := filepath.Join(dir, "snapshot.json")
	require.NoError(t, os.WriteFile(p, data, 0o600))
	j := filepath.Join(dir, "journal.jsonl")
	return map[string]string{beadswire.EnvMode: beadswire.ModeJournal, beadswire.EnvSnapshot: p, beadswire.EnvJournal: j}, j
}

func bd(t *testing.T, e map[string]string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, "/tmp/oh-work/api", envOf(e), strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func readJournal(t *testing.T, path string) []beadswire.JournalEntry {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	defer f.Close()
	var out []beadswire.JournalEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e beadswire.JournalEntry
		require.NoError(t, json.Unmarshal(sc.Bytes(), &e))
		out = append(out, e)
	}
	return out
}

func TestJournalReads(t *testing.T) {
	e, j := journalEnv(t)
	code, out, _ := bd(t, e, "", "show", "bd-42", "--json")
	require.Equal(t, 0, code)
	var recs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &recs))
	assert.Equal(t, "CSV export", recs[0]["title"])

	code, out, _ = bd(t, e, "", "show", "bd-42")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "bd-42 [in_progress] CSV export")
	assert.Contains(t, out, "Export as CSV")

	code, _, errOut := bd(t, e, "", "show", "bd-99")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "not in the snapshot")

	_, out, _ = bd(t, e, "", "list")
	assert.NotContains(t, out, "bd-7", "closed issues hidden by default")
	assert.Contains(t, out, "bd-44")
	_, out, _ = bd(t, e, "", "list", "--status", "closed")
	assert.Contains(t, out, "bd-7")
	_, out, _ = bd(t, e, "", "children", "bd-42", "--json")
	assert.Contains(t, out, "bd-43")
	_, out, _ = bd(t, e, "", "ready")
	assert.Contains(t, out, "bd-44")
	assert.NotContains(t, out, "bd-43", "blocked by bd-42")
	_, out, _ = bd(t, e, "", "search", "excel")
	assert.Contains(t, out, "bd-43")
	_, out, _ = bd(t, e, "", "count")
	assert.Equal(t, "4\n", out)
	assert.Empty(t, readJournal(t, j), "reads are not recorded")
}

func TestJournalWrites(t *testing.T) {
	e, j := journalEnv(t)
	code, out, _ := bd(t, e, "", "update", "bd-42", "--status", "review", "--notes", "done on the runner")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "recorded")
	_, out, _ = bd(t, e, "", "show", "bd-42", "--json")
	assert.Contains(t, out, `"status": "review"`, "later reads see the write")
	assert.Contains(t, out, "done on the runner")

	code, out, _ = bd(t, e, "", "create", "Follow-up", "--json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"id": "pending-2"`)
	code, _, _ = bd(t, e, "body from stdin", "comment", "bd-42", "--stdin")
	require.Equal(t, 0, code)
	_, _, _ = bd(t, e, "", "close", "bd-44", "--reason", "obsolete")
	_, out, _ = bd(t, e, "", "ready")
	assert.NotContains(t, out, "bd-44")

	entries := readJournal(t, j)
	require.Len(t, entries, 4)
	assert.Equal(t, []string{"update", "bd-42", "--status", "review", "--notes", "done on the runner"}, entries[0].Argv)
	assert.Equal(t, 1, entries[0].Seq)
	assert.Equal(t, "/tmp/oh-work/api", entries[0].Cwd)
	assert.WithinDuration(t, time.Now(), entries[0].Time, time.Minute)
	assert.Equal(t, "pending-2", entries[1].Placeholder)
	assert.Equal(t, []byte("body from stdin"), entries[2].Stdin)
	assert.Equal(t, 4, entries[3].Seq)
}

func TestJournalNeedsPaths(t *testing.T) {
	code, _, errOut := bd(t, map[string]string{beadswire.EnvMode: beadswire.ModeJournal}, "", "show", "x")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, beadswire.EnvSnapshot)
	e, _ := journalEnv(t)
	code, out, _ := bd(t, e, "", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "snapshot")
}
