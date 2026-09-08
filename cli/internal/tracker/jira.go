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

// jiraClient implements Tracker for Jira Cloud / Server.
//
// Auth: Bearer token (Jira Cloud API token) or Basic auth (Jira Server).
// The normalised state maps Jira statusCategory.key to "open" / "closed":
//   - "done"          → "closed"
//   - "new" | "indeterminate" → "open"
type jiraClient struct {
	cfg    Config
	client *http.Client
}

func newJira(cfg Config) *jiraClient {
	return &jiraClient{
		cfg:    cfg,
		client: httplog.Wrap(&http.Client{Timeout: 10 * time.Second}, "tracker.jira"),
	}
}

// ── Jira API response types ───────────────────────────────────────────────────

type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"` // e.g. "SRU-42"
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Status      struct {
			Name           string `json:"name"` // e.g. "In Progress", "Code Review"
			StatusCategory struct {
				Key string `json:"key"` // "new" | "indeterminate" | "done"
			} `json:"statusCategory"`
		} `json:"status"`
		Labels  []string `json:"labels"`
		Updated string   `json:"updated"` // ISO 8601
		Assignee *struct {
			Name        string `json:"name"`        // Jira Server username
			AccountID   string `json:"accountId"`   // Jira Cloud
			DisplayName string `json:"displayName"` // human-readable
		} `json:"assignee"`
	} `json:"fields"`
}

func (j jiraIssue) toIssueState() IssueState {
	state := "open"
	if j.Fields.Status.StatusCategory.Key == "done" {
		state = "closed"
	}

	var assignees []string
	if a := j.Fields.Assignee; a != nil {
		// Prefer name (Jira Server), fall back to displayName (Jira Cloud).
		username := a.Name
		if username == "" {
			username = a.DisplayName
		}
		if username != "" {
			assignees = []string{username}
		}
	}

	updatedAt, _ := time.Parse("2006-01-02T15:04:05.000-0700", j.Fields.Updated)

	// Extract the numeric part of the key (e.g. "SRU-42" → 42) for IID.
	iid := 0
	if parts := strings.SplitN(j.Key, "-", 2); len(parts) == 2 {
		iid, _ = strconv.Atoi(parts[1])
	}

	return IssueState{
		IID:            iid,
		Key:            j.Key,
		State:          state,
		StatusName:     j.Fields.Status.Name,
		StatusCategory: j.Fields.Status.StatusCategory.Key,
		Labels:         j.Fields.Labels,
		Assignees:      assignees,
		Title:          j.Fields.Summary,
		Description:    j.Fields.Description,
		UpdatedAt:      updatedAt,
	}
}

// ── Tracker interface ─────────────────────────────────────────────────────────

func (c *jiraClient) FetchIssue(ctx context.Context, projectID string, iid int) (*IssueState, error) {
	// Jira issue key = "<projectID>-<iid>" (e.g. "SRU-42").
	issueKey := fmt.Sprintf("%s-%d", strings.ToUpper(projectID), iid)
	path := "/rest/api/2/issue/" + url.PathEscape(issueKey)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var issue jiraIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("jira: parsing issue: %w", err)
	}
	s := issue.toIssueState()
	return &s, nil
}

func (c *jiraClient) ListAssignedIssues(ctx context.Context, projectID string, opts ListOpts) ([]IssueState, error) {
	// Build JQL query.
	jql := fmt.Sprintf(`project = "%s" AND assignee = "%s" AND statusCategory != Done`,
		projectID, opts.AssigneeUsername)
	if !opts.UpdatedAfter.IsZero() {
		jql += fmt.Sprintf(` AND updated >= "%s"`, opts.UpdatedAfter.UTC().Format("2006-01-02"))
	}

	maxResults := 20
	if opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}

	body := fmt.Sprintf(`{"jql":%q,"maxResults":%d,"fields":["summary","description","status","labels","updated","assignee"]}`,
		jql, maxResults)

	data, err := c.do(ctx, http.MethodPost, "/rest/api/2/search", strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("jira: parsing search response: %w", err)
	}

	issues := make([]IssueState, 0, len(resp.Issues))
	for _, j := range resp.Issues {
		issues = append(issues, j.toIssueState())
	}
	return issues, nil
}

func (c *jiraClient) ListUnassignedIssues(ctx context.Context, projectID string, opts ListUnassignedOpts) ([]IssueState, error) {
	// Build JQL query for unassigned issues.
	jql := fmt.Sprintf(`project = "%s" AND assignee is EMPTY AND statusCategory != Done`, projectID)
	if len(opts.Labels) > 0 {
		// JQL label filter: labels in ("label1", "label2")
		quoted := make([]string, 0, len(opts.Labels))
		for _, l := range opts.Labels {
			quoted = append(quoted, fmt.Sprintf("%q", l))
		}
		jql += fmt.Sprintf(` AND labels in (%s)`, strings.Join(quoted, ","))
	}
	if !opts.UpdatedAfter.IsZero() {
		jql += fmt.Sprintf(` AND updated >= "%s"`, opts.UpdatedAfter.UTC().Format("2006-01-02"))
	}

	maxResults := 20
	if opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}

	body := fmt.Sprintf(`{"jql":%q,"maxResults":%d,"fields":["summary","description","status","labels","updated","assignee"]}`,
		jql, maxResults)

	data, err := c.do(ctx, http.MethodPost, "/rest/api/2/search", strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("jira: parsing unassigned search response: %w", err)
	}

	issues := make([]IssueState, 0, len(resp.Issues))
	for _, j := range resp.Issues {
		issues = append(issues, j.toIssueState())
	}
	return issues, nil
}

func (c *jiraClient) AssignIssue(ctx context.Context, projectID string, iid int, username string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	issueKey := fmt.Sprintf("%s-%d", strings.ToUpper(projectID), iid)
	body := fmt.Sprintf(`{"name":%q}`, username)
	path := "/rest/api/2/issue/" + url.PathEscape(issueKey) + "/assignee"
	_, err := c.do(ctx, http.MethodPut, path, strings.NewReader(body))
	return err
}

func (c *jiraClient) TestConnection(ctx context.Context) (string, error) {
	data, err := c.do(ctx, http.MethodGet, "/rest/api/2/myself", nil)
	if err != nil {
		return "", fmt.Errorf("jira: test connexion: %w", err)
	}
	var resp struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("jira: parsing myself response: %w", err)
	}
	if resp.Name != "" {
		return resp.Name, nil
	}
	return resp.DisplayName, nil
}

func (c *jiraClient) TestProject(ctx context.Context, projectID string) (string, error) {
	data, err := c.do(ctx, http.MethodGet, "/rest/api/2/project/"+url.PathEscape(projectID), nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("jira: parsing project response: %w", err)
	}
	return fmt.Sprintf("%s (%s)", resp.Name, resp.Key), nil
}

func (c *jiraClient) AddLabels(ctx context.Context, projectID string, iid int, labels []string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	if len(labels) == 0 {
		return nil
	}
	issueKey := fmt.Sprintf("%s-%d", strings.ToUpper(projectID), iid)

	// Jira label update uses the "update" mechanism to append labels.
	addOps := make([]map[string]string, 0, len(labels))
	for _, l := range labels {
		addOps = append(addOps, map[string]string{"add": l})
	}
	payload := map[string]interface{}{
		"update": map[string]interface{}{
			"labels": addOps,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshaling label update: %w", err)
	}
	path := "/rest/api/2/issue/" + url.PathEscape(issueKey)
	_, err = c.do(ctx, http.MethodPut, path, strings.NewReader(string(body)))
	return err
}

func (c *jiraClient) RemoveLabels(ctx context.Context, projectID string, iid int, labels []string) error {
	if !c.cfg.WriteEnabled {
		return ErrWriteDisabled
	}
	if len(labels) == 0 {
		return nil
	}
	issueKey := fmt.Sprintf("%s-%d", strings.ToUpper(projectID), iid)

	removeOps := make([]map[string]string, 0, len(labels))
	for _, l := range labels {
		removeOps = append(removeOps, map[string]string{"remove": l})
	}
	payload := map[string]interface{}{
		"update": map[string]interface{}{
			"labels": removeOps,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshaling label remove: %w", err)
	}
	path := "/rest/api/2/issue/" + url.PathEscape(issueKey)
	_, err = c.do(ctx, http.MethodPut, path, strings.NewReader(string(body)))
	return err
}

func (c *jiraClient) CreateIssue(ctx context.Context, opts CreateIssueOpts) (*CreatedIssue, error) {
	if !c.cfg.WriteEnabled {
		return nil, ErrWriteDisabled
	}
	if opts.ProjectID == "" || opts.Title == "" {
		return nil, fmt.Errorf("jira: project and title are required")
	}

	issueType := opts.IssueType
	if issueType == "" {
		issueType = "Task"
	}

	fields := map[string]interface{}{
		"project":   map[string]string{"key": strings.ToUpper(opts.ProjectID)},
		"summary":   opts.Title,
		"issuetype": map[string]string{"name": issueType},
	}
	if opts.Description != "" {
		fields["description"] = opts.Description
	}
	if len(opts.Labels) > 0 {
		fields["labels"] = opts.Labels
	}
	if opts.AssignTo != "" {
		fields["assignee"] = map[string]string{"name": opts.AssignTo}
	}

	payload := map[string]interface{}{"fields": fields}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jira: marshaling create issue: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/rest/api/2/issue", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("jira: creating issue: %w", err)
	}

	var result struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("jira: parsing create response: %w", err)
	}

	// Build web URL from key
	webURL := strings.TrimRight(c.cfg.BaseURL, "/") + "/browse/" + result.Key

	// Parse IID from key (e.g., "PROJ-42" → 42)
	iid := 0
	if parts := strings.Split(result.Key, "-"); len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &iid)
	}

	return &CreatedIssue{
		ID:  iid,
		URL: webURL,
	}, nil
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (c *jiraClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	rawURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	// Jira Cloud uses Bearer; Jira Server also accepts it for API tokens.
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
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
		return nil, fmt.Errorf("jira: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// Cap response size to prevent OOM on abnormally large payloads.
	const maxResponseSize = 2 * 1024 * 1024 // 2 MB
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
}
