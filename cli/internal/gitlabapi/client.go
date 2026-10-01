// Package gitlabapi provides a lightweight GitLab API client for CLI operations
// such as merge request creation and reviewer assignment. It is designed to
// complement (not replace) the tracker and MCP GitLab clients, sharing no state
// with them.
//
// The tracker package owns issue lifecycle reconciliation; the MCP gitlab
// package owns agent-facing tools. This package owns CLI-initiated GitLab
// write operations (e.g., oh review --publish).
package gitlabapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
)

// Client is a lightweight GitLab API client for merge-request operations.
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewClient creates a new GitLab API client.
// baseURL is the GitLab instance URL (e.g. "https://gitlab.com").
// token is a personal/project access token with api scope.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "gitlabapi"),
	}
}

// MRInfo holds the result of creating or finding a merge request.
type MRInfo struct {
	IID    int    `json:"iid"`
	WebURL string `json:"web_url"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// CreateMR creates a merge request or returns an existing open one for the same branch.
// It is idempotent: if an MR already exists for sourceBranch, it returns that MR.
func (c *Client) CreateMR(ctx context.Context, projectID, sourceBranch, targetBranch, title, description string) (*MRInfo, error) {
	encoded := url.PathEscape(projectID)

	// Check for existing open MR on the same source branch.
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened",
		encoded, url.QueryEscape(sourceBranch))
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("checking existing MR: %w", err)
	}
	var existing []MRInfo
	if err := json.Unmarshal(data, &existing); err == nil && len(existing) > 0 {
		return &existing[0], nil // return existing MR
	}

	// Create new MR.
	body := map[string]interface{}{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"title":         title,
	}
	if description != "" {
		body["description"] = description
	}
	payload, _ := json.Marshal(body)
	respData, err := c.post(ctx, fmt.Sprintf("/api/v4/projects/%s/merge_requests", encoded), payload)
	if err != nil {
		return nil, fmt.Errorf("creating MR: %w", err)
	}
	var mr MRInfo
	if err := json.Unmarshal(respData, &mr); err != nil {
		return nil, fmt.Errorf("parsing MR response: %w", err)
	}
	return &mr, nil
}

// AssignReviewers sets the reviewer_ids on an existing merge request.
func (c *Client) AssignReviewers(ctx context.Context, projectID string, mrIID int, reviewerIDs []int) error {
	encoded := url.PathEscape(projectID)
	body := map[string]interface{}{
		"reviewer_ids": reviewerIDs,
	}
	payload, _ := json.Marshal(body)
	_, err := c.put(ctx, fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", encoded, mrIID), payload)
	if err != nil {
		return fmt.Errorf("assigning reviewers: %w", err)
	}
	return nil
}

// ResolveUserID resolves a GitLab username to a numeric user ID.
// Returns 0 and an error if the username is not found.
func (c *Client) ResolveUserID(ctx context.Context, username string) (int, error) {
	path := fmt.Sprintf("/api/v4/users?username=%s", url.QueryEscape(username))
	data, err := c.get(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("resolving user %q: %w", username, err)
	}
	var users []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &users); err != nil {
		return 0, fmt.Errorf("parsing user response: %w", err)
	}
	if len(users) == 0 {
		return 0, fmt.Errorf("GitLab user %q not found", username)
	}
	return users[0].ID, nil
}

// --- HTTP helpers ---

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

func (c *Client) put(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPut, path, body)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	rawURL := c.baseURL + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("authentication failed (HTTP %d) — check your GitLab token has 'api' scope", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	const maxResponseSize = 10 * 1024 * 1024 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	return data, err
}
