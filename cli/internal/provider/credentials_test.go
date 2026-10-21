package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/platform"
)

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

// The keys read are the canonical ones (KeychainKey), literal here so a change
// of format is a visible decision.
func TestResolveCredentials(t *testing.T) {
	hub := &Config{AWSProfile: "hub-profile", AWSRegion: "eu-west-1"}
	tests := []struct {
		name      string
		secrets   mapSecrets
		prov      Name
		projectID string
		cfg       *Config
		want      platform.Credentials
	}{
		{"bedrock: the token of the project wins", mapSecrets{
			"openhub.provider.bedrock.token.proj-1": "proj-token",
			"openhub.provider.bedrock.token":        "global-token",
		}, Bedrock, "proj-1", hub, platform.Credentials{BearerToken: "proj-token", AWSProfile: "hub-profile", AWSRegion: "eu-west-1"}},
		{"bedrock: falls back to the global token", mapSecrets{
			"openhub.provider.bedrock.token": "global-token",
		}, Bedrock, "proj-1", nil, platform.Credentials{BearerToken: "global-token"}},
		{"bedrock: the AWS profile and region without a token", mapSecrets{}, Bedrock, "p", hub,
			platform.Credentials{AWSProfile: "hub-profile", AWSRegion: "eu-west-1"}},
		{"bedrock: nothing stored, no config", mapSecrets{}, Bedrock, "", nil, platform.Credentials{}},
		{"anthropic: the key of the project wins", mapSecrets{
			"openhub.provider.anthropic.token.proj-2": "proj-key",
			"openhub.provider.anthropic.token":        "global-key",
		}, Anthropic, "proj-2", nil, platform.Credentials{APIKey: "proj-key"}},
		{"anthropic: falls back to the global key", mapSecrets{
			"openhub.provider.anthropic.token": "global-key",
		}, Anthropic, "proj-2", nil, platform.Credentials{APIKey: "global-key"}},
		{"openrouter: the key of the project wins", mapSecrets{
			"openhub.provider.openrouter.token.proj-3": "proj-key",
			"openhub.provider.openrouter.token":        "global-key",
		}, OpenRouter, "proj-3", nil, platform.Credentials{APIKey: "proj-key"}},
		{"openrouter: falls back to the global key", mapSecrets{
			"openhub.provider.openrouter.token": "global-key",
		}, OpenRouter, "proj-3", nil, platform.Credentials{APIKey: "global-key"}},
		{"an unknown provider reads nothing", mapSecrets{
			"openhub.provider.github-copilot.token": "ghu",
			"openhub.provider.unknown.token":        "x",
		}, Name("unknown"), "p", hub, platform.Credentials{}},
		{"github-copilot reads nothing", mapSecrets{
			"openhub.provider.github-copilot.token": "ghu",
		}, GithubCopilot, "", nil, platform.Credentials{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCredentials(context.Background(), tt.secrets, tt.prov, tt.projectID, tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("no secret store", func(t *testing.T) {
		assert.Equal(t, platform.Credentials{}, ResolveCredentials(context.Background(), nil, Bedrock, "proj-1", hub))
	})

	t.Run("the settings of the project override the hub ones", func(t *testing.T) {
		merged := ResolveProviderConfig(&ProviderConfig{AWSProfile: "project-profile", AWSRegion: "ap-southeast-1"}, *hub)
		got := ResolveCredentials(context.Background(), mapSecrets{}, Bedrock, "p1", &merged)
		assert.Equal(t, "project-profile", got.AWSProfile, "profile of the project")
		assert.Equal(t, "ap-southeast-1", got.AWSRegion, "region of the project")
	})
}
