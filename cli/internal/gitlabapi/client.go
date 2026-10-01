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
	"log/slog"
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
	trimmed := strings.TrimRight(baseURL, "/")

	// Warn if the token will be sent over a non-HTTPS connection.
	if trimmed != "" {
		if u, err := url.Parse(trimmed); err == nil && u.Scheme != "https" {
			slog.Warn("gitlabapi: PRIVATE-TOKEN will be sent over non-HTTPS connection — token may be exposed in transit", "url", trimmed)
		}
	}

	return &Client{
		baseURL: trimmed,
		token:   token,
		client:  httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "gitlabapi"),
	}
}

// MRInfo holds the result of creating or finding a merge request.
type MRInfo struct {
	IID          int    `json:"iid"`
	WebURL       string `json:"web_url"`
	Title        string `json:"title"`
	State        string `json:"state"`
	TargetBranch string `json:"target_branch"`
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

// ── MR discussion types ─────────────────────────────────────────────────────

// Discussion represents a GitLab MR discussion thread.
type Discussion struct {
	ID    string `json:"id"`
	Notes []Note `json:"notes"`
}

// Note represents a single note in a discussion.
type Note struct {
	ID         int      `json:"id"`
	Body       string   `json:"body"`
	Author     Author   `json:"author"`
	CreatedAt  string   `json:"created_at"`
	System     bool     `json:"system"`
	Resolvable bool     `json:"resolvable"`
	Resolved   bool     `json:"resolved"`
	Position   *NotePos `json:"position,omitempty"`
}

// Author represents a GitLab user reference.
type Author struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

// NotePos represents the position of an inline code comment.
type NotePos struct {
	NewPath string `json:"new_path"`
	NewLine int    `json:"new_line"`
	OldPath string `json:"old_path"`
	OldLine int    `json:"old_line"`
}

// ── MR discussion methods ───────────────────────────────────────────────────

// FindMRByBranch finds the first open merge request for the given source branch.
// Returns nil (no error) if no open MR exists.
func (c *Client) FindMRByBranch(ctx context.Context, projectID, branch string) (*MRInfo, error) {
	encoded := url.PathEscape(projectID)
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened",
		encoded, url.QueryEscape(branch))
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("finding MR by branch: %w", err)
	}
	var mrs []MRInfo
	if err := json.Unmarshal(data, &mrs); err != nil {
		return nil, fmt.Errorf("parsing MR list: %w", err)
	}
	if len(mrs) == 0 {
		return nil, nil
	}
	return &mrs[0], nil
}

// ListMRDiscussions returns all discussion threads for a merge request.
// System notes are excluded. If unresolvedOnly is true, only unresolved
// resolvable discussions are returned.
func (c *Client) ListMRDiscussions(ctx context.Context, projectID string, mrIID int, unresolvedOnly bool) ([]Discussion, error) {
	encoded := url.PathEscape(projectID)
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions", encoded, mrIID)
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("listing MR discussions: %w", err)
	}
	var all []Discussion
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("parsing discussions: %w", err)
	}

	// Filter: remove system notes and optionally keep only unresolved threads.
	var filtered []Discussion
	for _, d := range all {
		var notes []Note
		for _, n := range d.Notes {
			if n.System {
				continue
			}
			notes = append(notes, n)
		}
		if len(notes) == 0 {
			continue
		}
		d.Notes = notes

		// For unresolved-only filtering, check if the first resolvable note is unresolved.
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

		filtered = append(filtered, d)
	}
	return filtered, nil
}

// ReplyToDiscussion posts a reply note on a specific MR discussion thread.
func (c *Client) ReplyToDiscussion(ctx context.Context, projectID string, mrIID int, discussionID, body string) error {
	encoded := url.PathEscape(projectID)
	payload, _ := json.Marshal(map[string]string{"body": body})
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions/%s/notes",
		encoded, mrIID, url.PathEscape(discussionID))
	_, err := c.post(ctx, path, payload)
	if err != nil {
		return fmt.Errorf("replying to discussion: %w", err)
	}
	return nil
}
