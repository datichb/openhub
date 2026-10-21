package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/provider"
)

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

func TestHubProviderCfg(t *testing.T) {
	a := newMockApp(nil, &config.ProviderConfigs{
		Bedrock: config.ProviderConfig{AWSProfile: "my-profile", AWSRegion: "eu-west-1", AuthMode: provider.AuthProfile},
	})

	assert.Equal(t, provider.Config{AWSProfile: "my-profile", AWSRegion: "eu-west-1", AuthMode: provider.AuthProfile},
		hubProviderCfg(a, "bedrock"), "Bedrock takes the profile, the region and the auth mode of the hub")
	for _, prov := range []string{"anthropic", "openrouter", "unknown"} {
		assert.Equal(t, provider.Config{}, hubProviderCfg(a, prov), "%s has no hub settings", prov)
	}
}
