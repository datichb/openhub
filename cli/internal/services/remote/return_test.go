package remote

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
	"github.com/datichb/openhub/cli/internal/services/report"
	"github.com/datichb/openhub/cli/internal/services/wiki"
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
	e.svc.Human = func(context.Context, string) string { return "Ada" }
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
	// The runner seals the journal with the key of the envelope.
	key, _ := e.svc.Secrets.Get(context.Background(), remote.JournalKeySecret(e.sessionID))
	require.NotEmpty(t, key, "journal key kept on the machine")
	summary.JournalMAC, _ = remote.Seal(key, "journal", e.sessionID, journal)
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
		"update --notes=done on the runner --actor=Ada via oh/session (remote) -- bd-42", // merged: notes only, status kept local
		"create --json --actor=Ada via oh/session (remote) -- Follow-up",
		// dep add touches bd-42 (merge notes): skipped
		"note --actor=Ada via oh/session (remote) -- bd-43 checked",
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
	e.beads.execFail = "--status=review"
	plan, err := e.svc.ApplyReplay(ctx, e.sessionID, map[string]Resolution{"bd-42": ApplyRemote})
	require.NoError(t, err)
	assert.Equal(t, []string{"create --actor=Ada via oh/session (remote) --json -- Follow-up", "dep add --actor=Ada via oh/session (remote) -- bd-n1 bd-42", "update --status=review --actor=Ada via oh/session (remote) -- bd-42"}, e.beads.execs)
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

// L4: the reports filed in the job come back with the session (reports.json),
// cleaned and bounded again on the machine; fetching again adds nothing.
func TestFetchImportsReports(t *testing.T) {
	ctx := context.Background()
	e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted, Commit: "c9"}, nil)

	// The job side: reports filed in the session of the job, then exported
	// as the runner does.
	jobDB, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { jobDB.Close() })
	_, err = jobDB.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	require.NoError(t, sqlite.NewSessionStore(jobDB).Create(ctx, &domain.Session{ID: e.sessionID, ProjectID: "p1", Status: domain.SessionStatusRunning}))
	job := &report.Service{Store: sqlite.NewReportStore(jobDB), SessionsDir: t.TempDir()}
	_, err = job.Add(ctx, e.sessionID, "developer", "ses_child", "Fait", "## Changements par fichier\n- calc.go")
	require.NoError(t, err)
	_, err = job.Add(ctx, e.sessionID, "reviewer", "ses_child2", "Review", "## Verdict\nvalidé")
	require.NoError(t, err)
	exported, err := job.Export(ctx, e.sessionID)
	require.NoError(t, err)
	var rs remote.Reports
	rs.Schema = remote.ReportsSchema
	for _, x := range exported {
		rs.Reports = append(rs.Reports, remote.ReportEntry{Seq: x.Seq, Agent: x.Agent, ToolSession: x.ToolSession, Title: x.Title, CreatedAt: x.CreatedAt, Body: x.Body})
	}
	rs.Reports[1].Body += "\x1b[2J"                                                                           // not trusted
	rs.Reports = append(rs.Reports, remote.ReportEntry{Seq: 3, Body: strings.Repeat("a", report.MaxBytes+1)}) // too large
	rdata, _ := json.Marshal(rs)
	files := map[string][]byte{"oh-out/reports.json": rdata}
	zr, err := zip.NewReader(bytes.NewReader(e.pipeline.Artifacts[5]), int64(len(e.pipeline.Artifacts[5])))
	require.NoError(t, err)
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = data
	}
	e.pipeline.Artifacts[5] = artifactsZip(t, files)
	e.pipeline.Status = "success"

	machine := &report.Service{Store: sqlite.NewReportStore(e.store), SessionsDir: e.svc.SessionsDir}
	e.svc.Reports = machine
	f, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, f.Reports)
	list, err := machine.List(ctx, e.sessionID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "developer", list[0].Agent)
	assert.Equal(t, "ses_child", list[0].ToolSession)
	assert.True(t, list[0].CreatedAt.Equal(exported[0].CreatedAt))
	_, body, err := machine.Get(ctx, e.sessionID, "rp-2")
	require.NoError(t, err)
	assert.Equal(t, "## Verdict\nvalidé", body)

	f, err = e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.Zero(t, f.Reports, "fetched again: nothing added")
}

// withArtifacts adds files to the artifacts of the run job of e.
func (e *returnEnv) withArtifacts(t *testing.T, add map[string][]byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(e.pipeline.Artifacts[5]), int64(len(e.pipeline.Artifacts[5])))
	require.NoError(t, err)
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		add[f.Name] = data
	}
	e.pipeline.Artifacts[5] = artifactsZip(t, add)
	e.pipeline.Status = "success"
}

// p3-remote-wiki: the wiki proposals of the job come back with the session
// (wiki.json) and are queued, pending, in the queue of the project on the
// machine, where the user validates them; the reports cite them by their
// local id; fetching again adds nothing.
func TestFetchImportsWikiProposals(t *testing.T) {
	ctx := context.Background()
	e := newReturnEnv(t, remote.Summary{Schema: 1, Outcome: remote.OutcomeCompleted, Commit: "c9"}, nil)

	// The job side: proposals filed in the session of the job, never
	// applied there, exported as the runner does.
	jobDB, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { jobDB.Close() })
	job := &wiki.Service{Store: sqlite.NewWikiProposalStore(jobDB), SessionsDir: t.TempDir()}
	for _, c := range []string{"zod pour la validation.", "pino pour les logs."} {
		_, err := job.Propose(ctx, wiki.Filed{ProjectID: "p-api", SessionID: e.sessionID, ToolSession: "ses_child", Agent: "reviewer",
			Proposal: wiki.Proposal{Page: "technical/stack.md", Section: "Librairies clés", Content: c, Tag: "DÉDUIT", Source: "package.json"}})
		require.NoError(t, err)
	}
	filed, err := job.List(ctx, "p-api", e.sessionID)
	require.NoError(t, err)
	wk := remote.Wiki{Schema: remote.WikiSchema}
	for _, p := range filed {
		wk.Proposals = append(wk.Proposals, remote.WikiEntry{ID: p.ID(), Agent: p.Agent, ToolSession: p.ToolSession, Page: p.Page,
			Section: p.Section, Content: p.Content, Tag: p.Tag, Source: p.Source, CreatedAt: p.CreatedAt})
	}
	wk.Proposals = append(wk.Proposals, remote.WikiEntry{ID: "wp-3", Agent: "reviewer", Page: "index.md", Tag: "INCERTAIN",
		Content: "vu dans " + e.svc.SessionsDir + "/" + e.sessionID + "/scratch/debugger/a.md"}) // not trusted: session data
	wdata, _ := json.Marshal(wk)
	rdata, _ := json.Marshal(remote.Reports{Schema: remote.ReportsSchema, Reports: []remote.ReportEntry{{Seq: 1, Agent: "reviewer",
		Title: "Review (wp-1)", Body: "## Verdict\nvalidé ; proposé wp-1 et wp-2, pas wp-12."}}})
	e.withArtifacts(t, map[string][]byte{"oh-out/wiki.json": wdata, "oh-out/reports.json": rdata})

	// The machine: the queue of the project already numbers wp-1.
	machine := &wiki.Service{Store: sqlite.NewWikiProposalStore(e.store), SessionsDir: e.svc.SessionsDir}
	_, err = machine.Propose(ctx, wiki.Filed{ProjectID: "p-api", SessionID: "ses_local", Agent: "auditor-subagent",
		Proposal: wiki.Proposal{Page: "index.md", Content: "déjà en file", Tag: "INCERTAIN"}})
	require.NoError(t, err)
	reports := &report.Service{Store: sqlite.NewReportStore(e.store), SessionsDir: e.svc.SessionsDir}
	e.svc.Wiki, e.svc.Reports = machine, reports

	f, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, f.Wiki, "the one citing the session data is refused again")
	list, err := machine.List(ctx, "p-api", e.sessionID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, []string{"wp-2", "wp-3"}, []string{list[0].ID(), list[1].ID()})
	for _, p := range list {
		assert.Equal(t, domain.WikiPending, p.Status, "validated on the machine")
		assert.Equal(t, "reviewer", p.Agent)
		assert.Equal(t, "ses_child", p.ToolSession)
	}
	_, body, err := reports.Get(ctx, e.sessionID, "rp-1")
	require.NoError(t, err)
	assert.Equal(t, "## Verdict\nvalidé ; proposé wp-2 et wp-3, pas wp-12.", body, "the local ids, in one pass")
	rl, err := reports.List(ctx, e.sessionID)
	require.NoError(t, err)
	assert.Equal(t, "Review (wp-2)", rl[0].Title)

	f, err = e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.Zero(t, f.Wiki, "fetched again: nothing added")

	// --no-import leaves the queue alone.
	_, err = e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	list, _ = machine.List(ctx, "p-api", e.sessionID)
	assert.Len(t, list, 2)
}

// R3-remote: the verifications run in the job come back with summary.json
// and are handed over to the checkpoints of the machine (not with
// --no-import).
func TestFetchImportsVerifications(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	ver := []remote.SummaryVerification{{Checkpoint: "cp-2", Status: "fail", Expect: "pass", Excerpt: "--- FAIL: TestSub", Started: at,
		Commands: []remote.SummaryVerifyCommand{{Kind: "test", Cmd: "go test ./...", Exit: 1, Millis: 1200}}}}
	e := newReturnEnv(t, remote.Summary{Schema: remote.SummarySchema, Outcome: remote.OutcomeDeferred, Commit: "c9", Verifications: ver}, nil)
	e.pipeline.Status = "success"
	var got []remote.SummaryVerification
	calls := 0
	e.svc.Verifications = func(_ context.Context, sid string, list []remote.SummaryVerification) (int, error) {
		assert.Equal(t, e.sessionID, sid)
		calls++
		got = list
		return len(list), nil
	}
	f, err := e.svc.Fetch(ctx, e.sessionID, FetchOptions{NoImport: true})
	require.NoError(t, err)
	assert.Zero(t, calls, "--no-import: artifacts only")
	assert.Len(t, f.Summary.Verifications, 1)

	f, err = e.svc.Fetch(ctx, e.sessionID, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 1, f.Verifications)
	assert.Equal(t, ver, got)
}
