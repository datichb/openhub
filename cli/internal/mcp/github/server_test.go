package github

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

const headSHA = "0123456789abcdef0123456789abcdef01234567"

// seen is one request received by the fake GitHub.
type seen struct {
	Method, Path, RawQuery, Auth, Body string
}

type fakeGitHub struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []seen
}

// newFakeGitHub stands for api.github.com: it answers the pull request with
// a head commit, everything else with an empty object.
func newFakeGitHub(t *testing.T, handler http.HandlerFunc) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{}
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
		if strings.HasSuffix(r.URL.Path, "/pulls/42") {
			_, _ = w.Write([]byte(`{"head":{"sha":"` + headSHA + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(f.srv.Close)
	old := apiBase
	apiBase = f.srv.URL
	t.Cleanup(func() { apiBase = old })
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("GH_TOKEN", "")
	return f
}

func (f *fakeGitHub) requests() []seen {
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

// Every tool asks GitHub for the expected path (S-32, A-09).
func TestToolsRequestTheExpectedPath(t *testing.T) {
	ctx := context.Background()
	repo := map[string]any{"repo": "octo-org/Hello_World.js"}
	with := func(kv map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range repo {
			out[k] = v
		}
		for k, v := range kv {
			out[k] = v
		}
		return out
	}
	const base = "/repos/octo-org/Hello_World.js"
	tests := []struct {
		name  string
		call  func(json.RawMessage) error
		args  map[string]any
		want  []string // "METHOD path?query"
		write bool
	}{
		{"get_repo", func(a json.RawMessage) error { _, err := handleGetRepo(ctx, a); return err }, with(nil),
			[]string{"GET " + base}, false},
		{"list_issues", func(a json.RawMessage) error { _, err := handleListIssues(ctx, a); return err },
			with(map[string]any{"labels": "bug,help wanted", "assignee": "octocat"}),
			[]string{"GET " + base + "/issues?assignee=octocat&labels=bug%2Chelp+wanted&per_page=50&state=open"}, false},
		{"get_issue", func(a json.RawMessage) error { _, err := handleGetIssue(ctx, a); return err }, with(map[string]any{"issue_number": 7}),
			[]string{"GET " + base + "/issues/7"}, false},
		{"list_prs", func(a json.RawMessage) error { _, err := handleListPRs(ctx, a); return err }, with(map[string]any{"state": "all", "base": "main"}),
			[]string{"GET " + base + "/pulls?base=main&per_page=30&state=all"}, false},
		{"get_pr", func(a json.RawMessage) error { _, err := handleGetPR(ctx, a); return err }, with(map[string]any{"pr_number": 42}),
			[]string{"GET " + base + "/pulls/42", "GET " + base + "/commits/" + headSHA + "/check-runs", "GET " + base + "/pulls/42/reviews"}, false},
		{"list_workflows", func(a json.RawMessage) error { _, err := handleListWorkflows(ctx, a); return err }, with(nil),
			[]string{"GET " + base + "/actions/workflows"}, false},
		{"get_workflow_run", func(a json.RawMessage) error { _, err := handleGetWorkflowRun(ctx, a); return err }, with(map[string]any{"workflow_id": "ci.yml"}),
			[]string{"GET " + base + "/actions/workflows/ci.yml/runs?per_page=1"}, false},
		{"create_issue", func(a json.RawMessage) error { _, err := handleCreateIssue(ctx, a); return err },
			with(map[string]any{"title": "T", "body": "B", "labels": []string{"bug"}}),
			[]string{"POST " + base + "/issues"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitHub(t, nil)
			require.NoError(t, tc.call(args(t, tc.args)))
			var got []string
			for _, r := range f.requests() {
				line := r.Method + " " + r.Path
				if r.RawQuery != "" {
					line += "?" + r.RawQuery
				}
				got = append(got, line)
				assert.Equal(t, "Bearer gh-secret", r.Auth)
			}
			assert.ElementsMatch(t, tc.want, got)
			if tc.write {
				assert.JSONEq(t, `{"title":"T","body":"B","labels":["bug"]}`, f.requests()[0].Body)
			}
		})
	}
}

// Identifiers that are not what the API expects reach no resource: no
// request leaves.
func TestTrappedIdentifiersSendNoRequest(t *testing.T) {
	ctx := context.Background()
	f := newFakeGitHub(t, nil)
	repos := []string{
		"", "octo", "octo/", "/repo", "octo/repo/extra", "../../admin", "octo/..", "octo/.", "octo/../x", "../repo",
		"octo/repo?x=1", "octo/repo#frag", "octo/repo%2F..%2Fadmin", "octo%2Frepo", "octo/re po", " octo/repo", "octo/repo ", "octo/repo\n",
		"-octo/repo", "oc to/repo", "octo/repo/../..", "http://evil.example/a/b", "octo:80/repo", "octo/rép",
	}
	for _, repo := range repos {
		a := func(kv map[string]any) json.RawMessage {
			kv["repo"] = repo
			return args(t, kv)
		}
		calls := map[string]error{}
		_, calls["get_repo"] = handleGetRepo(ctx, a(map[string]any{}))
		_, calls["list_issues"] = handleListIssues(ctx, a(map[string]any{}))
		_, calls["get_issue"] = handleGetIssue(ctx, a(map[string]any{"issue_number": 1}))
		_, calls["list_prs"] = handleListPRs(ctx, a(map[string]any{}))
		_, calls["get_pr"] = handleGetPR(ctx, a(map[string]any{"pr_number": 1}))
		_, calls["list_workflows"] = handleListWorkflows(ctx, a(map[string]any{}))
		_, calls["get_workflow_run"] = handleGetWorkflowRun(ctx, a(map[string]any{"workflow_id": "ci.yml"}))
		_, calls["create_issue"] = handleCreateIssue(ctx, a(map[string]any{"title": "t"}))
		for name, err := range calls {
			require.Error(t, err, "%s %q", name, repo)
			assert.Contains(t, err.Error(), "invalid repo", "%s %q", name, repo)
		}
	}

	for _, n := range []int{0, -1, -42} {
		_, err := handleGetIssue(ctx, args(t, map[string]any{"repo": "o/r", "issue_number": n}))
		require.Error(t, err, "issue %d", n)
		_, err = handleGetPR(ctx, args(t, map[string]any{"repo": "o/r", "pr_number": n}))
		require.Error(t, err, "pr %d", n)
	}
	for _, raw := range []string{`"1/../../x"`, `1.5`, `"7"`, `null`} {
		_, err := handleGetIssue(ctx, json.RawMessage(`{"repo":"o/r","issue_number":`+raw+`}`))
		require.Error(t, err, raw)
	}

	for _, id := range []string{"", "../x", "ci.yml/../../x", "ci.yml?x=1", "ci.yml#x", "ci%2Fyml", "ci yml", "ci.yml\n", ".hidden", "-ci", "a/b", "ci.yml/runs"} {
		_, err := handleGetWorkflowRun(ctx, args(t, map[string]any{"repo": "o/r", "workflow_id": id}))
		require.Error(t, err, "workflow %q", id)
		assert.Contains(t, err.Error(), "invalid workflow_id", "workflow %q", id)
	}
	assert.Empty(t, f.requests(), "no request reached GitHub")
}

// Query parameters go through url.Values: a value cannot add a parameter.
func TestQueryValuesAreEscaped(t *testing.T) {
	f := newFakeGitHub(t, nil)
	_, err := handleListIssues(context.Background(), args(t, map[string]any{"repo": "o/r", "state": "open&per_page=100", "assignee": "a#b"}))
	require.NoError(t, err)
	got := f.requests()
	require.Len(t, got, 1)
	assert.Equal(t, "/repos/o/r/issues", got[0].Path)
	assert.Equal(t, "assignee=a%23b&per_page=50&state=open%26per_page%3D100", got[0].RawQuery)
}

// The commit of a pull request comes from the API: it is checked too.
func TestHeadSHAIsChecked(t *testing.T) {
	f := newFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"head":{"sha":"../../admin"}}`))
	})
	_, err := handleGetPR(context.Background(), args(t, map[string]any{"repo": "o/r", "pr_number": 3}))
	require.NoError(t, err)
	for _, r := range f.requests() {
		assert.NotContains(t, r.Path, "admin")
		assert.NotContains(t, r.Path, "check-runs")
	}
}

func TestOwnersAndReposGitHubAccepts(t *testing.T) {
	for _, repo := range []string{"octocat/Hello-World", "a/b", "octo_corp/x.y_z-1", "o-r-g/.github", "O/r.js", "octo-org/Hello_World.js"} {
		p, err := repoPath(repo)
		require.NoError(t, err, repo)
		assert.Equal(t, "/repos/"+repo, p)
	}
}

// The credential goes to the configured server only, never to the server a
// redirect names.
func TestCredentialGoesToTheConfiguredServerOnly(t *testing.T) {
	other := newFakeGitHub(t, nil)
	otherURL := other.srv.URL
	f := newFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherURL+"/stolen", http.StatusFound)
	})
	// Another host name than the target of the redirect (the standard client
	// forwards the credential between ports of one host).
	apiBase = strings.Replace(f.srv.URL, "127.0.0.1", "localhost", 1)
	_, _ = handleGetRepo(context.Background(), args(t, map[string]any{"repo": "o/r"}))

	require.Len(t, f.requests(), 1)
	assert.Equal(t, "Bearer gh-secret", f.requests()[0].Auth)
	for _, r := range other.requests() {
		assert.Empty(t, r.Auth, "the redirect target got the credential: %+v", r)
	}
}

func TestDefaultServerIsGitHub(t *testing.T) {
	assert.Equal(t, "https://api.github.com", githubAPIBase)
	assert.Equal(t, githubAPIBase, apiBase)
}
