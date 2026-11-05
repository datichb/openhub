package views

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

const brokenConfig = "this is {{{{ invalid toml\n[tracker]\ntype = \"jira\"\n"

// breakConfig makes config.toml of the team-state unreadable and returns its path.
func breakConfig(t *testing.T, repo *teamstate.Repo) string {
	t.Helper()
	path := filepath.Join(repo.Path(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(brokenConfig), 0o644))
	return path
}

// teamViewUnderTest is one of the three views that edit the team config, on a
// team-state, with a writer that counts its calls.
type teamViewUnderTest struct {
	name    string
	mount   func(t *testing.T, tc TeamResolution, sh *viewShell, writes *atomic.Int32) (View, *tview.Application, *tview.Flex)
	errored func(View) bool
	loaded  func(View) bool
}

func countingWriter(writes *atomic.Int32) func() TeamConfigWriter {
	return func() TeamConfigWriter {
		return func(context.Context, *teamstate.TeamConfig, *teamstate.TeamConfig) error {
			writes.Add(1)
			return nil
		}
	}
}

func teamViewsUnderTest() []teamViewUnderTest {
	return []teamViewUnderTest{
		{
			name: "detail",
			mount: func(t *testing.T, tc TeamResolution, sh *viewShell, writes *atomic.Int32) (View, *tview.Application, *tview.Flex) {
				v := NewTeamDetailView(TeamDetailViewConfig{
					GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
					GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
					ResolveTeam:           func() TeamResolution { return tc },
					SaveTeamConfig:        countingWriter(writes),
				})
				v.SetShell(sh)
				content := tview.NewFlex()
				app := runApp(t, content)
				onLoop(app, func() { v.Mount(content, app) })
				return v, app, content
			},
			errored: func(v View) bool { d := v.(*TeamDetailView); return d.cfgErr != nil },
			loaded:  func(v View) bool { d := v.(*TeamDetailView); return d.cfgErr == nil && d.teamCfg != nil },
		},
		{
			name: "mcp",
			mount: func(t *testing.T, tc TeamResolution, sh *viewShell, writes *atomic.Int32) (View, *tview.Application, *tview.Flex) {
				v := NewTeamMCPView(TeamMCPViewConfig{
					ResolveTeam:    func() TeamResolution { return tc },
					GetMCPConfig:   func() config.MCPConfig { return config.MCPConfig{} },
					SaveTeamConfig: countingWriter(writes),
				})
				v.SetShell(sh)
				content := tview.NewFlex()
				app := runApp(t, content)
				onLoop(app, func() { v.Mount(content, app) })
				return v, app, content
			},
			errored: func(v View) bool { return v.(*TeamMCPView).cfgErr != nil },
			loaded:  func(v View) bool { m := v.(*TeamMCPView); return m.cfgErr == nil && m.teamCfg != nil },
		},
		{
			name: "models",
			mount: func(t *testing.T, tc TeamResolution, sh *viewShell, writes *atomic.Int32) (View, *tview.Application, *tview.Flex) {
				v := NewTeamModelsView(TeamModelsViewConfig{
					ResolveTeam:    func() TeamResolution { return tc },
					SaveTeamConfig: countingWriter(writes),
				})
				v.SetShell(sh)
				content := tview.NewFlex()
				app := runApp(t, content)
				onLoop(app, func() { v.Mount(content, app) })
				return v, app, content
			},
			errored: func(v View) bool { return v.(*TeamModelsView).cfgErr != nil },
			loaded:  func(v View) bool { m := v.(*TeamModelsView); return m.cfgErr == nil && m.teamCfg != nil },
		},
	}
}

// A config.toml that does not parse is not an empty configuration: the view
// says it cannot read it, refuses Ctrl+S, writes nothing, and a new read gets
// out of it once the file is fixed.
func TestTeamViewsShowAnUnreadableConfigAndNeverSaveOverIt(t *testing.T) {
	for _, tv := range teamViewsUnderTest() {
		t.Run(tv.name, func(t *testing.T) {
			frLocale(t)
			repo, tc := soloState(t)
			require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
				c.Tracker.Type = "jira"
				return nil
			}))
			path := breakConfig(t, repo)

			var writes atomic.Int32
			sh := &viewShell{}
			v, app, content := tv.mount(t, tc, sh, &writes)
			waitFor(t, app, func() bool { return tv.errored(v) })

			screen := drawn(t, app, content, 120, 20)
			assert.Contains(t, screen, "config.toml illisible")
			assert.Contains(t, screen, "oh doctor")
			assert.Contains(t, v.StatusHints(), "r ", "the one way out is a new read")

			onLoop(app, func() { v.HandleKey(ctrlSKey()) })
			assert.Contains(t, sh.lastToast(), "config.toml illisible", "Ctrl+S is refused with the reason")
			if cp, ok := v.(CommandProvider); ok {
				for _, c := range cp.ContextCommands() {
					if c.ID == "team.mcp.save" || c.ID == "team.models.save" {
						onLoop(app, c.Action)
					}
				}
			}
			time.Sleep(50 * time.Millisecond)
			assert.Zero(t, writes.Load(), "nothing is written")
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, brokenConfig, string(got), "the file is left as it was")

			// once the file is fixed, r reads it again and the view works
			require.NoError(t, os.WriteFile(path, []byte("[tracker]\ntype = \"jira\"\n"), 0o644))
			onLoop(app, func() { v.HandleKey(runeKey('r')) })
			waitFor(t, app, func() bool { return tv.loaded(v) })
		})
	}
}

// A team-state that is not cloned is an error too, not an empty config.
func TestTeamViewsShowAMissingCloneAsAnError(t *testing.T) {
	for _, tv := range teamViewsUnderTest() {
		t.Run(tv.name, func(t *testing.T) {
			frLocale(t)
			tc := TeamResolution{Enabled: true, TeamID: "core", StatePath: filepath.Join(t.TempDir(), "absent"), MemberID: "alice"}
			var writes atomic.Int32
			sh := &viewShell{}
			v, app, content := tv.mount(t, tc, sh, &writes)
			waitFor(t, app, func() bool { return tv.errored(v) })
			assert.Contains(t, drawn(t, app, content, 120, 20), "config.toml illisible")
			onLoop(app, func() { v.HandleKey(ctrlSKey()) })
			time.Sleep(50 * time.Millisecond)
			assert.Zero(t, writes.Load())
		})
	}
}

// The writer the host gives the views: the changes between what the view read
// and what it edited, on top of the latest config.toml.
func repoWriter(repo *teamstate.Repo) func() TeamConfigWriter {
	return func() TeamConfigWriter {
		return func(ctx context.Context, base, edited *teamstate.TeamConfig) error {
			return repo.UpdateConfigFrom(ctx, base, edited)
		}
	}
}

func mountDetailOn(t *testing.T, tc TeamResolution, write func() TeamConfigWriter) (*TeamDetailView, *tview.Application, *viewShell) {
	t.Helper()
	v := NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
		ResolveTeam:           func() TeamResolution { return tc },
		SaveTeamConfig:        write,
	})
	sh := &viewShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.teamCfg != nil })
	return v, app, sh
}

// A colleague changes another setting while the detail is open: Ctrl+S keeps
// theirs and writes mine.
func TestTeamDetailSaveKeepsWhatAColleagueChangedMeanwhile(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	v, app, sh := mountDetailOn(t, tc, repoWriter(repo))

	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Notification.Channel = "from-colleague"
		c.MCP = map[string]teamstate.SharedMCPConfig{"gitlab": {URL: "https://git.example"}}
		return nil
	}))

	onLoop(app, func() { v.teamCfg.Tracker.Type = "jira"; v.dirtyTeam = true; v.HandleKey(ctrlSKey()) })
	waitFor(t, app, func() bool { return !v.saving && sh.lastToast() != "" })

	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "jira", got.Tracker.Type)
	assert.Equal(t, "from-colleague", got.Notification.Channel, "the setting of the colleague is not overwritten by the copy read earlier")
	assert.Equal(t, "https://git.example", got.MCP["gitlab"].URL)
	assert.Zero(t, sh.confirmCount())

	// the second save only carries what changed since the first
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Tracker.Type = "gitlab"
		return nil
	}))
	onLoop(app, func() { v.teamCfg.Takeover.StaleDays = 9; v.dirtyTeam = true; v.HandleKey(ctrlSKey()) })
	waitFor(t, app, func() bool { return !v.saving && !v.dirtyTeam })
	got, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 9, got.Takeover.StaleDays)
	assert.Equal(t, "gitlab", got.Tracker.Type, "a setting this view did not touch since its last save is not rewritten")
}

// The same setting changed by a colleague: nothing is written, the view says
// so and offers to reload, which brings their value.
func TestTeamDetailSaveRefusesToOverwriteTheSameSetting(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	v, app, sh := mountDetailOn(t, tc, repoWriter(repo))

	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Tracker.Type = "jira"
		c.Notification.Channel = "from-colleague"
		return nil
	}))
	onLoop(app, func() {
		v.teamCfg.Tracker.Type = "gitlab"
		v.teamCfg.Takeover.StaleDays = 9
		v.dirtyTeam = true
		v.HandleKey(ctrlSKey())
	})
	require.Eventually(t, func() bool { return sh.confirmCount() == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Contains(t, sh.lastToast(), "tracker.type")

	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "jira", got.Tracker.Type, "the value of the colleague stays")
	assert.NotEqual(t, 9, got.Takeover.StaleDays, "nothing of the refused save is written")
	onLoop(app, func() { assert.True(t, v.dirtyTeam, "the edit is still unpublished") })

	sh.confirmed(t, app) // « recharger ? » yes
	waitFor(t, app, func() bool { return v.teamCfg.Tracker.Type == "jira" && !v.dirtyTeam })
}

// The MCP view and the models view write through the same patch.
func TestTeamMCPAndModelsSaveKeepWhatAColleagueChangedMeanwhile(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	mcp := NewTeamMCPView(TeamMCPViewConfig{
		ResolveTeam: func() TeamResolution { return tc }, GetMCPConfig: func() config.MCPConfig { return config.MCPConfig{} },
		SaveTeamConfig: repoWriter(repo),
	})
	models := NewTeamModelsView(TeamModelsViewConfig{ResolveTeam: func() TeamResolution { return tc }, SaveTeamConfig: repoWriter(repo)})
	for _, v := range []View{mcp, models} {
		sh := &viewShell{}
		v.(interface{ SetShell(ShellAccess) }).SetShell(sh)
		content := tview.NewFlex()
		app := runApp(t, content)
		onLoop(app, func() { v.Mount(content, app) })
		waitFor(t, app, func() bool { return v.(interface{ ready() bool }).ready() })
		require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
			c.Notification.Channel = "from-colleague-" + v.ID()
			return nil
		}))
		onLoop(app, func() {
			switch m := v.(type) {
			case *TeamMCPView:
				s := m.teamCfg.MCP["figma"]
				s.WriteRecommended = true
				m.teamCfg.MCP["figma"] = s
				m.dirtyTeam = true
			case *TeamModelsView:
				m.teamCfg.Models.Default = "opus"
				m.dirty = true
			}
			v.HandleKey(ctrlSKey())
		})
		require.Eventually(t, func() bool {
			got, err := repo.LoadConfig(t.Context())
			return err == nil && (got.Models.Default == "opus" || got.MCP["figma"].WriteRecommended)
		}, 3*time.Second, 10*time.Millisecond, v.ID())
		// the write is over (config.toml is written before the commit and the
		// config_changed event): the folder of the test is not written any more
		waitFor(t, app, func() bool {
			switch m := v.(type) {
			case *TeamMCPView:
				return !m.saving
			case *TeamModelsView:
				return !m.saving
			}
			return true
		})
		got, err := repo.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "from-colleague-"+v.ID(), got.Notification.Channel, v.ID())
	}
}

func (v *TeamMCPView) ready() bool    { return v.cfgErr == nil && v.teamCfg != nil && v.list != nil }
func (v *TeamModelsView) ready() bool { return v.cfgErr == nil && v.teamCfg != nil && v.list != nil }
