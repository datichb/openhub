package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

// mockProjectStore implements domain.ProjectStore for testing.
type mockProjectStore struct {
	projects []domain.Project
}

func (m *mockProjectStore) List(_ context.Context, _ domain.ProjectStatus) ([]domain.Project, error) {
	return m.projects, nil
}
func (m *mockProjectStore) Get(_ context.Context, id string) (*domain.Project, error) {
	for _, p := range m.projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) GetByPath(_ context.Context, _ string) (*domain.Project, error) {
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) GetByName(_ context.Context, name string) (*domain.Project, error) {
	for _, p := range m.projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) Create(_ context.Context, p *domain.Project) error {
	m.projects = append(m.projects, *p)
	return nil
}
func (m *mockProjectStore) Update(_ context.Context, _ *domain.Project) error { return nil }
func (m *mockProjectStore) Delete(_ context.Context, _ string) error          { return nil }

// mockSessionStore implements domain.SessionStore for testing.
type mockSessionStore struct{}

func (m *mockSessionStore) List(_ context.Context, _ string) ([]domain.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) Get(_ context.Context, _ string) (*domain.Session, error) {
	return nil, domain.ErrNotFound
}
func (m *mockSessionStore) Create(_ context.Context, _ *domain.Session) error { return nil }
func (m *mockSessionStore) Update(_ context.Context, _ *domain.Session) error { return nil }

// mockSecretStore implements domain.SecretStore for testing.
type mockSecretStore struct {
	secrets map[string]string
}

func newMockSecretStore() *mockSecretStore {
	return &mockSecretStore{secrets: make(map[string]string)}
}

func (m *mockSecretStore) Get(_ context.Context, key string) (string, error) {
	return m.secrets[key], nil
}
func (m *mockSecretStore) Set(_ context.Context, key, value string) error {
	m.secrets[key] = value
	return nil
}
func (m *mockSecretStore) Delete(_ context.Context, key string) error {
	delete(m.secrets, key)
	return nil
}
func (m *mockSecretStore) List(_ context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func TestNew(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.NotNil(t, a.Config)
	assert.NotNil(t, a.IO)
	assert.NotNil(t, a.IO.In)
	assert.NotNil(t, a.IO.Out)
	assert.NotNil(t, a.IO.ErrOut)
}

func TestWithProjectStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)

	ps := &mockProjectStore{}
	result := a.WithProjectStore(ps)

	assert.Same(t, a, result, "WithProjectStore should return the same App for chaining")
	assert.Same(t, ps, a.Projects)
}

func TestWithSessionStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)

	ss := &mockSessionStore{}
	result := a.WithSessionStore(ss)

	assert.Same(t, a, result, "WithSessionStore should return the same App for chaining")
	assert.Same(t, ss, a.Sessions)
}

func TestWithSecretStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)

	sec := newMockSecretStore()
	result := a.WithSecretStore(sec)

	assert.Same(t, a, result, "WithSecretStore should return the same App for chaining")
	assert.Same(t, sec, a.Secrets)
}

func TestWithSecretStoreNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)

	// Setting Secrets to nil should not panic
	result := a.WithSecretStore(nil)
	assert.Same(t, a, result)
	assert.Nil(t, a.Secrets)
}

func TestNilSecretStoreAccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)
	a.WithSecretStore(nil)

	// Verify that the app works fine when Secrets is nil
	// (the pattern used throughout the codebase is: if a.Secrets != nil { ... })
	assert.Nil(t, a.Secrets)

	// Verify we can still use the app for non-secret operations
	ps := &mockProjectStore{}
	a.WithProjectStore(ps)
	assert.NotNil(t, a.Projects)
}

func TestWithIO(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.Reset()

	a, err := New()
	require.NoError(t, err)

	customIO := &IOStreams{}
	result := a.WithIO(customIO)

	assert.Same(t, a, result)
	assert.Same(t, customIO, a.IO)
}

func TestDefaultIOStreams(t *testing.T) {
	io := DefaultIOStreams()
	require.NotNil(t, io)
	assert.NotNil(t, io.In)
	assert.NotNil(t, io.Out)
	assert.NotNil(t, io.ErrOut)
}
