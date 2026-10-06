// Package opencodev2 implements the oh tool adapter for opencode V2
// (server API under /api, served by `opencode serve`).
package opencodev2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BasicUser is the fixed Basic-auth user of an opencode server.
const BasicUser = "opencode"

// Client is a typed client for the opencode V2 HTTP API.
type Client struct {
	baseURL  string
	password string
	http     *http.Client
	// stream is used for long-lived requests (SSE, wait) — no global timeout.
	stream *http.Client
}

// NewClient returns a client for the server at baseURL (e.g. http://127.0.0.1:4096).
func NewClient(baseURL, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
		stream:   &http.Client{},
	}
}

// BaseURL returns the server URL.
func (c *Client) BaseURL() string { return c.baseURL }

// APIError is a non-2xx response from the server.
type APIError struct {
	Status  int
	Tag     string
	Message string
}

func (e *APIError) Error() string {
	if e.Tag != "" {
		return fmt.Sprintf("opencode api: %d %s: %s", e.Status, e.Tag, e.Message)
	}
	return fmt.Sprintf("opencode api: %d: %s", e.Status, e.Message)
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// IsSettled reports whether err tells that a permission or form no longer
// waits for an answer (unknown, already answered or cancelled).
func IsSettled(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Status == http.StatusNotFound || strings.Contains(ae.Tag, "AlreadySettled") || strings.Contains(ae.Tag, "NotFound")
}

// IsInvalidAnswer reports whether err is a rejected form answer.
func IsInvalidAnswer(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && strings.Contains(ae.Tag, "InvalidAnswer")
}

// IsUnauthorized reports whether err is a 401 API error.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusUnauthorized
}

// ── Models ────────────────────────────────────────────────────────────────

// Info is the server information.
type Info struct {
	Version string   `json:"version"`
	PID     int      `json:"pid"`
	URLs    []string `json:"urls"`
}

// Agent as listed by the server.
type Agent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	Hidden      bool   `json:"hidden"`
	System      string `json:"system,omitempty"`
}

// Skill as listed by the server.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// MCPServer as listed by the server.
type MCPServer struct {
	Name   string `json:"name"`
	Status struct {
		Status string `json:"status"`
	} `json:"status"`
}

// ModelRef is the server model reference.
type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant,omitempty"`
}

// Rule is a permission rule.
type Rule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// Location identifies the working directory of a request or session.
type Location struct {
	Directory string `json:"directory"`
}

// Tokens is the token usage of a session.
type Tokens struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	Reasoning int64 `json:"reasoning"`
	Cache     struct {
		Read  int64 `json:"read"`
		Write int64 `json:"write"`
	} `json:"cache"`
}

// Session is a server session.
type Session struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parentID,omitempty"`
	ProjectID string    `json:"projectID"`
	Agent     string    `json:"agent"`
	Model     *ModelRef `json:"model,omitempty"`
	Title     string    `json:"title"`
	Cost      float64   `json:"cost"`
	Tokens    Tokens    `json:"tokens"`
	Outcome   string    `json:"outcome,omitempty"`
	Location  Location  `json:"location"`
	Time      struct {
		Created float64 `json:"created"`
		Updated float64 `json:"updated"`
		Idle    float64 `json:"idle,omitempty"`
	} `json:"time"`
}

// CreateSessionRequest is the body of POST /api/session.
type CreateSessionRequest struct {
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title,omitempty"`
	Agent       string    `json:"agent,omitempty"`
	Model       *ModelRef `json:"model,omitempty"`
	Location    *Location `json:"location,omitempty"`
	Permissions []Rule    `json:"permissions,omitempty"`
}

// PermissionRequest is a pending permission request of a session.
type PermissionRequest struct {
	ID        string   `json:"id"`
	SessionID string   `json:"sessionID"`
	Action    string   `json:"action"`
	Resources []string `json:"resources"`
	Save      []string `json:"save,omitempty"`
	Message   string   `json:"message,omitempty"`
	Source    *struct {
		Type      string `json:"type"`
		MessageID string `json:"messageID"`
		ID        string `json:"id"` // tool call id
	} `json:"source,omitempty"`
}

// FormOption is one choice of a form field.
type FormOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// FormField is one field of a form.
type FormField struct {
	Key         string       `json:"key"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Type        string       `json:"type"`
	Options     []FormOption `json:"options,omitempty"`
	Custom      bool         `json:"custom,omitempty"`
	Required    bool         `json:"required,omitempty"`
}

// Form is a pending form (e.g. an agent question).
type Form struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	Title     string         `json:"title"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Fields    []FormField    `json:"fields"`
}

// FileDiff is one changed file of a session.
type FileDiff struct {
	File      string `json:"file"`
	Patch     string `json:"patch"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
}

// PairingCode is a one-time browser/app connection code.
type PairingCode struct {
	Code      string `json:"code"`
	ExpiresIn int    `json:"expires_in"`
}

// ── Endpoints ─────────────────────────────────────────────────────────────

// Info returns server information.
func (c *Client) Info(ctx context.Context) (Info, error) {
	var out Info
	err := c.do(ctx, http.MethodGet, "/api/info", nil, nil, &out)
	return out, err
}

// Agents lists the agents visible at the given directory.
func (c *Client) Agents(ctx context.Context, dir string) ([]Agent, error) {
	var out struct {
		Data []Agent `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/agent", locQuery(dir), nil, &out)
	return out.Data, err
}

// Skills lists the skills visible at the given directory.
func (c *Client) Skills(ctx context.Context, dir string) ([]Skill, error) {
	var out struct {
		Data []Skill `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/skill", locQuery(dir), nil, &out)
	return out.Data, err
}

// MCP lists the MCP servers configured at the given directory.
func (c *Client) MCP(ctx context.Context, dir string) ([]MCPServer, error) {
	var out struct {
		Data []MCPServer `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/mcp", locQuery(dir), nil, &out)
	return out.Data, err
}

// CreateSession creates a session.
func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	var out struct {
		Data Session `json:"data"`
	}
	err := c.do(ctx, http.MethodPost, "/api/session", nil, req, &out)
	return out.Data, err
}

// GetSession returns a session.
func (c *Client) GetSession(ctx context.Context, id string) (Session, error) {
	var out struct {
		Data Session `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/session/"+url.PathEscape(id), nil, nil, &out)
	return out.Data, err
}

// ListSessions lists sessions (all locations when dir is empty).
func (c *Client) ListSessions(ctx context.Context, dir string) ([]Session, error) {
	var out struct {
		Data []Session `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/session", locQuery(dir), nil, &out)
	return out.Data, err
}

// ActiveSessions returns the IDs of sessions whose agent loop is running.
func (c *Client) ActiveSessions(ctx context.Context) ([]string, error) {
	var out struct {
		Data map[string]struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/session/active", nil, nil, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for id := range out.Data {
		ids = append(ids, id)
	}
	return ids, nil
}

// Prompt admits a user prompt; execution continues asynchronously.
func (c *Client) Prompt(ctx context.Context, sessionID, text string) error {
	return c.PromptWith(ctx, sessionID, text, "")
}

// PromptWith admits a user prompt with a delivery mode (steer | queue | "" = server default).
func (c *Client) PromptWith(ctx context.Context, sessionID, text, delivery string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "prompt"), nil, withDelivery(map[string]any{"text": text}, delivery), nil)
}

// Synthetic admits a synthetic (non-user) message.
func (c *Client) Synthetic(ctx context.Context, sessionID, text string) error {
	return c.SyntheticWith(ctx, sessionID, text, "")
}

// SyntheticWith admits a synthetic message with a delivery mode.
func (c *Client) SyntheticWith(ctx context.Context, sessionID, text, delivery string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "synthetic"), nil, withDelivery(map[string]any{"text": text}, delivery), nil)
}

func withDelivery(body map[string]any, delivery string) map[string]any {
	if delivery != "" {
		body["delivery"] = delivery
	}
	return body
}

// Compact asks the server to compact the session history (at the next step
// boundary when a loop runs).
func (c *Client) Compact(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "compact"), nil, map[string]any{}, nil)
}

// Fork creates a child session with a copy of the full history.
func (c *Client) Fork(ctx context.Context, sessionID string) (Session, error) {
	var out struct {
		Data Session `json:"data"`
	}
	err := c.do(ctx, http.MethodPost, sessionPath(sessionID, "fork"), nil, map[string]any{}, &out)
	return out.Data, err
}

// VCSInfo is the VCS state of a location.
type VCSInfo struct {
	Provider string `json:"provider,omitempty"`
	Branch   struct {
		Current string `json:"current,omitempty"`
		Default string `json:"default,omitempty"`
	} `json:"branch"`
}

// VCS returns the current and default branches of a location.
func (c *Client) VCS(ctx context.Context, dir string) (VCSInfo, error) {
	var out struct {
		Data VCSInfo `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/vcs", locQuery(dir), nil, &out)
	return out.Data, err
}

// Interrupt stops the running agent loop of a session.
func (c *Client) Interrupt(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "interrupt"), nil, map[string]any{}, nil)
}

// SwitchAgent changes the agent used by subsequent requests.
func (c *Client) SwitchAgent(ctx context.Context, sessionID, agent string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "agent"), nil, map[string]any{"agent": agent}, nil)
}

// SwitchModel changes the model used by subsequent requests.
func (c *Client) SwitchModel(ctx context.Context, sessionID string, model ModelRef) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "model"), nil, map[string]any{"model": model}, nil)
}

// SetEnvironment replaces the shell environment of a session.
func (c *Client) SetEnvironment(ctx context.Context, sessionID string, vars map[string]string) error {
	return c.do(ctx, http.MethodPut, sessionPath(sessionID, "environment"), nil, map[string]any{"variables": vars}, nil)
}

// Shell runs one shell command in the session working directory; the output
// arrives with the session.shell.ended event.
func (c *Client) Shell(ctx context.Context, sessionID, command string) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "shell"), nil, map[string]any{"command": command}, nil)
}

// Permissions lists pending permission requests of a session.
func (c *Client) Permissions(ctx context.Context, sessionID string) ([]PermissionRequest, error) {
	var out struct {
		Data []PermissionRequest `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "permission"), nil, nil, &out)
	return out.Data, err
}

// ReplyPermission answers a permission request (decision: once | always | reject).
func (c *Client) ReplyPermission(ctx context.Context, sessionID, requestID, decision, message string) error {
	body := map[string]any{"decision": decision}
	if message != "" {
		body["message"] = message
	}
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "permission/"+url.PathEscape(requestID)+"/reply"), nil, body, nil)
}

// SetSessionPermissions replaces the permission rules of a session.
func (c *Client) SetSessionPermissions(ctx context.Context, sessionID string, rules []Rule) error {
	if rules == nil {
		rules = []Rule{}
	}
	return c.do(ctx, http.MethodPatch, "/api/session/"+url.PathEscape(sessionID), nil, map[string]any{"permissions": rules}, nil)
}

// ToolCallInput returns the input of a tool call of a session (nil when not found).
func (c *Client) ToolCallInput(ctx context.Context, sessionID, callID string) (map[string]any, error) {
	var out struct {
		Data []struct {
			Content []struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				State struct {
					Input map[string]any `json:"input"`
				} `json:"state"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "message"), nil, nil, &out); err != nil {
		return nil, err
	}
	for i := len(out.Data) - 1; i >= 0; i-- {
		for _, p := range out.Data[i].Content {
			if p.Type == "tool" && p.ID == callID {
				return p.State.Input, nil
			}
		}
	}
	return nil, nil
}

// Forms lists pending forms (agent questions) of a session.
func (c *Client) Forms(ctx context.Context, sessionID string) ([]Form, error) {
	var out struct {
		Data []Form `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "form"), nil, nil, &out)
	return out.Data, err
}

// ReplyForm submits answers (field key → string | number | bool | []string).
func (c *Client) ReplyForm(ctx context.Context, sessionID, formID string, answer map[string]any) error {
	return c.do(ctx, http.MethodPost, sessionPath(sessionID, "form/"+url.PathEscape(formID)+"/reply"), nil, map[string]any{"answer": answer}, nil)
}

// Wait blocks until the session agent loop becomes idle (experimental API).
func (c *Client) Wait(ctx context.Context, sessionID string) error {
	return c.doWith(ctx, c.stream, http.MethodPost, "/api/experimental/session/"+url.PathEscape(sessionID)+"/wait", nil, map[string]any{}, nil)
}

// Diff returns the files changed by a session.
func (c *Client) Diff(ctx context.Context, sessionID string) ([]FileDiff, error) {
	var out struct {
		Data []FileDiff `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "diff"), nil, nil, &out)
	return out.Data, err
}

// AssistantText returns the concatenated text produced by the assistant in a session.
func (c *Client) AssistantText(ctx context.Context, sessionID string) (string, error) {
	var out struct {
		Data []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, sessionPath(sessionID, "message"), nil, nil, &out); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range out.Data {
		if m.Type != "assistant" {
			continue
		}
		for _, p := range m.Content {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
	}
	return b.String(), nil
}

// Pair returns a one-time browser/app connection code.
func (c *Client) Pair(ctx context.Context) (PairingCode, error) {
	var out PairingCode
	err := c.do(ctx, http.MethodPost, "/api/pair", nil, map[string]any{}, &out)
	return out, err
}

// PairURL returns the browser URL for a pairing code.
func (c *Client) PairURL(code string) string {
	return c.baseURL + "/auth/connect/" + url.PathEscape(code)
}

// ── Transport ─────────────────────────────────────────────────────────────

func sessionPath(id, suffix string) string {
	return "/api/session/" + url.PathEscape(id) + "/" + suffix
}

func locQuery(dir string) url.Values {
	if dir == "" {
		return nil
	}
	return url.Values{"location[directory]": {dir}}
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	return c.doWith(ctx, c.http, method, path, q, body, out)
}

func (c *Client) doWith(ctx context.Context, hc *http.Client, method, path string, q url.Values, body, out any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	c.authorize(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s: reading response: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return decodeAPIError(resp.StatusCode, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s %s: decoding response: %w", method, path, err)
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	if c.password != "" {
		req.SetBasicAuth(BasicUser, c.password)
	}
}

func decodeAPIError(status int, data []byte) error {
	ae := &APIError{Status: status}
	var payload struct {
		Tag     string `json:"_tag"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &payload) == nil && (payload.Tag != "" || payload.Message != "") {
		ae.Tag, ae.Message = payload.Tag, payload.Message
	} else {
		ae.Message = strings.TrimSpace(string(data))
		if ae.Message == "" {
			ae.Message = http.StatusText(status)
		}
	}
	return ae
}
