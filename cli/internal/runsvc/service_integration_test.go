//go:build integration

package runsvc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type mapSecrets map[string]string

func (m mapSecrets) Get(_ context.Context, k string) (string, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	return "", nil // real stores: absent key → empty value
}

type fixture struct {
	svc     *Service
	store   *sqlite.Store
	project string
	bundle  *bundle.Bundle
	adapter *opencodev2.Adapter
}

func writeSkill(t *testing.T, dir, id string) {
	require.NoError(t, os.MkdirAll(filepath.Join(dir, id), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, id, "SKILL.md"), []byte("---\nname: "+id+"\ndescription: skill "+id+"\n---\nbody\n"), 0o644))
}

func newFixture(t *testing.T, secrets mapSecrets) *fixture {
	t.Helper()
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode binary not found")
	}
	root, err := os.MkdirTemp("/tmp", "ohrun-") // short path for the daemon socket
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(root) })

	st, err := sqlite.Open(filepath.Join(root, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", project).Run())
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', ?)`, project)
	require.NoError(t, err)

	a := opencodev2.New("", filepath.Join(root, "cache"))
	_, err = a.Detect(context.Background())
	require.NoError(t, err)

	paths := daemon.Paths{Dir: filepath.Join(root, "run")}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Options{Paths: paths, Version: "test", Grants: sqlite.NewGrantStore(st), Servers: sqlite.NewServerStore(st), Secrets: secrets, Tick: time.Second,
			Sessions: sqlite.NewSessionStore(st), SessionsDir: filepath.Join(root, "sessions"),
			Adapter: func(string) adapters.ToolAdapter { return a }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	dc := daemon.NewClient(paths)
	require.Eventually(t, func() bool { _, err := dc.Health(context.Background()); return err == nil }, 5*time.Second, 20*time.Millisecond)

	skills := filepath.Join(root, "bundle-src", "skills")
	writeSkill(t, skills, "alpha")
	b := &bundle.Bundle{Spec: sessionspec.BundleSpec{
		Hash: "testbundlehash0001", Root: filepath.Join(root, "bundle-src"), EntryAgent: "lead",
		Agents:    []sessionspec.AgentDef{{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD. Always start every reply with the exact prefix 'LEAD:'."}},
		Skills:    []sessionspec.SkillDef{{ID: "alpha", Dir: filepath.Join(skills, "alpha")}},
		SkillsDir: skills, MaxDepth: 1,
		DefaultModel: func() *sessionspec.ModelRef {
			m := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
			return &m
		}(),
	}}

	svc := &Service{
		Adapter: a, AdapterVer: a.Ver, Servers: sqlite.NewServerStore(st), Sessions: sqlite.NewSessionStore(st),
		Secrets: secrets, ServersDir: filepath.Join(root, "servers"), BundlesDir: filepath.Join(root, "bundles"), SessionsDir: filepath.Join(root, "sessions"), Executable: "/usr/local/bin/oh",
		Daemon: func(context.Context) (DaemonClient, error) { return dc, nil },
	}
	t.Cleanup(func() {
		if list, err := svc.Servers.List(context.Background()); err == nil {
			for _, s := range list {
				_ = a.StopServer(context.Background(), handle(&s))
			}
		}
	})
	return &fixture{svc: svc, store: st, project: project, bundle: b, adapter: a}
}

func (f *fixture) request(prompt string) StartRequest {
	return StartRequest{
		ProjectID: "p1", Location: f.project, Bundle: f.bundle, Title: "test", Prompt: prompt,
		WorkflowID: "lead", Mode: "semi-auto", Provider: "bedrock",
		Attach: sessionspec.AttachNone,
	}
}

func TestStartSessionAndReuseServer(t *testing.T) {
	f := newFixture(t, mapSecrets{"openhub.provider.bedrock.token": "fake-key"})
	ctx := context.Background()

	r1, err := f.svc.StartSession(ctx, f.request(""))
	require.NoError(t, err)
	assert.False(t, r1.Reused)
	assert.True(t, r1.Report.OK())
	assert.True(t, strings.HasPrefix(r1.SessionID, "ses_"))
	assert.Equal(t, domain.ServerReady, r1.Server.Status)
	assert.Contains(t, r1.Server.ProxyToken, "ohs_")

	sess, err := f.svc.Sessions.Get(ctx, r1.SessionID)
	require.NoError(t, err)
	assert.Equal(t, r1.GroupKey, sess.GroupKey)
	assert.Contains(t, []domain.RunState{domain.RunActive, domain.RunIdle}, sess.State, "no prompt: the watcher may already report idle")
	assert.Equal(t, "lead", sess.EntryAgent)

	r2, err := f.svc.StartSession(ctx, f.request(""))
	require.NoError(t, err)
	assert.True(t, r2.Reused, "same bundle + project → same server")
	assert.Equal(t, r1.Server.PID, r2.Server.PID)
	assert.NotEqual(t, r1.SessionID, r2.SessionID)

	argv, env, err := f.svc.AttachCommand(ctx, r1.SessionID)
	require.NoError(t, err)
	assert.Equal(t, []string{"--server", r1.Server.URL, "-s", r1.SessionID}, argv[1:])
	assert.Equal(t, []string{"OPENCODE_SERVER_PASSWORD=" + r1.Server.Password}, env)
	assert.NotContains(t, strings.Join(argv, " "), r1.Server.Password)

	// The server process holds only the proxy token, never the real key.
	ps, err := exec.Command("ps", "eww", "-p", strconv.Itoa(r1.Server.PID)).Output()
	require.NoError(t, err)
	assert.NotContains(t, string(ps), "fake-key")
	assert.Contains(t, string(ps), r1.Server.ProxyToken)
}

func TestStartSessionWithoutCredential(t *testing.T) {
	f := newFixture(t, mapSecrets{})
	req := f.request("")
	req.Provider = "anthropic"
	_, err := f.svc.StartSession(context.Background(), req)
	assert.ErrorContains(t, err, "no credential")
}

func TestResumeSleepingSessionAndStop(t *testing.T) {
	f := newFixture(t, mapSecrets{"openhub.provider.bedrock.token": "fake-key"})
	ctx := context.Background()
	// Resume loads the bundle by hash: persist the fixture bundle.
	dir := filepath.Join(f.svc.BundlesDir, f.bundle.Spec.Hash)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data, _ := json.Marshal(f.bundle.Spec)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.json"), data, 0o644))
	minted := 0
	f.svc.SessionEnv = func(context.Context, SessionEnvRequest) (map[string]string, error) {
		v := "tok-" + strconv.Itoa(minted)
		minted++
		return map[string]string{"OH_TEST_DYN": v}, nil
	}
	r, err := f.svc.StartSession(ctx, withEnv(f.request("")))
	require.NoError(t, err)
	firstPID := r.Server.PID
	assert.Equal(t, "static=one dyn=tok-0 id="+r.SessionID, sessionShellEnv(t, r.Server, r.SessionID, f.project))

	// Simulate the daemon putting the group to sleep.
	require.NoError(t, f.adapter.StopServer(ctx, handle(r.Server)))
	require.NoError(t, f.svc.Servers.SetStatus(ctx, r.GroupKey, domain.ServerSleeping))
	_, _, err = f.svc.AttachCommand(ctx, r.SessionID)
	require.ErrorIs(t, err, ErrServerNotRunning)

	req := f.request("")
	req.Bundle, req.Location = nil, ""
	require.NoError(t, f.svc.ResumeSession(ctx, r.SessionID, req))
	srv, err := f.svc.Servers.Get(ctx, r.GroupKey)
	require.NoError(t, err)
	assert.Equal(t, domain.ServerReady, srv.Status)
	assert.NotEqual(t, firstPID, srv.PID, "a new server process")
	_, _, err = f.svc.AttachCommand(ctx, r.SessionID)
	require.NoError(t, err)

	// The session survived the restart (same data dir).
	got, err := opencodev2.NewClient(srv.URL, srv.Password).GetSession(ctx, r.SessionID)
	require.NoError(t, err)
	assert.Equal(t, r.SessionID, got.ID)
	// Its environment was applied again (static + fresh dynamic values).
	assert.Equal(t, "static=one dyn=tok-1 id="+r.SessionID, sessionShellEnv(t, srv, r.SessionID, f.project))

	require.NoError(t, f.svc.StopSession(ctx, r.SessionID))
	sess, _ := f.svc.Sessions.Get(ctx, r.SessionID)
	assert.Equal(t, domain.RunStopped, sess.State)
	srv, _ = f.svc.Servers.Get(ctx, r.GroupKey)
	assert.Equal(t, domain.ServerStopped, srv.Status)
	assert.Error(t, f.svc.ResumeSession(ctx, r.SessionID, f.request("")), "a stopped session cannot be resumed")
}

func withEnv(r StartRequest) StartRequest {
	r.SessionEnv = map[string]string{"OH_TEST_STATIC": "one"}
	return r
}

// sessionShellEnv runs a shell command in the session and reads back the
// session-specific variables it saw (through a file: no LLM involved).
func sessionShellEnv(t *testing.T, srv *domain.Server, sessionID, dir string) string {
	t.Helper()
	out := filepath.Join(dir, "env-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".txt")
	c := opencodev2.NewClient(srv.URL, srv.Password)
	require.NoError(t, c.Shell(context.Background(), sessionID,
		`printf 'static=%s dyn=%s id=%s' "$OH_TEST_STATIC" "$OH_TEST_DYN" "$OH_SESSION_ID" > `+out))
	var got string
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(out)
		got = string(data)
		return err == nil && got != ""
	}, 10*time.Second, 50*time.Millisecond)
	return got
}

func TestForkSession(t *testing.T) {
	f := newFixture(t, mapSecrets{"openhub.provider.bedrock.token": "fake-key"})
	ctx := context.Background()
	r, err := f.svc.StartSession(ctx, withEnv(f.request("")))
	require.NoError(t, err)
	_, err = f.svc.ForkSession(ctx, r.SessionID)
	assert.ErrorContains(t, err, "empty", "opencode refuses to fork a session without history")
	sessionShellEnv(t, r.Server, r.SessionID, f.project) // gives the session some history

	child, err := f.svc.ForkSession(ctx, r.SessionID)
	require.NoError(t, err)
	sess, err := f.svc.Sessions.Get(ctx, child)
	require.NoError(t, err)
	assert.Equal(t, r.GroupKey, sess.GroupKey)
	assert.Equal(t, "fork · test", *sess.Title)
	assert.Equal(t, "static=one dyn= id="+child, sessionShellEnv(t, r.Server, child, f.project), "static environment copied, own session id")
}
