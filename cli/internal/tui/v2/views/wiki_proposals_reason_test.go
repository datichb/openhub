package views

import (
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// p9-wiki-refusal: a refused proposal shows the reason of the refusal in
// its detail (golden), escaped; without a reason nothing is added, and a
// proposal not refused shows none.
func TestWikiRefusedReasonGolden(t *testing.T) {
	withFrench(t)
	be := &wikiSessions{}
	_, cw := newTestCheckpointWiki(t, be, be, &wikiCardShell{})
	list := nineProposals()
	list[2].Status, list[2].Reason = "refused", "déjà couvert par la page [red]stack[-]"
	list[3].Status = "refused"
	list[4].Reason = "ne doit pas s'afficher"
	cw.wiki.set(list, nil)

	cw.wiki.list.Select("wp-3")
	golden.Assert(t, "checkpoint_wiki_refused_reason", screenText(t, cw.wiki.list.Primitive(), 90, 18))
	detail := cw.wiki.list.detail.GetText(true)
	assert.Contains(t, detail, "Motif du refus : déjà couvert par la page [red]stack[-]")

	cw.wiki.list.Select("wp-4")
	assert.NotContains(t, cw.wiki.list.detail.GetText(true), "Motif du refus", "refused without a word")
	cw.wiki.list.Select("wp-5")
	assert.NotContains(t, cw.wiki.list.detail.GetText(true), "Motif du refus", "not refused")
}

// n asks for the reason: what is typed goes to the backend and shows on the
// refused proposal; an empty answer still refuses, as before.
func TestCheckpointWikiRefusalReason(t *testing.T) {
	withFrench(t)
	be := &wikiSessions{}
	sh := &wikiCardShell{}
	v, cw := newTestCheckpointWiki(t, be, be, sh)
	app, content, onLoop := runWikiApp(t)
	onLoop(func() { v.Mount(content, app); v.rows = checkpointRows(); v.render() })
	onLoop(func() { cw.Mount(tview.NewFlex(), app) })

	onLoop(func() { cw.wiki.list.Select("wp-3"); cw.HandleKey(key('n')) })
	require.Len(t, sh.inputs, 1)
	onLoop(func() { sh.answer("  trop   vague \n") })
	require.Eventually(t, func() bool { return len(be.got()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "ses_a/wp-3/refuse/trop   vague", be.got()[0], "the service cleans the text; the view passes it trimmed")
	var p WikiProposalView
	require.Eventually(t, func() bool {
		onLoop(func() { p, _ = cw.wiki.prop("wp-3") })
		return p.Status == "refused"
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "trop vague", p.Reason)
	var detail string
	onLoop(func() { cw.wiki.list.Select("wp-3"); detail = cw.wiki.list.detail.GetText(true) })
	assert.Contains(t, detail, "Motif du refus : trop vague")

	onLoop(func() { cw.wiki.list.Select("wp-5"); cw.HandleKey(key('n')) })
	onLoop(func() { sh.answer("") })
	require.Eventually(t, func() bool { return len(be.got()) == 2 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "ses_a/wp-5/refuse/", be.got()[1], "an empty reason refuses all the same")
	onLoop(func() { p, _ = cw.wiki.prop("wp-5") })
	assert.Equal(t, "refused", p.Status)
	assert.Empty(t, p.Reason)
	onLoop(func() { cw.Unmount(); v.Unmount() })
}

// The queue of the project passes the reason as well.
func TestProjectWikiQueueRefusalReason(t *testing.T) {
	withFrench(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	q := &queueBackend{props: queueProposals(now)}
	v := newQueueView(q, now)
	sh := &wikiCardShell{}
	v.SetShell(sh)
	app, content, onLoop := runWikiApp(t)
	onLoop(func() { v.Mount(content, app) })
	require.Eventually(t, func() bool { n := 0; onLoop(func() { n = v.wiki.list.Len() }); return n == 4 }, 3*time.Second, 20*time.Millisecond)
	onLoop(func() { v.wiki.list.Select("wp-17"); v.HandleKey(key('n')) })
	onLoop(func() { sh.answer("doublon avec wp-12") })
	require.Eventually(t, func() bool { return len(q.got()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, []string{"/src/openhub|wp-17|refuse|doublon avec wp-12"}, q.got())
	onLoop(func() { v.Unmount() })
}
