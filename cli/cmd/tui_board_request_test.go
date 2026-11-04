package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
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

// On an issue card the board used to offer only the workflows taking a
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

// hubCatalog is the catalogue of the workflows shipped with oh.
func hubCatalog(t *testing.T) []workflowsvc.Summary {
	t.Helper()
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, "workflows")); err != nil {
		t.Skip("hub content not found")
	}
	svc := &workflowsvc.Service{HubDir: root, HubWorkflowsDir: filepath.Join(root, "workflows"), Lang: "fr"}
	list, err := svc.Catalog(t.Context(), workflowsvc.Context{})
	require.NoError(t, err)
	var valid []workflowsvc.Summary
	for _, s := range list {
		if s.Valid {
			valid = append(valid, s)
		}
	}
	return valid
}

// The menu of the card of a GitLab issue offers, besides the workflows that
// take a Beads ticket, the ones of the hub that take a request: the cadrage
// (spec), quick, libre, infra-change, and the two that have a main input of
// their own, which the issue fills: spike (question) and incident. The retro
// takes no free request (it looks at past sessions): it is not offered.
func TestBoardMenuOfTheHubWorkflows(t *testing.T) {
	ts := testTUIStart()
	ts.catalog = hubCatalog(t)
	offered := workflowIDsOf(ts.requestWorkflows(views.StartScope{}))
	for _, id := range []string{"spec", "quick", "libre", "infra-change", "spike", "incident"} {
		assert.Contains(t, offered, id)
	}
	for _, id := range []string{"ticket", "review", "sweep", "release", "merge-conflicts", "retro"} {
		assert.NotContains(t, offered, id, "a workflow with a ticket, or with no free request")
	}
	for id, input := range map[string]string{"spec": "request", "infra-change": "request", "spike": "question", "incident": "incident", "retro": "request", "unknown": "request"} {
		assert.Equal(t, input, ts.requestInput(id), id)
	}
}

// Every workflow that the menu of a card offers, whether it takes a ticket or
// a request, shows a short title of its own in both languages, not its
// description (a sentence of a hundred characters).
func TestBoardMenuLabels(t *testing.T) {
	ts := testTUIStart()
	ts.catalog = hubCatalog(t)
	for _, lang := range []string{"fr", "en"} {
		prev := i18n.Locale()
		i18n.SetLocale(lang)
		labels, own := map[string]string{}, map[string]string{}
		for _, w := range append(ts.ticketWorkflows(views.StartScope{}), ts.requestWorkflows(views.StartScope{})...) {
			labels[w.ID], own[w.ID] = w.Label, i18n.T("tui.board.label."+w.ID)
		}
		i18n.SetLocale(prev)
		require.NotEmpty(t, labels)
		for id, label := range labels {
			assert.NotEmpty(t, label, "%s %s", lang, id)
			assert.Equal(t, own[id], label, "%s %s: a title of its own, not the description", lang, id)
			assert.LessOrEqual(t, len([]rune(label)), 40, "%s %s: a title, not a sentence: %q", lang, id, label)
		}
		assert.Contains(t, labels["spike"], "spike")
	}
}

// A workflow that takes a ticket, launched on the card of a GitLab issue,
// used to get the bare IID as a Beads id ("693" looks like one, and the
// session then ran `bd show 693`). The card is named as the issue it is
// (#693); a Beads ticket and a Jira key are passed as they are.
func TestBoardTicketLaunchNamesTheGitLabIssue(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	for _, c := range []struct {
		name   string
		app    *app.App
		ticket string
		want   string
	}{
		{"gitlab issue", trackerApp(t, "gitlab"), "693", "#693"},
		{"beads ticket on a gitlab board", trackerApp(t, "gitlab"), "bd-42", "bd-42"},
		{"jira key", trackerApp(t, "jira"), "SRU-42", "SRU-42"},
		{"number on a jira board", trackerApp(t, "jira"), "42", "42"},
		{"no tracker", trackerApp(t, ""), "693", "693"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := boardTicketLaunch(c.app, "ticket", views.TicketContext{ID: c.ticket, ProjectID: "p1"})
			assert.Equal(t, []string{c.want}, req.Tickets)
			assert.Equal(t, "ticket", req.WorkflowID)
			assert.Equal(t, "p1", req.ProjectID)
			assert.True(t, req.AtOptions)
		})
	}
}
