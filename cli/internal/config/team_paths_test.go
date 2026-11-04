package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

// fakeClone makes path look like the clone of origin for what TeamStatePath
// looks at: a .git folder whose config names the origin.
func fakeClone(t *testing.T, path, origin string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(path, ".git"), 0o755))
	cfg := "[remote \"origin\"]\n\turl = " + origin + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(path, ".git", "config"), []byte(cfg), 0o644))
}

func TestTeamStatePath_IsTheFullPathOfTheRepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	base := filepath.Join(home, "team-states")

	assert.Equal(t, filepath.Join(base, "gitlab.com", "acme", "team-state"), config.TeamStatePath("git@gitlab.com:acme/team-state.git"))
	assert.Equal(t, filepath.Join(base, "gitlab.com", "beta", "team-state"), config.TeamStatePath("https://gitlab.com/beta/team-state.git"))
	assert.Equal(t, filepath.Join(base, "gitlab.com", "org", "sub", "ts"), config.TeamStatePath("git@gitlab.com:org/sub/ts.git"))
	assert.Equal(t, filepath.Join(base, "gitlab.example.com", "acme", "team-state"), config.TeamStatePath("https://gitlab.example.com/acme/team-state"),
		"the same path on another server is another folder")
	assert.Equal(t, config.TeamStatePath("git@gitlab.com:acme/team-state.git"), config.TeamStatePath("https://GitLab.com/acme/team-state/"),
		"two spellings of one repository are one folder")
}

func TestTeamStatePath_NeverEscapesTheTeamStatesFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	base := filepath.Join(home, "team-states") + string(filepath.Separator)
	for _, remote := range []string{
		"https://gitlab.com/../../etc/ts.git",
		"git@gitlab.com:acme/./../x.git",
		"https://gitlab.com/a%2F..%2Fb/ts.git",
		`https://gitlab.com/acme/a:b*c.git`,
		"git@host:..",
	} {
		got := config.TeamStatePath(remote)
		assert.True(t, len(got) > len(base) && got[:len(base)] == base, "%q → %q stays under %q", remote, got, base)
		assert.NotContains(t, got[len(base):], "..", "%q → %q", remote, got)
	}
}

func TestTeamStatePath_ALocalRemoteGetsAFolderPerPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	a := config.TeamStatePath(filepath.Join(home, "x", "team-state.git"))
	b := config.TeamStatePath(filepath.Join(home, "y", "team-state.git"))
	assert.NotEqual(t, a, b, "two local repositories with the same name do not share a folder")
	assert.Equal(t, filepath.Join(home, "team-states", "local"), filepath.Dir(a))
}

func TestTeamStatePath_ReusesAClonePlacedElsewhereBefore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	base := filepath.Join(home, "team-states")
	remote := "git@gitlab.com:acme/team-state.git"
	newPath := filepath.Join(base, "gitlab.com", "acme", "team-state")
	assert.Equal(t, newPath, config.TeamStatePath(remote), "nothing exists yet: the new convention")

	hostName := filepath.Join(base, "gitlab.com", "team-state") // 5.3.2
	nameOnly := filepath.Join(base, "team-state")               // before 5.3
	first := filepath.Join(home, "team-state")                  // the first default

	t.Run("a folder that is no clone is not used", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(hostName, 0o755))
		assert.Equal(t, newPath, config.TeamStatePath(remote))
	})
	t.Run("a clone of another repository is never used", func(t *testing.T) {
		fakeClone(t, hostName, "git@gitlab.com:beta/team-state.git")
		fakeClone(t, nameOnly, "git@gitlab.com:beta/team-state.git")
		fakeClone(t, first, "git@gitlab.com:beta/team-state.git")
		assert.Equal(t, newPath, config.TeamStatePath(remote))
		assert.Equal(t, hostName, config.TeamStatePath("git@gitlab.com:beta/team-state.git"), "beta, whose clone it is, finds it")
	})
	t.Run("the clone of the right repository is kept", func(t *testing.T) {
		require.NoError(t, os.RemoveAll(hostName))
		fakeClone(t, hostName, "https://gitlab.com/acme/team-state.git")
		assert.Equal(t, hostName, config.TeamStatePath(remote), "<host>/<name> of 5.3.2")
		require.NoError(t, os.RemoveAll(hostName))
		fakeClone(t, nameOnly, "git@gitlab.com:acme/team-state.git")
		assert.Equal(t, nameOnly, config.TeamStatePath(remote), "<name> of before 5.3")
		require.NoError(t, os.RemoveAll(nameOnly))
		fakeClone(t, first, "git@gitlab.com:acme/team-state.git")
		assert.Equal(t, first, config.TeamStatePath(remote), "~/.oh/team-state")
	})
	t.Run("the new place wins when it holds the clone", func(t *testing.T) {
		fakeClone(t, newPath, remote)
		assert.Equal(t, newPath, config.TeamStatePath(remote))
	})
}

func TestResolveStatePath(t *testing.T) {
	home := configtest.Home(t)
	hub := filepath.Join(home, ".oh")
	remote := "git@gitlab.com:acme/team-state.git"

	assert.Equal(t, config.TeamStatePath(remote), config.ResolveStatePath(remote, ""), "empty: derived from the repository")
	assert.Equal(t, config.TeamStatePath(remote), config.ResolveStatePath(remote, "   "), "blank is empty")
	assert.Equal(t, "", config.ResolveStatePath("", ""), "no repository, no folder")
	abs := filepath.Join(home, "clones", "acme")
	assert.Equal(t, abs, config.ResolveStatePath(remote, abs), "an absolute folder is kept")
	assert.Equal(t, filepath.Join(home, "clones", "acme"), config.ResolveStatePath(remote, "~/clones/acme"), "~ is the home directory")
	assert.Equal(t, home, config.ResolveStatePath(remote, "~"))
	assert.Equal(t, filepath.Join(hub, "mine"), config.ResolveStatePath(remote, "mine"),
		"a relative folder is under the hub folder, never under the working directory")
}

func TestLoad_GivesEveryTeamItsFolder(t *testing.T) {
	home := configtest.Home(t)
	writeHub(t, `
[[teams]]
id = "acme"
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"

[[teams]]
id = "beta"
enabled = true
state_repo = "git@gitlab.com:beta/team-state.git"
state_path = "~/clones/beta"
member_id = "bob"
`)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Teams, 2)
	assert.Equal(t, config.TeamStatePath("git@gitlab.com:acme/team-state.git"), cfg.Teams[0].StatePath, "empty state_path: resolved when hub.toml is read")
	assert.NotEmpty(t, cfg.Teams[0].StatePath)
	assert.Equal(t, filepath.Join(home, "clones", "beta"), cfg.Teams[1].StatePath, "an explicit one is kept, ~ expanded")
	assert.Equal(t, cfg.Teams[0].StatePath, cfg.ActiveTeam().StatePath)
	assert.Equal(t, cfg.Teams[0].StatePath, config.ResolveTeamConfig(cfg.ActiveTeam(), nil).StatePath)
}

func TestLoad_AnEmptyStatePathKeepsTheCloneItHad(t *testing.T) {
	home := configtest.Home(t)
	hub := filepath.Join(home, ".oh")
	former := filepath.Join(hub, "team-states", "gitlab.com", "team-state")
	fakeClone(t, former, "git@gitlab.com:acme/team-state.git")
	writeHub(t, `
[[teams]]
id = "acme"
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"
`)
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, former, cfg.Teams[0].StatePath, "the clone of 5.3.2 is neither cloned again nor moved")
}

func TestLoad_TheFolderIsWrittenAtTheNextWrite(t *testing.T) {
	configtest.Home(t)
	writeHub(t, `
[[teams]]
id = "acme"
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"
`)
	before, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	_, err = config.Load()
	require.NoError(t, err)
	after, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "reading writes nothing")

	require.NoError(t, config.Update(func(c *config.Config) error { c.Name = "Hub"; return nil }))
	written, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Contains(t, string(written), "state_path", "the next write puts the folder in hub.toml")
}

func TestTeamIDFor(t *testing.T) {
	acme := "git@gitlab.com:acme/team-state.git"
	beta := "git@gitlab.com:beta/team-state.git"
	cfg := &config.Config{}

	assert.Equal(t, "team-state", cfg.TeamIDFor(acme), "the first team keeps the name of its repository")
	cfg.Teams = []config.TeamConfig{{ID: "team-state", StateRepo: acme}}
	assert.Equal(t, "team-state", cfg.TeamIDFor("https://gitlab.com/acme/team-state"), "the team of the repository: its ID, whatever the spelling")
	assert.Equal(t, "beta-team-state", cfg.TeamIDFor(beta), "the same name for another repository: group-name")

	cfg.Teams = append(cfg.Teams, config.TeamConfig{ID: "beta-team-state", StateRepo: beta})
	assert.Equal(t, "gitlab-example-com-beta-team-state", cfg.TeamIDFor("https://gitlab.example.com/beta/team-state.git"),
		"the same group and name on another server: with the host")

	cfg.Teams = append(cfg.Teams, config.TeamConfig{ID: "gitlab-example-com-beta-team-state", StateRepo: "https://other.example.com/x/y.git"})
	assert.Equal(t, "gitlab-example-com-beta-team-state-2", cfg.TeamIDFor("https://gitlab.example.com/beta/team-state.git"),
		"nothing else free (IDs chosen by the members): numbered")

	custom := &config.Config{Teams: []config.TeamConfig{{ID: "acme", StateRepo: acme}}}
	assert.Equal(t, "acme", custom.TeamIDFor(acme), "an ID chosen by the member is the ID of the repository")
	assert.Equal(t, "team-state", custom.TeamIDFor(beta), "and the name stays free for another repository")

	assert.Equal(t, "my-team", (&config.Config{}).TeamIDFor("git@gitlab.com:acme/My_Team.git"), "an ID is a slug")
	for _, remote := range []string{acme, beta, "git@gitlab.com:org/sub/Team_State.git", "/srv/git/State.git"} {
		assert.NoError(t, config.TeamConfig{ID: cfg.TeamIDFor(remote), Enabled: true, StateRepo: remote, MemberID: "a"}.Validate(), remote)
	}
}

func TestUpsertTeam_TwoRepositoriesWithTheSameName(t *testing.T) {
	cfg := &config.Config{}
	acme := config.TeamConfig{Enabled: true, StateRepo: "git@gitlab.com:acme/team-state.git", StatePath: "/s/acme", MemberID: "alice"}
	beta := config.TeamConfig{Enabled: true, StateRepo: "git@gitlab.com:beta/team-state.git", StatePath: "/s/beta", MemberID: "bob"}

	idAcme := cfg.UpsertTeam(acme)
	idBeta := cfg.UpsertTeam(beta)
	require.Len(t, cfg.Teams, 2, "the second team does not replace the first")
	assert.NotEqual(t, idAcme, idBeta)
	assert.Equal(t, "team-state", idAcme)
	assert.Equal(t, "beta-team-state", idBeta)
	assert.Equal(t, "alice", cfg.FindTeam(idAcme).MemberID)
	assert.Equal(t, "bob", cfg.FindTeam(idBeta).MemberID)

	acme.MemberID = "alice2"
	assert.Equal(t, idAcme, cfg.UpsertTeam(acme), "joining again finds the team by its repository")
	require.Len(t, cfg.Teams, 2)
	assert.Equal(t, "alice2", cfg.FindTeam(idAcme).MemberID)
	assert.Equal(t, "bob", cfg.FindTeam(idBeta).MemberID, "the other is untouched")

	asked := config.TeamConfig{ID: idAcme, Enabled: true, StateRepo: "git@gitlab.com:gamma/other.git", MemberID: "carol"}
	idGamma := cfg.UpsertTeam(asked)
	require.Len(t, cfg.Teams, 3, "an ID taken by another repository is never overwritten")
	assert.NotEqual(t, idAcme, idGamma)
	assert.Equal(t, "alice2", cfg.FindTeam(idAcme).MemberID)
}

func TestUpsertTeam_KeepsTheIDTheMemberChose(t *testing.T) {
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "acme", Name: "ACME", Enabled: true, StateRepo: "git@gitlab.com:acme/team-state.git", StatePath: "/s/acme", MemberID: "alice", GitLabURL: "https://g.example"}}}
	id := cfg.UpsertTeam(config.TeamConfig{ID: "team-state", Enabled: true, StateRepo: "https://gitlab.com/acme/team-state.git", MemberID: "alice"})
	assert.Equal(t, "acme", id, "Join does not rename the team to the name of its repository")
	require.Len(t, cfg.Teams, 1)
	got := cfg.Teams[0]
	assert.Equal(t, "acme", got.ID)
	assert.Equal(t, "ACME", got.Name)
	assert.Equal(t, "/s/acme", got.StatePath, "its folder too, when the caller has none")
	assert.Equal(t, "https://g.example", got.GitLabURL)
}

func TestMigrate_TheTeamSectionKeepsItsClone(t *testing.T) {
	home := configtest.Home(t)
	hub := filepath.Join(home, ".oh")
	remote := "git@gitlab.com:acme/team-state.git"
	first := filepath.Join(hub, "team-state")
	fakeClone(t, first, remote)

	for name, statePath := range map[string]string{"empty": "", "the first default": first} {
		c := &config.Config{Team: config.TeamConfig{Enabled: true, StateRepo: remote, StatePath: statePath, MemberID: "alice"}}
		require.True(t, config.MigrateTeamToTeams(c), name)
		assert.Equal(t, first, c.Teams[0].StatePath, "%s: the clone is where it was, not at a path nothing is cloned in", name)
	}

	custom := &config.Config{Team: config.TeamConfig{Enabled: true, StateRepo: remote, StatePath: filepath.Join(home, "mine"), MemberID: "alice"}}
	require.True(t, config.MigrateTeamToTeams(custom))
	assert.Equal(t, filepath.Join(home, "mine"), custom.Teams[0].StatePath, "a chosen folder is kept")

	none := &config.Config{Team: config.TeamConfig{Enabled: true, StateRepo: "git@gitlab.com:acme/other.git", MemberID: "alice"}}
	require.True(t, config.MigrateTeamToTeams(none))
	assert.Equal(t, config.TeamStatePath("git@gitlab.com:acme/other.git"), none.Teams[0].StatePath, "no clone yet: the convention of the other teams")
}
