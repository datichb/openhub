package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

type noSessions struct{ domain.SessionStore }

func (noSessions) List(context.Context, string) ([]domain.Session, error) { return nil, nil }

// rehomeFixture is the case of bug 7: the team repo holds 5 claims of the
// member under t-sru-b267fbf1, the project was created again as 88280c5b.
type rehomeFixture struct {
	repo    *teamstate.Repo
	bare    string
	app     *app.App
	project domain.Project
}

func newRehomeFixture(t *testing.T, name, origin string) *rehomeFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	i18ntest.Set(t, "en")
	bare, clone := claimRepos(t)
	repo := teamstate.NewRepo(bare, clone)
	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3", "SRU-4", "SRU-5"} {
		_, err := repo.CreateClaim(context.Background(), teamstate.Claim{Project: "t-sru-b267fbf1", TicketID: id, ClaimedBy: "testuser", Status: teamstate.ClaimStatusReview})
		require.NoError(t, err)
	}
	dir := gittest.Repo(t)
	if origin != "" {
		gittest.Run(t, dir, "remote", "add", "origin", origin)
	}
	teamID := "test-team"
	project := domain.Project{ID: "88280c5b", Name: name, Path: dir, Status: domain.ProjectStatusActive, TeamID: &teamID}
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: teamID, Enabled: true, StateRepo: bare, StatePath: clone, MemberID: "testuser"}}}
	io, _, _ := testIO()
	a := &app.App{Config: cfg, Projects: &mockProjectStore{projects: []domain.Project{project}}, Sessions: noSessions{}, IO: io}
	prev := application
	application = a
	t.Cleanup(func() { application = prev })
	return &rehomeFixture{repo: repo, bare: bare, app: a, project: project}
}

// Bug 7 (K4a): 5 claims under the ID the project had before it was created
// again were not reported: the check of the claims said « no orphan claim ».
func TestClaimsCheckFlagsTheClaimsOfAFormerProjectID(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.False(t, res[0].OK, res[0].Detail)
	assert.Equal(t, doctorIDClaimsProject, res[0].ID)
	assert.Contains(t, res[0].Detail, "t-sru-b267fbf1")
	assert.Contains(t, res[0].Detail, "88280c5b")
	assert.Contains(t, res[0].Detail, "5")
	assert.Contains(t, res[0].Detail, "oh doctor --fix")
	assert.NotContains(t, res[0].Detail, "no orphan claim")
	_ = f
}

func TestClaimsCheckIgnoresAnIDWithoutALocalProject(t *testing.T) {
	newRehomeFixture(t, "something-else", "")

	res := claimsDoctorChecks(context.Background())
	require.Len(t, res, 1)
	assert.True(t, res[0].OK, "the claims of a project this machine does not know are the business of another machine: %s", res[0].Detail)
}

func TestDoctorFixRehomesTheClaimsOnceAndAsksOnce(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	ctx := context.Background()
	before := commitsOf(t, f.bare)

	asked := 0
	var out bytes.Buffer
	err := repairClaimsProject(ctx, &out, func(string) (bool, error) { asked++; return true, nil })
	require.NoError(t, err)
	assert.Equal(t, 1, asked, "one confirmation for all the claims")
	assert.Contains(t, out.String(), "t-sru-b267fbf1")

	claims, err := f.repo.ListClaims("88280c5b")
	require.NoError(t, err)
	assert.Len(t, claims, 5)
	assert.Equal(t, before+1, commitsOf(t, f.bare), "one commit for the five claims")

	res := claimsDoctorChecks(ctx)
	require.Len(t, res, 1)
	assert.True(t, res[0].OK, res[0].Detail)

	asked = 0
	out.Reset()
	require.NoError(t, repairClaimsProject(ctx, &out, func(string) (bool, error) { asked++; return true, nil }))
	assert.Zero(t, asked, "nothing left to attach: no question")
	assert.Equal(t, before+1, commitsOf(t, f.bare), "idempotent: no second commit")
}

func TestDoctorFixDeclinedChangesNothing(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	var out bytes.Buffer
	require.NoError(t, repairClaimsProject(context.Background(), &out, func(string) (bool, error) { return false, nil }))
	left, err := f.repo.ListClaims("t-sru-b267fbf1")
	require.NoError(t, err)
	assert.Len(t, left, 5)
}

func TestRuntimeFixOffersTheRehome(t *testing.T) {
	fix := runtimeFix(doctor.Result{ID: doctorIDClaimsProject, OK: false})
	require.NotNil(t, fix)
	assert.NotNil(t, fix.Plan)
	assert.NotNil(t, fix.Apply)
	assert.Nil(t, runtimeFix(doctor.Result{ID: doctorIDClaimsProject, OK: true}))
	assert.Nil(t, runtimeFix(doctor.Result{ID: doctorIDClaimsUnknown, OK: false}), "a conflict has no automatic repair")
}

func TestMRRepository(t *testing.T) {
	for raw, want := range map[string]config.GitRemote{
		"https://gitlab.example.com/group/sub/proj/-/merge_requests/12":     {Host: "gitlab.example.com", Path: "group/sub/proj"},
		"https://gitlab.example.com:8443/group/proj/-/merge_requests/12#n1": {Host: "gitlab.example.com:8443", Path: "group/proj"},
		"https://GitHub.com/acme/api/pull/7":                                {Host: "github.com", Path: "acme/api"},
		"https://gitlab.example.com/group/proj/merge_requests/3":            {Host: "gitlab.example.com", Path: "group/proj"},
		"https://gitlab.example.com/group/proj/-/issues/3":                  {Host: "gitlab.example.com", Path: "group/proj"},
	} {
		got, ok := mrRepository(raw)
		assert.True(t, ok, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"", "not a url", "https://gitlab.example.com/-/merge_requests/1"} {
		_, ok := mrRepository(raw)
		assert.False(t, ok, raw)
	}
}

func TestSameRepository(t *testing.T) {
	a := config.GitRemote{Host: "gitlab.example.com:8443", Path: "Group/Proj"}
	assert.True(t, sameRepository(a, config.GitRemote{Host: "gitlab.example.com", Path: "group/proj.git"}), "port, case and .git do not count")
	assert.False(t, sameRepository(a, config.GitRemote{Host: "gitlab.example.com", Path: "group/other"}))
	assert.False(t, sameRepository(a, config.GitRemote{Host: "github.com", Path: "group/proj"}))
}

func claimsOfID(project string, member string, tickets ...string) []teamstate.Claim {
	var out []teamstate.Claim
	for _, tk := range tickets {
		out = append(out, teamstate.Claim{Project: project, TicketID: tk, ClaimedBy: member})
	}
	return out
}

func originsByDir(m map[string]config.GitRemote) remoteOfDir {
	return func(dir string) (config.GitRemote, bool) { r, ok := m[dir]; return r, ok }
}

// The rule that decides which project the claims of an unknown ID go to.
func TestMatchClaimsRehomes(t *testing.T) {
	noClaims := func(string) []teamstate.Claim { return nil }
	sru := domain.Project{ID: "88280c5b", Name: "T-SRU", Path: "/p/sru"}
	other := domain.Project{ID: "other-00000001", Name: "other", Path: "/p/other"}

	t.Run("same name", func(t *testing.T) {
		found, amb := matchClaimsRehomes(nil, "me", claimsOfID("t-sru-b267fbf1", "me", "SRU-1", "SRU-2"), map[string]bool{"88280c5b": true},
			[]domain.Project{sru, other}, noClaims, originsByDir(nil))
		require.Len(t, found, 1)
		assert.Empty(t, amb)
		assert.Equal(t, "t-sru-b267fbf1", found[0].From)
		assert.Equal(t, "88280c5b", found[0].To.ID)
		assert.Equal(t, claimEvidenceName, found[0].Evidence)
		assert.Equal(t, []string{"SRU-1", "SRU-2"}, found[0].Movable)
	})

	t.Run("another name, a bare uuid8, a known ID, another member: nothing", func(t *testing.T) {
		known := map[string]bool{"88280c5b": true, "t-sru-aaaaaaaa": true}
		claims := append(claimsOfID("api-b267fbf1", "me", "A-1"), claimsOfID("c0ffee00", "me", "B-1")...)
		claims = append(claims, claimsOfID("t-sru-aaaaaaaa", "me", "C-1")...)
		claims = append(claims, claimsOfID("t-sru-bbbbbbbb", "bob", "D-1")...)
		found, amb := matchClaimsRehomes(nil, "me", claims, known, []domain.Project{sru, other}, noClaims, originsByDir(nil))
		assert.Empty(t, found)
		assert.Empty(t, amb)
	})

	t.Run("merge request of the same repository", func(t *testing.T) {
		claims := claimsOfID("renamed-b267fbf1", "me", "SRU-1")
		claims[0].MRURL = "https://gitlab.example.com/acme/sru/-/merge_requests/4"
		origins := originsByDir(map[string]config.GitRemote{"/p/sru": {Host: "gitlab.example.com", Path: "acme/sru"}})
		found, _ := matchClaimsRehomes(nil, "me", claims, nil, []domain.Project{sru, other}, noClaims, origins)
		require.Len(t, found, 1, "the name does not matter when the repository is the same")
		assert.Equal(t, claimEvidenceRemote, found[0].Evidence)
		assert.Equal(t, "88280c5b", found[0].To.ID)
	})

	t.Run("merge request of another repository refuses the name", func(t *testing.T) {
		claims := claimsOfID("t-sru-b267fbf1", "me", "SRU-1")
		claims[0].MRURL = "https://gitlab.example.com/acme/elsewhere/-/merge_requests/4"
		origins := originsByDir(map[string]config.GitRemote{"/p/sru": {Host: "gitlab.example.com", Path: "acme/sru"}})
		found, amb := matchClaimsRehomes(nil, "me", claims, nil, []domain.Project{sru}, noClaims, origins)
		assert.Empty(t, found)
		assert.Empty(t, amb)
	})

	t.Run("a merge request without a readable origin falls back to the name", func(t *testing.T) {
		claims := claimsOfID("t-sru-b267fbf1", "me", "SRU-1")
		claims[0].MRURL = "https://gitlab.example.com/acme/sru/-/merge_requests/4"
		found, _ := matchClaimsRehomes(nil, "me", claims, nil, []domain.Project{sru}, noClaims, originsByDir(nil))
		require.Len(t, found, 1)
		assert.Equal(t, claimEvidenceName, found[0].Evidence)
	})

	t.Run("two projects of the same name are ambiguous", func(t *testing.T) {
		twin := domain.Project{ID: "twin-00000001", Name: "t-sru", Path: "/p/twin"}
		found, amb := matchClaimsRehomes(nil, "me", claimsOfID("t-sru-b267fbf1", "me", "SRU-1"), nil, []domain.Project{sru, twin}, noClaims, originsByDir(nil))
		assert.Empty(t, found)
		require.Len(t, amb, 1)
		assert.Equal(t, []string{"T-SRU", "t-sru"}, amb[0].Candidates)
	})

	t.Run("the repository breaks the tie", func(t *testing.T) {
		twin := domain.Project{ID: "twin-00000001", Name: "t-sru", Path: "/p/twin"}
		claims := claimsOfID("t-sru-b267fbf1", "me", "SRU-1")
		claims[0].MRURL = "https://gitlab.example.com/acme/sru/-/merge_requests/4"
		origins := originsByDir(map[string]config.GitRemote{
			"/p/twin": {Host: "gitlab.example.com", Path: "acme/sru"},
			"/p/sru":  {Host: "gitlab.example.com", Path: "acme/sru-fork"},
		})
		found, amb := matchClaimsRehomes(nil, "me", claims, nil, []domain.Project{sru, twin}, noClaims, origins)
		assert.Empty(t, amb)
		require.Len(t, found, 1)
		assert.Equal(t, "twin-00000001", found[0].To.ID)
	})

	t.Run("a ticket already claimed in the project stays a conflict", func(t *testing.T) {
		claimsOf := func(project string) []teamstate.Claim {
			if project == "88280c5b" {
				return claimsOfID(project, "me", "SRU-2")
			}
			return nil
		}
		found, _ := matchClaimsRehomes(nil, "me", claimsOfID("t-sru-b267fbf1", "me", "SRU-1", "SRU-2"), nil, []domain.Project{sru}, claimsOf, originsByDir(nil))
		require.Len(t, found, 1)
		assert.Equal(t, []string{"SRU-1"}, found[0].Movable)
		assert.Equal(t, []string{"SRU-2"}, found[0].Conflicts)
	})
}

func TestClaimsReportIsNeverGreenWithClaimsUnderAnUnknownID(t *testing.T) {
	i18ntest.Set(t, "en")
	assert.True(t, claimsReport(nil, nil, nil).OK)

	only := []claimsRehome{{From: "old-12345678", To: domain.Project{ID: "new", Name: "new"}, Evidence: claimEvidenceName, Conflicts: []string{"SRU-1"}}}
	res := claimsReport(nil, only, nil)
	assert.False(t, res.OK)
	assert.Equal(t, doctorIDClaimsUnknown, res.ID, "nothing to attach: no repair")
	assert.Contains(t, res.Detail, "oh team release SRU-1 -p old-12345678")

	res = claimsReport([]orphanClaim{{Project: "p", Ticket: "T-1", Reason: "closed"}},
		[]claimsRehome{{From: "old-12345678", To: domain.Project{ID: "new", Name: "new"}, Evidence: claimEvidenceName, Movable: []string{"SRU-1"}}}, nil)
	assert.False(t, res.OK)
	assert.Equal(t, doctorIDClaimsProject, res.ID)
	assert.Contains(t, res.Detail, "oh team release T-1 -p p", "the orphan claims are still told")
	assert.Contains(t, res.Detail, "oh doctor --fix")

	res = claimsReport(nil, nil, []claimsAmbiguity{{From: "old-12345678", Candidates: []string{"a", "b"}, Tickets: []string{"SRU-1"}}})
	assert.False(t, res.OK)
	assert.Equal(t, doctorIDClaimsUnknown, res.ID)
}

// oh doctor --fix --yes attaches the claims without a question; without
// --yes and without a terminal nothing is attached and the command refuses.
func TestDoctorFixCommandUsesTheCommonConfirmation(t *testing.T) {
	f := newRehomeFixture(t, "t-sru", "")
	cmd := &cobra.Command{Use: "doctor"}
	addYesFlag(cmd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())

	prev := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = prev })
	err := fixClaimsProject(cmd)
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	left, _ := f.repo.ListClaims("t-sru-b267fbf1")
	assert.Len(t, left, 5, "refused: nothing moved")

	require.NoError(t, cmd.Flags().Set("yes", "true"))
	require.NoError(t, fixClaimsProject(cmd))
	moved, _ := f.repo.ListClaims("88280c5b")
	assert.Len(t, moved, 5)
}
