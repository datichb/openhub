package gitlabapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectID used across tests — contains a slash to exercise PathEscape.
const testProjectID = "my/project"

// mrListPath is the URL path prefix for listing MRs on the test project.
// Note: httptest's server decodes %2F in the path, so r.URL.Path sees "my/project".
const mrListPath = "/api/v4/projects/my/project/merge_requests"

// usersPath is the URL path prefix for the users endpoint.
const usersPath = "/api/v4/users"

// --------------------------------------------------------------------------
// TestCreateMR_Existing
// --------------------------------------------------------------------------

func TestCreateMR_Existing(t *testing.T) {
	var postCalled atomic.Bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, mrListPath):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]MRInfo{{
				IID:          42,
				WebURL:       "https://example.com/mr/42",
				Title:        "existing",
				State:        "opened",
				TargetBranch: "main",
			}})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, mrListPath):
			postCalled.Store(true)
			http.Error(w, "unexpected POST", http.StatusInternalServerError)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	mr, err := c.CreateMR(context.Background(), testProjectID, "feat/x", "main", "new MR", "")

	require.NoError(t, err)
	assert.Equal(t, 42, mr.IID)
	assert.Equal(t, "existing", mr.Title)
	assert.Equal(t, "https://example.com/mr/42", mr.WebURL)
	assert.False(t, postCalled.Load(), "POST should not be called when an existing MR is found")
}

// --------------------------------------------------------------------------
// TestCreateMR_New
// --------------------------------------------------------------------------

func TestCreateMR_New(t *testing.T) {
	var capturedToken string
	var capturedContentType string
	var postBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture auth header on every request.
		capturedToken = r.Header.Get("PRIVATE-TOKEN")

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, mrListPath):
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]")) // no existing MR

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, mrListPath):
			capturedContentType = r.Header.Get("Content-Type")

			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(MRInfo{
				IID:          43,
				WebURL:       "https://example.com/mr/43",
				Title:        "new MR",
				State:        "opened",
				TargetBranch: "main",
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	mr, err := c.CreateMR(context.Background(), testProjectID, "feat/y", "main", "new MR", "description")

	require.NoError(t, err)
	assert.Equal(t, 43, mr.IID)
	assert.Equal(t, "new MR", mr.Title)

	// Verify POST body.
	assert.Equal(t, "feat/y", postBody["source_branch"])
	assert.Equal(t, "main", postBody["target_branch"])
	assert.Equal(t, "new MR", postBody["title"])

	// Header verification.
	assert.Equal(t, "test-token", capturedToken, "PRIVATE-TOKEN header must be set")
	assert.Equal(t, "application/json", capturedContentType, "POST Content-Type must be application/json")
}

// --------------------------------------------------------------------------
// TestFindMRByBranch_Found
// --------------------------------------------------------------------------

func TestFindMRByBranch_Found(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, mrListPath) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]MRInfo{{
				IID:          1,
				WebURL:       "https://example.com/mr/1",
				Title:        "mr",
				State:        "opened",
				TargetBranch: "main",
			}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	mr, err := c.FindMRByBranch(context.Background(), testProjectID, "feat/z")

	require.NoError(t, err)
	require.NotNil(t, mr)
	assert.Equal(t, 1, mr.IID)
}

// --------------------------------------------------------------------------
// TestFindMRByBranch_NotFound
// --------------------------------------------------------------------------

func TestFindMRByBranch_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, mrListPath) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	mr, err := c.FindMRByBranch(context.Background(), testProjectID, "no-such-branch")

	assert.NoError(t, err)
	assert.Nil(t, mr)
}

// --------------------------------------------------------------------------
// TestListMRDiscussions_FilterUnresolved
// --------------------------------------------------------------------------

func TestListMRDiscussions_FilterUnresolved(t *testing.T) {
	discussionsJSON := []Discussion{
		{
			// Discussion 1: single system note — should be filtered out entirely.
			ID: "d1",
			Notes: []Note{
				{ID: 1, Body: "system note", System: true, Resolvable: false},
			},
		},
		{
			// Discussion 2: one resolved human note.
			ID: "d2",
			Notes: []Note{
				{ID: 2, Body: "resolved comment", System: false, Resolvable: true, Resolved: true},
			},
		},
		{
			// Discussion 3: one unresolved human note with Position.
			ID: "d3",
			Notes: []Note{
				{
					ID: 3, Body: "unresolved comment", System: false,
					Resolvable: true, Resolved: false,
					Position: &NotePos{NewPath: "main.go", NewLine: 10},
				},
			},
		},
	}

	discussionsPath := mrListPath + "/1/discussions"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == discussionsPath {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(discussionsJSON)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")

	t.Run("unresolvedOnly=true", func(t *testing.T) {
		result, err := c.ListMRDiscussions(context.Background(), testProjectID, 1, true)
		require.NoError(t, err)
		require.Len(t, result, 1, "only the unresolved discussion should be returned")
		assert.Equal(t, "d3", result[0].ID)
		assert.NotNil(t, result[0].Notes[0].Position)
	})

	t.Run("unresolvedOnly=false", func(t *testing.T) {
		result, err := c.ListMRDiscussions(context.Background(), testProjectID, 1, false)
		require.NoError(t, err)
		require.Len(t, result, 2, "resolved + unresolved discussions should be returned (system-only excluded)")
		ids := []string{result[0].ID, result[1].ID}
		assert.Contains(t, ids, "d2")
		assert.Contains(t, ids, "d3")
	})
}

// --------------------------------------------------------------------------
// TestResolveUserID_Found
// --------------------------------------------------------------------------

func TestResolveUserID_Found(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == usersPath {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"id":42}]`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	id, err := c.ResolveUserID(context.Background(), "alice")

	require.NoError(t, err)
	assert.Equal(t, 42, id)
}

// --------------------------------------------------------------------------
// TestResolveUserID_NotFound
// --------------------------------------------------------------------------

func TestResolveUserID_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == usersPath {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[]`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	id, err := c.ResolveUserID(context.Background(), "ghost")

	assert.Equal(t, 0, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --------------------------------------------------------------------------
// TestAuthError
// --------------------------------------------------------------------------

func TestAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "bad-token")

	// Any method should surface the auth error.
	_, err := c.FindMRByBranch(context.Background(), testProjectID, "feat/a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}
