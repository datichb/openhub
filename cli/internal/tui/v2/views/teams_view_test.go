package views

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// The Teams view used to show raw keys and %!(EXTRA …): none of its
// tui.teams.* keys existed.
func TestTeamsViewTranslated(t *testing.T) {
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		cfg := &config.Config{Teams: []config.TeamConfig{
			{ID: "core", Name: "Core", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
			{ID: "solo", Solo: true, MemberID: "bob"},
		}}
		v := NewTeamsView(TeamsViewDeps{Config: cfg})
		v.rebuild()
		var texts []string
		for _, it := range v.list.GetItems() {
			texts = append(texts, it.MainText, it.SecondaryText)
		}
		for _, c := range v.ContextCommands() {
			texts = append(texts, c.Label, c.Description, c.Category)
		}
		v.cfg.Teams = nil
		v.rebuild()
		for _, it := range v.list.GetItems() {
			texts = append(texts, it.MainText, it.SecondaryText)
		}
		for _, s := range texts {
			if strings.Contains(s, "tui.teams.") || strings.Contains(s, "%!") {
				t.Errorf("[%s] untranslated or misformatted text: %q", lang, s)
			}
		}
		if got := v.list.GetItems(); len(got) == 0 {
			t.Fatalf("[%s] no item for an empty team list", lang)
		}
	}
}
