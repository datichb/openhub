package wiki

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// p5-fixes: a proposal may replace a line of a page (the exact old line
// and the new one): cleaned like an addition, the old line checked when it
// is filed and when it is applied (else stale, not applied), written in
// place with its tag line.

const replacePage = "---\nupdated: 2026-09-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Stack\n\n" +
	"## Paquets\n- Le code HTTP est dans `internal/http`.\n  — `CONFIRMÉ` · onboarder · 2026-09-01 · internal/http/server.go:1\n- Logs : slog.\n  — `CONFIRMÉ` · onboarder · 2026-09-01 · go.mod\n\n" +
	"## Notes\nGo 1.22.\n— `CONFIRMÉ` · onboarder · 2026-09-01 · go.mod\n- Logs : slog.\n"

func TestCleanReplacement(t *testing.T) {
	p, err := CleanProposal(Proposal{Page: "technical/stack.md", Content: "- Le code HTTP est dans `internal/api`.", Tag: "CONFIRMÉ", Source: "internal/api/server.go:1",
		Replaces: "  - Le code HTTP est dans `internal/http`.  "})
	require.NoError(t, err)
	assert.Equal(t, "- Le code HTTP est dans `internal/http`.", p.Replaces, "spaces around the line removed")

	for in, want := range map[Proposal]error{
		{Page: "index.md", Content: "x", Tag: "DÉDUIT", Replaces: "a\nb"}:                          ErrReplaceLine,
		{Page: "index.md", Content: "x", Tag: "DÉDUIT", Replaces: "## Paquets"}:                    ErrReplaceStructure,
		{Page: "index.md", Content: "x", Tag: "DÉDUIT", Replaces: "---"}:                           ErrReplaceStructure,
		{Page: "index.md", Content: "x", Tag: "DÉDUIT", Replaces: "```go"}:                         ErrReplaceStructure,
		{Page: "index.md", Content: "x\ny", Tag: "DÉDUIT", Replaces: "- a"}:                        ErrReplaceContent,
		{Page: "index.md", Content: "- a", Tag: "DÉDUIT", Replaces: "- a"}:                         ErrReplaceSame,
		{Page: "index.md", Content: "x", Tag: "DÉDUIT", Replaces: "— `CONFIRMÉ` · a · 2026-01-01"}: ErrReplaceStructure,
	} {
		_, err := CleanProposal(in)
		require.ErrorIs(t, err, want, "%+v", in)
	}
	// The new line is neutralised like an addition.
	p, err = CleanProposal(Proposal{Page: "index.md", Content: "# <b>titre</b>", Tag: "DÉDUIT", Replaces: "- a"})
	require.NoError(t, err)
	assert.Equal(t, "\\# &lt;b>titre&lt;/b>", p.Content)
}

func TestApplyReplacement(t *testing.T) {
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	root := writeWiki(t, map[string]string{"index.md": atlasIndex(""), "technical/stack.md": replacePage})

	require.NoError(t, Apply(root, Entry{Page: "technical/stack.md", Content: "Le code HTTP est dans `internal/api`.", Tag: TagConfirmed,
		Source: "internal/api/server.go:1", Agent: "onboarder", Date: at, Replaces: "- Le code HTTP est dans `internal/http`."}))
	text, _ := ReadPage(root, "technical/stack.md", "")
	assert.Contains(t, text, "## Paquets\n- Le code HTTP est dans `internal/api`.\n  — `CONFIRMÉ` · onboarder · 2026-10-14 · internal/api/server.go:1\n- Logs : slog.\n")
	assert.NotContains(t, text, "internal/http")
	assert.Contains(t, text, "updated: 2026-10-14")

	// A line that is no longer a list item keeps its form, its tag line goes.
	require.NoError(t, Apply(root, Entry{Page: "technical/stack.md", Section: "Notes", Content: "Go 1.23.", Tag: TagConfirmed, Source: "go.mod",
		Agent: "onboarder", Date: at, Replaces: "Go 1.22."}))
	text, _ = ReadPage(root, "technical/stack.md", "")
	assert.Contains(t, text, "## Notes\nGo 1.23.\n— `CONFIRMÉ` · onboarder · 2026-10-14 · go.mod\n- Logs : slog.")

	// Twice in the page: the section decides; without it, stale.
	err := Apply(root, Entry{Page: "technical/stack.md", Content: "Logs : zap.", Tag: TagDeduced, Agent: "onboarder", Date: at, Replaces: "- Logs : slog."})
	require.ErrorIs(t, err, ErrStale)
	require.NoError(t, Apply(root, Entry{Page: "technical/stack.md", Section: "Notes", Content: "Logs : zap.", Tag: TagDeduced, Agent: "onboarder", Date: at, Replaces: "- Logs : slog."}))
	text, _ = ReadPage(root, "technical/stack.md", "")
	assert.Equal(t, 1, strings.Count(text, "- Logs : slog."), "the line of Paquets stays")
	assert.Contains(t, text, "- Logs : zap.\n  — `DÉDUIT` · onboarder · 2026-10-14")

	require.ErrorIs(t, Apply(root, Entry{Page: "technical/stack.md", Content: "x", Tag: TagDeduced, Date: at, Replaces: "- absente"}), ErrStale)
	require.ErrorIs(t, Apply(root, Entry{Page: "technical/gone.md", Content: "x", Tag: TagDeduced, Date: at, Replaces: "- a"}), ErrStale, "no page is created")
	require.ErrorIs(t, Apply(root, Entry{Page: "technical/stack.md", Section: "Absente", Content: "x", Tag: TagDeduced, Date: at, Replaces: "Go 1.23."}), ErrStale)
}

func TestServiceReplacement(t *testing.T) {
	ctx := context.Background()
	root := writeWiki(t, map[string]string{"index.md": atlasIndex(""), "technical/stack.md": replacePage})
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	s := &Service{Store: &memStore{}, Now: func() time.Time { return at }}
	f := Filed{ProjectID: "p1", SessionID: "ses_o", Agent: "onboarder", Root: root,
		Proposal: Proposal{Page: "technical/stack.md", Content: "- Le code HTTP est dans `internal/api`.", Tag: "CONFIRMÉ", Source: "internal/api/server.go:1",
			Replaces: "- Le code HTTP est dans `internal/http`."}}

	// Checked when filed.
	_, err := s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_o", Root: root,
		Proposal: Proposal{Page: "technical/stack.md", Content: "x", Tag: "DÉDUIT", Replaces: "- absente"}})
	require.ErrorIs(t, err, ErrStale)
	_, err = s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_o", Root: root,
		Proposal: Proposal{Page: "technical/stack.md", Content: "x", Tag: "DÉDUIT", Replaces: "- Logs : slog."}})
	require.ErrorIs(t, err, ErrStale, "twice in the page")
	p1, err := s.Propose(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, "- Le code HTTP est dans `internal/http`.", p1.Replaces)
	again, err := s.Propose(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, p1.ID(), again.ID())
	f.Replaces = ""
	add, err := s.Propose(ctx, f)
	require.NoError(t, err)
	assert.NotEqual(t, p1.ID(), add.ID(), "an addition of the same content is another proposal")

	// The same line replaced twice: the second is stale when applied.
	f.Replaces, f.Content = "- Le code HTTP est dans `internal/http`.", "- Le code HTTP est dans `internal/rest`."
	p2, err := s.Propose(ctx, f)
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", p1.ID(), ChoiceEdit, "- Le code HTTP est dans `internal/api` (v2).", "oh")
	require.NoError(t, err, "an edit keeps the replacement")
	for _, id := range []string{p2.ID()} {
		_, err = s.Decide(ctx, "p1", id, ChoiceAccept, "", "oh")
		require.NoError(t, err)
	}
	res, err := s.ApplyAccepted(ctx, root, "p1", "ses_o")
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Empty(t, res[0].Err)
	assert.Contains(t, res[1].Err, ErrStale.Error())
	got, _ := s.Get(ctx, "p1", p2.ID())
	assert.Equal(t, domain.WikiStale, got.Status)
	assert.False(t, got.Open(), "stale: closed, never applied")
	text, _ := ReadPage(root, "technical/stack.md", "")
	assert.Contains(t, text, "- Le code HTTP est dans `internal/api` (v2).\n  — `CONFIRMÉ` · onboarder · 2026-10-14")
	assert.NotContains(t, text, "internal/rest")
}
