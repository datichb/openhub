package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// multiTeam is a hub of two teams, A (alice) and B (bob), a project in each and
// a project without team. When git is true the two team-states are real clones.
type multiTeam struct {
	a            *app.App
	repoA, repoB *teamstate.Repo
	projA, projB domain.Project
	stdout       *bytes.Buffer
}

func newMultiTeam(t *testing.T, git bool) *multiTeam {
	t.Helper()
	configtest.Home(t)
	io, stdout, _ := testIO()
	m := &multiTeam{stdout: stdout}
	teams := []config.TeamConfig{
		{ID: "a", Enabled: true, StateRepo: "git@x:org/a.git", StatePath: filepath.Join(t.TempDir(), "a"), MemberID: "alice"},
		{ID: "b", Enabled: true, StateRepo: "git@x:org/b.git", StatePath: filepath.Join(t.TempDir(), "b"), MemberID: "bob"},
	}
	if git {
		for i := range teams {
			bare, clone := claimRepos(t)
			teams[i].StateRepo, teams[i].StatePath = bare, clone
		}
		m.repoA = teamstate.NewRepo(teams[0].StateRepo, teams[0].StatePath)
		m.repoB = teamstate.NewRepo(teams[1].StateRepo, teams[1].StatePath)
	}
	cfg := &config.Config{Teams: teams}
	require.NoError(t, config.Save(cfg))
	m.projA = domain.Project{ID: "pa", Name: "proj-a", Path: filepath.Join(t.TempDir(), "pa"), TeamID: strPtr("a"), Status: domain.ProjectStatusActive}
	m.projB = domain.Project{ID: "pb", Name: "proj-b", Path: filepath.Join(t.TempDir(), "pb"), TeamID: strPtr("b"), Status: domain.ProjectStatusActive}
	loner := domain.Project{ID: "pn", Name: "proj-none", Path: filepath.Join(t.TempDir(), "pn"), Status: domain.ProjectStatusActive}
	for _, p := range []*domain.Project{&m.projA, &m.projB, &loner} {
		require.NoError(t, os.MkdirAll(p.Path, 0o755))
		real, err := filepath.EvalSymlinks(p.Path) // /var → /private/var on macOS: the folder the process is in
		require.NoError(t, err)
		p.Path = real
	}
	m.a = &app.App{Config: cfg, Projects: &mockProjectStore{projects: []domain.Project{m.projA, m.projB, loner}}, IO: io}
	application = m.a
	t.Cleanup(func() {
		application = nil
		teamRepo = nil
		pickedTeam.mu.Lock()
		pickedTeam.id = ""
		pickedTeam.mu.Unlock()
	})
	return m
}

// cwd moves the working directory into dir for the test.
func cwd(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestResolveTeam_Order(t *testing.T) {
	m := newMultiTeam(t, false)
	bg := context.Background()
	elsewhere := t.TempDir()
	cwd(t, elsewhere)

	// Nothing points to a team: the first enabled one.
	got, err := ResolveTeam(bg, m.a)
	require.NoError(t, err)
	assert.Equal(t, "a", got.TeamID)
	assert.Equal(t, config.TeamFromFirst, got.Source)

	// The default team (default = true) wins over the order of hub.toml.
	require.NoError(t, setDefaultTeam(m.a, "b"))
	got, _ = ResolveTeam(bg, m.a)
	assert.Equal(t, "b", got.TeamID)
	assert.Equal(t, "bob", got.MemberID, "the member is the one of the team")
	assert.Equal(t, config.TeamFromDefault, got.Source)

	// The project of the current folder beats the default team…
	cwd(t, m.projA.Path)
	got, _ = ResolveTeam(bg, m.a)
	assert.Equal(t, "a", got.TeamID)
	assert.Equal(t, "alice", got.MemberID)
	assert.Equal(t, config.TeamFromProject, got.Source)

	// …-p beats the current folder…
	got, _ = ResolveTeam(withTeamAsk(bg, teamAsk{Project: "proj-b"}), m.a)
	assert.Equal(t, "b", got.TeamID)

	// …the team chosen in the TUI beats the project…
	pickTeam(m.a, "b")
	got, _ = ResolveTeam(bg, m.a)
	assert.Equal(t, "b", got.TeamID)
	assert.Equal(t, config.TeamFromTUI, got.Source)

	// …and --team beats everything.
	got, err = ResolveTeam(withTeamAsk(bg, teamAsk{Team: "a", Project: "proj-a"}), m.a)
	require.NoError(t, err)
	assert.Equal(t, "a", got.TeamID)
	assert.Equal(t, config.TeamFromFlag, got.Source)
}

func TestResolveTeam_FlagErrors(t *testing.T) {
	m := newMultiTeam(t, false)
	bg := context.Background()
	cwd(t, t.TempDir())

	_, err := ResolveTeam(withTeamAsk(bg, teamAsk{Team: "zzz"}), m.a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zzz")

	// A project of B cannot be worked on in the team-state of A.
	_, err = ResolveTeam(withTeamAsk(bg, teamAsk{Team: "a", Project: "proj-b"}), m.a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proj-b")
}

func TestResolveTeam_ArchivedNeverByDefault(t *testing.T) {
	m := newMultiTeam(t, false)
	bg := context.Background()
	cwd(t, t.TempDir())
	require.NoError(t, setDefaultTeam(m.a, "a"))

	require.NoError(t, runTeamsArchive(&cobra.Command{}, []string{"a"}))
	got, err := ResolveTeam(bg, m.a)
	require.NoError(t, err)
	assert.Equal(t, "b", got.TeamID, "the archived team stops being the default one")
	assert.Contains(t, m.stdout.String(), "b", "the new default team is said")
	assert.Nil(t, defaultMarked(m.a.Config), "archiving removes the mark")

	// --team names it on purpose: the command says it is archived.
	got, err = ResolveTeam(withTeamAsk(bg, teamAsk{Team: "a"}), m.a)
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.True(t, got.Archived)
	require.Error(t, got.notEnabledError())
	assert.Contains(t, got.notEnabledError().Error(), "oh teams restore a")
	_, err = ensureTeamRepoFor(got)
	require.Error(t, err)

	// Every team archived: no team at all, never an archived one.
	require.NoError(t, runTeamsArchive(&cobra.Command{}, []string{"b"}))
	got, err = ResolveTeam(bg, m.a)
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Empty(t, got.TeamID)
}

func defaultMarked(cfg *config.Config) *config.TeamConfig {
	for i := range cfg.Teams {
		if cfg.Teams[i].Default {
			return &cfg.Teams[i]
		}
	}
	return nil
}

// A project of B is worked on in the team-state of B, with the member of B,
// whatever the team shown in the TUI or the default team.
func TestTeamOfProject_OwnTeamWins(t *testing.T) {
	m := newMultiTeam(t, false)
	bg := context.Background()
	cwd(t, t.TempDir())
	pickTeam(m.a, "a")

	got, err := teamOfProject(bg, m.a, &m.projB)
	require.NoError(t, err)
	assert.Equal(t, "b", got.TeamID)
	assert.Equal(t, "bob", got.MemberID)

	tc := resolvedTeamConfig(m.a, &m.projB)
	assert.Equal(t, "bob", tc.MemberID)

	// A project without team has no team features of its own.
	loner := domain.Project{ID: "pn"}
	assert.False(t, resolvedTeamConfig(m.a, &loner).Enabled)
	assert.Nil(t, startTicketClaims(m.a, &loner))
	assert.Nil(t, ticketRoom(m.a, &loner))
}

// --team, and the project, steer the whole subcommand: the claim of a project
// of B goes to the team-state of B, with the member of B; --team A for a project
// of B is refused and writes nothing.
func TestTeamClaim_MultiTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (git)")
	}
	m := newMultiTeam(t, true)
	require.NoError(t, setDefaultTeam(m.a, "a")) // A is the team of the hub
	cwd(t, t.TempDir())
	ctx := context.Background()

	newTeamTree := func(args ...string) *cobra.Command {
		parent := &cobra.Command{Use: "team", PersistentPreRunE: teamPreRunE, SilenceUsage: true, SilenceErrors: true}
		addTeamFlag(parent)
		child := &cobra.Command{Use: "claim <ticket>", Args: cobra.ExactArgs(1), RunE: runClaim}
		child.Flags().StringP("project", "p", "", "")
		child.Flags().String("worktree", "", "")
		child.Flags().Bool("planned", false, "")
		parent.AddCommand(child)
		parent.SetArgs(append([]string{"claim"}, args...))
		parent.SetContext(ctx)
		return parent
	}

	require.Error(t, newTeamTree("--team", "a", "-p", "proj-b", "T-9").Execute())
	_, err := m.repoA.GetClaim(ctx, "proj-b", "T-9")
	require.ErrorIs(t, err, teamstate.ErrClaimNotFound, "nothing is written in the team-state of A")

	require.NoError(t, newTeamTree("-p", "proj-b", "T-1").Execute())
	c, err := m.repoB.GetClaim(ctx, "proj-b", "T-1")
	require.NoError(t, err, "the claim is in the team-state of B")
	assert.Equal(t, "bob", c.ClaimedBy, "by the member of B")
	_, err = m.repoA.GetClaim(ctx, "proj-b", "T-1")
	require.ErrorIs(t, err, teamstate.ErrClaimNotFound)

	// From the folder of a project, the team is the one of the project.
	cwd(t, m.projA.Path)
	require.NoError(t, newTeamTree("T-2").Execute())
	_, err = m.repoA.GetClaim(ctx, "pa", "T-2") // the project detected from the folder is under its ID
	require.NoError(t, err)

	// --team makes the command act on that team (a project of the folder of the
	// same team, or none).
	cwd(t, t.TempDir())
	require.NoError(t, newTeamTree("--team", "b", "-p", "proj-b", "T-3").Execute())
	_, err = m.repoB.GetClaim(ctx, "proj-b", "T-3")
	require.NoError(t, err)
}

// A session on a project of B claims in B with the member of B (it used to take
// the member of the first team).
func TestStartTicketClaims_MemberOfTheTeamOfTheProject(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (git)")
	}
	m := newMultiTeam(t, true)
	hook := startTicketClaims(m.a, &m.projB)
	require.NotNil(t, hook)
	hook(context.Background(), "proj-b", []string{"T-7"})
	c, err := m.repoB.GetClaim(context.Background(), "proj-b", "T-7")
	require.NoError(t, err)
	assert.Equal(t, "bob", c.ClaimedBy)

	store, member, ok := cardTeam(m.a, &m.projB)
	require.True(t, ok)
	assert.Equal(t, "bob", member)
	assert.Equal(t, m.repoB.Path(), store.(*teamstate.Repo).Path())
}

// The services follow the team of the command line too.
func TestTeamServiceFor_Selection(t *testing.T) {
	m := newMultiTeam(t, false)
	cwd(t, t.TempDir())
	bg := context.Background()
	assert.Equal(t, "a", teamServiceFor(bg, m.a).ResolveTeam(nil).TeamID)
	assert.Equal(t, "b", teamServiceFor(withTeamAsk(bg, teamAsk{Team: "b"}), m.a).ResolveTeam(nil).TeamID)
	assert.Equal(t, "b", teamServiceFor(withTeamAsk(bg, teamAsk{Project: "proj-b"}), m.a).ResolveTeam(nil).TeamID)
	cwd(t, m.projB.Path)
	assert.Equal(t, "b", teamServiceFor(bg, m.a).ResolveTeam(nil).TeamID, "the project of the folder")
	pickTeam(m.a, "a")
	assert.Equal(t, "a", teamService(m.a).ResolveTeam(nil).TeamID, "the team chosen in the TUI")
	assert.Equal(t, "b", teamService(m.a).ResolveTeam(&m.projB).TeamID, "a project keeps its own team")
}

func TestTeamsDefaultCommand(t *testing.T) {
	m := newMultiTeam(t, false)
	m.a.Config.Teams = append(m.a.Config.Teams,
		config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "me"},
		config.TeamConfig{ID: "old", Enabled: false, StateRepo: "git@x:org/old.git", MemberID: "me"})
	require.NoError(t, config.Save(m.a.Config))

	require.NoError(t, runTeamsDefault(&cobra.Command{}, nil))
	assert.Contains(t, m.stdout.String(), "a")

	require.NoError(t, runTeamsDefault(&cobra.Command{}, []string{"b"}))
	assert.Equal(t, "b", m.a.Config.DefaultTeam().ID)
	assert.True(t, m.a.Config.FindTeam("b").Default)
	assert.False(t, m.a.Config.FindTeam("a").Default, "a single default team")

	// Persisted in hub.toml: a fresh read has it.
	fresh, err := config.UpdateAndGet(func(*config.Config) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, "b", fresh.DefaultTeam().ID)

	require.Error(t, runTeamsDefault(&cobra.Command{}, []string{"zzz"}))
	err = runTeamsDefault(&cobra.Command{}, []string{"solo"})
	require.Error(t, err, "a solo space cannot be the default team")
	err = runTeamsDefault(&cobra.Command{}, []string{"old"})
	require.Error(t, err, "an archived team cannot be the default team")
	assert.Contains(t, err.Error(), "oh teams restore old")
	assert.Equal(t, "b", m.a.Config.DefaultTeam().ID, "a refusal changes nothing")

	require.NoError(t, runTeamsList(&cobra.Command{}, nil))
	assert.Contains(t, m.stdout.String(), "b ★", "the list marks the default team")
}
