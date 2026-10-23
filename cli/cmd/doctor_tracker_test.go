package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// fakeTracker is a GitLab that accepts one token on /api/v4/user and counts
// the requests it receives.
type fakeTracker struct {
	*httptest.Server
	hits atomic.Int32
}

func newFakeTracker(t *testing.T, accepts string) *fakeTracker {
	t.Helper()
	f := &fakeTracker{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		if r.Header.Get("PRIVATE-TOKEN") != accepts {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"username":"alice"}`))
	}))
	t.Cleanup(f.Close)
	return f
}

// doctorTrackerHub installs a hub whose team "core" has its team-state on
// teamRepo and a GitLab tracker on trackerURL; secrets is the keychain. The
// environment holds no GitLab token.
func doctorTrackerHub(t *testing.T, trackerURL, teamRepo string, secrets map[string]string) *app.App {
	t.Helper()
	configtest.Home(t)
	i18ntest.Set(t, "en")
	for _, k := range []string{"GITLAB_TOKEN", "GITLAB_URL", "OH_GITLAB_URL"} {
		t.Setenv(k, "")
	}
	statePath := filepath.Join(t.TempDir(), "state")
	_, err := teamstate.InitSolo(context.Background(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"" + trackerURL + "\"\ntracker_project = \"acme/app\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(toml), 0o644))

	a := &app.App{
		Config: &config.Config{Teams: []config.TeamConfig{
			{ID: "core", Enabled: true, StateRepo: teamRepo, StatePath: statePath, MemberID: "alice"},
		}},
		Secrets: &mockSecretStore{secrets: secrets},
	}
	prev := application
	application = a
	t.Cleanup(func() { application = prev })
	return a
}

// doctorCheckResults runs one check of the registry of oh doctor.
func doctorCheckResults(t *testing.T, id string) []doctor.Result {
	t.Helper()
	reg, err := doctor.NewRegistry(doctorRegistry()...)
	require.NoError(t, err)
	c, ok := reg.Get(id)
	require.True(t, ok, "oh doctor has a %q check", id)
	return c.Results(context.Background())
}

func requireOneResult(t *testing.T, id string) doctor.Result {
	t.Helper()
	res := doctorCheckResults(t, id)
	require.Len(t, res, 1, "%+v", res)
	return res[0]
}

func serverOf(srv *httptest.Server) string { return srv.Listener.Addr().String() }

// Bug 6 (K4b): a token declared for another server than the tracker's left
// oh doctor green; the check names the server called and the sources read.
func TestDoctorTokensFlagsATokenDeclaredForAnotherServer(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	a := doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git",
		map[string]string{config.TeamGitLabTokenKey("core"): "team-token"})
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", "https://forge.other.example")
	_ = a

	res := doctorCheckResults(t, "tracker_tokens")
	require.Len(t, res, 2, "one result for the tracker, one for the team repository: %+v", res)
	byName := map[string]doctor.Result{}
	for _, r := range res {
		byName[r.Name] = r
	}
	trk := byName["GitLab token of the tracker (core)"]
	assert.False(t, trk.OK, "GITLAB_TOKEN is declared for forge.other.example, not for the tracker")
	assert.Contains(t, trk.Detail, serverOf(tr.Server), "names the server called")
	assert.Contains(t, trk.Detail, "GITLAB_TOKEN", "names the sources read")
	assert.Contains(t, trk.Detail, "forge.other.example")
	assert.Contains(t, trk.Detail, "oh config set tracker.tracker_token_url", "tells the command")
	team := byName["GitLab token of the team repository (core)"]
	assert.True(t, team.OK, "the team token serves gitlab.octo.example: %s", team.Detail)
	assert.Zero(t, tr.hits.Load(), "the tokens check makes no request")
}

func TestDoctorTokensPassWhenEachServerHasItsToken(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git",
		map[string]string{config.TeamGitLabTokenKey("core"): "team-token"})
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)

	res := doctorCheckResults(t, "tracker_tokens")
	require.Len(t, res, 2)
	for _, r := range res {
		assert.True(t, r.OK, "%s: %s", r.Name, r.Detail)
		assert.False(t, r.Warn, r.Name)
	}
	assert.Contains(t, res[0].Detail, "GITLAB_TOKEN", "says where the token comes from")
	assert.Contains(t, res[1].Detail, "gitlab.octo.example")
}

func TestDoctorTokensFlagsAMissingTeamRepositoryToken(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)

	res := doctorCheckResults(t, "tracker_tokens")
	require.Len(t, res, 2)
	assert.True(t, res[0].OK, res[0].Detail)
	assert.False(t, res[1].OK)
	assert.Contains(t, res[1].Detail, "gitlab.octo.example")
	assert.Contains(t, res[1].Detail, config.TeamGitLabTokenKey("core"), "the key to store")
}

func TestDoctorTokensSkipsATeamRepositoryThatIsNotGitLab(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@github.com:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)

	res := doctorCheckResults(t, "tracker_tokens")
	require.Len(t, res, 1, "no GitLab token is wanted for github.com: %+v", res)
	assert.True(t, res[0].OK)
}

func TestDoctorTrackerChecksReportNothingWithoutTeamOrTracker(t *testing.T) {
	i18ntest.Set(t, "en")
	withDoctorApp(t, &config.Config{})
	assert.Empty(t, doctorCheckResults(t, "tracker_tokens"))
	assert.Empty(t, doctorCheckResults(t, "tracker_connection"))
}

// Bug 6 (K4b): a tracker that refuses the token left oh doctor green.
func TestDoctorConnectionFlagsATokenTheTrackerRefuses(t *testing.T) {
	tr := newFakeTracker(t, "the-good-one")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "expired")
	t.Setenv("OH_GITLAB_URL", tr.URL)

	r := requireOneResult(t, "tracker_connection")
	assert.False(t, r.OK, "the server answers 401")
	assert.False(t, r.Warn)
	assert.Contains(t, r.Detail, serverOf(tr.Server), "names the server")
	assert.Contains(t, r.Detail, "401")
	assert.Contains(t, r.Detail, "GITLAB_TOKEN", "names the source of the token")
	assert.Equal(t, int32(1), tr.hits.Load())
}

func TestDoctorConnectionNamesTheKeyOfARefusedToken(t *testing.T) {
	tr := newFakeTracker(t, "the-good-one")
	a := doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git",
		map[string]string{"forge.token": "expired"})
	a.Config.Tracker.TrackerTokenKey = "forge.token"

	r := requireOneResult(t, "tracker_connection")
	assert.False(t, r.OK)
	assert.Contains(t, r.Detail, "forge.token", "names the key")
}

func TestDoctorConnectionPassesAWorkingTracker(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)

	r := requireOneResult(t, "tracker_connection")
	assert.True(t, r.OK, r.Detail)
	assert.False(t, r.Warn)
	assert.Contains(t, r.Detail, "alice")
}

// A tracker that cannot be reached is a warning, not a failure: the network
// may simply be down.
func TestDoctorConnectionWarnsWhenTheTrackerIsUnreachable(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	url := tr.URL
	tr.Close()
	doctorTrackerHub(t, url, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", url)

	r := requireOneResult(t, "tracker_connection")
	assert.True(t, r.OK, "does not turn oh doctor red")
	assert.True(t, r.Warn)
	assert.Contains(t, r.Detail, serverOf(tr.Server))
}

func TestDoctorConnectionIsBoundedInTime(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	t.Cleanup(func() { close(block); srv.Close() })
	doctorTrackerHub(t, srv.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", srv.URL)
	prev := doctorConnectionTimeout
	doctorConnectionTimeout = 100 * time.Millisecond
	t.Cleanup(func() { doctorConnectionTimeout = prev })

	start := time.Now()
	r := requireOneResult(t, "tracker_connection")
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.True(t, r.OK)
	assert.True(t, r.Warn, "a tracker that does not answer is a warning")
}

func TestDoctorConnectionOfflineMakesNoRequest(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)
	t.Setenv("GITLAB_TOKEN", "glpat-ok")
	t.Setenv("OH_GITLAB_URL", tr.URL)
	t.Setenv("OH_DOCTOR_OFFLINE", "1")

	r := requireOneResult(t, "tracker_connection")
	assert.True(t, r.OK)
	assert.Contains(t, r.Detail, "offline")
	assert.Zero(t, tr.hits.Load(), "--offline: no network call")
}

// The connection is not tried when no token serves the tracker: the tokens
// check says so, once.
func TestDoctorConnectionSkipsATrackerWithoutToken(t *testing.T) {
	tr := newFakeTracker(t, "glpat-ok")
	doctorTrackerHub(t, tr.URL, "git@gitlab.octo.example:acme/core.git", nil)

	assert.Empty(t, doctorCheckResults(t, "tracker_connection"))
	assert.Zero(t, tr.hits.Load())
	assert.False(t, requireOneResultOf(t, "tracker_tokens", 2)[0].OK)
}

func requireOneResultOf(t *testing.T, id string, n int) []doctor.Result {
	t.Helper()
	res := doctorCheckResults(t, id)
	require.Len(t, res, n, "%+v", res)
	return res
}

// The Doctor view of the TUI loads the registry of oh doctor (views.DoctorChecks
// in its load()), which holds the tracker checks, in the same order as the command.
func TestDoctorViewLoadsTheRegistryWithTheTrackerChecks(t *testing.T) {
	require.NotNil(t, views.DoctorChecks)
	var ids []string
	for _, c := range doctorRegistry() {
		ids = append(ids, c.ID)
	}
	assert.Contains(t, ids, doctorIDTrackerTokens)
	assert.Contains(t, ids, doctorIDTrackerConnection)
}

// A detail that lists sources and commands is printed on indented lines.
func TestRunDoctorChecksIndentsAMultiLineDetail(t *testing.T) {
	var out bytes.Buffer
	err := runDoctorChecks(&out, []doctor.Check{{
		ID: "x", Name: "X", Run: func(context.Context) []doctor.Result {
			return []doctor.Result{{Name: "Token", Detail: "no token\nSources read:\n  - GITLAB_TOKEN"}}
		},
	}})
	require.Error(t, err)
	assert.Contains(t, out.String(), "Token — no token\n      Sources read:\n        - GITLAB_TOKEN")
}
