package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/report"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

type memReportStore struct {
	domain.ReportStore
	reports map[string][]domain.Report
}

func (m *memReportStore) AddReport(_ context.Context, r *domain.Report, _ int, _ int64) error {
	r.Seq = len(m.reports[r.SessionID]) + 1
	m.reports[r.SessionID] = append(m.reports[r.SessionID], *r)
	return nil
}

func (m *memReportStore) ListReports(_ context.Context, sid string) ([]domain.Report, error) {
	return m.reports[sid], nil
}

func (m *memReportStore) GetReport(_ context.Context, sid string, seq int) (*domain.Report, error) {
	for _, r := range m.reports[sid] {
		if r.Seq == seq {
			return &r, nil
		}
	}
	return nil, domain.ErrNotFound
}

type sessionsByID map[string]*domain.Session

type byIDStore struct {
	domain.SessionStore
	byID sessionsByID
}

func (s byIDStore) Get(_ context.Context, id string) (*domain.Session, error) {
	if x, ok := s.byID[id]; ok {
		return x, nil
	}
	return nil, domain.ErrNotFound
}

func reportsFixture(t *testing.T, sessions sessionsByID) (*tuiSessions, *report.Service) {
	t.Helper()
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	rs := &report.Service{Store: &memReportStore{reports: map[string][]domain.Report{}}, SessionsDir: t.TempDir(),
		Now: func() time.Time { at = at.Add(time.Minute); return at }}
	a := &app.App{Sessions: byIDStore{byID: sessions}}
	return &tuiSessions{a: a, svc: &sessionsvc.Service{Reports: rs}}, rs
}

func addReport(t *testing.T, rs *report.Service, session, agent, title string) {
	t.Helper()
	_, err := rs.Add(context.Background(), session, agent, "", title, "# "+title+"\n\ncorps de "+session)
	require.NoError(t, err)
}

// On the parent of a fan-out, the reports of the children come with its
// own, by date, each one marked with its child; the id of a report is per
// session, so the entry says where to read it.
func TestSessionReportsFanoutParent(t *testing.T) {
	parent := &domain.Session{ID: "ses_p", Fanout: &domain.SessionFanout{Role: domain.RoleParent, Children: []domain.FanoutChild{
		{Ticket: "oh-12", Session: "ses_c1"},
		{Ticket: "oh-13", Session: "ses_c2", Task: &domain.PlanTask{Title: "Fix | login"}},
		{Ticket: "oh-14"}, // not launched yet
	}}}
	tui, rs := reportsFixture(t, sessionsByID{"ses_p": parent})
	addReport(t, rs, "ses_c2", "developer", "second child")
	addReport(t, rs, "ses_p", "reviewer", "parent")
	addReport(t, rs, "ses_c1", "developer", "first child")

	got, err := tui.SessionReports(context.Background(), "ses_p")
	require.NoError(t, err)
	require.Len(t, got, 3)
	var titles, children, owners []string
	for _, r := range got {
		titles = append(titles, r.Title)
		children = append(children, r.Child)
		owners = append(owners, r.Session)
		assert.Equal(t, "rp-1", r.ID)
	}
	assert.Equal(t, []string{"second child", "parent", "first child"}, titles, "by date")
	assert.Equal(t, []string{"oh-13 Fix / login", "", "oh-12"}, children)
	assert.Equal(t, []string{"ses_c2", "", "ses_c1"}, owners)

	_, body, err := tui.CheckpointReport(context.Background(), got[2].Session, got[2].ID)
	require.NoError(t, err)
	assert.Contains(t, body, "corps de ses_c1", "the body is the one of the session of the child")
}

// A child and an ordinary session list their own reports only.
func TestSessionReportsOtherSessions(t *testing.T) {
	child := &domain.Session{ID: "ses_c1", Fanout: &domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "oh-12"}}
	plain := &domain.Session{ID: "ses_x"}
	parent := &domain.Session{ID: "ses_p", Fanout: &domain.SessionFanout{Role: domain.RoleParent, Children: []domain.FanoutChild{{Ticket: "oh-12", Session: "ses_c1"}}}}
	tui, rs := reportsFixture(t, sessionsByID{"ses_p": parent, "ses_c1": child, "ses_x": plain})
	addReport(t, rs, "ses_p", "reviewer", "parent")
	addReport(t, rs, "ses_c1", "developer", "child")
	addReport(t, rs, "ses_x", "developer", "plain")

	for id, title := range map[string]string{"ses_c1": "child", "ses_x": "plain"} {
		got, err := tui.SessionReports(context.Background(), id)
		require.NoError(t, err)
		require.Len(t, got, 1, id)
		assert.Equal(t, title, got[0].Title)
		assert.Empty(t, got[0].Session, id)
		assert.Empty(t, got[0].Child, id)
	}
}
