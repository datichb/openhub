package teamstate

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// Nothing changed one field of a member's profile: UpdateMember
// replaces the whole record. PatchMember reads the member on top of the remote,
// changes what fn changes and commits it, in one transaction.
func TestPatchMemberChangesOnlyItsFields(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)
	require.NoError(t, r.AddMember(ctx, Member{ID: "ben", DisplayName: "Ben", GitLabUsername: "benjamin.datiche1",
		MattermostUsername: "ben.m", Role: "lead", DefaultMode: "auto"}))
	require.NoError(t, r.AddMember(ctx, Member{ID: "ana", DisplayName: "Ana", GitLabUsername: "ana"}))
	commits := func() int {
		n, err := strconv.Atoi(gittest.Run(t, bare, "rev-list", "--count", "main"))
		require.NoError(t, err)
		return n
	}
	before := commits()

	require.NoError(t, r.PatchMember(ctx, "ben", func(m *Member) error {
		m.TrackerUsername = "benjamin.datiche"
		return nil
	}))

	// another clone sees it, with the rest of the record and the other member untouched
	other := NewRepo(bare, clone+"-other")
	require.NoError(t, other.Clone(ctx))
	got, err := other.GetMember(ctx, "ben")
	require.NoError(t, err)
	assert.Equal(t, Member{ID: "ben", DisplayName: "Ben", GitLabUsername: "benjamin.datiche1", MattermostUsername: "ben.m",
		TrackerUsername: "benjamin.datiche", Role: "lead", DefaultMode: "auto"}, *got)
	ana, err := other.GetMember(ctx, "ana")
	require.NoError(t, err)
	assert.Equal(t, "ana", ana.GitLabUsername)

	assert.Equal(t, before+1, commits(), "one commit")
	assert.Equal(t, "members: update ben", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
	assert.Equal(t, "members.toml", strings.TrimSpace(gittest.Run(t, bare, "show", "--name-only", "--format=", "main")))

	t.Run("nothing changed, nothing committed", func(t *testing.T) {
		head := gittest.Run(t, bare, "rev-parse", "main")
		require.NoError(t, r.PatchMember(ctx, "ben", func(*Member) error { return nil }))
		assert.Equal(t, head, gittest.Run(t, bare, "rev-parse", "main"))
	})
	t.Run("an unknown member", func(t *testing.T) {
		err := r.PatchMember(ctx, "ghost", func(*Member) error { return nil })
		assert.ErrorIs(t, err, ErrMemberNotFound)
	})
	t.Run("an error of fn commits nothing", func(t *testing.T) {
		head := gittest.Run(t, bare, "rev-parse", "main")
		boom := errors.New("boom")
		err := r.PatchMember(ctx, "ben", func(m *Member) error { m.Role = "dev"; return boom })
		assert.ErrorIs(t, err, boom)
		assert.Equal(t, head, gittest.Run(t, bare, "rev-parse", "main"))
		got, err := r.GetMember(ctx, "ben")
		require.NoError(t, err)
		assert.Equal(t, "lead", got.Role, "the file is as it was")
	})
}

// The GitLab handle is unique in a team: changing a profile to the handle of a colleague is
// refused, an existing duplicate does not block the edit of another field.
func TestPatchMember_HandleStaysUnique(t *testing.T) {
	t.Parallel()
	r, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, r.AddMember(ctx, Member{ID: "ben", DisplayName: "Ben", GitLabUsername: "ben.g", Role: "lead"}))
	require.NoError(t, r.AddMember(ctx, Member{ID: "eve", DisplayName: "Eve", GitLabUsername: "eve.g", Role: "dev"}))

	err := r.PatchMember(ctx, "eve", func(m *Member) error { m.GitLabUsername = "@Ben.G"; return nil })
	var taken *HandleTakenError
	require.ErrorAs(t, err, &taken)
	assert.Equal(t, "ben", taken.Member.ID)
	got, _ := r.GetMember(ctx, "eve")
	assert.Equal(t, "eve.g", got.GitLabUsername, "nothing is written")

	err = r.UpdateMember(ctx, Member{ID: "eve", DisplayName: "Eve", GitLabUsername: "ben.g", Role: "dev"})
	require.ErrorIs(t, err, ErrHandleTaken)

	require.NoError(t, r.PatchMember(ctx, "eve", func(m *Member) error { m.GitLabUsername = "eve.new"; return nil }))
	require.NoError(t, r.PatchMember(ctx, "eve", func(m *Member) error { m.DisplayName = "Eve E."; return nil }))
	require.NoError(t, r.PatchMember(ctx, "eve", func(m *Member) error { m.GitLabUsername = ""; return nil }), "clearing is free")
}
