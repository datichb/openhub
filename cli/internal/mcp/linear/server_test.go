package linear

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUpdateIssue_EmptyGuard(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "test-key")

	params, err := json.Marshal(map[string]string{"issue_id": "X"})
	require.NoError(t, err)

	result, err := handleUpdateIssue(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "No fields to update")
}

func TestHandleUpdateIssue_PartialFields(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "test-key")

	params, err := json.Marshal(map[string]string{
		"issue_id": "X",
		"state_id": "uuid-state",
	})
	require.NoError(t, err)

	// The guard should NOT fire; the call reaches linearQuery which fails on HTTP.
	result, callErr := handleUpdateIssue(context.Background(), params)
	assert.Nil(t, result, "result should be nil (not a domain error)")
	assert.Error(t, callErr, "should get an HTTP-level error from linearQuery")
}

func TestHandleListIssues_DefaultFirst(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "test-key")

	params, err := json.Marshal(map[string]string{})
	require.NoError(t, err)

	// With an empty body the handler defaults First=50, then calls linearQuery
	// which fails because api.linear.app is unreachable in tests.
	result, callErr := handleListIssues(context.Background(), params)
	assert.Nil(t, result, "result should be nil (no domain error)")
	assert.Error(t, callErr, "should get an HTTP-level error from linearQuery")
}
