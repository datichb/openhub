package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

func boardCatalog() []workflowsvc.Summary {
	text := func(id string) workflowsvc.InputSummary { return workflowsvc.InputSummary{ID: id, Type: wf.InputText} }
	return []workflowsvc.Summary{
		{ID: "ticket", TicketInput: "ticket", Inputs: []workflowsvc.InputSummary{{ID: "ticket", Type: wf.InputBeadsID}}},
		{ID: "spec", Inputs: []workflowsvc.InputSummary{text("request"), {ID: "source", Type: wf.InputPath}}},
		{ID: "quick", Inputs: []workflowsvc.InputSummary{text("request")}},
		{ID: "debug", Inputs: []workflowsvc.InputSummary{text("symptom")}},
		{ID: "mixed", TicketInput: "ticket", Inputs: []workflowsvc.InputSummary{{ID: "ticket", Type: wf.InputBeadsID}, text("request")}},
		{ID: "flag", Inputs: []workflowsvc.InputSummary{{ID: "request", Type: wf.InputBool}}},
	}
}

func workflowIDsOf(ws []views.TicketWorkflow) []string {
	var ids []string
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	return ids
}

// A1: on an issue card the board used to offer only the workflows taking a
// Beads ticket (the cadrage, a free request, was not among them). The ones
// taking a request are now listed apart; the ticket ones are unchanged.
func TestBoardWorkflowsOfARequest(t *testing.T) {
	ts := testTUIStart()
	ts.catalog = boardCatalog()
	assert.Equal(t, []string{"ticket", "mixed"}, workflowIDsOf(ts.ticketWorkflows(views.StartScope{})))
	assert.Equal(t, []string{"spec", "quick"}, workflowIDsOf(ts.requestWorkflows(views.StartScope{})),
		"a workflow with a text request and no Beads ticket; a bool or another name is not a request")
}

func TestBoardLaunchRequestPrefillsTheIssue(t *testing.T) {
	req := boardLaunchRequest("spec", views.TicketContext{ID: "693", ProjectID: "p1"}, tuiLaunchRequest{Prefill: map[string]string{boardRequestInput: "#693"}})
	assert.Equal(t, "spec", req.WorkflowID)
	assert.Equal(t, "p1", req.ProjectID)
	assert.Equal(t, map[string]string{"request": "#693"}, req.Prefill)
	assert.Empty(t, req.Tickets, "the issue is the request, not a Beads ticket")
	assert.False(t, req.AtOptions, "the form opens on its inputs, where the request can be completed")

	req = boardLaunchRequest("ticket", views.TicketContext{ID: "bd-42"}, tuiLaunchRequest{Tickets: []string{"bd-42"}, AtOptions: true})
	assert.Equal(t, []string{"bd-42"}, req.Tickets)
	assert.True(t, req.AtOptions)
}

func trackerApp(t *testing.T, trackerType string) *app.App {
	t.Helper()
	statePath := filepath.Join(t.TempDir(), "state")
	_, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	if trackerType != "" {
		require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte("[tracker]\ntype = \""+trackerType+"\"\n"), 0o644))
	}
	return &app.App{Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@gitlab.example:acme/core.git", StatePath: statePath, MemberID: "alice"},
	}}}
}

// The team board names a GitLab issue by its bare IID: a number is an issue
// where the tracker is GitLab; a Beads id and a Jira key never are.
func TestBoardIssueRef(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	gitlab := trackerApp(t, "gitlab")
	assert.Equal(t, "#693", boardIssueRef(gitlab, views.TicketContext{ID: "693"}))
	assert.Empty(t, boardIssueRef(gitlab, views.TicketContext{ID: "bd-42"}))
	assert.Empty(t, boardIssueRef(gitlab, views.TicketContext{ID: "SRU-42"}))
	assert.Empty(t, boardIssueRef(gitlab, views.TicketContext{ID: ""}))
	assert.Empty(t, boardIssueRef(trackerApp(t, "jira"), views.TicketContext{ID: "42"}), "a number on a Jira board is not a GitLab issue")
	assert.Empty(t, boardIssueRef(trackerApp(t, ""), views.TicketContext{ID: "693"}), "no tracker declared")
	assert.Empty(t, boardIssueRef(nil, views.TicketContext{ID: "693"}))
}
