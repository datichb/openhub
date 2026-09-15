package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
)

// gitLabClient implements Tracker for GitLab.
type gitLabClient struct {
	cfg    Config
	client *http.Client
}

func newGitLab(cfg Config) *gitLabClient {
	return &gitLabClient{
		cfg:    cfg,
		client: httplog.Wrap(&http.Client{Timeout: 10 * time.Second}, "tracker.gitlab"),
	}
}

// ── GitLab API response types ─────────────────────────────────────────────────

type glIssue struct {
	IID         int       `json:"iid"`
	State       string    `json:"state"` // "opened" | "closed"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Labels      []string  `json:"labels"`
	UpdatedAt   time.Time `json:"updated_at"`
	Assignees   []struct {
		Username string `json:"username"`
	} `json:"assignees"`
}

func (g glIssue) toIssueState() IssueState {
	assignees := make([]string, 0, len(g.Assignees))
	for _, a := range g.Assignees {
		assignees = append(assignees, a.Username)
	}
	state := "open"
	if g.State == "closed" {
		state = "closed"
	}
	return IssueState{
		IID:            g.IID,
		State:          state,
		StatusName:     g.State,
		StatusCategory: g.State,
		Labels:         g.Labels,
		Assignees:      assignees,
		Title:          g.Title,
		Description:    g.Description,
		UpdatedAt:      g.UpdatedAt,
	}
}

// ── Tracker interface ─────────────────────────────────────────────────────────

func (c *gitLabClient) FetchIssue(ctx context.Context, projectID string, iid int) (*IssueState, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d", url.PathEscape(projectID), iid)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var issue glIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("gitlab: parsing issue: %w", err)
	}
	s := issue.toIssueState()
	return &s, nil
}

func (c *gitLabClient) ListAssignedIssues(ctx context.Context, projectID string, opts ListOpts) ([]IssueState, error) {
	q := url.Values{}
	q.Set("state", "opened")
	q.Set("assignee_username", opts.AssigneeUsername)
	if !opts.UpdatedAfter.IsZero() {
		q.Set("updated_after", opts.UpdatedAfter.UTC().Format(time.RFC3339))
	}
	perPage := 20
	if opts.MaxResults > 0 && opts.MaxResults < perPage {
		perPage = opts.MaxResults
	}
	q.Set("per_page", strconv.Itoa(perPage))

	basePath := fmt.Sprintf("/api/v4/projects/%s/issues?%s", url.PathEscape(projectID), q.Encode())
	return c.paginatedListIssues(ctx, basePath, opts.MaxResults)
}

func (c *gitLabClient) ListUnassignedIssues(ctx context.Context, projectID string, opts ListUnassignedOpts) ([]IssueState, error) {
	q := url.Values{}
	q.Set("state", "opened")
	q.Set("assignee_id", "0") // GitLab: 0 = no assignee
	if len(opts.Labels) > 0 {
		q.Set("labels", strings.Join(opts.Labels, ","))
	}
	if !opts.UpdatedAfter.IsZero() {
		q.Set("updated_after", opts.UpdatedAfter.UTC().Format(time.RFC3339))
	}
	perPage := 20
	if opts.MaxResults > 0 && opts.MaxResults < perPage {
		perPage = opts.MaxResults
	}
	q.Set("per_page", strconv.Itoa(perPage))

	basePath := fmt.Sprintf("/api/v4/projects/%s/issues?%s", url.PathEscape(projectID), q.Encode())
	return c.paginatedListIssues(ctx, basePath, opts.MaxResults)
}

func (c *gitLabClient) ListIssuesByLabels(ctx context.Context, projectID string, opts ListByLabelsOpts) ([]IssueState, error) {
	q := url.Values{}
	q.Set("state", "opened")
	if len(opts.Labels) > 0 {
		q.Set("labels", strings.Join(opts.Labels, ","))
	}
	// No assignee filter — fetch all open issues with the given labels.
	if !opts.UpdatedAfter.IsZero() {
		q.Set("updated_after", opts.UpdatedAfter.UTC().Format(time.RFC3339))
	}
	perPage := 20
	if opts.MaxResults > 0 && opts.MaxResults < perPage {
		perPage = opts.MaxResults
	}
	q.Set("per_page", strconv.Itoa(perPage))

	basePath := fmt.Sprintf("/api/v4/projects/%s/issues?%s", url.PathEscape(projectID), q.Encode())
	return c.paginatedListIssues(ctx, basePath, opts.MaxResults)
}

func (c *gitLabClient) AssignIssue(ctx context.Context, projectID string, iid int, username string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	// Resolve username → user ID via GitLab users API.
	userPath := fmt.Sprintf("/api/v4/users?username=%s", url.QueryEscape(username))
	data, err := c.do(ctx, http.MethodGet, userPath, nil)
	if err != nil {
		return fmt.Errorf("gitlab: resolving user %q: %w", username, err)
	}
	var users []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("gitlab: parsing user lookup: %w", err)
	}
	if len(users) == 0 {
		return fmt.Errorf("gitlab: user %q not found", username)
	}

	body := fmt.Sprintf(`{"assignee_ids":[%d]}`, users[0].ID)
	issuePath := fmt.Sprintf("/api/v4/projects/%s/issues/%d", url.PathEscape(projectID), iid)
	_, err = c.do(ctx, http.MethodPut, issuePath, strings.NewReader(body))
	return err
}

func (c *gitLabClient) TestConnection(ctx context.Context) (string, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/v4/user", nil)
	if err != nil {
		return "", fmt.Errorf("gitlab: test connexion: %w", err)
	}
	var resp struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("gitlab: parsing user response: %w", err)
	}
	return resp.Username, nil
}

func (c *gitLabClient) TestProject(ctx context.Context, projectID string) (string, error) {
	path := fmt.Sprintf("/api/v4/projects/%s", url.PathEscape(projectID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Name              string `json:"name"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("gitlab: parsing project response: %w", err)
	}
	name := resp.PathWithNamespace
	if name == "" {
		name = resp.Name
	}
	return name, nil
}

func (c *gitLabClient) AddLabels(ctx context.Context, projectID string, iid int, labels []string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	if len(labels) == 0 {
		return nil
	}
	body := fmt.Sprintf(`{"add_labels":%q}`, strings.Join(labels, ","))
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d", url.PathEscape(projectID), iid)
	_, err := c.do(ctx, http.MethodPut, path, strings.NewReader(body))
	return err
}

func (c *gitLabClient) RemoveLabels(ctx context.Context, projectID string, iid int, labels []string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	if len(labels) == 0 {
		return nil
	}
	body := fmt.Sprintf(`{"remove_labels":%q}`, strings.Join(labels, ","))
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d", url.PathEscape(projectID), iid)
	_, err := c.do(ctx, http.MethodPut, path, strings.NewReader(body))
	return err
}

func (c *gitLabClient) CreateIssue(ctx context.Context, opts CreateIssueOpts) (*CreatedIssue, error) {
	if !c.cfg.WriteEnabled {
		return nil, ErrWriteDisabled
	}
	if opts.ProjectID == "" || opts.Title == "" {
		return nil, fmt.Errorf("gitlab: project and title are required")
	}

	payload := map[string]interface{}{
		"title": opts.Title,
	}
	if opts.Description != "" {
		payload["description"] = opts.Description
	}
	if len(opts.Labels) > 0 {
		payload["labels"] = strings.Join(opts.Labels, ",")
	}
	if opts.AssignTo != "" {
		payload["assignee_username"] = opts.AssignTo
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gitlab: marshaling create issue: %w", err)
	}

	apiPath := fmt.Sprintf("/api/v4/projects/%s/issues", url.PathEscape(opts.ProjectID))
	respBody, err := c.do(ctx, http.MethodPost, apiPath, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("gitlab: creating issue: %w", err)
	}

	var result struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("gitlab: parsing create response: %w", err)
	}

	return &CreatedIssue{
		ID:  result.IID,
		URL: result.WebURL,
	}, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *gitLabClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	data, _, err := c.doWithHeaders(ctx, method, path, body)
	return data, err
}

// doWithHeaders is like do but also returns response headers.
// Used by the pagination helper to read X-Next-Page.
func (c *gitLabClient) doWithHeaders(ctx context.Context, method, path string, body io.Reader) ([]byte, http.Header, error) {
	rawURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("gitlab: building request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, nil, ErrTokenInvalid
	case http.StatusNotFound:
		return nil, nil, ErrIssueNotFound
	case http.StatusTooManyRequests:
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, nil, &ErrRateLimited{RetryAfter: ra}
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, nil, fmt.Errorf("gitlab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// Cap response size to prevent OOM on abnormally large payloads.
	const maxResponseSize = 2 * 1024 * 1024 // 2 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	return data, resp.Header, err
}

// paginatedListIssues fetches all pages of a GitLab issues list endpoint
// until maxResults is reached or there are no more pages.
// basePath must include the query string (e.g. "/api/v4/projects/.../issues?state=opened&...").
func (c *gitLabClient) paginatedListIssues(ctx context.Context, basePath string, maxResults int) ([]IssueState, error) {
	var all []IssueState
	page := 1

	for {
		sep := "&"
		if !strings.Contains(basePath, "?") {
			sep = "?"
		}
		pagePath := fmt.Sprintf("%s%spage=%d", basePath, sep, page)

		data, headers, err := c.doWithHeaders(ctx, http.MethodGet, pagePath, nil)
		if err != nil {
			return all, err
		}

		var raw []glIssue
		if err := json.Unmarshal(data, &raw); err != nil {
			return all, fmt.Errorf("gitlab: parsing issues list: %w", err)
		}
		for _, g := range raw {
			all = append(all, g.toIssueState())
		}

		// Stop if we've reached the requested max.
		if maxResults > 0 && len(all) >= maxResults {
			all = all[:maxResults]
			break
		}

		// Stop if there are no more pages.
		nextPage := headers.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
		np, err := strconv.Atoi(nextPage)
		if err != nil || np <= page {
			break
		}
		page = np
	}

	return all, nil
}

// parseRetryAfter parses the Retry-After header value (seconds as integer).
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}
