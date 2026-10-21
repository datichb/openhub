package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// childReportsSessions reads the report bodies per (session, id): the ids
// are per session, rp-1 exists in each child.
type childReportsSessions struct {
	tabSessions
	rmu    sync.Mutex
	reads  []string
	bodies map[string]string
}

func (s *childReportsSessions) CheckpointReport(_ context.Context, session, id string) (string, string, error) {
	s.rmu.Lock()
	defer s.rmu.Unlock()
	s.reads = append(s.reads, session+"/"+id)
	return id, s.bodies[session+"/"+id], nil
}

func (s *childReportsSessions) readList() []string {
	s.rmu.Lock()
	defer s.rmu.Unlock()
	return append([]string(nil), s.reads...)
}

// On the parent of a fan-out, the reports of the children are listed with
// the child that filed them, and each one is read in its own session (the
// ids rp-1 of two children are not mixed up). The backend lists them oldest
// first, as the adapter does.
func TestSessionsReportsTabFanoutChildren(t *testing.T) {
	frLocale(t)
	at := func(h, m int) time.Time { return time.Date(2026, 10, 15, h, m, 0, 0, time.Local) }
	be := &childReportsSessions{
		tabSessions: tabSessions{fakeSessions: fakeSessions{rows: fanoutRows()}, fanout: sampleFanout(),
			reports: []CheckpointReport{
				{ID: "rp-1", Agent: "developer", Title: "oh-12 terminé", Bytes: 2100, At: at(10, 9), Session: "ses_91aa01", Child: "oh-12"},
				{ID: "rp-1", Agent: "developer", Title: "oh-13 en cours", Bytes: 900, At: at(10, 15), Session: "ses_91ab01", Child: "oh-13"},
				{ID: "rp-1", Agent: "reviewer", Title: "Revue de l'intégration", Bytes: 4200, At: at(10, 28)},
			}},
		bodies: map[string]string{
			"ses_8f2a01/rp-1": "Corps de la parente",
			"ses_91aa01/rp-1": "Corps de oh-12",
			"ses_91ab01/rp-1": "Corps de oh-13",
		},
	}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(&dialogShell{})
	onLoop := harness(t, v, be.rows)
	onLoop(func() { v.selectKey("s:ses_8f2a01"); v.onSelectionChanged() })
	onLoop(func() { v.showTab(tabReports) })
	reader := func() string {
		var text string
		onLoop(func() { text = strings.Join(v.tabs.reports.reader.lines, "\n") })
		return text
	}
	require.Eventually(t, func() bool { return reader() != "" }, 3*time.Second, 10*time.Millisecond)

	var ids []string
	onLoop(func() {
		for _, it := range v.tabs.reports.list.Items() {
			ids = append(ids, it.ID)
		}
		golden.Assert(t, "sessions_tab_reports_fanout", screenText(t, v.tabs.root, 120, 11))
	})
	require.Equal(t, []string{"rp-1", "ses_91ab01/rp-1", "ses_91aa01/rp-1"}, ids, "three reports of the same id, the last one first")

	for _, tc := range []struct{ item, read, body string }{
		{"ses_91ab01/rp-1", "ses_91ab01/rp-1", "Corps de oh-13"},
		{"ses_91aa01/rp-1", "ses_91aa01/rp-1", "Corps de oh-12"},
		{"rp-1", "ses_8f2a01/rp-1", "Corps de la parente"},
	} {
		onLoop(func() { v.tabs.reports.list.Select(tc.item) })
		require.Eventually(t, func() bool { return reader() == tc.body }, 3*time.Second, 10*time.Millisecond)
		assert.Contains(t, be.readList(), tc.read, "read in the session that holds the report")
	}
}
