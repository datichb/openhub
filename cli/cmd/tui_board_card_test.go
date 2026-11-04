package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Where the ticket of a card lives: a number is a GitLab issue where the
// tracker is GitLab, a Jira key is Jira's, the rest are Beads tickets.
func TestBoardCardSource(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	gitlab, jira := trackerApp(t, "gitlab"), trackerApp(t, "jira")
	for _, c := range []struct {
		name   string
		app    *app.App
		id     string
		source views.CardSource
		ref    string
	}{
		{"issue", gitlab, "693", views.CardGitLab, "#693"},
		{"beads ticket on a gitlab team", gitlab, "t-sru-ab1", views.CardBeads, ""},
		{"jira key", jira, "SRU-42", views.CardOther, ""},
		{"number on a jira team", jira, "42", views.CardOther, ""},
		{"beads ticket on a jira team", jira, "bd-9z", views.CardBeads, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			src, ref := boardCardSource(c.app, views.TicketContext{ID: c.id})
			assert.Equal(t, c.source, src)
			assert.Equal(t, c.ref, ref)
		})
	}
}

// The request of the launch form of a card: the issue as the request, the
// prefilled inputs of a review, and the claim to start only when asked.
func TestCardLaunchRequest(t *testing.T) {
	ticket := views.TicketContext{ID: "623", Project: "p-dir", ProjectID: "p1"}

	req := cardLaunchRequest(views.CardLaunch{Workflow: "spec", Request: "#623"}, ticket)
	assert.Equal(t, "spec", req.WorkflowID)
	assert.Equal(t, "p1", req.ProjectID)
	assert.Equal(t, map[string]string{"request": "#623"}, req.Prefill)
	assert.Nil(t, req.Card, "framing does not start the work")
	assert.False(t, req.AtOptions)

	req = cardLaunchRequest(views.CardLaunch{Workflow: "quick", Request: "#623", Start: true}, ticket)
	require.NotNil(t, req.Card)
	assert.Equal(t, tuiCardClaim{Project: "p-dir", Ticket: "623"}, *req.Card)

	req = cardLaunchRequest(views.CardLaunch{Workflow: "review", Prefill: map[string]string{"branch": "feat/x"}}, ticket)
	assert.Equal(t, map[string]string{"branch": "feat/x"}, req.Prefill)
	req = cardLaunchRequest(views.CardLaunch{Workflow: "review-feedback", Prefill: map[string]string{"mr": "https://gl/-/merge_requests/3"}}, ticket)
	assert.Equal(t, map[string]string{"mr": "https://gl/-/merge_requests/3"}, req.Prefill)
	req = cardLaunchRequest(views.CardLaunch{Workflow: "review"}, ticket)
	assert.Nil(t, req.Prefill)
}

// A GitLab reference given to the input of a Beads id is explained in the
// language of the user, as a workflow that is not offered here, and not as a
// failure to load.
func TestLaunchErrorForAGitLabRefOnABeadsInput(t *testing.T) {
	i18ntest.Set(t, "fr")
	err := fmt.Errorf("render: %w", &workflow.GitLabRefForBeadsError{Input: "ticket", Value: "#623", Kind: "issue"})
	got := tuiLaunchError(err)
	require.ErrorIs(t, got, views.ErrNotOffered)
	assert.Contains(t, got.Error(), "#623 est une référence GitLab, pas un ticket Beads")
	assert.Contains(t, got.Error(), "ticket", "the input")
	assert.Contains(t, got.Error(), "spec ou quick", "what to launch instead")
	assert.NotContains(t, got.Error(), "GitLab issue, not a Beads id", "not the English error of the engine")

	assert.NotErrorIs(t, tuiLaunchError(errors.New("other")), views.ErrNotOffered)
}

// The session that works on a card is the one its claim names, while it is
// not finished.
func TestBoardActiveSession(t *testing.T) {
	var none *tuiSessions
	assert.Empty(t, none.openSession("ses_1"), "no sessions wiring")

	open, ended := domain.Session{ID: "ses_open", State: domain.RunActive}, domain.Session{ID: "ses_old", State: domain.RunStopped}
	ts := &tuiSessions{}
	ts.cache = append(ts.cache, sessionsvc.View{Session: open}, sessionsvc.View{Session: ended})
	assert.Equal(t, "ses_open", ts.openSession("ses_open"))
	assert.Empty(t, ts.openSession("ses_old"), "finished")
	assert.Empty(t, ts.openSession("ses_unknown"))
	assert.Empty(t, ts.openSession(""))
}

// The limit of tickets in progress, for the work started on a card: the pool
// and a card with no claim are one more ticket in progress, a card the member
// works on or someone else's are not.
func TestCardLaunchRoom(t *testing.T) {
	a, repo := wipTeam(t, 2, 2)
	ctx := t.Context()
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "POOL-1", Project: claimsProject, Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "PLN-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	claimsCreate(t, repo, "B-1", "bob", "")
	room := func(ticket string) error {
		return checkCardRoom(ctx, repo, a, "testuser", claimsProject, ticket)
	}

	requireLimitError(t, room("NEW-1"), 2, 2)
	requireLimitError(t, room("POOL-1"), 2, 2)
	requireLimitError(t, room("PLN-1"), 2, 2)
	assert.NoError(t, room("W-1"), "already in progress: nothing new")
	assert.NoError(t, room("B-1"), "somebody else's")

	// Through the launch: the project of the card, the team of the project.
	project := teamProject(claimsProject)
	a.Projects = &mockProjectStore{projects: []domain.Project{project}}
	requireLimitError(t, cardLaunchRoom(ctx, a, &project, &tuiCardClaim{Project: claimsProject, Ticket: "NEW-1"}), 2, 2)
	requireLimitError(t, cardLaunchRoom(ctx, a, &project, &tuiCardClaim{Ticket: "NEW-1"}), 2, 2)
	assert.NoError(t, cardLaunchRoom(ctx, a, &project, &tuiCardClaim{Ticket: "W-1"}))
	assert.NoError(t, cardLaunchRoom(ctx, a, nil, &tuiCardClaim{Ticket: "NEW-1"}), "no project, no team")
	setTeamLimit(t, repo, intp(0))
	assert.NoError(t, cardLaunchRoom(ctx, a, &project, &tuiCardClaim{Ticket: "NEW-1"}), "0 is no limit")
}

// The work starts on a card: its claim goes in progress for the member with
// the session on it, whatever it was (none, planned, the pool).
func TestStartCardClaim(t *testing.T) {
	_, repo := wipTeam(t, 0, 0)
	ctx := runsvc.WithStartedSession(t.Context(), "ses_card")

	// No claim yet: it is taken in progress.
	note := startCardClaim(ctx, repo, claimsProject, "testuser", "623")
	assert.Contains(t, note, "623")
	c := claimsOnDisk(t, repo, "623")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)
	assert.Equal(t, "ses_card", c.Session)

	// Planned and the member's: in progress.
	_, err := repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "624", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	note = startCardClaim(ctx, repo, claimsProject, "testuser", "624")
	assert.Contains(t, note, "624")
	c = claimsOnDisk(t, repo, "624")
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)
	assert.Equal(t, "ses_card", c.Session)

	// The pool: taken by the member, in progress, with the session.
	_, err = repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "625", Project: claimsProject, Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	startCardClaim(ctx, repo, claimsProject, "testuser", "625")
	c = claimsOnDisk(t, repo, "625")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)
	assert.Equal(t, "ses_card", c.Session)

	// Somebody else's stays theirs; the note says so.
	claimsCreate(t, repo, "626", "bob", "")
	assert.Contains(t, startCardClaim(ctx, repo, claimsProject, "testuser", "626"), "bob")
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "626").ClaimedBy)

	// Twice: nothing more to say.
	assert.NotPanics(t, func() { startCardClaim(ctx, repo, claimsProject, "testuser", "623") })
}

// The launch claims the card of the project of the launch's team once the
// session runs, and a project without team claims nothing.
func TestClaimCardAtLaunch(t *testing.T) {
	a, repo := wipTeam(t, 0, 0)
	project := teamProject(claimsProject)
	a.Projects = &mockProjectStore{projects: []domain.Project{project}}

	note := claimCardAtLaunch(t.Context(), a, &project, &tuiCardClaim{Ticket: "700"}, "ses_7")
	assert.NotEmpty(t, note)
	c := claimsOnDisk(t, repo, "700")
	assert.Equal(t, "ses_7", c.Session)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)
	_, err := repo.GetClaim(t.Context(), claimsProject, "bt-1")
	assert.ErrorIs(t, err, teamstate.ErrClaimNotFound, "nothing but the card is claimed")

	assert.Empty(t, claimCardAtLaunch(t.Context(), a, nil, &tuiCardClaim{Ticket: "701"}, "ses_8"))
}

// Importing the issue of a card needs the project of the card with Beads
// initialised; the project of the card is its own, not the active one.
func TestImportCardIssueNeedsBeadsInTheProjectOfTheCard(t *testing.T) {
	i18ntest.Set(t, "en")
	a, _ := wipTeam(t, 0, 0)
	_, _, err := importCardIssue(a, views.TicketContext{ID: "623", Title: "x", ProjectPath: ""})
	assert.Error(t, err)

	dir := t.TempDir()
	_, _, err = importCardIssue(a, views.TicketContext{ID: "623", Title: "x", ProjectPath: dir})
	require.Error(t, err)
	assert.Equal(t, i18n.T("v53f.card.no_beads"), err.Error())
	assert.False(t, boardCanImport(a, views.TicketContext{ID: "623", ProjectPath: dir}))
}

// With the real bd: the same issue launched twice gives the same Beads
// ticket, linked by its external ref, and a single one exists.
func TestImportCardIssueRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	i18ntest.Set(t, "en")
	a, _ := wipTeam(t, 0, 0)
	dir := bdtest.InitedRepo(t)
	t.Setenv("BEADS_DIR", "")
	ticket := views.TicketContext{ID: "623", Title: "Export CSV", Description: "The users want a CSV.", ProjectPath: dir}
	assert.True(t, boardCanImport(a, ticket))

	id, created, err := importCardIssue(a, ticket)
	require.NoError(t, err)
	assert.True(t, created)
	again, created, err := importCardIssue(a, ticket)
	require.NoError(t, err)
	assert.False(t, created, "the second launch finds the ticket of the first")
	assert.Equal(t, id, again)

	all, err := beads.ListAll(dir)
	require.NoError(t, err)
	require.Len(t, all, 1, "one ticket, not two")
	assert.Equal(t, "gitlab-623", all[0].ExternalRef)
	detail, err := beads.Show(dir, id)
	require.NoError(t, err)
	assert.Contains(t, detail.Description, "The users want a CSV.")
	assert.Contains(t, detail.Description, "#623")
}

// The actions of the board work on the project of the card, the active one
// only when the card has none.
func TestBoardProjectIDIsTheOneOfTheCard(t *testing.T) {
	a, _ := wipTeam(t, 0, 0)
	assert.Equal(t, "other-project", boardProjectID(a, "other-project"))
	assert.Equal(t, claimsProject, boardProjectID(a, ""), "the only project is the active one")
}

// The take key works on the project of the card, not on the active project:
// a board shows the tickets of several projects.
func TestTeamBoardTakesTheTicketInTheProjectOfTheCard(t *testing.T) {
	a, repo := wipTeam(t, 0, 0)
	actions := buildTeamBoardViewConfig(a).Actions
	require.NotNil(t, actions)

	require.NoError(t, actions.OnClaim("another-project", "42"))
	c, err := repo.GetClaim(t.Context(), "another-project", "42")
	require.NoError(t, err)
	assert.Equal(t, "testuser", c.ClaimedBy)
	_, err = repo.GetClaim(t.Context(), claimsProject, "42")
	assert.ErrorIs(t, err, teamstate.ErrClaimNotFound, "not under the active project")

	// A card with no project falls back to the active one.
	require.NoError(t, actions.OnClaim("", "43"))
	assert.Equal(t, "testuser", claimsOnDisk(t, repo, "43").ClaimedBy)

	require.NoError(t, actions.OnStatus("another-project", "42", teamstate.ClaimStatusReview))
	c, err = repo.GetClaim(t.Context(), "another-project", "42")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusReview, c.Status)
	require.NoError(t, actions.OnRelease("another-project", "42"))
	_, err = repo.GetClaim(t.Context(), "another-project", "42")
	assert.ErrorIs(t, err, teamstate.ErrClaimNotFound)
}
