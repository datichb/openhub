package views

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// queueProposals are the proposals of the sketch of the project queue (§4).
func queueProposals(now time.Time) []WikiProposalView {
	return []WikiProposalView{
		{ID: "wp-17", Agent: "documentarian", Page: "technical/conventions.md", Tag: "PROBABLE", Status: "pending", SessionID: "ses_41c0",
			Workflow: "doc-sync", Created: now.Add(-3 * time.Hour), Source: "CONTRIBUTING.md:12",
			Content: "- Les commits suivent Conventional Commits,\n  suffixe `(modernisation <lot>)`."},
		{ID: "wp-18", Agent: "documentarian", Page: "technical/stack.md", Tag: "CONFIRMÉ", Status: "pending", SessionID: "ses_41c0",
			Workflow: "doc-sync", Created: now.Add(-3 * time.Hour), Replaces: "- Go 1.23", Content: "- Go 1.24"},
		{ID: "wp-12", Agent: "onboarder", Page: "technical/tests.md", Tag: "DÉDUIT", Status: "accepted", SessionID: "ses_30aa",
			Workflow: "onboarding", Created: now.Add(-26 * time.Hour), Content: "- tests en table", Edited: true},
		{ID: "wp-9", Agent: "onboarder", Page: "technical/architecture.md", Tag: "PROBABLE", Status: "stale", SessionID: "ses_30aa",
			Workflow: "onboarding", Created: now.Add(-50 * time.Hour), Replaces: "- hexagonale", Content: "- en couches",
			Error: "the replaced line is no longer in the page once"},
	}
}

type queueBackend struct {
	mu      sync.Mutex
	props   []WikiProposalView
	decided []string
	loads   int
}

func (q *queueBackend) list(_ context.Context, path string) ([]WikiProposalView, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.loads++
	return append([]WikiProposalView(nil), q.props...), nil
}

func (q *queueBackend) decide(_ context.Context, path, id, choice, content string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.decided = append(q.decided, path+"|"+id+"|"+choice+"|"+content)
	for i := range q.props {
		if q.props[i].ID == id {
			switch choice {
			case wikiAccept, wikiEdit:
				q.props[i].Status = "accepted"
			case wikiRefuse:
				q.props = append(q.props[:i], q.props[i+1:]...)
			case wikiPend:
				q.props[i].Status = "pending"
			}
			break
		}
	}
	return nil
}

func (q *queueBackend) got() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.decided...)
}

func newQueueView(q *queueBackend, now time.Time) *ProjectWikiView {
	return NewProjectWikiView(ProjectWikiViewConfig{
		Project: func() (string, string) { return "openhub", "/src/openhub" },
		Metrics: func(context.Context, string) (ProjectWikiMetrics, error) {
			return ProjectWikiMetrics{Pages: 14, Domains: 4, DomainsCovered: 3, Tagged: 100, Confirmed: 61, Uncertain: 9,
				Pending: 2, Accepted: 1, Synced: "0123456789abcdef"}, nil
		},
		Proposals: q.list,
		Decide:    q.decide,
		Now:       func() time.Time { return now },
	})
}

// Golden: the queue of the project outside any checkpoint, by section,
// the detail beside the list, the summary of the metrics above.
func TestProjectWikiQueueGolden(t *testing.T) {
	withFrench(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	q := &queueBackend{props: queueProposals(now)}
	v := newQueueView(q, now)
	sh := &wikiCardShell{}
	v.SetShell(sh)
	app, content, onLoop := runWikiApp(t)
	onLoop(func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n, text := 0, ""
		onLoop(func() { n = v.wiki.list.Len(); text = v.tv.GetText(true) })
		return n == 4 && strings.Contains(text, "Pages 14")
	}, 3*time.Second, 20*time.Millisecond)
	var screen string
	onLoop(func() {
		v.wiki.list.Select("wp-17")
		box := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(v.tv, 2, 0, false).AddItem(v.wiki.list.Primitive(), 0, 1, true)
		screen = screenText(t, box, 120, 18)
	})
	assertGolden(t, "project_wiki_queue", screen)
	assert.Equal(t, "y accepter · n refuser · e modifier · s laisser · A tout accepter · / filtrer · m métriques · r rafraîchir · Échap revenir",
		v.StatusHints())

	// Detail of a stale proposal, of an edited one.
	var detail string
	onLoop(func() { v.wiki.list.Select("wp-9"); detail = v.wiki.list.detail.GetText(true) })
	assert.Contains(t, detail, "Périmée : la ligne remplacée n'est plus dans la page")
	onLoop(func() { v.wiki.list.Select("wp-12"); detail = v.wiki.list.detail.GetText(true) })
	assert.Contains(t, detail, "Contenu modifié par toi")

	// y on a stale proposal does not apply; on wp-18 it accepts, the queue
	// is read again and the proposal moves to « Acceptées ».
	onLoop(func() { v.wiki.list.Select("wp-9"); v.HandleKey(key('y')) })
	assert.Empty(t, q.got())
	onLoop(func() { v.wiki.list.Select("wp-18"); v.HandleKey(key('y')) })
	require.Eventually(t, func() bool {
		sec := ""
		onLoop(func() {
			for _, it := range v.wiki.list.Items() {
				if it.ID == "wp-18" {
					sec = it.Section
				}
			}
		})
		return strings.HasPrefix(sec, "Acceptées")
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, []string{"/src/openhub|wp-18|accept|"}, q.got())

	// n: the prompt confirms; the refused proposal leaves the queue.
	onLoop(func() { v.wiki.list.Select("wp-17"); v.HandleKey(key('n')) })
	require.NotNil(t, sh.answer)
	onLoop(func() { sh.answer("doublon") })
	require.Eventually(t, func() bool { n := 0; onLoop(func() { n = v.wiki.list.Len() }); return n == 3 }, 3*time.Second, 20*time.Millisecond)

	// / filters; m shows every metric.
	onLoop(func() { v.HandleKey(key('/')) })
	onLoop(func() { sh.answer("architecture") })
	n := 0
	onLoop(func() { n = v.wiki.list.Len() })
	assert.Equal(t, 1, n)
	onLoop(func() { v.HandleKey(key('m')) })
	title, body, _ := sh.modal()
	assert.Equal(t, "Wiki du projet · openhub", title)
	assert.Contains(t, body, "75 % (3/4)")
	onLoop(func() { v.Unmount() })
}

func TestProjectWikiSummary(t *testing.T) {
	withFrench(t)
	s := renderProjectWikiSummary(ProjectWikiMetrics{Pages: 3}, nil)
	assert.Contains(t, s, "Pages 3 · jamais synchronisé")
	assert.Contains(t, renderProjectWikiSummary(ProjectWikiMetrics{NoWiki: true}, nil), "onboarding")
	assert.Contains(t, renderProjectWikiSummary(ProjectWikiMetrics{}, errors.New("boom")), "boom")
	assert.Equal(t, "3 min", shortAge(time.Unix(600, 0), time.Unix(420, 0)))
	assert.Equal(t, "", shortAge(time.Now(), time.Time{}))
}
