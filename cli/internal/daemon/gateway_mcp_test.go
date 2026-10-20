package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func mcpMessage(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &msg))
	return msg
}

func call(params string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + params + `}`
}

// S-13: the MCP gateway fails closed. A tools/call is relayed only when its
// `_meta` names one session of the group of the token; discovery messages
// carry no session and pass.
func TestCheckMCPFailsClosed(t *testing.T) {
	adoptTries, adoptRetry = 1, time.Millisecond
	t.Cleanup(func() { adoptTries, adoptRetry = 5, 600*time.Millisecond })

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	for _, s := range []struct{ id, group string }{{"ses_a", "g1"}, {"ses_b", "g1"}, {"ses_x", "g2"}} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: s.id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: s.group, State: domain.RunActive}))
	}
	d := &Daemon{
		opts:     Options{Sessions: sessions},
		watchers: map[string]*watcher{"g1": {children: map[string]string{"ses_child": "ses_a", "ses_alien": "ses_x"}}},
	}

	tests := []struct {
		name    string
		msg     string
		allowed bool
	}{
		{"session of the group", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_a"}}`), true},
		{"another session of the same group", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_b"}}`), true},
		{"subagent of a session of the group", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_child"}}`), true},
		{"same session under two keys", call(`{"name":"t","_meta":{"a/sessionID":"ses_a","b/sessionid":"ses_a"}}`), true},
		{"other keys of _meta are ignored", call(`{"name":"t","_meta":{"progressToken":2,"ai.tool/sessionID":"ses_a"}}`), true},

		{"no params", `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`, false},
		{"no _meta", call(`{"name":"t","arguments":{}}`), false},
		{"empty _meta", call(`{"name":"t","_meta":{}}`), false},
		{"_meta without session", call(`{"name":"t","_meta":{"progressToken":2}}`), false},
		{"unknown session", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_unknown"}}`), false},
		{"session of another group", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_x"}}`), false},
		{"subagent of another group", call(`{"name":"t","_meta":{"ai.tool/sessionID":"ses_alien"}}`), false},
		{"two sessions: A acting for B", call(`{"name":"t","_meta":{"a/sessionID":"ses_a","b/sessionID":"ses_b"}}`), false},
		{"own session hiding a foreign one", call(`{"name":"t","_meta":{"a/sessionID":"ses_a","b/sessionID":"ses_x"}}`), false},
		{"empty session", call(`{"name":"t","_meta":{"ai.tool/sessionID":""}}`), false},
		{"session not a string", call(`{"name":"t","_meta":{"ai.tool/sessionID":7}}`), false},
		{"_meta not an object", call(`{"name":"t","_meta":"ses_a"}`), false},
		{"params not an object", call(`"ses_a"`), false},
		{"unknown method", `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"x"}}`, false},
		{"no method", `{"jsonrpc":"2.0","id":1}`, false},

		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`, true},
		{"initialized notification", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, true},
		{"tools/list", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, true},
		{"tools/list with the modern envelope", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`, true},
		{"prompts/list", `{"jsonrpc":"2.0","id":3,"method":"prompts/list"}`, true},
		{"ping", `{"jsonrpc":"2.0","id":4,"method":"ping"}`, true},
		{"discovery naming a foreign session", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"a/sessionID":"ses_x"}}}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := d.checkMCP(ctx, "g1", mcpMessage(t, tc.msg))
			if tc.allowed {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, gateway.ErrMCPDenied)
			assert.NotEmpty(t, err.Error())
		})
	}

	t.Run("sessions cannot be verified", func(t *testing.T) {
		blind := &Daemon{}
		assert.Error(t, blind.checkMCP(ctx, "g1", mcpMessage(t, call(`{"name":"t","_meta":{"a/sessionID":"ses_a"}}`))))
		assert.NoError(t, blind.checkMCP(ctx, "g1", mcpMessage(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)))
	})
}
