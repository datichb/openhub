package wiki

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	wikisvc "github.com/datichb/openhub/cli/internal/services/wiki"
)

// refusingBackend is a daemon that knows the refusals of the user.
type refusingBackend struct {
	*fakeBackend
	refused    []daemon.WikiProposal
	refusedErr error
	asked      []bool // sessionOnly of each call
	prior      *daemon.WikiRefusal
}

func (r *refusingBackend) WikiRefused(_ context.Context, _ string, only bool) ([]daemon.WikiProposal, error) {
	r.asked = append(r.asked, only)
	return r.refused, r.refusedErr
}

func (r *refusingBackend) WikiPropose(ctx context.Context, req daemon.WikiProposeRequest) (daemon.WikiProposal, error) {
	p, err := r.fakeBackend.WikiPropose(ctx, req)
	p.PreviouslyRefused = r.prior
	return p, err
}

func refusal(seq int, reason string) daemon.WikiProposal {
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	return daemon.WikiProposal{ID: "wp-" + string(rune('0'+seq)), WikiProposal: domain.WikiProposal{Seq: seq, Agent: "reviewer", Page: "technical/stack.md",
		Tag: "DÉDUIT", Status: domain.WikiRefused, Content: "zod pour la validation\nsuite", Reason: reason, DecidedAt: &at}}
}

// wiki_pending lists the open proposals as before, then the refused ones with
// their reason, so that the agents do not propose them again.
func TestPendingListsTheRecentRefusals(t *testing.T) {
	i18n.SetLocale("fr")
	b := &refusingBackend{fakeBackend: &fakeBackend{}, refused: []daemon.WikiProposal{refusal(7, "on utilise valibot"), refusal(5, "")}}
	out := call(t, b, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	txt := out.Result.Content[0].Text
	lines := strings.Split(txt, "\n")
	assert.Contains(t, lines[0], "wp-1 · pending · auditor", "the open proposals come first, unchanged")
	assert.Contains(t, txt, "Refusées récemment (2 affichées, 30 derniers jours)")
	assert.Contains(t, txt, "Ne les propose pas de nouveau telles quelles")
	assert.Contains(t, txt, "- wp-7 · refused · reviewer · technical/stack.md · DÉDUIT · « zod pour la validation » · motif du refus : on utilise valibot")
	assert.Contains(t, txt, "- wp-5 · refused · reviewer · technical/stack.md · DÉDUIT · « zod pour la validation »")
	assert.NotContains(t, lines[len(lines)-1], "motif", "no reason: nothing added")
	assert.Equal(t, []bool{false}, b.asked)

	// Only the session: the same scope for the refusals.
	out = call(t, b, "tools/call", `{"name":"pending","arguments":{"session_only":true},`+meta+`}`)
	txt = out.Result.Content[0].Text
	assert.True(t, strings.HasPrefix(txt, "Aucune proposition en attente.\n"), txt)
	assert.Contains(t, txt, "wp-7 · refused")
	assert.Equal(t, []bool{false, true}, b.asked)
}

func TestPendingWithoutRefusalsIsUnchanged(t *testing.T) {
	i18n.SetLocale("fr")
	plain := call(t, &fakeBackend{}, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`).Result.Content[0].Text
	b := &refusingBackend{fakeBackend: &fakeBackend{}}
	got := call(t, b, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`).Result.Content[0].Text
	assert.Equal(t, plain, got, "nothing refused: the answer of before")
	assert.NotContains(t, got, "Refusées")
}

// A daemon too old to have the route, or an error on it, does not break
// wiki_pending.
func TestPendingSurvivesAnOldDaemon(t *testing.T) {
	i18n.SetLocale("fr")
	plain := call(t, &fakeBackend{}, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`).Result.Content[0].Text
	for _, err := range []error{&daemon.APIError{Status: http.StatusNotFound, Message: "404 page not found"}, daemon.ErrNotRunning, context.DeadlineExceeded} {
		b := &refusingBackend{fakeBackend: &fakeBackend{}, refused: []daemon.WikiProposal{refusal(7, "x")}, refusedErr: err}
		out := call(t, b, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`)
		require.False(t, out.Result.IsError, out.Result.Content)
		assert.Equal(t, plain, out.Result.Content[0].Text, "%v", err)
	}
}

// wiki_propose queues the proposal and warns when it repeats a refused one:
// the reason is given, the agent decides.
func TestProposeWarnsAboutAPriorRefusal(t *testing.T) {
	i18n.SetLocale("fr")
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	b := &refusingBackend{fakeBackend: &fakeBackend{}, prior: &daemon.WikiRefusal{ID: "wp-7", Reason: "on utilise valibot", At: &at}}
	out := call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"technical/stack.md","content":"zod","tag":"DÉDUIT"},`+meta+`}`)
	require.False(t, out.Result.IsError, "a warning, not a refusal")
	txt := out.Result.Content[0].Text
	lines := strings.Split(txt, "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "wp-4 en file", "queued as before")
	assert.Contains(t, lines[1], "Attention : cette proposition reprend wp-7, que l'utilisateur a refusée")
	assert.Contains(t, lines[1], "motif du refus : on utilise valibot")
	assert.Contains(t, lines[1], "Elle est mise en file malgré tout")
	assert.Len(t, b.proposed, 1)

	b.prior.Reason = ""
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"technical/stack.md","content":"zod","tag":"DÉDUIT"},`+meta+`}`)
	assert.Contains(t, out.Result.Content[0].Text, "wp-7, que l'utilisateur a refusée (sans motif donné)")

	b.prior = nil
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"technical/stack.md","content":"zod","tag":"DÉDUIT"},`+meta+`}`)
	assert.NotContains(t, out.Result.Content[0].Text, "Attention")
	assert.Len(t, strings.Split(out.Result.Content[0].Text, "\n"), 1)
}

func TestRefusalTextsInBothLanguages(t *testing.T) {
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		for _, k := range []string{"cmd.mcp.wiki.refused_header", "cmd.mcp.wiki.refused_before", "cmd.mcp.wiki.refused_before_noreason", "cmd.mcp.wiki.pending_desc", "cmd.mcp.wiki.propose_desc"} {
			assert.NotEqual(t, k, i18n.T(k), lang)
		}
		// The tool descriptions, which are all the agents are told, say it.
		assert.Contains(t, i18n.T("cmd.mcp.wiki.pending_desc"), "30", lang)
		assert.Contains(t, i18n.T("cmd.mcp.wiki.propose_desc"), "refus", lang+" (fr: refusée, en: refused)")
	}
	assert.Equal(t, 30, int(wikisvc.RefusedWindow.Hours()/24), "the descriptions say 30 days")
}
