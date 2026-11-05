package views

import (
	"strings"
	"sync"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// The solo spaces of the hub: the Teams section of the landing ends with a
// discreet row, `🧑 2 espaces solo ›`, only when there is at least one; it
// opens the Teams view, which lists them.

// navShell records the views the shell was asked to open.
type navShell struct {
	mockShell
	mu        sync.Mutex
	navigated []string
}

func (s *navShell) NavigateTo(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.navigated = append(s.navigated, id)
}

func (s *navShell) views() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.navigated...)
}

// teamsSectionRows are the labels (tags and icons kept) of the rows of the
// Teams section, in order.
func teamsSectionRows(v *HomeView) (rows []string) {
	in := false
	for _, it := range v.items {
		if it.Icon == "─" {
			in = it.Label == "Équipes"
			continue
		}
		if in {
			rows = append(rows, it.Icon+" "+it.Label)
		}
	}
	return rows
}

func soloHome(solo int, teams ...TeamEntry) *HomeView {
	v := NewHomeView(HomeViewConfig{
		ListTeams:    func() []TeamEntry { return teams },
		ListProjects: func() []ProjectEntry { return []ProjectEntry{{ID: "p1", Name: "api", Path: "/src/api"}} },
		SoloSpaces:   func() int { return solo },
	})
	v.items = v.buildItems(readHomeData(v.cfg))
	return v
}

func TestHomeSoloRow_OnlyWithASoloSpace(t *testing.T) {
	frLocale(t)
	team := TeamEntry{ID: "acme", Name: "acme", MemberCount: 3, ActiveCount: 2, Default: true}

	cases := []struct {
		name  string
		solo  int
		teams []TeamEntry
		want  []string
	}{
		{"no solo space: no row", 0, nil, []string{"➕ Nouvelle équipe"}},
		{"no solo space, a team: no row", 0, []TeamEntry{team}, []string{"👥 acme", "➕ Nouvelle équipe"}},
		{"one solo space: singular", 1, nil, []string{"➕ Nouvelle équipe", "🧑 1 espace solo ›"}},
		{"several solo spaces: plural", 3, []TeamEntry{team}, []string{"👥 acme", "➕ Nouvelle équipe", "🧑 3 espaces solo ›"}},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, teamsSectionRows(soloHome(tc.solo, tc.teams...)), tc.name)
	}

	bare := NewHomeView(HomeViewConfig{})
	bare.items = bare.buildItems(readHomeData(bare.cfg))
	assert.Equal(t, []string{"➕ Nouvelle équipe"}, teamsSectionRows(bare), "without the field of the host: no row")
}

// The row is not on the frame drawn before the read: the count comes with the
// teams, so the row does not blink in and out.
func TestHomeSoloRow_NotBeforeTheRead(t *testing.T) {
	frLocale(t)
	v := NewHomeView(HomeViewConfig{SoloSpaces: func() int { return 2 }})
	v.items = v.buildStaticItems()
	assert.Equal(t, []string{"➕ Nouvelle équipe"}, teamsSectionRows(v))
}

// The sections keep their order and the new row only ends the Teams section:
// the rows before it do not move, in one column and in two.
func TestHomeSoloRow_KeepsTheOrderOfTheLanding(t *testing.T) {
	frLocale(t)
	headers := func(items []homeItem) (out []string) {
		for _, it := range items {
			if it.Icon == "─" {
				out = append(out, it.Label)
			}
		}
		return out
	}
	without, with := soloHome(0), soloHome(2)
	assert.Equal(t, headers(without.items), headers(with.items), "same sections in the same order")

	_, rightWithout := without.splitItems()
	leftWith, rightWith := with.splitItems()
	leftWithout, _ := without.splitItems()
	texts := func(items []widgets.SectionItem) (out []string) {
		for _, it := range items {
			out = append(out, it.MainText+" | "+it.SecondaryText)
		}
		return out
	}
	assert.Equal(t, texts(leftWithout), texts(leftWith), "the left column does not change")
	require.Len(t, rightWith, len(rightWithout)+1)
	assert.Equal(t, texts(rightWithout), texts(rightWith[:len(rightWithout)]), "the right column only gets a last row")
	assert.Equal(t, []string{"Projets", "Équipes"}, collectHeaders(rightWith))
	last := rightWith[len(rightWith)-1]
	assert.Equal(t, "🧑  2 espaces solo ›", last.MainText)
	assert.Empty(t, last.SecondaryText)

	single := with.singleItems()
	assert.Equal(t, "🧑  2 espaces solo ›", single[len(single)-len(with.buildSystemItems())-1].MainText,
		"one column: the row ends the Teams section, right before Système")
}

// Enter on the row opens the Teams view; the host needs no callback for it.
func TestHomeSoloRow_OpensTheTeamsView(t *testing.T) {
	frLocale(t)
	sh := &navShell{}
	v := NewHomeView(HomeViewConfig{SoloSpaces: func() int { return 2 }})
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	eventually(t, app, func() bool { return contains(homeLabels(v), "2 espaces solo ›") })

	onLoop(app, func() {
		for i, it := range v.items {
			if it.Label == "2 espaces solo ›" {
				assert.Equal(t, "teams", it.ViewID)
				v.executeItem(i)
			}
		}
	})
	assert.Equal(t, []string{"teams"}, sh.views())
}

// The landing without a solo space, then with one, as it is drawn (the
// banner and the footer aside): the row is absent from the first.
func TestHomeSoloGolden(t *testing.T) {
	frLocale(t)
	teams := []TeamEntry{{ID: "acme", Name: "Acme Platform", MemberCount: 4, ActiveCount: 2, Default: true}, {ID: "lab", Name: "lab", NotSynced: true}}
	for _, tc := range []struct {
		name string
		solo int
	}{
		{"home_no_solo", 0},
		{"home_solo", 3},
	} {
		v := soloHome(tc.solo, teams...)

		single := widgets.NewSectionedList()
		single.SetItems(v.singleItems())
		oneColumn := screenText(t, single, 100, 44)
		assert.Equal(t, tc.solo > 0, strings.Contains(oneColumn, " solo ›"), tc.name+": the row is there only with a solo space")
		golden.Assert(t, tc.name+"_1col", oneColumn)

		left, right := v.splitItems()
		dual := buildDualColumn(homeFlexConfig{LeftItems: left, RightItems: right})
		golden.Assert(t, tc.name+"_2col", screenText(t, buildDualFlex(dual), 140, 24))
	}
}

// What the row opens lists every solo space the row counts: the Teams view
// shows the solo spaces next to the teams (and an archived one too, marked as
// such, which the row does not count).
func TestHomeSoloRow_CountedSpacesAreInTheTeamsView(t *testing.T) {
	frLocale(t)
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
		{ID: "mine", Enabled: true, Solo: true, StatePath: "/s/mine", MemberID: "alice"},
		{ID: "scratch", Enabled: true, Solo: true, StatePath: "/s/scratch", MemberID: "alice"},
		{ID: "gone", Enabled: false, Solo: true, StatePath: "/s/gone", MemberID: "alice"},
	}}
	v := NewHomeView(HomeViewConfig{SoloSpaces: cfg.SoloTeamCount})
	v.items = v.buildItems(readHomeData(v.cfg))
	assert.Equal(t, []string{"➕ Nouvelle équipe", "🧑 2 espaces solo ›"}, teamsSectionRows(v))

	teams := NewTeamsView(TeamsViewDeps{Config: cfg})
	teams.rebuild()
	var listed []string
	for _, it := range teams.list.GetItems() {
		if id, ok := it.Reference.(string); ok && id != newTeamRef {
			listed = append(listed, id)
		}
	}
	assert.Equal(t, []string{"core", "mine", "scratch", "gone"}, listed)
}
