package wiki

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

type refusedFixture struct {
	ctx context.Context
	s   *Service
	now time.Time
}

func newRefusedFixture() *refusedFixture {
	f := &refusedFixture{ctx: context.Background(), now: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	f.s = &Service{Store: &memStore{}, Now: func() time.Time { return f.now }}
	return f
}

// propose files a proposal of a session and returns it.
func (f *refusedFixture) propose(t *testing.T, session, page, content string) domain.WikiProposal {
	t.Helper()
	p, err := f.s.Propose(f.ctx, Filed{ProjectID: "p1", SessionID: session, Agent: "reviewer",
		Proposal: Proposal{Page: page, Content: content, Tag: "DÉDUIT"}})
	require.NoError(t, err)
	return p
}

func (f *refusedFixture) refuse(t *testing.T, p domain.WikiProposal, reason string) {
	t.Helper()
	_, err := f.s.Decide(f.ctx, "p1", p.ID(), ChoiceRefuse, reason, "oh")
	require.NoError(t, err)
}

func ids(list []domain.WikiProposal) []string {
	var out []string
	for _, p := range list {
		out = append(out, p.ID())
	}
	return out
}

// The recent refusals: refused, newest first, within the window, bounded.
func TestRefusedListsTheRecentRefusals(t *testing.T) {
	f := newRefusedFixture()
	old := f.propose(t, "ses_a", "index.md", "trop ancienne")
	f.refuse(t, old, "périmée")
	f.now = f.now.Add(RefusedWindow + time.Hour) // the first one falls out of the window
	first := f.propose(t, "ses_a", "index.md", "première")
	other := f.propose(t, "ses_b", "index.md", "d'une autre session")
	f.propose(t, "ses_a", "index.md", "encore ouverte")
	accepted := f.propose(t, "ses_a", "index.md", "acceptée")
	_, err := f.s.Decide(f.ctx, "p1", accepted.ID(), ChoiceAccept, "", "oh")
	require.NoError(t, err)

	f.now = f.now.Add(time.Hour)
	f.refuse(t, first, "déjà dans la page stack.md")
	f.now = f.now.Add(time.Hour)
	f.refuse(t, other, "")

	got, err := f.s.Refused(f.ctx, "p1", "")
	require.NoError(t, err)
	assert.Equal(t, []string{other.ID(), first.ID()}, ids(got), "newest first; not the one out of the window, nor the open ones")
	assert.Equal(t, "déjà dans la page stack.md", got[1].Reason)
	assert.Empty(t, got[0].Reason)

	got, err = f.s.Refused(f.ctx, "p1", "ses_a")
	require.NoError(t, err)
	assert.Equal(t, []string{first.ID()}, ids(got), "the refusals of the session")

	got, err = f.s.Refused(f.ctx, "other-project", "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRefusedKeepsTheLastTwenty(t *testing.T) {
	f := newRefusedFixture()
	var all []domain.WikiProposal
	for i := range RefusedShown + 5 {
		p := f.propose(t, fmt.Sprintf("ses_%d", i), "index.md", fmt.Sprintf("proposition %d", i))
		f.now = f.now.Add(time.Minute)
		f.refuse(t, p, "")
		all = append(all, p)
	}
	got, err := f.s.Refused(f.ctx, "p1", "")
	require.NoError(t, err)
	require.Len(t, got, RefusedShown)
	assert.Equal(t, all[len(all)-1].ID(), got[0].ID(), "the most recent first")
	assert.Equal(t, all[5].ID(), got[RefusedShown-1].ID(), "the 5 oldest are left out")
}

func TestRefusedWithoutStore(t *testing.T) {
	_, err := (&Service{}).Refused(context.Background(), "p1", "")
	assert.ErrorIs(t, err, ErrNoStore)
	_, err = (&Service{}).PriorRefusal(context.Background(), domain.WikiProposal{})
	assert.ErrorIs(t, err, ErrNoStore)
}

// A proposal that repeats a refused one (same page, same text once the case
// and the spaces are set aside) finds it; the section does not matter.
func TestPriorRefusal(t *testing.T) {
	f := newRefusedFixture()
	refused := f.propose(t, "ses_a", "technical/stack.md", "Zod pour la validation")
	f.now = f.now.Add(time.Hour)
	f.refuse(t, refused, "on utilise valibot")

	repeat := func(session, page, content string) *domain.WikiProposal {
		t.Helper()
		p := f.propose(t, session, page, content)
		prior, err := f.s.PriorRefusal(f.ctx, p)
		require.NoError(t, err)
		return prior
	}
	got := repeat("ses_b", "technical/stack.md", "  zod  pour   la validation\n")
	require.NotNil(t, got, "case and spaces aside")
	assert.Equal(t, refused.ID(), got.ID())
	assert.Equal(t, "on utilise valibot", got.Reason)
	assert.NotNil(t, got.DecidedAt)

	assert.Nil(t, repeat("ses_b", "technical/other.md", "Zod pour la validation"), "another page")
	assert.Nil(t, repeat("ses_b", "technical/stack.md", "Zod pour la validation des schémas"), "another text")

	// The twin is the answer of a proposal that is still open: nothing to say.
	open := f.propose(t, "ses_c", "technical/stack.md", "valibot pour la validation")
	prior, err := f.s.PriorRefusal(f.ctx, open)
	require.NoError(t, err)
	assert.Nil(t, prior)
}

// A proposal the user edited before refusing it is found by what the agent
// wrote as well as by what the user left.
func TestPriorRefusalOfAnEditedProposal(t *testing.T) {
	f := newRefusedFixture()
	p := f.propose(t, "ses_a", "index.md", "texte de l'agent")
	_, err := f.s.Decide(f.ctx, "p1", p.ID(), ChoiceEdit, "texte corrigé par l'utilisateur", "oh")
	require.NoError(t, err)
	f.refuse(t, p, "hors sujet")

	again := f.propose(t, "ses_b", "index.md", "Texte de l'agent")
	prior, err := f.s.PriorRefusal(f.ctx, again)
	require.NoError(t, err)
	require.NotNil(t, prior, "the original text is compared")
	assert.Equal(t, p.ID(), prior.ID())

	edited := f.propose(t, "ses_b", "index.md", "texte corrigé par l'utilisateur")
	prior, err = f.s.PriorRefusal(f.ctx, edited)
	require.NoError(t, err)
	require.NotNil(t, prior, "and the edited one")
}

// The most recent refusal is the one given.
func TestPriorRefusalIsTheMostRecent(t *testing.T) {
	f := newRefusedFixture()
	a := f.propose(t, "ses_a", "index.md", "même texte")
	f.refuse(t, a, "premier motif")
	f.now = f.now.Add(time.Hour)
	b := f.propose(t, "ses_b", "index.md", "même texte")
	f.now = f.now.Add(time.Hour)
	f.refuse(t, b, "second motif")
	again := f.propose(t, "ses_c", "index.md", "même texte")
	prior, err := f.s.PriorRefusal(f.ctx, again)
	require.NoError(t, err)
	require.NotNil(t, prior)
	assert.Equal(t, "second motif", prior.Reason)
}

func TestSameText(t *testing.T) {
	assert.True(t, SameText("A  b\nc", "a b c"))
	assert.False(t, SameText("a b", "a b c"))
	assert.True(t, SameText("", "  "))
}
