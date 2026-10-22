package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
)

// setupEnv is a hub (temporary home, remote mode enabled, in-memory secrets)
// facing a fake GitLab that knows the group acme/dev. The terminal is
// absent: the secrets come from the environment.
type setupEnv struct {
	srv     *gitlabtest.Server
	secrets *mockSecretStore
}

func newSetupEnv(t *testing.T) setupEnv {
	t.Helper()
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a := withRemoteApp(t, true)
	a.Config.Remote.Targets = nil
	srv := gitlabtest.New(t, "acme/dev")
	srv.Token = "glpat-member"
	t.Setenv("OH_TEST_GITLAB_TOKEN", "glpat-member")
	t.Setenv("OH_TEST_LLM_KEY", "bedrock-api-key-123")
	return setupEnv{srv: srv, secrets: a.Secrets.(*mockSecretStore)}
}

// linuxBinary is a file the setup accepts as the Linux oh to upload: a
// development build has no released binary for the jobs to download.
func linuxBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "oh-linux")
	require.NoError(t, os.WriteFile(bin, append([]byte{0x7f, 'E', 'L', 'F'}, []byte("payload")...), 0o755))
	return bin
}

// complete are the options of a setup with nothing left to fix: the LLM key
// of the jobs and the binary (a development build).
func complete(t *testing.T, e setupEnv, extra ...string) []string {
	t.Helper()
	return append([]string{"--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_GITLAB_TOKEN",
		"--llm-key-env", "OH_TEST_LLM_KEY", "--oh-binary", linuxBinary(t)}, extra...)
}

// runSetup runs `oh remote setup <args>` and returns what it printed.
func runSetup(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := remoteSetupCmd()
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetContext(t.Context())
	require.NoError(t, c.ParseFlags(args))
	err := runRemoteSetup(c, nil)
	return out.String(), err
}

func savedTargets(t *testing.T) []config.RemoteTarget {
	t.Helper()
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	return cfg.Remote.Targets
}

// The first setup creates oh-runner on the instance, stores the token and the
// trigger on the machine, and saves the target in hub.toml with the id of the
// runner project.
func TestRemoteSetupCreatesTheRunnerAndSavesTheTarget(t *testing.T) {
	e := newSetupEnv(t)
	out, err := runSetup(t, complete(t, e)...)
	require.NoError(t, err, out)

	runner := e.srv.Project("acme/dev/oh-runner")
	require.NotNil(t, runner, "oh-runner is created in the group")
	assert.Contains(t, string(runner.Files[".gitlab-ci.yml"]), "oh", "the generated pipeline is committed")
	assert.Contains(t, out, "acme/dev/oh-runner")
	assert.Contains(t, out, "Target acme-dev ready")

	assert.Equal(t, "glpat-member", e.secrets.secrets["openhub.remote.acme-dev.token"])
	require.Len(t, runner.Triggers, 1)
	assert.Equal(t, runner.Triggers[0]["token"], e.secrets.secrets["openhub.remote.acme-dev.trigger"])

	targets := savedTargets(t)
	require.Len(t, targets, 1)
	assert.Equal(t, "acme-dev", targets[0].Name, "named after the group")
	assert.Equal(t, e.srv.URL, targets[0].URL)
	assert.Equal(t, "acme/dev", targets[0].Group)
	assert.Equal(t, runner.ID, targets[0].RunnerProjectID)
	assert.Equal(t, "acme/dev/oh-runner", targets[0].RunnerProject)
}

// What is still missing (here the LLM key and the binary of a development
// build) is listed with the command that fixes it, the command fails, and the
// target is saved all the same so that the next run completes it.
func TestRemoteSetupReportsWhatIsMissingAndSavesTheTarget(t *testing.T) {
	e := newSetupEnv(t)
	out, err := runSetup(t, "--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_GITLAB_TOKEN")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setup incomplete")
	assert.Contains(t, out, "OH_LLM_KEY (missing)")
	assert.Contains(t, out, "oh remote setup --llm")
	require.NotNil(t, e.srv.Project("acme/dev/oh-runner"), "what could be done is done")
	assert.Len(t, savedTargets(t), 1)
}

// A second run needs no flag: the saved target gives the instance and the
// group, the stored token the access; nothing is created twice.
func TestRemoteSetupAgainReusesTheSavedTarget(t *testing.T) {
	e := newSetupEnv(t)
	_, err := runSetup(t, complete(t, e)...)
	require.NoError(t, err)
	runner := e.srv.Project("acme/dev/oh-runner")
	commits := runner.Commits

	a := MustApp()
	a.Config.Remote.Targets = savedTargets(t)
	out, err := runSetup(t)
	require.NoError(t, err, out)
	assert.Equal(t, commits, runner.Commits, "the pipeline is up to date: no new commit")
	assert.Len(t, runner.Triggers, 1, "the stored trigger is reused")
	assert.Len(t, savedTargets(t), 1)
}

// --no-create stops where the runner project is missing, and saves nothing.
func TestRemoteSetupNoCreateStopsWithoutRunnerProject(t *testing.T) {
	e := newSetupEnv(t)
	out, err := runSetup(t, "--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_GITLAB_TOKEN", "--no-create")
	require.Error(t, err)
	assert.Nil(t, e.srv.Project("acme/dev/oh-runner"))
	assert.Contains(t, out, "acme/dev/oh-runner", "the report names the missing project")
	assert.Empty(t, savedTargets(t))
}

// A token GitLab refuses is reported and nothing is saved.
func TestRemoteSetupWithARefusedToken(t *testing.T) {
	e := newSetupEnv(t)
	t.Setenv("OH_TEST_GITLAB_TOKEN", "glpat-wrong")
	_, err := runSetup(t, "--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_GITLAB_TOKEN")
	require.Error(t, err)
	assert.Nil(t, e.srv.Project("acme/dev/oh-runner"))
	assert.Empty(t, savedTargets(t))
}

// The LLM key and the project tokens go to masked, protected CI variables
// scoped to the run job, never to the image build.
func TestRemoteSetupStoresTheSecretsAsMaskedVariables(t *testing.T) {
	e := newSetupEnv(t)
	api := e.srv.AddProject("acme/dev/api")
	t.Setenv("OH_TEST_PROJECT_TOKEN", "glpat-project-api")

	out, err := runSetup(t, complete(t, e, "--llm-region", "eu-west-3",
		"--project", "acme/dev/api", "--project-token-env", "OH_TEST_PROJECT_TOKEN")...)
	require.NoError(t, err, out)

	runner := e.srv.Project("acme/dev/oh-runner")
	key := runner.Variables[gitlabtest.VarKey(remote.VarLLMKey, remote.RunEnvironmentScope)]
	assert.Equal(t, "bedrock-api-key-123", key.Value)
	assert.True(t, key.Masked && key.Protected, "the LLM key is masked and protected")
	assert.Equal(t, "bedrock", runner.Variables[remote.VarLLMProvider].Value, "Bedrock is the default provider")
	pt := runner.Variables[gitlabtest.VarKey(remote.ProjectTokenVar(api.ID), remote.RunEnvironment(api.ID))]
	assert.Equal(t, "glpat-project-api", pt.Value)
	assert.True(t, pt.Masked)
	assert.NotContains(t, out, "bedrock-api-key-123", "a secret is never printed")
	assert.NotContains(t, out, "glpat-project-api")
}

// Without a terminal a secret cannot be typed: the error names the option
// that gives it, and nothing is sent to GitLab.
func TestRemoteSetupWithoutTerminalNeedsTheTokenFromTheEnvironment(t *testing.T) {
	e := newSetupEnv(t)
	_, err := runSetup(t, "--url", e.srv.URL, "--group", "acme/dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--token-env")
	assert.Nil(t, e.srv.Project("acme/dev/oh-runner"))

	t.Setenv("OH_TEST_EMPTY", "  ")
	_, err = runSetup(t, "--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_EMPTY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OH_TEST_EMPTY")
	assert.Contains(t, err.Error(), "empty")
}

// The options are checked before anything is created.
func TestRemoteSetupRefusesInconsistentOptions(t *testing.T) {
	e := newSetupEnv(t)
	base := []string{"--url", e.srv.URL, "--group", "acme/dev", "--token-env", "OH_TEST_GITLAB_TOKEN"}
	t.Setenv("OH_TEST_LLM_KEY", "sk-ant-123456")
	t.Setenv("OH_TEST_PROJECT_TOKEN", "glpat-project-api")
	for name, extra := range map[string][]string{
		"unknown LLM provider":                {"--llm-provider", "vertex", "--llm-key-env", "OH_TEST_LLM_KEY"},
		"one token for several projects":      {"--project", "acme/dev/a", "--project", "acme/dev/b", "--project-token-env", "OH_TEST_PROJECT_TOKEN"},
		"unknown builder":                     {"--builder", "buildah"},
		"a weak LLM key":                      {"--llm-key-env", "OH_TEST_WEAK"},
		"an LLM key variable that is not set": {"--llm-key-env", "OH_TEST_NOT_SET"},
	} {
		t.Setenv("OH_TEST_WEAK", "short")
		_, err := runSetup(t, append(append([]string{}, base...), extra...)...)
		assert.Error(t, err, name)
		assert.Nil(t, e.srv.Project("acme/dev/oh-runner"), "%s: nothing created", name)
	}
	assert.Empty(t, savedTargets(t))
}
