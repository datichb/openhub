package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// fakeGitLab answers the API paths of routes (path → body) and records each
// request URL. A path absent from routes answers 404.
func fakeGitLab(t *testing.T, routes map[string]string, headers map[string]http.Header) (requests *[]*url.URL) {
	t.Helper()
	var seen []*url.URL
	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL)
		body, ok := routes[r.URL.EscapedPath()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
			return
		}
		for k, vs := range headers[r.URL.EscapedPath()] {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		_, _ = w.Write([]byte(body))
	})
	return &seen
}

func call(t *testing.T, h protocol.Handler, args any) string {
	t.Helper()
	params, err := json.Marshal(args)
	require.NoError(t, err)
	res, err := h(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, res.Content, 1)
	return res.Content[0].Text
}

const issue623 = `{
  "id": 9001, "iid": 623, "project_id": 42,
  "title": "Export CSV des factures",
  "description": "En tant que comptable, je veux exporter les factures.\n\n- [ ] colonnes TVA",
  "state": "opened",
  "labels": ["feature", "DEV::DOING"],
  "assignees": [{"id": 7, "username": "benjamin.datiche", "name": "Benjamin D."}],
  "author": {"id": 3, "username": "alice", "name": "Alice"},
  "milestone": {"id": 5, "title": "Sprint 12", "due_date": "2026-11-06", "state": "active"},
  "web_url": "https://gitlab.example.com/group/app/-/issues/623",
  "references": {"short": "#623", "relative": "#623", "full": "group/app#623"},
  "created_at": "2026-10-01T08:00:00Z", "updated_at": "2026-10-20T09:00:00Z",
  "user_notes_count": 3, "confidential": false
}`

const issue623Notes = `[
  {"id": 1, "body": "assigned to @benjamin.datiche", "system": true, "author": {"username": "alice"}, "created_at": "2026-10-02T08:00:00Z"},
  {"id": 2, "body": "Précision : séparateur point-virgule", "system": false, "author": {"username": "alice", "name": "Alice"}, "created_at": "2026-10-03T08:00:00Z"},
  {"id": 3, "body": "Ok, noté", "system": false, "author": {"username": "benjamin.datiche", "name": "Benjamin D."}, "created_at": "2026-10-04T08:00:00Z"}
]`

// Bug 20: no tool read an issue by its IID.
func TestGetIssueReadsOneIssueByIID(t *testing.T) {
	reqs := fakeGitLab(t, map[string]string{
		"/api/v4/projects/group%2Fapp/issues/623":                        issue623,
		"/api/v4/projects/group%2Fapp/issues/623/notes":                  issue623Notes,
		"/api/v4/projects/group%2Fapp/issues/623/links":                  `[{"iid": 600, "title": "Modèle facture", "state": "closed", "link_type": "relates_to", "web_url": "https://gitlab.example.com/group/app/-/issues/600", "project_id": 42}]`,
		"/api/v4/projects/group%2Fapp/issues/623/related_merge_requests": `[{"iid": 70, "title": "feat: export CSV", "state": "opened", "source_branch": "feat/623-csv", "web_url": "https://gitlab.example.com/group/app/-/merge_requests/70"}]`,
	}, nil)

	out := call(t, handleGetIssue, map[string]any{"project_id": "group/app", "iid": 623})

	var got struct {
		IID         int      `json:"iid"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		State       string   `json:"state"`
		Labels      []string `json:"labels"`
		Assignees   []struct {
			Username string `json:"username"`
		} `json:"assignees"`
		Milestone struct {
			Title string `json:"title"`
		} `json:"milestone"`
		WebURL string `json:"web_url"`
		Notes  []struct {
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"notes"`
		RelatedIssues []struct {
			IID int `json:"iid"`
		} `json:"related_issues"`
		RelatedMRs []struct {
			IID          int    `json:"iid"`
			SourceBranch string `json:"source_branch"`
		} `json:"related_merge_requests"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, 623, got.IID)
	assert.Equal(t, "Export CSV des factures", got.Title)
	assert.Contains(t, got.Description, "colonnes TVA")
	assert.Equal(t, "opened", got.State)
	assert.Equal(t, []string{"feature", "DEV::DOING"}, got.Labels)
	require.Len(t, got.Assignees, 1)
	assert.Equal(t, "benjamin.datiche", got.Assignees[0].Username)
	assert.Equal(t, "Sprint 12", got.Milestone.Title)
	assert.Equal(t, "https://gitlab.example.com/group/app/-/issues/623", got.WebURL)
	require.Len(t, got.Notes, 2, "system notes are left out")
	assert.Equal(t, "alice", got.Notes[0].Author)
	assert.Contains(t, got.Notes[0].Body, "point-virgule")
	require.Len(t, got.RelatedIssues, 1)
	assert.Equal(t, 600, got.RelatedIssues[0].IID)
	require.Len(t, got.RelatedMRs, 1)
	assert.Equal(t, "feat/623-csv", got.RelatedMRs[0].SourceBranch)
	assert.NotEmpty(t, *reqs)
}

func TestGetIssueAcceptsLooseIIDs(t *testing.T) {
	fakeGitLab(t, map[string]string{
		"/api/v4/projects/42/issues/623":       issue623,
		"/api/v4/projects/42/issues/623/notes": `[]`,
	}, nil)
	for name, iid := range map[string]any{"number": 623, "string": "623", "hash": "#623", "issue_iid": nil} {
		t.Run(name, func(t *testing.T) {
			args := map[string]any{"project_id": "42", "iid": iid}
			if name == "issue_iid" {
				args = map[string]any{"project_id": "42", "issue_iid": 623}
			}
			assert.Contains(t, call(t, handleGetIssue, args), `"iid": 623`)
		})
	}
}

func TestGetIssueWithoutNotes(t *testing.T) {
	reqs := fakeGitLab(t, map[string]string{"/api/v4/projects/42/issues/623": issue623}, nil)
	call(t, handleGetIssue, map[string]any{"project_id": "42", "iid": 623, "include_notes": false})
	for _, u := range *reqs {
		assert.NotContains(t, u.Path, "/notes")
	}
}

func TestGetIssueSideLookupsDoNotFailTheCall(t *testing.T) {
	// notes, links and related MRs answer 404: the issue is still returned,
	// and the gaps are said.
	fakeGitLab(t, map[string]string{"/api/v4/projects/42/issues/623": issue623}, nil)
	out := call(t, handleGetIssue, map[string]any{"project_id": "42", "iid": 623})
	assert.Contains(t, out, "Export CSV des factures")
	assert.Contains(t, out, "notes_error")
	assert.Contains(t, out, "links_error")
}

func TestGetIssueNotFoundSaysWhat(t *testing.T) {
	fakeGitLab(t, map[string]string{}, nil)
	params, _ := json.Marshal(map[string]any{"project_id": "group/app", "iid": 999})
	_, err := handleGetIssue(context.Background(), params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issue #999")
	assert.Contains(t, err.Error(), "group/app")
}

func TestGetIssueRequiresProjectAndIID(t *testing.T) {
	fakeGitLab(t, map[string]string{}, nil)
	for name, args := range map[string]map[string]any{
		"no project": {"iid": 1},
		"no iid":     {"project_id": "42"},
		"zero iid":   {"project_id": "42", "iid": 0},
		"not a iid":  {"project_id": "42", "iid": "abc"},
	} {
		t.Run(name, func(t *testing.T) {
			params, _ := json.Marshal(args)
			_, err := handleGetIssue(context.Background(), params)
			require.Error(t, err)
		})
	}
}

func TestGetIssueTruncatesNotes(t *testing.T) {
	var notes []string
	for i := 1; i <= 5; i++ {
		notes = append(notes, `{"id": `+string(rune('0'+i))+`, "body": "n", "system": false, "author": {"username": "a"}}`)
	}
	fakeGitLab(t, map[string]string{
		"/api/v4/projects/42/issues/623":       issue623,
		"/api/v4/projects/42/issues/623/notes": "[" + strings.Join(notes, ",") + "]",
	}, nil)
	out := call(t, handleGetIssue, map[string]any{"project_id": "42", "iid": 623, "notes_limit": 2})
	var got struct {
		Notes          []json.RawMessage `json:"notes"`
		NotesTruncated bool              `json:"notes_truncated"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Len(t, got.Notes, 2)
	assert.True(t, got.NotesTruncated)
}

// Bug 20: list_issues had no IID filter and a single page.
func TestListIssuesFiltersAndPagination(t *testing.T) {
	reqs := fakeGitLab(t,
		map[string]string{"/api/v4/projects/group%2Fapp/issues": `[
			{"iid": 623, "title": "Export CSV", "description": "long", "state": "opened", "labels": ["feature"], "assignees": [{"username": "bd"}], "author": {"username": "alice"}, "milestone": {"title": "Sprint 12"}, "web_url": "https://x/623", "updated_at": "2026-10-20T09:00:00Z"},
			{"iid": 624, "title": "Autre", "state": "opened", "labels": [], "assignees": [], "web_url": "https://x/624"}
		]`},
		map[string]http.Header{"/api/v4/projects/group%2Fapp/issues": {
			"X-Total": {"45"}, "X-Total-Pages": {"3"}, "X-Page": {"2"}, "X-Per-Page": {"20"}, "X-Next-Page": {"3"}, "X-Prev-Page": {"1"},
		}})

	out := call(t, handleListIssues, map[string]any{
		"project_id": "group/app", "iids": []int{623, 624}, "search": "export",
		"assignee_username": "bd", "labels": "feature,DEV::DOING", "state": "opened", "page": 2, "per_page": 20,
	})

	require.Len(t, *reqs, 1)
	q := (*reqs)[0].Query()
	assert.Equal(t, []string{"623", "624"}, q["iids[]"])
	assert.Equal(t, "export", q.Get("search"))
	assert.Equal(t, "bd", q.Get("assignee_username"))
	assert.Equal(t, "feature,DEV::DOING", q.Get("labels"))
	assert.Equal(t, "opened", q.Get("state"))
	assert.Equal(t, "2", q.Get("page"))
	assert.Equal(t, "20", q.Get("per_page"))

	var got struct {
		Issues []struct {
			IID       int      `json:"iid"`
			Title     string   `json:"title"`
			Assignees []string `json:"assignees"`
			Milestone string   `json:"milestone"`
			Desc      string   `json:"description"`
		} `json:"issues"`
		Page       int  `json:"page"`
		PerPage    int  `json:"per_page"`
		Total      *int `json:"total"`
		TotalPages *int `json:"total_pages"`
		NextPage   *int `json:"next_page"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	require.Len(t, got.Issues, 2)
	assert.Equal(t, 623, got.Issues[0].IID)
	assert.Equal(t, []string{"bd"}, got.Issues[0].Assignees)
	assert.Equal(t, "Sprint 12", got.Issues[0].Milestone)
	assert.Empty(t, got.Issues[0].Desc, "the description is read with get_issue")
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, 20, got.PerPage)
	require.NotNil(t, got.Total)
	assert.Equal(t, 45, *got.Total)
	require.NotNil(t, got.TotalPages)
	assert.Equal(t, 3, *got.TotalPages)
	require.NotNil(t, got.NextPage)
	assert.Equal(t, 3, *got.NextPage)
}

func TestListIssuesDefaultsAndLastPage(t *testing.T) {
	reqs := fakeGitLab(t,
		map[string]string{"/api/v4/projects/42/issues": `[]`},
		map[string]http.Header{"/api/v4/projects/42/issues": {"X-Page": {"1"}, "X-Next-Page": {""}}})
	out := call(t, handleListIssues, map[string]any{"project_id": "42"})
	q := (*reqs)[0].Query()
	assert.Equal(t, "1", q.Get("page"))
	assert.Equal(t, "20", q.Get("per_page"), "a bounded page by default")
	assert.Empty(t, q.Get("state"))
	assert.Contains(t, out, `"issues": []`)
	assert.Contains(t, out, `"next_page": null`)
	assert.Contains(t, out, `"total": null`, "GitLab omits the total past 10 000 results")
}

func TestListIssuesBoundsThePageSize(t *testing.T) {
	reqs := fakeGitLab(t, map[string]string{"/api/v4/projects/42/issues": `[]`}, nil)
	call(t, handleListIssues, map[string]any{"project_id": "42", "per_page": 5000, "page": -3})
	q := (*reqs)[0].Query()
	assert.Equal(t, "100", q.Get("per_page"))
	assert.Equal(t, "1", q.Get("page"))
}

func TestGetMR(t *testing.T) {
	fakeGitLab(t, map[string]string{
		"/api/v4/projects/group%2Fapp/merge_requests/70": `{
			"iid": 70, "title": "feat: export CSV", "description": "Closes #623", "state": "opened", "draft": false,
			"source_branch": "feat/623-csv", "target_branch": "main", "sha": "abc123",
			"author": {"username": "alice"}, "assignees": [{"username": "bd"}], "reviewers": [{"username": "carl"}],
			"labels": ["feature"], "milestone": {"title": "Sprint 12"},
			"detailed_merge_status": "mergeable", "has_conflicts": false,
			"web_url": "https://gitlab.example.com/group/app/-/merge_requests/70"}`,
		"/api/v4/projects/group%2Fapp/merge_requests/70/closes_issues": `[{"iid": 623, "title": "Export CSV des factures", "state": "opened", "web_url": "https://gitlab.example.com/group/app/-/issues/623"}]`,
	}, nil)

	out := call(t, handleGetMR, map[string]any{"project_id": "group/app", "mr_iid": 70})

	var got struct {
		IID          int      `json:"iid"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		SourceBranch string   `json:"source_branch"`
		TargetBranch string   `json:"target_branch"`
		Author       string   `json:"author"`
		Reviewers    []string `json:"reviewers"`
		MergeStatus  string   `json:"merge_status"`
		ClosesIssues []struct {
			IID int `json:"iid"`
		} `json:"closes_issues"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, 70, got.IID)
	assert.Equal(t, "feat/623-csv", got.SourceBranch)
	assert.Equal(t, "main", got.TargetBranch)
	assert.Equal(t, "alice", got.Author)
	assert.Equal(t, []string{"carl"}, got.Reviewers)
	assert.Equal(t, "mergeable", got.MergeStatus)
	assert.Equal(t, "Closes #623", got.Description)
	require.Len(t, got.ClosesIssues, 1)
	assert.Equal(t, 623, got.ClosesIssues[0].IID)
}

func TestGetMRNotFoundSaysWhat(t *testing.T) {
	fakeGitLab(t, map[string]string{}, nil)
	params, _ := json.Marshal(map[string]any{"project_id": "group/app", "mr_iid": 5})
	_, err := handleGetMR(context.Background(), params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge request !5")
}

func TestListMRsFiltersAndPagination(t *testing.T) {
	reqs := fakeGitLab(t,
		map[string]string{"/api/v4/projects/42/merge_requests": `[{"iid": 70, "title": "feat", "state": "opened", "source_branch": "b", "target_branch": "main", "author": {"username": "alice"}, "web_url": "https://x/70"}]`},
		map[string]http.Header{"/api/v4/projects/42/merge_requests": {"X-Total": {"1"}, "X-Total-Pages": {"1"}, "X-Page": {"1"}, "X-Next-Page": {""}}})
	out := call(t, handleListMRs, map[string]any{"project_id": "42", "iids": []int{70}, "source_branch": "b", "state": "opened", "search": "feat"})
	q := (*reqs)[0].Query()
	assert.Equal(t, []string{"70"}, q["iids[]"])
	assert.Equal(t, "b", q.Get("source_branch"))
	assert.Equal(t, "feat", q.Get("search"))
	assert.Equal(t, "opened", q.Get("state"))
	var got struct {
		MergeRequests []struct {
			IID          int    `json:"iid"`
			SourceBranch string `json:"source_branch"`
			Author       string `json:"author"`
		} `json:"merge_requests"`
		Total *int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	require.Len(t, got.MergeRequests, 1)
	assert.Equal(t, "alice", got.MergeRequests[0].Author)
	require.NotNil(t, got.Total)
	assert.Equal(t, 1, *got.Total)
}

type schema struct {
	Properties map[string]map[string]any
	Required   []string
}

// toolSchemas lists the tools of the server with their input properties and
// required fields, as a client sees them.
func toolSchemas(t *testing.T) map[string]schema {
	t.Helper()
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	require.NoError(t, newServer().ServeIO(context.Background(), in, &out))
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]map[string]any `json:"properties"`
					Required   []string                  `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &resp), out.String())
	got := map[string]schema{}
	for _, tool := range resp.Result.Tools {
		got[tool.Name] = schema{tool.InputSchema.Properties, tool.InputSchema.Required}
	}
	return got
}

func TestReadToolsAreListed(t *testing.T) {
	t.Setenv(EnvWriteEnabled, "")
	tools := toolSchemas(t)
	for _, want := range []string{"get_project", "get_issue", "list_issues", "get_mr", "list_mrs", "list_mr_discussions", "get_mr_approvals"} {
		assert.Contains(t, tools, want)
	}
	assert.NotContains(t, tools, "create_mr", "write tools stay off by default")

	assert.ElementsMatch(t, []string{"project_id", "iid"}, tools["get_issue"].Required)
	for _, p := range []string{"project_id", "iid", "include_notes", "notes_limit"} {
		assert.Contains(t, tools["get_issue"].Properties, p)
	}
	assert.Equal(t, "integer", tools["get_issue"].Properties["iid"]["type"])

	assert.Equal(t, []string{"project_id"}, tools["list_issues"].Required)
	for _, p := range []string{"project_id", "iids", "search", "assignee_username", "labels", "state", "page", "per_page"} {
		assert.Contains(t, tools["list_issues"].Properties, p)
	}
	assert.Equal(t, "array", tools["list_issues"].Properties["iids"]["type"])

	assert.ElementsMatch(t, []string{"project_id", "mr_iid"}, tools["get_mr"].Required)
}
