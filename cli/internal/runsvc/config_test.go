package runsvc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/provider"
)

// E14-M6: provider, region, credential and exact project id separate groups.
func TestConfigFingerprintSeparatesGroups(t *testing.T) {
	base := StartRequest{ProjectID: "a.b", Provider: "bedrock"}
	cred := provider.ResolvedCredential{Source: domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "k"}, Secret: "s1"}
	fp := configFingerprint(base, cred, "eu-west-1")
	assert.Equal(t, fp, configFingerprint(base, cred, "eu-west-1"), "deterministic")

	other := base
	other.ProjectID = "a_b"
	assert.NotEqual(t, fp, configFingerprint(other, cred, "eu-west-1"))
	assert.NotEqual(t, fp, configFingerprint(base, cred, "us-east-1"))
	other = base
	other.Provider = "anthropic"
	assert.NotEqual(t, fp, configFingerprint(other, cred, "eu-west-1"))
	rotated := cred
	rotated.Secret = "s2"
	assert.NotEqual(t, fp, configFingerprint(base, rotated, "eu-west-1"), "a rotated key restarts the server")
	assert.NotContains(t, fp, "s1")
}

// E14-M5: the AWS region chain is used before the us-east-1 default.
func TestResolveProviderRegionFromEnv(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-west-3")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	s := &Service{}
	req := StartRequest{Provider: "bedrock"}
	_, region, err := s.resolveProvider(t.Context(), &req)
	assert.NoError(t, err)
	assert.Equal(t, "eu-west-3", region)

	req.ProviderCfg.AWSRegion = "eu-central-1"
	_, region, _ = s.resolveProvider(t.Context(), &req)
	assert.Equal(t, "eu-central-1", region, "oh config first")
}
