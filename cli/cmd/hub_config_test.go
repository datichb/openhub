package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

func seedHubToml(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(config.HubDir(), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(content), 0o600))
}

func diskConfig(t *testing.T) *config.Config {
	t.Helper()
	c, err := config.Read()
	require.NoError(t, err)
	return c
}

// updateHubConfig changes the settings of its caller on the file as it is and
// the Config the app shares follows, whatever it held.
func TestUpdateHubConfigKeepsTheFileAndRefreshesTheSharedConfig(t *testing.T) {
	configtest.Home(t)
	seedHubToml(t, "[cli]\nlanguage = \"en\"\n")
	a := newMockApp(nil, nil)
	shared, err := config.Load()
	require.NoError(t, err)
	a.Config = shared
	config.Reset() // the cache no longer is the Config of the app

	seedHubToml(t, "[cli]\nlanguage = \"en\"\nsetup_done = true\n[llm]\ndefault_provider = \"bedrock\"\n")

	require.NoError(t, updateHubConfig(a, func(c *config.Config) error {
		c.Worktree.BaseBranch = "develop"
		return nil
	}))

	got := diskConfig(t)
	assert.Equal(t, "develop", got.Worktree.BaseBranch)
	assert.True(t, got.CLI.SetupDone)
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)
	assert.Same(t, shared, a.Config, "the Config of the app keeps its pointer")
	assert.True(t, a.Config.CLI.SetupDone, "and takes the content of the file")
	assert.Equal(t, "develop", a.Config.Worktree.BaseBranch)
}

// The first-run wizard completes: the Config the views hold (the App they
// captured) gets setup_done, the provider, the region and the branch pattern;
// the App is not replaced.
func TestInitWizardCompletionUpdatesTheSharedConfig(t *testing.T) {
	configtest.Home(t)
	seedHubToml(t, "[cli]\nlanguage = \"en\"\n")
	a := newMockApp(nil, nil)
	shared, err := config.Load()
	require.NoError(t, err)
	a.Config = shared
	captured := a // what the views hold
	s := &initStepState{TeamState: &initWizardTeamState{}, AppPtr: &a}

	// A step that re-reads the file (team connect) leaves the cache with
	// another pointer than the Config of the app.
	config.Reset()
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.LLM.DefaultProvider = "bedrock"
		c.Provider.Bedrock.AWSRegion = "eu-west-1"
		c.Worktree.BranchPattern = "feat/{ticket}"
		return nil
	}))

	buildInitOnComplete(s)(true, nil)

	assert.Same(t, captured, *s.AppPtr, "the App the views captured is not replaced")
	assert.Same(t, shared, captured.Config)
	assert.True(t, captured.Config.CLI.SetupDone)
	assert.Equal(t, "bedrock", captured.Config.LLM.DefaultProvider)
	assert.Equal(t, "eu-west-1", captured.Config.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "feat/{ticket}", captured.Config.Worktree.BranchPattern)

	// A view that saves afterwards keeps all of it.
	require.NoError(t, updateHubConfig(captured, func(c *config.Config) error {
		c.MCP.Gitlab.Enabled = true
		return nil
	}))
	got := diskConfig(t)
	assert.True(t, got.CLI.SetupDone)
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)
	assert.Equal(t, "eu-west-1", got.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "feat/{ticket}", got.Worktree.BranchPattern)
	assert.True(t, got.MCP.Gitlab.Enabled)
}

func TestRefreshSharedConfigKeepsThePointer(t *testing.T) {
	configtest.Home(t)
	seedHubToml(t, "name = \"a\"\n")
	a := newMockApp(nil, nil)
	shared, err := config.Load()
	require.NoError(t, err)
	a.Config = shared
	seedHubToml(t, "name = \"b\"\n")

	require.NoError(t, refreshSharedConfig(a))
	assert.Same(t, shared, a.Config)
	assert.Equal(t, "b", a.Config.Name)
}
