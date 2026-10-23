package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

const (
	modelAtStart = "anthropic/claude-sonnet-4-6"
	modelLater   = "anthropic/claude-sonnet-5-5"
)

// runningTUI is a hub whose hub.toml names modelAtStart when the TUI starts:
// the Config of the app is the one it read then (not the shared cache, which
// config.Update would change in place: another process cannot do that).
func runningTUI(t *testing.T) (*app.App, *domain.Project) {
	t.Helper()
	a, p := dryRunHub(t)
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.Models.Default = modelAtStart
		return nil
	}))
	started, err := config.Read()
	require.NoError(t, err)
	a.Config = started
	return a, p
}

// anotherTerminal changes hub.toml the way `oh config model default` does in
// another process: the Config of the TUI is not told.
func anotherTerminal(t *testing.T, fn func(*config.Config)) {
	t.Helper()
	other, err := config.Read()
	require.NoError(t, err)
	fn(other)
	require.NoError(t, config.Save(other))
}

func entryModel(t *testing.T, p *launch.Prepared) string {
	t.Helper()
	require.NotNil(t, p.Bundle.Spec.DefaultModel, "the bundle gives its entry agent a model")
	return p.Bundle.Spec.DefaultModel.Model // the provider format: anthropic.claude-… with Bedrock
}

// Bug 19: a launch after `oh config model default …` in another terminal
// builds its bundle with the model of hub.toml as it is now.
func TestLaunchUsesHubTomlChangedSinceStart(t *testing.T) {
	a, p := runningTUI(t)
	opts := launch.Options{Workflow: "libre", Project: p, Inputs: map[string]string{}, DryRun: true}

	before, err := launch.Prepare(context.Background(), launchDeps(a), opts)
	require.NoError(t, err)
	assert.Contains(t, entryModel(t, before), "claude-sonnet-4-6")

	anotherTerminal(t, func(c *config.Config) { c.Models.Default = modelLater })

	after, err := launch.Prepare(context.Background(), launchDeps(a), opts)
	require.NoError(t, err)
	assert.Contains(t, entryModel(t, after), "claude-sonnet-5-5", "the new model of hub.toml, not the one read at start")
}

// The launch says what changed in the launch settings since the app read
// hub.toml.
func TestFreshLaunchAppReportsTheChangedSettings(t *testing.T) {
	a, _ := runningTUI(t)

	fa, changes := freshLaunchApp(a)
	assert.Same(t, a, fa, "nothing changed: the app is used as is")
	assert.Empty(t, changes)

	anotherTerminal(t, func(c *config.Config) {
		c.Models.Default = modelLater
		c.Provider.Bedrock.AWSRegion = "eu-west-3"
		c.CLI.Language = "fr" // not a launch setting
	})
	fa, changes = freshLaunchApp(a)
	assert.NotSame(t, a, fa)
	assert.Equal(t, modelLater, fa.Config.Models.Default)
	assert.Equal(t, modelAtStart, a.Config.Models.Default, "the shared Config is changed on the event loop only")
	require.Len(t, changes, 2, "%v", changes)
	joined := strings.Join(changes, "\n")
	assert.Contains(t, joined, "models.default")
	assert.Contains(t, joined, modelAtStart)
	assert.Contains(t, joined, modelLater)
	assert.Contains(t, joined, "provider.bedrock.aws_region")
	assert.NotContains(t, joined, "language", "only the settings that touch a launch")
}

// A hub.toml that cannot be read any more does not stop a launch: the
// settings in memory are used, and the recap says so.
func TestFreshLaunchAppUnreadableFile(t *testing.T) {
	a, _ := runningTUI(t)
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte("models = [broken\n"), 0o600))

	fa, changes := freshLaunchApp(a)
	assert.Same(t, a, fa)
	require.Len(t, changes, 1)
	assert.Contains(t, changes[0], "hub.toml")
}

// The shared Config follows on the event loop: what the views hold changes
// in place, nothing changes it off the loop.
func TestFreshLaunchAppSharesOnTheLoop(t *testing.T) {
	a, _ := runningTUI(t)
	var queued []func()
	prev := runOnLoop
	runOnLoop = func(fn func()) { queued = append(queued, fn) }
	t.Cleanup(func() { runOnLoop = prev })
	shared := a.Config

	anotherTerminal(t, func(c *config.Config) { c.Models.Default = modelLater })
	_, changes := freshLaunchApp(a)
	require.Len(t, changes, 1)
	require.Len(t, queued, 1)
	assert.Equal(t, modelAtStart, shared.Models.Default, "not before the loop runs it")
	queued[0]()
	assert.Same(t, shared, a.Config, "same pointer")
	assert.Equal(t, modelLater, shared.Models.Default, "the views see the file")

	_, changes = freshLaunchApp(a)
	assert.Empty(t, changes, "told once: the shared Config is up to date")
}

// TUI: the recap of the launch form uses the new model and signals the
// change; the launch that follows with another change in between rebuilds the
// bundle instead of starting the one that was shown.
func TestLaunchFormRecapSignalsChangedSettings(t *testing.T) {
	a, p := runningTUI(t)
	runOnLoop = func(fn func()) { fn() }
	t.Cleanup(func() { runOnLoop = func(func()) {} })

	cfg, err := launchFormConfig(context.Background(), a, p, tuiLaunchRequest{WorkflowID: "libre"})
	require.NoError(t, err)
	choices := views.LaunchChoices{Runtime: "local", Location: "base", Inputs: map[string]string{}}

	recap, err := cfg.Recap(context.Background(), choices)
	require.NoError(t, err)
	assert.False(t, hasConfigWarning(recap), "nothing changed: no signal")

	anotherTerminal(t, func(c *config.Config) { c.Models.Default = modelLater })
	recap, err = cfg.Recap(context.Background(), choices)
	require.NoError(t, err)
	assert.True(t, hasConfigWarning(recap), "warnings = %v", recap.Warnings)

	choices.Attach = "none" // another recap: the signal stays on screen
	recap, err = cfg.Recap(context.Background(), choices)
	require.NoError(t, err)
	assert.True(t, hasConfigWarning(recap), "warnings = %v", recap.Warnings)
}

func hasConfigWarning(r *views.LaunchRecap) bool {
	for _, w := range r.Warnings {
		if strings.Contains(w, "hub.toml") && strings.Contains(w, "models.default") {
			return true
		}
	}
	return false
}

// The prepared launch of a recap is dropped when hub.toml changed after it
// was built: the launch does not start a bundle with the old settings.
func TestRunCacheDropsPreparedLaunchWhenHubTomlChanges(t *testing.T) {
	a, p := runningTUI(t)
	runOnLoop = func(fn func()) { fn() }
	t.Cleanup(func() { runOnLoop = func(func()) {} })
	opts := launch.Options{Workflow: "libre", Project: p, Inputs: map[string]string{}, DryRun: true}
	c := &tuiRunCache{}

	first, err := c.get(context.Background(), a, opts)
	require.NoError(t, err)
	again, err := c.get(context.Background(), a, opts)
	require.NoError(t, err)
	assert.Same(t, first, again, "same choices, same file: no second build")
	assert.Empty(t, c.unshown())

	anotherTerminal(t, func(c *config.Config) { c.Models.Default = modelLater })
	rebuilt, err := c.get(context.Background(), a, opts)
	require.NoError(t, err)
	assert.NotSame(t, first, rebuilt)
	assert.Contains(t, entryModel(t, rebuilt), "claude-sonnet-5-5")
	require.Len(t, c.unshown(), 1)
	c.markShown()
	assert.Empty(t, c.unshown())
	assert.Len(t, c.allChanges(), 1, "the recap keeps showing it")
}
