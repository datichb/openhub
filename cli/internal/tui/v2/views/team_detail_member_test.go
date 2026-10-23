package views

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// memberProfileEnv is a team detail whose member profile lives in memory.
type memberProfileEnv struct {
	*tokenEnv
	member teamstate.Member
	saves  [][2]string
	err    error
}

func newMemberProfileEnv(t *testing.T) *memberProfileEnv {
	t.Helper()
	e := &memberProfileEnv{
		tokenEnv: newTokenEnv(t, "gitlab"),
		member:   teamstate.Member{ID: "alice", DisplayName: "Alice", GitLabUsername: "alice1", Role: "dev"},
	}
	e.v.cfg.ResolveTeam = func() TeamResolution {
		return TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir(), StateRepo: "git@gitlab.corp.example:acme/core.git", MemberID: "alice"}
	}
	e.v.cfg.GetMember = func() *teamstate.Member { m := e.member; return &m }
	e.v.cfg.SaveMemberField = func(_ context.Context, field, value string) error {
		if e.err != nil {
			return e.err
		}
		e.saves = append(e.saves, [2]string{field, value})
		switch field {
		case MemberFieldDisplayName:
			e.member.DisplayName = value
		case MemberFieldTrackerUsername:
			e.member.TrackerUsername = value
		}
		return nil
	}
	e.v.loadData()
	e.v.buildFields()
	return e
}

// Bug 12: no view showed or changed the profile of the member, so
// tracker_username could only be set at team.init → Identity.
func TestTeamDetailShowsAndEditsTheMemberProfile(t *testing.T) {
	e := newMemberProfileEnv(t)

	for _, key := range []string{"member_display_name", "member_gitlab_username", "member_mattermost_username", "member_tracker_username"} {
		_, ok := fieldByKey(e.v, key)
		assert.True(t, ok, "the profile has a %s field", key)
	}
	assert.Contains(t, e.shown(t, "member_display_name"), "Alice")
	assert.Contains(t, e.shown(t, "member_tracker_username"), i18n.T("tui.config.not_configured"), "an empty tracker name is shown as not set")

	f := e.field(t, "member_tracker_username")
	f.Set("@benjamin.datiche")
	require.Equal(t, [][2]string{{MemberFieldTrackerUsername, "@benjamin.datiche"}}, e.saves, "written at once to the team-state, not with w")
	assert.False(t, e.v.dirtyTeam)
	assert.False(t, e.v.dirtyLocal)
	assert.Contains(t, e.shown(t, "member_tracker_username"), "@benjamin.datiche", "the view shows what the profile now holds")
	assert.Contains(t, e.sh.lastToast(), "@benjamin.datiche")
}

func TestTeamDetailMemberProfileSaveFailureIsSaid(t *testing.T) {
	e := newMemberProfileEnv(t)
	e.err = errors.New("push rejected")
	e.field(t, "member_display_name").Set("Bob")
	assert.Contains(t, e.sh.lastToast(), "push rejected")
	assert.Equal(t, "Alice", e.v.member.DisplayName, "unchanged")
}

// A display name cannot be emptied; the other names can.
func TestTeamDetailMemberProfileRules(t *testing.T) {
	e := newMemberProfileEnv(t)
	assert.Error(t, e.field(t, "member_display_name").Validator.Validate(""))
	assert.NoError(t, e.field(t, "member_tracker_username").Validator.Validate(""))
}

// Without a member (not rejoined) or in a solo space there is no profile section.
func TestTeamDetailNoMemberProfileWithoutMember(t *testing.T) {
	e := newMemberProfileEnv(t)
	e.v.cfg.ResolveTeam = func() TeamResolution { return TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir()} }
	e.v.buildFields()
	_, ok := fieldByKey(e.v, "member_tracker_username")
	assert.False(t, ok)
}
