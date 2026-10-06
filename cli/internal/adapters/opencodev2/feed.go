package opencodev2

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Live feed decoding (P3-T10): opencode V2 events → tool-agnostic feed items.
// Event shapes observed on opencode 2.0.20 (session.step.started → agent,
// session.text.ended → text, session.tool.input.started → tool name,
// session.tool.called → input, session.tool.success|failed → status).

const (
	feedTextMax  = 400
	feedTitleMax = 160
	toolSubagent = "subagent"
)

// feedDecoder keeps the per-stream state needed to decode tool calls.
type feedDecoder struct {
	tools map[string]string // tool call id → tool name
}

func newFeedDecoder() *feedDecoder { return &feedDecoder{tools: map[string]string{}} }

type feedData struct {
	SessionID string          `json:"sessionID"`
	ParentID  string          `json:"parentID"`
	Agent     string          `json:"agent"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Input     json.RawMessage `json:"input"`
	Cost      float64         `json:"cost"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// decode returns the feed item of an event (nil when it is not shown) and
// the parent session of a created child session.
func (f *feedDecoder) decode(e Event) (item *domain.FeedItem, parentID string) {
	if !strings.HasPrefix(e.Type, "session.") && !strings.HasPrefix(e.Type, "permission.") && !strings.HasPrefix(e.Type, "form.") {
		return nil, ""
	}
	var d feedData
	if len(e.Data) > 0 {
		_ = json.Unmarshal(e.Data, &d)
	}
	it := &domain.FeedItem{Time: e.Time(), SessionID: d.SessionID}
	switch e.Type {
	case "session.created":
		return nil, d.ParentID
	case "session.step.started":
		if d.Agent == "" {
			return nil, ""
		}
		it.Kind, it.Agent = domain.FeedAgent, d.Agent
	case "session.text.ended":
		if strings.TrimSpace(d.Text) == "" {
			return nil, ""
		}
		it.Kind, it.Text = domain.FeedText, clip(strings.TrimSpace(d.Text), feedTextMax)
	case "session.tool.input.started":
		if d.ID != "" && d.Name != "" {
			f.tools[d.ID] = d.Name
		}
		return nil, ""
	case "session.tool.called":
		name := f.tools[d.ID]
		if name == toolSubagent {
			it.Kind, it.Agent, it.Title = domain.FeedDelegate, inputField(d.Input, "agent"), clip(inputField(d.Input, "description"), feedTitleMax)
			return it, ""
		}
		it.Kind, it.Tool, it.Title = domain.FeedTool, name, clip(toolTitle(d.Input), feedTitleMax)
	case "session.tool.success", "session.tool.failed", "session.tool.error":
		name := f.tools[d.ID]
		delete(f.tools, d.ID)
		if name == toolSubagent || name == "" {
			return nil, "" // delegation result, or a call started before the stream
		}
		it.Kind, it.Tool, it.Status = domain.FeedTool, name, "ok"
		if e.Type != "session.tool.success" {
			it.Status = "failed"
		}
	case "session.execution.started":
		it.Kind, it.Status = domain.FeedState, "started"
	case "session.usage.updated":
		it.Kind, it.Cost = domain.FeedUsage, d.Cost
	case "permission.asked":
		it.Kind, it.Title = domain.FeedDecision, "permission"
	case "form.created":
		it.Kind, it.Title = domain.FeedDecision, "question"
	default:
		if strings.HasPrefix(e.Type, "session.execution.") {
			it.Kind, it.Status = domain.FeedState, strings.TrimPrefix(e.Type, "session.execution.")
			break
		}
		return nil, ""
	}
	if it.SessionID == "" {
		it.SessionID = e.SessionID()
	}
	return it, ""
}

func inputField(raw json.RawMessage, key string) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// toolTitle summarizes a tool input: command, path, pattern, or the first
// short string value.
func toolTitle(raw json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	for _, k := range []string{"command", "filePath", "path", "file", "pattern", "query", "url", "description"} {
		if s, ok := m[k].(string); ok && s != "" {
			return oneLine(s)
		}
	}
	return ""
}

func oneLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	return s
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// callOf returns the tool call carried by a tool event (nil otherwise). It
// must run before decode, which forgets the tool name of finished calls.
func (f *feedDecoder) callOf(e Event) *adapters.ToolCall {
	switch e.Type {
	case "session.tool.called", "session.tool.success", "session.tool.failed", "session.tool.error":
	default:
		return nil
	}
	var d struct {
		ID    string          `json:"id"`
		Input json.RawMessage `json:"input"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if len(e.Data) == 0 || json.Unmarshal(e.Data, &d) != nil || d.ID == "" {
		return nil
	}
	name := f.tools[d.ID]
	if name == "" {
		return nil
	}
	c := &adapters.ToolCall{ID: d.ID, Action: NeutralAction(name)}
	switch e.Type {
	case "session.tool.called":
		c.Status = adapters.CallCalled
		_ = json.Unmarshal(d.Input, &c.Input)
	case "session.tool.success":
		c.Status = adapters.CallOK
	default:
		c.Status, c.Error = adapters.CallFailed, d.Error.Message
	}
	return c
}

// workflowTools are the oh workflow MCP tools, by opencode tool name.
var workflowTools = func() map[string]string {
	out := map[string]string{}
	for _, t := range []string{sessionspec.WorkflowToolStatus, sessionspec.WorkflowToolCheckpoint, sessionspec.WorkflowToolOutputs} {
		a := sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, t)
		out[ToolAction(a)] = a
	}
	return out
}()

// NeutralAction translates an opencode tool name to its neutral action
// (the oh workflow tools; built-in names are already neutral).
func NeutralAction(name string) string {
	if a, ok := workflowTools[name]; ok {
		return a
	}
	return name
}
