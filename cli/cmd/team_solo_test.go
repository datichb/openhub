package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// setupSoloTestApp isolates OH_HOME and installs an App with one project
// and, optionally, existing teams.
func setupSoloTestApp(t *testing.T, teams ...config.TeamConfig) (*app.App, *bytes.Buffer) {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "t@t.com", "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "t@t.com"} {
		t.Setenv(k, v)
	}
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(""), 0o644))
	io, stdout, _ := testIO()
	a := &app.App{
		Config:   &config.Config{Teams: teams},
		Projects: &mockProjectStore{projects: []domain.Project{{ID: "web", Name: "web-app", Path: t.TempDir()}}},
		IO:       io,
	}
	application = a
	t.Cleanup(func() { application = nil })
	return a, stdout
}

func runTeamSub(t *testing.T, sub *cobra.Command, out *bytes.Buffer, args ...string) error {
	t.Helper()
	sub.SetOut(out)
	sub.SetContext(t.Context())
	sub.SetArgs(args)
	// Flags keep their values between runs of the same command.
	sub.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	if err := sub.ParseFlags(args); err != nil {
		return err
	}
	return sub.RunE(sub, sub.Flags().Args())
}

func findTeamSub(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, c := range teamCmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("team %s not registered", name)
	return nil
}

func TestTeamInitSolo(t *testing.T) {
	existing := config.TeamConfig{ID: "acme", Enabled: true, StateRepo: "git@x:acme/state.git", StatePath: "/tmp/acme", MemberID: "alice"}
	a, out := setupSoloTestApp(t, existing)

	require.NoError(t, runTeamSub(t, teamInitCmd, out, "--solo", "--project", "web-app"))
	assert.Contains(t, out.String(), "solo")

	require.Len(t, a.Config.Teams, 2, "existing teams are kept")
	assert.Equal(t, existing, a.Config.Teams[0])
	solo := a.Config.Teams[1]
	assert.Equal(t, "solo", solo.ID)
	assert.True(t, solo.Solo && solo.Enabled)
	assert.Equal(t, "alice", solo.MemberID, "member id reused from the other team")
	assert.Equal(t, filepath.Join(config.HubDir(), "teams", "solo"), solo.StatePath)
	assert.Equal(t, "acme", a.Config.ActiveTeam().ID, "a solo team is never the active team")

	// hub.toml round trip.
	config.Reset()
	saved, err := config.Load()
	require.NoError(t, err)
	require.Len(t, saved.Teams, 2)
	assert.True(t, saved.Teams[1].Solo)
	assert.Empty(t, saved.Teams[1].StateRepo)

	p, err := a.Projects.Get(t.Context(), "web")
	require.NoError(t, err)
	require.NotNil(t, p.TeamID)
	assert.Equal(t, "solo", *p.TeamID)
	tc := config.ResolveTeamForProject(a.Config, p)
	assert.False(t, tc.Enabled, "team features (MCP team, claims, board) stay off in solo")
	assert.True(t, tc.Solo)
	assert.Equal(t, solo.StatePath, tc.StatePath)

	repo := teamstate.NewRepo("", solo.StatePath)
	assert.True(t, repo.IsLocalOnly(t.Context()))
	_, err = os.Stat(filepath.Join(solo.StatePath, "projects", "web", "workflows", "published", ".gitkeep"))
	require.NoError(t, err)
	require.NoError(t, repo.CheckPublish("alice"))

	// The project layer of the solo space is loaded for the project.
	layers, err := resolveWorkflowTeamLayers(t.Context(), a, "web")
	require.NoError(t, err)
	require.NotNil(t, layers)
	assert.Equal(t, "web", layers.Project)
	assert.Equal(t, solo.StatePath, layers.Repo.Path())

	// Same id twice, or a project already attached, are refused.
	err = runTeamSub(t, teamInitCmd, out, "--solo")
	require.Error(t, err)
	err = runTeamSub(t, teamInitCmd, out, "--solo", "--id", "other", "--project", "web")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "solo")
}

// Acceptance criterion 4: a project without a team creates and publishes a
// workflow in its solo space, then `team promote` shares it without loss.
func TestTeamPromote(t *testing.T) {
	a, out := setupSoloTestApp(t)
	require.NoError(t, runTeamSub(t, teamInitCmd, out, "--solo", "--member-id", "bob", "--project", "web"))
	solo := a.Config.Teams[0]
	repo := teamstate.NewRepo("", solo.StatePath)

	// Publish a project workflow locally.
	rel, _ := teamstate.PublishedRel(teamstate.ProjectScope("web"), "quick")
	writeTestFile(t, filepath.Join(repo.Path(), rel), "apiVersion: oh/v1\nkind: Workflow\nid: quick\nrisk: read\n")
	_, files, err := repo.SealWorkflowLocal(teamstate.ProjectScope("web"), "quick", teamstate.LockMeta{Version: 1, PublishedBy: "bob", PublishedAt: time.Now()})
	require.NoError(t, err)
	require.NoError(t, repo.CommitAndPush(t.Context(), "workflow: publish quick v1", files...))

	promote := findTeamSub(t, "promote")
	// Hooks run for every parent (EnableTraverseRunHooks): teamPreRunE must
	// let promote through although no active team exists.
	require.NoError(t, teamPreRunE(promote, nil))
	require.Error(t, runTeamSub(t, promote, out, "--remote", ""))

	bare := t.TempDir()
	testGitCmd(t, bare, "init", "--bare")
	out.Reset()
	require.NoError(t, runTeamSub(t, promote, out, "--remote", bare))
	assert.Contains(t, out.String(), "oh team init")
	assert.Contains(t, out.String(), bare)

	team := a.Config.Teams[0]
	assert.False(t, team.Solo)
	assert.Equal(t, bare, team.StateRepo)
	assert.Equal(t, solo.ID, team.ID)
	assert.Equal(t, solo.StatePath, team.StatePath)
	assert.Equal(t, "solo", a.Config.ActiveTeam().ID, "the promoted team is a regular team")

	// Another member clones the shared team-state: the workflow is there and valid.
	other := filepath.Join(t.TempDir(), "other")
	testGitCmd(t, t.TempDir(), "clone", bare, other)
	cat := workflow.NewMemCatalog()
	require.Empty(t, teamstate.NewRepo(bare, other).LoadWorkflowLayers(cat, "web"))
	_, ok := cat.Lookup(workflow.Ref{Layer: workflow.LayerProject, ID: "quick"})
	assert.True(t, ok)

	// Nothing left to promote.
	err = runTeamSub(t, promote, out, "--remote", bare)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "solo"), err.Error())
}
