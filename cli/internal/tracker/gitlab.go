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
)

// gitLabClient implements Tracker for GitLab.
type gitLabClient struct {
	cfg    Config
	client *http.Client
}

func newGitLab(cfg Config) *gitLabClient {
	return &gitLabClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// ── GitLab API response types ─────────────────────────────────────────────────

type glIssue struct {
	IID       int       `json:"iid"`
	State     string    `json:"state"` // "opened" | "closed"
	Title     string    `json:"title"`
	Labels    []string  `json:"labels"`
	UpdatedAt time.Time `json:"updated_at"`
	Assignees []struct {
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
		IID:       g.IID,
		State:     state,
		Labels:    g.Labels,
		Assignees: assignees,
		Title:     g.Title,
		UpdatedAt: g.UpdatedAt,
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

	path := fmt.Sprintf("/api/v4/projects/%s/issues?%s", url.PathEscape(projectID), q.Encode())
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var raw []glIssue
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("gitlab: parsing issues list: %w", err)
	}
	issues := make([]IssueState, 0, len(raw))
	for _, g := range raw {
		issues = append(issues, g.toIssueState())
	}
	return issues, nil
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

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (c *gitLabClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	rawURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: building request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrTokenInvalid
	case http.StatusNotFound:
		return nil, ErrIssueNotFound
	case http.StatusTooManyRequests:
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &ErrRateLimited{RetryAfter: ra}
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gitlab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// Cap response size to prevent OOM on abnormally large payloads.
	const maxResponseSize = 2 * 1024 * 1024 // 2 MB
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
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
