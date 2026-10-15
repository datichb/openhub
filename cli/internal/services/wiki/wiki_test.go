package wiki

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeWiki writes the pages of a wiki (relative to docs/wiki/) in a new
// project and returns its root.
func writeWiki(t *testing.T, pages map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, text := range pages {
		p := filepath.Join(root, "docs", "wiki", filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(text), 0o644))
	}
	return root
}

const fm = "---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n"

func atlasIndex(rows string) string {
	return fm + "# Atlas — Index Wiki\n\n## Stack critique\nGo.\n— `CONFIRMÉ` · onboarder · 2026-09-12 · go.mod\n\n" +
		"## God nodes — concepts les plus connectés\n\n| Concept | Pages liées | Criticité |\n|---------|-------------|-----------|\n" + rows +
		"\n## Points critiques actifs 🔴\n\n- Le **Panier** perd des lignes — `INCERTAIN` · reviewer · 2026-09-30\n"
}

func TestIndexGodNodes(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex(
			"| Commande | [business/orders.md](business/orders.md), [technical/architecture.md#events](technical/architecture.md#events) | Critique |\n" +
				"| Facturation | [business/billing.md](business/billing.md) | Haute |\n" +
				"| Fantôme | [business/ghost.md](business/ghost.md), [business/orders.md](business/orders.md) | Normale |\n"),
		"business/orders.md": fm + "# Commandes\n\n## Règles\n- Une commande a un `CartService` et un **Panier**.\n— `CONFIRMÉ` · dev · 2026-09-01 · src/a.go:1\n",
		"technical/architecture.md": fm + "# Architecture\n\n## Events\n- Les événements partent du `CartService` vers le **panier**.\n" +
			"— `DÉDUIT` · dev · 2026-09-01\n",
		"business/billing.md":      fm + "# Facturation\n\n- La facturation lit le `CartService`. **Règle :** pas de TVA.\n— `DÉDUIT` · dev · 2026-09-01\n",
		"technical/conventions.md": fm + "# Conventions\n\n- `snake_case` et `JWT` partout, **Règle :** rien.\n— `CONFIRMÉ` · dev · 2026-09-01 · a.go:1\n",
	})
	w, err := Load(root)
	require.NoError(t, err)
	assert.Equal(t, IndexPage, w.Pages[0].Path, "index first")
	ix := w.Index()
	require.True(t, ix.Present)
	assert.Equal(t, "Atlas — Index Wiki", ix.Title)
	require.Len(t, ix.Declared, 3)

	byName := map[string]GodNode{}
	var order []string
	for _, g := range ix.GodNodes {
		byName[g.Concept] = g
		order = append(order, g.Concept)
	}
	// Declared first, in their order; Facturation is cited by billing only
	// (a word is not a page), Fantôme links a missing page.
	assert.Equal(t, []string{"Commande", "CartService", "Panier"}, order)
	cmd := byName["Commande"]
	assert.Equal(t, []string{"business/orders.md", "technical/architecture.md"}, cmd.Pages)
	assert.Equal(t, CriticalityCritical, cmd.Criticality, "declared Critique")
	assert.Equal(t, "| Commande | [business/orders.md](business/orders.md), [technical/architecture.md#events](technical/architecture.md#events) | Critique |",
		cmd.Row(), "same data: the declared row is kept, its anchor included")
	cart := byName["CartService"]
	assert.Equal(t, []string{"business/billing.md", "business/orders.md", "technical/architecture.md"}, cart.Pages)
	assert.Equal(t, CriticalityHigh, cart.Criticality, "three pages")
	assert.Equal(t, CriticalityCritical, byName["Panier"].Criticality, "named in the critical points")
	assert.NotContains(t, order, "JWT", "not camel case")
	assert.NotContains(t, order, "Règle", "a bold label")

	table := ix.GodNodeTable(2)
	require.Len(t, table, 4)
	assert.Equal(t, "| Concept | Pages liées | Criticité |", table[0])
	assert.Equal(t, "| CartService | [business/billing.md](business/billing.md), [business/orders.md](business/orders.md), [technical/architecture.md](technical/architecture.md) | Haute |", table[3])

	// Without index: no index; without docs/wiki: ErrNoWiki.
	_, err = Load(t.TempDir())
	require.ErrorIs(t, err, ErrNoWiki)
	w2, err := Load(writeWiki(t, map[string]string{"a.md": fm + "# A\n"}))
	require.NoError(t, err)
	assert.False(t, w2.Index().Present)
}

func TestGodNodeRowChanges(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md":            atlasIndex("| Commande | [business/orders.md](business/orders.md) | Critique |\n"),
		"business/orders.md":  fm + "# Commandes\n\n- Commande.\n— `DÉDUIT` · dev · 2026-09-01\n",
		"business/billing.md": fm + "# Facturation\n\n- Une commande est facturée.\n— `DÉDUIT` · dev · 2026-09-01\n",
	})
	w, err := Load(root)
	require.NoError(t, err)
	ix := w.Index()
	require.Len(t, ix.GodNodes, 1)
	assert.Equal(t, "| Commande | [business/billing.md](business/billing.md), [business/orders.md](business/orders.md) | Critique |", ix.GodNodes[0].Row(),
		"a page more: the row is computed again, its declared link kept")
}

// A declared row whose concept no page names any more (renamed, accented)
// is stale even when the pages it links still exist: the table alone does
// not keep it (p8-fixes: "Contenu embarque" stayed after "Contenu embarqué").
func TestGodNodeDeclaredOnlyByTable(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex("| Contenu embarque | [technical/architecture.md](technical/architecture.md), [technical/stack.md](technical/stack.md) | Normale |\n" +
			"| Commande | [business/orders.md](business/orders.md), [technical/stack.md](technical/stack.md) | Normale |\n"),
		"technical/architecture.md": fm + "# Architecture\n\n- Le **Contenu embarqué** est copié.\n— `DÉDUIT` · dev · 2026-09-01\n",
		"technical/stack.md":        fm + "# Stack\n\n- Go et le contenu embarqué.\n— `DÉDUIT` · dev · 2026-09-01\n",
		"business/orders.md":        fm + "# Commandes\n\n- Une commande.\n— `DÉDUIT` · dev · 2026-09-01\n",
	})
	w, err := Load(root)
	require.NoError(t, err)
	var order []string
	for _, g := range w.Index().GodNodes {
		order = append(order, g.Concept)
	}
	assert.Equal(t, []string{"Commande", "Contenu embarqué"}, order, "a page still names Commande: its declared link counts")

	rep, err := Check(context.Background(), root, nil)
	require.NoError(t, err)
	stale := 0
	for _, i := range rep.Issues {
		if i.Kind == IssueGodNodeStale {
			stale++
			assert.Contains(t, i.Message, "Contenu embarque")
		}
	}
	assert.Equal(t, 1, stale, "%+v", rep.Issues)

	_, err = Fix(root, time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	index, _ := ReadPage(root, IndexPage, "")
	assert.NotContains(t, index, "| Contenu embarque |")
	assert.Contains(t, index, "| Contenu embarqué | [technical/architecture.md](technical/architecture.md), [technical/stack.md](technical/stack.md) | Normale |")
	assert.Contains(t, index, "| Commande | [business/orders.md](business/orders.md), [technical/stack.md](technical/stack.md) | Normale |", "kept as written")
}

func TestReadPage(t *testing.T) {
	root := writeWiki(t, map[string]string{"technical/stack.md": fm + "# Stack\n\n## Go\n- 1.26\n\n## Web\n- React\n"})
	text, err := ReadPage(root, "docs/wiki/technical/stack.md", "")
	require.NoError(t, err)
	assert.Contains(t, text, "React")
	part, err := ReadPage(root, "technical/stack.md", "go")
	require.NoError(t, err)
	assert.Equal(t, "## Go\n- 1.26", part)
	_, err = ReadPage(root, "technical/stack.md", "Rust")
	require.ErrorIs(t, err, ErrNoSection)
	assert.Contains(t, err.Error(), "Go · Web")
	_, err = ReadPage(root, "technical/none.md", "")
	require.ErrorIs(t, err, ErrNoPage)
	for _, bad := range []string{"../secret.md", "/etc/passwd.md", "a/../../b.md", ".hidden/a.md", "a.txt", "*.md", "", "c:x.md"} {
		_, err := ReadPage(root, bad, "")
		require.ErrorIs(t, err, ErrBadPage, bad)
	}
	// A symbolic link is never followed.
	outside := filepath.Join(t.TempDir(), "secret.md")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "docs", "wiki", "link.md")))
	_, err = ReadPage(root, "link.md", "")
	require.ErrorIs(t, err, ErrSymlink)
	w, err := Load(root)
	require.NoError(t, err)
	assert.Equal(t, []string{"link.md"}, w.Skipped)
}

func TestCheck(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex("| Fantôme | [business/ghost.md](business/ghost.md) | Normale |\n"),
		"business/orders.md": "---\nupdated: 2026-09-01\nconfidence: confirmed\nagents: [dev]\n---\n\n# Commandes\n\n## Règles\n" +
			"- Le `CartService` valide.\n— `DÉDUIT` · dev · 2026-09-01 · src/cart.go:12\n" +
			"- Voir [archi](../technical/architecture.md#events) et [absent](../technical/none.md).\n" +
			"- Ancre [locale](#règles), [perdue](#nulle-part).\n" +
			"- Code ignoré dans `[a](b.md)`.\n" +
			"- Le fichier [main](../../../src/main.go).\n" +
			"- Mal taggé — `CONFIRMÉ` · dev\n" +
			"- Sans source — `CONFIRMÉ` · dev · 2026-09-01\n" +
			"- Absent — `INCERTAIN` · dev · 2026-09-01 · src/gone.go\n",
		"technical/architecture.md": "---\nupdated: hier\nconfidence: sure\nagents: []\n---\n\n# Architecture\n\n## Events\n- Le `CartService` publie.\n",
		"technical/raw.md":          "# Sans frontmatter\n",
		"technical/bad.md":          "---\nupdated: [\n---\n# Bad\n",
	})
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	for _, f := range []string{"src/cart.go", "src/main.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("package x\n"), 0o644))
	}
	modified := func(_ context.Context, root, file string) (time.Time, bool) {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			return time.Time{}, false
		}
		if file == "src/cart.go" {
			return time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC), true
		}
		return time.Date(2026, 9, 1, 23, 0, 0, 0, time.UTC), true // same day: fresh
	}
	rep, err := Check(context.Background(), root, modified)
	require.NoError(t, err)
	kinds := map[string][]Issue{}
	for _, i := range rep.Issues {
		kinds[i.Kind] = append(kinds[i.Kind], i)
	}
	has := func(kind, page, text string) {
		t.Helper()
		for _, i := range kinds[kind] {
			if i.Page == page && strings.Contains(i.Message, text) {
				return
			}
		}
		t.Errorf("no %s issue on %s with %q in %+v", kind, page, text, kinds[kind])
	}
	has(IssueFrontmatter, "technical/raw.md", "")
	has(IssueFrontmatter, "technical/bad.md", "")
	has(IssueUpdated, "technical/architecture.md", "hier")
	has(IssueConfidence, "technical/architecture.md", "sure")
	has(IssueAgents, "technical/architecture.md", "")
	has(IssueUntagged, "technical/architecture.md", "")
	has(IssueConfidenceMismatch, "business/orders.md", "uncertain")
	has(IssueBrokenLink, "business/orders.md", "../technical/none.md")
	has(IssueBrokenAnchor, "business/orders.md", "#nulle-part")
	has(IssueTag, "business/orders.md", "")
	has(IssueTagSource, "business/orders.md", "")
	has(IssueStale, "business/orders.md", "src/cart.go")
	has(IssueCitedMissing, "business/orders.md", "src/gone.go")
	has(IssueGodNodeStale, IndexPage, "Fantôme")
	has(IssueGodNodeMissing, IndexPage, "CartService")
	has(IssueBrokenLink, IndexPage, "business/ghost.md")
	assert.Len(t, kinds[IssueBrokenLink], 2, "the anchor of an existing page, links in code and the project file are fine: %+v", kinds[IssueBrokenLink])
	assert.Len(t, kinds[IssueBrokenAnchor], 1, "#règles exists: %+v", kinds[IssueBrokenAnchor])
	assert.Len(t, kinds[IssueStale], 1, "src/main.go changed the day of the page: %+v", kinds[IssueStale])
	assert.Positive(t, rep.Errors())

	// Fix: confidence and god nodes table.
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	res, err := Fix(root, now)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"business/orders.md", IndexPage}, res.Pages)
	orders, _ := ReadPage(root, "business/orders.md", "")
	assert.Contains(t, orders, "confidence: uncertain")
	index, _ := ReadPage(root, IndexPage, "")
	assert.Contains(t, index, "updated: 2026-10-13")
	assert.Contains(t, index, "| CartService | [business/orders.md](business/orders.md), [technical/architecture.md](technical/architecture.md) | Normale |")
	assert.NotContains(t, index, "Fantôme")
	assert.Contains(t, index, "## Points critiques actifs 🔴", "the rest of the index stays")
	rep, err = Check(context.Background(), root, modified)
	require.NoError(t, err)
	for _, i := range rep.Issues {
		assert.NotContains(t, []string{IssueGodNodeMissing, IssueGodNodeStale, IssueGodNodeChanged, IssueConfidenceMismatch}, i.Kind, "fixed: %+v", i)
	}
	again, err := Fix(root, now)
	require.NoError(t, err)
	assert.Empty(t, again.Pages, "nothing left to fix")
}

func TestCheckIndexMissing(t *testing.T) {
	rep, err := Check(context.Background(), writeWiki(t, map[string]string{"a.md": fm + "# A\n- x — `DÉDUIT` · dev · 2026-09-01\n"}), nil)
	require.NoError(t, err)
	require.NotEmpty(t, rep.Issues)
	assert.Equal(t, IssueNoIndex, rep.Issues[0].Kind)
}
