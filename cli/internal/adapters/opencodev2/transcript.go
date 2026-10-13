package opencodev2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Session transcript (`oh session trace`, evaluations): the messages of a
// session and of its subagent sessions (GET /api/session/{id}/message,
// oldest first, followed through its cursors), in neutral terms.

var _ adapters.Transcriber = (*Adapter)(nil)

// transcriptPage is the page size of the message reads.
const transcriptPage = 200

// transcriptMaxPages bounds the reads of one session (runaway cursor).
const transcriptMaxPages = 200

type wireMessage struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Time struct {
		Created   float64 `json:"created"`
		Completed float64 `json:"completed"`
	} `json:"time"`
	Text    string `json:"text"`
	Agent   string `json:"agent"`
	Skill   string `json:"skill"`
	Name    string `json:"name"`
	Command string `json:"command"`
	Outcome string `json:"outcome"`
	Model   struct {
		ID         string `json:"id"`
		ProviderID string `json:"providerID"`
	} `json:"model"`
	Content []wirePart `json:"content"`
	Cost    float64    `json:"cost"`
	Tokens  struct {
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
	} `json:"tokens"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Output *struct {
		Output string `json:"output"`
	} `json:"output"`
}

type wirePart struct {
	Type string `json:"type"`
	Time struct {
		Created float64 `json:"created"`
	} `json:"time"`
	Text  string `json:"text"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	State struct {
		Status  string          `json:"status"`
		Input   json.RawMessage `json:"input"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Metadata map[string]any `json:"metadata"`
	} `json:"state"`
}

// Messages returns every message of a session, oldest first.
func (c *Client) Messages(ctx context.Context, sessionID string) ([]json.RawMessage, error) {
	var out []json.RawMessage
	q := url.Values{"order": {"asc"}, "limit": {jsonInt(transcriptPage)}}
	for page := 0; page < transcriptMaxPages; page++ {
		var resp struct {
			Data   []json.RawMessage `json:"data"`
			Cursor struct {
				Next *string `json:"next"`
			} `json:"cursor"`
		}
		if err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "message"), q, nil, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Data...)
		if len(resp.Data) < transcriptPage || resp.Cursor.Next == nil || *resp.Cursor.Next == "" {
			break
		}
		q = url.Values{"cursor": {*resp.Cursor.Next}, "limit": {jsonInt(transcriptPage)}}
	}
	return out, nil
}

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Transcript implements adapters.Transcriber.
func (a *Adapter) Transcript(ctx context.Context, h adapters.ServerHandle, sessionID string) ([]adapters.TranscriptEntry, error) {
	c := client(h)
	parents, err := a.Children(ctx, h)
	if err != nil {
		parents = map[string]string{}
	}
	var out []adapters.TranscriptEntry
	seen := map[string]bool{}
	queue := []string{sessionID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		msgs, err := c.Messages(ctx, id)
		if err != nil {
			if id == sessionID {
				return nil, err
			}
			continue // a child session gone: its parent call still tells
		}
		parent := ""
		if id != sessionID {
			parent = parents[id]
		}
		entries := decodeTranscript(id, parent, msgs)
		for _, e := range entries {
			if e.Child != "" {
				queue = append(queue, e.Child)
			}
		}
		out = append(out, entries...)
		for child, p := range parents {
			if p == id {
				queue = append(queue, child)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}

// decodeTranscript turns the messages of one tool session into entries.
func decodeTranscript(sessionID, parentID string, msgs []json.RawMessage) []adapters.TranscriptEntry {
	var out []adapters.TranscriptEntry
	add := func(e adapters.TranscriptEntry) {
		e.SessionID, e.ParentID = sessionID, parentID
		out = append(out, e)
	}
	for _, raw := range msgs {
		var m wireMessage
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		at := msTime(m.Time.Created)
		switch m.Type {
		case "user", "synthetic", "system":
			add(adapters.TranscriptEntry{Time: at, Kind: adapters.EntryPrompt, Role: m.Type, Text: m.Text})
		case "skill":
			name := m.Skill
			if name == "" {
				name = m.Name
			}
			add(adapters.TranscriptEntry{Time: at, Kind: adapters.EntryTool, Role: "skill",
				Call: &adapters.ToolCall{Action: sessionspec.ActionSkill, Skill: name, Status: adapters.CallOK, Input: map[string]any{"name": name}}})
		case "shell":
			out1 := ""
			if m.Output != nil {
				out1 = m.Output.Output
			}
			add(adapters.TranscriptEntry{Time: at, Kind: adapters.EntryTool, Role: "user",
				Call:   &adapters.ToolCall{ID: m.ID, Action: sessionspec.ActionShell, Status: adapters.CallOK, Input: map[string]any{"command": m.Command}},
				Output: out1})
		case "idle":
			add(adapters.TranscriptEntry{Time: at, Kind: adapters.EntryEnd, Outcome: m.Outcome})
		case "assistant":
			model := m.Model.ID
			if m.Model.ProviderID != "" && model != "" {
				model = m.Model.ProviderID + "/" + model
			}
			for _, p := range m.Content {
				switch p.Type {
				case "text":
					if strings.TrimSpace(p.Text) != "" {
						add(adapters.TranscriptEntry{Time: at, Kind: adapters.EntryText, Agent: m.Agent, Model: model, Text: p.Text})
					}
				case "tool":
					add(toolEntry(at, m.Agent, model, p))
				}
			}
			if m.Error != nil && m.Error.Message != "" {
				add(adapters.TranscriptEntry{Time: msTimeOr(m.Time.Completed, at), Kind: adapters.EntryText, Agent: m.Agent, Model: model, Role: "error", Text: m.Error.Message})
			}
			add(adapters.TranscriptEntry{Time: msTimeOr(m.Time.Completed, at), Kind: adapters.EntryUsage, Agent: m.Agent, Model: model,
				Cost: m.Cost, TokensIn: m.Tokens.Input, TokensOut: m.Tokens.Output})
		}
	}
	return out
}

func toolEntry(at time.Time, agent, model string, p wirePart) adapters.TranscriptEntry {
	c := &adapters.ToolCall{ID: p.ID, Action: NeutralAction(p.Name)}
	_ = json.Unmarshal(p.State.Input, &c.Input)
	switch p.State.Status {
	case "completed":
		c.Status = adapters.CallOK
	case "error":
		c.Status = adapters.CallFailed
		if p.State.Error != nil {
			c.Error = p.State.Error.Message
		}
	default:
		c.Status = adapters.CallCalled
	}
	if c.Action == sessionspec.ActionSkill {
		for _, k := range []string{"name", "skill", "id"} {
			if v, _ := c.Input[k].(string); v != "" {
				c.Skill = v
				break
			}
		}
	}
	var text strings.Builder
	for _, ct := range p.State.Content {
		if ct.Type == "text" {
			text.WriteString(ct.Text)
		}
	}
	if p.Time.Created > 0 {
		at = msTime(p.Time.Created)
	}
	e := adapters.TranscriptEntry{Time: at, Kind: adapters.EntryTool, Agent: agent, Model: model, Call: c, Output: text.String()}
	if p.Name == toolSubagent {
		if s, _ := p.State.Metadata["sessionID"].(string); s != "" {
			e.Child = s
		}
	}
	return e
}

func msTime(ms float64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(ms)).UTC()
}

func msTimeOr(ms float64, def time.Time) time.Time {
	if ms <= 0 {
		return def
	}
	return msTime(ms)
}
