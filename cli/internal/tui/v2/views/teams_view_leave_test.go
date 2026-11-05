package views

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// leavingShell records the list of choices of d and the confirmation.
type leavingShell struct {
	teamsShell
	selectTitle string
	opts        []SelectOption
	pick        func(string)
}

func (s *leavingShell) ShowSelectModal(title string, opts []SelectOption, _ string, pick func(string)) {
	s.selectTitle, s.opts, s.pick = title, opts, pick
}

// A team with a team-state offers two ways out on d: leave it, or only take it
// out of hub.toml. Leaving shows what it does, and runs only on the
// confirmation, off the event loop.
func TestTeamsViewDeleteOffersToLeave(t *testing.T) {
	i18ntest.Set(t, "en")
	h := newTeamsHost()
	var mu sync.Mutex
	var left []string
	deps := h.deps()
	deps.PrepareLeave = func(teamID string, removeMember bool) TeamLeave {
		return TeamLeave{
			Plan: func(context.Context) []string {
				return []string{"release the 2 open claim(s) of alice", "delete the clone"}
			},
			Leave: func(context.Context) ([]string, error) {
				mu.Lock()
				defer mu.Unlock()
				left = append(left, teamID+":"+map[bool]string{false: "card kept", true: "card removed"}[removeMember])
				if teamID == "side" {
					return nil, errors.New("team-state unreachable")
				}
				return []string{"2 claim(s) released"}, nil
			},
		}
	}
	v := NewTeamsView(deps)
	sh := &leavingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })

	choose := func(team, choice string) {
		onLoop(app, func() {
			v.selectTeam(team)
			v.handleDelete()
			sh.pick(choice)
		})
	}

	onLoop(app, func() { v.selectTeam("core"); v.handleDelete() })
	require.Len(t, sh.opts, 3)
	assert.Equal(t, []string{"leave", "leave_card", "remove"}, []string{sh.opts[0].Value, sh.opts[1].Value, sh.opts[2].Value})
	assert.Contains(t, sh.selectTitle, "Core")

	choose("core", "leave_card")
	require.Eventually(t, func() bool {
		var yes func()
		onLoop(app, func() { yes = sh.yes })
		return yes != nil
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "Leave Core?", sh.teamsShell.title)
	assert.Contains(t, sh.detail, "release the 2 open claim(s) of alice")
	mu.Lock()
	assert.Empty(t, left, "nothing runs before the confirmation")
	mu.Unlock()

	onLoop(app, sh.yes)
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "Left Core") }, 3*time.Second, 20*time.Millisecond)
	assert.Contains(t, sh.lastToast(), "2 claim(s) released")
	mu.Lock()
	assert.Equal(t, []string{"core:card removed"}, left)
	mu.Unlock()

	// a failure is told, with the team
	sh.yes = nil
	choose("side", "leave")
	require.Eventually(t, func() bool {
		var yes func()
		onLoop(app, func() { yes = sh.yes })
		return yes != nil
	}, 3*time.Second, 20*time.Millisecond)
	onLoop(app, sh.yes)
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "unreachable") }, 3*time.Second, 20*time.Millisecond)
	assert.Contains(t, sh.lastToast(), "side")
	onLoop(app, v.Unmount)
}

// The other choice is the removal from hub.toml that d always did.
func TestTeamsViewDeleteStillRemovesFromHubToml(t *testing.T) {
	i18ntest.Set(t, "en")
	h := newTeamsHost()
	deps := h.deps()
	deps.PrepareLeave = func(string, bool) TeamLeave { return TeamLeave{} }
	v := NewTeamsView(deps)
	sh := &leavingShell{}
	v.SetShell(sh)
	v.rebuild()

	v.selectTeam("side")
	v.handleDelete()
	require.NotNil(t, sh.pick)
	sh.pick("remove")
	require.NotNil(t, sh.yes, "the removal is confirmed as before")
	assert.Contains(t, sh.teamsShell.title, "side")

	// a solo space has no team-state to leave: the removal is asked at once
	sh.yes, sh.pick = nil, nil
	v.selectTeam("mine")
	v.handleDelete()
	assert.Nil(t, sh.pick)
	assert.NotNil(t, sh.yes)
}
