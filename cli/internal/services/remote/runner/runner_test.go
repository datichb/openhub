package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/runsvc"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func jobEnv(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func baseVars() map[string]string {
	return map[string]string{
		remote.VarSessionID: "ses_1", remote.VarProjectID: "42", remote.VarProjectPath: "acme/api", remote.VarRef: "main",
		remote.VarBundleURL:   "https://g/api/v4/projects/1/packages/generic/oh-bundle/abc/bundle.tar.gz",
		remote.VarSessionURL:  "https://g/api/v4/projects/1/packages/generic/oh-session/def/session.tar.gz",
		remote.VarLLMProvider: "bedrock", remote.VarLLMRegion: "eu-west-1", remote.VarLLMKey: "llm-secret-key",
		"OH_PROJECT_TOKEN_42": "project-secret", "CI_JOB_TOKEN": "job-secret", "CI_SERVER_URL": "https://g/",
		remote.VarPipelineSchema: "1", "CI_PIPELINE_ID": "812",
	}
}

func TestReadJob(t *testing.T) {
	j, err := ReadJob(jobEnv(baseVars()))
	require.NoError(t, err)
	assert.Equal(t, int64(42), j.ProjectID)
	assert.Equal(t, "project-secret", j.Secrets.ProjectToken)
	assert.Equal(t, "https://g", j.ServerURL)
	assert.Equal(t, int64(812), j.PipelineID)
	assert.Equal(t, "token [masked] and [masked]", j.Secrets.Redact("token project-secret and llm-secret-key"))

	v := baseVars()
	delete(v, "OH_PROJECT_TOKEN_42")
	delete(v, remote.VarLLMKey)
	_, err = ReadJob(jobEnv(v))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OH_LLM_KEY, OH_PROJECT_TOKEN_42")

	v = baseVars()
	v[remote.VarPipelineSchema] = "9"
	_, err = ReadJob(jobEnv(v))
	assert.ErrorContains(t, err, "oh remote setup")
}

func TestScrubEnv(t *testing.T) {
	t.Setenv("OH_LLM_KEY", "x")
	t.Setenv("CI_JOB_TOKEN", "x")
	t.Setenv("CI_REGISTRY_PASSWORD", "x")
	t.Setenv("OH_PROJECT_TOKEN_42", "x")
	t.Setenv("OH_SESSION_ID", "ses_1")
	removed := ScrubEnv()
	for _, k := range []string{"OH_LLM_KEY", "CI_JOB_TOKEN", "CI_REGISTRY_PASSWORD", "OH_PROJECT_TOKEN_42"} {
		_, ok := os.LookupEnv(k)
		assert.False(t, ok, k)
		assert.Contains(t, removed, k)
	}
	assert.Equal(t, "ses_1", os.Getenv("OH_SESSION_ID"))
}

func TestCISecrets(t *testing.T) {
	s := CISecrets{LLMKey: "k"}
	v, _ := s.Get(context.Background(), "openhub.provider.bedrock.token")
	assert.Equal(t, "k", v)
	v, _ = s.Get(context.Background(), "openhub.provider.anthropic.token.proj-1")
	assert.Equal(t, "k", v)
	v, _ = s.Get(context.Background(), "openhub.mcp.gitlab.token")
	assert.Empty(t, v, "only the LLM key")
}

func TestFetch(t *testing.T) {
	man := remote.Manifest{Schema: remote.ManifestSchema, SessionID: "ses_1", BundleHash: "abc123"}
	md, _ := json.Marshal(man)
	sd, _ := json.Marshal(remote.Snapshot{Schema: 1, Issues: map[string]json.RawMessage{"bd-1": json.RawMessage(`{"id":"bd-1"}`)}})
	env, err := remote.PackFiles(map[string][]byte{remote.ManifestFile: md, remote.SnapshotFile: sd})
	require.NoError(t, err)
	bspec, _ := json.Marshal(sessionspec.BundleSpec{Hash: "abc123", Root: "/Users/x/.oh/bundles/abc123", Skills: []sessionspec.SkillDef{{ID: "s1"}}})
	bun, err := remote.PackFiles(map[string][]byte{"bundle.json": bspec, "skills/s1/SKILL.md": []byte("# s1")})
	require.NoError(t, err)
	envSum := remote.SHA256(env)
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.Header.Get("JOB-TOKEN"))
		switch {
		case strings.Contains(r.URL.Path, "/oh-session/"):
			_, _ = w.Write(env)
		case strings.Contains(r.URL.Path, "/oh-bundle/"):
			_, _ = w.Write(bun)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	j := &Job{SessionID: "ses_1", Secrets: Secrets{JobToken: "job-secret"},
		SessionURL: srv.URL + "/api/v4/projects/1/packages/generic/oh-session/" + envSum + "/session.tar.gz",
		BundleURL:  srv.URL + "/api/v4/projects/1/packages/generic/oh-bundle/abc123/bundle.tar.gz"}
	dir := t.TempDir()
	f, err := Fetch(context.Background(), j, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"job-secret", "job-secret"}, tokens)
	assert.Contains(t, f.Snapshot.Issues, "bd-1")
	assert.Equal(t, filepath.Join(dir, "abc123"), f.Bundle.Spec.Root, "relocated")
	assert.Equal(t, filepath.Join(dir, "abc123", "skills", "s1"), f.Bundle.Spec.Skills[0].Dir)

	j.SessionURL = strings.Replace(j.SessionURL, envSum, strings.Repeat("0", 64), 1)
	_, err = Fetch(context.Background(), j, dir)
	assert.ErrorContains(t, err, "does not match")

	j.SessionURL = srv.URL + "/api/v4/projects/1/packages/generic/oh-session/" + envSum + "/session.tar.gz"
	j.SessionID = "ses_other"
	_, err = Fetch(context.Background(), j, dir)
	assert.ErrorContains(t, err, "pipeline of ses_other")
}

// fakeDecisions is an in-memory decision list answered by Decide.
type fakeDecisions struct {
	mu      sync.Mutex
	open    []domain.Decision
	replies []sessionsvc.Reply
}

func (f *fakeDecisions) ListOpen(_ context.Context, _ domain.DecisionFilter) ([]domain.Decision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Decision{}, f.open...), nil
}

func (f *fakeDecisions) decide(_ context.Context, r sessionsvc.Reply) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, r)
	for i, d := range f.open {
		if d.ID == r.DecisionID {
			f.open = append(f.open[:i], f.open[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeDecisions) add(d domain.Decision) {
	f.mu.Lock()
	f.open = append(f.open, d)
	f.mu.Unlock()
}

func cpDecision(id, cp string) domain.Decision {
	return domain.Decision{ID: id, Kind: domain.DecisionCheckpoint, Payload: domain.DecisionPayload{Title: cp, Data: map[string]any{"checkpoint": cp}}}
}

func TestResponderPolicy(t *testing.T) {
	ctx := context.Background()
	m := &remote.Manifest{Checkpoints: []remote.ManifestCheckpoint{{ID: "cp-1", Policy: "auto"}, {ID: "cp-2", Policy: "defer"}}}
	fd := &fakeDecisions{}
	r := NewResponder(fd, fd.decide, m)

	// auto checkpoint (id case-insensitive) and an ask: approved / rejected, the session goes on.
	fd.add(cpDecision("d1", "CP-1"))
	fd.add(domain.Decision{ID: "d2", Kind: domain.DecisionPermission, Payload: domain.DecisionPayload{Action: "shell", Resources: []string{"rm -rf /"}}})
	assert.Nil(t, r.Step(ctx, "ses_1"))
	require.Len(t, fd.replies, 2)
	assert.Equal(t, "approve", fd.replies[0].Decision)
	assert.Equal(t, "reject", fd.replies[1].Decision, "never approved blindly")
	assert.Equal(t, RejectMessage, fd.replies[1].Message)

	// defer checkpoint: stop, never approved.
	fd.add(cpDecision("d3", "cp-2"))
	st := r.Step(ctx, "ses_1")
	require.NotNil(t, st)
	assert.Equal(t, remote.OutcomeDeferred, st.Outcome)
	assert.Len(t, fd.replies, 2)

	// unknown checkpoint, question, error: stop.
	for _, c := range []struct {
		d       domain.Decision
		outcome string
	}{
		{cpDecision("d4", "cp-x"), remote.OutcomeDeferred},
		{domain.Decision{ID: "d5", Kind: domain.DecisionQuestion, Payload: domain.DecisionPayload{Title: "Which DB?"}}, remote.OutcomeQuestion},
		{domain.Decision{ID: "d6", Kind: domain.DecisionBudget, Payload: domain.DecisionPayload{Message: "budget"}}, remote.OutcomeFailed},
	} {
		fd := &fakeDecisions{}
		r := NewResponder(fd, fd.decide, m)
		fd.add(c.d)
		st := r.Step(ctx, "ses_1")
		require.NotNil(t, st, c.d.ID)
		assert.Equal(t, c.outcome, st.Outcome, c.d.ID)
		assert.Empty(t, fd.replies, c.d.ID)
	}
	answers := r.Answers()
	require.Len(t, answers, 3)
	assert.Equal(t, "approved", answers[0].Answer)
	assert.Equal(t, "rejected", answers[1].Answer)
	assert.Equal(t, "deferred", answers[2].Answer)
}

// fakeSession plays a session: decisions are raised while the turn runs.
type fakeSession struct {
	fd       *fakeDecisions
	started  runsvc.StartRequest
	raise    []domain.Decision
	turns    int
	stopped  bool
	intr     bool
	turnWait time.Duration
	failed   bool
}

func (s *fakeSession) Start(_ context.Context, req runsvc.StartRequest) (string, error) {
	s.started = req
	return req.SessionID, nil
}

func (s *fakeSession) AwaitTurn(ctx context.Context, sid string) (*runsvc.HeadlessResult, error) {
	s.turns++
	if len(s.raise) > 0 {
		d := s.raise[0]
		s.raise = s.raise[1:]
		s.fd.add(d)
		return nil, runsvc.ErrDecisionPending
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.turnWait):
	}
	return &runsvc.HeadlessResult{SessionID: sid, Text: "done"}, nil
}

func (s *fakeSession) Interrupt(context.Context, string) error { s.intr = true; return nil }
func (s *fakeSession) Stop(context.Context, string) error      { s.stopped = true; return nil }
func (s *fakeSession) Results(context.Context, string) (adapters.SessionResult, error) {
	return adapters.SessionResult{Cost: 1.5, TokensIn: 100, TokensOut: 20, Model: "amazon-bedrock/haiku"}, nil
}
func (s *fakeSession) Export(_ context.Context, sid string) ([]json.RawMessage, []string, error) {
	return []json.RawMessage{json.RawMessage(`{"info":{"id":"` + sid + `"}}`)}, []string{sid}, nil
}
func (s *fakeSession) Outputs(context.Context, string) map[string]any {
	return map[string]any{"tickets": []string{"bd-42"}}
}
func (s *fakeSession) Failed(context.Context, string) bool { return s.failed }

type fakeRepo struct {
	cloned, finished bool
	pushErr          error
	result           PushResult
}

func (r *fakeRepo) Clone(context.Context, string, string, string, string, string) (string, error) {
	r.cloned = true
	return "base1", nil
}
func (r *fakeRepo) Finish(context.Context, string, string, string, string, string, string) (*PushResult, error) {
	r.finished = true
	if r.pushErr != nil {
		return nil, r.pushErr
	}
	res := r.result
	return &res, nil
}

type fakeProgress struct{ statuses []string }

func (p *fakeProgress) Set(_ context.Context, status, _, _ string) {
	p.statuses = append(p.statuses, status)
}

func runInput(t *testing.T, sess *fakeSession, repo Repo, m remote.Manifest) (Input, string) {
	t.Helper()
	out := t.TempDir()
	r := NewResponder(sess.fd, sess.fd.decide, &m)
	r.Poll = 10 * time.Millisecond
	return Input{
		Job:     &Job{Secrets: Secrets{ProjectToken: "project-secret"}},
		Fetched: &Fetched{Manifest: m, Bundle: &bundle.Bundle{Spec: sessionspec.BundleSpec{EntryAgent: "lead"}}},
		OutDir:  out, Git: repo, Session: sess, Responder: r, TurnPoll: 10 * time.Millisecond,
	}, out
}

func manifest() remote.Manifest {
	return remote.Manifest{SessionID: "ses_1", Title: "ticket bd-42", Branch: "feat/bd-42", Ref: "main", WorkDir: "/tmp/oh-work/api",
		Mode: "semi-auto", Workflow: remote.ManifestWorkflow{ID: "ticket"}, Project: remote.ManifestProject{OhID: "p-api", Path: "acme/api"},
		Provider: "bedrock", Tickets: []string{"bd-42"},
		Checkpoints: []remote.ManifestCheckpoint{{ID: "cp-1", Policy: "auto"}, {ID: "cp-2", Policy: "defer"}}}
}

func readSummary(t *testing.T, out string) remote.Summary {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, remote.SummaryFile))
	require.NoError(t, err)
	var s remote.Summary
	require.NoError(t, json.Unmarshal(data, &s))
	return s
}

func TestRunCompleted(t *testing.T) {
	fd := &fakeDecisions{}
	sess := &fakeSession{fd: fd, raise: []domain.Decision{cpDecision("d1", "cp-1")}}
	repo := &fakeRepo{result: PushResult{Commit: "c9", MRURL: "https://g/acme/api/-/merge_requests/7"}}
	in, out := runInput(t, sess, repo, manifest())
	prog := &fakeProgress{}
	in.Progress = prog
	require.NoError(t, os.WriteFile(JournalPath(out), []byte("{}\n{}\n"), 0o600))

	sum, err := Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, remote.OutcomeCompleted, sum.Outcome)
	assert.Equal(t, "ses_1", sess.started.SessionID, "same id as on the machine")
	assert.Equal(t, sessionspec.RuntimeRemote, sess.started.Runtime)
	assert.True(t, sess.started.Headless)
	assert.True(t, repo.cloned && repo.finished && sess.stopped)
	assert.False(t, sess.intr)

	s := readSummary(t, out)
	assert.Equal(t, "https://g/acme/api/-/merge_requests/7", s.MRURL)
	assert.Equal(t, "base1", s.BaseCommit)
	assert.Equal(t, 1.5, s.Cost)
	assert.Equal(t, 2, s.JournalEntries)
	assert.Equal(t, "feat/bd-42", s.Outputs["branch"])
	require.Len(t, s.Decisions, 1)
	assert.Equal(t, "approved", s.Decisions[0].Answer)
	assert.FileExists(t, filepath.Join(out, remote.ExportFile))
	assert.Equal(t, []string{"running", "running", "mr_ready"}, prog.statuses)
}

func TestRunDeferredAndFailures(t *testing.T) {
	fd := &fakeDecisions{}
	sess := &fakeSession{fd: fd, raise: []domain.Decision{cpDecision("d2", "cp-2")}, turnWait: time.Minute}
	repo := &fakeRepo{result: PushResult{Commit: "c9", MRURL: "https://g/mr/8"}}
	in, out := runInput(t, sess, repo, manifest())
	sum, err := Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, remote.OutcomeDeferred, sum.Outcome)
	require.NotNil(t, sum.Deferred)
	assert.Equal(t, "cp-2", sum.Deferred.ID)
	assert.True(t, sess.intr, "the session is interrupted at the deferred checkpoint")
	assert.True(t, repo.finished, "the work is pushed: MR ready")
	assert.Equal(t, remote.OutcomeDeferred, readSummary(t, out).Outcome)

	// Error raised as the turn ends: failed, with the message.
	fd = &fakeDecisions{}
	sess = &fakeSession{fd: fd}
	fd.add(domain.Decision{ID: "e1", Kind: domain.DecisionError, Payload: domain.DecisionPayload{Message: "Invalid API Key"}})
	in, out = runInput(t, sess, &fakeRepo{}, manifest())
	in.Responder.Poll = time.Hour // only the final check sees it
	_, err = Run(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, readSummary(t, out).Error, "Invalid API Key")

	// Tool outcome failed without decision.
	fd = &fakeDecisions{}
	sess = &fakeSession{fd: fd, failed: true}
	in, out = runInput(t, sess, &fakeRepo{}, manifest())
	_, err = Run(context.Background(), in)
	require.Error(t, err)
	assert.Equal(t, remote.OutcomeFailed, readSummary(t, out).Outcome)

	// Push failure: failed outcome, token redacted.
	fd = &fakeDecisions{}
	sess = &fakeSession{fd: fd}
	repo = &fakeRepo{pushErr: errors.New("auth failed for project-secret")}
	in, out = runInput(t, sess, repo, manifest())
	_, err = Run(context.Background(), in)
	require.Error(t, err)
	s := readSummary(t, out)
	assert.Equal(t, remote.OutcomeFailed, s.Outcome)
	assert.NotContains(t, s.Error, "project-secret")
	assert.Contains(t, s.Error, "[masked]")
}

// Real git: clone, uncommitted work committed, push with merge request
// options to a bare origin that accepts push options.
func TestGitCloneAndFinish(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	ctx := context.Background()
	root := t.TempDir()
	gitc := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	origin := filepath.Join(root, "origin.git")
	gitc(root, "init", "-q", "--bare", "-b", "main", origin)
	gitc(origin, "config", "receive.advertisePushOptions", "true")
	seed := filepath.Join(root, "seed")
	gitc(root, "clone", "-q", origin, seed)
	require.NoError(t, os.WriteFile(filepath.Join(seed, "README.md"), []byte("x"), 0o644))
	gitc(seed, "add", ".")
	gitc(seed, "commit", "-q", "-m", "init")
	gitc(seed, "push", "-q", "origin", "HEAD:main")

	g := Git{Token: "project-secret"}
	work := filepath.Join(root, "work")
	base, err := g.Clone(ctx, origin, "main", "", "feat/bd-42", work)
	require.NoError(t, err)
	assert.Len(t, base, 40)
	assert.Equal(t, "feat/bd-42", gitc(work, "rev-parse", "--abbrev-ref", "HEAD"))
	cfg, _ := os.ReadFile(filepath.Join(work, ".git", "config"))
	assert.NotContains(t, string(cfg), "Authorization", "token never written in the clone")

	res, err := g.Finish(ctx, work, base, "main", "feat/bd-42", "oh: nothing", "")
	require.NoError(t, err)
	assert.Empty(t, res.Commit, "nothing to push")

	require.NoError(t, os.WriteFile(filepath.Join(work, "csv.go"), []byte("package x"), 0o644))
	res, err = g.Finish(ctx, work, base, "main", "feat/bd-42", "oh: ticket bd-42", "Remote oh session")
	require.NoError(t, err)
	assert.Len(t, res.Commit, 40)
	assert.Equal(t, res.Commit, gitc(origin, "rev-parse", "feat/bd-42"), "branch pushed")
	assert.Equal(t, "oh: ticket bd-42", gitc(origin, "log", "-1", "--format=%s", "feat/bd-42"))
}

func TestTeamProgress(t *testing.T) {
	rec := &recClaims{}
	p := &TeamProgress{Repo: rec, Project: "p", Tickets: []string{"bd-1", "bd-2"}, Session: "ses_1", Pipeline: 812}
	p.Set(context.Background(), "mr_ready", "completed", "https://g/mr/1")
	require.Len(t, rec.got, 2)
	assert.Equal(t, "mr_ready", rec.got[0].Status)
	assert.Equal(t, int64(812), rec.got[1].Pipeline)
}

type recClaims struct{ got []teamstate.ClaimRemote }

func (r *recClaims) SetClaimRemote(_ context.Context, _, _ string, rem teamstate.ClaimRemote) error {
	r.got = append(r.got, rem)
	return nil
}

func TestInstallUserAndLibc(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte("root:x:0:0:root:/root:/bin/sh"), 0o644))
	o := InstallOptions{Root: root}
	require.NoError(t, ensureUser(o))
	require.NoError(t, ensureUser(o), "idempotent")
	data, _ := os.ReadFile(filepath.Join(root, "etc", "passwd"))
	assert.Equal(t, 1, bytes.Count(data, []byte("\noh:x:10001:10001:")))
	assert.Equal(t, "glibc", Libc(root))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "lib", "ld-musl-x86_64.so.1"), nil, 0o644))
	assert.Equal(t, "musl", Libc(root))
	assert.ErrorContains(t, Install(context.Background(), InstallOptions{Root: root}), "OH_OPENCODE_VERSION")
}

func TestJobRuntimeCommand(t *testing.T) {
	rt := &JobRuntime{Home: t.TempDir()}
	cmd, err := rt.Command(context.Background(), nil, ohProc([]string{"sh", "serve", "--hostname", "0.0.0.0"}, map[string]string{"OPENCODE_SERVER_PASSWORD": "pw"}, t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, []string{"serve", "--hostname", "127.0.0.1"}, cmd.Args[1:], "loopback only")
	env := strings.Join(cmd.Env, "\n")
	assert.Contains(t, env, "OPENCODE_SERVER_PASSWORD=pw")
	assert.Contains(t, env, "HOME="+rt.Home)
	assert.NotContains(t, env, "CI_JOB_TOKEN")
}

func ohProc(argv []string, env map[string]string, dir string) ohruntime.Proc {
	return ohruntime.Proc{Argv: argv, Env: env, Dir: dir}
}
