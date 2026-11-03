package views

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/modelcheck"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The views of the Configuration group read their data in load(): the mount
// returns at once with the loading placeholder, the answer of a read made
// before an unmount is not applied to the next mount, and the view can be
// mounted again (run with -race).

// contentCount is the number of items of the content, read on the loop.
func contentCount(loop *tview.Application, content *tview.Flex) (n int) {
	onLoop(loop, func() { n = content.GetItemCount() })
	return
}

// eventually waits for cond, evaluated on the loop.
func eventually(t *testing.T, loop *tview.Application, cond func() bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = cond() })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
}

func TestSecretsViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewSecretsView(SecretsViewConfig{
		GetStore:             func() *keychain.Store { return nil },
		GetActiveProjectID:   func() string { return "" },
		GetActiveProjectName: func() string { return "" },
		GetExpectedKeys: func() []ExpectedSecret {
			if calls.Add(1) == 1 {
				close(first)
				<-release
				return []ExpectedSecret{{Key: "ancienne-cle", Source: "mcp.gitlab", Scope: "global"}}
			}
			return []ExpectedSecret{{Key: "nouvelle-cle", Source: "mcp.jira", Scope: "global"}}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	text := func() (txt string) {
		onLoop(loop, func() {
			if v.list != nil {
				txt = listText(v.list)
			}
		})
		return
	}

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.Nil(t, v.list, "the list waits for the keychain")
		assert.Equal(t, 1, content.GetItemCount(), "the loading placeholder is shown")
	})

	onLoop(loop, v.Unmount)
	second := tview.NewFlex()
	onLoop(loop, func() { v.Mount(second, loop) })
	require.Eventually(t, func() bool { return strings.Contains(text(), "nouvelle-cle") }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, text(), "ancienne-cle", "the answer of the previous mount is dropped")
	assert.Equal(t, 1, contentCount(loop, content), "nor does it reach the old content")
}

func TestModelsViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	hubConfig(t, func(c *config.Config) { c.Models.Default = "sonnet" })
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewModelsView(nil)
	v.SetCatalog(func(context.Context) modelcheck.Catalog {
		if calls.Add(1) == 1 {
			close(first)
			<-release
			return modelcheck.NewCatalog(map[string]string{"ancien": "quality"})
		}
		return modelcheck.NewCatalog(map[string]string{"recent": "quality"})
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.Nil(t, v.table, "the table waits for the catalogue")
		assert.Equal(t, 1, content.GetItemCount())
	})
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	eventually(t, loop, func() bool { return v.table != nil })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.NoError(t, v.catalog.CheckAgent("recent"), "the catalogue is the one of the second read")
		assert.Error(t, v.catalog.CheckAgent("ancien"))
	})
}

func TestModelsViewMountFailureIsShown(t *testing.T) {
	frLocale(t)
	t.Setenv("OH_HOME", t.TempDir())
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, os.MkdirAll(filepath.Dir(config.ConfigPath()), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte("this is not [toml"), 0o600))
	v := NewModelsView(nil)
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	time.Sleep(100 * time.Millisecond)
	onLoop(loop, func() {
		assert.Nil(t, v.table, "no table on a hub.toml that cannot be read")
		require.Equal(t, 1, content.GetItemCount())
	})
}

func TestProviderViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	t.Setenv("HOME", t.TempDir())
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	live := &config.Config{}
	v := NewProviderView(nil, ProviderViewConfig{
		GetConfig: func() *config.Config { return live },
		CheckSecret: func(context.Context, string) (bool, string) {
			if calls.Load() == 0 {
				calls.Add(1)
				close(first)
				<-release
			}
			return false, ""
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.Nil(t, v.list, "the list waits for the machine and the keychain")
		assert.Equal(t, 1, content.GetItemCount())
	})
	onLoop(loop, v.Unmount)
	close(release)
	time.Sleep(100 * time.Millisecond)
	onLoop(loop, func() { assert.Nil(t, v.list, "the answer of an unmounted view is dropped") })

	onLoop(loop, func() { v.Mount(content, loop) })
	eventually(t, loop, func() bool { return v.list != nil && len(v.lines) > 0 })
}

func TestProjectConfigViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	p := &domain.Project{ID: "p1", Name: "web", Path: "/tmp/web"}
	v := NewProjectConfigView(ProjectConfigViewConfig{
		GetProject: func() *domain.Project { cp := *p; return &cp },
		ExecHints: func(string) ProjectExecHints {
			if calls.Add(1) == 1 {
				close(first)
				<-release
			}
			return ProjectExecHints{}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.Nil(t, v.list, "the list waits for the hints")
		assert.Equal(t, 1, content.GetItemCount())
	})
	onLoop(loop, v.Unmount)
	second := tview.NewFlex()
	onLoop(loop, func() { v.Mount(second, loop) })
	eventually(t, loop, func() bool { return v.list != nil && len(v.fields) > 0 })

	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, contentCount(loop, content), "the answer of the previous mount does not reach the old content")
}

// The team detail reads the profile of the member off the loop: the fields
// wait for it, and the answer of a previous mount is dropped.
func TestTeamDetailLoadAndRemount(t *testing.T) {
	frLocale(t)
	shared := seedHome(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return shared.MCP },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return shared.Tracker },
		ResolveTeam:           func() TeamResolution { return TeamResolution{} },
		GetMember: func(context.Context, string) *teamstate.Member {
			if calls.Add(1) == 1 {
				close(first)
				<-release
				return &teamstate.Member{ID: "ancien"}
			}
			return &teamstate.Member{ID: "recent"}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() { assert.Empty(t, v.fields, "no field before the read") })
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	eventually(t, loop, func() bool { return len(v.fields) > 0 && v.member.ID == "recent" })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() { assert.Equal(t, "recent", v.member.ID, "the answer of the previous mount is dropped") })
}

// The tracker type of the team is read at mount, off the loop, with the
// active team captured on it.
func TestSettingsViewReadsTheTrackerTypeOffTheLoop(t *testing.T) {
	frLocale(t)
	started, release := make(chan struct{}), make(chan struct{})
	live := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@gitlab.example:core.git"}}}
	var got atomic.Value
	v := NewSettingsView(SettingsViewConfig{
		GetConfig: func() *config.Config { return live },
		TrackerType: func(team config.TeamConfig) string {
			got.Store(team.ID)
			close(started)
			<-release
			return "jira"
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	<-started
	onLoop(loop, func() { assert.Nil(t, v.list, "the list waits for the team-state") })
	close(release)
	eventually(t, loop, func() bool { return v.list != nil })
	assert.Equal(t, "core", got.Load())
	onLoop(loop, func() { assert.True(t, v.trackerIsJira()) })
}
