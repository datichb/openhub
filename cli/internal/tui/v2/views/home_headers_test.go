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

// The Teams section is on the landing without any team, between Projets and
// Système, in the order of the landing: Démarrer, Sessions, Projets, Équipes,
// Système (the teams came first, before Démarrer, in one column).
func TestHubLandingShowsTeamsBetweenProjectsAndSystem(t *testing.T) {
	frLocale(t)
	v := NewHomeView(HomeViewConfig{
		ListProjects: func() []ProjectEntry { return []ProjectEntry{{ID: "p1", Name: "api", Path: "/src/api"}} },
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	eventually(t, app, func() bool { return contains(homeLabels(v), "api") })

	var headers, rows []string
	onLoop(app, func() {
		require.NotNil(t, v.list, "one column at this width")
		for _, it := range v.list.GetItems() {
			if it.IsHeader {
				headers = append(headers, it.MainText)
			} else {
				rows = append(rows, stripTviewTags(it.MainText)+" | "+it.SecondaryText)
			}
		}
	})
	idx := func(h string) int {
		for i, x := range headers {
			if x == h {
				return i
			}
		}
		return -1
	}
	assert.Less(t, idx("Projets"), idx("Équipes"))
	assert.Less(t, idx("Équipes"), idx("Système"))
	assert.Contains(t, rows, "➕  Nouvelle équipe | Partagez board et réservations : créez une équipe ou rejoignez celle de vos collègues")
}

// The Team view has a section for the team itself (members and my card,
// configuration, MCP, models, policies, patterns, wiki) and its navigation
// lets the member switch team or start another one.
func TestTeamLandingHasTheTeamSection(t *testing.T) {
	frLocale(t)
	var navigated []string
	var switched, started int
	v := NewTeamModeView(TeamModeConfig{
		OnNavigate:   func(id string) { navigated = append(navigated, id) },
		OnSwitchTeam: func() { switched++ },
		OnNewTeam:    func() { started++ },
	})
	v.team = &ActiveTeam{ID: "core", Name: "Core"}
	v.items = v.buildItems()

	section := func(header string) (rows []teamModeItem) {
		in := false
		for _, it := range v.items {
			if it.Icon == "─" {
				in = it.Label == header
				continue
			}
			if in {
				rows = append(rows, it)
			}
		}
		return rows
	}
	labels := func(rows []teamModeItem) (out []string) {
		for _, r := range rows {
			out = append(out, r.Label)
		}
		return out
	}

	team := section("Équipe")
	assert.Equal(t, []string{"Membres et ma fiche", "Config Équipe", "MCP de l'équipe", "Modèles de l'équipe", "Politiques", "Patterns", "Wiki de l'équipe"}, labels(team))
	for _, r := range team {
		r.Action()
	}
	assert.Equal(t, []string{"team.detail", "team.detail", "team.mcp", "team.models", "team.policies", "team.patterns", "team.wiki"}, navigated)

	nav := section("Navigation")
	assert.Equal(t, []string{"Changer d'équipe…", "Nouvelle équipe", "Projets", "Métriques", "Vue Hub"}, labels(nav))
	nav[0].Action()
	nav[1].Action()
	assert.Equal(t, 1, switched)
	assert.Equal(t, 1, started)

	left, right := v.splitItems()
	assert.NotContains(t, collectHeaders(left), "Équipe", "the team section opens the right column")
	assert.Equal(t, []string{"Équipe", "Configuration", "Navigation"}, collectHeaders(right))
}

// Without a team the Team view does not stop at a text: it lists what can be
// done — start a team, choose one, go back to the hub.
func TestTeamLandingWithoutTeamIsActionable(t *testing.T) {
	frLocale(t)
	var started, switched, hub int
	v := NewTeamModeView(TeamModeConfig{
		OnNewTeam:      func() { started++ },
		OnSwitchTeam:   func() { switched++ },
		OnExitTeamMode: func() { hub++ },
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })

	var rows []string
	onLoop(app, func() {
		require.NotNil(t, v.list)
		for _, it := range v.list.GetItems() {
			rows = append(rows, stripTviewTags(it.MainText))
		}
		for i := range v.items {
			v.executeItem(i)
		}
	})
	assert.Equal(t, []string{"➕  Nouvelle équipe", "👥  Changer d'équipe…", "🏠  Vue Hub"}, rows)
	assert.Equal(t, 1, started)
	assert.Equal(t, 1, switched)
	assert.Equal(t, 1, hub)
}

// The landing of a project always has its « Équipe » line, with the team or
// « aucune », and Enter on it opens the choice of the team.
func TestProjectLandingTeamLine(t *testing.T) {
	frLocale(t)
	attach := 0
	line := func(info ProjectTeamInfo) projectModeItem {
		v := NewProjectModeView(ProjectModeConfig{
			Team:         func(*ActiveProject) ProjectTeamInfo { return info },
			OnAttachTeam: func() { attach++ },
		})
		v.project = &ActiveProject{ID: "p1", Name: "api"}
		for i, it := range v.buildItems() {
			if it.Icon == "─" && it.SectionID == "team" {
				return v.buildItems()[i+1]
			}
		}
		t.Fatal("no team section")
		return projectModeItem{}
	}

	none := line(ProjectTeamInfo{})
	assert.Equal(t, "Équipe : aucune", none.Label)
	assert.Equal(t, "› rattacher le projet à une équipe…", none.Desc)
	none.Action()
	assert.Equal(t, 1, attach)

	team := line(ProjectTeamInfo{ID: "core", Name: "Core Platform"})
	assert.Equal(t, "Équipe : Core Platform", team.Label)
	assert.Equal(t, "› changer…", team.Desc)

	assert.Equal(t, "Équipe : espace solo mine", line(ProjectTeamInfo{ID: "mine", Name: "mine", Solo: true}).Label)
	assert.Equal(t, "Équipe : old (archivée)", line(ProjectTeamInfo{ID: "old", Name: "old", Archived: true}).Label)

	bare := NewProjectModeView(ProjectModeConfig{})
	bare.project = &ActiveProject{ID: "p1", Name: "api"}
	assert.NotPanics(t, func() { bare.teamLine().Action() }, "without a host: nothing to open")
	assert.Equal(t, "Équipe : aucune", bare.teamLine().Label)
}
