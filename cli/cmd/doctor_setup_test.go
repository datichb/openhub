package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// withDoctorApp installs a (and no AWS region of the environment) for a check.
func withDoctorApp(t *testing.T, cfg *config.Config) {
	t.Helper()
	home := configtest.Home(t)
	for _, k := range []string{"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE"} {
		t.Setenv(k, "")
	}
	// The SDK finds ~/.aws of the real user whatever HOME is.
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(home, "no-aws-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(home, "no-aws-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	prev := application
	t.Cleanup(func() { application = prev })
	application = &app.App{Config: cfg}
}

func setupResults(t *testing.T) map[string]doctor.Result {
	t.Helper()
	reg, err := doctor.NewRegistry(doctorRegistry()...)
	require.NoError(t, err)
	c, ok := reg.Get("setup")
	require.True(t, ok, "oh doctor checks the setup of hub.toml")
	out := map[string]doctor.Result{}
	for _, r := range c.Results(context.Background()) {
		out[r.ID] = r
	}
	return out
}

// Bug 6 (K4a): a hub.toml with no setup_done, no default_provider and no
// region of the provider did not turn oh doctor red.
func TestDoctorSetupFlagsAnIncompleteHubToml(t *testing.T) {
	i18ntest.Set(t, "en")
	withDoctorApp(t, &config.Config{})

	res := setupResults(t)
	require.Len(t, res, 3)
	for id, want := range map[string]string{
		"setup_done":       "oh init",
		"default_provider": "oh config set llm.default_provider",
		"provider_region":  "oh config set provider.bedrock.aws_region",
	} {
		assert.False(t, res[id].OK, id)
		assert.Contains(t, res[id].Detail, want, "%s tells how to repair", id)
	}
}

func TestDoctorSetupPassesACompleteHubToml(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{}
	cfg.CLI.SetupDone = true
	cfg.LLM.DefaultProvider = "bedrock"
	cfg.Provider.Bedrock.AWSRegion = "eu-west-1"
	withDoctorApp(t, cfg)

	res := setupResults(t)
	require.Len(t, res, 3)
	for id, r := range res {
		assert.True(t, r.OK, id)
		assert.False(t, r.Warn, id)
	}
	assert.Contains(t, res["provider_region"].Detail, "eu-west-1")
}

// A region comes from the AWS environment or profile when hub.toml has none:
// not an error, but a warning that says how to write it down.
func TestDoctorSetupRegionFromTheAWSEnvironmentIsAWarning(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{}
	cfg.CLI.SetupDone = true
	cfg.LLM.DefaultProvider = "bedrock"
	withDoctorApp(t, cfg)
	t.Setenv("AWS_REGION", "eu-west-3")

	r := setupResults(t)["provider_region"]
	assert.True(t, r.OK)
	assert.True(t, r.Warn)
	assert.Contains(t, r.Detail, "eu-west-3")
	assert.Contains(t, r.Detail, "oh config set provider.bedrock.aws_region eu-west-3")
}

// Only Bedrock has a region; without a default provider Bedrock is the one
// a launch uses, so the region is still checked.
func TestDoctorSetupRegionOnlyForBedrock(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{}
	cfg.CLI.SetupDone = true
	cfg.LLM.DefaultProvider = "anthropic"
	withDoctorApp(t, cfg)

	res := setupResults(t)
	assert.Len(t, res, 2, "no region result for a provider that has none")
	assert.True(t, res["default_provider"].OK)
	assert.NotContains(t, res, "provider_region")
}

func TestDoctorSetupWithoutAnAppReportsNothing(t *testing.T) {
	withDoctorApp(t, nil)
	application = nil
	reg, err := doctor.NewRegistry(doctorRegistry()...)
	require.NoError(t, err)
	c, ok := reg.Get("setup")
	require.True(t, ok)
	assert.Empty(t, c.Results(context.Background()), "the config check already says the app is unavailable")
}
