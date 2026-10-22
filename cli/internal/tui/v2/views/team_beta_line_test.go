package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Team mode is a beta: the team board and the team configuration say so on
// their first line (colour by theme tag, text from the locales).
func TestTeamViewsShowBetaLine(t *testing.T) {
	i18ntest.Set(t, "en")
	want := theme.ColorTag(theme.WarningHex) + i18n.T("tui.team.beta.line") + theme.TagColor

	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets:      []TeamTicket{{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"}},
		IsConfigured: func() bool { return true },
	})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	defer board.Unmount()
	first, ok := board.boardLayout.GetItem(0).(*tview.TextView)
	require.True(t, ok)
	assert.Equal(t, want, first.GetText(false))

	detail := teamDetailFor(TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir()}, nil)
	detail.Mount(tview.NewFlex(), nil)
	defer detail.Unmount()
	first, ok = detail.page.Left.GetItem(0).(*tview.TextView)
	require.True(t, ok)
	assert.Equal(t, want, first.GetText(false))
}

// A solo space is not the team mode: no beta line.
func TestTeamDetailSoloHasNoBetaLine(t *testing.T) {
	detail := teamDetailFor(TeamResolution{Solo: true, TeamID: "solo", StatePath: t.TempDir()}, nil)
	detail.Mount(tview.NewFlex(), nil)
	defer detail.Unmount()
	assert.Equal(t, 1, detail.page.Left.GetItemCount(), "the list only")
}
