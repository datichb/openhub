package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func TestRenderProjectWiki(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	text := renderProjectWiki("atlas", ProjectWikiMetrics{Pages: 9, Specs: 1, Decisions: 2, Domains: 4, DomainsCovered: 3, Uncovered: []string{"business/[x].md"},
		MedianAgeDays: 12, Dated: 9, Oldest: "business/stock.md", OldestDays: 102, Tagged: 20, Uncertain: 5, Confirmed: 10, Deduced: 5,
		Pending: 2, Accepted: 1, Synced: "0123456789abcdef"}, nil)
	for _, want := range []string{"Wiki du projet · atlas", "9 (specs 1, décisions 2, postmortems 0)", "75 % (3/4)", "business/[x[].md",
		"12 jour(s)", "business/stock.md (102 j)", "25 % (5 sur 20", "2 en file, 1 acceptée(s)", "0123456789ab", "doc-sync"} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "0123456789abcdef")
	assert.Contains(t, renderProjectWiki("a", ProjectWikiMetrics{NoWiki: true}, nil), "onboarding")
	assert.Contains(t, renderProjectWiki("a", ProjectWikiMetrics{}, errors.New("boom")), "boom")
	empty := renderProjectWiki("a", ProjectWikiMetrics{Pending: -1}, nil)
	assert.Contains(t, empty, "aucun domaine")
	assert.Contains(t, empty, "inconnues")
	assert.Contains(t, empty, "jamais")
	assert.NotContains(t, empty, "doc-sync les présente")
	assert.Equal(t, 0, wikiPercent(1, 0))
	assert.Equal(t, 33, wikiPercent(1, 3))
	assert.Equal(t, []string{"a", "b", "+2"}, wikiClip([]string{"a", "b", "c", "d"}, 2))
	v := NewProjectWikiView(ProjectWikiViewConfig{})
	assert.Equal(t, "project.wiki", v.ID())
}
