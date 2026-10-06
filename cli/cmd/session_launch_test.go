package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/provider"
)

// ─────────────────────────────────────────────────────────────────────────────
// Tests for resolveCredentials
// ─────────────────────────────────────────────────────────────────────────────

// mockSecretStore is a minimal in-memory secret store for testing.
type mockSecretStore struct {
	secrets map[string]string
}

func (m *mockSecretStore) Get(_ context.Context, key string) (string, error) {
	return m.secrets[key], nil
}

func (m *mockSecretStore) Set(_ context.Context, key, val string) error {
	m.secrets[key] = val
	return nil
}

func (m *mockSecretStore) Delete(_ context.Context, key string) error {
	delete(m.secrets, key)
	return nil
}

func (m *mockSecretStore) List(_ context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func newMockApp(secrets map[string]string, provCfg *config.ProviderConfigs) *app.App {
	sc := &mockSecretStore{secrets: secrets}
	if secrets == nil {
		sc.secrets = make(map[string]string)
	}
	cfg := &config.Config{}
	if provCfg != nil {
		cfg.Provider = *provCfg
	}
	return &app.App{
		Config:  cfg,
		Secrets: sc,
	}
}

func TestResolveCredentials_BedrockWithProjectToken(t *testing.T) {
	projectID := "proj-123"
	projectKey := provider.KeychainKey(provider.Bedrock, projectID)
	require.Equal(t, "openhub.provider.bedrock.token.proj-123", projectKey)

	a := newMockApp(map[string]string{
		projectKey: "my-project-token",
	}, nil)

	project := &domain.Project{ID: projectID}
	bearer, apiKey, _, _ := resolveCredentials(context.Background(), a, project, "bedrock")
	assert.Equal(t, "my-project-token", bearer)
	assert.Empty(t, apiKey)
}

func TestResolveCredentials_BedrockFallbackToDefault(t *testing.T) {
	defaultKey := provider.KeychainKey(provider.Bedrock, "")
	require.Equal(t, "openhub.provider.bedrock.token", defaultKey)

	a := newMockApp(map[string]string{
		defaultKey: "my-default-token",
	}, nil)

	project := &domain.Project{ID: "proj-456"}
	bearer, _, _, _ := resolveCredentials(context.Background(), a, project, "bedrock")
	assert.Equal(t, "my-default-token", bearer)
}

func TestResolveCredentials_BedrockAWSProfile(t *testing.T) {
	a := newMockApp(nil, &config.ProviderConfigs{
		Bedrock: config.ProviderConfig{
			AWSProfile: "my-profile",
			AWSRegion:  "eu-west-1",
		},
	})
	project := &domain.Project{ID: "p1"}
	_, _, profile, region := resolveCredentials(context.Background(), a, project, "bedrock")
	assert.Equal(t, "my-profile", profile)
	assert.Equal(t, "eu-west-1", region)
}

func TestResolveCredentials_BedrockProjectAWSOverride(t *testing.T) {
	a := newMockApp(nil, &config.ProviderConfigs{
		Bedrock: config.ProviderConfig{
			AWSProfile: "hub-profile",
			AWSRegion:  "us-east-1",
		},
	})
	project := &domain.Project{
		ID: "p1",
		ProviderConfig: &domain.ProjectProviderConfig{
			AWSProfile: "project-profile",
			AWSRegion:  "ap-southeast-1",
		},
	}
	_, _, profile, region := resolveCredentials(context.Background(), a, project, "bedrock")
	assert.Equal(t, "project-profile", profile)
	assert.Equal(t, "ap-southeast-1", region)
}

func TestResolveCredentials_AnthropicWithProjectKey(t *testing.T) {
	projectID := "proj-789"
	projectKey := provider.KeychainKey(provider.Anthropic, projectID)

	a := newMockApp(map[string]string{
		projectKey: "sk-ant-project",
	}, nil)

	project := &domain.Project{ID: projectID}
	_, apiKey, _, _ := resolveCredentials(context.Background(), a, project, "anthropic")
	assert.Equal(t, "sk-ant-project", apiKey)
}

func TestResolveCredentials_AnthropicFallbackToDefault(t *testing.T) {
	defaultKey := provider.KeychainKey(provider.Anthropic, "")

	a := newMockApp(map[string]string{
		defaultKey: "sk-ant-default",
	}, nil)

	project := &domain.Project{ID: "proj-000"}
	_, apiKey, _, _ := resolveCredentials(context.Background(), a, project, "anthropic")
	assert.Equal(t, "sk-ant-default", apiKey)
}

func TestResolveCredentials_OpenRouter(t *testing.T) {
	defaultKey := provider.KeychainKey(provider.OpenRouter, "")

	a := newMockApp(map[string]string{
		defaultKey: "sk-or-default",
	}, nil)

	project := &domain.Project{ID: "proj-111"}
	_, apiKey, _, _ := resolveCredentials(context.Background(), a, project, "openrouter")
	assert.Equal(t, "sk-or-default", apiKey)
}

func TestResolveCredentials_UnknownProvider(t *testing.T) {
	a := newMockApp(nil, nil)
	project := &domain.Project{ID: "p"}
	bearer, apiKey, profile, region := resolveCredentials(context.Background(), a, project, "unknown")
	assert.Empty(t, bearer)
	assert.Empty(t, apiKey)
	assert.Empty(t, profile)
	assert.Empty(t, region)
}

func TestResolveCredentials_NoSecrets(t *testing.T) {
	a := newMockApp(map[string]string{}, nil)
	project := &domain.Project{ID: "p"}
	bearer, apiKey, _, _ := resolveCredentials(context.Background(), a, project, "bedrock")
	assert.Empty(t, bearer)
	assert.Empty(t, apiKey)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests for credential key format consistency
// ─────────────────────────────────────────────────────────────────────────────

func TestCredentialKeysUseCanonicalFormat(t *testing.T) {
	// Ensure resolveCredentials uses the same key format as provider.KeychainKey
	// This is the core invariant that Phase 0a fixed.
	tests := []struct {
		prov      string
		projectID string
		wantKey   string
	}{
		{"bedrock", "proj-1", "openhub.provider.bedrock.token.proj-1"},
		{"bedrock", "", "openhub.provider.bedrock.token"},
		{"anthropic", "proj-2", "openhub.provider.anthropic.token.proj-2"},
		{"anthropic", "", "openhub.provider.anthropic.token"},
		{"openrouter", "proj-3", "openhub.provider.openrouter.token.proj-3"},
		{"openrouter", "", "openhub.provider.openrouter.token"},
	}

	for _, tc := range tests {
		t.Run(tc.prov+"/"+tc.projectID, func(t *testing.T) {
			a := newMockApp(map[string]string{tc.wantKey: "test-value"}, nil)
			project := &domain.Project{ID: tc.projectID}

			bearer, apiKey, _, _ := resolveCredentials(context.Background(), a, project, tc.prov)
			got := bearer
			if got == "" {
				got = apiKey
			}
			assert.Equal(t, "test-value", got,
				"resolveCredentials should read key %q for provider=%s projectID=%s",
				tc.wantKey, tc.prov, tc.projectID)
		})
	}
}
