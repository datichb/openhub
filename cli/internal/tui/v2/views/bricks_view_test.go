package views

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testBrickRows() []BrickRow {
	return []BrickRow{
		{Kind: "agent", ID: "developer", Description: "Implements tickets", Family: "developer", Mode: "subagent",
			Origin: "hub", Skills: []string{"developer/dev-standards"}, Tokens: 1200, Workflows: []string{"quick", "ticket"}},
		{Kind: "agent", ID: "my-agent", Origin: "team", Tokens: 300},
		{Kind: "skill", ID: "developer/dev-standards", Name: "dev-standards", Origin: "hub", Tokens: 800,
			Agents: []string{"developer"}, Workflows: []string{"ticket"}},
	}
}

func headersOf(v *BricksView) []string {
	var out []string
	for _, it := range v.list.GetItems() {
		if it.IsHeader {
			out = append(out, it.MainText)
		}
	}
	return out
}

func TestBricksViewRendersSectionsAndDetail(t *testing.T) {
	v := NewBricksView(BricksViewConfig{})
	v.SetShell(&mockShell{})
	assert.Equal(t, "project.agents", v.ID(), "replaces the former agent selection view")
	v.Mount(tview.NewFlex(), nil)
	v.SetRows(testBrickRows())

	assert.Equal(t, []string{"Agents (2)", "Skills (1)"}, headersOf(v))
	_, it, ok := v.list.CurrentItem()
	require.True(t, ok)
	assert.Contains(t, it.MainText, "developer")
	detail := v.detail.GetText(true)
	assert.Contains(t, detail, "quick, ticket")
	assert.Contains(t, detail, "~1200 tokens")
	assert.Contains(t, detail, "developer/dev-standards")

	team := v.list.GetItems()[2] // my-agent
	assert.Contains(t, team.MainText, i18n.T("tui.bricks.origin.team"))
	assert.Contains(t, team.SecondaryText, i18n.T("tui.bricks.unused"))
}

func TestBricksViewFilters(t *testing.T) {
	v := NewBricksView(BricksViewConfig{})
	v.SetShell(&mockShell{})
	v.Mount(tview.NewFlex(), nil)
	v.SetRows(testBrickRows())

	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'f', 0)))
	assert.Equal(t, []string{"Agents (2)"}, headersOf(v), "agents only")
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'f', 0))
	assert.Equal(t, []string{"Skills (1)"}, headersOf(v), "skills only")
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'f', 0))
	assert.Len(t, headersOf(v), 2, "all")

	v.query = "implements"
	v.render()
	assert.Equal(t, []string{"Agents (1)"}, headersOf(v))
	v.query = "nothing-matches"
	v.render()
	require.Len(t, v.list.GetItems(), 1)
	assert.True(t, strings.Contains(v.list.GetItems()[0].MainText, i18n.T("tui.bricks.empty")))
}
