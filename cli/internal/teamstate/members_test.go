package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	repo := NewRepo("", dir)
	return repo
}

func writeMembersToml(t *testing.T, repo *Repo, content string) {
	t.Helper()
	err := os.WriteFile(repo.membersFilePath(), []byte(content), 0o644)
	require.NoError(t, err)
}

func TestListMembers(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
gitlab_username = "bdatiche"
mattermost_username = "benjamin.datiche"
role = "lead"
default_mode = "semi-auto"

[members.alice]
display_name = "Alice"
gitlab_username = "alice"
mattermost_username = "alice.dev"
role = "dev"
default_mode = "semi-auto"
`)

	members, err := repo.ListMembers()
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Members should have IDs set
	ids := map[string]bool{}
	for _, m := range members {
		ids[m.ID] = true
	}
	assert.True(t, ids["benjamin"])
	assert.True(t, ids["alice"])
}

func TestGetMember(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
gitlab_username = "bdatiche"
mattermost_username = "benjamin.datiche"
role = "lead"
default_mode = "semi-auto"
`)

	m, err := repo.GetMember("benjamin")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", m.ID)
	assert.Equal(t, "Benjamin", m.DisplayName)
	assert.Equal(t, "bdatiche", m.GitLabUsername)
	assert.Equal(t, "benjamin.datiche", m.MattermostUsername)
	assert.Equal(t, "lead", m.Role)
	assert.Equal(t, "semi-auto", m.DefaultMode)
}

func TestGetMemberNotFound(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
`)

	_, err := repo.GetMember("unknown")
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestFindMemberByGitLab(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
gitlab_username = "bdatiche"
mattermost_username = "benjamin.datiche"
role = "lead"
default_mode = "semi-auto"
`)

	m, err := repo.FindMemberByGitLab("bdatiche")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", m.ID)

	// Case-insensitive
	m, err = repo.FindMemberByGitLab("BDatiche")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", m.ID)

	// Not found
	_, err = repo.FindMemberByGitLab("unknown")
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestFindMemberByMattermost(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.alice]
display_name = "Alice"
gitlab_username = "alice"
mattermost_username = "alice.dev"
role = "dev"
default_mode = "auto"
`)

	m, err := repo.FindMemberByMattermost("alice.dev")
	require.NoError(t, err)
	assert.Equal(t, "alice", m.ID)

	_, err = repo.FindMemberByMattermost("nobody")
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestAddMember(t *testing.T) {
	repo := setupTestRepo(t)
	// No members.toml yet — should create one
	err := repo.AddMember(Member{
		ID:                 "charlie",
		DisplayName:        "Charlie",
		GitLabUsername:     "charlie",
		MattermostUsername: "charlie.dev",
		Role:               "dev",
		DefaultMode:        "manual",
	})
	require.NoError(t, err)

	// Verify it was written
	m, err := repo.GetMember("charlie")
	require.NoError(t, err)
	assert.Equal(t, "Charlie", m.DisplayName)
}

func TestAddMemberDuplicate(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
`)

	err := repo.AddMember(Member{
		ID:          "benjamin",
		DisplayName: "Benjamin Duplicate",
	})
	assert.ErrorIs(t, err, ErrMemberExists)
}

func TestRemoveMember(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
gitlab_username = "bdatiche"

[members.alice]
display_name = "Alice"
gitlab_username = "alice"
`)

	err := repo.RemoveMember("benjamin")
	require.NoError(t, err)

	_, err = repo.GetMember("benjamin")
	assert.ErrorIs(t, err, ErrMemberNotFound)

	// Alice still exists
	m, err := repo.GetMember("alice")
	require.NoError(t, err)
	assert.Equal(t, "Alice", m.DisplayName)
}

func TestRemoveMemberNotFound(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.benjamin]
display_name = "Benjamin"
`)

	err := repo.RemoveMember("unknown")
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestParseMembersInvalid(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `this is not valid toml {{{{`)

	_, err := repo.ListMembers()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing members.toml")
}

func TestListMembersFileNotFound(t *testing.T) {
	repo := setupTestRepo(t)
	// No members.toml — should return error (not panic)
	_, err := repo.ListMembers()
	assert.Error(t, err)
}

func TestMembersFilePath(t *testing.T) {
	repo := NewRepo("", "/some/path")
	assert.Equal(t, filepath.Join("/some/path", "members.toml"), repo.membersFilePath())
}

func TestUpdateMember(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `
[members.alice]
display_name = "Alice"
gitlab_username = "alice"
mattermost_username = "alice.dev"
role = "dev"
default_mode = "semi-auto"
`)

	updated := Member{
		ID:                 "alice",
		DisplayName:        "Alice Updated",
		GitLabUsername:     "alice-new",
		MattermostUsername: "alice.new",
		Role:               "lead",
		DefaultMode:        "auto",
	}
	err := repo.UpdateMember(updated)
	require.NoError(t, err)

	// Verify
	m, err := repo.GetMember("alice")
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", m.DisplayName)
	assert.Equal(t, "alice-new", m.GitLabUsername)
	assert.Equal(t, "lead", m.Role)
}

func TestUpdateMemberNotFound(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, `[members.alice]
display_name = "Alice"
role = "dev"
default_mode = "semi-auto"
`)

	err := repo.UpdateMember(Member{ID: "bob", DisplayName: "Bob"})
	assert.ErrorIs(t, err, ErrMemberNotFound)
}
