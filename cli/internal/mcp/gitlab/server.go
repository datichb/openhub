// Package gitlab implements the GitLab MCP server.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// EnvWriteEnabled enables the write tools of the server when set to "true".
const EnvWriteEnabled = "GITLAB_WRITE_ENABLED"

// Serve starts the GitLab MCP server.
func Serve() error {
	return newServer().Serve()
}

// newServer builds the server with its tools.
func newServer() *protocol.Server {
	server := protocol.NewServer("gitlab-mcp", "2.1.0")
	server.AcceptLegacyNames("gitlab_")

	// Read-only tools (always registered)
	registerReadTools(server)

	// Write tools (registered only if GITLAB_WRITE_ENABLED=true)
	if isWriteEnabled() {
		registerWriteTools(server)
	}

	return server
}

// isWriteEnabled checks if write operations are enabled via environment variable.
func isWriteEnabled() bool {
	return os.Getenv(EnvWriteEnabled) == "true"
}

func handleGetProject(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := gitlabAPI(fmt.Sprintf("/api/v4/projects/%s", url.PathEscape(args.ProjectID)))
	if err != nil {
		return nil, err
	}
	return toolText(string(data)), nil
}

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.gitlab")

const maxResponseSize = 50 << 20 // 50 MB

// gitlabAPI performs a GET request to the GitLab API.
func gitlabAPI(path string) ([]byte, error) {
	return gitlabRequest("GET", path, nil)
}

// gitlabAPIWrite performs a write request (POST/PUT/DELETE) to the GitLab API.
func gitlabAPIWrite(method, path string, body io.Reader) ([]byte, error) {
	return gitlabRequest(method, path, body)
}

// gitlabGet performs a GET request and also returns the response headers
// (pagination: X-Total, X-Next-Page…).
func gitlabGet(path string) ([]byte, http.Header, error) {
	return gitlabRoundTrip("GET", path, nil)
}

// gitlabRequest performs an HTTP request to the GitLab API.
func gitlabRequest(method, path string, body io.Reader) ([]byte, error) {
	data, _, err := gitlabRoundTrip(method, path, body)
	return data, err
}

func gitlabRoundTrip(method, path string, body io.Reader) ([]byte, http.Header, error) {
	token := os.Getenv("GITLAB_TOKEN")
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	if token == "" {
		return nil, nil, fmt.Errorf("GITLAB_TOKEN environment variable not set")
	}

	// Validate GITLAB_URL: must be https and not point to private networks
	if err := validateGitLabURL(baseURL); err != nil {
		return nil, nil, err
	}

	if body == nil {
		body = http.NoBody
	}

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, resp.Header, nil
}

// skipURLValidation is a package-level hook for tests only.
// Set via SetSkipURLValidationForTest in validate_test_helper_test.go (test build only).
var skipURLValidation bool

// validateGitLabURL checks that the URL is safe to send credentials to.
func validateGitLabURL(rawURL string) error {
	if skipURLValidation {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("GITLAB_URL is not a valid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("GITLAB_URL must use https:// (got %s://)", parsed.Scheme)
	}
	host := parsed.Hostname()
	if isPrivateHost(host) {
		return fmt.Errorf("GITLAB_URL must not point to a private/internal address")
	}
	return nil
}

// ── MR discussion & approval handlers ───────────────────────────────────────

func handleListMRDiscussions(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID      string  `json:"project_id"`
		MRIID          float64 `json:"mr_iid"` // JSON numbers
		UnresolvedOnly *bool   `json:"unresolved_only"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	unresolvedOnly := true
	if args.UnresolvedOnly != nil {
		unresolvedOnly = *args.UnresolvedOnly
	}

	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions",
		url.PathEscape(args.ProjectID), int(args.MRIID))
	data, err := gitlabAPI(path)
	if err != nil {
		return nil, err
	}

	// Client-side filtering: remove system notes and optionally filter unresolved.
	var all []json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return &protocol.ToolResult{ //nolint:nilerr // not a JSON array: return the raw payload
			Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
		}, nil
	}

	type note struct {
		System     bool `json:"system"`
		Resolvable bool `json:"resolvable"`
		Resolved   bool `json:"resolved"`
	}
	type discussion struct {
		Notes []note `json:"notes"`
	}

	var filtered []json.RawMessage
	for _, raw := range all {
		var d discussion
		if err := json.Unmarshal(raw, &d); err != nil {
			filtered = append(filtered, raw) // keep if unparseable
			continue
		}
		// Skip discussions with only system notes.
		hasHumanNote := false
		for _, n := range d.Notes {
			if !n.System {
				hasHumanNote = true
				break
			}
		}
		if !hasHumanNote {
			continue
		}
		// Filter by resolved status.
		if unresolvedOnly {
			hasUnresolved := false
			for _, n := range d.Notes {
				if n.Resolvable && !n.Resolved {
					hasUnresolved = true
					break
				}
			}
			if !hasUnresolved {
				continue
			}
		}
		filtered = append(filtered, raw)
	}

	result, _ := json.MarshalIndent(filtered, "", "  ")
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(result)}},
	}, nil
}

func handleGetMRApprovals(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string  `json:"project_id"`
		MRIID     float64 `json:"mr_iid"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/approvals",
		url.PathEscape(args.ProjectID), int(args.MRIID))
	data, err := gitlabAPI(path)
	if err != nil {
		// GitLab Free returns 403 for the approvals API.
		if strings.Contains(err.Error(), "403") {
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{{
					Type: "text",
					Text: `{"error": "Approvals API requires GitLab Premium or Ultimate. This project appears to be on GitLab Free."}`,
				}},
			}, nil
		}
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

// isPrivateHost checks if a hostname or IP belongs to a private/reserved range.
func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		// Not an IP literal — resolve to check
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return false // cannot resolve — let the HTTP client fail later
		}
		ip = ips[0]
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
