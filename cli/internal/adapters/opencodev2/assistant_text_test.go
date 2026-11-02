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
)

// assistantMsg is a message of GET /api/session/{id}/message (shape recorded
// on opencode 2.0.20, see testdata/transcript): the text parts have no id and
// no time, the message has both.
func assistantMsg(id string, created int, texts ...string) json.RawMessage {
	var parts []map[string]any
	for _, s := range texts {
		parts = append(parts, map[string]any{"type": "text", "text": s})
	}
	data, _ := json.Marshal(map[string]any{"id": id, "type": "assistant", "time": map[string]any{"created": created}, "content": parts})
	return data
}

// --headless: the text of the assistant follows the real order of the steps
// (time, then id of the message) and of the parts of a message, whatever the
// order the server lists them in, with a separator between the pieces.
func TestAssistantTextOrder(t *testing.T) {
	serve := func(t *testing.T, msgs []json.RawMessage) *Client {
		c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
			if r.URL.Path != "/api/session/ses_1/message" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": msgs, "cursor": map[string]any{"next": nil}})
		})
		return c
	}
	user := json.RawMessage(`{"id":"msg_0","type":"user","time":{"created":1},"text":"question"}`)
	idle := json.RawMessage(`{"id":"msg_9","type":"idle","time":{"created":99},"outcome":"succeeded"}`)

	t.Run("newest first from the server", func(t *testing.T) {
		c := serve(t, []json.RawMessage{idle, assistantMsg("msg_c", 30, "third"), assistantMsg("msg_b", 20, "second"), assistantMsg("msg_a", 10, "first"), user})
		got, err := c.AssistantText(context.Background(), "ses_1")
		require.NoError(t, err)
		assert.Equal(t, "first\n\nsecond\n\nthird", got)
	})
	t.Run("same time: by message id", func(t *testing.T) {
		c := serve(t, []json.RawMessage{assistantMsg("msg_b", 10, "second"), assistantMsg("msg_a", 10, "first")})
		got, err := c.AssistantText(context.Background(), "ses_1")
		require.NoError(t, err)
		assert.Equal(t, "first\n\nsecond", got)
	})
	t.Run("parts of a message keep their order, tools and empty parts are skipped", func(t *testing.T) {
		msg := json.RawMessage(`{"id":"msg_a","type":"assistant","time":{"created":10},"content":[
			{"type":"text","text":"I run the tests."},{"type":"tool","id":"t1","name":"shell","state":{"status":"completed"}},
			{"type":"text","text":""},{"type":"text","text":"They pass.\n"}]}`)
		c := serve(t, []json.RawMessage{assistantMsg("msg_b", 20, "Done."), msg})
		got, err := c.AssistantText(context.Background(), "ses_1")
		require.NoError(t, err)
		assert.Equal(t, "I run the tests.\n\nThey pass.\n\nDone.", got)
	})
	t.Run("no assistant text", func(t *testing.T) {
		c := serve(t, []json.RawMessage{user, idle})
		got, err := c.AssistantText(context.Background(), "ses_1")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// A long session is read in pages, oldest first: nothing is missing from the
// text when the server cuts the list.
func TestAssistantTextReadsEveryPage(t *testing.T) {
	var all []json.RawMessage
	for i := 0; i < transcriptPage+5; i++ {
		all = append(all, assistantMsg(fmt.Sprintf("msg_%04d", i), 1000+i, fmt.Sprintf("step %d", i)))
	}
	c, calls := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		page := all[:transcriptPage]
		next := any("c2")
		if r.URL.Query().Get("cursor") != "" {
			page, next = all[transcriptPage:], nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": page, "cursor": map[string]any{"next": next}})
	})
	got, err := c.AssistantText(context.Background(), "ses_1")
	require.NoError(t, err)
	assert.Contains(t, got, "step 0\n\nstep 1\n\n")
	assert.Contains(t, got, fmt.Sprintf("step %d", transcriptPage+4))
	assert.Contains(t, (*calls)[0].query, "order=asc")
}

// The messages recorded on opencode 2.0.20: the answer of the root session is
// its two text steps, in order.
func TestAssistantTextOnRecordedMessages(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "transcript", "root.json"))
	require.NoError(t, err)
	var msgs []json.RawMessage
	require.NoError(t, json.Unmarshal(data, &msgs))
	// listed newest first, as a server that ignores the order would
	rev := make([]json.RawMessage, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		rev = append(rev, msgs[i])
	}
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request, _ recorded) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rev, "cursor": map[string]any{"next": nil}})
	})
	got, err := c.AssistantText(context.Background(), "ses_1")
	require.NoError(t, err)
	var want []string
	for _, m := range msgs {
		var w wireMessage
		require.NoError(t, json.Unmarshal(m, &w))
		for _, p := range w.Content {
			if p.Type == "text" && p.Text != "" {
				want = append(want, p.Text)
			}
		}
	}
	require.Len(t, want, 2)
	assert.Equal(t, want[0]+"\n\n"+want[1], got)
}
