package opencodev2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// pluginDriver runs the evaluate hook of the embedded plugin on events
// (bun, skipped without it) and returns the decisions.
const pluginDriver = `
import plugin from "./index.ts"
const input = JSON.parse(process.env.OH_DRIVER_INPUT)
process.env.OH_TEST_TOKEN = "tok"
const hooks = {}
const reg = (name) => ({ hook: async (k, fn) => { hooks[name + "." + k] = fn }, transform: async () => {} })
const ctx = { options: input.options, agent: reg("agent"), skill: reg("skill"), shell: reg("shell"), tool: reg("tool"), permission: reg("permission"), session: reg("session") }
await plugin.setup(ctx)
const out = []
for (const ev of input.events) {
  await hooks["permission.evaluate"](ev)
  out.push({ effect: ev.effect, message: ev.message ?? "" })
}
console.log(JSON.stringify(out))
`

type driverResult struct {
	Effect  string `json:"effect"`
	Message string `json:"message"`
}

func runPlugin(t *testing.T, opts map[string]any, events []map[string]any) []driverResult {
	t.Helper()
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun not installed")
	}
	dir := t.TempDir()
	require.NoError(t, WritePlugin(dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "driver.ts"), []byte(pluginDriver), 0o644))
	in, err := json.Marshal(map[string]any{"options": opts, "events": events})
	require.NoError(t, err)
	cmd := exec.Command(bun, "driver.ts")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "OH_DRIVER_INPUT="+string(in))
	raw, err := cmd.Output()
	require.NoError(t, err, string(raw))
	var out []driverResult
	require.NoError(t, json.Unmarshal(raw, &out), string(raw))
	return out
}

// p6-debts, closed by default: a request the session rules may lock is
// refused unless oh confirms it (daemon down, session unknown to oh); the
// others keep the rendered decision.
func TestPluginHoldsLockedRequests(t *testing.T) {
	pending := toRules([]sessionspec.PermissionRule{{Action: sessionspec.ActionShell, Resource: "git commit*", Effect: sessionspec.EffectDeny}})
	ev := func(cmd, effect string) map[string]any {
		return map[string]any{"sessionID": "ses_child", "agent": "developer", "action": "shell", "resources": []string{cmd}, "effect": effect}
	}
	events := []map[string]any{ev("git commit -m wip", "allow"), ev("git status", "allow"), ev("git commit -m x", "deny"), ev("git commit -m y", "ask")}
	base := map[string]any{"tokenEnv": "OH_TEST_TOKEN", "pending": pending, "pendingMessage": "held by oh"}
	with := func(url string) map[string]any {
		o := map[string]any{"hookURL": url}
		for k, v := range base {
			o[k] = v
		}
		return o
	}

	// Daemon down.
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	out := runPlugin(t, with(down.URL), events)
	assert.Equal(t, []driverResult{{"deny", "held by oh"}, {"allow", ""}, {"deny", ""}, {"deny", "held by oh"}}, out)

	// Session unknown to oh (403).
	var calls atomic.Int32
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer forbidden.Close()
	out = runPlugin(t, with(forbidden.URL), events[:1])
	assert.Equal(t, "deny", out[0].Effect)

	// oh confirms: the rendered decision stands; oh refuses: refused with its message.
	var seen atomic.Bool
	confirm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["pending"] == true {
			seen.Store(true)
		}
		res := body["resources"].([]any)[0].(string)
		if res == "git commit -m y" {
			_, _ = w.Write([]byte(`{"effect":"deny","message":"locked until cp-2"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer confirm.Close()
	out = runPlugin(t, with(confirm.URL), events)
	assert.Equal(t, []driverResult{{"allow", ""}, {"allow", ""}, {"deny", ""}, {"deny", "locked until cp-2"}}, out)
	assert.True(t, seen.Load(), "held requests are marked for oh")

	// Without held rules (bundle without workflow): unchanged.
	out = runPlugin(t, map[string]any{"hookURL": down.URL, "tokenEnv": "OH_TEST_TOKEN"}, events[:1])
	assert.Equal(t, "allow", out[0].Effect)
}

func TestPendingGuardOptions(t *testing.T) {
	b := sessionspec.BundleSpec{EntryAgent: "a", Agents: []sessionspec.AgentDef{{ID: "a"}}}
	opts := withOhPlugin(b, "/p", "", nil, nil).Plugins[0].Options
	_, ok := opts["pending"]
	assert.False(t, ok)
	b.PendingGuard = []sessionspec.PermissionRule{{Action: "mcp:gitlab/create_merge_request", Resource: "*", Effect: sessionspec.EffectDeny}}
	b.PendingMessage = "m"
	opts = withOhPlugin(b, "/p", "", nil, nil).Plugins[0].Options
	assert.Equal(t, []Rule{{Action: "gitlab_create_merge_request", Resource: "*", Effect: "deny"}}, opts["pending"], "tool action names")
	assert.Equal(t, "m", opts["pendingMessage"])

	// The fallback mode (no plugin) is refused when subagents would run
	// without their locks.
	assert.NoError(t, fallbackAllowed(b))
	b.SubagentGraph = map[string][]string{"a": {"b"}}
	assert.Error(t, fallbackAllowed(b))
	b.PendingGuard = nil
	assert.NoError(t, fallbackAllowed(b))
}
