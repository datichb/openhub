package tracker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/tracker"
)

// ─────────────────────────────────────────────────────────────────────────────
// GitLab client tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGitLab_FetchIssue_Open(t *testing.T) {
	srv := gitlabServer(t, http.MethodGet, "/api/v4/projects/42/issues/7", map[string]interface{}{
		"iid":        7,
		"state":      "opened",
		"title":      "Fix auth bug",
		"labels":     []string{"bug", "backend"},
		"updated_at": "2026-07-01T10:00:00Z",
		"assignees":  []map[string]interface{}{{"username": "alice"}},
	})
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "tok"})
	issue, err := gl.FetchIssue(context.Background(), "42", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.State != "open" {
		t.Errorf("state: got %q, want %q", issue.State, "open")
	}
	if issue.IID != 7 {
		t.Errorf("IID: got %d, want 7", issue.IID)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "bug" {
		t.Errorf("labels: got %v", issue.Labels)
	}
	if len(issue.Assignees) != 1 || issue.Assignees[0] != "alice" {
		t.Errorf("assignees: got %v", issue.Assignees)
	}
}

func TestGitLab_FetchIssue_Closed(t *testing.T) {
	srv := gitlabServer(t, http.MethodGet, "/api/v4/projects/42/issues/8", map[string]interface{}{
		"iid":   8,
		"state": "closed",
		"title": "Done ticket",
	})
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "tok"})
	issue, err := gl.FetchIssue(context.Background(), "42", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !issue.IsClosed() {
		t.Errorf("expected closed issue")
	}
}

func TestGitLab_FetchIssue_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "tok"})
	_, err := gl.FetchIssue(context.Background(), "42", 99)
	if err != tracker.ErrIssueNotFound {
		t.Errorf("expected ErrIssueNotFound, got %v", err)
	}
}

func TestGitLab_FetchIssue_TokenInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "bad"})
	_, err := gl.FetchIssue(context.Background(), "42", 1)
	if err != tracker.ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestGitLab_FetchIssue_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "tok"})
	_, err := gl.FetchIssue(context.Background(), "42", 1)
	if !tracker.IsRateLimited(err) {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestGitLab_AddLabels_WriteDisabled(t *testing.T) {
	gl, _ := tracker.New(tracker.Config{
		Type:         tracker.TypeGitLab,
		BaseURL:      "http://unused",
		Token:        "tok",
		WriteEnabled: false,
	})
	err := gl.AddLabels(context.Background(), "42", 1, []string{"my-label"})
	if err != tracker.ErrWriteDisabled {
		t.Errorf("expected ErrWriteDisabled, got %v", err)
	}
}

func TestGitLab_AddLabels_Success(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"iid": 1})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{
		Type:         tracker.TypeGitLab,
		BaseURL:      srv.URL,
		Token:        "tok",
		WriteEnabled: true,
	})
	err := gl.AddLabels(context.Background(), "42", 1, []string{"agent-reviewed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Jira client tests
// ─────────────────────────────────────────────────────────────────────────────

func TestJira_FetchIssue_Open(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "10042",
			"key": "SRU-42",
			"fields": map[string]interface{}{
				"summary":     "Implement login",
				"description": "As a user, I want to login with my email and password.",
				"status": map[string]interface{}{
					"name":           "In Progress",
					"statusCategory": map[string]interface{}{"key": "indeterminate"},
				},
				"labels":   []string{"backend"},
				"updated":  "2026-07-01T10:00:00.000+0000",
				"assignee": map[string]interface{}{"name": "alice", "displayName": "Alice"},
			},
		})
	}))
	defer srv.Close()

	j, _ := tracker.New(tracker.Config{Type: tracker.TypeJira, BaseURL: srv.URL, Token: "tok"})
	issue, err := j.FetchIssue(context.Background(), "SRU", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.State != "open" {
		t.Errorf("state: got %q, want %q", issue.State, "open")
	}
	if issue.Key != "SRU-42" {
		t.Errorf("key: got %q, want %q", issue.Key, "SRU-42")
	}
	if issue.StatusName != "In Progress" {
		t.Errorf("statusName: got %q, want %q", issue.StatusName, "In Progress")
	}
	if issue.StatusCategory != "indeterminate" {
		t.Errorf("statusCategory: got %q, want %q", issue.StatusCategory, "indeterminate")
	}
	if issue.Description != "As a user, I want to login with my email and password." {
		t.Errorf("description: got %q", issue.Description)
	}
	if issue.Title != "Implement login" {
		t.Errorf("title: got %q, want %q", issue.Title, "Implement login")
	}
}

func TestJira_FetchIssue_Closed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "10043",
			"key": "SRU-43",
			"fields": map[string]interface{}{
				"summary": "Done",
				"status": map[string]interface{}{
					"statusCategory": map[string]interface{}{"key": "done"},
				},
				"labels":  []string{},
				"updated": "2026-07-01T10:00:00.000+0000",
			},
		})
	}))
	defer srv.Close()

	j, _ := tracker.New(tracker.Config{Type: tracker.TypeJira, BaseURL: srv.URL, Token: "tok"})
	issue, err := j.FetchIssue(context.Background(), "SRU", 43)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !issue.IsClosed() {
		t.Errorf("expected closed issue")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SyncState tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncState_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	s, err := tracker.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.LastSync(tracker.TypeGitLab, "42") != (time.Time{}) {
		t.Error("expected zero time for unknown project")
	}

	now := time.Now().UTC().Truncate(time.Second)
	s.SetLastSync(tracker.TypeGitLab, "42", now)
	if err := s.Save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2, err := tracker.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := s2.LastSync(tracker.TypeGitLab, "42")
	if !got.Equal(now) {
		t.Errorf("last sync: got %v, want %v", got, now)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// gitlabServer creates a test HTTP server that responds to one (method, path) pair.
func gitlabServer(t *testing.T, method, path string, body interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
}

// ─────────────────────────────────────────────────────────────────────────────
// MapTrackerStatus tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMapTrackerStatus_ExplicitMapping(t *testing.T) {
	mapping := map[string]string{
		"In Progress": "in_progress",
		"Code Review": "review",
		"In QA":       "review",
		"Blocked":     "blocked",
		"Done":        "done",
		"To Do":       "planned",
	}

	tests := []struct {
		statusName     string
		statusCategory string
		expected       string
	}{
		{"In Progress", "indeterminate", "in_progress"},
		{"Code Review", "indeterminate", "review"},
		{"In QA", "indeterminate", "review"},
		{"Blocked", "indeterminate", "blocked"},
		{"Done", "done", "done"},
		{"To Do", "new", "planned"},
	}
	for _, tt := range tests {
		t.Run(tt.statusName, func(t *testing.T) {
			issue := &tracker.IssueState{
				StatusName:     tt.statusName,
				StatusCategory: tt.statusCategory,
			}
			got := tracker.MapTrackerStatus(issue, mapping, nil)
			if got != tt.expected {
				t.Errorf("MapTrackerStatus(%q): got %q, want %q", tt.statusName, got, tt.expected)
			}
		})
	}
}

func TestMapTrackerStatus_CaseInsensitive(t *testing.T) {
	mapping := map[string]string{
		"code review": "review",
	}
	issue := &tracker.IssueState{
		StatusName:     "Code Review",
		StatusCategory: "indeterminate",
	}
	got := tracker.MapTrackerStatus(issue, mapping, nil)
	if got != "review" {
		t.Errorf("case insensitive mapping failed: got %q, want %q", got, "review")
	}
}

func TestMapTrackerStatus_CategoryFallback(t *testing.T) {
	tests := []struct {
		name           string
		statusName     string
		statusCategory string
		expected       string
	}{
		{"jira done", "Terminé", "done", "done"},
		{"jira new", "À faire", "new", "planned"},
		{"jira indeterminate", "En développement", "indeterminate", "in_progress"},
		{"gitlab closed", "closed", "closed", "done"},
		{"gitlab opened", "opened", "opened", "in_progress"},
		{"unknown category", "Custom", "custom", "in_progress"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &tracker.IssueState{
				StatusName:     tt.statusName,
				StatusCategory: tt.statusCategory,
			}
			// Empty mapping → always uses category fallback
			got := tracker.MapTrackerStatus(issue, nil, nil)
			if got != tt.expected {
				t.Errorf("MapTrackerStatus(%q/%q): got %q, want %q",
					tt.statusName, tt.statusCategory, got, tt.expected)
			}
		})
	}
}

func TestMapTrackerStatus_InvalidMappingValue(t *testing.T) {
	// If the mapping value is not a valid claim status, fall through to category.
	mapping := map[string]string{
		"In Progress": "invalid_status",
	}
	issue := &tracker.IssueState{
		StatusName:     "In Progress",
		StatusCategory: "indeterminate",
	}
	got := tracker.MapTrackerStatus(issue, mapping, nil)
	// Should fall back to category-based mapping since "invalid_status" is not valid.
	if got != "in_progress" {
		t.Errorf("invalid mapping value: got %q, want %q", got, "in_progress")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TruncateDescription tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTruncateDescription_Short(t *testing.T) {
	desc := "Short description"
	got := tracker.TruncateDescription(desc)
	if got != desc {
		t.Errorf("short description should not be truncated: got %q", got)
	}
}

func TestTruncateDescription_ExactLimit(t *testing.T) {
	desc := make([]byte, tracker.MaxDescriptionLen)
	for i := range desc {
		desc[i] = 'a'
	}
	got := tracker.TruncateDescription(string(desc))
	if got != string(desc) {
		t.Errorf("exact-limit description should not be truncated")
	}
}

func TestTruncateDescription_Long(t *testing.T) {
	desc := make([]byte, tracker.MaxDescriptionLen+100)
	for i := range desc {
		desc[i] = 'b'
	}
	got := tracker.TruncateDescription(string(desc))
	if len(got) != tracker.MaxDescriptionLen+3 { // +3 for "..."
		t.Errorf("truncated length: got %d, want %d", len(got), tracker.MaxDescriptionLen+3)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("truncated description should end with '...'")
	}
}

func TestTruncateDescription_Empty(t *testing.T) {
	got := tracker.TruncateDescription("")
	if got != "" {
		t.Errorf("empty description should stay empty: got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GitLab description + status parsing tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGitLab_FetchIssue_WithDescription(t *testing.T) {
	srv := gitlabServer(t, http.MethodGet, "/api/v4/projects/42/issues/10", map[string]interface{}{
		"iid":         10,
		"state":       "opened",
		"title":       "Add OAuth support",
		"description": "## Context\n\nWe need OAuth 2.0 for third-party auth.",
		"labels":      []string{"feature"},
		"updated_at":  "2026-07-01T10:00:00Z",
		"assignees":   []map[string]interface{}{{"username": "bob"}},
	})
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "tok"})
	issue, err := gl.FetchIssue(context.Background(), "42", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Description != "## Context\n\nWe need OAuth 2.0 for third-party auth." {
		t.Errorf("description: got %q", issue.Description)
	}
	if issue.StatusName != "opened" {
		t.Errorf("statusName: got %q, want %q", issue.StatusName, "opened")
	}
	if issue.StatusCategory != "opened" {
		t.Errorf("statusCategory: got %q, want %q", issue.StatusCategory, "opened")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GitLab DiscoverProject / ListIssuesByLabels / Pagination integration tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGitLab_DiscoverProject_Labels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/myproject/labels" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// No X-Next-Page header → single page.
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "bug", "description": "Something is broken", "color": "#d9534f"},
			{"name": "feature", "description": "New functionality", "color": "#428bca"},
			{"name": "urgent", "description": "", "color": "#d10069"},
		})
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "test-token"})
	info, err := gl.DiscoverProject(context.Background(), "myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.Labels) != 3 {
		t.Fatalf("labels count: got %d, want 3", len(info.Labels))
	}

	want := []struct{ name, desc, color string }{
		{"bug", "Something is broken", "#d9534f"},
		{"feature", "New functionality", "#428bca"},
		{"urgent", "", "#d10069"},
	}
	for i, w := range want {
		if info.Labels[i].Name != w.name {
			t.Errorf("label[%d].Name: got %q, want %q", i, info.Labels[i].Name, w.name)
		}
		if info.Labels[i].Description != w.desc {
			t.Errorf("label[%d].Description: got %q, want %q", i, info.Labels[i].Description, w.desc)
		}
		if info.Labels[i].Color != w.color {
			t.Errorf("label[%d].Color: got %q, want %q", i, info.Labels[i].Color, w.color)
		}
	}
}

func TestGitLab_DiscoverProject_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/myproject/labels" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case "", "1":
			w.Header().Set("X-Next-Page", "2")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"name": "bug", "description": "Defect", "color": "#d9534f"},
				{"name": "feature", "description": "Enhancement", "color": "#428bca"},
			})
		case "2":
			// No X-Next-Page → last page.
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"name": "urgent", "description": "High priority", "color": "#d10069"},
			})
		default:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "test-token"})
	info, err := gl.DiscoverProject(context.Background(), "myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.Labels) != 3 {
		t.Fatalf("labels count: got %d, want 3", len(info.Labels))
	}
	names := []string{info.Labels[0].Name, info.Labels[1].Name, info.Labels[2].Name}
	if names[0] != "bug" || names[1] != "feature" || names[2] != "urgent" {
		t.Errorf("label names: got %v", names)
	}
}

func TestGitLab_DiscoverProject_PageCap(t *testing.T) {
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/myproject/labels" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		requestCount++

		page := r.URL.Query().Get("page")
		pageNum := 1
		if page != "" {
			pageNum, _ = strconv.Atoi(page)
		}

		w.Header().Set("Content-Type", "application/json")
		// Always advertise a next page → infinite pagination.
		w.Header().Set("X-Next-Page", strconv.Itoa(pageNum+1))
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": fmt.Sprintf("label-page-%d", pageNum), "description": "", "color": "#000"},
		})
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "test-token"})
	info, err := gl.DiscoverProject(context.Background(), "myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// maxLabelPages = 50 in gitlab.go. The loop should stop at page 50.
	if requestCount > 51 {
		t.Errorf("expected at most 51 requests (50-page cap), got %d", requestCount)
	}
	if len(info.Labels) == 0 {
		t.Error("expected at least some labels")
	}
	if len(info.Labels) > 50 {
		t.Errorf("expected at most 50 labels (one per page), got %d", len(info.Labels))
	}
}

func TestGitLab_ListIssuesByLabels(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/v4/projects/myproject/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"iid": 1, "state": "opened", "title": "First issue",
				"labels": []string{"backend", "ready"}, "updated_at": "2026-07-01T10:00:00Z",
				"assignees": []map[string]interface{}{{"username": "alice"}},
			},
			{
				"iid": 2, "state": "opened", "title": "Second issue",
				"labels": []string{"backend", "ready"}, "updated_at": "2026-07-02T10:00:00Z",
				"assignees": []map[string]interface{}{},
			},
		})
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "test-token"})
	issues, err := gl.ListIssuesByLabels(context.Background(), "myproject", tracker.ListByLabelsOpts{
		Labels: []string{"backend", "ready"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("issues count: got %d, want 2", len(issues))
	}
	if issues[0].IID != 1 || issues[1].IID != 2 {
		t.Errorf("issue IIDs: got %d, %d", issues[0].IID, issues[1].IID)
	}
	// Verify the query string contains the expected parameters.
	if capturedQuery.Get("state") != "opened" {
		t.Errorf("query state: got %q, want %q", capturedQuery.Get("state"), "opened")
	}
	if capturedQuery.Get("labels") != "backend,ready" {
		t.Errorf("query labels: got %q, want %q", capturedQuery.Get("labels"), "backend,ready")
	}
}

func TestGitLab_PaginatedListIssues_MaxResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v4/projects/myproject/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case "", "1":
			w.Header().Set("X-Next-Page", "2")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"iid": 1, "state": "opened", "title": "A", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
				{"iid": 2, "state": "opened", "title": "B", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
				{"iid": 3, "state": "opened", "title": "C", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
			})
		case "2":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"iid": 4, "state": "opened", "title": "D", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
				{"iid": 5, "state": "opened", "title": "E", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
				{"iid": 6, "state": "opened", "title": "F", "labels": []string{}, "updated_at": "2026-07-01T10:00:00Z"},
			})
		default:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}))
	defer srv.Close()

	gl, _ := tracker.New(tracker.Config{Type: tracker.TypeGitLab, BaseURL: srv.URL, Token: "test-token"})
	issues, err := gl.ListIssuesByLabels(context.Background(), "myproject", tracker.ListByLabelsOpts{
		MaxResults: 4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 4 {
		t.Fatalf("issues count: got %d, want 4", len(issues))
	}
	// Verify the first 4 issues are returned in order.
	for i, iid := range []int{1, 2, 3, 4} {
		if issues[i].IID != iid {
			t.Errorf("issue[%d].IID: got %d, want %d", i, issues[i].IID, iid)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Jira DiscoverProject / ListIssuesByLabels integration tests
// ─────────────────────────────────────────────────────────────────────────────

func TestJira_DiscoverProject_Statuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/rest/api/2/project/PROJ/statuses":
			// Two issue types with overlapping statuses.
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name": "Bug",
					"statuses": []map[string]interface{}{
						{"name": "To Do", "statusCategory": map[string]string{"key": "new"}},
						{"name": "In Progress", "statusCategory": map[string]string{"key": "indeterminate"}},
						{"name": "Done", "statusCategory": map[string]string{"key": "done"}},
					},
				},
				{
					"name": "Story",
					"statuses": []map[string]interface{}{
						{"name": "In Progress", "statusCategory": map[string]string{"key": "indeterminate"}}, // duplicate
						{"name": "Code Review", "statusCategory": map[string]string{"key": "indeterminate"}},
						{"name": "Done", "statusCategory": map[string]string{"key": "done"}}, // duplicate
					},
				},
			})
		case "/rest/api/2/label":
			// Return empty labels (best-effort endpoint).
			json.NewEncoder(w).Encode(map[string]interface{}{"values": []string{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	j, _ := tracker.New(tracker.Config{Type: tracker.TypeJira, BaseURL: srv.URL, Token: "test-token"})
	info, err := j.DiscoverProject(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 4 unique statuses: "To Do", "In Progress", "Done", "Code Review".
	if len(info.Statuses) != 4 {
		t.Fatalf("statuses count: got %d, want 4 (deduplicated)", len(info.Statuses))
	}

	// Build a lookup for assertion.
	statusMap := make(map[string]string, len(info.Statuses))
	for _, s := range info.Statuses {
		statusMap[s.Name] = s.Category
	}
	wantStatuses := map[string]string{
		"To Do":       "new",
		"In Progress": "indeterminate",
		"Done":        "done",
		"Code Review": "indeterminate",
	}
	for name, wantCat := range wantStatuses {
		gotCat, ok := statusMap[name]
		if !ok {
			t.Errorf("missing status %q", name)
			continue
		}
		if gotCat != wantCat {
			t.Errorf("status %q category: got %q, want %q", name, gotCat, wantCat)
		}
	}
}

func TestJira_ListIssuesByLabels(t *testing.T) {
	var capturedJQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var reqBody struct {
			JQL string `json:"jql"`
		}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedJQL = reqBody.JQL

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issues": []map[string]interface{}{
				{
					"id":  "10001",
					"key": "PROJ-10",
					"fields": map[string]interface{}{
						"summary": "Label-matched issue",
						"status": map[string]interface{}{
							"name":           "In Progress",
							"statusCategory": map[string]interface{}{"key": "indeterminate"},
						},
						"labels":  []string{"backend", "sprint-42"},
						"updated": "2026-07-01T10:00:00.000+0000",
					},
				},
			},
		})
	}))
	defer srv.Close()

	j, _ := tracker.New(tracker.Config{Type: tracker.TypeJira, BaseURL: srv.URL, Token: "test-token"})
	issues, err := j.ListIssuesByLabels(context.Background(), "PROJ", tracker.ListByLabelsOpts{
		Labels: []string{"backend", "sprint-42"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues count: got %d, want 1", len(issues))
	}
	if issues[0].Key != "PROJ-10" {
		t.Errorf("issue key: got %q, want %q", issues[0].Key, "PROJ-10")
	}
	if issues[0].State != "open" {
		t.Errorf("issue state: got %q, want %q", issues[0].State, "open")
	}
	// Verify the JQL contains the expected labels clause.
	if !strings.Contains(capturedJQL, `labels in (`) {
		t.Errorf("JQL missing labels clause: %q", capturedJQL)
	}
	if !strings.Contains(capturedJQL, `"backend"`) || !strings.Contains(capturedJQL, `"sprint-42"`) {
		t.Errorf("JQL missing expected label values: %q", capturedJQL)
	}
}
