package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

type fakeGit struct {
	remote, branch, head, fetched string
	ancestor, dirty               bool
	files                         map[string]string
	fetchedBranches               []string
}

func (g *fakeGit) RemoteURL(context.Context, string) (string, error)     { return g.remote, nil }
func (g *fakeGit) CurrentBranch(context.Context, string) (string, error) { return g.branch, nil }
func (g *fakeGit) Head(context.Context, string) (string, error)          { return g.head, nil }
func (g *fakeGit) FetchHead(context.Context, string, string) (string, error) {
	return g.fetched, nil
}
func (g *fakeGit) IsAncestor(context.Context, string, string, string) (bool, error) {
	return g.ancestor, nil
}
func (g *fakeGit) Dirty(context.Context, string) (bool, error) { return g.dirty, nil }
func (g *fakeGit) FetchBranch(_ context.Context, _, b string) error {
	g.fetchedBranches = append(g.fetchedBranches, b)
	return nil
}
func (g *fakeGit) ShowFile(_ context.Context, _, _, p string) ([]byte, error) {
	if c, ok := g.files[p]; ok {
		return []byte(c), nil
	}
	return nil, errors.New("not committed")
}

type fakeBeads struct {
	issues   map[string]map[string]any
	kids     map[string][]string
	claimed  []string
	unclaim  []string
	failOn   string
	execs    []string
	execFail string
	created  int
}

func (b *fakeBeads) rec(id string) json.RawMessage {
	data, _ := json.Marshal(b.issues[id])
	return data
}
func (b *fakeBeads) Show(_ context.Context, _ string, ids ...string) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for _, id := range ids {
		if _, ok := b.issues[id]; !ok {
			return nil, errors.New("no issue " + id)
		}
		out = append(out, b.rec(id))
	}
	return out, nil
}
func (b *fakeBeads) Children(_ context.Context, _, id string) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for _, k := range b.kids[id] {
		out = append(out, b.rec(k))
	}
	return out, nil
}
func (b *fakeBeads) Claim(_ context.Context, _, id string) error {
	if id == b.failOn {
		return errors.New("already claimed by bob")
	}
	b.claimed = append(b.claimed, id)
	b.issues[id]["status"] = "in_progress"
	b.issues[id]["revision"] = "r2-" + id
	return nil
}
func (b *fakeBeads) Exec(_ context.Context, _ string, argv []string, stdin []byte) ([]byte, error) {
	b.execs = append(b.execs, strings.Join(argv, " "))
	if b.execFail != "" && strings.Contains(strings.Join(argv, " "), b.execFail) {
		return nil, errors.New("bd failed")
	}
	if len(argv) > 0 && argv[0] == "create" {
		b.created++
		id := "bd-n" + strconv.Itoa(b.created)
		b.issues[id] = map[string]any{"id": id, "status": "open"}
		return []byte(`{"id":"` + id + `"}`), nil
	}
	return nil, nil
}
func (b *fakeBeads) Unclaim(_ context.Context, _, id string) error {
	b.unclaim = append(b.unclaim, id)
	b.issues[id]["status"] = "open"
	return nil
}

type fakeTeam struct {
	claims   map[string]string
	released []string
	remote   map[string]teamstate.ClaimRemote
}

func (f *fakeTeam) Claim(_ context.Context, _, tk, member string) (bool, error) {
	if who, ok := f.claims[tk]; ok {
		if who != member {
			return false, ErrTicketTaken
		}
		return false, nil
	}
	f.claims[tk] = member
	return true, nil
}
func (f *fakeTeam) Release(_ context.Context, _, tk string) error {
	f.released = append(f.released, tk)
	delete(f.claims, tk)
	return nil
}
func (f *fakeTeam) SetRemote(_ context.Context, _, tk string, r teamstate.ClaimRemote) error {
	f.remote[tk] = r
	return nil
}

type sendEnv struct {
	svc    *Service
	srv    *gitlabtest.Server
	git    *fakeGit
	beads  *fakeBeads
	team   *fakeTeam
	target config.RemoteTarget
	req    SendRequest
	store  *sqlite.Store
}

const (
	secretAPI     = "glpat-member-SECRET"
	secretLLM     = "bedrock-api-key-SECRET"
	secretProject = "glpat-project-SECRET"
)

func newSendEnv(t *testing.T) *sendEnv {
	t.Helper()
	ctx := context.Background()
	svc, srv, _, saved := newSvc(t)
	srv.Token = secretAPI
	srv.AddProject("acme/dev/api")
	_, err := svc.Setup(ctx, SetupRequest{Target: target(srv), Token: secretAPI, Create: true,
		LLM:      &LLMSecret{Provider: "bedrock", Region: "eu-west-1", Key: secretLLM},
		Projects: []ProjectToken{{Path: "acme/dev/api", Token: secretProject}}})
	require.NoError(t, err)
	tg := (*saved)[len(*saved)-1]

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p-api','api','/work/api')`)
	require.NoError(t, err)

	bdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bdir, "bundle.json"), []byte(`{"hash":"abc123"}`), 0o444))
	b := &bundle.Bundle{Spec: sessionspec.BundleSpec{Hash: "abc123"}, Dir: bdir}

	g := &fakeGit{remote: "ssh://git@127.0.0.1/acme/dev/api.git", branch: "main",
		head: "c1", fetched: "c2", ancestor: true, files: map[string]string{}}
	bd := &fakeBeads{issues: map[string]map[string]any{
		"bd-42": {"id": "bd-42", "status": "open", "revision": "r1", "dependencies": []map[string]any{{"id": "bd-7", "dependency_type": "blocks"}}},
		"bd-7":  {"id": "bd-7", "status": "closed", "revision": "r7"},
		"bd-43": {"id": "bd-43", "status": "open", "updated_at": "2026-10-06T10:00:00Z"},
	}, kids: map[string][]string{"bd-42": {"bd-43"}}}
	team := &fakeTeam{claims: map[string]string{}, remote: map[string]teamstate.ClaimRemote{}}
	svc.Sessions, svc.Remote = sqlite.NewSessionStore(st), sqlite.NewRemoteStore(st)
	svc.Git, svc.Beads, svc.ToolVersion = g, bd, "2.0.20"
	svc.NewSessionID = func() string { return "ses_remote1" }
	svc.Now = func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) }
	member := "alice"
	return &sendEnv{svc: svc, srv: srv, git: g, beads: bd, team: team, target: tg, store: st, req: SendRequest{
		Target: tg, ProjectID: "p-api", Project: "api", ProjectDir: t.TempDir(), MemberID: &member,
		Bundle: b, Workflow: remote.ManifestWorkflow{ID: "ticket", Layer: "hub", Version: 3, Risk: "write"},
		Mode: "semi-auto", Title: "api · ticket · bd-42", EntryAgent: "orchestrator-dev", Prompt: "Implement bd-42",
		Inputs: map[string]any{"ticket": "bd-42"}, Branch: "feat/bd-42",
		Checkpoints: []remote.ManifestCheckpoint{{ID: "cp-2", Policy: "defer", Mandatory: true}},
		Provider:    "bedrock", BeadsAllow: []string{"show", "update"}, Tickets: []string{"bd-42"},
		Team: team, TeamInfo: &remote.ManifestTeam{ID: "core", Repo: "https://gitlab/acme/team-state.git", MemberID: member},
	}}
}

func unpack(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	files, err := remote.UnpackFiles(bytes.NewReader(data))
	require.NoError(t, err)
	return files
}

func TestSend(t *testing.T) {
	ctx := context.Background()
	e := newSendEnv(t)
	res, err := e.svc.Send(ctx, e.req)
	require.NoError(t, err)
	assert.Equal(t, "ses_remote1", res.SessionID)
	assert.Contains(t, res.Warnings, WarnImageBuild, "image not in the registry yet")

	runner := e.srv.Project("acme/dev/oh-runner")
	require.Len(t, runner.Pipelines, 1)
	pl := runner.Pipelines[0]
	assert.Equal(t, "main", pl.Ref)
	v := pl.Variables
	api := e.srv.Project("acme/dev/api")
	assert.Equal(t, "ses_remote1", v[remote.VarSessionID])
	assert.Equal(t, fmtID(api.ID), v[remote.VarProjectID])
	assert.Equal(t, "acme/dev/api", v[remote.VarProjectPath])
	assert.Equal(t, "main", v[remote.VarRef])
	assert.Equal(t, "c2", v[remote.VarCommit])
	assert.Equal(t, "true", v[remote.VarImageBuild])
	assert.Equal(t, "5.0.0", v[remote.VarCLIVersion])
	assert.Equal(t, "2.0.20", v[remote.VarToolVersion])
	assert.Equal(t, "", v[remote.VarDockerfile], "no dev Dockerfile: oh default base")
	assert.True(t, strings.HasPrefix(v[remote.VarImage], "registry.example.com/acme/dev/oh-runner/acme-dev-api:"))
	assert.True(t, strings.HasPrefix(v[remote.VarImageBase], "registry.example.com/acme/dev/oh-runner/acme-dev-api-base:"))

	// Packages: the bundle by its hash, the envelope by its content hash.
	bundleTar := runner.Generic["oh-bundle/abc123/bundle.tar.gz"]
	require.NotNil(t, bundleTar)
	assert.Contains(t, unpack(t, bundleTar), "bundle.json")
	var envKey string
	for k := range runner.Generic {
		if strings.HasPrefix(k, "oh-session/") {
			envKey = k
		}
	}
	require.NotEmpty(t, envKey)
	env := runner.Generic[envKey]
	assert.Equal(t, "oh-session/"+remote.SHA256(env)+"/session.tar.gz", envKey)
	assert.True(t, strings.HasSuffix(v[remote.VarSessionURL], "/packages/generic/"+envKey))
	files := unpack(t, env)
	var man remote.Manifest
	require.NoError(t, json.Unmarshal(files[remote.ManifestFile], &man))
	assert.Equal(t, "ses_remote1", man.SessionID)
	assert.Equal(t, "feat/bd-42", man.Branch)
	assert.Equal(t, "/tmp/oh-work/api", man.WorkDir)
	assert.Equal(t, api.ID, man.Project.ID)
	assert.Equal(t, []string{"show", "update"}, man.BeadsAllow)
	assert.Equal(t, "defer", man.Checkpoints[0].Policy)

	// Snapshot: ticket (claimed state), its dependency and its child.
	var snap remote.Snapshot
	require.NoError(t, json.Unmarshal(files[remote.SnapshotFile], &snap))
	assert.ElementsMatch(t, []string{"bd-42", "bd-7", "bd-43"}, keys(snap.Issues))
	assert.Equal(t, "r2-bd-42", snap.Revisions["bd-42"], "snapshot taken after the claim")
	assert.Equal(t, "2026-10-06T10:00:00Z", snap.Revisions["bd-43"], "no revision: update time")
	assert.Equal(t, []string{"bd-43"}, snap.Children["bd-42"])

	// Reservation and records.
	assert.Equal(t, []string{"bd-42"}, e.beads.claimed)
	assert.Equal(t, "alice", e.team.claims["bd-42"])
	assert.Equal(t, teamstate.RemoteSent, e.team.remote["bd-42"].Status)
	assert.Equal(t, pl.ID, e.team.remote["bd-42"].Pipeline)
	sess, err := sqlite.NewSessionStore(e.store).Get(ctx, "ses_remote1")
	require.NoError(t, err)
	assert.Equal(t, "remote", sess.Runtime)
	assert.Equal(t, domain.RunActive, sess.State)
	assert.Equal(t, "", sess.GroupKey, "no server group: ignored by the daemon")
	ref, err := sqlite.NewRemoteStore(e.store).GetRemoteRef(ctx, "ses_remote1")
	require.NoError(t, err)
	assert.Equal(t, pl.ID, ref.Pipeline)
	assert.Equal(t, domain.RemoteSent, ref.Status)

	// Critère 3: no secret of the machine leaves it (variables, packages).
	for _, secret := range []string{secretAPI, secretLLM, secretProject, "glptt-1-secret"} {
		for k, val := range v {
			assert.NotContains(t, val, secret, k)
		}
		for k, data := range runner.Generic {
			for name, f := range unpack(t, data) {
				assert.NotContains(t, string(f), secret, k+"/"+name)
			}
		}
	}

	// Same bundle again: not uploaded twice; image now in the registry.
	runner.Images["acme-dev-api"] = []string{strings.SplitN(v[remote.VarImage], ":", 2)[1]}
	e.svc.NewSessionID = func() string { return "ses_remote2" }
	before := countPuts(e.srv, "/packages/generic/oh-bundle/")
	res, err = e.svc.Send(ctx, e.req)
	require.NoError(t, err)
	assert.NotContains(t, res.Warnings, WarnImageBuild)
	assert.Equal(t, before, countPuts(e.srv, "/packages/generic/oh-bundle/"))
	assert.Empty(t, runner.Pipelines[1].Variables[remote.VarImageBuild])
}

func TestSendRefusals(t *testing.T) {
	ctx := context.Background()

	e := newSendEnv(t)
	e.git.ancestor = false
	_, err := e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrUnpushed)
	assert.Empty(t, e.beads.claimed, "nothing reserved")

	e = newSendEnv(t)
	e.git.branch = ""
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrDetached)

	e = newSendEnv(t)
	e.git.remote = "git@gitlab.other.example:acme/dev/api.git"
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrForeignRemote)

	e = newSendEnv(t)
	e.svc.OhVersion = "v5.0.0-3-gabc"
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrNoBinary)

	e = newSendEnv(t)
	e.srv.Project("acme/dev/oh-runner").Files[CIFile] = []byte("# Generated by oh (oh remote setup) — pipeline schema 0.\n")
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrPipelineOutdated)

	// Dev Dockerfile present but not committed.
	e = newSendEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(e.req.ProjectDir, "Dockerfile.dev"), []byte("FROM x"), 0o644))
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrDockerfileNotCommitted)
	e.git.files["Dockerfile.dev"] = "FROM x"
	res, err := e.svc.Send(ctx, e.req)
	require.NoError(t, err)
	assert.Equal(t, "Dockerfile.dev", e.srv.Project("acme/dev/oh-runner").Pipelines[0].Variables[remote.VarDockerfile])
	assert.Equal(t, "ses_remote1", res.SessionID)

	// Ticket held by another member in the team-state: Beads claim undone.
	e = newSendEnv(t)
	e.team.claims["bd-42"] = "bob"
	_, err = e.svc.Send(ctx, e.req)
	assert.ErrorIs(t, err, ErrTicketTaken)
	assert.Equal(t, []string{"bd-42"}, e.beads.unclaim)
	assert.Equal(t, "open", e.beads.issues["bd-42"]["status"])
}

func TestSendUndoesReservationWhenTriggerFails(t *testing.T) {
	ctx := context.Background()
	e := newSendEnv(t)
	e.srv.Fail["POST /projects/"+fmtID(e.srv.Project("acme/dev/oh-runner").ID)+"/trigger"] = 500
	_, err := e.svc.Send(ctx, e.req)
	require.Error(t, err)
	assert.Equal(t, []string{"bd-42"}, e.beads.unclaim)
	assert.Equal(t, []string{"bd-42"}, e.team.released)
	_, err = sqlite.NewSessionStore(e.store).Get(ctx, "ses_remote1")
	assert.Error(t, err, "no session recorded")

	// A ticket already in progress (held before) is not unclaimed.
	e = newSendEnv(t)
	e.beads.issues["bd-42"]["status"] = "in_progress"
	e.team.claims["bd-42"] = "alice"
	e.srv.Fail["POST /projects/"+fmtID(e.srv.Project("acme/dev/oh-runner").ID)+"/trigger"] = 500
	_, err = e.svc.Send(ctx, e.req)
	require.Error(t, err)
	assert.Empty(t, e.beads.unclaim)
	assert.Empty(t, e.team.released)
}

func fmtID(id int64) string { return strconv.FormatInt(id, 10) }

func keys(m map[string]json.RawMessage) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func countPuts(s *gitlabtest.Server, part string) int {
	n := 0
	for _, r := range s.Requests {
		if strings.HasPrefix(r, "PUT ") && strings.Contains(r, part) {
			n++
		}
	}
	return n
}
