package views

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

func testBrickRows() []BrickRow {
	return []BrickRow{
		{Kind: "agent", ID: "developer", Description: "Implements tickets", Family: "developer", Mode: "subagent",
			Origin: "hub", Skills: []string{"developer/dev-standards"}, Tokens: 1200, Workflows: []string{"quick", "ticket"}},
		{Kind: "agent", ID: "my-agent", Origin: "team", TeamID: "my-agent", Source: "catalog/agents/team/my-agent.md", Tokens: 300,
			Version: 1, PublishedBy: "Grace", Problems: []BrickProblem{{Message: "description > 200 caractères"}}},
		{Kind: "skill", ID: "developer/dev-standards", Name: "dev-standards", Origin: "hub", Tokens: 800,
			Agents: []string{"developer"}, Workflows: []string{"ticket"}},
		{Kind: "skill", ID: "review/review-protocol", Name: "review-protocol", Origin: "team", TeamID: "acme/review-protocol",
			Source: "catalog/skills/acme/review-protocol.md", Shadows: "hub:review/review-protocol", Version: 3, PublishedBy: "Grace",
			PublishedAt: time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local), Tokens: 1240, Agents: []string{"reviewer"}, Workflows: []string{"review"}},
		{Kind: "skill", ID: "legacy-helper", Origin: "team", TeamID: "legacy-helper", Unsealed: true, Tokens: 90,
			Problems: []BrickProblem{{Error: true, Message: "frontmatter [illisible]"}}},
	}
}

func brickSections(v *BricksView) []string {
	var out []string
	for _, it := range v.list.Items() {
		if len(out) == 0 || out[len(out)-1] != it.Section {
			out = append(out, it.Section)
		}
	}
	return out
}

func mountedBricks(t *testing.T) *BricksView {
	t.Helper()
	withFrench(t)
	v := NewBricksView(BricksViewConfig{})
	v.SetShell(&mockShell{})
	v.Mount(tview.NewFlex(), nil)
	v.SetRows(testBrickRows())
	return v
}

// Golden: origin (hub, team, team ⟳ hub, not sealed), version, state; the
// detail of a team brick that replaces a hub one.
func TestBricksViewGolden(t *testing.T) {
	v := mountedBricks(t)
	assert.Equal(t, "project.agents", v.ID(), "replaces the former agent selection view")
	require.True(t, v.list.Select("skill:review/review-protocol"))
	golden.Assert(t, "bricks_view", screenText(t, v.list.Primitive(), 120, 20))
	require.NoError(t, v.list.cfg.Validate())
}

func TestBricksViewDetail(t *testing.T) {
	v := mountedBricks(t)
	assert.Equal(t, []string{"Agents (2)", "Skills (3)"}, brickSections(v))
	detail := func(id string) string {
		require.True(t, v.list.Select(id))
		return v.list.detail.GetText(true)
	}
	d := detail("agent:developer")
	assert.Contains(t, d, "quick, ticket")
	assert.Contains(t, d, "~1200 tokens")
	assert.Contains(t, d, "Origine : hub")
	assert.Contains(t, d, "oh brick new")
	assert.NotContains(t, d, "Version", "hub bricks are not versioned")

	d = detail("skill:review/review-protocol")
	assert.Contains(t, d, "équipe (remplace hub:review/review-protocol)")
	assert.Contains(t, d, "v3 · publiée par Grace le 12/10/2026")
	assert.Contains(t, d, "Chargé par : reviewer")
	assert.Contains(t, d, "oh brick edit skill:acme/review-protocol · oh brick publish skill:acme/review-protocol")

	d = detail("skill:legacy-helper")
	assert.Contains(t, d, "non scellée (catalog.lock absent)")
	assert.Contains(t, d, "non publiée")
	assert.Contains(t, d, "✗ frontmatter [illisible]", "escaped")

	d = detail("agent:my-agent")
	assert.Contains(t, d, "v1 · publiée par Grace")
	assert.Contains(t, d, "! description > 200 caractères")
	assert.Contains(t, d, "aucun workflow")
}

func TestBricksViewFilters(t *testing.T) {
	v := mountedBricks(t)
	assert.Nil(t, v.HandleKey(key('f')))
	assert.Equal(t, []string{"Agents (2)"}, brickSections(v), "agents only")
	v.HandleKey(key('f'))
	assert.Equal(t, []string{"Skills (3)"}, brickSections(v), "skills only")
	v.HandleKey(key('f'))
	assert.Len(t, brickSections(v), 2, "all")

	v.query = "implements"
	v.render()
	assert.Equal(t, []string{"Agents (1)"}, brickSections(v))
	v.query = "hub:review"
	v.render()
	assert.Equal(t, []string{"Skills (1)"}, brickSections(v), "the replaced brick is searched too")
	v.query = "nothing-matches"
	v.render()
	assert.Equal(t, 0, v.list.Len())
	assert.Contains(t, screenText(t, v.list.Primitive(), 80, 6), "Aucune brique ne correspond")
	assert.Contains(t, v.StatusHints(), "(écriture : oh brick …)")
	assert.Contains(t, v.StatusHints(), "/ filtrer · f agents/skills/tout")
}

type brickShell struct {
	mockShell
	title string
	opts  []SelectOption
	query func(string)
}

func (s *brickShell) ShowSelectModal(title string, o []SelectOption, _ string, _ func(string)) {
	s.title, s.opts = title, o
}
func (s *brickShell) ShowInputModal(_, _ string, fn func(string)) { s.query = fn }

// Enter offers the workflows that use the brick; / searches.
func TestBricksViewOpenAndSearch(t *testing.T) {
	withFrench(t)
	sh := &brickShell{}
	v := NewBricksView(BricksViewConfig{OpenWorkflow: func(string) {}})
	v.SetShell(sh)
	v.Mount(tview.NewFlex(), nil)
	v.SetRows(testBrickRows())
	v.list.Select("agent:developer")
	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))
	assert.Equal(t, "Workflows qui utilisent developer", sh.title)
	assert.Len(t, sh.opts, 2)
	assert.Nil(t, v.HandleKey(key('/')))
	require.NotNil(t, sh.query)
	sh.query("legacy")
	assert.Equal(t, 1, v.list.Len())
}
