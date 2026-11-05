package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

type recordingJoiner struct{ joined []teamstate.Member }

func (r *recordingJoiner) JoinMember(_ context.Context, m teamstate.Member, _ teamstate.JoinMode) (teamstate.JoinResult, error) {
	r.joined = append(r.joined, m)
	return teamstate.JoinResult{Member: m, Created: true}, nil
}

// The card of « Rejoindre » is verified before it is published: a handle the
// token does not confirm stops the step, with the choice to publish all the
// same; a token that cannot tell stops nothing.
func TestMemberJoinVerifiesTheIdentityBeforePublishing(t *testing.T) {
	i18ntest.Set(t, "en")
	var id, name, handle, role = "newbie", "New", "new.bie", "dev"
	var members []teamstate.Member
	var adding = true
	joiner := &recordingJoiner{}
	verdict := error(&identityMismatchError{AuthenticatedAs: "someone.else", ExpectedUser: "new.bie", MemberID: "newbie"})
	j := &memberJoin{
		Members: &members, MemberID: &id, Adding: &adding,
		Fields:   identityFields{ID: &id, DisplayName: &name, GitLab: &handle, Role: &role},
		Repo:     func() teamstate.MemberJoiner { return joiner },
		Verify:   func(context.Context, teamstate.Member) error { return verdict },
		PickerID: "p", CardID: "c", HandleID: "h",
	}
	step := j.CardStep()

	err := step.OnDone()
	var mismatch *identityMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Empty(t, joiner.joined, "nothing is published")
	var labels []string
	for _, a := range step.ErrorActions(err) {
		labels = append(labels, a.Label)
	}
	assert.Contains(t, labels, i18n.T("v54a.join.publish_anyway"))
	assert.Contains(t, labels, i18n.T("v532m.join.action_change"))

	for _, a := range step.ErrorActions(err) {
		if a.Label == i18n.T("v54a.join.publish_anyway") {
			assert.True(t, a.Retry)
			a.Before()
		}
	}
	require.NoError(t, step.OnDone())
	require.Len(t, joiner.joined, 1)
	assert.Equal(t, "new.bie", joiner.joined[0].GitLabUsername)

	// the choice was for that card only
	adding = true
	assert.Error(t, step.OnDone())

	// a token that cannot tell, or a card without a handle: nothing stops
	verdict = &identityUnverifiedError{Err: errors.New("403")}
	require.NoError(t, step.OnDone())
	verdict = &identityMismatchError{}
	handle = ""
	require.NoError(t, step.OnDone(), "no handle: nothing to verify")
	assert.Len(t, joiner.joined, 3)
}
