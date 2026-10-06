package remote

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/ciconfig"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
)

type memSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memSecrets) Get(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}
func (s *memSecrets) Set(_ context.Context, k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memSecrets) Delete(_ context.Context, k string) error { delete(s.m, k); return nil }
func (s *memSecrets) List(context.Context) ([]string, error)   { return nil, nil }

func newSvc(t *testing.T) (*Service, *gitlabtest.Server, *memSecrets, *[]config.RemoteTarget) {
	srv := gitlabtest.New(t, "acme/dev")
	srv.Token = "glpat-member"
	sec := &memSecrets{m: map[string]string{}}
	var saved []config.RemoteTarget
	svc := &Service{Secrets: sec, OhVersion: "5.0.0",
		SaveTarget: func(tg config.RemoteTarget) error { saved = append(saved, tg); return nil }}
	return svc, srv, sec, &saved
}

func target(srv *gitlabtest.Server) config.RemoteTarget {
	return config.RemoteTarget{Name: "acme", URL: srv.URL, Group: "acme/dev"}
}

func status(rep *Report, id string) []StepStatus {
	var out []StepStatus
	for _, s := range rep.Steps {
		if s.ID == id {
			out = append(out, s.Status)
		}
	}
	return out
}

func TestSetupCreatesEverything(t *testing.T) {
	ctx := context.Background()
	svc, srv, sec, saved := newSvc(t)
	api := srv.AddProject("acme/dev/api")

	rep, err := svc.Setup(ctx, SetupRequest{
		Target: target(srv), Token: "glpat-member", Create: true,
		LLM:            &LLMSecret{Provider: "bedrock", Region: "eu-west-1", Key: "bedrock-api-key-123"},
		Projects:       []ProjectToken{{Path: "acme/dev/api", Token: "glpat-project-api"}},
		TeamStateToken: "glpat-teamstate",
	})
	require.NoError(t, err)
	assert.True(t, rep.OK(), "%+v", rep.Steps)

	runner := srv.Project("acme/dev/oh-runner")
	require.NotNil(t, runner)
	assert.Equal(t, []StepStatus{StepCreated}, status(rep, StepProject))
	want, _ := ciconfig.Generate(ciconfig.Options{})
	assert.Equal(t, string(want), string(runner.Files[CIFile]))

	// Secrets: masked + protected in GitLab, stored on the machine, never in hub.toml.
	key := runner.Variables[remote.VarLLMKey]
	assert.True(t, key.Masked && key.Protected && key.Raw)
	assert.Equal(t, "bedrock-api-key-123", key.Value)
	assert.False(t, runner.Variables[remote.VarLLMProvider].Masked)
	pt := runner.Variables[remote.ProjectTokenVar(api.ID)]
	assert.True(t, pt.Masked)
	assert.Equal(t, "glpat-project-api", pt.Value)
	assert.True(t, runner.Variables[remote.VarTeamStateToken].Masked)

	assert.Equal(t, "glpat-member", sec.m["openhub.remote.acme.token"])
	assert.Equal(t, runner.Triggers[0]["token"], sec.m["openhub.remote.acme.trigger"])

	require.Len(t, *saved, 1)
	assert.Equal(t, runner.ID, (*saved)[0].RunnerProjectID)
	assert.Equal(t, "acme/dev/oh-runner", (*saved)[0].RunnerProject)

	// Runners: none online yet → warning, not a failure.
	assert.Equal(t, []StepStatus{StepWarn}, status(rep, StepRunners))
}

func TestSetupIsIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, srv, _, _ := newSvc(t)
	req := SetupRequest{Target: target(srv), Token: "glpat-member", Create: true,
		LLM: &LLMSecret{Provider: "anthropic", Key: "sk-ant-123456"}}
	_, err := svc.Setup(ctx, req)
	require.NoError(t, err)
	runner := srv.Project("acme/dev/oh-runner")
	commits := runner.Commits
	runner.Runners = []map[string]any{{"id": 1, "online": true, "status": "online", "tags": []string{"oh"}}}

	rep, err := svc.Setup(ctx, SetupRequest{Target: target(srv)})
	require.NoError(t, err)
	assert.Equal(t, []StepStatus{StepOK}, status(rep, StepProject))
	assert.Equal(t, []StepStatus{StepOK}, status(rep, StepPipeline))
	assert.Equal(t, []StepStatus{StepOK}, status(rep, StepTrigger))
	assert.Equal(t, []StepStatus{StepOK}, status(rep, StepRunners))
	assert.Equal(t, commits, runner.Commits, "no new commit when the pipeline is up to date")
	assert.Len(t, runner.Triggers, 1, "the stored trigger is reused")

	// Another builder rewrites the generated file.
	tg := target(srv)
	tg.Builder = ciconfig.BuilderDinD
	rep, err = svc.Setup(ctx, SetupRequest{Target: tg})
	require.NoError(t, err)
	assert.Equal(t, []StepStatus{StepUpdated}, status(rep, StepPipeline))
	assert.Contains(t, string(runner.Files[CIFile]), "docker:27-dind")
}

func TestSetupRefusesForeignPipelineAndMissingProject(t *testing.T) {
	ctx := context.Background()
	svc, srv, _, _ := newSvc(t)
	_, err := svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "glpat-member"})
	assert.ErrorIs(t, err, ErrRunnerProjectMissing)

	runner := srv.AddProject("acme/dev/oh-runner")
	runner.Files[CIFile] = []byte("build:\n  script: make\n")
	_, err = svc.Setup(ctx, SetupRequest{Target: target(srv)})
	assert.ErrorIs(t, err, ErrForeignPipeline)
	assert.Equal(t, "build:\n  script: make\n", string(runner.Files[CIFile]))

	_, err = svc.Setup(ctx, SetupRequest{Target: target(srv), Force: true})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(runner.Files[CIFile]), ciconfig.Marker))
}

func TestSetupErrors(t *testing.T) {
	ctx := context.Background()
	svc, srv, _, _ := newSvc(t)
	_, err := svc.Setup(ctx, SetupRequest{Target: target(srv)})
	assert.ErrorIs(t, err, ErrNoToken)

	_, err = svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "bad-token"})
	require.Error(t, err)

	_, err = svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "glpat-member", LLM: &LLMSecret{Provider: "x", Key: "short"}})
	assert.ErrorIs(t, err, ErrWeakSecret)

	_, err = svc.Setup(ctx, SetupRequest{Target: config.RemoteTarget{Name: "x", URL: "nope", Group: "g"}})
	assert.Error(t, err)

	// Registry off → blocking.
	r := srv.AddProject("acme/dev/oh-runner")
	r.Registry = "disabled"
	rep, err := svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "glpat-member"})
	require.Error(t, err)
	assert.Equal(t, []StepStatus{StepFailed}, status(rep, StepFeatures))
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	svc, srv, _, saved := newSvc(t)
	assert.False(t, svc.Check(ctx, target(srv)).OK(), "no token")

	_, err := svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "glpat-member", Create: true})
	require.NoError(t, err)
	tg := (*saved)[0]
	rep := svc.Check(ctx, tg)
	assert.False(t, rep.OK(), "LLM variables missing")
	assert.Contains(t, status(rep, StepVariable), StepFailed)

	runner := srv.Project("acme/dev/oh-runner")
	runner.Variables[remote.VarLLMProvider] = gitlabtest.Variable{Key: remote.VarLLMProvider, Value: "anthropic"}
	runner.Variables[remote.VarLLMKey] = gitlabtest.Variable{Key: remote.VarLLMKey, Value: "sk-ant-xxxxxx", Masked: true}
	rep = svc.Check(ctx, tg)
	assert.True(t, rep.OK(), "%+v", rep.Steps)

	runner.Unprotected = true
	assert.Equal(t, []StepStatus{StepFailed}, status(svc.Check(ctx, tg), StepProtected))
	runner.Unprotected = false

	runner.Files[CIFile] = append([]byte(ciconfig.Marker), []byte(" — old\n")...)
	assert.Equal(t, []StepStatus{StepWarn}, status(svc.Check(ctx, tg), StepPipeline))

	// Development build without uploaded binary.
	svc.OhVersion = "v5.0.0-3-gabc-dirty"
	assert.Equal(t, []StepStatus{StepFailed}, status(svc.Check(ctx, tg), StepBinary))
}

func TestSetupUploadsDevBinary(t *testing.T) {
	ctx := context.Background()
	svc, srv, _, saved := newSvc(t)
	svc.OhVersion = "v5.0.0-3-gabc"
	dir := t.TempDir()
	bin := filepath.Join(dir, "oh")
	require.NoError(t, os.WriteFile(bin, append([]byte{0x7f, 'E', 'L', 'F'}, []byte("payload")...), 0o755))
	notELF := filepath.Join(dir, "oh-darwin")
	require.NoError(t, os.WriteFile(notELF, []byte("\xcf\xfa\xed\xfe"), 0o755))

	_, err := svc.Setup(ctx, SetupRequest{Target: target(srv), Token: "glpat-member", Create: true, OhBinary: notELF})
	require.Error(t, err)

	rep, err := svc.Setup(ctx, SetupRequest{Target: target(srv), OhBinary: bin})
	require.NoError(t, err)
	assert.Equal(t, []StepStatus{StepUpdated}, status(rep, StepBinary))
	tg := (*saved)[len(*saved)-1]
	ver := tg.Binaries[BinaryKey("v5.0.0-3-gabc", "amd64")]
	require.Len(t, ver, 64)
	assert.Contains(t, srv.Project("acme/dev/oh-runner").Generic, "oh-cli/"+ver+"/oh-linux-amd64")
	assert.Equal(t, []StepStatus{StepOK}, status(svc.Check(ctx, tg), StepBinary))
}

func TestIsRelease(t *testing.T) {
	assert.True(t, IsRelease("5.0.0"))
	assert.True(t, IsRelease("v5.0.0"))
	assert.False(t, IsRelease("dev"))
	assert.False(t, IsRelease("v5.0.0-rc.1"))
	assert.False(t, IsRelease("v4.2.0-37-gb890679a-dirty"))
}
