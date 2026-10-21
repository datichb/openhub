package wiki

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func frLocale(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

func godNames(w *Wiki) []string {
	var out []string
	for _, g := range w.Index().GodNodes {
		out = append(out, g.Concept)
	}
	return out
}

// LD-J calibration: labels, emphasis words, product names and the
// metadata of the tags make no god node; the maps of the wiki cite none.
func TestGodNodeNoise(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex(""),
		"technical/stack.md": fm + "# Stack\n\n- **Important** : `TypeScript` partout, **jamais** de `JavaScript`.\n" +
			"— `CONFIRMÉ` · developer · 2026-09-01 · package.json\n- **Note** : `GitHub` pour la CI. **Règles**\n",
		"technical/architecture.md": fm + "# Architecture\n\n- **Note :** le `AuthService` signe le **jeton**. **Important** ; `TypeScript` aussi.\n" +
			"— `CONFIRMÉ` · developer · 2026-09-01 · src/auth/AuthService.ts:3\n" +
			"- Les dépôts suivent `OrderRepo`. **Jamais** de requête directe. **Fichiers** : voir **Ce point.**\n— `DÉDUIT` · developer · 2026-09-01\n",
		"business/accounts.md": fm + "# Comptes\n\n- Le `AuthService` émet le jeton ; **Important**, voir [AuthService](../../src/AuthService.md). " +
			"`GitHub`, **Règles**.\n— `DÉDUIT` · developer · 2026-09-01 · src/OrderRepo.go:1\n",
		"business/index.md": fm + "# Domaines\n\n- **Compte client** — [comptes](accounts.md), `OrderRepo`.\n",
		"glossary.md":       fm + "# Glossaire\n\n- **Jeton** : la preuve d'identité. **Compte client** : un compte, `OrderRepo`.\n",
	})
	w, err := Load(root)
	require.NoError(t, err)
	names := godNames(w)
	assert.ElementsMatch(t, []string{"AuthService", "Jeton"}, names,
		"AuthService and the glossary term jeton; not OrderRepo (a source, a map), Compte client (maps only), labels, emphasis nor products")

	// A real concept named by two pages stays, whatever its case.
	assert.True(t, boldConcept("Panier", " dans la page", false))
	assert.False(t, boldConcept("Fichiers", " : liste", false), "a label")
	assert.True(t, boldConcept("Panier", " : un panier", true), "a glossary term")
	assert.False(t, boldConcept("⚠️ Attention", "", false), "an emphasis word behind an emoji")
	assert.False(t, boldConcept("Ce point.", "", false), "a sentence")
	assert.False(t, boldConcept("2026", "", false), "no letter")
}

func TestPageKindAndLayout(t *testing.T) {
	for rel, want := range map[string]string{
		"index.md": KindIndex, "glossary.md": KindGlossary, "technical/stack.md": KindTechnical, "business/a.md": KindBusiness,
		"specs/panier.md": KindSpec, "decisions/adr-001.md": KindDecision, RejectedDecisionsPage: KindDecision,
		"postmortems/2026-10-01-panne.md": KindPostmortem, "retros/2026-10-14-semaine.md": KindRetro, "notes.md": KindOther, "misc/x.md": KindOther,
	} {
		assert.Equal(t, want, PageKind(rel), rel)
	}
	frLocale(t)
	root := writeWiki(t, map[string]string{
		"index.md":                     atlasIndex(""),
		"glossary.md":                  fm + "# Glossaire\n\n- **Panier** : les lignes d'une commande.\n— `DÉDUIT` · onboarder · 2026-09-01\n",
		"specs/panier.md":              fm + "# Spec du panier\n\n## Objectif\nGarder les lignes.\n",
		"decisions/adr-001-sqlite.md":  fm + "# ADR-001 SQLite\n\nStatut : accepté.\n",
		RejectedDecisionsPage:          fm + "# Décisions rejetées\n\n## Redis — EXPIRED\n",
		"postmortems/2026-10-01-a.md":  fm + "# Panne du 1er octobre\n\n## Chronologie\n",
		"misc/notes.md":                fm + "# Notes\n\n- x — `DÉDUIT` · dev · 2026-09-01\n",
		"technical/conventions.md":     fm + "# Conventions\n\nSans tag.\n",
		"business/index.md":            fm + "# Domaines\n",
		"business/orders.md":           fm + "# Commandes\n\n- x — `DÉDUIT` · dev · 2026-09-01\n",
		"technical/review-rules.md":    fm + "# Règles\n\n- y — `DÉDUIT` · dev · 2026-09-01\n",
		"specs/sub/panier-detaille.md": fm + "# Détail\n\nTexte.\n",
	})
	rep, err := Check(context.Background(), root, nil)
	require.NoError(t, err)
	var layout, untagged []string
	for _, is := range rep.Issues {
		switch is.Kind {
		case IssueLayout:
			layout = append(layout, is.Page)
		case IssueUntagged:
			untagged = append(untagged, is.Page)
		}
	}
	assert.Equal(t, []string{"misc/notes.md"}, layout)
	assert.ElementsMatch(t, []string{"technical/conventions.md", "business/index.md"}, untagged,
		"specs, decisions and postmortems record decisions: no tag needed")
}

func writeFile(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(text), 0o644))
}

const mixed = "---\nupdated: 2026-10-01\nconfidence: mixed\nagents: [onboarder]\n---\n\n"

func driftWiki(t *testing.T) string {
	t.Helper()
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex("| AuthService | [technical/architecture.md](technical/architecture.md), [business/accounts.md](business/accounts.md) | Haute |\n"),
		"technical/architecture.md": mixed + "# Architecture\n\n- Le `AuthService` signe les jetons.\n— `DÉDUIT` · dev · 2026-09-01 · src/auth/auth_service.go\n" +
			"- Le panier est recalculé à chaque ligne.\n  — `CONFIRMÉ` · dev · 2026-09-01 · src/cart.go:3\n",
		"business/accounts.md": mixed + "# Comptes\n\n- Un compte passe par `AuthService`.\n— `DÉDUIT` · dev · 2026-09-01\n",
		"business/orders.md":   fm + "# Commandes\n\n- Voir [le panier](../../../src/cart.go).\n— `CONFIRMÉ` · dev · 2026-09-01 · src/order.go:1\n",
		"business/notes.md":    mixed + "# Notes\n\n- Le panier — `DÉDUIT` · dev · 2026-09-01 · src/cart.go\n",
	})
	for _, f := range []string{"src/cart.go", "src/order.go", "src/auth/auth_service.go"} {
		writeFile(t, root, f, "package x\n")
	}
	return root
}

func issuesOf(rep CheckReport, kinds ...string) map[string][]Issue {
	out := map[string][]Issue{}
	for _, is := range rep.Issues {
		for _, k := range kinds {
			if is.Kind == k {
				out[k] = append(out[k], is)
			}
		}
	}
	return out
}

// LD-H: restricted to the files a change touched, wiki_check reports the
// CONFIRMÉ pages and the god nodes it does not update (finding 🟡 of the
// reviewer), and the pages concerned.
func TestCheckScopedDrift(t *testing.T) {
	frLocale(t)
	ctx := context.Background()
	root := driftWiki(t)
	rep, err := CheckWith(ctx, root, CheckOptions{Paths: []string{"./src/cart.go", "src/auth/auth_service.go", "../outside.go", root + "/README.md"}}, nil)
	require.NoError(t, err)
	require.NotNil(t, rep.Scope)
	assert.Equal(t, FromPaths, rep.Scope.From)
	assert.Equal(t, []string{"README.md", "src/auth/auth_service.go", "src/cart.go"}, rep.Scope.Changed, "normalized, outside dropped")
	assert.Equal(t, []string{"business/accounts.md", "business/notes.md", "business/orders.md", "technical/architecture.md"}, rep.Scope.Pages)
	got := issuesOf(rep, IssueDriftConfirmed, IssueDriftGodNode)
	require.Len(t, got[IssueDriftConfirmed], 2, "%+v", rep.Issues)
	assert.Equal(t, "business/orders.md", got[IssueDriftConfirmed][0].Page, "a confirmed page links to it")
	assert.Equal(t, 9, got[IssueDriftConfirmed][0].Line)
	assert.Equal(t, "technical/architecture.md", got[IssueDriftConfirmed][1].Page, "a CONFIRMÉ tag on src/cart.go")
	assert.Equal(t, 12, got[IssueDriftConfirmed][1].Line)
	assert.Contains(t, got[IssueDriftConfirmed][1].Message, "src/cart.go")
	require.Len(t, got[IssueDriftGodNode], 1)
	assert.Equal(t, "technical/architecture.md", got[IssueDriftGodNode][0].Page)
	assert.Contains(t, got[IssueDriftGodNode][0].Message, "« AuthService »")
	for _, is := range rep.Issues {
		assert.Contains(t, []string{IssueDriftConfirmed, IssueDriftGodNode}, is.Kind, "no page updated: only the drift (%+v)", is)
	}

	// The change updates the pages: no drift; the issues of those pages.
	writeFile(t, root, "docs/wiki/business/accounts.md", "# Comptes sans frontmatter\n")
	rep, err = CheckWith(ctx, root, CheckOptions{Paths: []string{"src/cart.go", "src/auth/auth_service.go", "docs/wiki/business/accounts.md",
		"docs/wiki/technical/architecture.md", "docs/wiki/business/orders.md"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"business/accounts.md", "business/orders.md", "technical/architecture.md"}, rep.Scope.Updated)
	got = issuesOf(rep, IssueDriftConfirmed, IssueDriftGodNode, IssueFrontmatter)
	assert.Empty(t, got[IssueDriftConfirmed])
	assert.Empty(t, got[IssueDriftGodNode], "a page of the god node is updated")
	require.Len(t, got[IssueFrontmatter], 1)
	assert.Equal(t, "business/accounts.md", got[IssueFrontmatter][0].Page)

	// The name of a file is enough to touch a god node.
	assert.True(t, sameName("src/auth/AuthService.java", "AuthService"))
	assert.True(t, sameName("web/auth.service.ts", "AuthService"))
	assert.False(t, sameName("src/auth_service_test.go", "AuthService"), "a test file")
	assert.False(t, sameName("x/ab.go", "Ab"), "too short")
}

// LD-H, LD-D: a git range, and the last synchronization of the wiki.
func TestCheckRangeAndSync(t *testing.T) {
	frLocale(t)
	ctx := context.Background()
	root := driftWiki(t)
	gittest.Run(t, root, "init", "-q", "-b", "main")
	gittest.Run(t, root, "add", "-A")
	gittest.Run(t, root, "commit", "-q", "-m", "init")
	first := gittest.Run(t, root, "rev-parse", "HEAD")
	gittest.Run(t, root, "checkout", "-q", "-b", "feature/cart")
	writeFile(t, root, "src/cart.go", "package x\n\nfunc Total() {}\n")
	gittest.Run(t, root, "commit", "-q", "-am", "feat(cart): total")

	rep, err := CheckWith(ctx, root, CheckOptions{Base: "main", Head: "feature/cart"}, nil)
	require.NoError(t, err)
	sc := rep.Scope
	assert.Equal(t, FromRange, sc.From)
	assert.Equal(t, first, sc.Base)
	assert.Equal(t, []string{"src/cart.go"}, sc.Changed)
	require.Len(t, sc.Commits, 1)
	assert.Equal(t, "feat(cart): total", sc.Commits[0].Subject)
	assert.Len(t, issuesOf(rep, IssueDriftConfirmed)[IssueDriftConfirmed], 2)

	// Head empty: the working tree, untracked files included.
	writeFile(t, root, "src/new.go", "package x\n")
	rep, err = CheckWith(ctx, root, CheckOptions{Base: "main"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"src/cart.go", "src/new.go"}, rep.Scope.Changed)
	require.NoError(t, os.Remove(filepath.Join(root, "src", "new.go")))

	_, err = CheckWith(ctx, root, CheckOptions{Base: "--output=/tmp/x"}, nil)
	require.ErrorIs(t, err, ErrBadRef)
	_, err = CheckWith(ctx, root, CheckOptions{Base: "nope"}, nil)
	require.ErrorIs(t, err, ErrBadRef)

	// No synchronization recorded: since the last commit of the wiki.
	rep, err = CheckWith(ctx, root, CheckOptions{SinceSync: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, FromWikiEdit, rep.Scope.From)
	assert.Equal(t, first, rep.Scope.Base)
	assert.Len(t, rep.Scope.Commits, 1)

	// doc-sync records the commit it synchronized in the index.
	head := gittest.Run(t, root, "rev-parse", "HEAD")
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	ok, err := SetSyncMarker(root, head, at)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, head, SyncMarker(root))
	index, _ := ReadPage(root, IndexPage, "")
	assert.Contains(t, index, "synced: "+head)
	assert.Contains(t, index, "updated: 2026-10-13")
	w, _ := Load(root)
	assert.Empty(t, issuesOf(w.Check(ctx, nil), IssueFrontmatter)[IssueFrontmatter], "the marker keeps the frontmatter readable")
	gittest.Run(t, root, "commit", "-q", "-am", "docs(wiki): sync")
	writeFile(t, root, "src/order.go", "package x\n\nfunc Place() {}\n")
	gittest.Run(t, root, "commit", "-q", "-am", "feat(order): place")
	rep, err = CheckWith(ctx, root, CheckOptions{SinceSync: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, FromSync, rep.Scope.From)
	assert.Equal(t, head, rep.Scope.Base)
	require.Len(t, rep.Scope.Commits, 2)
	assert.Equal(t, "feat(order): place", rep.Scope.Commits[0].Subject, "newest first")
	assert.Equal(t, []string{"docs/wiki/index.md", "src/order.go"}, rep.Scope.Changed)
	assert.Contains(t, rep.Scope.Pages, "business/orders.md")

	// A marker no longer reachable (history rewritten): the last wiki commit.
	_, err = SetSyncMarker(root, "0123456789012345678901234567890123456789", at)
	require.NoError(t, err)
	rep, err = CheckWith(ctx, root, CheckOptions{SinceSync: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, FromWikiEdit, rep.Scope.From)

	// A wiki never committed: nothing to synchronize.
	fresh := driftWiki(t)
	gittest.Run(t, fresh, "init", "-q", "-b", "main")
	rep, err = CheckWith(ctx, fresh, CheckOptions{SinceSync: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, FromNoHistory, rep.Scope.From)
	assert.Empty(t, rep.Scope.Changed)
}

// LD-D: at the checkpoint of doc-sync, oh applies the accepted proposals
// of the whole queue and records the synchronization.
func TestCommitPassedSync(t *testing.T) {
	frLocale(t)
	ctx := context.Background()
	root := writeWiki(t, map[string]string{"index.md": atlasIndex(""), "technical/stack.md": fm + "# Stack\n\n## Librairies clés\n"})
	gittest.Run(t, root, "init", "-q", "-b", "main")
	gittest.Run(t, root, "add", "-A")
	gittest.Run(t, root, "commit", "-q", "-m", "init")
	head := gittest.Run(t, root, "rev-parse", "HEAD")
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	s := &Service{Store: &memStore{}, Now: func() time.Time { return at }}
	queued, err := s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_review", Agent: "reviewer",
		Proposal: Proposal{Page: "technical/stack.md", Section: "Librairies clés", Content: "zod.", Tag: "DÉDUIT"}})
	require.NoError(t, err)
	left, err := s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_audit", Agent: "auditor",
		Proposal: Proposal{Page: "technical/stack.md", Content: "pino.", Tag: "INCERTAIN"}})
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", queued.ID(), ChoiceAccept, "", "oh")
	require.NoError(t, err)
	sess := &domain.Session{ID: "ses_sync", ProjectID: "p1", LaunchPath: root, StartRef: head}

	msg, ids := s.CommitPassed(ctx, sess, "cp-commit", false)
	assert.Empty(t, ids, "a commit checkpoint applies the proposals of its own session only")
	assert.Empty(t, msg)

	msg, ids = s.CommitPassed(ctx, sess, "cp-sync", true)
	assert.Equal(t, []string{queued.ID()}, ids)
	assert.Contains(t, msg, "docs/wiki/technical/stack.md")
	assert.Contains(t, msg, "synced: "+head[:12])
	assert.Equal(t, head, SyncMarker(root))
	p, _ := s.Get(ctx, "p1", left.ID())
	assert.Equal(t, domain.WikiPending, p.Status, "not decided: stays queued")
}

func TestMetrics(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": "---\nupdated: 2026-10-01\nconfidence: mixed\nagents: [onboarder]\nsynced: abc123\n---\n\n# Atlas\n\n" +
			"## Carte des domaines métier\n\n- [Commandes](business/orders.md) — les commandes\n- [Factures](business/billing.md) — à venir\n" +
			"- [Archi](technical/architecture.md)\n",
		"business/orders.md":  "---\nupdated: 2026-09-01\nconfidence: mixed\nagents: [dev]\n---\n\n# Commandes\n\n- a — `CONFIRMÉ` · dev · 2026-09-01 · a.go:1\n- b — `INCERTAIN` · dev · 2026-09-01\n",
		"business/stock.md":   "---\nupdated: 2026-07-03\nconfidence: mixed\nagents: [dev]\n---\n\n# Stock\n\n*(À documenter)*\n",
		"business/index.md":   fm + "# Domaines\n",
		"specs/panier.md":     fm + "# Spec\n",
		"decisions/adr-1.md":  "---\nupdated: pas de date\n---\n# ADR\n",
		"technical/stack.md":  "---\nupdated: 2026-10-12\nconfidence: mixed\nagents: [dev]\n---\n\n# Stack\n\n- c — `DÉDUIT` · dev · 2026-09-01\n- d — `INCERTAIN` · dev · 2026-09-01\n",
		"postmortems/2026.md": "# Sans frontmatter\n",
	})
	w, err := Load(root)
	require.NoError(t, err)
	m := w.Metrics(time.Date(2026, 10, 13, 18, 0, 0, 0, time.UTC))
	assert.Equal(t, 8, m.Pages)
	assert.Equal(t, 1, m.ByKind[KindSpec])
	assert.Equal(t, 1, m.ByKind[KindDecision])
	assert.Equal(t, 1, m.ByKind[KindPostmortem])
	assert.Equal(t, 3, m.Domains, "orders, stock and billing (mapped, missing); not the map nor architecture")
	assert.Equal(t, 1, m.DomainsCovered)
	assert.Equal(t, []string{"business/billing.md", "business/stock.md"}, m.Uncovered)
	assert.InDelta(t, 1.0/3, m.Coverage(), 0.001)
	// Ages: index 12, orders 42, stock 102, business/index 12, spec 12, stack 1.
	assert.Equal(t, 6, m.Dated)
	assert.Equal(t, 12, m.MedianAgeDays)
	assert.Equal(t, "business/stock.md", m.Oldest)
	assert.Equal(t, 102, m.OldestDays)
	assert.Equal(t, map[string]int{TagConfirmed: 1, TagDeduced: 1, TagUncertain: 2}, m.Tags)
	assert.InDelta(t, 0.5, m.UncertainShare(), 0.001)
	assert.Equal(t, -1, m.Pending, "no queue")
	assert.Equal(t, "abc123", m.Synced)

	ctx := context.Background()
	s := &Service{Store: &memStore{}, Now: func() time.Time { return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC) }}
	for _, c := range []string{"x.", "y.", "z."} {
		_, err := s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "s", Proposal: Proposal{Page: "index.md", Content: c, Tag: "INCERTAIN"}})
		require.NoError(t, err)
	}
	_, err = s.Decide(ctx, "p1", "wp-1", ChoiceAccept, "", "oh")
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", "wp-2", ChoiceRefuse, "", "oh")
	require.NoError(t, err)
	m, err = s.Metrics(ctx, root, "p1")
	require.NoError(t, err)
	assert.Equal(t, 1, m.Pending)
	assert.Equal(t, 1, m.Accepted)
	empty := Metrics{}
	assert.Zero(t, empty.Coverage())
	assert.Zero(t, empty.UncertainShare())
}
