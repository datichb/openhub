package runsvc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/provider"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type fakeSecrets map[string]string

func (m fakeSecrets) Get(_ context.Context, k string) (string, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	return "", domain.ErrNotFound
}

// rtAdapter is a tool adapter whose servers are the test process itself
// (always alive): only the calls made by the service are checked.
type rtAdapter struct {
	mu        sync.Mutex
	started   []adapters.ServerGroup
	attested  []string
	created   []sessionspec.SessionSpec
	stopped   int
	attestErr error
}

func (a *rtAdapter) Name() string { return "fake" }
func (a *rtAdapter) Detect(context.Context) (adapters.ToolInfo, error) {
	return adapters.ToolInfo{}, nil
}
func (a *rtAdapter) Capabilities() adapters.Capabilities { return adapters.Capabilities{} }
func (a *rtAdapter) ContainerTool() ohruntime.Tool       { return fakeTool{} }
func (a *rtAdapter) SendPrompt(context.Context, adapters.ServerHandle, string, string) error {
	return nil
}
func (a *rtAdapter) Render(sessionspec.BundleSpec, sessionspec.ProviderSpec) (adapters.RenderedConfig, error) {
	return adapters.RenderedConfig{}, nil
}
func (a *rtAdapter) StartServer(_ context.Context, g adapters.ServerGroup) (adapters.ServerHandle, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = append(a.started, g)
	return adapters.ServerHandle{URL: "http://127.0.0.1:4096", Password: "pw", PID: os.Getpid()}, nil
}
func (a *rtAdapter) StopServer(context.Context, adapters.ServerHandle) error {
	a.mu.Lock()
	a.stopped++
	a.mu.Unlock()
	return nil
}
func (a *rtAdapter) Attest(_ context.Context, _ adapters.ServerHandle, _ sessionspec.BundleSpec, loc string) (adapters.VisibilityReport, error) {
	a.mu.Lock()
	a.attested = append(a.attested, loc)
	err := a.attestErr
	a.mu.Unlock()
	return adapters.VisibilityReport{}, err
}
func (a *rtAdapter) CreateSession(_ context.Context, _ adapters.ServerHandle, s sessionspec.SessionSpec) error {
	a.mu.Lock()
	a.created = append(a.created, s)
	a.mu.Unlock()
	return nil
}
func (a *rtAdapter) AttachCommand(adapters.ServerHandle, string) ([]string, []string) {
	return nil, nil
}
func (a *rtAdapter) Events(context.Context, adapters.ServerHandle) (<-chan adapters.ToolEvent, error) {
	return nil, errors.New("no events")
}
func (a *rtAdapter) ActiveSessions(context.Context, adapters.ServerHandle) ([]string, error) {
	return nil, nil
}
func (a *rtAdapter) Pending(context.Context, adapters.ServerHandle, string) ([]adapters.PendingDecision, error) {
	return nil, nil
}
func (a *rtAdapter) Reply(context.Context, adapters.ServerHandle, adapters.DecisionReply) error {
	return nil
}
func (a *rtAdapter) Control(context.Context, adapters.ServerHandle, string, adapters.ControlOp) error {
	return nil
}
func (a *rtAdapter) Results(context.Context, adapters.ServerHandle, string) (adapters.SessionResult, error) {
	return adapters.SessionResult{}, nil
}

type fakeTool struct{}

func (fakeTool) Name() string    { return "tool" }
func (fakeTool) Version() string { return "1" }
func (fakeTool) LinuxBinary(context.Context, string, string) (string, error) {
	return "/bin/tool", nil
}

// fakeRuntime maps every location to /work/<base>.
type fakeRuntime struct {
	mu         sync.Mutex
	listenHost string
	prepared   map[string]*ohruntime.Prepared // by group id
	prepares   []ohruntime.Group
	teardowns  int
}

func (r *fakeRuntime) Kind() sessionspec.RuntimeKind { return sessionspec.RuntimeContainer }
func (r *fakeRuntime) Available(context.Context) (ohruntime.Availability, error) {
	return ohruntime.Availability{OK: true}, nil
}
func (r *fakeRuntime) HostAddress() string { return "host.docker.internal" }
func (r *fakeRuntime) Prepare(_ context.Context, g ohruntime.Group) (*ohruntime.Prepared, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prepares = append(r.prepares, g)
	pg := &ohruntime.Prepared{Group: g, HostAddress: "host.docker.internal", ListenHost: r.listenHost}
	pg.Paths = append(pg.Paths, ohruntime.Mapping{Host: g.BundleDir, Inner: "/opt/oh/bundle"}, ohruntime.Mapping{Host: g.DataDir, Inner: "/opt/oh/data"})
	for _, l := range g.Locations {
		pg.Paths = append(pg.Paths, ohruntime.Mapping{Host: l, Inner: "/work/" + filepath.Base(l)})
	}
	r.prepared[g.GroupID] = pg
	return pg, nil
}
func (r *fakeRuntime) Load(_ context.Context, g ohruntime.Group) (*ohruntime.Prepared, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pg, ok := r.prepared[g.GroupID]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := *pg
	cp.Group.Locations = append([]string(nil), pg.Group.Locations...)
	return &cp, nil
}
func (r *fakeRuntime) Command(context.Context, *ohruntime.Prepared, ohruntime.Proc) (*exec.Cmd, error) {
	return nil, errors.New("unused")
}
func (r *fakeRuntime) Teardown(context.Context, *ohruntime.Prepared) error {
	r.mu.Lock()
	r.teardowns++
	r.mu.Unlock()
	return nil
}

type fakeDaemon struct {
	mu      sync.Mutex
	listens []string
	revoked []string
}

func (d *fakeDaemon) IssueGrant(context.Context, daemon.GrantRequest) (daemon.GrantResponse, error) {
	return daemon.GrantResponse{Token: "ohs_tok", BaseURL: "http://127.0.0.1:5555/amazon-bedrock"}, nil
}
func (d *fakeDaemon) RevokeOwner(_ context.Context, owner string) error {
	d.mu.Lock()
	d.revoked = append(d.revoked, owner)
	d.mu.Unlock()
	return nil
}
func (d *fakeDaemon) Usage(context.Context, string) (daemon.UsageResponse, error) {
	return daemon.UsageResponse{}, nil
}
func (d *fakeDaemon) Touch(context.Context, string) error { return nil }
func (d *fakeDaemon) ProxyListen(_ context.Context, host string) (string, error) {
	d.mu.Lock()
	d.listens = append(d.listens, host)
	d.mu.Unlock()
	return "http://" + host + ":5556", nil
}

type rtFixture struct {
	svc     *Service
	ad      *rtAdapter
	rt      *fakeRuntime
	dc      *fakeDaemon
	root    string
	project string
	b       *bundle.Bundle
}

func newRTFixture(t *testing.T) *rtFixture {
	t.Helper()
	root := t.TempDir()
	st, err := sqlite.Open(filepath.Join(root, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', ?)`, project)
	require.NoError(t, err)
	f := &rtFixture{ad: &rtAdapter{}, rt: &fakeRuntime{prepared: map[string]*ohruntime.Prepared{}}, dc: &fakeDaemon{}, root: root, project: project}
	f.b = &bundle.Bundle{Dir: filepath.Join(root, "bundles", "h"), Spec: sessionspec.BundleSpec{
		Hash: "bundlehash000001", Root: filepath.Join(root, "bundles", "h"), EntryAgent: "lead",
		Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}},
	}}
	f.svc = &Service{
		Adapter: f.ad, AdapterVer: "1", Servers: sqlite.NewServerStore(st), Sessions: sqlite.NewSessionStore(st),
		Secrets: fakeSecrets{"openhub.provider.bedrock.token": "real-secret"}, ServersDir: filepath.Join(root, "servers"),
		Daemon:   func(context.Context) (DaemonClient, error) { return f.dc, nil },
		Runtimes: map[sessionspec.RuntimeKind]ohruntime.Runtime{sessionspec.RuntimeContainer: f.rt},
	}
	return f
}

func (f *rtFixture) request(loc string) StartRequest {
	return StartRequest{ProjectID: "p1", ProjectDir: f.project, Location: loc, Bundle: f.b, Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, Runtime: sessionspec.RuntimeContainer,
		Dockerfile: "Dockerfile.dev", BuildArgs: map[string]string{"A": "1"}}
}

func (f *rtFixture) worktree(t *testing.T, name string) string {
	p := filepath.Join(f.root, name)
	require.NoError(t, os.MkdirAll(p, 0o755))
	return p
}

func TestStartSessionInContainer(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	res, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)
	assert.Contains(t, res.GroupKey, "-container")

	require.Len(t, f.rt.prepares, 1)
	g := f.rt.prepares[0]
	assert.Equal(t, res.GroupKey, g.GroupID)
	assert.Equal(t, f.b.Spec.Root, g.BundleDir)
	assert.Equal(t, "Dockerfile.dev", g.Dockerfile)
	assert.Equal(t, "1", g.Tool.Version())

	require.Len(t, f.ad.started, 1)
	sg := f.ad.started[0]
	assert.Same(t, f.rt, sg.Runtime)
	assert.Equal(t, "http://host.docker.internal:5555/amazon-bedrock", sg.Provider.BaseURL, "proxy reached through the machine address")
	assert.Equal(t, "ohs_tok", sg.Provider.SessionToken)
	assert.Equal(t, "http://host.docker.internal:5555/oh-gateway", f.svc.loadGatewayURL(res.GroupKey), "Beads gateway next to the proxy")
	assert.Equal(t, []string{"/work/proj"}, f.ad.attested, "attestation in the container's view")
	require.Len(t, f.ad.created, 1)
	assert.Equal(t, "/work/proj", f.ad.created[0].Location)
	assert.Equal(t, sessionspec.RuntimeContainer, f.ad.created[0].Runtime)

	srv, err := f.svc.Servers.Get(ctx, res.GroupKey)
	require.NoError(t, err)
	assert.Equal(t, "container", srv.Runtime)
	assert.Equal(t, f.project, srv.WorkDir, "the registry keeps machine paths")
	sess, err := f.svc.Sessions.Get(ctx, res.SessionID)
	require.NoError(t, err)
	assert.Equal(t, f.project, sess.LaunchPath)
	assert.Equal(t, "container", sess.Runtime)

	// Same location: the running group is reused.
	res2, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)
	assert.True(t, res2.Reused)
	assert.Len(t, f.rt.prepares, 1)
}

func TestContainerGroupNewLocation(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	r1, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)

	// The first session is working: a new worktree gets a sibling group.
	wt1 := f.worktree(t, "proj-a")
	r2, err := f.svc.StartSession(ctx, f.request(wt1))
	require.NoError(t, err)
	assert.Equal(t, r1.GroupKey+"-s1", r2.GroupKey)
	assert.Equal(t, 0, f.ad.stopped, "a busy group is never restarted")

	// Once idle, the group is restarted with both locations mounted.
	for _, id := range []string{r1.SessionID, r2.SessionID} {
		sess, err := f.svc.Sessions.Get(ctx, id)
		require.NoError(t, err)
		sess.State = domain.RunIdle
		require.NoError(t, f.svc.Sessions.Update(ctx, sess))
	}
	wt2 := f.worktree(t, "proj-b")
	r3, err := f.svc.StartSession(ctx, f.request(wt2))
	require.NoError(t, err)
	assert.Equal(t, r1.GroupKey, r3.GroupKey)
	assert.Equal(t, 1, f.ad.stopped)
	assert.Equal(t, 1, f.rt.teardowns)
	last := f.rt.prepares[len(f.rt.prepares)-1]
	assert.ElementsMatch(t, []string{wt2, f.project}, last.Locations, "previous locations stay mounted")
	assert.Equal(t, "/work/proj-b", f.ad.created[len(f.ad.created)-1].Location)
}

func TestContainerProxyListenerOnLinux(t *testing.T) {
	f := newRTFixture(t)
	f.rt.listenHost = "172.17.0.1"
	_, err := f.svc.StartSession(context.Background(), f.request(f.project))
	require.NoError(t, err)
	assert.Equal(t, []string{"172.17.0.1"}, f.dc.listens)
	assert.Equal(t, "http://host.docker.internal:5556/amazon-bedrock", f.ad.started[0].Provider.BaseURL)
	assert.Equal(t, "http://host.docker.internal:5556/oh-gateway", f.svc.loadGatewayURL(f.ad.started[0].Key.String()))
}

func TestStopSessionTearsDownContainer(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	r, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)
	require.NoError(t, f.svc.StopSession(ctx, r.SessionID))
	assert.Equal(t, 1, f.ad.stopped)
	assert.Equal(t, 1, f.rt.teardowns)
}

func TestResumeKeepsContainerRuntime(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	r, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)
	require.NoError(t, f.svc.Servers.SetStatus(ctx, r.GroupKey, domain.ServerSleeping))
	sess, err := f.svc.Sessions.Get(ctx, r.SessionID)
	require.NoError(t, err)
	sess.State = domain.RunSleeping
	require.NoError(t, f.svc.Sessions.Update(ctx, sess))

	req := f.request("")
	req.Runtime, req.Location = "", ""
	req.Bundle = f.b
	require.NoError(t, f.svc.ResumeSession(ctx, r.SessionID, req))
	require.Len(t, f.ad.started, 2)
	assert.Same(t, f.rt, f.ad.started[1].Runtime, "resume restarts the group in its own runtime")
	assert.Equal(t, r.GroupKey, f.rt.prepares[1].GroupID)
}

func TestContainerRuntimeMissing(t *testing.T) {
	f := newRTFixture(t)
	f.svc.Runtimes = nil
	_, err := f.svc.StartSession(context.Background(), f.request(f.project))
	assert.ErrorContains(t, err, `runtime "container" is not available`)
}

func providerCfg() provider.Config { return provider.Config{AWSRegion: "eu-west-1"} }

func TestStartSessionSetsCheckpointRules(t *testing.T) {
	f := newRTFixture(t)
	f.b.Spec.Workflow = &sessionspec.WorkflowRuntime{ID: "wf", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}}},
		Gates:       []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}}}
	ctx := context.Background()
	res, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)
	require.Len(t, f.ad.created, 1)
	assert.Equal(t, "manuel", f.ad.created[0].Mode, "workflow default mode")
	assert.Equal(t, bundle.SessionRules(f.b.Spec.Workflow, "manuel", nil), f.ad.created[0].SessionRules)
	sess, err := f.svc.Sessions.Get(ctx, res.SessionID)
	require.NoError(t, err)
	assert.Equal(t, "manuel", sess.Mode)
}

// A server that started but whose isolation check fails is not left running:
// process stopped, runtime torn down, grant revoked, row stopped.
func TestStartedServerCleanedUpWhenAttestFails(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	f.ad.attestErr = errors.New("tool API down")
	_, err := f.svc.StartSession(ctx, f.request(f.project))
	require.Error(t, err)
	assert.Equal(t, 1, f.ad.stopped)
	assert.Equal(t, 1, f.rt.teardowns)
	require.NotEmpty(t, f.dc.revoked)
	srvs, err := f.svc.Servers.List(ctx)
	require.NoError(t, err)
	require.Len(t, srvs, 1)
	assert.Equal(t, domain.ServerStopped, srvs[0].Status)
}

func TestResumeUsesProjectDevImageSettings(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	wt := f.worktree(t, "proj-wt")
	r, err := f.svc.StartSession(ctx, f.request(wt))
	require.NoError(t, err)
	require.NoError(t, f.svc.Servers.SetStatus(ctx, r.GroupKey, domain.ServerSleeping))
	sess, err := f.svc.Sessions.Get(ctx, r.SessionID)
	require.NoError(t, err)
	sess.State = domain.RunSleeping
	require.NoError(t, f.svc.Sessions.Update(ctx, sess))

	// The resume request carries the project settings of the moment (the
	// session location is a worktree: the Dockerfile is not looked up there).
	req := f.request("")
	req.Runtime = ""
	req.Dockerfile, req.BuildArgs, req.Volumes = "docker/dev.Dockerfile", map[string]string{"B": "2"}, []string{"node_modules"}
	require.NoError(t, f.svc.ResumeSession(ctx, r.SessionID, req))
	require.Len(t, f.rt.prepares, 2)
	g := f.rt.prepares[1]
	assert.Equal(t, f.project, g.ProjectDir)
	assert.Equal(t, []string{wt}, g.Locations)
	assert.Equal(t, "docker/dev.Dockerfile", g.Dockerfile)
	assert.Equal(t, map[string]string{"B": "2"}, g.BuildArgs)
	assert.Equal(t, []string{"node_modules"}, g.Volumes)
}

func TestIsolateUserConfigReachesTheAdapter(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	req := f.request(f.project)
	req.Runtime = ""
	req.IsolateUserConfig = true
	r, err := f.svc.StartSession(ctx, req)
	require.NoError(t, err)
	require.Len(t, f.ad.started, 1)
	assert.True(t, f.ad.started[0].IsolateUserConfig)

	plain := f.request(f.project)
	plain.Runtime = ""
	cred := provider.ResolvedCredential{Secret: "s"}
	assert.NotEqual(t, configFingerprint(plain, cred, "r"), configFingerprint(req, cred, "r"), "a strict group is not shared")
	assert.NotEmpty(t, r.GroupKey)
}
