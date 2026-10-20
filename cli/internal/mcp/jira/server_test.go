package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seen is one request received by the fake Jira.
type seen struct {
	Method, Path, RawQuery, Auth, Body string
}

// fakeJira serves every path with a JSON object and records the requests.
type fakeJira struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []seen
}

func newFakeJira(t *testing.T, handler http.HandlerFunc) *fakeJira {
	t.Helper()
	f := &fakeJira{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization"), string(body)})
		f.mu.Unlock()
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"PROJ-7","ok":true}`))
	}))
	t.Cleanup(f.srv.Close)
	t.Setenv("JIRA_URL", f.srv.URL)
	t.Setenv("JIRA_TOKEN", "tok-secret")
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_API_TOKEN", "")
	return f
}

func (f *fakeJira) requests() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.reqs...)
}

func args(t *testing.T, kv map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(kv)
	require.NoError(t, err)
	return b
}

// Every tool asks Jira for the expected path (S-32, A-09).
func TestToolsRequestTheExpectedPath(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		call   func(json.RawMessage) error
		args   map[string]any
		method string
		path   string
		query  string
		body   string // substring of the body
	}{
		{"list_issues", func(a json.RawMessage) error { _, err := handleListIssues(ctx, a); return err },
			map[string]any{"jql": `project = "A B" AND status = Done`, "max_results": 5}, "GET", "/rest/api/3/search", "jql=project+%3D+%22A+B%22+AND+status+%3D+Done&maxResults=5", ""},
		{"get_issue", func(a json.RawMessage) error { _, err := handleGetIssue(ctx, a); return err },
			map[string]any{"issue_key": "PROJ-123"}, "GET", "/rest/api/3/issue/PROJ-123", "", ""},
		{"get_project", func(a json.RawMessage) error { _, err := handleGetProject(ctx, a); return err },
			map[string]any{"project_key": "MY_PROJ2"}, "GET", "/rest/api/3/project/MY_PROJ2", "", ""},
		{"list_comments", func(a json.RawMessage) error { _, err := handleListComments(ctx, a); return err },
			map[string]any{"issue_key": "PROJ-123"}, "GET", "/rest/api/3/issue/PROJ-123/comment", "", ""},
		{"transition_issue", func(a json.RawMessage) error { _, err := handleTransitionIssue(ctx, a); return err },
			map[string]any{"issue_key": "PROJ-123", "transition_id": "31"}, "POST", "/rest/api/3/issue/PROJ-123/transitions", "", `"id":"31"`},
		{"create_issue", func(a json.RawMessage) error { _, err := handleCreateIssue(ctx, a); return err },
			map[string]any{"project_key": "PROJ", "summary": "S", "labels": []string{"a"}}, "POST", "/rest/api/2/issue", "", `"key":"PROJ"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeJira(t, nil)
			require.NoError(t, tc.call(args(t, tc.args)))
			got := f.requests()
			require.Len(t, got, 1)
			assert.Equal(t, tc.method, got[0].Method)
			assert.Equal(t, tc.path, got[0].Path)
			assert.Equal(t, tc.query, got[0].RawQuery)
			assert.Contains(t, got[0].Body, tc.body)
			assert.Equal(t, "Bearer tok-secret", got[0].Auth)
		})
	}
}

// Identifiers that are not a key reach no resource: no request leaves.
func TestTrappedIdentifiersSendNoRequest(t *testing.T) {
	ctx := context.Background()
	trapped := []string{
		"../../admin", "PROJ-1/../../x", "PROJ-1?foo=", "PROJ-1#frag", "PROJ-1%2F..%2Fadmin", "PROJ-1%3Ffoo",
		"PROJ-1 ", " PROJ-1", "PROJ 1", "PROJ-1\n", "proj-1", "PROJ-", "PROJ-1/comment", "PROJ-0x1", "PROJ-１", "", "..", ".", "X?foo=",
		"PROJ-1\x00", "PROJ\\-1", "http://evil.example/PROJ-1",
	}
	issueTools := map[string]func(json.RawMessage) error{
		"get_issue":        func(a json.RawMessage) error { _, err := handleGetIssue(ctx, a); return err },
		"list_comments":    func(a json.RawMessage) error { _, err := handleListComments(ctx, a); return err },
		"transition_issue": func(a json.RawMessage) error { _, err := handleTransitionIssue(ctx, a); return err },
	}
	f := newFakeJira(t, nil)
	for _, key := range trapped {
		for name, call := range issueTools {
			err := call(args(t, map[string]any{"issue_key": key, "transition_id": "1"}))
			require.Error(t, err, "%s %q", name, key)
			assert.Contains(t, err.Error(), "invalid issue key", "%s %q", name, key)
		}
		_, err := handleGetProject(ctx, args(t, map[string]any{"project_key": key}))
		require.Error(t, err, "get_project %q", key)
		_, err = handleCreateIssue(ctx, args(t, map[string]any{"project_key": key, "summary": "s"}))
		require.Error(t, err, "create_issue %q", key)
	}
	assert.Empty(t, f.requests(), "no request reached Jira")
}

func TestProjectKeyIsNotAnIssueKey(t *testing.T) {
	f := newFakeJira(t, nil)
	_, err := handleGetIssue(context.Background(), args(t, map[string]any{"issue_key": "PROJ"}))
	require.Error(t, err)
	_, err = handleGetProject(context.Background(), args(t, map[string]any{"project_key": "PROJ-1"}))
	require.Error(t, err)
	assert.Empty(t, f.requests())
}

// The query goes through url.Values: a JQL cannot add a parameter.
func TestJQLIsEscaped(t *testing.T) {
	f := newFakeJira(t, nil)
	_, err := handleListIssues(context.Background(), args(t, map[string]any{"jql": "a&maxResults=1000#x", "max_results": -4}))
	require.NoError(t, err)
	got := f.requests()
	require.Len(t, got, 1)
	assert.Equal(t, "jql=a%26maxResults%3D1000%23x&maxResults=50", got[0].RawQuery)
}

// The credential goes to the configured server only: with a token or with
// basic auth, never to the server a redirect names.
func TestCredentialGoesToTheConfiguredServerOnly(t *testing.T) {
	other := newFakeJira(t, nil)
	otherURL := other.srv.URL

	f := newFakeJira(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherURL+"/stolen", http.StatusFound)
	})
	// Another host name than the target of the redirect (the standard client
	// forwards the credential between ports of one host).
	t.Setenv("JIRA_URL", strings.Replace(f.srv.URL, "127.0.0.1", "localhost", 1))
	_, _ = handleGetIssue(context.Background(), args(t, map[string]any{"issue_key": "PROJ-1"}))

	require.Len(t, f.requests(), 1)
	assert.Equal(t, "Bearer tok-secret", f.requests()[0].Auth)
	for _, r := range other.requests() {
		assert.Empty(t, r.Auth, "the redirect target got the credential: %+v", r)
	}

	t.Run("basic auth", func(t *testing.T) {
		g := newFakeJira(t, nil)
		t.Setenv("JIRA_TOKEN", "")
		t.Setenv("JIRA_USER", "me@example.com")
		t.Setenv("JIRA_API_TOKEN", "api-token")
		_, err := handleGetIssue(context.Background(), args(t, map[string]any{"issue_key": "PROJ-1"}))
		require.NoError(t, err)
		require.Len(t, g.requests(), 1)
		assert.True(t, strings.HasPrefix(g.requests()[0].Auth, "Basic "))
	})

	t.Run("no JIRA_URL, no request", func(t *testing.T) {
		g := newFakeJira(t, nil)
		t.Setenv("JIRA_URL", "")
		_, err := handleGetIssue(context.Background(), args(t, map[string]any{"issue_key": "PROJ-1"}))
		require.Error(t, err)
		assert.Empty(t, g.requests())
	})
}

func TestCreateIssueAnswersWithTheBrowseURL(t *testing.T) {
	f := newFakeJira(t, nil)
	res, err := handleCreateIssue(context.Background(), args(t, map[string]any{"project_key": "PROJ", "summary": "S"}))
	require.NoError(t, err)
	assert.Equal(t, "Created issue PROJ-7: "+f.srv.URL+"/browse/PROJ-7", res.Content[0].Text)
}
