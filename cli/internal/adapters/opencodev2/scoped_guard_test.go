package opencodev2

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// SCR: the scoped guard (scratch folders) reaches the oh plugin with its
// exceptions, and the plugin applies it to the agents outside them; it is
// never rendered in the tool configuration.
func TestScopedGuardInPluginOptions(t *testing.T) {
	b := sessionspec.BundleSpec{EntryAgent: "a", Agents: []sessionspec.AgentDef{{ID: "a"}}}
	withP := withOhPlugin(b, "/p", "", nil, nil)
	_, ok := withP.Plugins[len(withP.Plugins)-1].Options["scopedGuard"]
	assert.False(t, ok, "no option without scoped rule")

	b.ScopedGuard = []sessionspec.ScopedRule{{
		PermissionRule: sessionspec.PermissionRule{Action: "external_directory", Resource: "*.oh/sessions/*/scratch/a/*", Effect: sessionspec.EffectDeny},
		Except:         []string{"a", "b"},
	}}
	withP = withOhPlugin(b, "/p", "", nil, nil)
	opts := withP.Plugins[len(withP.Plugins)-1].Options
	data, err := json.Marshal(opts["scopedGuard"])
	require.NoError(t, err)
	assert.JSONEq(t, `[{"action":"external_directory","resource":"*.oh/sessions/*/scratch/a/*","effect":"deny","except":["a","b"]}]`, string(data))

	for _, bs := range []sessionspec.BundleSpec{withP, withoutOhPlugin(b)} {
		cfg, err := BuildConfig(bs, sessionspec.ProviderSpec{}, nil)
		require.NoError(t, err)
		rendered, err := json.Marshal([]any{cfg["agents"], cfg["permissions"]})
		require.NoError(t, err)
		assert.NotContains(t, string(rendered), `"except"`)
		assert.NotContains(t, string(rendered), "scratch")
	}

	src := string(pluginIndex)
	assert.True(t, strings.Contains(src, "scopedGuard") && strings.Contains(src, "except"), "the plugin reads the scoped guard")
}
