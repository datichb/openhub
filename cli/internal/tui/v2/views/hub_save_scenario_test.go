package views

import (
	"os"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Bug 1 of v5.1: the first-run wizard wrote setup_done, the provider, the
// region and the branch pattern, then the team MCP and team detail views saved
// the whole hub.toml from the copy they held since the start, and the wizard
// came back at every launch. The views now change only the settings they
// edited, on the file as it is.

// hubHost is what cmd gives the views: it changes hub.toml with config.Update
// and brings the shared Config (the one the views read) up to date.
type hubHost struct{ shared *config.Config }

func (h hubHost) update(fn func(*config.Config) error) error {
	fresh, err := config.UpdateAndGet(fn)
	if err == nil {
		*h.shared = *fresh
	}
	return err
}

// wizardWrites is what the wizard and the other parts of oh wrote to hub.toml
// while the view was open, without going through the Config the view reads
// (it re-read the file in between: ReloadApp, ReadConfig).
func wizardWrites(t *testing.T) {
	t.Helper()
	config.Reset()
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.CLI.SetupDone = true
		c.LLM.DefaultProvider = "bedrock"
		c.Provider.Bedrock.AWSRegion = "eu-west-1"
		c.Worktree.BranchPattern = "feat/{ticket}"
		c.MCP.Gitlab.TrustedURL = "https://gitlab.example.com"
		c.Tracker.TrackerTokenKey = "openhub.tracker.token"
		return nil
	}))
	_, err := config.Load()
	require.NoError(t, err)
}

func requireWizardFields(t *testing.T) *config.Config {
	t.Helper()
	got, err := config.Read()
	require.NoError(t, err)
	assert.True(t, got.CLI.SetupDone, "setup_done survives")
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)
	assert.Equal(t, "eu-west-1", got.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "feat/{ticket}", got.Worktree.BranchPattern)
	assert.Equal(t, "https://gitlab.example.com", got.MCP.Gitlab.TrustedURL, "what the member trusted survives")
	assert.Equal(t, "openhub.tracker.token", got.Tracker.TrackerTokenKey)
	return got
}

func seedHome(t *testing.T) *config.Config {
	t.Helper()
	home := configtest.Home(t)
	require.NoError(t, os.MkdirAll(home+"/.oh", 0o755))
	require.NoError(t, os.WriteFile(home+"/.oh/hub.toml", []byte("[cli]\nlanguage = \"en\"\n"), 0o600))
	shared, err := config.Load()
	require.NoError(t, err)
	return shared
}

func TestTeamMCPViewSaveKeepsTheFieldsTheWizardWrote(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	host := hubHost{shared}
	_, tc := soloState(t)
	v := NewTeamMCPView(TeamMCPViewConfig{
		ResolveTeam:     func() TeamResolution { return tc },
		GetMCPConfig:    func() config.MCPConfig { return shared.MCP },
		UpdateHubConfig: host.update,
	})
	v.SetShell(&viewShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })

	wizardWrites(t)

	onLoop(app, func() {
		v.localMCP.Gitlab.Enabled = true
		v.localMCP.Jira.URL = "https://jira.example.com"
		v.dirtyLocal = true
		v.save()
	})

	got := requireWizardFields(t)
	assert.True(t, got.MCP.Gitlab.Enabled, "the edit is saved")
	assert.Equal(t, "https://jira.example.com", got.MCP.Jira.URL)
	assert.True(t, shared.CLI.SetupDone, "the shared Config follows the file")

	// a second save, after another change of the file
	require.NoError(t, config.Update(func(c *config.Config) error { c.Websearch.Enabled = true; return nil }))
	onLoop(app, func() {
		v.localMCP.Gitlab.WriteEnabled = true
		v.dirtyLocal = true
		v.save()
	})
	got = requireWizardFields(t)
	assert.True(t, got.MCP.Gitlab.Enabled)
	assert.True(t, got.MCP.Gitlab.WriteEnabled)
	assert.True(t, got.Websearch.Enabled, "the second external change survives too")
}

func TestTeamDetailViewSaveKeepsTheFieldsTheWizardWrote(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	host := hubHost{shared}
	_, tc := soloState(t)
	v := NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return shared.MCP },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return shared.Tracker },
		ResolveTeam:           func() TeamResolution { return tc },
		GetSecrets:            func() tracker.SecretGetter { return nil },
		GetHubConfig:          func() *config.Config { return shared },
		UpdateHubConfig:       host.update,
	})
	v.SetShell(&viewShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })

	wizardWrites(t)

	yes := true
	onLoop(app, func() {
		v.localTrk.AutoSync = &yes
		v.localMCP.Gitlab.Enabled = true
		v.dirtyLocal = true
		v.save()
	})

	got := requireWizardFields(t)
	require.NotNil(t, got.Tracker.AutoSync)
	assert.True(t, *got.Tracker.AutoSync, "the edit is saved")
	assert.True(t, got.MCP.Gitlab.Enabled)
}

// hub.toml edited by another process between the load of the view and its save.
func TestTeamMCPViewSaveKeepsWhatAnotherProcessWrote(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	_, tc := soloState(t)
	v := NewTeamMCPView(TeamMCPViewConfig{
		ResolveTeam:     func() TeamResolution { return tc },
		GetMCPConfig:    func() config.MCPConfig { return shared.MCP },
		UpdateHubConfig: hubHost{shared}.update,
	})
	v.SetShell(&viewShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })

	path := config.ConfigPath()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, []byte("\n[mcp.figma]\nenabled = true\n[session]\nnotify = \"off\"\n")...), 0o600))

	onLoop(app, func() {
		v.localMCP.Jira.Enabled = true
		v.dirtyLocal = true
		v.save()
	})

	got, err := config.Read()
	require.NoError(t, err)
	assert.True(t, got.MCP.Jira.Enabled)
	assert.True(t, got.MCP.Figma.Enabled, "the change made by the other process is kept")
	assert.Equal(t, "off", got.Session.Notify)
}
