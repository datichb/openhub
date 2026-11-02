package cmd

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// registryApps are the apps whose commands and views are pinned: nothing
// configured, a project, a team, and the GitLab write mode.
func registryApps() map[string]*app.App {
	team := config.TeamConfig{ID: "t", Enabled: true, StateRepo: "git@x:t.git", MemberID: "me"}
	project := domain.Project{ID: "p1", Name: "p1", Path: "/tmp/p1", Status: domain.ProjectStatusActive}
	gitlab := &config.Config{}
	gitlab.MCP.Gitlab.WriteEnabled = true
	return map[string]*app.App{
		"empty":   {Config: &config.Config{}},
		"project": {Config: &config.Config{}, Projects: &mockProjectStore{projects: []domain.Project{project}}},
		"team":    {Config: &config.Config{Teams: []config.TeamConfig{team}}},
		"gitlab":  {Config: gitlab},
	}
}

func inEnglish(t *testing.T) {
	t.Helper()
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

// funcName names a function reference; a closure is only "closure" (its name
// depends on where it is written).
func funcName(f any) string {
	if f == nil || reflect.ValueOf(f).IsNil() {
		return "-"
	}
	name := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	if strings.Contains(name, ".func") {
		return "closure"
	}
	return name[strings.LastIndex(name, "/")+1:]
}

func dumpCommands(cmds []shell.Command) string {
	var b strings.Builder
	for _, c := range cmds {
		modes := make([]string, len(c.Modes))
		for i, m := range c.Modes {
			modes[i] = fmt.Sprint(m)
		}
		fmt.Fprintf(&b, "%s | %q | %v | %q | %q | prio=%d | view=%s | action=%s | direct=%v | enabled=%v | modes=%s\n",
			c.ID, c.Label, c.Aliases, c.Description, c.Category, c.Priority, c.ViewID,
			funcName(c.Action), c.RunsDirect, c.Enabled != nil, strings.Join(modes, ","))
	}
	return b.String()
}

// The commands of the omnibar, in order, for each kind of app: splitting
// buildCommands must not move, drop or change one.
func TestBuildCommandsRegistry(t *testing.T) {
	inEnglish(t)
	for name, a := range registryApps() {
		t.Run(name, func(t *testing.T) {
			golden.Assert(t, "tui_commands_"+name, dumpCommands(buildCommands(a)))
		})
	}
}

// The views of the shell, in registration order: the shell finds a view by its
// ID, and the order is the one of the navigation.
func TestBuildViewsRegistry(t *testing.T) {
	inEnglish(t)
	for name, a := range registryApps() {
		t.Run(name, func(t *testing.T) {
			var b strings.Builder
			for _, v := range buildViews(a) {
				fmt.Fprintf(&b, "%s | %T | %q\n", v.ID(), v, v.Title())
			}
			require.NotEmpty(t, b.String())
			golden.Assert(t, "tui_views_"+name, b.String())
		})
	}
}

// The commands that are closures do nothing outside of the TUI.
func TestBuildCommandsClosuresWithoutShell(t *testing.T) {
	prev := tuiShell
	tuiShell = nil
	t.Cleanup(func() { tuiShell = prev })
	byID := map[string]shell.Command{}
	for _, c := range buildCommands(registryApps()["empty"]) {
		byID[c.ID] = c
	}
	for _, id := range []string{"project.mode", "hub.mode", "quit"} {
		require.NotNil(t, byID[id].Action, id)
		byID[id].Action()
	}
}
