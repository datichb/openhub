package views

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
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

// The title of the first section ("MCP Gitlab") is drawn when the view
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

// t calls the server of the selected service (it used to check that a
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
	assert.Contains(t, lastToast(sh), "Ctrl+S")
}

// The Figma server always calls api.figma.com and the Google Slides
// one slides.googleapis.com, so the view offers no URL for them (neither the
// one of the team, nor the imposed one, nor the personal one); the other
// services keep theirs.
func TestTeamMCPViewOffersNoURLForFigmaAndGoogleSlides(t *testing.T) {
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
	assert.Empty(t, urlKeys["gslides"])
	assert.Empty(t, urlKeys["gslides.perso"])
	assert.Equal(t, []string{"url", "url_enforced"}, urlKeys["gitlab"])
	assert.Equal(t, []string{"url"}, urlKeys["gitlab.perso"])
	assert.Equal(t, []string{"url", "url_enforced"}, urlKeys["jira"])
	assert.Equal(t, []string{"url"}, urlKeys["jira.perso"])
}

// A URL recorded for Figma or Google Slides in hub.toml or in the team config
// is neither shown nor rewritten by a save.
func TestTeamMCPViewSaveKeepsARecordedFigmaURL(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.MCP.Figma.URL = "https://recorded.example"
		c.MCP.Gslides.URL = "https://slides.recorded.example"
		return nil
	}))
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
	waitFor(t, app, func() bool { return !v.saving }) // the write runs off the loop

	got, err := config.Read()
	require.NoError(t, err)
	assert.True(t, got.MCP.Figma.Enabled)
	assert.Equal(t, "https://recorded.example", got.MCP.Figma.URL)
	assert.Equal(t, "https://slides.recorded.example", got.MCP.Gslides.URL)
}

// Jalon 2: an edit made while a reload reads the team config is not erased
// without asking. The edit is kept until the user agrees to drop it.
func TestTeamMCPViewReloadAsksWhenAnEditCameDuringTheRead(t *testing.T) {
	frLocale(t)
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{})
	rebuilt := 0
	onLoop(app, func() {
		v.open(func() { rebuilt++ })
		// the read is in flight (its result comes later, on this loop)
		v.localMCP.Gitlab.Enabled = true
		v.dirtyLocal = true
	})
	waitFor(t, app, func() bool { return sh.confirmCount() == 1 })
	onLoop(app, func() {
		assert.True(t, v.localMCP.Gitlab.Enabled, "the edit is still there while the question is asked")
		assert.Zero(t, rebuilt)
	})

	// no answer: the edit stays; a yes takes the file
	sh.confirmed(t, app)
	onLoop(app, func() {
		assert.False(t, v.localMCP.Gitlab.Enabled, "dropped after the yes")
		assert.Equal(t, 1, rebuilt)
	})

	// without edit during the read, nothing is asked
	onLoop(app, func() { v.open(func() { rebuilt++ }) })
	waitFor(t, app, func() bool { return rebuilt == 2 })
	assert.Equal(t, 1, sh.confirmCount(), "no second question")
}

// Saved off the event loop, one at a time: an edit made while a save runs is
// written by a second save, and a failed write gives the changes back.
func TestTeamMCPViewSavesOffTheLoopOneAtATime(t *testing.T) {
	frLocale(t)
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	var calls atomic.Int32
	var failFirst atomic.Bool
	failFirst.Store(true)
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{
		UpdateHubConfig: func(fn func(*config.Config) error) error {
			calls.Add(1)
			started <- struct{}{}
			<-release
			if failFirst.CompareAndSwap(true, false) {
				return errors.New("disk full")
			}
			return fn(&config.Config{})
		},
	})
	onLoop(app, func() {
		v.localMCP.Gitlab.Enabled = true
		v.dirtyLocal = true
		v.save() // returns at once
		assert.True(t, v.saving)
		assert.False(t, v.dirtyLocal, "the flag goes down for the write; an edit sets it again")
		v.localMCP.Jira.Enabled = true
		v.dirtyLocal = true
		v.save()
		assert.True(t, v.saveAgain)
	})
	<-started
	assert.EqualValues(t, 1, calls.Load(), "one write at a time")
	release <- struct{}{} // the first write fails
	waitFor(t, app, func() bool { return !v.saving })
	onLoop(app, func() {
		assert.True(t, v.dirtyLocal, "a failed save leaves the changes to save")
		assert.Contains(t, sh.lastToast(), "disk full")
	})
}

// selectTeamMCPField puts the cursor on a field of a section (key and
// section as the view names them: "enabled", "gitlab").
func selectTeamMCPField(t *testing.T, v *TeamMCPView, section, key string) *configField {
	t.Helper()
	for i := range v.fields {
		f := &v.fields[i]
		if f.Section == section && f.Key == key && f.Scope == ScopeTeamShared {
			for idx, it := range v.list.GetItems() {
				if ref, ok := it.Reference.(int); ok && ref == i {
					v.list.SelectIndex(idx)
					return f
				}
			}
		}
	}
	t.Fatalf("no field %s of %s", key, section)
	return nil
}

func pressSpace(v *TeamMCPView) {
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
}

// Every known service has a section, GitHub and Linear included, and the
// team can say three things about each: enabled, disabled, or nothing.
func TestTeamMCPViewHasASectionForEveryService(t *testing.T) {
	frLocale(t)
	v, _, _, app := mountTeamMCP(t, TeamMCPViewConfig{})

	headers := map[string]bool{}
	states := map[string]string{}
	onLoop(app, func() {
		for _, f := range v.fields {
			if f.Kind == CfgFieldSectionHeader {
				headers[f.Label] = true
			}
			if f.Scope == ScopeTeamShared && f.Key == "enabled" {
				states[f.Section] = f.Get()
				assert.Equal(t, CfgFieldTriBool, f.Kind, f.Section)
			}
		}
	})
	for _, name := range []string{"Gitlab", "Jira", "Figma", "Gslides", "Github", "Linear"} {
		assert.True(t, headers["MCP "+name], "section %s", name)
	}
	assert.Len(t, states, 6)
	for svc, val := range states {
		assert.Empty(t, val, "%s: the team says nothing yet", svc)
	}

	// GitHub and Linear servers call a fixed address: no URL to offer.
	onLoop(app, func() {
		for _, f := range v.fields {
			if f.Key == "url" || f.Key == "url_enforced" {
				assert.NotContains(t, []string{"github", "linear", "github.perso", "linear.perso"}, f.Section)
			}
		}
	})
}

// A file written before the three states reads the same way: true is
// enabled, false is disabled, no key is inherited. All three are shown.
func TestTeamMCPViewShowsTheThreeStates(t *testing.T) {
	frLocale(t)
	v, _, content, app := mountTeamMCP(t, TeamMCPViewConfig{})
	onLoop(app, func() {
		v.teamCfg.MCP["gitlab"] = teamstate.SharedMCPConfig{Enabled: boolRef(true)}
		v.teamCfg.MCP["jira"] = teamstate.SharedMCPConfig{Enabled: boolRef(false)}
		v.buildFields()
		v.renderFields()
	})
	get := func(svc string) string {
		var val string
		onLoop(app, func() { val = selectTeamMCPField(t, v, svc, "enabled").Get() })
		return val
	}
	assert.Equal(t, "true", get("gitlab"))
	assert.Equal(t, "false", get("jira"))
	assert.Equal(t, "", get("figma"))

	var texts []string
	onLoop(app, func() {
		for _, svc := range []string{"gitlab", "jira", "figma"} {
			f := selectTeamMCPField(t, v, svc, "enabled")
			texts = append(texts, f.Display(f.Get()))
		}
	})
	assert.Contains(t, texts[0], "activé")
	assert.Contains(t, texts[1], "désactivé")
	assert.Contains(t, texts[2], "hérité")
	_ = content
}

func boolRef(b bool) *bool { return &b }

// The space bar goes through the states: inherited, enabled, disabled,
// inherited. Back to inherited removes the key (and the empty table); the
// enforcement only exists next to an opinion.
func TestTeamMCPViewSpaceCyclesThroughTheStates(t *testing.T) {
	frLocale(t)
	v, _, _, app := mountTeamMCP(t, TeamMCPViewConfig{})

	step := func() (value string, entry bool, enforcedShown bool) {
		onLoop(app, func() {
			selectTeamMCPField(t, v, "linear", "enabled")
			pressSpace(v)
			value = selectTeamMCPField(t, v, "linear", "enabled").Get()
			_, entry = v.teamCfg.MCP["linear"]
			for i, f := range v.fields {
				if f.Section == "linear" && f.Key == "enabled_enforced" {
					for _, it := range v.list.GetItems() {
						if ref, ok := it.Reference.(int); ok && ref == i {
							enforcedShown = true
						}
					}
				}
			}
		})
		return value, entry, enforcedShown
	}

	val, entry, enforced := step()
	assert.Equal(t, "true", val)
	assert.True(t, entry)
	assert.True(t, enforced, "with an opinion the team may enforce it")

	val, entry, enforced = step()
	assert.Equal(t, "false", val)
	assert.True(t, entry, "false is written, not dropped")
	assert.True(t, enforced)

	val, entry, enforced = step()
	assert.Equal(t, "", val)
	assert.False(t, entry, "inherited: no entry left")
	assert.False(t, enforced, "nothing to enforce without an opinion")
	onLoop(app, func() { assert.True(t, v.dirtyTeam) })
}

// Enter offers the three states; choosing one sets it.
func TestTeamMCPViewEnterOffersTheThreeStates(t *testing.T) {
	frLocale(t)
	v, sh, _, app := mountTeamMCP(t, TeamMCPViewConfig{})
	onLoop(app, func() {
		v.teamCfg.MCP["github"] = teamstate.SharedMCPConfig{Enabled: boolRef(true), EnabledEnforced: boolRef(true)}
		v.buildFields()
		v.renderFields()
		selectTeamMCPField(t, v, "github", "enabled")
		v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	})
	sh.mu.Lock()
	opts, choose := sh.selectOpts, sh.selectFn
	sh.mu.Unlock()
	require.Len(t, opts, 3)
	assert.Equal(t, []string{"", "true", "false"}, []string{opts[0].Value, opts[1].Value, opts[2].Value})

	onLoop(app, func() { choose("") })
	onLoop(app, func() {
		assert.Empty(t, v.teamCfg.MCP, "inherited: the entry and its enforcement are gone")
	})
}

// Saving writes only the key that changed, on top of the latest file, and
// returning to inherited removes the key from what is written.
func TestTeamMCPViewSaveWritesTheStateAndRemovesTheKey(t *testing.T) {
	frLocale(t)
	type call struct{ base, edited *teamstate.TeamConfig }
	calls := make(chan call, 4)
	v, _, _, app := mountTeamMCP(t, TeamMCPViewConfig{
		SaveTeamConfig: func() TeamConfigWriter {
			return func(_ context.Context, base, edited *teamstate.TeamConfig) error {
				calls <- call{base, edited}
				return nil
			}
		},
	})
	onLoop(app, func() {
		v.teamCfg.MCP["jira"] = teamstate.SharedMCPConfig{Enabled: boolRef(true)}
		v.baseTeam = teamstate.CloneConfig(v.teamCfg) // as if read from the file
		selectTeamMCPField(t, v, "jira", "enabled")
		pressSpace(v) // enabled → disabled
		selectTeamMCPField(t, v, "jira", "enabled")
		pressSpace(v) // disabled → inherited
		selectTeamMCPField(t, v, "linear", "enabled")
		pressSpace(v) // inherited → enabled
		v.save()
	})
	c := <-calls
	waitFor(t, app, func() bool { return !v.saving })

	patch := teamstate.DiffConfig(c.base, c.edited)
	assert.Equal(t, []string{"mcp.jira", "mcp.linear"}, patch.Keys())

	onDisk := teamstate.CloneConfig(c.base)
	require.NoError(t, patch.Apply(onDisk))
	assert.NotContains(t, onDisk.MCP, "jira", "the key is removed, not written as false")
	require.NotNil(t, onDisk.MCP["linear"].Enabled)
	assert.True(t, *onDisk.MCP["linear"].Enabled)
}
