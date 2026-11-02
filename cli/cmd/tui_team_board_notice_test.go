package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// A2: the board says how many issues the auto-plan left off the board, from
// the sync state of the machine, for the projects it reads.
func TestAutoPlanNoticeSaysWhatTheLimitHeldBack(t *testing.T) {
	i18ntest.Set(t, "en")
	stateDir := t.TempDir()
	a := &app.App{Config: &config.Config{}, Projects: &mockProjectStore{projects: []domain.Project{
		{ID: "p1", Status: domain.ProjectStatusActive}, {ID: "p2", Status: domain.ProjectStatusActive},
	}}}
	assert.Empty(t, autoPlanNoticeText(a, stateDir), "nothing synced yet")

	state, err := tracker.LoadSyncState(stateDir)
	require.NoError(t, err)
	state.SetAutoPlanHeld("p1", 5, []tracker.HeldIssue{{TicketID: "7", MemberID: "alice"}, {TicketID: "8", MemberID: "alice"}})
	state.SetAutoPlanHeld("p2", 3, []tracker.HeldIssue{{TicketID: "9", MemberID: "bob"}})
	state.SetAutoPlanHeld("elsewhere", 5, []tracker.HeldIssue{{TicketID: "1"}})
	require.NoError(t, state.Save(stateDir))

	assert.Equal(t, "3 issue(s) outside the board, limit reached (max_auto_plan_per_member = 5)", autoPlanNoticeText(a, stateDir),
		"the projects it reads, and the largest limit")

	state.SetAutoPlanHeld("p1", 5, nil)
	state.SetAutoPlanHeld("p2", 3, nil)
	require.NoError(t, state.Save(stateDir))
	assert.Empty(t, autoPlanNoticeText(a, stateDir), "nothing held: nothing said")
	assert.Empty(t, autoPlanNoticeText(nil, stateDir))
}
