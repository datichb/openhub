package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/provider"
)

// The credential check follows the launch cascade (project key of the
// current project → team → hub → AWS profile), not only the hub key.
func TestProviderCredentialStatusFollowsCascade(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_BEARER_TOKEN_BEDROCK", "AWS_PROFILE"} {
		t.Setenv(k, "")
	}
	ctx := context.Background()
	project := &domain.Project{ID: "p1", Name: "demo"}

	projectKey := provider.KeychainKey(provider.Bedrock, "p1")
	a := newMockApp(map[string]string{projectKey: "secret"}, nil)
	a.Config.LLM.DefaultProvider = "bedrock"
	detail, ok := providerCredentialStatus(ctx, a, project)
	assert.True(t, ok, detail)
	assert.Contains(t, detail, "project key")
	assert.Contains(t, detail, projectKey)

	// Outside the project, the hub level has no key and no AWS profile.
	detail, ok = providerCredentialStatus(ctx, a, nil)
	assert.False(t, ok, detail)
	assert.Contains(t, detail, "hub level")

	teamID := "t1"
	teamKey := provider.TeamKeychainKey(provider.Bedrock, teamID)
	a = newMockApp(map[string]string{teamKey: "secret"}, nil)
	a.Config.LLM.DefaultProvider = "bedrock"
	a.Config.Teams = []config.TeamConfig{{ID: teamID, Enabled: true}}
	teamProject := &domain.Project{ID: "p2", Name: "team", TeamID: &teamID}
	detail, ok = providerCredentialStatus(ctx, a, teamProject)
	assert.True(t, ok, detail)
	assert.Contains(t, detail, "team key")

	// The project's own provider wins over the hub default.
	a = newMockApp(map[string]string{provider.KeychainKey(provider.Anthropic, ""): "k"}, nil)
	a.Config.LLM.DefaultProvider = "bedrock"
	detail, ok = providerCredentialStatus(ctx, a, &domain.Project{ID: "p3", Name: "x", Provider: "anthropic"})
	assert.True(t, ok, detail)
	assert.True(t, strings.HasPrefix(detail, "anthropic"), detail)
	assert.Contains(t, detail, "hub key")

	a = newMockApp(nil, &config.ProviderConfigs{Bedrock: config.ProviderConfig{AWSProfile: "dev"}})
	a.Config.LLM.DefaultProvider = "bedrock"
	detail, ok = providerCredentialStatus(ctx, a, nil)
	assert.True(t, ok, detail)
	assert.Contains(t, detail, "AWS profile dev")
}
