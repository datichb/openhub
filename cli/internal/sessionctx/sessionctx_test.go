package sessionctx

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// fakeTool records the session context writes and the synthetic messages.
type fakeTool struct {
	adapters.ToolAdapter
	capable     bool
	unsupported bool
	sets        []string
	clears      []string
	synthetic   []string
}

func (f *fakeTool) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{SessionContext: f.capable}
}

func (f *fakeTool) SetSessionContext(_ context.Context, _ adapters.ServerHandle, sid, key string, value any) error {
	if f.unsupported {
		return fmt.Errorf("%w: 405", adapters.ErrUnsupported)
	}
	data, _ := json.Marshal(value)
	f.sets = append(f.sets, sid+" "+key+"="+string(data))
	return nil
}

func (f *fakeTool) ClearSessionContext(_ context.Context, _ adapters.ServerHandle, sid, key string) error {
	f.clears = append(f.clears, sid+" "+key)
	return nil
}

func (f *fakeTool) Control(_ context.Context, _ adapters.ServerHandle, sid string, op adapters.ControlOp) error {
	f.synthetic = append(f.synthetic, sid+" "+op.Text)
	return nil
}

var h = adapters.ServerHandle{URL: "http://127.0.0.1:1"}

// QB8: an entry is written only when its value changes (each write is a
// message in the tool history), and the last value survives the process.
func TestWriteOnlyOnChange(t *testing.T) {
	ctx := context.Background()
	f := &fakeTool{capable: true}
	dir := t.TempDir()
	w := &Writer{Adapter: f, Dir: dir}
	e := Entry{Key: KeyCheckpoints, Value: map[string]any{"passed": []string{"cp-1"}}}
	wrote, err := w.Set(ctx, h, "ses_a", e)
	require.NoError(t, err)
	assert.True(t, wrote)
	wrote, err = w.Set(ctx, h, "ses_a", e)
	require.NoError(t, err)
	assert.False(t, wrote, "unchanged")
	other := &Writer{Adapter: f, Dir: dir}
	wrote, _ = other.Set(ctx, h, "ses_a", e)
	assert.False(t, wrote, "last value read from ~/.oh/sessions/<id>/context.json")
	e.Value = map[string]any{"passed": []string{"cp-1", "cp-2"}}
	wrote, _ = w.Set(ctx, h, "ses_a", e)
	assert.True(t, wrote)
	assert.Len(t, f.sets, 2)
	assert.JSONEq(t, `{"passed":["cp-1","cp-2"]}`, string(w.Last("ses_a", KeyCheckpoints)))

	require.NoError(t, w.Clear(ctx, h, "ses_a", KeyCheckpoints))
	require.NoError(t, w.Clear(ctx, h, "ses_a", KeyCheckpoints), "nothing to clear")
	assert.Equal(t, []string{"ses_a oh.checkpoints"}, f.clears)
	assert.Nil(t, w.Last("ses_a", KeyCheckpoints))
	assert.Empty(t, f.synthetic)
}

// QB8: without the capability (or when the API is gone), an entry with a
// fallback text is sent as a synthetic message, the others are not sent.
func TestFallback(t *testing.T) {
	ctx := context.Background()
	for _, f := range []*fakeTool{{capable: false}, {capable: true, unsupported: true}} {
		w := &Writer{Adapter: f}
		wrote, err := w.Set(ctx, h, "ses_b", Entry{Key: KeyBudget, Value: map[string]any{"remaining_usd": 0}})
		require.NoError(t, err)
		assert.False(t, wrote, "no fallback: readable through the workflow MCP tools")
		wrote, err = w.Set(ctx, h, "ses_b", Entry{Key: KeyResume, Value: map[string]any{"text": "go on"}, Fallback: "go on"})
		require.NoError(t, err)
		assert.True(t, wrote)
		assert.Equal(t, []string{"ses_b go on"}, f.synthetic)
		wrote, _ = w.Set(ctx, h, "ses_b", Entry{Key: KeyResume, Value: map[string]any{"text": "go on"}, Fallback: "go on"})
		assert.False(t, wrote, "sent once")
		assert.Empty(t, f.sets)
	}
}
