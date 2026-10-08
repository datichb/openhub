package views

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// screenText draws a primitive on a simulation screen and returns its text.
func screenText(t *testing.T, p tview.Primitive, w, h int) string {
	t.Helper()
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(w, h)
	p.SetRect(0, 0, w, h)
	p.Draw(sc)
	sc.Show()
	cells, cw, _ := sc.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteByte(' ')
		}
		if (i+1)%cw == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// A33: Ctrl+T enters the team mode from the hub and the project mode, and
// goes back to the hub from the team mode: every hint says so.
func TestCtrlTHintsMatchTheMode(t *testing.T) {
	team, hub := "Ctrl+T "+i18n.T("tui.hints.team_mode"), "Ctrl+T "+i18n.T("tui.hints.hub_mode")

	pm := NewProjectModeView(ProjectModeConfig{})
	pm.SetShell(&mockShell{project: &ActiveProject{ID: "p1", Name: "my-app", Path: "/tmp/my-app"}})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	pm.Mount(content, tview.NewApplication())
	assert.Contains(t, pm.StatusHints(), team)
	assert.NotContains(t, pm.StatusHints(), hub)
	assert.Contains(t, screenText(t, content, 200, 60), "Ctrl+T "+i18n.T("tui.home.shortcut.team_mode"))
	assert.Contains(t, NewProjectModeView(ProjectModeConfig{}).StatusHints(), team)

	tm := NewTeamModeView(TeamModeConfig{})
	tm.SetShell(&mockShell{team: &ActiveTeam{ID: "t1", Name: "alpha"}})
	content = tview.NewFlex().SetDirection(tview.FlexRow)
	tm.Mount(content, tview.NewApplication())
	assert.Contains(t, tm.StatusHints(), hub)
	screen := screenText(t, content, 200, 60)
	assert.Contains(t, screen, "Ctrl+T "+i18n.T("tui.home.shortcut.hub_mode"))
	assert.NotContains(t, screen, "Ctrl+T "+i18n.T("tui.home.shortcut.team_mode"))
}

// A33: the team entries are translated; the team mode entry of the catalogue
// says what it opens.
func TestTeamEntriesTranslated(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	for key, want := range map[string]string{
		"tui.pm.item.team_status": "Statut de l'équipe", "tui.pm.item.team_board": "Board de l'équipe",
		"tui.pm.item.team_activity": "Activité de l'équipe", "tui.tm.item.team_board": "Board de l'équipe",
		"tui.tm.item.team_status": "Statut de l'équipe", "tui.tm.item.workflow": "Workflows de l'équipe",
	} {
		assert.Equal(t, want, i18n.T(key), key)
	}
	assert.NotContains(t, i18n.T("tui.cmd.project_mode.desc"), "Ctrl+T")
	assert.NotContains(t, i18n.T("tui.pm.hint_hub"), "Ctrl+T")
}
