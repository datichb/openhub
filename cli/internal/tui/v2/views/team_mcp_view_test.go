package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

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
