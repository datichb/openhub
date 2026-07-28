package tracker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
				"summary": "Implement login",
				"status": map[string]interface{}{
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
