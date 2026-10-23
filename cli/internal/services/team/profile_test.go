package team

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func str(s string) *string { return &s }

// Bug 12: the tracker name of the member is set in the profile, in the
// team-state, and the rest of the profile is kept.
func TestUpdateProfileWritesTheMemberOfTheTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "ben", DisplayName: "Ben",
		GitLabUsername: "benjamin.datiche1", Role: "lead", DefaultMode: "auto"}))
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: bare, StatePath: clone, MemberID: "ben"}}}
	svc := New(cfg, nil, nil)

	got, err := svc.Profile("")
	require.NoError(t, err)
	assert.Empty(t, got.TrackerUsername, "not set yet")

	got, err = svc.UpdateProfile(t.Context(), "", ProfilePatch{TrackerUsername: str("  @benjamin.datiche ")})
	require.NoError(t, err)
	assert.Equal(t, "benjamin.datiche", got.TrackerUsername, "trimmed, without the @")
	assert.Equal(t, "benjamin.datiche1", got.GitLabUsername)
	assert.Equal(t, "lead", got.Role)
	assert.Equal(t, "auto", got.DefaultMode)
	assert.Equal(t, "members: update ben", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))

	got, err = svc.UpdateProfile(t.Context(), "", ProfilePatch{TrackerUsername: str("")})
	require.NoError(t, err)
	assert.Empty(t, got.TrackerUsername, "an empty value clears it")

	_, err = svc.UpdateProfile(t.Context(), "", ProfilePatch{DisplayName: str(" ")})
	assert.ErrorIs(t, err, ErrEmptyDisplayName)
}

func TestProfileErrors(t *testing.T) {
	_, err := New(&config.Config{}, nil, nil).Profile("")
	assert.ErrorIs(t, err, ErrTeamNotConfigured)

	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@x:a/core.git", StatePath: t.TempDir()}}}
	_, err = New(cfg, nil, nil).Profile("")
	assert.ErrorIs(t, err, ErrNoMemberID)

	cfg.Teams[0].MemberID = "ben"
	_, err = New(cfg, nil, nil).Profile("")
	assert.ErrorIs(t, err, ErrNotCloned)
}

func TestProfileOfAnotherTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "ben", DisplayName: "Ben"}))
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "hub", Enabled: true, StateRepo: "git@x:a/hub.git", StatePath: t.TempDir(), MemberID: "ben"},
		{ID: "core", Enabled: true, StateRepo: bare, StatePath: clone, MemberID: "ben"},
	}}
	svc := New(cfg, nil, nil)
	got, err := svc.UpdateProfile(t.Context(), "core", ProfilePatch{TrackerUsername: str("ben.t")})
	require.NoError(t, err)
	assert.Equal(t, "ben.t", got.TrackerUsername)
	_, err = svc.Profile("nope")
	assert.ErrorIs(t, err, ErrTeamNotConfigured)
}

func TestProfilePatchEmpty(t *testing.T) {
	assert.True(t, ProfilePatch{}.Empty())
	assert.False(t, ProfilePatch{TrackerUsername: str("")}.Empty())
	assert.Equal(t, "bob", CleanUsername(" @bob "))
}

func TestTrackerIdentity(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state")
	repo, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "ben", DisplayName: "Ben", GitLabUsername: "Ben.G"})
	require.NoError(t, err)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "ana", GitLabUsername: "ana", TrackerUsername: "Ana.T"}))
	cfgFile := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"https://gitlab.other.example\"\ntracker_project = \"a/b\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(cfgFile), 0o644))
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@gitlab.corp.example:a/core.git", StatePath: statePath, MemberID: "ben"}}}
	svc := New(cfg, nil, nil)

	ti, err := svc.TrackerIdentity(nil)
	require.NoError(t, err)
	assert.True(t, ti.CrossServer, "the tracker is not on the server of the team repository")
	assert.Equal(t, "https://gitlab.other.example", ti.TrackerURL)
	require.NotNil(t, ti.Member)
	assert.Equal(t, "ben", ti.Member.ID)
	assert.True(t, ti.HasName("ana.t"), "the tracker name of a member, whatever the case")
	assert.True(t, ti.HasName("ben.g"), "the GitLab name when there is no tracker name")
	assert.False(t, ti.HasName("ana"), "the GitLab name of a member who has a tracker name is not used")
	assert.False(t, ti.HasName("benjamin.datiche"))

	cfg.Teams[0].StateRepo = "git@gitlab.other.example:a/core.git"
	ti, err = svc.TrackerIdentity(nil)
	require.NoError(t, err)
	assert.False(t, ti.CrossServer, "same server")

	cfg.Teams[0].StateRepo, cfg.Teams[0].GitLabURL = "git@mon-alias:a/core.git", "https://gitlab.corp.example"
	ti, err = svc.TrackerIdentity(nil)
	require.NoError(t, err)
	assert.True(t, ti.CrossServer, "an SSH alias is read through the GitLab declared for it")

	cfg.Teams[0].StateRepo, cfg.Teams[0].GitLabURL = "/srv/team-state.git", ""
	ti, err = svc.TrackerIdentity(nil)
	require.NoError(t, err)
	assert.False(t, ti.CrossServer, "a repository whose server is not known is not said to be elsewhere")
}
