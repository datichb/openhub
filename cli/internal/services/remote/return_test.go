package remote

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func artifactsZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, d := range files {
		w, err := zw.Create(n)
		require.NoError(t, err)
		_, _ = w.Write(d)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func journalLines(entries ...beadswire.JournalEntry) []byte {
	var b bytes.Buffer
	for _, e := range entries {
		data, _ := json.Marshal(e)
		b.Write(append(data, '\n'))
	}
	return b.Bytes()
}

type returnEnv struct {
	*sendEnv
	adopted   [][]byte
	adoptLoc  string
	notified  []string
	pipeline  *gitlabtest.Pipeline
	sessionID string
}

func newReturnEnv(t *testing.T, summary remote.Summary, journal []byte) *returnEnv {
	t.Helper()
	e := &returnEnv{sendEnv: newSendEnv(t)}
	e.svc.SessionsDir = t.TempDir()
	e.req.BeadsAllow = []string{"show", "update", "create", "dep add", "note"}
	e.svc.Target = func(name string) (*config.RemoteTarget, bool) {
		if name == e.target.Name {
			tg := e.target
			return &tg, true
		}
		return nil, false
	}
	e.svc.Adopt = func(_ context.Context, sid string, trs [][]byte, loc string) error {
		e.adopted, e.adoptLoc = trs, loc
		sess, err := e.svc.Sessions.Get(context.Background(), sid)
		require.NoError(t, err)
		sess.Runtime, sess.LaunchPath = "local", loc
		return e.svc.Sessions.Update(context.Background(), sess)
	}
	e.svc.Worktree = func(dir, branch string) (string, error) { return dir + "-" + strings.ReplaceAll(branch, "/", "-"), nil }
	e.svc.Notify = func(title, body string) { e.notified = append(e.notified, title+": "+body) }
	res, err := e.svc.Send(context.Background(), e.req)
	require.NoError(t, err)
	e.sessionID = res.SessionID
	e.pipeline = e.srv.Project("acme/dev/oh-runner").Pipelines[0]
	sdata, _ := json.Marshal(summary)
	ex, _ := json.Marshal(remote.Export{Schema: 1, Sessions: []json.RawMessage{json.RawMessage(`{"info":{"id":"ses_remote1"}}`), json.RawMessage(`{"info":{"id":"ses_child"}}`)}})
	e.pipeline.Jobs = []map[string]any{{"id": 4, "name": "oh-image"}, {"id": 5, "name": "oh-run"}}
	e.pipeline.Artifacts = map[int64][]byte{5: artifactsZip(t, map[string][]byte{
		"oh-out/summary.json": sdata, "oh-out/session.export": ex, "oh-out/journal.jsonl": journal, "oh-out/other.txt": []byte("x"),
	})}
	return e
}

func TestTrackAndFetch(t *testing.T) {
	ctx := context.Background()
	e := newReturnEnv(t, remote.Summary{Schema: 1, SessionID: "ses_remote1", Outcome: remote.OutcomeDeferred, Commit: "c9",
		MRURL: "https://g/mr/7", Cost: 1.25, TokensIn: 10, Outputs: map[string]any{"merge_request": "https://g/mr/7"}},
		journalLines(beadswire.JournalEntry{Seq: 1, Argv: []string{"update", "bd-42", "--status", "review"}}))

	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	assert.ErrorIs(t, err, ErrNotFinished, "pipeline created")

	e.pipeline.Status = "running"
	res, err := e.svc.Track(ctx)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, domain.RemoteRunning, res[0].Ref.Status)
	assert.Empty(t, e.notified)

	e.pipeline.Status = "success"
	_, err = e.svc.Track(ctx)
	require.NoError(t, err)
	assert.Len(t, e.notified, 1, "ready to fetch")
	ref, _ := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.True(t, ToFetch(*ref))
	sess, _ := e.svc.Sessions.Get(ctx, e.sessionID)
	assert.Equal(t, domain.RunCompleted, sess.State)
	_, err = e.svc.Track(ctx)
	require.NoError(t, err)
	assert.Len(t, e.notified, 1, "notified once")

	f, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.True(t, f.Imported)
	assert.Len(t, e.adopted, 2, "session and sub-agent session")
	assert.Equal(t, e.req.ProjectDir+"-feat-bd-42", e.adoptLoc, "worktree of the pushed branch")
	assert.Equal(t, []string{"feat/bd-42"}, e.git.fetchedBranches)
	assert.Len(t, f.Journal, 1)
	assert.FileExists(t, e.svc.RemoteDir(e.sessionID)+"/summary.json")
	assert.NoFileExists(t, e.svc.RemoteDir(e.sessionID)+"/other.txt")
	sess, _ = e.svc.Sessions.Get(ctx, e.sessionID)
	assert.Equal(t, 1.25, sess.Cost)
	assert.Equal(t, "https://g/mr/7", sess.Outputs["merge_request"])
	ref, _ = sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteFetched, ref.Status)
	assert.Equal(t, "https://g/mr/7", ref.MRURL)
	assert.True(t, ToResolve(*ref))
}

func TestFetchWithoutJournalIsResolved(t *testing.T) {
	ctx := context.Background()
	e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted}, nil)
	e.pipeline.Status = "failed"
	f, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	assert.False(t, f.Imported)
	assert.Equal(t, domain.RemoteResolved, f.Ref.Status)
	assert.Contains(t, f.Ref.Error, "pipeline failed")

	e2 := newReturnEnv(t, remote.Summary{}, nil)
	e2.pipeline.Status = "success"
	e2.pipeline.Jobs = nil
	_, err = e2.svc.Fetch(ctx, e2.sessionID, FetchOptions{})
	assert.ErrorIs(t, err, ErrNoArtifacts)
}

// Critère 2: the journal is replayed with a simulated conflict, resolved.
func TestReplayWithConflict(t *testing.T) {
	ctx := context.Background()
	journal := journalLines(
		beadswire.JournalEntry{Seq: 1, Argv: []string{"update", "bd-42", "--status", "review", "--notes", "done on the runner"}},
		beadswire.JournalEntry{Seq: 2, Argv: []string{"create", "Follow-up", "--json"}, Placeholder: "pending-2"},
		beadswire.JournalEntry{Seq: 3, Argv: []string{"dep", "add", "pending-2", "bd-42"}},
		beadswire.JournalEntry{Seq: 4, Argv: []string{"delete", "bd-42"}},
		beadswire.JournalEntry{Seq: 5, Argv: []string{"update", "bd-99", "--status", "closed"}},
		beadswire.JournalEntry{Seq: 6, Argv: []string{"update", "bd-43", "--db", "/x"}},
		beadswire.JournalEntry{Seq: 7, Argv: []string{"update", "bd-43", "--body-file", "/tmp/oh-work/x.md"}},
		beadswire.JournalEntry{Seq: 8, Argv: []string{"note", "bd-43", "checked"}},
	)
	e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted}, journal)
	e.pipeline.Status = "success"
	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)

	// Meanwhile on the machine, alice moved bd-42 (new revision).
	e.beads.issues["bd-42"]["status"] = "in_progress"
	e.beads.issues["bd-42"]["assignee"] = "alice"
	e.beads.issues["bd-42"]["revision"] = "r3-local"

	plan, err := e.svc.PlanReplay(ctx, e.sessionID)
	require.NoError(t, err)
	states := map[int]string{}
	for _, it := range plan.Items {
		states[it.Seq] = it.State
	}
	assert.Equal(t, map[int]string{1: ItemPending, 2: ItemPending, 3: ItemPending, 4: ItemRefused, 5: ItemRefused,
		6: ItemRefused, 7: ItemRefused, 8: ItemPending}, states)
	require.Len(t, plan.Conflicts, 1, "bd-43 untouched locally: no conflict")
	c := plan.Conflicts[0]
	assert.Equal(t, "bd-42", c.Ticket)
	assert.Equal(t, []FieldChange{{Field: "status", Snapshot: "in_progress", Local: "in_progress", Remote: "review"}}, c.Fields)
	assert.Equal(t, []string{"done on the runner"}, c.Notes)
	assert.Equal(t, []int{1, 3}, c.Entries)

	// Nothing without a resolution.
	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	assert.ErrorIs(t, err, ErrUnresolved)
	assert.Empty(t, e.beads.execs)

	plan, err = e.svc.ApplyReplay(ctx, e.sessionID, map[string]Resolution{"bd-42": MergeNotes})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"update bd-42 --notes done on the runner", // merged: notes only, status kept local
		"create Follow-up --json",
		// dep add touches bd-42 (merge notes): skipped
		"note bd-43 checked",
	}, e.beads.execs)
	states = map[int]string{}
	for _, it := range plan.Items {
		states[it.Seq] = it.State
	}
	assert.Equal(t, ItemSkipped, states[3])
	assert.Equal(t, ItemApplied, states[2])
	ref, _ := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteResolved, ref.Status)

	// Idempotent: a second run applies nothing more.
	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	require.NoError(t, err)
	assert.Len(t, e.beads.execs, 3)
}

func TestReplayApplyRemoteSubstitutesPlaceholders(t *testing.T) {
	ctx := context.Background()
	journal := journalLines(
		beadswire.JournalEntry{Seq: 1, Argv: []string{"create", "Follow-up"}, Placeholder: "pending-1"},
		beadswire.JournalEntry{Seq: 2, Argv: []string{"dep", "add", "pending-1", "bd-42"}},
		beadswire.JournalEntry{Seq: 3, Argv: []string{"update", "bd-42", "--status", "review"}},
	)
	e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted}, journal)
	e.pipeline.Status = "success"
	_, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	e.beads.issues["bd-42"]["revision"] = "r3-local"
	e.beads.execFail = "--status review"
	plan, err := e.svc.ApplyReplay(ctx, e.sessionID, map[string]Resolution{"bd-42": ApplyRemote})
	require.NoError(t, err)
	assert.Equal(t, []string{"create Follow-up --json", "dep add bd-n1 bd-42", "update bd-42 --status review"}, e.beads.execs)
	assert.Equal(t, ItemFailed, plan.Items[2].State)
	ref, _ := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteFetched, ref.Status, "a failed entry keeps the session to resolve")

	// Retry once bd works: only the failed entry runs again.
	e.beads.execFail = ""
	_, err = e.svc.ApplyReplay(ctx, e.sessionID, nil)
	require.NoError(t, err)
	assert.Len(t, e.beads.execs, 4)
	ref, _ = sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, e.sessionID)
	assert.Equal(t, domain.RemoteResolved, ref.Status)
}
