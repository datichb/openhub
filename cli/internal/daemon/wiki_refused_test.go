package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// p9-wiki-refusal follow-up: the agents learn that a proposal was refused,
// and why. wiki_pending gets the recent refusals from /wiki/refused, and
// wiki_propose answers with the refusal a new proposal repeats; the proposal
// is queued all the same.
func TestWikiRefusalsAreVisibleToTheAgents(t *testing.T) {
	d, wk, _, _ := wikiEnv(t)
	h := d.routes()
	ctx := context.Background()
	propose := func(tool, content string) WikiProposal {
		t.Helper()
		var p WikiProposal
		require.Equal(t, http.StatusCreated, wikiDo(t, h, http.MethodPost, "/wiki/propose",
			`{"tool_session":"`+tool+`","page":"technical/stack.md","section":"Librairies clés","content":"`+content+`","tag":"DÉDUIT","source":"package.json"}`, &p))
		return p
	}

	first := propose("ses_rev", "zod pour la validation")
	assert.Nil(t, first.PreviouslyRefused, "nothing refused yet")

	var refused []WikiProposal
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/refused?tool_session=ses_rev", "", &refused))
	assert.Empty(t, refused, "an open proposal is not a refusal")

	_, err := wk.Decide(ctx, "p1", first.ID, wiki.ChoiceRefuse, "on utilise valibot", "oh")
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/refused?tool_session=ses_b", "", &refused))
	require.Len(t, refused, 1, "the refusals of the project, seen from another session")
	assert.Equal(t, first.ID, refused[0].ID)
	assert.Equal(t, domain.WikiRefused, refused[0].Status)
	assert.Equal(t, "on utilise valibot", refused[0].Reason)
	assert.Equal(t, "technical/stack.md", refused[0].Page)
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/refused?tool_session=ses_b&scope=session", "", &refused))
	assert.Empty(t, refused, "the refusals of the session only")
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/refused?tool_session=ses_a&scope=session", "", &refused))
	assert.Len(t, refused, 1)

	// wiki_pending still lists the open proposals only.
	var pending []WikiProposal
	require.Equal(t, http.StatusOK, wikiDo(t, h, http.MethodGet, "/wiki/pending?tool_session=ses_b", "", &pending))
	assert.Empty(t, pending, "the contract of /wiki/pending is unchanged")

	// The same proposal again: queued, with the reason it was refused.
	again := propose("ses_doc", "Zod  pour la validation")
	assert.NotEqual(t, first.ID, again.ID, "queued all the same")
	assert.Equal(t, domain.WikiPending, again.Status)
	require.NotNil(t, again.PreviouslyRefused)
	assert.Equal(t, first.ID, again.PreviouslyRefused.ID)
	assert.Equal(t, "on utilise valibot", again.PreviouslyRefused.Reason)
	assert.NotNil(t, again.PreviouslyRefused.At)

	other := propose("ses_doc", "pino pour les logs")
	assert.Nil(t, other.PreviouslyRefused, "another text")

	// Same rules as the other wiki routes.
	assert.Equal(t, http.StatusBadRequest, wikiDo(t, h, http.MethodGet, "/wiki/refused", "", nil))
}
