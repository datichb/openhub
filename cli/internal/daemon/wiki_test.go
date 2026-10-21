package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// wikiEnv is a daemon with sessions working in a project with a wiki.
func wikiEnv(t *testing.T) (*Daemon, *wiki.Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	project := filepath.Join(dir, "proj")
	page := filepath.Join(project, "docs", "wiki", "technical", "stack.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(page), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "docs", "wiki", "index.md"), []byte("---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Atlas\n\n## God nodes\n\n| Concept | Pages liées | Criticité |\n|---|---|---|\n"), 0o644))
	require.NoError(t, os.WriteFile(page, []byte("---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Stack\n\n## Librairies clés\n- Go — `CONFIRMÉ` · onboarder · 2026-09-01 · go.mod\n"), 0o644))
	sessions := sqlite.NewSessionStore(st)
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
			Mode: "manuel", State: domain.RunActive, EntryAgent: "orchestrator-dev", LaunchPath: project}))
	}
	sdir := filepath.Join(dir, "sessions")
	wk := &wiki.Service{Store: sqlite.NewWikiProposalStore(st), SessionsDir: sdir, Now: func() time.Time { return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC) }}
	w := &watcher{tracks: map[string]*sessionTrack{"ses_a": {own: "orchestrator-dev"}},
		children: map[string]string{"ses_rev": "ses_a", "ses_doc": "ses_a"}, childAgent: map[string]string{"ses_rev": "reviewer", "ses_doc": "documentarian"}}
	d := &Daemon{opts: Options{Sessions: sessions, Wiki: wk}, watchers: map[string]*watcher{"g1": w}}
	return d, wk, project, sdir
}

func wikiDo(t *testing.T, h http.Handler, method, path, body string, out any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = &bytes.Buffer{}
	}
	h.ServeHTTP(rec, httptest.NewRequest(method, apiPrefix+path, rdr))
	if out != nil {
		_ = json.Unmarshal(rec.Body.Bytes(), out)
	}
	return rec.Code
}

// LD-J: the author of a proposal is the agent oh sees; scratch paths and
// the session data are refused; wiki_apply is the documentarian's only.
func TestWikiRoutes(t *testing.T) {
	d, wk, project, sdir := wikiEnv(t)
	h := d.routes()

	var p WikiProposal
	code := wikiDo(t, h, http.MethodPost, "/wiki/propose", `{"tool_session":"ses_rev","page":"technical/stack.md","section":"Librairies clés",
		"content":"zod pour la validation","tag":"DÉDUIT","source":"package.json","agent":"documentarian"}`, &p)
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "wp-1", p.ID)
	assert.Equal(t, "reviewer", p.Agent, "the subagent oh saw, not the one claimed")
	assert.Equal(t, "ses_a", p.SessionID, "filed under the oh session")
	assert.Equal(t, "p1", p.ProjectID)

	for _, body := range []string{
		`{"tool_session":"ses_rev","page":"index.md","content":"voir ~/.oh/sessions/ses_a/scratch/debugger/repro.md","tag":"INCERTAIN"}`,
		`{"tool_session":"ses_rev","page":"index.md","content":"x","tag":"INCERTAIN","source":"` + sdir + `/ses_a/scratch/debugger/a.md"}`,
		`{"tool_session":"ses_rev","page":"../../etc/x.md","content":"x","tag":"INCERTAIN"}`,
		`{"tool_session":"ses_rev","page":"index.md","content":"x","tag":"SÛR"}`,
	} {
		assert.Equal(t, http.StatusUnprocessableEntity, wikiDo(t, h, http.MethodPost, "/wiki/propose", body, nil), body)
	}
	assert.Equal(t, http.StatusBadRequest, wikiDo(t, h, http.MethodPost, "/wiki/propose", `{"page":"index.md"}`, nil))
	assert.Equal(t, http.StatusRequestEntityTooLarge, wikiDo(t, h, http.MethodPost, "/wiki/propose",
		`{"tool_session":"ses_rev","page":"index.md","tag":"INCERTAIN","content":"`+strings.Repeat("a", wikiRequestMax)+`"}`, nil))

	var ix WikiIndex
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/index?tool_session=ses_rev", "", &ix))
	assert.Equal(t, "Atlas", ix.Title)
	assert.Len(t, ix.Pages, 2)
	assert.Equal(t, 1, ix.Pending)
	require.Len(t, ix.Session, 1)

	var page WikiPage
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/page?tool_session=ses_rev&page=technical/stack.md&section=librairies", "", &page))
	assert.Equal(t, "## Librairies clés\n- Go — `CONFIRMÉ` · onboarder · 2026-09-01 · go.mod", page.Text)
	assert.Equal(t, http.StatusNotFound, wikiDo(t, h, http.MethodGet, "/wiki/page?tool_session=ses_rev&page=none.md", "", nil))

	var pending []WikiProposal
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/pending?tool_session=ses_b&scope=session", "", &pending))
	assert.Empty(t, pending, "another session")
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/pending?tool_session=ses_b", "", &pending))
	assert.Len(t, pending, 1, "the queue of the project")

	// Only the documentarian applies, whatever the request says.
	assert.Equal(t, http.StatusForbidden, wikiDo(t, h, http.MethodPost, "/wiki/apply", `{"tool_session":"ses_rev","ids":["wp-1"],"agent":"documentarian"}`, nil))
	assert.Equal(t, http.StatusForbidden, wikiDo(t, h, http.MethodPost, "/wiki/apply", `{"tool_session":"ses_a","ids":["wp-1"]}`, nil))
	text, _ := wiki.ReadPage(project, "technical/stack.md", "")
	assert.NotContains(t, text, "zod")
	var res []wiki.Result
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodPost, "/wiki/apply", `{"tool_session":"ses_doc","ids":["wp-1"]}`, &res))
	require.Len(t, res, 1)
	assert.Empty(t, res[0].Err)
	text, _ = wiki.ReadPage(project, "technical/stack.md", "")
	assert.Contains(t, text, "- zod pour la validation\n  — `DÉDUIT` · reviewer · 2026-10-13 · package.json")
	got, err := wk.Get(context.Background(), "p1", "wp-1")
	require.NoError(t, err)
	assert.Equal(t, "documentarian", got.AppliedBy)

	var rep wiki.CheckReport
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/check?tool_session=ses_rev", "", &rep))
	assert.Equal(t, 2, rep.Pages)
	assert.Nil(t, rep.Scope)

	// LD-H: restricted to the files of a change; a bad ref is refused.
	rep = wiki.CheckReport{}
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/check?tool_session=ses_rev&path=package.json&path=src/a.go", "", &rep))
	require.NotNil(t, rep.Scope)
	assert.Equal(t, []string{"package.json", "src/a.go"}, rep.Scope.Changed)
	assert.Contains(t, rep.Scope.Pages, "technical/stack.md", "its tag cites package.json")
	assert.Equal(t, http.StatusUnprocessableEntity, wikiDo(t, h, http.MethodGet, "/wiki/check?tool_session=ses_rev&base=-x", "", nil))
	// Without the wiki service: not served.
	d.opts.Wiki = nil
	assert.Equal(t, http.StatusServiceUnavailable, wikiDo(t, h, http.MethodGet, "/wiki/index?tool_session=ses_rev", "", nil))
}

// p5-fixes: a replacement is checked against the page of the session
// location when it is filed: a line not in the page is refused.
func TestWikiProposeReplacement(t *testing.T) {
	d, _, _, _ := wikiEnv(t)
	h := d.routes()
	var p WikiProposal
	require.Equal(t, http.StatusCreated, wikiDo(t, h, http.MethodPost, "/wiki/propose", `{"tool_session":"ses_rev","page":"technical/stack.md",
		"content":"- Go 1.23","tag":"CONFIRMÉ","source":"go.mod","replaces":"- Go — `+"`CONFIRMÉ`"+` · onboarder · 2026-09-01 · go.mod"}`, &p))
	assert.Equal(t, "- Go — `CONFIRMÉ` · onboarder · 2026-09-01 · go.mod", p.Replaces)
	for _, body := range []string{
		`{"tool_session":"ses_rev","page":"technical/stack.md","content":"- x","tag":"DÉDUIT","replaces":"- absente"}`,
		`{"tool_session":"ses_rev","page":"technical/stack.md","content":"- x","tag":"DÉDUIT","replaces":"## Librairies clés"}`,
	} {
		assert.Equal(t, http.StatusUnprocessableEntity, wikiDo(t, h, http.MethodPost, "/wiki/propose", body, nil), body)
	}
}

// The wiki routes are workflow routes: the group token of the session.
func TestWikiRoutesNeedTheGroupToken(t *testing.T) {
	p := shortPaths(t)
	e := newReportEnv(t, p.Dir)
	ctx := context.Background()
	wk := &wiki.Service{Store: &memWiki{}}
	runDaemon(t, Options{Paths: p, Version: "t", Sessions: e.sessions, Checkpoints: e.cp, Wiki: wk,
		Capability: "cap", Tick: time.Hour, IdleAfter: time.Hour})
	_, err := NewClient(p).WikiPropose(ctx, WikiProposeRequest{ToolSession: "ses_a", Page: "index.md", Content: "x", Tag: "INCERTAIN"})
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))
	_, err = NewClient(p).WikiIndex(ctx, "ses_a")
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))
	_, err = NewClient(p).WikiApply(ctx, "ses_a", []string{"wp-1"})
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))
}

// memWiki is an empty proposal store.
type memWiki struct{}

func (memWiki) AddWikiProposal(context.Context, *domain.WikiProposal, int, int) error { return nil }
func (memWiki) ListWikiProposals(context.Context, domain.WikiProposalFilter) ([]domain.WikiProposal, error) {
	return nil, nil
}
func (memWiki) GetWikiProposal(context.Context, string, int) (*domain.WikiProposal, error) {
	return nil, domain.ErrNotFound
}
func (memWiki) UpdateWikiProposal(context.Context, *domain.WikiProposal) error { return nil }

// p3-remote-wiki: the daemon of a remote job serves the wiki of the clone
// (read, check, propose) but refuses wiki_apply to every agent, the
// documentarian included: the proposals go back to the machine.
func TestWikiRoutesInRemoteJob(t *testing.T) {
	d, wk, project, _ := wikiEnv(t)
	d.opts.WikiNoApply = true
	h := d.routes()

	var ix WikiIndex
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/index?tool_session=ses_rev", "", &ix))
	assert.Equal(t, "Atlas", ix.Title)
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/page?tool_session=ses_rev&page=technical/stack.md", "", nil))
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/check?tool_session=ses_rev", "", nil))
	var p WikiProposal
	require.Equal(t, http.StatusCreated, wikiDo(t, h, http.MethodPost, "/wiki/propose",
		`{"tool_session":"ses_rev","page":"technical/stack.md","section":"Librairies clés","content":"zod pour la validation","tag":"DÉDUIT","source":"package.json"}`, &p))

	var e struct {
		Error string `json:"error"`
	}
	assert.Equal(t, http.StatusForbidden, wikiDo(t, h, http.MethodPost, "/wiki/apply", `{"tool_session":"ses_doc","ids":["wp-1"]}`, &e))
	assert.Contains(t, e.Error, "wiki_apply")
	text, _ := wiki.ReadPage(project, "technical/stack.md", "")
	assert.NotContains(t, text, "zod", "nothing written in the clone")
	got, err := wk.Get(context.Background(), "p1", p.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WikiPending, got.Status)
}
