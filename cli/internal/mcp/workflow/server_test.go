package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

type fakeBackend struct {
	sessions []string
	calls    []checkpoint.Call
	outputs  []checkpoint.Output
	err      error
}

func (f *fakeBackend) WorkflowStatus(_ context.Context, s string) (checkpoint.Status, error) {
	f.sessions = append(f.sessions, s)
	return checkpoint.Status{SessionID: s, Workflow: "ticket", Mode: "manuel", Next: "cp-1"}, f.err
}

func (f *fakeBackend) WorkflowCheckpoint(_ context.Context, s string, c checkpoint.Call) (checkpoint.Result, error) {
	f.sessions = append(f.sessions, s)
	f.calls = append(f.calls, c)
	return checkpoint.Result{ID: c.ID, Label: "Démarrer", Message: "go", Next: "cp-2"}, f.err
}

func (f *fakeBackend) WorkflowOutput(_ context.Context, s string, o checkpoint.Output) error {
	f.sessions = append(f.sessions, s)
	f.outputs = append(f.outputs, o)
	return f.err
}

type rpcResult struct {
	Result struct {
		Tools   []struct{ Name string } `json:"tools"`
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	} `json:"result"`
}

// call runs one tools/call (or tools/list) through the stdio transport.
func call(t *testing.T, b Backend, method, params string) rpcResult {
	t.Helper()
	in := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + "}\n"
	pr, pw := io.Pipe()
	go func() {
		_ = New(b).ServeIO(context.Background(), strings.NewReader(in), pw)
		_ = pw.Close()
	}()
	line, err := bufio.NewReader(pr).ReadBytes('\n')
	require.NoError(t, err)
	var out rpcResult
	require.NoError(t, json.Unmarshal(line, &out), string(line))
	return out
}

const meta = `"_meta":{"ai.tool/sessionID":"ses_child","progressToken":2}`

func TestToolsList(t *testing.T) {
	out := call(t, &fakeBackend{}, "tools/list", `{}`)
	names := []string{}
	for _, tl := range out.Result.Tools {
		names = append(names, tl.Name)
	}
	assert.ElementsMatch(t, []string{"status", "checkpoint", "outputs"}, names)
}

func TestCheckpointCall(t *testing.T) {
	i18n.SetLocale("fr")
	b := &fakeBackend{}
	out := call(t, b, "tools/call", `{"name":"checkpoint","arguments":{"id":" cp-1 ","summary":"prêt"},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	assert.Equal(t, []string{"ses_child"}, b.sessions, "session read from _meta")
	assert.Equal(t, []checkpoint.Call{{ID: "cp-1", Summary: "prêt"}}, b.calls)
	txt := out.Result.Content[0].Text
	assert.Contains(t, txt, "cp-1")
	assert.Contains(t, txt, "go")
	assert.Contains(t, txt, "cp-2")
}

func TestCheckpointWithoutDaemonStillLetsTheAgentGoOn(t *testing.T) {
	out := call(t, &fakeBackend{err: daemon.ErrNotRunning}, "tools/call", `{"name":"checkpoint","arguments":{"id":"cp-1","summary":"x"},`+meta+`}`)
	assert.False(t, out.Result.IsError)
}

func TestErrors(t *testing.T) {
	out := call(t, &fakeBackend{}, "tools/call", `{"name":"status","arguments":{}}`)
	assert.True(t, out.Result.IsError, "no session in _meta")

	out = call(t, &fakeBackend{}, "tools/call", `{"name":"checkpoint","arguments":{"summary":"x"},`+meta+`}`)
	assert.True(t, out.Result.IsError, "id required")

	api := &daemon.APIError{Status: 422, Message: `unknown checkpoint "cp-9" (cp-1, cp-2)`}
	out = call(t, &fakeBackend{err: api}, "tools/call", `{"name":"checkpoint","arguments":{"id":"cp-9","summary":"x"},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Equal(t, api.Message, out.Result.Content[0].Text, "daemon message shown as is")

	out = call(t, &fakeBackend{err: daemon.ErrNotRunning}, "tools/call", `{"name":"status","arguments":{},`+meta+`}`)
	assert.True(t, out.Result.IsError)
}

func TestStatusAndOutputs(t *testing.T) {
	b := &fakeBackend{}
	out := call(t, b, "tools/call", `{"name":"status","arguments":{},`+meta+`}`)
	require.False(t, out.Result.IsError)
	var st checkpoint.Status
	require.NoError(t, json.Unmarshal([]byte(out.Result.Content[0].Text), &st))
	assert.Equal(t, "cp-1", st.Next)

	out = call(t, b, "tools/call", `{"name":"outputs","arguments":{"type":"branch","value":"feat/x"},`+meta+`}`)
	require.False(t, out.Result.IsError)
	assert.Equal(t, "feat/x", b.outputs[0].Value)

	out = call(t, b, "tools/call", `{"name":"outputs","arguments":{"type":"branch"},`+meta+`}`)
	assert.True(t, out.Result.IsError, "value required")
}

func TestToolSession(t *testing.T) {
	assert.Equal(t, "", ToolSession(context.Background()))
}
