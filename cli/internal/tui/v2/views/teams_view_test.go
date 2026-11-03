package views

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// The Teams view used to show raw keys and %!(EXTRA …): none of its
// tui.teams.* keys existed. The golden of each language holds every text of
// the view: the list, the commands, then the empty list.
func TestTeamsViewTranslated(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		t.Run(lang, func(t *testing.T) {
			prev := i18n.Locale()
			i18n.SetLocale(lang)
			t.Cleanup(func() { i18n.SetLocale(prev) })

			cfg := &config.Config{Teams: []config.TeamConfig{
				{ID: "core", Name: "Core", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
				{ID: "solo", Solo: true, MemberID: "bob"},
			}}
			v := NewTeamsView(TeamsViewDeps{Config: cfg})
			var b strings.Builder
			items := func(title string) {
				fmt.Fprintf(&b, "== %s\n", title)
				for _, it := range v.list.GetItems() {
					fmt.Fprintf(&b, "%s | %s\n", stripTviewTags(it.MainText), it.SecondaryText)
				}
			}
			v.rebuild()
			items("list")
			b.WriteString("== commands\n")
			for _, c := range v.ContextCommands() {
				fmt.Fprintf(&b, "%s | %s | %s\n", c.Label, c.Description, c.Category)
			}
			v.cfg.Teams = nil
			v.rebuild()
			require.NotEmpty(t, v.list.GetItems(), "an empty team list still shows an item")
			items("empty")

			text := b.String()
			assert.NotContains(t, text, "tui.teams.", "untranslated text")
			assert.NotContains(t, text, "%!", "misformatted text")
			golden.Assert(t, "teams_view_"+lang, text)
		})
	}
}

// confirmingShell keeps the title of the last confirmation and its action.
type confirmingShell struct {
	recordingShell
	title string
	yes   func()
}

func (s *confirmingShell) Confirm(title, _ string, _ bool, onYes func()) {
	s.title, s.yes = title, onYes
}

// The list of the teams names the member (read in the team-state, so after the
// mount), and the confirmation and the toast of a removal name the team, not its
// ID. A team-state that does not tell leaves the member ID.
func TestTeamsViewNamesTheMemberAndTheTeam(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Name: "Core Platform", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
		{ID: "side", Enabled: true, StateRepo: "git@example.com:side/state.git", MemberID: "bob"},
	}}
	v := NewTeamsView(TeamsViewDeps{
		Config: cfg,
		MemberNames: func(_ context.Context, teams []config.TeamConfig) map[string]string {
			assert.Len(t, teams, 2)
			return map[string]string{"core": "Alice Martin"}
		},
	})
	sh := &confirmingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	rows := func() (rows []string) {
		onLoop(app, func() {
			for _, it := range v.list.GetItems() {
				rows = append(rows, it.MainText+" | "+it.SecondaryText)
			}
		})
		return
	}
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool { return strings.Contains(strings.Join(rows(), "\n"), "@Alice Martin") }, 3*time.Second, 20*time.Millisecond)
	all := strings.Join(rows(), "\n")
	assert.NotContains(t, all, "@alice")
	assert.Contains(t, all, "@bob", "a member the team-state does not tell is their ID")

	onLoop(app, func() {
		v.list.SelectIndex(1)
		v.handleDelete()
	})
	require.NotNil(t, sh.yes)
	assert.Contains(t, sh.title, "Core Platform")
	assert.NotContains(t, sh.title, "core")
	onLoop(app, sh.yes)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	require.NotEmpty(t, sh.toasts)
	assert.Equal(t, "Team Core Platform removed", sh.toasts[len(sh.toasts)-1])
}
