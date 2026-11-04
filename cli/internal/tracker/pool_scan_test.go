package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// fakeGitLab answers the issues API the way GitLab does: the labels filter is
// an AND (labels=a,b needs both), updated_after and assignee_username filter,
// the newest updated come first, and the lists are paginated.
type fakeGitLab struct {
	mu       sync.Mutex
	issues   []glFake
	requests []string // raw query of each list request
}

type glFake struct {
	iid       int
	title     string
	labels    []string
	assignee  string
	updatedAt time.Time
}

var issueByIID = regexp.MustCompile(`/issues/(\d+)$`)

func (f *fakeGitLab) toJSON(i glFake) map[string]any {
	as := []map[string]any{}
	if i.assignee != "" {
		as = append(as, map[string]any{"username": i.assignee})
	}
	return map[string]any{"iid": i.iid, "state": "opened", "title": i.title, "labels": i.labels,
		"updated_at": i.updatedAt.UTC().Format(time.RFC3339), "assignees": as}
}

func (f *fakeGitLab) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if m := issueByIID.FindStringSubmatch(r.URL.Path); m != nil {
		iid, _ := strconv.Atoi(m[1])
		for _, i := range f.issues {
			if i.iid == iid {
				_ = json.NewEncoder(w).Encode(f.toJSON(i))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	f.requests = append(f.requests, r.URL.RawQuery)
	var out []glFake
	for _, i := range f.issues {
		if v := q.Get("assignee_username"); v != "" && i.assignee != v {
			continue
		}
		if q.Get("assignee_id") == "0" && i.assignee != "" {
			continue
		}
		if v := q.Get("updated_after"); v != "" {
			after, _ := time.Parse(time.RFC3339, v)
			if !i.updatedAt.After(after) {
				continue
			}
		}
		if v := q.Get("labels"); v != "" {
			ok := true
			for _, want := range strings.Split(v, ",") {
				found := false
				for _, l := range i.labels {
					found = found || l == want
				}
				ok = ok && found
			}
			if !ok {
				continue
			}
		}
		out = append(out, i)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].updatedAt.After(out[b].updatedAt) })
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage <= 0 {
		perPage = 20
	}
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)
	start := min((page-1)*perPage, len(out))
	end := min(start+perPage, len(out))
	if end < len(out) {
		w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
	}
	body := make([]map[string]any, 0, end-start)
	for _, i := range out[start:end] {
		body = append(body, f.toJSON(i))
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (f *fakeGitLab) labelRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, raw := range f.requests {
		if strings.Contains(raw, "labels=") {
			out = append(out, raw)
		}
	}
	return out
}

func (f *fakeGitLab) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = nil
}

func (f *fakeGitLab) add(i glFake) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issues = append(f.issues, i)
}

// poolFixture is a team whose config.toml is the anonymised real one
// (teamstate/testdata/gitlab_team_todo.toml: four labels mapped to 'todo',
// four unassigned_labels, an empty [board]) on a fake GitLab.
type poolFixture struct {
	repo     *teamstate.Repo
	gl       *fakeGitLab
	cfg      teamstate.TrackerConfig
	stateDir string
}

func newPoolFixture(t *testing.T, issues ...glFake) *poolFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	i18ntest.Set(t, "en")
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: "alice", DisplayName: "Alice", GitLabUsername: "alice"}))
	data, err := os.ReadFile(filepath.Join("..", "teamstate", "testdata", "gitlab_team_todo.toml"))
	require.NoError(t, err)
	gittest.Commit(t, clone, "config.toml", string(data), "config")
	gittest.Run(t, clone, "push", "-q")

	gl := &fakeGitLab{issues: issues}
	srv := httptest.NewServer(gl)
	t.Cleanup(srv.Close)

	loaded, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	cfg := loaded.Tracker
	cfg.TrackerURL = srv.URL
	cfg.Projects = map[string]string{"p": "grp/p"} //nolint:staticcheck // the engine still reads Projects
	return &poolFixture{repo: repo, gl: gl, cfg: cfg, stateDir: t.TempDir()}
}

func (f *poolFixture) run(t *testing.T) *SyncResult {
	t.Helper()
	srvURL := f.cfg.TrackerURL
	tr := newGitLab(Config{Type: TypeGitLab, BaseURL: srvURL, Token: "tok"})
	res, err := NewEngine(tr, f.repo, f.cfg, f.stateDir).Run(t.Context())
	require.NoError(t, err)
	return res
}

func (f *poolFixture) claims(t *testing.T) map[string]teamstate.Claim {
	t.Helper()
	list, err := f.repo.ListClaims(t.Context(), "p")
	require.NoError(t, err)
	out := map[string]teamstate.Claim{}
	for _, c := range list {
		out[c.TicketID] = c
	}
	return out
}

// seedSyncState writes a state of a project already synced an hour ago, which
// is what the runs of every member of the team had before the first complete
// scan of the pool existed.
func (f *poolFixture) seedSyncState(t *testing.T) {
	t.Helper()
	state := &SyncState{LastSyncAt: map[string]time.Time{}}
	state.SetLastSync(TypeGitLab, "grp/p", time.Now().Add(-time.Hour))
	state.MarkAutoPlanScanned(TypeGitLab, "grp/p", time.Now().Add(-time.Hour))
	require.NoError(t, state.Save(f.stateDir))
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d) }

// The real case, anonymised: tickets to do carry one of the four labels of the
// pool, never all four; they are old and untouched; the config maps them to
// 'todo'. They come onto the board, once each, in the todo column (planned),
// not in progress.
func TestPoolOfAnAnonymisedRealConfig(t *testing.T) {
	day := 24 * time.Hour
	f := newPoolFixture(t,
		glFake{iid: 101, title: "ready", labels: []string{"Ready to dev"}, updatedAt: ago(90 * day)},
		glFake{iid: 102, title: "todo", labels: []string{"Todo"}, updatedAt: ago(60 * day)},
		glFake{iid: 103, title: "two pool labels", labels: []string{"Design to do", "US to do"}, updatedAt: ago(40 * day)},
		glFake{iid: 104, title: "blocked and ready", labels: []string{"Blocked", "Ready to dev"}, updatedAt: ago(30 * day)},
		glFake{iid: 105, title: "not in the pool", labels: []string{"Dev doing"}, updatedAt: ago(20 * day)},
		glFake{iid: 106, title: "mine, in validation", labels: []string{"Testing"}, assignee: "alice", updatedAt: ago(10 * day)},
	)
	f.seedSyncState(t)
	// The project was synced before: it already has a claim (ticket 106 of
	// alice), so the run is not "the first sync" of the project.
	_, err := f.repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "106", Project: "p", ClaimedBy: "alice",
		Status: teamstate.ClaimStatusInProgress, ExternalIID: 106})
	require.NoError(t, err)

	res := f.run(t)
	got := f.claims(t)
	for _, iid := range []string{"101", "102", "103", "104"} {
		require.Contains(t, got, iid, "ticket %s of the pool", iid)
		assert.Equal(t, teamstate.ClaimStatusPlanned, got[iid].Status, "%s is to do, not in progress", iid)
		assert.Empty(t, got[iid].ClaimedBy, "%s is in the pool", iid)
	}
	assert.NotContains(t, got, "105", "no label of the pool")
	assert.Equal(t, teamstate.ClaimStatusValidation, got["106"].Status, "an existing claim follows its labels")
	assert.Equal(t, "alice", got["106"].ClaimedBy)
	assert.Equal(t, 4, res.ClaimsCreated)
	assert.Empty(t, res.Warnings, "the 'todo' values were read as planned: nothing left to say")

	// One request per label (an OR), never the four joined (an AND).
	var asked []string
	for _, raw := range f.gl.labelRequests() {
		assert.NotContains(t, raw, "%2C", "labels are never joined")
		assert.NotContains(t, raw, "updated_after", "the first complete scan does not filter on the update time")
		asked = append(asked, raw)
	}
	assert.Len(t, asked, 4)

	// The scan is made once: the next run only fetches what was updated.
	f.gl.forget()
	f.gl.add(glFake{iid: 107, title: "new", labels: []string{"US to do"}, updatedAt: time.Now()})
	res = f.run(t)
	assert.Equal(t, 1, res.ClaimsCreated)
	assert.Contains(t, f.claims(t), "107")
	for _, raw := range f.gl.labelRequests() {
		assert.Contains(t, raw, "updated_after", "incremental from then on")
	}
	f.gl.forget()
	assert.Zero(t, f.run(t).ClaimsCreated)
}

// Changing unassigned_labels scans the pool in full again: the issues of the
// new label were never looked at.
func TestPoolScansAgainWhenTheLabelsChange(t *testing.T) {
	day := 24 * time.Hour
	f := newPoolFixture(t,
		glFake{iid: 101, labels: []string{"Todo"}, updatedAt: ago(90 * day)},
		glFake{iid: 108, labels: []string{"Extra"}, updatedAt: ago(80 * day)},
	)
	f.run(t)
	assert.Contains(t, f.claims(t), "101")
	assert.NotContains(t, f.claims(t), "108")

	f.cfg.UnassignedLabels = append(f.cfg.UnassignedLabels, "Extra")
	f.run(t)
	assert.Contains(t, f.claims(t), "108", "found by the scan the new label triggered")

	data, err := os.ReadFile(filepath.Join(f.stateDir, "sync-state.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "autopool_scanned")
}

// A scan that failed is made again by the next run.
func TestPoolScanIsNotRecordedWhenTheFetchFails(t *testing.T) {
	f := newPoolFixture(t, glFake{iid: 101, labels: []string{"Todo"}, updatedAt: ago(90 * 24 * time.Hour)})
	f.seedSyncState(t)
	f.cfg.TrackerURL = "http://127.0.0.1:1" // nobody there

	tr := newGitLab(Config{Type: TypeGitLab, BaseURL: f.cfg.TrackerURL, Token: "tok"})
	_, _ = NewEngine(tr, f.repo, f.cfg, f.stateDir).Run(t.Context())
	state, err := LoadSyncState(f.stateDir)
	require.NoError(t, err)
	assert.False(t, state.AutoPoolScan(TypeGitLab, "grp/p", f.cfg.UnassignedLabels))
}

// The sync state of a version without the full scan has no record of it.
func TestSyncStateOfAnOlderVersionHasNoPoolScan(t *testing.T) {
	dir := t.TempDir()
	old := `{"last_sync_at":{"gitlab/grp/p":"2026-10-01T00:00:00Z"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), []byte(old), 0o600))
	state, err := LoadSyncState(dir)
	require.NoError(t, err)
	assert.False(t, state.AutoPoolScan(TypeGitLab, "grp/p", []string{"Todo"}))
	state.MarkAutoPoolScanned(TypeGitLab, "grp/p", []string{"Todo", "Ready"}, time.Now())
	assert.True(t, state.AutoPoolScan(TypeGitLab, "grp/p", []string{"ready", " TODO"}), "order and case do not matter")
	assert.False(t, state.AutoPoolScan(TypeGitLab, "grp/p", []string{"Todo"}))
}

// A value that stands for nothing is said by the sync, which ignores it.
func TestSyncWarnsOfAMappingThatStandsForNothing(t *testing.T) {
	f := newPoolFixture(t, glFake{iid: 101, labels: []string{"Odd", "Todo"}, updatedAt: ago(time.Hour)})
	f.cfg.LabelStatusMapping = map[string]string{"Odd": "wip", "Todo": "planned"}

	res := f.run(t)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0].Message, `"Odd" = 'wip'`)
	assert.Equal(t, teamstate.ClaimStatusPlanned, f.claims(t)["101"].Status, "the valid label still places the issue")
}
