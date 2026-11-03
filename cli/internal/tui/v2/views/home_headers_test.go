package views

import (
	"context"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The landings against the mock-ups of the TUI design.

// §1: the header of the hub tells the tool and its adapter; the Système
// section lists Réglages, Métriques, Doctor, Secrets, Notifications and the
// catalogue of the bricks.
func TestHubLandingShowsTheToolAndTheWholeSystemSection(t *testing.T) {
	frLocale(t)
	v := NewHomeView(HomeViewConfig{ToolLine: func() string { return "acme v1.2.3 · adaptateur acme-v1 ✔" }})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.info != nil && v.list != nil || v.dual != nil })

	// The items are written by the load, on the event loop: read them there.
	var info string
	var items []homeItem
	onLoop(app, func() { info = v.info.GetText(true); items = append(items, v.items...) })
	assert.Contains(t, info, "acme v1.2.3 · adaptateur acme-v1 ✔")

	var system []string
	inSystem := false
	for _, it := range items {
		if it.Icon == "─" {
			inSystem = it.Label == "Système"
			continue
		}
		if inSystem {
			system = append(system, it.Label)
		}
	}
	assert.Equal(t, []string{"Configuration", "Métriques", "Doctor", "Secrets", "Notifications", "Catalogue des briques"}, system)
	for _, it := range items {
		switch it.Label {
		case "Notifications":
			assert.Equal(t, "notifications", it.ViewID)
		case "Catalogue des briques":
			assert.Equal(t, "project.agents", it.ViewID)
		}
	}
}

func TestHubLandingWithoutToolLine(t *testing.T) {
	frLocale(t)
	v := NewHomeView(HomeViewConfig{})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.info != nil })
	onLoop(app, func() { assert.Empty(t, v.info.GetText(true)) })
}

// §2: the header of the project tells its execution environments with their
// state, read off the event loop.
func TestProjectLandingShowsTheExecutionEnvironments(t *testing.T) {
	frLocale(t)
	v := NewProjectModeView(ProjectModeConfig{
		ToolLine: func() string { return "acme v1.2.3" },
		Runtimes: func(context.Context, *ActiveProject) []LaunchRuntime {
			return []LaunchRuntime{
				{Kind: "local", Label: "⌂ local", Available: true},
				{Kind: "container", Label: "▣ conteneur · colima", Available: true},
				{Kind: "remote", Label: "☁ distant", Available: false},
			}
		},
	})
	v.SetShell(&mockShell{project: &ActiveProject{ID: "p1", Name: "openhub", Path: "/src/openhub", Branch: "main"}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	eventually(t, app, func() bool { return len(v.runtimes) == 3 })

	var header string
	onLoop(app, func() { header = v.headerTV.GetText(true) })
	assert.Contains(t, header, "exécution : ⌂ local ✔  ▣ conteneur · colima ✔  ☁ distant ✗")
	assert.Contains(t, header, "/src/openhub · main")
	assert.Contains(t, header, "acme v1.2.3")
}

func TestRuntimesLine(t *testing.T) {
	frLocale(t)
	assert.Empty(t, runtimesLine(nil))
	text := tview.NewTextView().SetDynamicColors(true).SetText(runtimesLine([]LaunchRuntime{{Label: "⌂ local", Available: true}, {Label: "☁ distant"}}))
	assert.Equal(t, "exécution : ⌂ local ✔  ☁ distant ✗", text.GetText(true))
}

// The name of the team is drawn as it was typed, like the name of a project
// (it was upper-cased in the team landing only).
func TestTeamLandingDrawsTheNameAsTyped(t *testing.T) {
	frLocale(t)
	v := NewTeamModeView(TeamModeConfig{})
	v.SetShell(&mockShell{team: &ActiveTeam{ID: "core", Name: "Core Team"}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.bannerTV != nil })

	var text string
	onLoop(app, func() { text = v.bannerTV.GetText(true) })
	plain := func(name string) string {
		banner, _ := renderBanner(name, 100, "#000000")
		return tview.NewTextView().SetDynamicColors(true).SetText(banner).GetText(true)
	}
	require.NotEqual(t, plain("Core Team"), plain("CORE TEAM"))
	assert.Contains(t, text, plain("Core Team"), "the banner of the name as typed")
	assert.NotContains(t, text, plain("CORE TEAM"))
	assert.Equal(t, "Core Team", v.ModeInfo().Label)
}
