package views

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
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
					fmt.Fprintf(&b, "%s | %s\n", it.MainText, it.SecondaryText)
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
