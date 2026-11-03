package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// The checks of the doctor that could not read what they check say so: a
// warning (the doctor stays green, the exit code does not change) that names
// the cause and what was not checked, never "nothing to report".

var errUnreadable = errors.New("disk on fire")

type failingProjects struct{ domain.ProjectStore }

func (failingProjects) List(context.Context, domain.ProjectStatus) ([]domain.Project, error) {
	return nil, errUnreadable
}

type failingSessions struct{ domain.SessionStore }

func (failingSessions) List(context.Context, string) ([]domain.Session, error) {
	return nil, errUnreadable
}

func useApp(t *testing.T, a *app.App) {
	t.Helper()
	prev := application
	application = a
	t.Cleanup(func() { application = prev })
}

func requireUnverifiedWarning(t *testing.T, detail string, notes ...string) {
	t.Helper()
	assert.Contains(t, detail, errUnreadable.Error(), "the cause")
	for _, n := range notes {
		assert.Contains(t, detail, n)
	}
}

func TestClaimsCheckWarnsWhenTheProjectsCannotBeListed(t *testing.T) {
	i18ntest.Set(t, "en")
	useApp(t, &app.App{Projects: failingProjects{}, Sessions: noSessions{}})

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "a warning: %+v", res[0])
	requireUnverifiedWarning(t, res[0].Detail, "orphan claims not checked")
	assert.NotContains(t, res[0].Detail, "no orphan claim")
}

func TestClaimsCheckWarnsWhenTheClaimsOfAProjectCannotBeRead(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	// An ID the team-state refuses: ListClaims fails for this project.
	f.addProject(t, "bad/id", "broken", "")

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	r := res[0]
	assert.Equal(t, doctorIDClaimsProject, r.ID, "the claims under the former ID are still reported")
	assert.False(t, r.OK, "the former ID needs its repair: %s", r.Detail)
	assert.Contains(t, r.Detail, "t-sru-b267fbf1")
	assert.Contains(t, r.Detail, "broken [bad/id]", "the project, by name and ID")
	assert.Contains(t, r.Detail, "orphan claims not checked")
	assert.Contains(t, r.Detail, "invalid project name", "the cause")

	// Nothing else to report: the warning replaces « no orphan claim ».
	f.app.Projects = &mockProjectStore{projects: []domain.Project{f.project, {ID: "bad/id", Name: "broken", Path: f.project.Path, Status: domain.ProjectStatusActive, TeamID: f.project.TeamID}}}
	_, err := f.repo.RehomeClaims(t.Context(), "testuser", []teamstate.ClaimMove{{From: "t-sru-b267fbf1", To: f.project.ID}})
	require.NoError(t, err)
	res = claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "%+v", res[0])
	assert.NotContains(t, res[0].Detail, "no orphan claim")
	assert.Contains(t, res[0].Detail, "broken [bad/id]")
}

func TestClaimsCheckWarnsWhenTheSessionsCannotBeRead(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	_, err := f.repo.RehomeClaims(t.Context(), "testuser", []teamstate.ClaimMove{{From: "t-sru-b267fbf1", To: f.project.ID}})
	require.NoError(t, err)
	f.app.Sessions = failingSessions{}

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "%+v", res[0])
	requireUnverifiedWarning(t, res[0].Detail, "t-sru [88280c5b]", "ended sessions")
	assert.NotContains(t, res[0].Detail, "no orphan claim")
}

func TestClaimsCheckWarnsWhenTheClaimsOfATeamCannotBeRead(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	clone := f.repo.Path()
	require.NoError(t, os.RemoveAll(filepath.Join(clone, "projects")))
	require.NoError(t, os.WriteFile(filepath.Join(clone, "projects"), []byte("not a folder"), 0o644))

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "%+v", res[0])
	assert.Contains(t, res[0].Detail, clone)
	assert.Contains(t, res[0].Detail, "former project ID not checked")
	assert.NotContains(t, res[0].Detail, "no orphan claim")
}

func TestClaimsReportKeepsWhatItFoundNextToWhatItCouldNotRead(t *testing.T) {
	i18ntest.Set(t, "en")
	res := claimsReport(nil, nil, nil, "project x: claims unreadable (boom)")
	assert.True(t, res.OK && res.Warn)
	assert.NotContains(t, res.Detail, "no orphan claim")
	assert.Contains(t, res.Detail, "boom")

	res = claimsReport([]orphanClaim{{Project: "p", ProjectName: "api", Ticket: "T-1", Reason: "closed"}}, nil, nil, "boom")
	assert.False(t, res.OK, "an orphan claim still fails the check")
	assert.Contains(t, res.Detail, "api [p]/T-1", "the project by name and ID")
	assert.Contains(t, res.Detail, "oh team release T-1 -p p", "the command keeps the ID")
	assert.Contains(t, res.Detail, "boom")

	res = claimsReport(nil, nil, []claimsUnresolved{{From: "old-12345678", Tickets: []string{"SRU-1"}}}, "boom")
	assert.True(t, res.OK && res.Warn)
	assert.Equal(t, doctorIDClaimsUnknown, res.ID)
	assert.Contains(t, res.Detail, "boom")
}

func TestRecordClosedFailuresRemembersTheTicketsItCouldNotRead(t *testing.T) {
	f := recordClosedFailures(func(id string) (bool, error) {
		if id == "T-2" {
			return false, errUnreadable
		}
		return true, nil
	})
	claims := []teamstate.Claim{
		{Project: "p", TicketID: "T-1", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
		{Project: "p", TicketID: "T-2", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
	}
	got := orphanClaims("p", "me", claims, nil, nil, f.check)
	require.Len(t, got, 1)
	assert.Equal(t, "T-1", got[0].Ticket)
	assert.Equal(t, []string{"T-2"}, f.tickets, "T-2 was not known to be open")
	assert.ErrorIs(t, f.first, errUnreadable)
}

func TestClaimsClosedSourceIsNotAFailureWithoutBeads(t *testing.T) {
	closed, err := claimsClosedSource(t.TempDir())("T-1")
	assert.False(t, closed)
	assert.NoError(t, err, "a project that keeps its tickets in a tracker has no status to read")
}

func TestUnknownTicketsAreNotFailuresButOtherErrorsAre(t *testing.T) {
	notFound := errors.New("bd show: warning: beads.role not configured\nFix: git config beads.role maintainer\nIssue 491 not found\nHint: try 'bd history 491'")
	read := func(id string) (bool, error) {
		switch id {
		case "491":
			return false, notFound
		case "closed":
			return true, nil
		}
		return false, errUnreadable
	}
	present, absent := func() error { return nil }, func() error { return errors.New("bd not found in PATH") }

	src := unknownTicketsAreNotFailures(read, present)
	closed, err := src("491")
	assert.False(t, closed)
	assert.NoError(t, err, "a tracker issue Beads does not know has no status to read")
	closed, err = src("closed")
	assert.True(t, closed)
	assert.NoError(t, err)
	_, err = src("other")
	assert.ErrorIs(t, err, errUnreadable, "any other error stays a failure")

	_, err = unknownTicketsAreNotFailures(read, absent)("491")
	assert.Error(t, err, "bd missing: nothing was read")

	assert.Equal(t, "bd show: warning: beads.role not configured Fix: git config beads.role maintainer Issue 491 not found Hint: try 'bd history 491'", oneLine(notFound))
	assert.LessOrEqual(t, len([]rune(oneLine(errors.New(strings.Repeat("x", 500))))), 201)
}

func TestOrphanListNamesTheProjectNextToItsID(t *testing.T) {
	i18ntest.Set(t, "en")
	res := orphanClaimsResult([]orphanClaim{{Project: "88280c5b", ProjectName: "transparence-sru", Ticket: "SRU-1", Reason: "ended"}, {Project: "p2", Ticket: "T-9", Reason: "closed"}})
	assert.Contains(t, res.Detail, "transparence-sru [88280c5b]/SRU-1")
	assert.Contains(t, res.Detail, "p2/T-9", "no name: the ID alone")
	assert.Contains(t, res.Detail, "oh team release SRU-1 -p 88280c5b", "the ID in the command")
}

func TestLeaseListsNameTheProjectNextToItsID(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: claimsProject, Name: "Mon Projet", Path: "/p/x", TeamID: strp("test-team")}}}
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")

	rs := claimsLeaseResults(context.Background(), a, time.Now().Add(48*time.Hour))
	require.Len(t, rs, 1)
	assert.Contains(t, rs[0].Detail, "Mon Projet ["+claimsProject+"]/pt-a")
	assert.Contains(t, rs[0].Detail, "oh team release pt-a -p "+claimsProject, "the ID in the command")

	require.NoError(t, repo.UpdateClaimStatus(context.Background(), claimsProject, "pt-a", teamstate.ClaimStatusReview))
	rs = claimsLeaseResults(context.Background(), a, time.Now().Add(10*24*time.Hour))
	require.Len(t, rs, 2)
	assert.Contains(t, rs[1].Detail, "Mon Projet ["+claimsProject+"]/pt-a")
	assert.Contains(t, rs[1].Detail, "oh team release pt-a -p "+claimsProject)
}

func TestLeaseCheckWarnsWhenTheProjectsCannotBeListed(t *testing.T) {
	i18ntest.Set(t, "en")
	a, _ := setupBeat(t)
	a.Projects = failingProjects{}

	rs := claimsLeaseResults(context.Background(), a, time.Now())
	require.Len(t, rs, 1)
	assert.Equal(t, doctorIDClaimsExpired, rs[0].ID)
	assert.True(t, rs[0].OK && rs[0].Warn, "%+v", rs[0])
	requireUnverifiedWarning(t, rs[0].Detail, "expired claims")
	assert.NotContains(t, rs[0].Detail, "no expired claim")
}

func TestLeaseCheckWarnsWhenTheClaimsOfAProjectCannotBeRead(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	a.Projects = &mockProjectStore{projects: []domain.Project{
		{ID: claimsProject, Name: "Mon Projet", Path: "/p/x", TeamID: strp("test-team")},
		{ID: "bad/id", Name: "broken", Path: "/p/y", TeamID: strp("test-team")},
	}}
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")

	rs := claimsLeaseResults(context.Background(), a, time.Now())
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK && rs[0].Warn, "%+v", rs[0])
	assert.Contains(t, rs[0].Detail, "broken [bad/id]")
	assert.Contains(t, rs[0].Detail, "invalid project name")
	assert.NotContains(t, rs[0].Detail, "no expired claim")

	// An expired claim of the other project is still told, with the warning.
	rs = claimsLeaseResults(context.Background(), a, time.Now().Add(48*time.Hour))
	require.Len(t, rs, 1)
	assert.Contains(t, rs[0].Detail, "Mon Projet ["+claimsProject+"]/pt-a")
	assert.Contains(t, rs[0].Detail, "broken [bad/id]")
}

func TestBeadsChecksWarnWhenTheProjectsCannotBeListed(t *testing.T) {
	i18ntest.Set(t, "en")
	useApp(t, &app.App{Projects: failingProjects{}})

	res := checkBeadsSanity()
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "%+v", res[0])
	requireUnverifiedWarning(t, res[0].Detail, "side effects of Beads not checked")

	res = beadsTrackingChecks()
	require.Len(t, res, 1)
	assert.True(t, res[0].OK && res[0].Warn, "%+v", res[0])
	requireUnverifiedWarning(t, res[0].Detail, "Beads follow-up not checked")
}

type failingIdentity struct{ ti *teamsvc.TrackerIdentity }

func (f failingIdentity) TrackerIdentity(context.Context, *domain.Project) (*teamsvc.TrackerIdentity, error) {
	if f.ti != nil {
		return f.ti, nil
	}
	return nil, errUnreadable
}

type usernameTracker struct{ tracker.Tracker }

func (usernameTracker) TestConnection(context.Context) (string, error) { return "alice", nil }

func TestTrackerIdentityWarnsWhenItCannotCompareTheNames(t *testing.T) {
	i18ntest.Set(t, "en")
	probes := []trackerProbe{{label: "core", teamID: "core", creds: tracker.Config{Type: tracker.TypeGitLab}}}
	newOK := func(tracker.Config) (tracker.Tracker, error) { return usernameTracker{}, nil }

	rs := trackerIdentityResults(t.Context(), probes, failingIdentity{}, newOK)
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK && rs[0].Warn, "%+v", rs[0])
	assert.Contains(t, rs[0].Name, "core")
	requireUnverifiedWarning(t, rs[0].Detail, "not compared")

	ti := &teamsvc.TrackerIdentity{TeamID: "core", MemberNames: []string{"alice"}}
	rs = trackerIdentityResults(t.Context(), probes, failingIdentity{ti: ti}, func(tracker.Config) (tracker.Tracker, error) { return nil, errUnreadable })
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK && rs[0].Warn, "%+v", rs[0])
	requireUnverifiedWarning(t, rs[0].Detail, "cannot be created", "not compared")

	rs = trackerIdentityResults(t.Context(), probes, failingIdentity{ti: ti}, newOK)
	require.Len(t, rs, 1)
	assert.False(t, rs[0].Warn, "the name is a member's: green")
}

func TestTrackerUsernameWarnsWhenItCannotReadTheIdentity(t *testing.T) {
	i18ntest.Set(t, "en")
	probes := []trackerProbe{{label: "core", teamID: "core"}}

	rs := trackerUsernameResults(t.Context(), probes, failingIdentity{})
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK && rs[0].Warn, "%+v", rs[0])
	requireUnverifiedWarning(t, rs[0].Detail, "tracker_username not checked")

	cross := &teamsvc.TrackerIdentity{TeamID: "core", MemberID: "alice", CrossServer: true}
	rs = trackerUsernameResults(t.Context(), probes, failingIdentity{ti: cross})
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK && rs[0].Warn, "a member that cannot be found: nothing was checked")
	assert.Contains(t, rs[0].Detail, "alice")

	same := &teamsvc.TrackerIdentity{TeamID: "core", MemberID: "alice"}
	assert.Empty(t, trackerUsernameResults(t.Context(), probes, failingIdentity{ti: same}), "same server: nothing to check, nothing to say")
}
