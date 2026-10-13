package opencodev2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The transcript of a session reads its pages (cursor) and the messages of
// its subagent sessions (messages recorded on opencode 2.0.20).
func TestTranscriptPagesAndChildren(t *testing.T) {
	const root, child = "ses_T7V9Chf1PJA3F2M5Dam5zvqgcD", "ses_ecc2aa405ffewc7sdEbiT23B83"
	load := func(name string) []json.RawMessage {
		data, err := os.ReadFile(filepath.Join("testdata", "transcript", name))
		require.NoError(t, err)
		var out []json.RawMessage
		require.NoError(t, json.Unmarshal(data, &out))
		return out
	}
	rootMsgs, childMsgs := load("root.json"), load("child.json")
	c, calls := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		page := func(data []json.RawMessage, next string) {
			resp := map[string]any{"data": data, "cursor": map[string]any{"next": nil}}
			if next != "" {
				resp["cursor"] = map[string]any{"next": next}
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
		switch r.URL.Path {
		case "/api/session":
			fmt.Fprintf(w, `{"data":[{"id":%q},{"id":%q,"parentID":%q}]}`, root, child, root)
		case "/api/session/" + root + "/message":
			if r.URL.Query().Get("cursor") == "" {
				// A full first page: the reader follows the cursor.
				first := make([]json.RawMessage, 0, transcriptPage)
				for len(first) < transcriptPage {
					first = append(first, rootMsgs[0])
				}
				page(first, "c2")
				return
			}
			page(rootMsgs[1:], "")
		case "/api/session/" + child + "/message":
			page(childMsgs, "")
		default:
			http.NotFound(w, r)
		}
	})
	a := &Adapter{}
	entries, err := a.Transcript(context.Background(), adapters.ServerHandle{URL: c.baseURL, Password: "secret"}, root)
	require.NoError(t, err)

	var prompts, ends int
	var skill, delegate, shell *adapters.TranscriptEntry
	for i := range entries {
		e := &entries[i]
		switch {
		case e.Kind == adapters.EntryPrompt:
			prompts++
		case e.Kind == adapters.EntryEnd:
			ends++
		case e.Kind == adapters.EntryTool && e.Call.Action == sessionspec.ActionSkill:
			skill = e
		case e.Kind == adapters.EntryTool && e.Call.Action == sessionspec.ActionSubagent:
			delegate = e
		case e.Kind == adapters.EntryTool && e.Call.Action == sessionspec.ActionShell:
			shell = e
		}
	}
	assert.Equal(t, transcriptPage+1, prompts, "second page read (the first one repeats the prompt)")
	assert.Equal(t, 2, ends, "root and child sessions")
	require.NotNil(t, skill)
	assert.Equal(t, "alpha", skill.Call.Skill)
	assert.Equal(t, "lead", skill.Agent)
	require.NotNil(t, delegate)
	assert.Equal(t, child, delegate.Child)
	assert.Equal(t, "helper", delegate.Call.Input["agent"])
	require.NotNil(t, shell)
	assert.Equal(t, child, shell.SessionID)
	assert.Equal(t, root, shell.ParentID)
	assert.Equal(t, "helper", shell.Agent)
	assert.Equal(t, "echo probe-ok", shell.Call.Input["command"])
	assert.Equal(t, "probe-ok\n", shell.Output)
	assert.Equal(t, adapters.CallOK, shell.Call.Status)
	assert.Equal(t, "amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0", shell.Model)
	for i := 1; i < len(entries); i++ {
		assert.False(t, entries[i].Time.Before(entries[i-1].Time), "oldest first")
	}
	var cost float64
	for _, e := range entries {
		cost += e.Cost
	}
	assert.InDelta(t, 0.00164373+0.006634925+0.0042119+0.0044033, cost, 1e-9)
	assert.Contains(t, (*calls)[1].query, "order=asc")
}
