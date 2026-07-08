package gitlab

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWriteTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	httpClient = srv.Client()
	t.Setenv("GITLAB_TOKEN", "test-token")
	t.Setenv("GITLAB_URL", srv.URL)
	t.Setenv("GITLAB_SKIP_URL_VALIDATION", "true")
}

func TestHandleCreateMR(t *testing.T) {
	var receivedMethod string
	var receivedBody []byte

	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		if r.Method == "POST" {
			receivedBody, _ = io.ReadAll(r.Body)
		}
		if r.Method == "GET" {
			// No existing MR
			w.Write([]byte("[]"))
			return
		}
		// POST response
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"iid": 234, "web_url": "https://gitlab.com/mr/234"}`))
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id":    "my-group/my-project",
		"source_branch": "feat/SRU-142",
		"target_branch": "main",
		"title":         "feat: implement user auth",
		"description":   "Implements SRU-142",
	})

	result, err := handleCreateMR(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content[0].Text, "234")
	assert.Equal(t, "POST", receivedMethod)
	assert.Contains(t, string(receivedBody), "feat/SRU-142")
}

func TestHandleCreateMRAlreadyExists(t *testing.T) {
	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Return existing MR
			w.Write([]byte(`[{"iid": 100, "web_url": "https://gitlab.com/mr/100", "title": "existing"}]`))
			return
		}
		// POST should NOT be called
		t.Error("POST should not be called when MR already exists")
		w.WriteHeader(http.StatusConflict)
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id":    "project",
		"source_branch": "feat/existing",
		"title":         "should not create",
	})

	result, err := handleCreateMR(params)
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "already exists")
	assert.Contains(t, result.Content[0].Text, "feat/existing")
}

func TestHandleAddMRNote(t *testing.T) {
	var receivedBody []byte

	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/merge_requests/42/notes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1, "body": "test note"}`))
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id": "123",
		"mr_iid":     42,
		"body":       "Review complete: LGTM",
	})

	result, err := handleAddMRNote(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, string(receivedBody), "Review complete: LGTM")
}

func TestHandleUpdateIssue(t *testing.T) {
	var receivedBody map[string]interface{}

	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/issues/10")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Write([]byte(`{"iid": 10, "state": "closed"}`))
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id":  "123",
		"issue_iid":   10,
		"state_event": "close",
		"add_labels":  "ai-reviewed,done",
	})

	result, err := handleUpdateIssue(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "close", receivedBody["state_event"])
	assert.Equal(t, "ai-reviewed,done", receivedBody["add_labels"])
}

func TestHandleAssignReviewer(t *testing.T) {
	var receivedBody map[string]interface{}

	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/merge_requests/55")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Write([]byte(`{"iid": 55, "reviewers": [{"id": 42}]}`))
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id":   "project",
		"mr_iid":       55,
		"reviewer_ids": []int{42, 43},
	})

	result, err := handleAssignReviewer(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	ids := receivedBody["reviewer_ids"].([]interface{})
	assert.Len(t, ids, 2)
}

func TestHandleAddLabel(t *testing.T) {
	var receivedBody map[string]interface{}

	setupWriteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Write([]byte(`{"iid": 10, "labels": ["ai-reviewed", "security"]}`))
	})

	params, _ := json.Marshal(map[string]interface{}{
		"project_id": "123",
		"issue_iid":  10,
		"labels":     "ai-reviewed,security",
	})

	result, err := handleAddLabel(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "ai-reviewed,security", receivedBody["add_labels"])
}

func TestIsWriteEnabled(t *testing.T) {
	t.Setenv("GITLAB_WRITE_ENABLED", "true")
	assert.True(t, isWriteEnabled())

	t.Setenv("GITLAB_WRITE_ENABLED", "false")
	assert.False(t, isWriteEnabled())

	os.Unsetenv("GITLAB_WRITE_ENABLED")
	assert.False(t, isWriteEnabled())
}
