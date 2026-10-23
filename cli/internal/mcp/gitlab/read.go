package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

const (
	// defaultPerPage is the size of a page of a list tool; a response is
	// read by a model, so a page stays short (the next one is one call away).
	defaultPerPage = 20
	maxPerPage     = 100
	// defaultNotesLimit and maxNotesLimit bound the comments of get_issue.
	defaultNotesLimit = 20
	maxNotesLimit     = 100
)

type obj = map[string]interface{}

// registerReadTools adds the read-only tools (always registered).
func registerReadTools(server *protocol.Server) {
	server.RegisterTool(protocol.Tool{
		Name:        "get_project",
		Description: "Get a GitLab project by ID or path",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id": obj{"type": "string", "description": "Project ID or URL-encoded path"},
			},
			"required": []string{"project_id"},
		},
	}, handleGetProject)

	server.RegisterTool(protocol.Tool{
		Name: "get_issue",
		Description: "Read one issue by its IID (the number shown as #<iid>): title, description, state, labels, assignees, author, milestone, " +
			"web_url, related issues, related merge requests and the first user comments (system notes excluded). " +
			"Use it when a ticket is given as #<iid> or as an issue URL.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id":    obj{"type": "string", "description": "Project ID or path (group/project)"},
				"iid":           obj{"type": "integer", "description": "Issue IID: the number after # (623 for #623)"},
				"include_notes": obj{"type": "boolean", "description": "Include the user comments (default: true)"},
				"notes_limit":   obj{"type": "integer", "description": fmt.Sprintf("Maximum number of comments, oldest first (default: %d, max: %d)", defaultNotesLimit, maxNotesLimit)},
			},
			"required": []string{"project_id", "iid"},
		},
	}, handleGetIssue)

	server.RegisterTool(protocol.Tool{
		Name: "list_issues",
		Description: "List the issues of a project, one page at a time. Each issue is a summary (no description: read it with get_issue). " +
			"The result tells the page, the total and the next page. To find a ticket given as #<iid>, pass iids instead of paging through the list.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id":        obj{"type": "string", "description": "Project ID or path (group/project)"},
				"iids":              obj{"type": "array", "items": obj{"type": "integer"}, "description": "Only these issue IIDs (the numbers after #)"},
				"search":            obj{"type": "string", "description": "Search the title and the description"},
				"assignee_username": obj{"type": "string", "description": "Only the issues assigned to this username"},
				"labels":            obj{"type": "string", "description": "Comma-separated labels; the issue must have all of them"},
				"state":             obj{"type": "string", "description": "Filter by state (opened, closed, all)"},
				"page":              obj{"type": "integer", "description": "Page number, from 1 (default: 1)"},
				"per_page":          obj{"type": "integer", "description": fmt.Sprintf("Issues per page (default: %d, max: %d)", defaultPerPage, maxPerPage)},
			},
			"required": []string{"project_id"},
		},
	}, handleListIssues)

	server.RegisterTool(protocol.Tool{
		Name: "get_mr",
		Description: "Read one merge request by its IID (the number shown as !<iid>): title, description, state, branches, author, assignees, reviewers, labels, " +
			"merge status, web_url and the issues it closes. The review threads are read with list_mr_discussions.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id": obj{"type": "string", "description": "Project ID or path (group/project)"},
				"mr_iid":     obj{"type": "integer", "description": "Merge request IID: the number after ! (12 for !12)"},
			},
			"required": []string{"project_id", "mr_iid"},
		},
	}, handleGetMR)

	server.RegisterTool(protocol.Tool{
		Name: "list_mrs",
		Description: "List the merge requests of a project, one page at a time (summaries: read one with get_mr). " +
			"The result tells the page, the total and the next page.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id":    obj{"type": "string", "description": "Project ID or path (group/project)"},
				"iids":          obj{"type": "array", "items": obj{"type": "integer"}, "description": "Only these merge request IIDs (the numbers after !)"},
				"search":        obj{"type": "string", "description": "Search the title and the description"},
				"source_branch": obj{"type": "string", "description": "Only the merge requests from this branch"},
				"state":         obj{"type": "string", "description": "Filter by state (opened, merged, closed, all)"},
				"page":          obj{"type": "integer", "description": "Page number, from 1 (default: 1)"},
				"per_page":      obj{"type": "integer", "description": fmt.Sprintf("Merge requests per page (default: %d, max: %d)", defaultPerPage, maxPerPage)},
			},
			"required": []string{"project_id"},
		},
	}, handleListMRs)

	server.RegisterTool(protocol.Tool{
		Name:        "list_mr_discussions",
		Description: "List discussion threads on a merge request. Returns inline code comments and general discussions with author, body, resolved status, and file position. System notes are excluded.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id":      obj{"type": "string", "description": "Project ID or URL-encoded path"},
				"mr_iid":          obj{"type": "integer", "description": "Merge request IID (internal ID)"},
				"unresolved_only": obj{"type": "boolean", "description": "Return only unresolved discussions (default: true)"},
			},
			"required": []string{"project_id", "mr_iid"},
		},
	}, handleListMRDiscussions)

	server.RegisterTool(protocol.Tool{
		Name:        "get_mr_approvals",
		Description: "Get approval status for a merge request. Returns who approved, how many approvals are required, and how many remain. Note: requires GitLab Premium or Ultimate — returns an explicit message on GitLab Free.",
		InputSchema: obj{
			"type": "object",
			"properties": obj{
				"project_id": obj{"type": "string", "description": "Project ID or URL-encoded path"},
				"mr_iid":     obj{"type": "integer", "description": "Merge request IID (internal ID)"},
			},
			"required": []string{"project_id", "mr_iid"},
		},
	}, handleGetMRApprovals)
}

// iidValue is an IID given as a number, "623" or "#623" (the forms a model
// copies from a ticket reference).
type iidValue int

func (v *iidValue) UnmarshalJSON(data []byte) error {
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*v = iidValue(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("an IID is a number, got %s", string(data))
	}
	i, err := strconv.Atoi(strings.TrimLeft(strings.TrimSpace(s), "#!"))
	if err != nil {
		return fmt.Errorf("an IID is a number, got %q", s)
	}
	*v = iidValue(i)
	return nil
}

func requireProjectAndIID(project, what string, iid int) error {
	if strings.TrimSpace(project) == "" {
		return errors.New("project_id is required (a project ID or its path, group/project)")
	}
	if iid <= 0 {
		return fmt.Errorf("%s is required (the number after # or !)", what)
	}
	return nil
}

func toolText(text string) *protocol.ToolResult {
	return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text", Text: text}}}
}

func toolJSON(v any) (*protocol.ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return toolText(string(data)), nil
}

// notFound explains a 404 of a single-resource read: the project or the IID
// is wrong, or the token does not see it (GitLab answers 404 to hide it).
func notFound(err error, what, project string) error {
	if err != nil && strings.Contains(err.Error(), "GitLab API error 404") {
		return fmt.Errorf("%s not found in project %q (check project_id and the IID; a private project the token cannot see also answers 404): %w", what, project, err)
	}
	return err
}

// ── API shapes ──────────────────────────────────────────────────────────────

type userRef struct {
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
}

func usernames(us []userRef) []string {
	out := make([]string, 0, len(us))
	for _, u := range us {
		out = append(out, u.Username)
	}
	return out
}

type milestoneRef struct {
	Title   string `json:"title"`
	DueDate string `json:"due_date,omitempty"`
	State   string `json:"state,omitempty"`
}

type apiIssue struct {
	IID            int           `json:"iid"`
	ProjectID      int           `json:"project_id"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	State          string        `json:"state"`
	Type           string        `json:"issue_type"`
	Labels         []string      `json:"labels"`
	Assignees      []userRef     `json:"assignees"`
	Author         userRef       `json:"author"`
	Milestone      *milestoneRef `json:"milestone"`
	WebURL         string        `json:"web_url"`
	CreatedAt      string        `json:"created_at"`
	UpdatedAt      string        `json:"updated_at"`
	ClosedAt       string        `json:"closed_at"`
	DueDate        string        `json:"due_date"`
	Confidential   bool          `json:"confidential"`
	UserNotesCount int           `json:"user_notes_count"`
	References     struct {
		Full string `json:"full"`
	} `json:"references"`
}

type issueSummary struct {
	IID          int      `json:"iid"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Labels       []string `json:"labels"`
	Assignees    []string `json:"assignees"`
	Author       string   `json:"author,omitempty"`
	Milestone    string   `json:"milestone,omitempty"`
	Confidential bool     `json:"confidential,omitempty"`
	WebURL       string   `json:"web_url"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

func (i apiIssue) summary() issueSummary {
	s := issueSummary{IID: i.IID, Title: i.Title, State: i.State, Labels: nonNil(i.Labels), Assignees: usernames(i.Assignees),
		Author: i.Author.Username, Confidential: i.Confidential, WebURL: i.WebURL, UpdatedAt: i.UpdatedAt}
	if i.Milestone != nil {
		s.Milestone = i.Milestone.Title
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type noteView struct {
	ID        int    `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
}

type relatedIssue struct {
	IID      int    `json:"iid"`
	Title    string `json:"title"`
	State    string `json:"state"`
	LinkType string `json:"link_type,omitempty"`
	WebURL   string `json:"web_url"`
	Project  int    `json:"project_id,omitempty"`
}

type relatedMR struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch,omitempty"`
	WebURL       string `json:"web_url"`
}

// issueView is what get_issue returns. The lookups next to the issue
// (comments, links) are best effort: a failure is said in <x>_error, never
// dropped, and the issue is still returned.
type issueView struct {
	IID            int            `json:"iid"`
	Title          string         `json:"title"`
	State          string         `json:"state"`
	Type           string         `json:"type,omitempty"`
	Description    string         `json:"description"`
	Labels         []string       `json:"labels"`
	Assignees      []userRef      `json:"assignees"`
	Author         userRef        `json:"author"`
	Milestone      *milestoneRef  `json:"milestone,omitempty"`
	Reference      string         `json:"reference,omitempty"`
	WebURL         string         `json:"web_url"`
	Confidential   bool           `json:"confidential,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
	ClosedAt       string         `json:"closed_at,omitempty"`
	DueDate        string         `json:"due_date,omitempty"`
	Notes          []noteView     `json:"notes,omitempty"`
	NotesTruncated bool           `json:"notes_truncated,omitempty"`
	NotesError     string         `json:"notes_error,omitempty"`
	RelatedIssues  []relatedIssue `json:"related_issues"`
	RelatedMRs     []relatedMR    `json:"related_merge_requests"`
	LinksError     string         `json:"links_error,omitempty"`
}

func projectPath(project, tail string, args ...any) string {
	return fmt.Sprintf("/api/v4/projects/%s/", url.PathEscape(project)) + fmt.Sprintf(tail, args...)
}

func clampNotesLimit(n *int) int {
	switch {
	case n == nil || *n <= 0:
		return defaultNotesLimit
	case *n > maxNotesLimit:
		return maxNotesLimit
	}
	return *n
}

func handleGetIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID    string   `json:"project_id"`
		IID          iidValue `json:"iid"`
		IssueIID     iidValue `json:"issue_iid"` // the name update_issue uses
		IncludeNotes *bool    `json:"include_notes"`
		NotesLimit   *int     `json:"notes_limit"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	iid := int(args.IID)
	if iid == 0 {
		iid = int(args.IssueIID)
	}
	if err := requireProjectAndIID(args.ProjectID, "iid", iid); err != nil {
		return nil, err
	}

	data, err := gitlabAPI(projectPath(args.ProjectID, "issues/%d", iid))
	if err != nil {
		return nil, notFound(err, fmt.Sprintf("issue #%d", iid), args.ProjectID)
	}
	var issue apiIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("unexpected GitLab answer for issue #%d: %w", iid, err)
	}
	view := issueView{
		IID: issue.IID, Title: issue.Title, State: issue.State, Type: issue.Type, Description: issue.Description,
		Labels: nonNil(issue.Labels), Assignees: issue.Assignees, Author: issue.Author, Milestone: issue.Milestone,
		Reference: issue.References.Full, WebURL: issue.WebURL, Confidential: issue.Confidential,
		CreatedAt: issue.CreatedAt, UpdatedAt: issue.UpdatedAt, ClosedAt: issue.ClosedAt, DueDate: issue.DueDate,
		RelatedIssues: []relatedIssue{}, RelatedMRs: []relatedMR{},
	}
	if args.IncludeNotes == nil || *args.IncludeNotes {
		view.Notes, view.NotesTruncated, view.NotesError = issueNotes(args.ProjectID, iid, clampNotesLimit(args.NotesLimit))
	}
	var linkErrs []string
	if err := getInto(projectPath(args.ProjectID, "issues/%d/links", iid), &view.RelatedIssues); err != nil {
		linkErrs = append(linkErrs, "links: "+err.Error())
	}
	if err := getInto(projectPath(args.ProjectID, "issues/%d/related_merge_requests", iid), &view.RelatedMRs); err != nil {
		linkErrs = append(linkErrs, "related_merge_requests: "+err.Error())
	}
	view.LinksError = strings.Join(linkErrs, "; ")
	return toolJSON(view)
}

// issueNotes reads the first user comments of an issue (oldest first); system
// notes (assigned, labelled…) are left out.
func issueNotes(project string, iid, limit int) (notes []noteView, truncated bool, errText string) {
	q := url.Values{"sort": {"asc"}, "order_by": {"created_at"}, "per_page": {strconv.Itoa(maxPerPage)}}
	data, header, err := gitlabGet(projectPath(project, "issues/%d/notes?", iid) + q.Encode())
	if err != nil {
		return nil, false, err.Error()
	}
	var raw []struct {
		ID        int     `json:"id"`
		Body      string  `json:"body"`
		System    bool    `json:"system"`
		Author    userRef `json:"author"`
		CreatedAt string  `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false, "unexpected GitLab answer: " + err.Error()
	}
	for _, n := range raw {
		if n.System {
			continue
		}
		if len(notes) == limit {
			truncated = true
			break
		}
		notes = append(notes, noteView{ID: n.ID, Author: n.Author.Username, Body: n.Body, CreatedAt: n.CreatedAt})
	}
	if header.Get("X-Next-Page") != "" {
		truncated = true
	}
	return notes, truncated, ""
}

// getInto reads a JSON answer into v.
func getInto(path string, v any) error {
	data, err := gitlabAPI(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unexpected GitLab answer: %w", err)
	}
	return nil
}

// pageResult is the answer of a list tool: the items of the page and where
// it stands in the list.
type pageResult[T any] struct {
	items      []T
	page       int
	perPage    int
	total      *int
	totalPages *int
	nextPage   *int
}

func headerInt(h http.Header, key string) *int {
	n, err := strconv.Atoi(h.Get(key))
	if err != nil {
		return nil
	}
	return &n
}

// listPage reads one page of a collection; q holds the filters.
func listPage[T any](path string, q url.Values, page, perPage *int) (*pageResult[T], error) {
	p, pp := 1, defaultPerPage
	if page != nil && *page > 0 {
		p = *page
	}
	if perPage != nil && *perPage > 0 {
		pp = min(*perPage, maxPerPage)
	}
	q.Set("page", strconv.Itoa(p))
	q.Set("per_page", strconv.Itoa(pp))
	data, header, err := gitlabGet(path + "?" + q.Encode())
	if err != nil {
		return nil, err
	}
	res := &pageResult[T]{page: p, perPage: pp}
	if err := json.Unmarshal(data, &res.items); err != nil {
		return nil, fmt.Errorf("unexpected GitLab answer: %w", err)
	}
	if n := headerInt(header, "X-Page"); n != nil {
		res.page = *n
	}
	// GitLab omits the totals past 10 000 results: null, not 0.
	res.total = headerInt(header, "X-Total")
	res.totalPages = headerInt(header, "X-Total-Pages")
	res.nextPage = headerInt(header, "X-Next-Page")
	return res, nil
}

// listPayload renders a page under key, with its position in the list.
func listPayload[T, V any](key string, res *pageResult[T], view func(T) V) obj {
	items := make([]V, 0, len(res.items))
	for _, it := range res.items {
		items = append(items, view(it))
	}
	return obj{key: items, "page": res.page, "per_page": res.perPage, "total": res.total, "total_pages": res.totalPages, "next_page": res.nextPage}
}

func iidFilters(q url.Values, iids []iidValue) {
	for _, i := range iids {
		if i > 0 {
			q.Add("iids[]", strconv.Itoa(int(i)))
		}
	}
}

func handleListIssues(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID        string     `json:"project_id"`
		IIDs             []iidValue `json:"iids"`
		Search           string     `json:"search"`
		AssigneeUsername string     `json:"assignee_username"`
		Labels           string     `json:"labels"`
		State            string     `json:"state"`
		Page             *int       `json:"page"`
		PerPage          *int       `json:"per_page"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.ProjectID) == "" {
		return nil, errors.New("project_id is required (a project ID or its path, group/project)")
	}
	q := url.Values{}
	iidFilters(q, args.IIDs)
	for k, v := range map[string]string{"search": args.Search, "assignee_username": args.AssigneeUsername, "labels": args.Labels, "state": args.State} {
		if v != "" {
			q.Set(k, v)
		}
	}
	res, err := listPage[apiIssue](projectPath(args.ProjectID, "issues"), q, args.Page, args.PerPage)
	if err != nil {
		return nil, err
	}
	return toolJSON(listPayload("issues", res, apiIssue.summary))
}

type apiMR struct {
	IID          int       `json:"iid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        string    `json:"state"`
	Draft        bool      `json:"draft"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	SHA          string    `json:"sha"`
	Author       userRef   `json:"author"`
	Assignees    []userRef `json:"assignees"`
	Reviewers    []userRef `json:"reviewers"`
	Labels       []string  `json:"labels"`
	Milestone    *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	MergeStatus         string `json:"merge_status"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
	HasConflicts        bool   `json:"has_conflicts"`
	WebURL              string `json:"web_url"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	MergedAt            string `json:"merged_at"`
}

func (m apiMR) mergeStatus() string {
	if m.DetailedMergeStatus != "" {
		return m.DetailedMergeStatus
	}
	return m.MergeStatus
}

func (m apiMR) milestone() string {
	if m.Milestone == nil {
		return ""
	}
	return m.Milestone.Title
}

type mrSummary struct {
	IID          int      `json:"iid"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Draft        bool     `json:"draft,omitempty"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Author       string   `json:"author,omitempty"`
	Labels       []string `json:"labels"`
	WebURL       string   `json:"web_url"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

func (m apiMR) summary() mrSummary {
	return mrSummary{IID: m.IID, Title: m.Title, State: m.State, Draft: m.Draft, SourceBranch: m.SourceBranch,
		TargetBranch: m.TargetBranch, Author: m.Author.Username, Labels: nonNil(m.Labels), WebURL: m.WebURL, UpdatedAt: m.UpdatedAt}
}

type closedIssue struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	State  string `json:"state"`
	WebURL string `json:"web_url"`
}

type mrView struct {
	mrSummary
	Description       string        `json:"description"`
	SHA               string        `json:"sha,omitempty"`
	Assignees         []string      `json:"assignees"`
	Reviewers         []string      `json:"reviewers"`
	Milestone         string        `json:"milestone,omitempty"`
	MergeStatus       string        `json:"merge_status,omitempty"`
	HasConflicts      bool          `json:"has_conflicts,omitempty"`
	CreatedAt         string        `json:"created_at,omitempty"`
	MergedAt          string        `json:"merged_at,omitempty"`
	ClosesIssues      []closedIssue `json:"closes_issues"`
	ClosesIssuesError string        `json:"closes_issues_error,omitempty"`
}

func handleGetMR(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string   `json:"project_id"`
		MRIID     iidValue `json:"mr_iid"`
		IID       iidValue `json:"iid"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	iid := int(args.MRIID)
	if iid == 0 {
		iid = int(args.IID)
	}
	if err := requireProjectAndIID(args.ProjectID, "mr_iid", iid); err != nil {
		return nil, err
	}
	data, err := gitlabAPI(projectPath(args.ProjectID, "merge_requests/%d", iid))
	if err != nil {
		return nil, notFound(err, fmt.Sprintf("merge request !%d", iid), args.ProjectID)
	}
	var mr apiMR
	if err := json.Unmarshal(data, &mr); err != nil {
		return nil, fmt.Errorf("unexpected GitLab answer for merge request !%d: %w", iid, err)
	}
	view := mrView{
		mrSummary: mr.summary(), Description: mr.Description, SHA: mr.SHA, Assignees: usernames(mr.Assignees),
		Reviewers: usernames(mr.Reviewers), Milestone: mr.milestone(), MergeStatus: mr.mergeStatus(),
		HasConflicts: mr.HasConflicts, CreatedAt: mr.CreatedAt, MergedAt: mr.MergedAt, ClosesIssues: []closedIssue{},
	}
	if err := getInto(projectPath(args.ProjectID, "merge_requests/%d/closes_issues", iid), &view.ClosesIssues); err != nil {
		view.ClosesIssuesError = err.Error()
	}
	return toolJSON(view)
}

func handleListMRs(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID    string     `json:"project_id"`
		IIDs         []iidValue `json:"iids"`
		Search       string     `json:"search"`
		SourceBranch string     `json:"source_branch"`
		State        string     `json:"state"`
		Page         *int       `json:"page"`
		PerPage      *int       `json:"per_page"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.ProjectID) == "" {
		return nil, errors.New("project_id is required (a project ID or its path, group/project)")
	}
	q := url.Values{}
	iidFilters(q, args.IIDs)
	for k, v := range map[string]string{"search": args.Search, "source_branch": args.SourceBranch, "state": args.State} {
		if v != "" {
			q.Set(k, v)
		}
	}
	res, err := listPage[apiMR](projectPath(args.ProjectID, "merge_requests"), q, args.Page, args.PerPage)
	if err != nil {
		return nil, err
	}
	return toolJSON(listPayload("merge_requests", res, apiMR.summary))
}
