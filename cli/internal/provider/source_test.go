package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

type mapSecrets map[string]string

func (m mapSecrets) Get(_ context.Context, k string) (string, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	return "", nil // real stores: absent key → empty value, no error
}

func TestResolveCredentialSourceCascade(t *testing.T) {
	ctx := context.Background()
	all := mapSecrets{
		"custom.key":                                 "custom",
		"openhub.provider.bedrock.token.p1":          "project",
		"openhub.team.core.provider.bedrock.token":   "team",
		"openhub.provider.bedrock.token":             "hub",
		"openhub.team.core.provider.anthropic.token": "team-anthropic",
	}

	r, err := ResolveCredentialSource(ctx, all, Bedrock, "p1", "core", "custom.key", nil)
	require.NoError(t, err)
	assert.Equal(t, "custom", r.Secret)

	r, _ = ResolveCredentialSource(ctx, all, Bedrock, "p1", "core", "", nil)
	assert.Equal(t, "project", r.Secret)
	assert.Equal(t, "project", r.Source.Scope)

	r, _ = ResolveCredentialSource(ctx, all, Bedrock, "p2", "core", "", nil)
	assert.Equal(t, "team", r.Secret)
	assert.Equal(t, domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "openhub.team.core.provider.bedrock.token", Scope: "team"}, r.Source)

	r, _ = ResolveCredentialSource(ctx, all, Bedrock, "p2", "other", "", nil)
	assert.Equal(t, "hub", r.Secret)

	r, _ = ResolveCredentialSource(ctx, all, Anthropic, "p2", "core", "", nil)
	assert.Equal(t, domain.CredentialAPIKey, r.Source.Kind)
	assert.Equal(t, "team-anthropic", r.Secret)
}

func TestResolveCredentialSourceFallbacks(t *testing.T) {
	ctx := context.Background()
	r, err := ResolveCredentialSource(ctx, mapSecrets{}, Bedrock, "p", "t", "", &Config{AWSProfile: "work", AWSRegion: "eu-west-1"})
	require.NoError(t, err)
	assert.Equal(t, domain.CredentialSigV4, r.Source.Kind)
	assert.Equal(t, "work", r.Source.Profile)
	assert.Equal(t, "eu-west-1", r.Region)
	assert.Empty(t, r.Secret)

	_, err = ResolveCredentialSource(ctx, nil, OpenRouter, "p", "", "", nil)
	assert.Error(t, err)
	assert.Equal(t, "", TeamKeychainKey(GithubCopilot, "core"))
}

type failingSecrets struct{}

func (failingSecrets) Get(context.Context, string) (string, error) {
	return "", errors.New("keychain locked")
}

// E14-K: a keychain failure never falls back to another key.
func TestResolveCredentialSourceKeychainErrorIsReported(t *testing.T) {
	_, err := ResolveCredentialSource(context.Background(), failingSecrets{}, Bedrock, "p1", "t1", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keychain locked")
}
