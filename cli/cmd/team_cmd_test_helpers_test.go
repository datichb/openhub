package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
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
func setupClaimTestApp(t *testing.T) (*app.App, *teamstate.Repo, *bytes.Buffer) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create bare + clone
	bare := t.TempDir()
	testGitCmd(t, bare, "init", "--bare")
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", bare, clone)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "clone failed: %s", string(out))
	testGitCmd(t, clone, "config", "user.email", "test@test.com")
	testGitCmd(t, clone, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(clone, "README.md"), []byte("init"), 0o644))
	testGitCmd(t, clone, "add", ".")
	testGitCmd(t, clone, "commit", "-m", "init")
	// Detect branch and push
	branchOut, _ := exec.Command("git", "-C", clone, "branch", "--show-current").Output()
	branch := "main"
	if b := string(branchOut); len(b) > 1 {
		branch = b[:len(b)-1]
	}
	testGitCmd(t, clone, "push", "-u", "origin", branch)

	// Write members.toml (required for team to be functional)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "members.toml"),
		[]byte("[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\n"), 0o644))
	testGitCmd(t, clone, "add", ".")
	testGitCmd(t, clone, "commit", "-m", "members")
	testGitCmd(t, clone, "push")

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

	// Override HOME so config.Save() writes to a temp dir (not real ~/.oh/)
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

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

// testGitCmd runs a git command in the given directory.
func testGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v in %s failed: %s", args, dir, string(out))
}

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
}
