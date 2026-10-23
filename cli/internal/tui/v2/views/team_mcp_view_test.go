package views

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

func mountTeamMCP(t *testing.T, cfg TeamMCPViewConfig) (*TeamMCPView, *viewShell, *tview.Flex, *tview.Application) {
	t.Helper()
	_, tc := soloState(t)
	cfg.ResolveTeam = func() TeamResolution { return tc }
	if cfg.GetMCPConfig == nil {
		cfg.GetMCPConfig = func() config.MCPConfig { return config.MCPConfig{} }
	}
	v := NewTeamMCPView(cfg)
	sh := &viewShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })
	return v, sh, content, app
}

func pressTeamMCPList(v *TeamMCPView, r rune) {
	v.list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
}

// Bug 9: the title of the first section ("MCP Gitlab") is drawn when the view
// opens and when the cursor comes back to the first field after a scroll
// (tview then scrolled the title out of the list).
func TestTeamMCPViewShowsTheFirstSectionTitle(t *testing.T) {
	frLocale(t)
	v, _, content, app := mountTeamMCP(t, TeamMCPViewConfig{})
	const title = "── MCP Gitlab ──"
	var screen string
	onLoop(app, func() { screen = screenText(t, content, 140, 40) })
	assert.Contains(t, screen, title, "at opening, screen:\n%s", screen)

	onLoop(app, func() {
		screenText(t, content, 140, 40)
		pressTeamMCPList(v, 'G')
		screenText(t, content, 140, 40)
		pressTeamMCPList(v, 'g')
		screen = screenText(t, content, 140, 40)
	})
	assert.Contains(t, screen, title, "back on the first field, screen:\n%s", screen)
	golden.Assert(t, "team_mcp_view_back_on_top", screen)
}

// selectTeamMCPService puts the cursor on the first field of the section of svc.
func selectTeamMCPService(t *testing.T, v *TeamMCPView, svc string) {
	t.Helper()
	for i, f := range v.fields {
		if f.Section == svc && f.Key != "" {
			v.list.SelectIndex(i)
			return
		}
	}
	t.Fatalf("no field for %s", svc)
}

// fakeTest records the services t was pressed on and what each run does.
type fakeTest struct {
	mu       sync.Mutex
	prepared []string
	runs     int
	run      func(ctx context.Context) (string, error)
}

func (f *fakeTest) prepare(svc string) func(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prepared = append(f.prepared, svc)
	if f.run == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		f.mu.Lock()
		f.runs++
		f.mu.Unlock()
		return f.run(ctx)
	}
}

func (f *fakeTest) counts() ([]string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prepared...), f.runs
}

func pressT(t *testing.T, app *tview.Application, v *TeamMCPView, svc string) {
	t.Helper()
	onLoop(app, func() {
		selectTeamMCPService(t, v, svc)
		v.HandleKey(runeKey('t'))
	})
}

func lastToast(sh *viewShell) string {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if len(sh.toasts) == 0 {
		return ""
	}
	return sh.toasts[len(sh.toasts)-1]
}

// Bug 8: t calls the server of the selected service (it used to check that a
// token was present and say "testing" without any call) and shows the result.
func TestTeamMCPViewTestCallsTheServer(t *testing.T) {
	frLocale(t)
	ft := &fakeTest{run: func(context.Context) (string, error) { return "connecté en tant que alice", nil }}
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{TestMCP: ft.prepare})

	pressT(t, app, v, "jira")

	require.Eventually(t, func() bool { return len(lastToast(sh)) > 0 && lastToast(sh) != "Test connexion jira..." }, 3*time.Second, 5*time.Millisecond)
	prepared, runs := ft.counts()
	assert.Equal(t, []string{"jira"}, prepared)
	assert.Equal(t, 1, runs, "one call to the server")
	assert.Contains(t, lastToast(sh), "jira")
	assert.Contains(t, lastToast(sh), "connecté en tant que alice")
}

// The error says everything the server side knows (server, source, key): it is
// shown whole, in a modal, not cut in a toast.
func TestTeamMCPViewTestShowsThePreciseError(t *testing.T) {
	frLocale(t)
	msg := "le serveur gitlab.example.com a refusé le jeton (HTTP 401) — source : jeton MCP (clé my.key).\nRemplace-le : oh secrets set my.key"
	ft := &fakeTest{run: func(context.Context) (string, error) { return "", errors.New(msg) }}
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{TestMCP: ft.prepare})

	pressT(t, app, v, "gitlab")

	waitFor(t, app, func() bool { return sh.modalBody != "" }) // written on the loop
	assert.Contains(t, sh.modalTitle, "gitlab")
	assert.Contains(t, sh.modalBody, "gitlab.example.com")
	assert.Contains(t, sh.modalBody, "my.key")
	assert.Contains(t, sh.modalBody, "oh secrets set my.key")
}

// The call leaves the event loop: while it waits for the server the view
// answers, and the result is applied once the server has answered.
func TestTeamMCPViewTestDoesNotBlockTheLoop(t *testing.T) {
	frLocale(t)
	release := make(chan struct{})
	ft := &fakeTest{run: func(context.Context) (string, error) { <-release; return "ok", nil }}
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{TestMCP: ft.prepare})

	pressT(t, app, v, "gitlab")
	onLoop(app, func() {}) // the loop is free although the call is pending
	assert.Equal(t, "Test connexion gitlab...", lastToast(sh))

	close(release)
	require.Eventually(t, func() bool { return lastToast(sh) != "Test connexion gitlab..." }, 3*time.Second, 5*time.Millisecond)
	assert.Contains(t, lastToast(sh), "ok")
}

// A service without a connection test says so; unsaved changes are not tested
// (the test reads the saved configuration).
func TestTeamMCPViewTestUnavailableAndUnsaved(t *testing.T) {
	frLocale(t)
	ft := &fakeTest{}
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{TestMCP: ft.prepare})

	pressT(t, app, v, "figma")
	prepared, runs := ft.counts()
	assert.Equal(t, []string{"figma"}, prepared)
	assert.Zero(t, runs)
	assert.Contains(t, lastToast(sh), "figma")
	assert.Contains(t, lastToast(sh), "Aucun test")

	ft.run = func(context.Context) (string, error) { return "ok", nil }
	onLoop(app, func() { v.dirtyLocal = true })
	pressT(t, app, v, "gitlab")
	prepared, runs = ft.counts()
	assert.Equal(t, []string{"figma"}, prepared, "nothing prepared with unsaved changes")
	assert.Zero(t, runs)
	assert.Contains(t, lastToast(sh), "w")
}

// Bug 10: the Figma server always calls api.figma.com, so the view offers no
// URL for Figma (neither the one of the team, nor the imposed one, nor the
// personal one); the other services keep theirs.
func TestTeamMCPViewOffersNoURLForFigma(t *testing.T) {
	frLocale(t)
	v, _, _, app := mountTeamMCP(t, TeamMCPViewConfig{})

	urlKeys := map[string][]string{}
	onLoop(app, func() {
		for _, f := range v.fields {
			if f.Key == "url" || f.Key == "url_enforced" {
				urlKeys[f.Section] = append(urlKeys[f.Section], f.Key)
			}
		}
	})

	assert.Empty(t, urlKeys["figma"])
	assert.Empty(t, urlKeys["figma.perso"])
	assert.Equal(t, []string{"url", "url_enforced"}, urlKeys["gitlab"])
	assert.Equal(t, []string{"url"}, urlKeys["gitlab.perso"])
	assert.Equal(t, []string{"url", "url_enforced"}, urlKeys["jira"])
	assert.Equal(t, []string{"url"}, urlKeys["jira.perso"])
}

// A URL recorded for Figma in hub.toml or in the team config is neither shown
// nor rewritten by a save.
func TestTeamMCPViewSaveKeepsARecordedFigmaURL(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	require.NoError(t, config.Update(func(c *config.Config) error { c.MCP.Figma.URL = "https://recorded.example"; return nil }))
	host := hubHost{shared}
	v, _, _, app := mountTeamMCP(t, TeamMCPViewConfig{
		GetMCPConfig:    func() config.MCPConfig { return shared.MCP },
		UpdateHubConfig: host.update,
	})
	onLoop(app, func() {
		v.localMCP.Figma.Enabled = true
		v.dirtyLocal = true
		v.save()
	})

	got, err := config.Read()
	require.NoError(t, err)
	assert.True(t, got.MCP.Figma.Enabled)
	assert.Equal(t, "https://recorded.example", got.MCP.Figma.URL)
}
