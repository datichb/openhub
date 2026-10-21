package teamstate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) *Repo {
	t.Helper()
	return NewRepo("", t.TempDir())
}

func writeMembersToml(t *testing.T, repo *Repo, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(repo.membersFilePath(), []byte(content), 0o644))
}

const twoMembers = `
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
default_mode = "auto"
`

func TestMemberLookups(t *testing.T) {
	t.Parallel()
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, twoMembers)

	members, err := repo.ListMembers()
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, m := range members {
		ids[m.ID] = true
	}
	assert.Equal(t, map[string]bool{"benjamin": true, "alice": true}, ids, "every member comes with its id")

	m, err := repo.GetMember("benjamin")
	require.NoError(t, err)
	assert.Equal(t, Member{
		ID: "benjamin", DisplayName: "Benjamin", GitLabUsername: "bdatiche", MattermostUsername: "benjamin.datiche",
		Role: "lead", DefaultMode: "semi-auto",
	}, *m, "GetMember reads every field")
	assert.True(t, repo.HasMember("alice"), "HasMember")
	assert.False(t, repo.HasMember("bob"), "HasMember of an unknown member")

	m, err = repo.FindMemberByGitLab("bdatiche")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", m.ID, "by GitLab username")
	m, err = repo.FindMemberByGitLab("BDatiche")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", m.ID, "the GitLab username ignores the case")
	m, err = repo.FindMemberByMattermost("alice.dev")
	require.NoError(t, err)
	assert.Equal(t, "alice", m.ID, "by Mattermost username")
}

func TestMemberLookupErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		toml    string // "" : no members.toml
		call    func(r *Repo) error
		wantErr error
		wantMsg string
	}{
		{"unknown member", twoMembers, func(r *Repo) error { _, err := r.GetMember("unknown"); return err }, ErrMemberNotFound, ""},
		{"unknown GitLab username", twoMembers, func(r *Repo) error { _, err := r.FindMemberByGitLab("unknown"); return err }, ErrMemberNotFound, ""},
		{"unknown Mattermost username", twoMembers, func(r *Repo) error { _, err := r.FindMemberByMattermost("nobody"); return err }, ErrMemberNotFound, ""},
		{"invalid TOML", `this is not valid toml {{{{`, func(r *Repo) error { _, err := r.ListMembers(); return err }, nil, "parsing members.toml"},
		{"no members.toml", "", func(r *Repo) error { _, err := r.ListMembers(); return err }, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupTestRepo(t)
			if tt.toml != "" {
				writeMembersToml(t, repo, tt.toml)
			}
			err := tt.call(repo)
			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			if tt.wantMsg != "" {
				assert.Contains(t, err.Error(), tt.wantMsg)
			}
		})
	}

	t.Run("HasMember is false without a member", func(t *testing.T) {
		repo := setupTestRepo(t)
		assert.False(t, repo.HasMember("anyone"), "no members.toml")
		writeMembersToml(t, repo, "[members]\n")
		assert.False(t, repo.HasMember("anyone"), "an empty members table")
	})
}

func TestMembersCRUD(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	charlie := Member{ID: "charlie", DisplayName: "Charlie", GitLabUsername: "charlie", MattermostUsername: "charlie.dev", Role: "dev", DefaultMode: "manual"}
	require.NoError(t, repo.AddMember(ctx, charlie), "AddMember creates the members.toml")
	got, err := repo.GetMember("charlie")
	require.NoError(t, err)
	assert.Equal(t, charlie, *got, "the member is written")

	assert.ErrorIs(t, repo.AddMember(ctx, Member{ID: "charlie", DisplayName: "Duplicate"}), ErrMemberExists, "an id is added once")

	require.NoError(t, repo.AddMember(ctx, Member{ID: "alice", DisplayName: "Alice", GitLabUsername: "alice"}))
	updated := Member{ID: "alice", DisplayName: "Alice Updated", GitLabUsername: "alice-new", MattermostUsername: "alice.new", Role: "lead", DefaultMode: "auto"}
	require.NoError(t, repo.UpdateMember(ctx, updated))
	got, err = repo.GetMember("alice")
	require.NoError(t, err)
	assert.Equal(t, updated, *got, "UpdateMember replaces the member")
	assert.ErrorIs(t, repo.UpdateMember(ctx, Member{ID: "bob", DisplayName: "Bob"}), ErrMemberNotFound, "updating an unknown member")

	require.NoError(t, repo.RemoveMember(ctx, "charlie"))
	_, err = repo.GetMember("charlie")
	assert.ErrorIs(t, err, ErrMemberNotFound, "the removed member is gone")
	_, err = repo.GetMember("alice")
	assert.NoError(t, err, "the others stay")
	assert.ErrorIs(t, repo.RemoveMember(ctx, "unknown"), ErrMemberNotFound, "removing an unknown member")
}
