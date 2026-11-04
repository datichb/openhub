package teamstate

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestCheckJoinRefusesATakenIDAndAHandleOfAnother(t *testing.T) {
	t.Parallel()
	members := []Member{
		{ID: "alice", DisplayName: "Alice", GitLabUsername: "alice.g"},
		{ID: "bob", DisplayName: "Bob"},
	}

	var idErr *MemberIDTakenError
	require.ErrorAs(t, CheckJoin(members, Member{ID: "alice", DisplayName: "Eve"}, JoinNew), &idErr)
	assert.ErrorIs(t, idErr, ErrMemberExists)
	assert.Equal(t, "Alice", idErr.Member.DisplayName, "the error tells whose card it is")

	var handleErr *HandleTakenError
	require.ErrorAs(t, CheckJoin(members, Member{ID: "eve", GitLabUsername: "@Alice.G"}, JoinNew), &handleErr)
	assert.ErrorIs(t, handleErr, ErrHandleTaken)
	assert.Equal(t, "alice", handleErr.Member.ID, "case and @ do not hide a handle")

	assert.NoError(t, CheckJoin(members, Member{ID: "eve", GitLabUsername: "eve.g"}, JoinNew))
	assert.NoError(t, CheckJoin(members, Member{ID: "eve"}, JoinNew), "the handle is optional")
	assert.NoError(t, CheckJoin(members, Member{ID: "alice", GitLabUsername: "alice.g"}, JoinOwn), "one's own card and handle")
	assert.NoError(t, CheckJoin(members, Member{ID: "alice", GitLabUsername: "alice.g"}, JoinClaim))
	assert.ErrorIs(t, CheckJoin(members, Member{ID: "bob", GitLabUsername: "alice.g"}, JoinClaim), ErrHandleTaken,
		"« C'est moi » does not give a handle that is another's")
}

// The card of a colleague is not replaced by the one of whoever types the same
// ID: nothing is written, nothing is committed.
func TestJoinMemberNeverOverwritesTheCardOfAColleague(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)
	colleague := Member{ID: "ben", DisplayName: "Benjamin", GitLabUsername: "ben.g", MattermostUsername: "ben.m",
		TrackerUsername: "ben.t", Role: "lead", DefaultMode: "auto"}
	require.NoError(t, r.AddMember(ctx, colleague))
	head := gittest.Run(t, bare, "rev-parse", "main")

	_, err := r.JoinMember(ctx, Member{ID: "ben", DisplayName: "Impostor", GitLabUsername: "other", Role: "dev"}, JoinNew)
	var taken *MemberIDTakenError
	require.ErrorAs(t, err, &taken)
	assert.Equal(t, "Benjamin", taken.Member.DisplayName)

	_, err = r.JoinMember(ctx, Member{ID: "eve", GitLabUsername: "ben.g"}, JoinNew)
	var handle *HandleTakenError
	require.ErrorAs(t, err, &handle)
	assert.Equal(t, "ben", handle.Member.ID)

	got, err := r.GetMember(ctx, "ben")
	require.NoError(t, err)
	colleague.ID = "ben"
	assert.Equal(t, colleague, *got, "the card is untouched")
	assert.False(t, r.HasMember(ctx, "eve"))
	assert.Equal(t, head, gittest.Run(t, bare, "rev-parse", "main"), "nothing committed, nothing pushed")
}

func TestJoinMemberCreatesWithTheDefaults(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)

	res, err := r.JoinMember(ctx, Member{ID: "eve", DisplayName: "Eve", GitLabUsername: "@eve.g"}, JoinNew)
	require.NoError(t, err)
	assert.True(t, res.Created)
	got, err := r.GetMember(ctx, "eve")
	require.NoError(t, err)
	assert.Equal(t, Member{ID: "eve", DisplayName: "Eve", GitLabUsername: "eve.g", Role: "dev", DefaultMode: "semi-auto"}, *got)
	assert.Equal(t, "members: add eve", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
}

// « C'est moi »: the card is attached, its empty fields are filled, what it
// holds stays.
func TestJoinMemberClaimFillsOnlyTheEmptyFields(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)
	require.NoError(t, r.AddMember(ctx, Member{ID: "ben", DisplayName: "Benjamin", Role: "lead", DefaultMode: "auto"}))
	count := func() int {
		n, err := strconv.Atoi(gittest.Run(t, bare, "rev-list", "--count", "main"))
		require.NoError(t, err)
		return n
	}

	res, err := r.JoinMember(ctx, Member{ID: "ben", DisplayName: "Someone else", GitLabUsername: "ben.g", Role: "dev"}, JoinClaim)
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.Equal(t, []string{"gitlab_username"}, res.Filled)
	assert.ElementsMatch(t, []string{"display_name", "role"}, res.Kept, "what the card holds is kept and reported")
	got, err := r.GetMember(ctx, "ben")
	require.NoError(t, err)
	assert.Equal(t, Member{ID: "ben", DisplayName: "Benjamin", GitLabUsername: "ben.g", Role: "lead", DefaultMode: "auto"}, *got)

	before := count()
	_, err = r.JoinMember(ctx, Member{ID: "ben", GitLabUsername: "ben.g"}, JoinClaim)
	require.NoError(t, err)
	assert.Equal(t, before, count(), "a card that is already as it should be is not committed again")
}

// The reinstallation of the person (the member_id of hub.toml): their card is
// changed where the form shows it, the rest of the record is kept.
func TestJoinMemberOwnChangesTheFormFieldsOnly(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)
	require.NoError(t, r.AddMember(ctx, Member{ID: "ben", DisplayName: "Ben", GitLabUsername: "old", TrackerUsername: "ben.t",
		MattermostUsername: "ben.m", Role: "lead", DefaultMode: "auto"}))

	_, err := r.JoinMember(ctx, Member{ID: "ben", DisplayName: "Benjamin", GitLabUsername: "ben.g", Role: "reviewer"}, JoinOwn)
	require.NoError(t, err)
	got, err := r.GetMember(ctx, "ben")
	require.NoError(t, err)
	assert.Equal(t, Member{ID: "ben", DisplayName: "Benjamin", GitLabUsername: "ben.g", TrackerUsername: "ben.t",
		MattermostUsername: "ben.m", Role: "reviewer", DefaultMode: "auto"}, *got)
}
