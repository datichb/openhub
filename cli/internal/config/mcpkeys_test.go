package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory keychain (the real keychain is never used in tests).
type memStore struct {
	m       map[string]string
	getErr  error
	setErr  error
	setKeys []string
}

func newMemStore(kv map[string]string) *memStore { return &memStore{m: kv} }

func (s *memStore) Get(_ context.Context, k string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	return s.m[k], nil
}

func (s *memStore) Set(_ context.Context, k, v string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.m[k] = v
	s.setKeys = append(s.setKeys, k)
	return nil
}

func TestMCPTokenKeyNames(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "openhub.mcp.gitlab.token", MCPTokenKey("gitlab"))
	assert.Equal(t, "openhub.mcp.jira.token", MCPTokenKey("jira"))
	assert.Equal(t, "openhub.mcp.linear.token", MCPTokenKey("linear"))
	assert.Equal(t, "openhub.mcp.gitlab.token.p1", MCPProjectTokenKey("gitlab", "p1"))
	assert.Equal(t, "gitlab-token", LegacyMCPTokenKey("gitlab"))
	assert.Equal(t, "gitlab-token-p1", LegacyMCPProjectTokenKey("gitlab", "p1"))
}

func TestMCPTokenKeyCandidates(t *testing.T) {
	t.Parallel()
	hub := []string{"openhub.mcp.gitlab.token", "gitlab-token"}
	assert.Equal(t, hub, MCPTokenKeyCandidates("gitlab", ""))
	assert.Equal(t, hub, MCPTokenKeyCandidates("gitlab", "gitlab-token"))
	assert.Equal(t, hub, MCPTokenKeyCandidates("gitlab", "openhub.mcp.gitlab.token"))
	project := []string{"openhub.mcp.gitlab.token.p1", "gitlab-token-p1"}
	assert.Equal(t, project, MCPTokenKeyCandidates("gitlab", "gitlab-token-p1"))
	assert.Equal(t, project, MCPTokenKeyCandidates("gitlab", "openhub.mcp.gitlab.token.p1"))
	assert.Equal(t, []string{"my-key"}, MCPTokenKeyCandidates("gitlab", "my-key"), "a chosen name is read as is")
	assert.Equal(t, []string{"jira-token-p1"}, MCPTokenKeyCandidates("gitlab", "jira-token-p1"), "another service's key is not rewritten")
}

// The setup writes under the current name and the readers find it there.
func TestStoreMCPTokenThenRead(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newMemStore(map[string]string{})

	key, err := StoreMCPToken(ctx, store, "gitlab", "", "s3cret")
	require.NoError(t, err)
	assert.Equal(t, "openhub.mcp.gitlab.token", key)
	assert.Equal(t, map[string]string{"openhub.mcp.gitlab.token": "s3cret"}, store.m, "no legacy name is written")

	got, err := ReadMCPToken(ctx, store, "gitlab", key)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", got)

	pkey, err := StoreMCPToken(ctx, store, "gitlab", "p1", "proj")
	require.NoError(t, err)
	assert.Equal(t, "openhub.mcp.gitlab.token.p1", pkey)
	got, _ = ReadMCPToken(ctx, store, "gitlab", pkey)
	assert.Equal(t, "proj", got)

	_, err = StoreMCPToken(ctx, nil, "gitlab", "", "x")
	assert.Error(t, err)
	store.setErr = errors.New("locked")
	_, err = StoreMCPToken(ctx, store, "gitlab", "", "x")
	assert.Error(t, err)
}

// A key found only under the legacy name is read, copied under the current
// name, and the legacy one is kept.
func TestReadMCPTokenFallsBackToLegacyAndCopies(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newMemStore(map[string]string{"gitlab-token": "old"})

	got, err := ReadMCPToken(ctx, store, "gitlab", "openhub.mcp.gitlab.token")
	require.NoError(t, err)
	assert.Equal(t, "old", got)
	assert.Equal(t, "old", store.m["openhub.mcp.gitlab.token"], "copied under the current name")
	assert.Equal(t, "old", store.m["gitlab-token"], "the legacy key is kept")

	store.setKeys = nil
	got, _ = ReadMCPToken(ctx, store, "gitlab", "openhub.mcp.gitlab.token")
	assert.Equal(t, "old", got)
	assert.Empty(t, store.setKeys, "nothing is copied once the current key is set")
}

func TestReadMCPTokenPrefersTheCurrentName(t *testing.T) {
	t.Parallel()
	store := newMemStore(map[string]string{"openhub.mcp.gitlab.token": "new", "gitlab-token": "old"})
	got, _ := ReadMCPToken(t.Context(), store, "gitlab", "gitlab-token")
	assert.Equal(t, "new", got)
	assert.Empty(t, store.setKeys)
}

func TestReadMCPTokenProjectKeyFallback(t *testing.T) {
	t.Parallel()
	store := newMemStore(map[string]string{"gitlab-token-p1": "old", "gitlab-token": "hub"})
	got, _ := ReadMCPToken(t.Context(), store, "gitlab", "openhub.mcp.gitlab.token.p1")
	assert.Equal(t, "old", got)
	assert.Equal(t, "old", store.m["openhub.mcp.gitlab.token.p1"])
	assert.NotContains(t, store.m, "openhub.mcp.gitlab.token", "the hub token is not touched")
}

func TestReadMCPTokenAbsentAndErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newMemStore(map[string]string{})
	got, err := ReadMCPToken(ctx, store, "gitlab", "openhub.mcp.gitlab.token")
	assert.NoError(t, err, "an absent key is not an error")
	assert.Empty(t, got)

	got, err = ReadMCPToken(ctx, nil, "gitlab", "")
	assert.NoError(t, err)
	assert.Empty(t, got)

	store.getErr = errors.New("keychain locked")
	_, err = ReadMCPToken(ctx, store, "gitlab", "")
	assert.Error(t, err)

	// A failing copy does not hide the value.
	store = newMemStore(map[string]string{"gitlab-token": "old"})
	store.setErr = errors.New("read-only")
	got, err = ReadMCPToken(ctx, store, "gitlab", "")
	require.NoError(t, err)
	assert.Equal(t, "old", got)
}
