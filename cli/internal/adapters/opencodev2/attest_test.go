package opencodev2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestSkillVisibleOnServerUsesServerRules(t *testing.T) {
	b := sessionspec.BundleSpec{
		Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}, {ID: "helper", Mode: "subagent"}, {ID: "quiet", Mode: "subagent"}},
		Skills: []sessionspec.SkillDef{{ID: "alpha"}},
	}
	closed := []sessionspec.PermissionRule{{Action: "*", Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: "skill", Resource: "*", Effect: sessionspec.EffectDeny}, {Action: "skill", Resource: "alpha", Effect: sessionspec.EffectAllow}}
	reopened := append(append([]sessionspec.PermissionRule(nil), closed...), sessionspec.PermissionRule{Action: "skill", Resource: "report", Effect: sessionspec.EffectAllow})
	agents := []Agent{
		{ID: "lead", Permissions: closed},
		{ID: "helper", Permissions: reopened}, // what the server applies, whatever was rendered
		{ID: "quiet"},                         // no rules listed: judged on the rendered rules
	}
	assert.Equal(t, []string{"helper"}, skillVisibleOnServer(agents, b, "report"))
	assert.Empty(t, skillVisibleOnServer(agents[:1], b, "report"))
}

// A bundle skill the server exposes to an agent outside its catalogue
// breaks the closed world of that agent.
func TestOutsideCatalogue(t *testing.T) {
	b := sessionspec.BundleSpec{
		Agents: []sessionspec.AgentDef{{ID: "lead", Skills: []string{"alpha"}}, {ID: "helper", Skills: []string{}}, {ID: "old"}},
		Skills: []sessionspec.SkillDef{{ID: "alpha"}},
	}
	open := []sessionspec.PermissionRule{{Action: "skill", Resource: "alpha", Effect: sessionspec.EffectAllow}}
	agents := []Agent{{ID: "lead", Permissions: open}, {ID: "helper", Permissions: open}, {ID: "old", Permissions: open}}
	assert.Equal(t, []string{"helper"}, outsideCatalogue(agents, b, "alpha"), "old: no catalogue, every bundle skill")
	assert.Empty(t, outsideCatalogue(agents[:1], b, "alpha"))
	// Rendered rules (server listing no rules) follow the catalogue.
	assert.Empty(t, outsideCatalogue([]Agent{{ID: "lead"}, {ID: "helper"}}, b, "alpha"))
}

// fakeToolServer answers the listings Attest reads. pluginsJSON is the body
// of /api/plugin; an empty one makes the route fail (500).
func fakeToolServer(t *testing.T, pluginsJSON string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/agent":
			_, _ = w.Write([]byte(`{"data":[{"id":"lead","mode":"primary"}]}`))
		case "/api/skill", "/api/mcp":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/plugin":
			if pluginsJSON == "" {
				http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(pluginsJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "")
}

// S-11: a plugin the bundle does not bring (typically from the user's own
// opencode configuration) is a warning, and a break of the isolation under
// strict isolation; a plugin list that cannot be read is a warning, and a
// failure under strict isolation.
func TestAttestUserPlugins(t *testing.T) {
	b := sessionspec.BundleSpec{
		Agents:  []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}},
		Plugins: []sessionspec.PluginDef{{ID: "bundled-plugin"}},
	}
	const plugins = `{"data":[
		{"id":"` + OhPluginID + `","source":{"type":"file","path":"/data/oh-plugin"},"state":{"status":"active"}},
		{"id":"bundled-plugin","source":{"type":"file"},"state":{"status":"active"}},
		{"id":"core","source":{"type":"builtin"},"state":{"status":"active"}},
		{"id":"user-plugin","source":{"type":"file","path":"/home/me/.config/opencode/plugin/x.ts"},"state":{"status":"active"}}]}`
	const onlyBundle = `{"data":[{"id":"` + OhPluginID + `","source":{"type":"file"},"state":{"status":"active"}},
		{"id":"bundled-plugin","source":{"type":"file"},"state":{"status":"active"}}]}`

	t.Run("unexpected plugin, not strict: warning", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, plugins), b, "/p", false)
		require.NoError(t, err)
		assert.True(t, rep.OK(), "unexpected: %v", rep.Unexpected)
		assert.Equal(t, []string{"plugin:user-plugin (/home/me/.config/opencode/plugin/x.ts)"}, rep.Warnings)
	})
	t.Run("unexpected plugin, strict: Attest fails the isolation", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, plugins), b, "/p", true)
		require.NoError(t, err)
		assert.False(t, rep.OK())
		assert.Equal(t, []string{"plugin:user-plugin (/home/me/.config/opencode/plugin/x.ts)"}, rep.Unexpected)
		assert.Empty(t, rep.Warnings)
		assert.Equal(t, sessionspec.IsolationNone, rep.Level)
	})
	t.Run("only the plugins of the bundle, strict: fine", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, onlyBundle), b, "/p", true)
		require.NoError(t, err)
		assert.True(t, rep.OK(), "unexpected: %v", rep.Unexpected)
		assert.Empty(t, rep.Warnings)
		assert.Equal(t, sessionspec.IsolationFull, rep.Level)
	})
	t.Run("plugin list unreadable, not strict: explicit warning", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, ""), b, "/p", false)
		require.NoError(t, err)
		assert.True(t, rep.OK())
		require.Len(t, rep.Warnings, 1)
		assert.Contains(t, rep.Warnings[0], "plugins:unchecked")
		assert.Contains(t, rep.Warnings[0], "/api/plugin")
	})
	t.Run("plugin list unreadable, strict: failure", func(t *testing.T) {
		_, err := Attest(context.Background(), fakeToolServer(t, ""), b, "/p", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing plugins")
	})
}

// A plugin the workflow or the package declares but the server does not run
// (not listed, or listed in another state than active) is reported: a warning,
// a break of the isolation under strict isolation. A plugin list that cannot
// be read leaves it unchecked, and says so.
func TestAttestDeclaredPluginInactive(t *testing.T) {
	old := declaredPluginWait
	declaredPluginWait = 0
	t.Cleanup(func() { declaredPluginWait = old })

	b := sessionspec.BundleSpec{
		Agents:  []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}},
		Plugins: []sessionspec.PluginDef{{ID: "@scope/pkg", Dir: "@scope/pkg@^1"}, {ID: "other", Dir: "other"}},
	}
	const ohActive = `{"id":"` + OhPluginID + `","source":{"type":"file"},"state":{"status":"active"}}`
	const allActive = `{"data":[` + ohActive + `,
		{"id":"@scope/pkg","source":{"type":"npm"},"state":{"status":"active"}},
		{"id":"other","source":{"type":"npm"},"state":{"status":"active"}}]}`
	const notLoaded = `{"data":[` + ohActive + `,{"id":"other","source":{"type":"npm"},"state":{"status":"active"}}]}`
	const failed = `{"data":[` + ohActive + `,
		{"id":"@scope/pkg","source":{"type":"npm"},"state":{"status":"failed"}},
		{"id":"other","source":{"type":"npm"},"state":{"status":"active"}}]}`
	const byPath = `{"data":[` + ohActive + `,
		{"id":"pkg-own-id","source":{"type":"npm","path":"/data/node_modules/@scope/pkg/index.js"},"state":{"status":"active"}},
		{"id":"other","source":{"type":"npm"},"state":{"status":"active"}}]}`

	t.Run("all declared plugins active: nothing to report", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, allActive), b, "/p", true)
		require.NoError(t, err)
		assert.True(t, rep.OK(), "unexpected: %v", rep.Unexpected)
		assert.Empty(t, rep.Warnings)
	})
	t.Run("declared plugin not loaded, not strict: warning", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, notLoaded), b, "/p", false)
		require.NoError(t, err)
		assert.True(t, rep.OK())
		assert.Equal(t, []string{"plugin-inactive:@scope/pkg (not loaded)"}, rep.Warnings)
	})
	t.Run("declared plugin failed, not strict: warning with the state", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, failed), b, "/p", false)
		require.NoError(t, err)
		assert.True(t, rep.OK())
		assert.Equal(t, []string{"plugin-inactive:@scope/pkg (failed)"}, rep.Warnings)
	})
	t.Run("declared plugin not loaded, strict: the isolation check fails", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, notLoaded), b, "/p", true)
		require.NoError(t, err)
		assert.False(t, rep.OK())
		assert.Equal(t, []string{"plugin-inactive:@scope/pkg (not loaded)"}, rep.Unexpected)
		assert.Empty(t, rep.Warnings)
		assert.Equal(t, sessionspec.IsolationNone, rep.Level)
	})
	t.Run("a plugin under its own id is found by its install path", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, byPath), b, "/p", true)
		require.NoError(t, err)
		assert.True(t, rep.OK(), "unexpected: %v", rep.Unexpected)
	})
	t.Run("plugin list unreadable: unchecked, no plugin reported inactive", func(t *testing.T) {
		rep, err := Attest(context.Background(), fakeToolServer(t, ""), b, "/p", false)
		require.NoError(t, err)
		require.Len(t, rep.Warnings, 1)
		assert.Contains(t, rep.Warnings[0], "plugins:unchecked")
	})
}
