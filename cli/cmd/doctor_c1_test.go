package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// A1: a pre-release or a dev build newer than the latest published release
// is never told to "update" to it; a newer release is a warning only.
func TestOhUpdateResultNeverOffersOlder(t *testing.T) {
	useLocale(t, "en")
	for _, current := range []string{"5.0.0-test", "5.0.0", "v4.2.0-195-gfe9ed932", "v4.2.0-195-gfe9ed932-dirty"} {
		r := ohUpdateResult(current, "4.2.0")
		assert.True(t, r.OK, current)
		assert.False(t, r.Warn, current)
		assert.NotContains(t, r.Detail, "available", current)
	}
	r := ohUpdateResult("4.2.0", "4.2.0")
	assert.True(t, r.OK)
	assert.False(t, r.Warn)

	r = ohUpdateResult("5.0.0-test", "5.0.0")
	assert.True(t, r.OK, "an update is not a failure")
	assert.True(t, r.Warn)
	assert.Contains(t, r.Detail, "5.0.0 available")

	r = ohUpdateResult("dev", "4.2.0")
	assert.True(t, r.OK)
	assert.False(t, r.Warn)
}

// A1: `oh upgrade oh` never installs an older version unless it is asked for
// explicitly (then flagged as a downgrade).
func TestPlanUpgradeNeverDowngradesImplicitly(t *testing.T) {
	tests := []struct {
		current, latest, requested string
		want                       upgradePlan
	}{
		{"5.0.0-test", "4.2.0", "", upgradePlan{status: versionAhead}},
		{"v4.2.0-195-gfe9ed932", "4.2.0", "", upgradePlan{status: versionAhead}},
		{"4.2.0", "4.2.0", "", upgradePlan{status: versionSame}},
		{"4.1.0", "4.2.0", "", upgradePlan{status: versionAvailable, version: "4.2.0"}},
		{"5.0.0-test", "5.0.0", "", upgradePlan{status: versionAvailable, version: "5.0.0"}},
		{"dev", "4.2.0", "", upgradePlan{status: versionUnknown, version: "4.2.0"}},
		{"5.0.0", "5.0.0", "v4.2.0", upgradePlan{status: versionAhead, version: "4.2.0", downgrade: true}},
		{"4.2.0", "5.0.0", "4.2.0", upgradePlan{status: versionSame}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, planUpgrade(tt.current, tt.latest, tt.requested), "%+v", tt)
	}
}

// A2: the credential check follows the launch cascade (project key of the
// current project → team → hub → AWS profile), not only the hub key.
func TestProviderCredentialStatusFollowsCascade(t *testing.T) {
	useLocale(t, "en")
	t.Setenv("HOME", t.TempDir())
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

// A40: the TUI Doctor view runs the same registry as `oh doctor`.
func TestDoctorViewUsesDoctorRegistry(t *testing.T) {
	if views.DoctorChecks == nil {
		t.Fatal("views.DoctorChecks not set")
	}
	assert.Equal(t, reflect.ValueOf(collectDoctorChecks).Pointer(), reflect.ValueOf(views.DoctorChecks).Pointer())
	useLocale(t, "en")
	var names []string
	for _, c := range doctorRegistry() {
		names = append(names, c.name)
	}
	for _, want := range []string{"oh version", "Provider credentials", "API keys (keychain)", "Beads zero-impact (hooks and gitignore)", "v5 runtime"} {
		assert.Contains(t, names, want)
	}
}

func useLocale(t *testing.T, l string) {
	t.Helper()
	prev := i18n.Locale()
	i18n.SetLocale(l)
	t.Cleanup(func() { i18n.SetLocale(prev) })
}
