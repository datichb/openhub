// Package gitlab is the GitLab REST client of the remote runtime (v5 phase 5):
// the oh-runner project (pipeline file, CI variables, trigger), the generic
// package registry, pipelines and their artifacts.
//
// It shares no state with gitlabapi (merge requests from the CLI), tracker
// (issues) or the gitlab MCP server (agent tools).
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
)

// ErrNotFound is returned (wrapped in *APIError) for HTTP 404.
var ErrNotFound = errors.New("not found")

// ErrUnauthorized is returned (wrapped in *APIError) for HTTP 401 and 403.
var ErrUnauthorized = errors.New("unauthorized")

// APIError is a failed GitLab API call.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return fmt.Sprintf("gitlab: %s %s: HTTP %d: %s", e.Method, e.Path, e.Status, msg)
}

// Unwrap maps the status to ErrNotFound / ErrUnauthorized.
func (e *APIError) Unwrap() error {
	switch e.Status {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	}
	return nil
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsUnauthorized reports whether err is a 401/403 from the API.
func IsUnauthorized(err error) bool { return errors.Is(err, ErrUnauthorized) }

// Client is a GitLab REST v4 client authenticated by a token.
type Client struct {
	base   string // https://host (no trailing slash)
	token  string
	header string // PRIVATE-TOKEN (default) or JOB-TOKEN
	http   *http.Client
}

// New returns a client for the instance at baseURL with a personal, group or
// project access token.
func New(baseURL, token string) *Client {
	base := strings.TrimRight(baseURL, "/")
	if u, err := url.Parse(base); err == nil && u.Scheme != "https" && u.Host != "" {
		slog.Warn("remote/gitlab: token sent over a non-HTTPS connection", "url", base)
	}
	return &Client{
		base:   base,
		token:  token,
		header: "PRIVATE-TOKEN",
		http:   httplog.Wrap(&http.Client{Timeout: 60 * time.Second}, "remote-gitlab"),
	}
}

// WithJobToken authenticates with a CI job token (JOB-TOKEN header).
func (c *Client) WithJobToken() *Client {
	cp := *c
	cp.header = "JOB-TOKEN"
	return &cp
}

// WithHTTPClient replaces the HTTP client (tests, longer timeouts).
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	cp := *c
	cp.http = h
	return &cp
}

// BaseURL returns the instance URL.
func (c *Client) BaseURL() string { return c.base }

// PathID encodes a full path ("group/sub/project") as a URL path segment.
func PathID(path string) string {
	return strings.ReplaceAll(url.PathEscape(path), "/", "%2F")
}

func projectRef(project string) string { return "/projects/" + PathID(project) }

// do sends a request to /api/v4<path>. body is JSON-encoded unless it is an
// io.Reader (sent as is). out, when non-nil, receives the decoded JSON.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.send(ctx, method, path, body, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("gitlab: %s %s: reading response: %w", method, path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("gitlab: %s %s: decoding response: %w", method, path, err)
	}
	return nil
}

// send performs the request and returns the response on 2xx (caller closes).
func (c *Client) send(ctx context.Context, method, path string, body any, contentType string) (*http.Response, error) {
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case io.Reader:
		r = b
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("gitlab: encoding request: %w", err)
		}
		r = bytes.NewReader(data)
		contentType = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+"/api/v4"+path, r)
	if err != nil {
		return nil, fmt.Errorf("gitlab: building request: %w", err)
	}
	if c.token != "" {
		req.Header.Set(c.header, c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if sz, ok := body.(interface{ Len() int }); ok {
		req.ContentLength = int64(sz.Len())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %s %s: %w", method, redactPath(path), err)
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, &APIError{Method: method, Path: redactPath(path), Status: resp.StatusCode, Message: errorMessage(resp.Body)}
	}
	return resp, nil
}

func decodeJSON(r io.Reader, out any) error {
	data, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// redactPath drops the query string (it may carry a trigger token).
func redactPath(p string) string {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		return p[:i]
	}
	return p
}

// errorMessage extracts GitLab's {"message": …} or {"error": …}.
func errorMessage(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, 4096))
	var e struct {
		Message any    `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil {
		switch m := e.Message.(type) {
		case string:
			return m
		case nil:
		default:
			b, _ := json.Marshal(m)
			return string(b)
		}
		if e.Error != "" {
			return e.Error
		}
	}
	return strings.TrimSpace(string(data))
}
