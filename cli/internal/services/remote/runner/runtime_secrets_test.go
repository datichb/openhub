package runner

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// Regression test of the leak of 2026-09-29 (the Bedrock key of the
// environment of oh read by an agent with `env`), for the tool server of a
// remote job. The runner and the daemon of the job hold the secrets of the
// job (the project token, CI_JOB_TOKEN…) and, with a job variable gone
// wrong, the credentials of a model: the server gets PATH, the certificates,
// its HOME and what the adapter gives it, nothing else.
func TestJobRuntimeHoldsNoCredentialOfTheJob(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("the server command is a shell command")
	}
	secrets := testenv.SetFakeSecrets(t)
	secrets["CI_JOB_TOKEN"] = "oh-fake-secret/CI_JOB_TOKEN/end"
	secrets["OH_PROJECT_TOKEN"] = "oh-fake-secret/OH_PROJECT_TOKEN/end"
	t.Setenv("CI_JOB_TOKEN", secrets["CI_JOB_TOKEN"])
	t.Setenv("OH_PROJECT_TOKEN", secrets["OH_PROJECT_TOKEN"])
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("SSL_CERT_FILE", "/etc/ssl/job-ca.pem")

	rt := &JobRuntime{Home: t.TempDir()}
	cmd, err := rt.Command(context.Background(), nil, ohProc([]string{"sh", "serve", "--hostname", "0.0.0.0"},
		map[string]string{"TOOL_SERVER_PASSWORD": "pw", "AWS_BEARER_TOKEN_BEDROCK": "ohs_session"}, t.TempDir()))
	require.NoError(t, err)

	assert.Empty(t, testenv.Leaks(secrets, cmd.Env))
	assert.Empty(t, testenv.LeaksIn(secrets, cmd.Path+" "+strings.Join(cmd.Args, " ")))
	assert.Contains(t, cmd.Env, "SSL_CERT_FILE=/etc/ssl/job-ca.pem", "the certificates of the job pass")
	assert.Contains(t, cmd.Env, "LANG=C.UTF-8", "its own language, not the one of the job")
	assert.Contains(t, cmd.Env, "AWS_BEARER_TOKEN_BEDROCK=ohs_session", "the token of the proxy session, not the one of the user")
}
