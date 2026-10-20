package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func TestWikiCheckCommand(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	root := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(root, "docs", "wiki", filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(text), 0o644))
	}
	fm := "---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n"
	write("index.md", fm+"# Atlas\n\n## God nodes\n\n| Concept | Pages liées | Criticité |\n|---|---|---|\n")
	write("a.md", fm+"# A\n\n- `CartService` — `DÉDUIT` · dev · 2026-09-01\n")
	write("b.md", fm+"# B\n\n- `CartService` et [x](none.md) — `CONFIRMÉ` · dev · 2026-09-01 · go.mod\n")
	sub := filepath.Join(root, "docs", "wiki")

	run := func(args ...string) (string, error) {
		cmd := &cobra.Command{RunE: runWikiCheck}
		cmd.Flags().Bool("fix", false, "")
		cmd.Flags().Bool("json", false, "")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetContext(context.Background())
		require.NoError(t, cmd.ParseFlags(args))
		err := cmd.RunE(cmd, cmd.Flags().Args())
		return out.String(), err
	}
	out, err := run(sub)
	assert.True(t, errors.Is(err, errWikiCheck), "broken link: exit non-zero")
	assert.Contains(t, out, "✗ b.md:9 · broken_link · lien vers none.md : page introuvable")
	assert.Contains(t, out, "god_node_missing")
	assert.Contains(t, out, "les tags de la page donnent mixed", "the tags of a.md")
	assert.Contains(t, out, "CartService : a.md, b.md (Normale)")

	out, _ = run("--fix", root)
	assert.Contains(t, out, "Réparé : a.md, index.md")
	assert.NotContains(t, out, "god_node_missing")

	_, err = run(t.TempDir())
	assert.ErrorContains(t, err, "pas de docs/wiki/")
}

func TestWikiPendingHelpers(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	root := filepath.Join(t.TempDir(), "proj")
	id, err := wikiProject(context.Background(), filepath.Join(root, "sub"), func(_ context.Context, p string) (string, error) {
		if p == root {
			return "p1", nil
		}
		return "", domain.ErrNotFound
	})
	require.NoError(t, err)
	assert.Equal(t, "p1", id, "the project above the directory")

	var b bytes.Buffer
	printWikiProposals(&b, []domain.WikiProposal{{Seq: 2, Status: domain.WikiPending, Agent: "reviewer", SessionID: "ses_a", Page: "index.md",
		Section: "Zones d'ombre", Tag: "INCERTAIN", Content: "Rétention des exports\nsuite"}})
	for _, want := range []string{"ID", "AUTEUR", "wp-2", "pending", "reviewer", "ses_a", "index.md § Zones d'ombre", "INCERTAIN", "Rétention des exports"} {
		assert.Contains(t, b.String(), want)
	}
	assert.NotContains(t, b.String(), "suite")
	// p5-fixes: a replacement, before → after.
	b.Reset()
	printWikiProposals(&b, []domain.WikiProposal{{Seq: 3, Status: domain.WikiStale, Page: "technical/stack.md", Tag: "CONFIRMÉ",
		Replaces: "- HTTP dans internal/http", Content: "- HTTP dans internal/api"}})
	assert.Contains(t, b.String(), "« - HTTP dans internal/http » → « - HTTP dans internal/api »")
	assert.Contains(t, b.String(), "stale")
	// p9-wiki-refusal: the reason of a refusal gets a column when there is
	// one; the table is unchanged otherwise.
	assert.NotContains(t, b.String(), "MOTIF")
	b.Reset()
	printWikiProposals(&b, []domain.WikiProposal{
		{Seq: 4, Status: domain.WikiRefused, Page: "index.md", Tag: "DÉDUIT", Content: "un", Reason: "déjà couvert\nsuite"},
		{Seq: 5, Status: domain.WikiPending, Page: "index.md", Tag: "DÉDUIT", Content: "deux"}})
	assert.Contains(t, b.String(), "MOTIF DU REFUS")
	assert.Contains(t, b.String(), "déjà couvert")
	assert.NotContains(t, b.String(), "suite")
	b.Reset()
	printWikiProposals(&b, nil)
	assert.Contains(t, b.String(), "Aucune proposition")
}

// p9-wiki-refusal: the views get the reason of a refusal.
func TestWikiProposalViewReason(t *testing.T) {
	v := wikiProposalView(domain.WikiProposal{Seq: 4, Status: domain.WikiRefused, Page: "index.md", Content: "x", Reason: "doublon"})
	assert.Equal(t, "wp-4", v.ID)
	assert.Equal(t, "doublon", v.Reason)
	assert.Equal(t, "doublon", cardWiki([]domain.WikiProposal{{Seq: 4, Reason: "doublon"}})[0].Reason)
}
