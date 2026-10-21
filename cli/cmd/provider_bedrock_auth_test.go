package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func clearAWSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_BEARER_TOKEN_BEDROCK", "AWS_PROFILE", "OH_DOCTOR_OFFLINE"} {
		t.Setenv(k, "")
	}
}

// auth_mode = bearer without a stored key refuses the launch clearly (the
// TUI shows « Configure the provider »), and proposes the shell key.
func TestLaunchBearerModeWithoutStoredKey(t *testing.T) {
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	ctx := context.Background()
	a := newMockApp(nil, &config.ProviderConfigs{Bedrock: config.ProviderConfig{AuthMode: "bearer", AWSProfile: "dev"}})
	a.Config.LLM.DefaultProvider = "bedrock"
	project := &domain.Project{ID: "p1", Name: "demo"}

	detail, ok := credentialStatus(ctx, a, project, "")
	assert.False(t, ok, "bearer mode never falls back to the AWS profile")
	assert.Contains(t, detail, "auth_mode is bearer")
	assert.NotContains(t, detail, "AWS_BEARER_TOKEN_BEDROCK")

	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "ABSK-shell")
	err := launchCredentials(ctx, a, project, "")
	var pe *ProviderCredentialError
	require.ErrorAs(t, err, &pe)
	assert.Contains(t, err.Error(), "oh provider setup bedrock --from-env -p demo")
	assert.ErrorIs(t, tuiLaunchError(fmt.Errorf("prepare: %w", err)), views.ErrProviderCredentials)
	assert.True(t, resolveCredentialReport(ctx, a, project, "").envBearerUnused)
}

// The shell API key is never reported as a usable credential: oh does not
// pass it to the tool.
func TestDoctorDoesNotReportTheShellKeyAsSigV4(t *testing.T) {
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "ABSK-shell")
	a := newMockApp(nil, nil)
	a.Config.LLM.DefaultProvider = "bedrock"
	detail, ok := credentialStatus(context.Background(), a, nil, "")
	assert.False(t, ok, detail)
	assert.Contains(t, detail, "oh does not use it")
	assert.Contains(t, detail, "--from-env")

	// A stored key differing from the shell one is used, with a warning.
	a = newMockApp(map[string]string{provider.KeychainKey(provider.Bedrock, ""): "stored"}, nil)
	a.Config.LLM.DefaultProvider = "bedrock"
	r := resolveCredentialReport(context.Background(), a, nil, "")
	assert.True(t, r.ok)
	assert.True(t, r.envBearerDiffers)
}

// auth_mode = profile ignores a stored key.
func TestProfileModeIgnoresStoredKey(t *testing.T) {
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	a := newMockApp(map[string]string{provider.KeychainKey(provider.Bedrock, ""): "stored"},
		&config.ProviderConfigs{Bedrock: config.ProviderConfig{AuthMode: "profile", AWSProfile: "dev"}})
	a.Config.LLM.DefaultProvider = "bedrock"
	detail, ok := credentialStatus(context.Background(), a, nil, "")
	assert.True(t, ok)
	assert.Contains(t, detail, "AWS profile dev")
}

// oh doctor validates a stored Bedrock key with AWS (fake endpoint here).
func TestDoctorValidatesBedrockKey(t *testing.T) {
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"Authentication failed: Please make sure your API Key is valid."}`)
			return
		}
		_, _ = io.WriteString(w, `{"modelSummaries":[]}`)
	}))
	t.Cleanup(srv.Close)
	prev := bedrockProbeURL
	var region string
	bedrockProbeURL = func(r string) (string, error) { region = r; return srv.URL, nil }
	t.Cleanup(func() { bedrockProbeURL = prev })

	ctx := context.Background()
	key := provider.KeychainKey(provider.Bedrock, "p1")
	project := &domain.Project{ID: "p1", Name: "demo"}
	cfg := &config.ProviderConfigs{Bedrock: config.ProviderConfig{AWSRegion: "eu-west-1", AuthMode: "bearer"}}

	a := newMockApp(map[string]string{key: "good"}, cfg)
	a.Config.LLM.DefaultProvider = "bedrock"
	res := providerCredentialCheck(ctx, a, project)
	assert.True(t, res.OK, res.Detail)
	assert.False(t, res.Warn, res.Detail)
	assert.Contains(t, res.Detail, key)
	assert.Contains(t, res.Detail, "accepted by AWS")
	assert.Equal(t, "eu-west-1", region)

	a = newMockApp(map[string]string{key: "old"}, cfg)
	a.Config.LLM.DefaultProvider = "bedrock"
	res = providerCredentialCheck(ctx, a, project)
	assert.False(t, res.OK, res.Detail)
	assert.Contains(t, res.Detail, "rejected by AWS (expired?)")
	assert.Contains(t, res.Detail, "oh provider setup bedrock -p demo")

	t.Setenv("OH_DOCTOR_OFFLINE", "1")
	res = providerCredentialCheck(ctx, a, project)
	assert.True(t, res.OK, "offline: not validated")
	assert.Contains(t, res.Detail, "offline")

	t.Setenv("OH_DOCTOR_OFFLINE", "")
	bedrockProbeURL = func(string) (string, error) { return "http://127.0.0.1:1", nil }
	res = providerCredentialCheck(ctx, a, project)
	assert.True(t, res.OK, "unreachable: a warning only")
	assert.True(t, res.Warn)
}

// The shell key is imported under the key a launch uses (here the stale
// project key), with auth_mode = bearer.
func TestImportBedrockEnvToken(t *testing.T) {
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	t.Setenv("OH_HOME", t.TempDir())
	ctx := context.Background()
	projectKey := provider.KeychainKey(provider.Bedrock, "p1")
	a := newMockApp(map[string]string{projectKey: "stale"}, nil)
	project := domain.Project{ID: "p1", Name: "demo"}
	a.Projects = &mockProjectStore{projects: []domain.Project{project}}

	_, err := importBedrockEnvToken(ctx, a, &project, false)
	assert.Error(t, err, "no shell key")

	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "ABSK-fresh")
	key, err := importBedrockEnvToken(ctx, a, &project, false)
	require.NoError(t, err)
	assert.Equal(t, projectKey, key)
	got, _ := a.Secrets.Get(ctx, projectKey)
	assert.Equal(t, "ABSK-fresh", got)
	require.NotNil(t, project.ProviderConfig)
	assert.Equal(t, "bearer", project.ProviderConfig.AuthMode)

	// Without a stored key: the hub key, hub auth_mode = bearer.
	a = newMockApp(nil, nil)
	key, err = importBedrockEnvToken(ctx, a, nil, false)
	require.NoError(t, err)
	assert.Equal(t, provider.KeychainKey(provider.Bedrock, ""), key)
	assert.Equal(t, "bearer", a.Config.Provider.Bedrock.AuthMode)
	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "bearer", saved.Provider.Bedrock.AuthMode)

	// Explicit project: its own key.
	other := domain.Project{ID: "p2", Name: "two"}
	a.Projects = &mockProjectStore{projects: []domain.Project{other}}
	key, err = importBedrockEnvToken(ctx, a, &other, true)
	require.NoError(t, err)
	assert.Equal(t, provider.KeychainKey(provider.Bedrock, "p2"), key)
	assert.Equal(t, key, other.ProviderConfig.TokenKey)
}
