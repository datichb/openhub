package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// S-11: the plugins of the user configuration are visible in `oh doctor`
// (warning outside strict isolation) and, localized, at launch.
type fakePluginLister struct {
	plugins []string
	err     error
}

func (f fakePluginLister) UserPlugins() ([]string, error) { return f.plugins, f.err }

func TestUserPluginsCheck(t *testing.T) {
	_, ok := userPluginsCheck(nil, false)
	assert.False(t, ok, "an adapter without the capability")
	_, ok = userPluginsCheck(fakePluginLister{}, false)
	assert.False(t, ok, "nothing to report without plugins")
	_, ok = userPluginsCheck(fakePluginLister{plugins: []string{"x"}, err: errors.New("boom")}, false)
	assert.False(t, ok, "an unreadable configuration is not reported")

	l := fakePluginLister{plugins: []string{"user-pkg", "plugin/local.ts"}}
	check, ok := userPluginsCheck(l, false)
	require.True(t, ok)
	assert.True(t, check.OK)
	assert.True(t, check.Warn, "a warning outside strict isolation")
	assert.Contains(t, check.Detail, "user-pkg, plugin/local.ts")
	assert.Contains(t, check.Detail, "execution.strict_isolation")

	check, ok = userPluginsCheck(l, true)
	require.True(t, ok)
	assert.True(t, check.OK)
	assert.False(t, check.Warn, "hidden by strict isolation")
	assert.Contains(t, check.Detail, "user-pkg")
}

func TestIsolationWarningText(t *testing.T) {
	got := isolationWarningText(adapters.WarnUserPlugin + "user-plugin (/home/me/.config/x.ts)")
	assert.Contains(t, got, "user-plugin (/home/me/.config/x.ts)")
	assert.Contains(t, got, "execution.strict_isolation")
	assert.NotContains(t, got, adapters.WarnUserPlugin, "the raw prefix is not shown")

	got = isolationWarningText(adapters.WarnPluginsUnchecked + " (GET /api/plugin failed: boom)")
	assert.Contains(t, got, "(GET /api/plugin failed: boom)")
	assert.Contains(t, got, "execution.strict_isolation")

	assert.Contains(t, isolationWarningText("something else"), "something else")
}
