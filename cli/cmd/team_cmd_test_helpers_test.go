package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// mockProjectStore implements domain.ProjectStore for cmd tests.
type mockProjectStore struct {
	projects []domain.Project
}

func (m *mockProjectStore) List(_ context.Context, _ domain.ProjectStatus) ([]domain.Project, error) {
	return m.projects, nil
}
func (m *mockProjectStore) Get(_ context.Context, id string) (*domain.Project, error) {
	for i, p := range m.projects {
		if p.ID == id {
			return &m.projects[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) GetByPath(_ context.Context, path string) (*domain.Project, error) {
	for i, p := range m.projects {
		if p.Path == path {
			return &m.projects[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) GetByName(_ context.Context, name string) (*domain.Project, error) {
	for i, p := range m.projects {
		if p.Name == name {
			return &m.projects[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockProjectStore) Create(_ context.Context, p *domain.Project) error {
	m.projects = append(m.projects, *p)
	return nil
}
func (m *mockProjectStore) Update(_ context.Context, p *domain.Project) error {
	for i, existing := range m.projects {
		if existing.ID == p.ID || existing.Name == p.Name {
			m.projects[i] = *p
			return nil
		}
	}
	return nil
}
func (m *mockProjectStore) Delete(_ context.Context, _ string) error { return nil }

// testIO creates a captured IOStreams for tests.
func testIO() (*app.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return &app.IOStreams{
		In:     os.Stdin,
		Out:    stdout,
		ErrOut: stderr,
	}, stdout, stderr
}

// setupClaimTestApp creates an App with a real teamstate git repo for claim tests.
// Returns the app, the teamstate repo, and the stdout buffer.
var claimTemplates templates

// claimRepos is a bare team-state repository and a clone of it, on main, with
// the members.toml the team needs, pushed: copies of a pair built once.
func claimRepos(t *testing.T) (bare, clone string) {
	t.Helper()
	root := claimTemplates.get(t, "claims", func(root string) {
		bare, clone := gittest.BareClone(t)
		// members.toml is required for the team to be functional
		gittest.Commit(t, clone, "members.toml", "[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\n", "members")
		gittest.Run(t, clone, "push", "-q")
		require.NoError(t, os.Rename(bare, filepath.Join(root, "bare")))
		require.NoError(t, os.Rename(clone, filepath.Join(root, "clone")))
		repointOrigin(t, filepath.Join(root, "clone"), bare, filepath.Join(root, "bare"))
	})
	bare, clone = t.TempDir(), filepath.Join(t.TempDir(), "clone")
	require.NoError(t, os.Mkdir(clone, 0o755))
	copyTree(t, filepath.Join(root, "bare"), bare)
	copyTree(t, filepath.Join(root, "clone"), clone)
	repointOrigin(t, clone, filepath.Join(root, "bare"), bare)
	return bare, clone
}

// repointOrigin makes the clone's origin from (the bare repository it was
// made from) to (where it is now).
func repointOrigin(t *testing.T, clone, from, to string) {
	t.Helper()
	cfg := filepath.Join(clone, ".git", "config")
	data, err := os.ReadFile(cfg)
	require.NoError(t, err)
	require.Contains(t, string(data), from)
	require.NoError(t, os.WriteFile(cfg, []byte(strings.ReplaceAll(string(data), from, to)), 0o644))
}

func setupClaimTestApp(t *testing.T) (*app.App, *teamstate.Repo, *bytes.Buffer) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bare, clone := claimRepos(t)

	repo := teamstate.NewRepo(bare, clone)

	io, stdout, _ := testIO()
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{
				ID:        "test-team",
				Enabled:   true,
				StateRepo: bare,
				StatePath: clone,
				MemberID:  "testuser",
			},
		},
	}

	a := &app.App{
		Config:   cfg,
		Projects: &mockProjectStore{},
		IO:       io,
	}

	// Set the package-level application variable
	application = a
	teamRepo = repo
	t.Cleanup(func() { application = nil; teamRepo = nil })

	return a, repo, stdout
}

// setupTeamsTestApp creates a lightweight App for teams commands (no git required).
func setupTeamsTestApp(t *testing.T, teams []config.TeamConfig, projects []domain.Project) (*app.App, *bytes.Buffer) {
	t.Helper()

	configtest.Home(t) // config.Save() writes to a temp hub

	io, stdout, _ := testIO()
	cfg := &config.Config{
		Teams: teams,
	}

	a := &app.App{
		Config:   cfg,
		Projects: &mockProjectStore{projects: projects},
		IO:       io,
	}

	application = a
	t.Cleanup(func() { application = nil })
	return a, stdout
}
