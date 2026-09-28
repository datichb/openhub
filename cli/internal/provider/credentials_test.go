package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- mockSecrets ---

type mockSecrets struct {
	store map[string]string
}

func (m *mockSecrets) Get(_ context.Context, key string) (string, error) {
	return m.store[key], nil
}

// --- ResolveProvider tests ---

func TestResolveProvider(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		project  string
		hub      string
		want     string
	}{
		{"explicit wins", "anthropic", "bedrock", "openrouter", "anthropic"},
		{"project fallback", "", "openrouter", "bedrock", "openrouter"},
		{"hub fallback", "", "", "anthropic", "anthropic"},
		{"default bedrock", "", "", "", "bedrock"},
		{"explicit empty strings", "", "", "", "bedrock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveProvider(tt.explicit, tt.project, tt.hub)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- ResolveProviderConfig tests ---

func TestResolveProviderConfig(t *testing.T) {
	hubCfg := Config{AWSProfile: "hub-profile", AWSRegion: "us-east-1"}

	t.Run("nil project config returns hub defaults", func(t *testing.T) {
		got := ResolveProviderConfig(nil, hubCfg)
		assert.Equal(t, "hub-profile", got.AWSProfile)
		assert.Equal(t, "us-east-1", got.AWSRegion)
	})

	t.Run("project overrides take precedence", func(t *testing.T) {
		projCfg := &ProviderConfig{AWSProfile: "proj-profile", AWSRegion: "eu-west-3"}
		got := ResolveProviderConfig(projCfg, hubCfg)
		assert.Equal(t, "proj-profile", got.AWSProfile)
		assert.Equal(t, "eu-west-3", got.AWSRegion)
	})

	t.Run("partial project override", func(t *testing.T) {
		projCfg := &ProviderConfig{AWSRegion: "eu-west-1"} // only region
		got := ResolveProviderConfig(projCfg, hubCfg)
		assert.Equal(t, "hub-profile", got.AWSProfile) // hub fallback
		assert.Equal(t, "eu-west-1", got.AWSRegion)    // project override
	})

	t.Run("empty project config returns hub defaults", func(t *testing.T) {
		projCfg := &ProviderConfig{}
		got := ResolveProviderConfig(projCfg, hubCfg)
		assert.Equal(t, "hub-profile", got.AWSProfile)
		assert.Equal(t, "us-east-1", got.AWSRegion)
	})
}

// --- ResolveCredentials tests ---

func TestResolveCredentials_Bedrock(t *testing.T) {
	t.Run("project-scoped token takes precedence", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{
			"openhub.provider.bedrock.token.proj-1": "proj-token",
			"openhub.provider.bedrock.token":        "global-token",
		}}
		cfg := &Config{AWSProfile: "my-profile", AWSRegion: "eu-west-1"}
		creds := ResolveCredentials(context.Background(), secrets, Bedrock, "proj-1", cfg)
		assert.Equal(t, "proj-token", creds.BearerToken)
		assert.Equal(t, "my-profile", creds.AWSProfile)
		assert.Equal(t, "eu-west-1", creds.AWSRegion)
	})

	t.Run("falls back to global token", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{
			"openhub.provider.bedrock.token": "global-token",
		}}
		cfg := &Config{AWSProfile: "default", AWSRegion: "us-east-1"}
		creds := ResolveCredentials(context.Background(), secrets, Bedrock, "proj-1", cfg)
		assert.Equal(t, "global-token", creds.BearerToken)
	})

	t.Run("empty token when nothing in keychain", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{}}
		creds := ResolveCredentials(context.Background(), secrets, Bedrock, "", nil)
		assert.Empty(t, creds.BearerToken)
	})

	t.Run("nil config does not panic", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{}}
		creds := ResolveCredentials(context.Background(), secrets, Bedrock, "", nil)
		assert.Empty(t, creds.AWSProfile)
		assert.Empty(t, creds.AWSRegion)
	})
}

func TestResolveCredentials_Anthropic(t *testing.T) {
	t.Run("project-scoped key takes precedence", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{
			"openhub.provider.anthropic.token.proj-2": "proj-key",
			"openhub.provider.anthropic.token":        "global-key",
		}}
		creds := ResolveCredentials(context.Background(), secrets, Anthropic, "proj-2", nil)
		assert.Equal(t, "proj-key", creds.APIKey)
	})

	t.Run("falls back to global key", func(t *testing.T) {
		secrets := &mockSecrets{store: map[string]string{
			"openhub.provider.anthropic.token": "global-key",
		}}
		creds := ResolveCredentials(context.Background(), secrets, Anthropic, "proj-2", nil)
		assert.Equal(t, "global-key", creds.APIKey)
	})
}

func TestResolveCredentials_OpenRouter(t *testing.T) {
	secrets := &mockSecrets{store: map[string]string{
		"openhub.provider.openrouter.token": "or-key",
	}}
	creds := ResolveCredentials(context.Background(), secrets, OpenRouter, "", nil)
	assert.Equal(t, "or-key", creds.APIKey)
}

func TestResolveCredentials_NilSecrets(t *testing.T) {
	creds := ResolveCredentials(context.Background(), nil, Bedrock, "proj-1", nil)
	assert.Empty(t, creds.BearerToken)
	assert.Empty(t, creds.APIKey)
}

func TestResolveCredentials_UnknownProvider(t *testing.T) {
	secrets := &mockSecrets{store: map[string]string{}}
	creds := ResolveCredentials(context.Background(), secrets, GithubCopilot, "", nil)
	assert.Empty(t, creds.BearerToken)
	assert.Empty(t, creds.APIKey)
}
