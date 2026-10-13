package workflow

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

type reportCall struct{ session, title, body string }

func (f *fakeBackend) WorkflowReport(_ context.Context, s, title, body string) (daemon.WorkflowReport, error) {
	f.reports = append(f.reports, reportCall{s, title, body})
	if f.reportErr != nil {
		return daemon.WorkflowReport{}, f.reportErr
	}
	return daemon.WorkflowReport{ID: "rp-3", Report: domain.Report{Seq: 3, Agent: "developer", Title: title, Bytes: 2150}}, nil
}

func (f *fakeBackend) WorkflowReports(_ context.Context, s string) ([]daemon.WorkflowReport, error) {
	f.sessions = append(f.sessions, s)
	if f.reportErr != nil {
		return nil, f.reportErr
	}
	at := time.Date(2026, 10, 13, 10, 3, 0, 0, time.Local)
	return []daemon.WorkflowReport{{ID: "rp-1", Report: domain.Report{Seq: 1, Agent: "developer", Title: "Fait", Bytes: 800, CreatedAt: at,
		Sections: []string{"Changements par fichier", "Tests"}}}}, nil
}

func (f *fakeBackend) WorkflowReportGet(_ context.Context, s, id, section string) (daemon.WorkflowReport, error) {
	f.sessions = append(f.sessions, s+"/"+id+"/"+section)
	if f.reportErr != nil {
		return daemon.WorkflowReport{}, f.reportErr
	}
	return daemon.WorkflowReport{ID: id, Report: domain.Report{Agent: "reviewer", Title: "Review"}, Body: "## Verdict\ncorriger"}, nil
}

func TestReportCall(t *testing.T) {
	i18n.SetLocale("fr")
	b := &fakeBackend{}
	// An author given by the agent is not even read.
	out := call(t, b, "tools/call", `{"name":"report","arguments":{"title":"Fait","body":"## Tests\nok","agent":"reviewer"},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	assert.Equal(t, []reportCall{{"ses_child", "Fait", "## Tests\nok"}}, b.reports)
	txt := out.Result.Content[0].Text
	assert.Contains(t, txt, "rp-3")
	assert.Contains(t, txt, "developer")
	assert.Contains(t, txt, "2.1 Ko")

	out = call(t, b, "tools/call", `{"name":"report","arguments":{"title":"x","body":"  "},`+meta+`}`)
	assert.True(t, out.Result.IsError, "body required")
	out = call(t, b, "tools/call", `{"name":"report","arguments":{"body":"x"}}`)
	assert.True(t, out.Result.IsError, "no session")
}

func TestReportWithoutDaemonFallsBackOnTheBlock(t *testing.T) {
	i18n.SetLocale("fr")
	for _, err := range []error{daemon.ErrNotRunning, &daemon.APIError{Status: http.StatusServiceUnavailable, Message: "off"},
		&daemon.APIError{Status: http.StatusNotFound, Message: "404 Not Found"}} {
		out := call(t, &fakeBackend{reportErr: err}, "tools/call", `{"name":"report","arguments":{"body":"x"},`+meta+`}`)
		require.False(t, out.Result.IsError, "the agent goes on")
		assert.Contains(t, out.Result.Content[0].Text, "non enregistré")
		assert.Contains(t, out.Result.Content[0].Text, "rapport complet")
	}
	out := call(t, &fakeBackend{reportErr: &daemon.APIError{Status: http.StatusUnprocessableEntity, Message: "the report is larger than 64 KiB"}},
		"tools/call", `{"name":"report","arguments":{"body":"x"},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Contains(t, out.Result.Content[0].Text, "64 KiB")
	assert.Contains(t, out.Result.Content[0].Text, "rapport complet")

	out = call(t, &fakeBackend{reportErr: daemon.ErrNotRunning}, "tools/call", `{"name":"reports","arguments":{},`+meta+`}`)
	assert.True(t, out.Result.IsError)
}

func TestReportGetAndList(t *testing.T) {
	i18n.SetLocale("fr")
	b := &fakeBackend{}
	out := call(t, b, "tools/call", `{"name":"report_get","arguments":{"id":" rp-2 ","section":"Verdict"},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	assert.Equal(t, []string{"ses_child/rp-2/Verdict"}, b.sessions)
	assert.Contains(t, out.Result.Content[0].Text, "rp-2 · reviewer · Review")
	assert.Contains(t, out.Result.Content[0].Text, "corriger")
	out = call(t, b, "tools/call", `{"name":"report_get","arguments":{},`+meta+`}`)
	assert.True(t, out.Result.IsError, "id required")

	out = call(t, b, "tools/call", `{"name":"reports","arguments":{},`+meta+`}`)
	require.False(t, out.Result.IsError)
	assert.Equal(t, "rp-1 · developer · « Fait » · 800 o · 10:03 · Changements par fichier | Tests", out.Result.Content[0].Text)

	api := &daemon.APIError{Status: http.StatusNotFound, Message: "report rp-9: not found"}
	out = call(t, &fakeBackend{reportErr: api}, "tools/call", `{"name":"report_get","arguments":{"id":"rp-9"},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Equal(t, api.Message, out.Result.Content[0].Text)
}
