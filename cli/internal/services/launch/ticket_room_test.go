package launch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
)

// A launch on tickets asks the check of the team, before anything starts,
// with the tickets of the sessions that start something new: a session that
// continues an existing one (resume, transfer) starts none, and a launch on
// no ticket asks nothing.
func TestCheckTicketRoomAsksForTheTicketsThatStart(t *testing.T) {
	var asked []string
	var project string
	room := func(_ *domain.Project) func(context.Context, string, []string) error {
		return func(_ context.Context, projectID string, tickets []string) error {
			project, asked = projectID, append([]string(nil), tickets...)
			return nil
		}
	}
	plan := &runsvc.RunPlan{Sessions: []runsvc.PlannedSession{
		{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-1", "T-2"}}},
		{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-3"}}, Existing: "ses_a", Action: runsvc.OnExistingResume},
		{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-4"}}, Existing: "ses_b", Action: runsvc.OnExistingTransfer},
		{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-5"}}, Existing: "ses_c", Action: runsvc.OnExistingKeep},
	}}
	p := &Prepared{Project: &domain.Project{ID: "proj"}, Plan: plan, deps: Deps{TicketRoom: room}}
	require.NoError(t, checkTicketRoom(t.Context(), p))
	assert.Equal(t, "proj", project)
	assert.Equal(t, []string{"T-1", "T-2", "T-5"}, asked)

	asked = nil
	p.Plan = &runsvc.RunPlan{Sessions: []runsvc.PlannedSession{{}}}
	require.NoError(t, checkTicketRoom(t.Context(), p))
	assert.Nil(t, asked, "no ticket, no question")

	// A remote launch asks for the tickets of its sessions.
	p.Remote = &RemotePrep{tickets: [][]string{{"R-1"}, {"R-2", "R-3"}}}
	require.NoError(t, checkTicketRoom(t.Context(), p))
	assert.Equal(t, []string{"R-1", "R-2", "R-3"}, asked)
}

// The refusal of the check is the one Start returns, before the plan is
// started or anything is sent.
func TestStartIsRefusedWhenTheTeamHasNoRoom(t *testing.T) {
	refused := errors.New("limit reached")
	p := &Prepared{Project: &domain.Project{ID: "proj"}, deps: Deps{
		TicketRoom: func(*domain.Project) func(context.Context, string, []string) error {
			return func(context.Context, string, []string) error { return refused }
		},
	}, Plan: &runsvc.RunPlan{Sessions: []runsvc.PlannedSession{{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-1"}}}}}}
	_, err := Start(t.Context(), p, StartOptions{})
	require.ErrorIs(t, err, refused)

	p.Remote = &RemotePrep{tickets: [][]string{{"T-1"}}}
	_, err = Start(t.Context(), p, StartOptions{})
	require.ErrorIs(t, err, refused)

	// Without a team there is nothing to check.
	p.deps.TicketRoom = nil
	assert.NoError(t, checkTicketRoom(t.Context(), p))
	p.deps.TicketRoom = func(*domain.Project) func(context.Context, string, []string) error { return nil }
	assert.NoError(t, checkTicketRoom(t.Context(), p))
}

// A ticket another claim covers (NoClaim: the card of the board whose issue
// was imported as that Beads ticket) is neither asked to the check nor given
// to the hook that claims the tickets of a started session.
func TestNoClaimTicketsAreLeftOutOfTheRoomAndTheClaims(t *testing.T) {
	var asked []string
	room := func(*domain.Project) func(context.Context, string, []string) error {
		return func(_ context.Context, _ string, tickets []string) error {
			asked = append([]string(nil), tickets...)
			return errors.New("no room")
		}
	}
	plan := &runsvc.RunPlan{Sessions: []runsvc.PlannedSession{
		{PlannedInput: runsvc.PlannedInput{Tickets: []string{"T-1", "T-2"}}},
	}}
	p := &Prepared{Project: &domain.Project{ID: "proj"}, Plan: plan, deps: Deps{TicketRoom: room}, Opts: Options{NoClaim: []string{"T-1", "T-2"}}}
	require.NoError(t, checkTicketRoom(t.Context(), p), "nothing is left to ask: no refusal")
	assert.Nil(t, asked)

	p.Opts.NoClaim = []string{"T-2"}
	require.Error(t, checkTicketRoom(t.Context(), p))
	assert.Equal(t, []string{"T-1"}, asked)

	var claimed []string
	hook := func(_ context.Context, _ string, tickets []string) []string {
		claimed = append([]string(nil), tickets...)
		return []string{"note"}
	}
	wrapped := withoutTickets(hook, []string{"T-2"})
	assert.Equal(t, []string{"note"}, wrapped(t.Context(), "proj", []string{"T-1", "T-2"}))
	assert.Equal(t, []string{"T-1"}, claimed)
	claimed = nil
	assert.Nil(t, wrapped(t.Context(), "proj", []string{"T-2"}), "only the skipped ticket: the hook is not called")
	assert.Nil(t, claimed)
	assert.Nil(t, withoutTickets(nil, []string{"T-2"}), "no hook stays no hook")
	assert.NotNil(t, withoutTickets(hook, nil))
}
