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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tracker"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// jsonServer answers every request with status, the headers and body as JSON.
func jsonServer(t *testing.T, status int, headers map[string]string, body any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// gitlabServer creates a test HTTP server that responds to one (method, path) pair.
func gitlabServer(t *testing.T, method, path string, body interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, typ tracker.Type, baseURL string, write bool) tracker.Tracker {
	t.Helper()
	c, err := tracker.New(tracker.Config{Type: typ, BaseURL: baseURL, Token: "tok", WriteEnabled: write})
	require.NoError(t, err, "creating the %s client", typ)
	return c
}

// ─────────────────────────────────────────────────────────────────────────────
// FetchIssue
// ─────────────────────────────────────────────────────────────────────────────

func TestGitLab_FetchIssue(t *testing.T) {
	tests := []struct {
		name    string
		iid     int
		status  int
		headers map[string]string
		body    map[string]any
		check   func(t *testing.T, issue *tracker.IssueState, err error)
	}{
		{
			name: "open issue with its labels and assignees", iid: 7, status: http.StatusOK,
			body: map[string]any{
				"iid": 7, "state": "opened", "title": "Fix auth bug", "labels": []string{"bug", "backend"},
				"updated_at": "2026-07-01T10:00:00Z", "assignees": []map[string]any{{"username": "alice"}},
			},
			check: func(t *testing.T, issue *tracker.IssueState, err error) {
				require.NoError(t, err)
				assert.Equal(t, "open", issue.State, "state")
				assert.Equal(t, 7, issue.IID, "IID")
				assert.Equal(t, []string{"bug", "backend"}, issue.Labels, "labels")
				assert.Equal(t, []string{"alice"}, issue.Assignees, "assignees")
			},
		},
		{
			name: "closed issue", iid: 8, status: http.StatusOK,
			body: map[string]any{"iid": 8, "state": "closed", "title": "Done ticket"},
			check: func(t *testing.T, issue *tracker.IssueState, err error) {
				require.NoError(t, err)
				assert.True(t, issue.IsClosed(), "the issue is closed")
			},
		},
		{
			name: "description and status", iid: 10, status: http.StatusOK,
			body: map[string]any{
				"iid": 10, "state": "opened", "title": "Add OAuth support",
				"description": "## Context\n\nWe need OAuth 2.0 for third-party auth.",
				"labels":      []string{"feature"}, "updated_at": "2026-07-01T10:00:00Z",
				"assignees": []map[string]any{{"username": "bob"}},
			},
			check: func(t *testing.T, issue *tracker.IssueState, err error) {
				require.NoError(t, err)
				assert.Equal(t, "## Context\n\nWe need OAuth 2.0 for third-party auth.", issue.Description, "description")
				assert.Equal(t, "opened", issue.StatusName, "status name")
				assert.Equal(t, "opened", issue.StatusCategory, "status category")
			},
		},
		{
			name: "unknown issue", iid: 99, status: http.StatusNotFound,
			check: func(t *testing.T, _ *tracker.IssueState, err error) { assert.Equal(t, tracker.ErrIssueNotFound, err) },
		},
		{
			name: "invalid token", iid: 1, status: http.StatusUnauthorized,
			check: func(t *testing.T, _ *tracker.IssueState, err error) {
				assert.ErrorIs(t, err, tracker.ErrTokenInvalid)
				var rejected *tracker.TokenRejectedError
				require.ErrorAs(t, err, &rejected)
				assert.Equal(t, http.StatusUnauthorized, rejected.Status)
				assert.NotEmpty(t, rejected.Origin.Server, "the error names the server")
			},
		},
		{
			name: "rate limited", iid: 1, status: http.StatusTooManyRequests, headers: map[string]string{"Retry-After": "30"},
			check: func(t *testing.T, _ *tracker.IssueState, err error) {
				assert.True(t, tracker.IsRateLimited(err), "expected a rate limit error, got %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body any
			if tt.body != nil {
				body = tt.body
			}
			srv := jsonServer(t, tt.status, tt.headers, body)
			issue, err := newClient(t, tracker.TypeGitLab, srv.URL, false).FetchIssue(context.Background(), "42", tt.iid)
			tt.check(t, issue, err)
		})
	}

	t.Run("the request targets the issue of the project", func(t *testing.T) {
		srv := gitlabServer(t, http.MethodGet, "/api/v4/projects/42/issues/7", map[string]any{"iid": 7, "state": "opened"})
		issue, err := newClient(t, tracker.TypeGitLab, srv.URL, false).FetchIssue(context.Background(), "42", 7)
		require.NoError(t, err)
		assert.Equal(t, 7, issue.IID)
	})
}

func TestGitLab_AddLabels(t *testing.T) {
	t.Run("refused when writing is disabled", func(t *testing.T) {
		gl := newClient(t, tracker.TypeGitLab, "http://unused", false)
		assert.Equal(t, tracker.ErrWriteDisabled, gl.AddLabels(context.Background(), "42", 1, []string{"my-label"}))
	})

	t.Run("sends the labels with a PUT", func(t *testing.T) {
		var method, path string
		var received map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&received)
			_ = json.NewEncoder(w).Encode(map[string]any{"iid": 1})
		}))
		t.Cleanup(srv.Close)

		require.NoError(t, newClient(t, tracker.TypeGitLab, srv.URL, true).AddLabels(context.Background(), "42", 1, []string{"agent-reviewed"}))
		assert.Equal(t, http.MethodPut, method, "method")
		assert.Equal(t, "/api/v4/projects/42/issues/1", path, "path")
		assert.Equal(t, "agent-reviewed", received["add_labels"], "labels")
	})
}

func TestJira_FetchIssue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		iid   int
		body  map[string]any
		check func(t *testing.T, issue *tracker.IssueState)
	}{
		{
			name: "open issue", key: "SRU", iid: 42,
			body: map[string]any{
				"id": "10042", "key": "SRU-42",
				"fields": map[string]any{
					"summary":     "Implement login",
					"description": "As a user, I want to login with my email and password.",
					"status": map[string]any{
						"name":           "In Progress",
						"statusCategory": map[string]any{"key": "indeterminate"},
					},
					"labels":   []string{"backend"},
					"updated":  "2026-07-01T10:00:00.000+0000",
					"assignee": map[string]any{"name": "alice", "displayName": "Alice"},
				},
			},
			check: func(t *testing.T, issue *tracker.IssueState) {
				assert.Equal(t, "open", issue.State, "state")
				assert.Equal(t, "SRU-42", issue.Key, "key")
				assert.Equal(t, "In Progress", issue.StatusName, "status name")
				assert.Equal(t, "indeterminate", issue.StatusCategory, "status category")
				assert.Equal(t, "As a user, I want to login with my email and password.", issue.Description, "description")
				assert.Equal(t, "Implement login", issue.Title, "title")
			},
		},
		{
			name: "closed issue", key: "SRU", iid: 43,
			body: map[string]any{
				"id": "10043", "key": "SRU-43",
				"fields": map[string]any{
					"summary": "Done",
					"status": map[string]any{
						"statusCategory": map[string]any{"key": "done"},
					},
					"labels":  []string{},
					"updated": "2026-07-01T10:00:00.000+0000",
				},
			},
			check: func(t *testing.T, issue *tracker.IssueState) { assert.True(t, issue.IsClosed(), "the issue is closed") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := jsonServer(t, http.StatusOK, nil, tt.body)
			issue, err := newClient(t, tracker.TypeJira, srv.URL, false).FetchIssue(context.Background(), tt.key, tt.iid)
			require.NoError(t, err)
			tt.check(t, issue)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SyncState
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncState_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	s, err := tracker.LoadSyncState(dir)
	require.NoError(t, err, "load")
	assert.True(t, s.LastSync(tracker.TypeGitLab, "42").IsZero(), "zero time for an unknown project")

	now := time.Now().UTC().Truncate(time.Second)
	s.SetLastSync(tracker.TypeGitLab, "42", now)
	require.NoError(t, s.Save(dir), "save")

	s2, err := tracker.LoadSyncState(dir)
	require.NoError(t, err, "reload")
	assert.True(t, s2.LastSync(tracker.TypeGitLab, "42").Equal(now), "last sync survives the reload")
}

// ─────────────────────────────────────────────────────────────────────────────
// MapTrackerStatus and TruncateDescription
// ─────────────────────────────────────────────────────────────────────────────

func TestMapTrackerStatus(t *testing.T) {
	explicit := map[string]string{
		"In Progress": "in_progress",
		"Code Review": "review",
		"In QA":       "review",
		"Blocked":     "blocked",
		"Done":        "done",
		"To Do":       "planned",
	}
	tests := []struct {
		name     string
		mapping  map[string]string
		status   string
		category string
		want     string
	}{
		{"mapped: in progress", explicit, "In Progress", "indeterminate", "in_progress"},
		{"mapped: code review", explicit, "Code Review", "indeterminate", "review"},
		{"mapped: in QA", explicit, "In QA", "indeterminate", "review"},
		{"mapped: blocked", explicit, "Blocked", "indeterminate", "blocked"},
		{"mapped: done", explicit, "Done", "done", "done"},
		{"mapped: to do", explicit, "To Do", "new", "planned"},
		{"the mapping ignores the case", map[string]string{"code review": "review"}, "Code Review", "indeterminate", "review"},
		{"a mapping to an invalid status falls through to the category", map[string]string{"In Progress": "invalid_status"}, "In Progress", "indeterminate", "in_progress"},
		{"jira done", nil, "Terminé", "done", "done"},
		{"jira new", nil, "À faire", "new", "planned"},
		{"jira indeterminate", nil, "En développement", "indeterminate", "in_progress"},
		{"gitlab closed", nil, "closed", "closed", "done"},
		{"gitlab opened", nil, "opened", "opened", "in_progress"},
		{"unknown category", nil, "Custom", "custom", "in_progress"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &tracker.IssueState{StatusName: tt.status, StatusCategory: tt.category}
			assert.Equal(t, tt.want, tracker.MapTrackerStatus(issue, tt.mapping, nil))
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	limit := tracker.MaxDescriptionLen
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"short", "Short description", "Short description"},
		{"exactly at the limit", strings.Repeat("a", limit), strings.Repeat("a", limit)},
		{"over the limit", strings.Repeat("b", limit+100), strings.Repeat("b", limit) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tracker.TruncateDescription(tt.in))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Discovery and listing
// ─────────────────────────────────────────────────────────────────────────────

func TestGitLab_DiscoverProject(t *testing.T) {
	t.Run("labels of a single page", func(t *testing.T) {
		srv := jsonServer(t, http.StatusOK, nil, []map[string]any{
			{"name": "bug", "description": "Something is broken", "color": "#d9534f"},
			{"name": "feature", "description": "New functionality", "color": "#428bca"},
			{"name": "urgent", "description": "", "color": "#d10069"},
		})
		info, err := newClient(t, tracker.TypeGitLab, srv.URL, false).DiscoverProject(context.Background(), "myproject")
		require.NoError(t, err)
		require.Len(t, info.Labels, 3, "labels")
		for i, want := range []struct{ name, desc, color string }{
			{"bug", "Something is broken", "#d9534f"},
			{"feature", "New functionality", "#428bca"},
			{"urgent", "", "#d10069"},
		} {
			assert.Equal(t, want.name, info.Labels[i].Name, "label[%d] name", i)
			assert.Equal(t, want.desc, info.Labels[i].Description, "label[%d] description", i)
			assert.Equal(t, want.color, info.Labels[i].Color, "label[%d] color", i)
		}
	})

	t.Run("labels follow X-Next-Page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v4/projects/myproject/labels" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"name": "bug", "description": "Defect", "color": "#d9534f"},
					{"name": "feature", "description": "Enhancement", "color": "#428bca"},
				})
			case "2":
				_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "urgent", "description": "High priority", "color": "#d10069"}})
			default:
				_ = json.NewEncoder(w).Encode([]map[string]any{})
			}
		}))
		t.Cleanup(srv.Close)

		info, err := newClient(t, tracker.TypeGitLab, srv.URL, false).DiscoverProject(context.Background(), "myproject")
		require.NoError(t, err)
		var names []string
		for _, l := range info.Labels {
			names = append(names, l.Name)
		}
		assert.Equal(t, []string{"bug", "feature", "urgent"}, names, "the labels of both pages, in order")
	})

	t.Run("a server that never ends the pages is stopped at the page cap", func(t *testing.T) {
		var requests int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			page := 1
			if p := r.URL.Query().Get("page"); p != "" {
				page, _ = strconv.Atoi(p)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": fmt.Sprintf("label-page-%d", page), "color": "#000"}})
		}))
		t.Cleanup(srv.Close)

		info, err := newClient(t, tracker.TypeGitLab, srv.URL, false).DiscoverProject(context.Background(), "myproject")
		require.NoError(t, err)
		// maxLabelPages = 50 in gitlab.go.
		assert.LessOrEqual(t, requests, 51, "requests (50-page cap)")
		assert.NotEmpty(t, info.Labels, "the labels read before the cap are kept")
		assert.LessOrEqual(t, len(info.Labels), 50, "one label per page at most")
	})
}

func TestGitLab_ListIssuesByLabels(t *testing.T) {
	issue := func(iid int, title string) map[string]any {
		return map[string]any{"iid": iid, "state": "opened", "title": title, "labels": []string{"backend", "ready"}, "updated_at": "2026-07-01T10:00:00Z"}
	}

	t.Run("the query carries the state and the labels", func(t *testing.T) {
		var query url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/v4/projects/myproject/issues") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			query = r.URL.Query()
			_ = json.NewEncoder(w).Encode([]map[string]any{issue(1, "First issue"), issue(2, "Second issue")})
		}))
		t.Cleanup(srv.Close)

		issues, err := newClient(t, tracker.TypeGitLab, srv.URL, false).ListIssuesByLabels(context.Background(), "myproject",
			tracker.ListByLabelsOpts{Labels: []string{"backend", "ready"}})
		require.NoError(t, err)
		require.Len(t, issues, 2, "issues")
		assert.Equal(t, []int{1, 2}, []int{issues[0].IID, issues[1].IID}, "issue IIDs")
		assert.Equal(t, "opened", query.Get("state"), "query state")
		assert.Equal(t, "backend,ready", query.Get("labels"), "query labels")
	})

	t.Run("MaxResults stops across the pages", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/v4/projects/myproject/issues") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_ = json.NewEncoder(w).Encode([]map[string]any{issue(1, "A"), issue(2, "B"), issue(3, "C")})
			case "2":
				_ = json.NewEncoder(w).Encode([]map[string]any{issue(4, "D"), issue(5, "E"), issue(6, "F")})
			default:
				_ = json.NewEncoder(w).Encode([]map[string]any{})
			}
		}))
		t.Cleanup(srv.Close)

		issues, err := newClient(t, tracker.TypeGitLab, srv.URL, false).ListIssuesByLabels(context.Background(), "myproject",
			tracker.ListByLabelsOpts{MaxResults: 4})
		require.NoError(t, err)
		var iids []int
		for _, is := range issues {
			iids = append(iids, is.IID)
		}
		assert.Equal(t, []int{1, 2, 3, 4}, iids, "the first 4 issues, in order")
	})
}

func TestJira_Discovery(t *testing.T) {
	t.Run("statuses of every issue type, without duplicates", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/project/PROJ/statuses":
				cat := func(k string) map[string]string { return map[string]string{"key": k} }
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"name": "Bug", "statuses": []map[string]any{
						{"name": "To Do", "statusCategory": cat("new")},
						{"name": "In Progress", "statusCategory": cat("indeterminate")},
						{"name": "Done", "statusCategory": cat("done")},
					}},
					{"name": "Story", "statuses": []map[string]any{
						{"name": "In Progress", "statusCategory": cat("indeterminate")},
						{"name": "Code Review", "statusCategory": cat("indeterminate")},
						{"name": "Done", "statusCategory": cat("done")},
					}},
				})
			case "/rest/api/2/label":
				_ = json.NewEncoder(w).Encode(map[string]any{"values": []string{}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)

		info, err := newClient(t, tracker.TypeJira, srv.URL, false).DiscoverProject(context.Background(), "PROJ")
		require.NoError(t, err)
		got := make(map[string]string, len(info.Statuses))
		for _, s := range info.Statuses {
			got[s.Name] = s.Category
		}
		assert.Len(t, info.Statuses, 4, "4 unique statuses")
		assert.Equal(t, map[string]string{"To Do": "new", "In Progress": "indeterminate", "Done": "done", "Code Review": "indeterminate"}, got)
	})

	t.Run("issues by labels are searched with a JQL", func(t *testing.T) {
		var jql string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/search" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var req struct {
				JQL string `json:"jql"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			jql = req.JQL
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{{
				"id": "10001", "key": "PROJ-10",
				"fields": map[string]any{
					"summary": "Label-matched issue",
					"status": map[string]any{
						"name":           "In Progress",
						"statusCategory": map[string]any{"key": "indeterminate"},
					},
					"labels":  []string{"backend", "sprint-42"},
					"updated": "2026-07-01T10:00:00.000+0000",
				},
			}}})
		}))
		t.Cleanup(srv.Close)

		issues, err := newClient(t, tracker.TypeJira, srv.URL, false).ListIssuesByLabels(context.Background(), "PROJ",
			tracker.ListByLabelsOpts{Labels: []string{"backend", "sprint-42"}})
		require.NoError(t, err)
		require.Len(t, issues, 1, "issues")
		assert.Equal(t, "PROJ-10", issues[0].Key, "issue key")
		assert.Equal(t, "open", issues[0].State, "issue state")
		assert.Contains(t, jql, "labels in (", "JQL labels clause")
		assert.Contains(t, jql, `"backend"`, "JQL first label")
		assert.Contains(t, jql, `"sprint-42"`, "JQL second label")
	})
}
