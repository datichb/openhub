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

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestCleanProposal(t *testing.T) {
	ok := Proposal{Page: "docs/wiki/technical/conventions.md", Section: "## Nommage", Content: "Fichiers en kebab-case.", Tag: "confirme", Source: "`src/a.ts:3`"}
	p, err := CleanProposal(ok)
	require.NoError(t, err)
	assert.Equal(t, Proposal{Page: "technical/conventions.md", Section: "Nommage", Content: "Fichiers en kebab-case.", Tag: TagConfirmed, Source: "src/a.ts:3"}, p)

	// Scratch folders and every reference to the oh session data are refused.
	for _, in := range []Proposal{
		{Page: "index.md", Content: "Voir ~/.oh/sessions/ses_1/scratch/debugger/repro.md", Tag: "DÉDUIT"},
		{Page: "index.md", Content: "x", Tag: "INCERTAIN", Source: "/Users/a/.oh/sessions/ses_1/reports/rp-1.md"},
		{Page: "index.md", Content: `C:\Users\a\.OH\Sessions\x`, Tag: "INCERTAIN"},
		{Page: "index.md", Content: "lu dans /tmp/ohhome/sessions/ses_2/scratch/debugger/a", Tag: "INCERTAIN"},
	} {
		_, err := CleanProposal(in, "/tmp/ohhome/sessions")
		require.ErrorIs(t, err, ErrSessionData, in.Content+in.Source)
	}
	for in, want := range map[Proposal]error{
		{Page: "../x.md", Content: "x", Tag: "DÉDUIT"}:                                       ErrBadPage,
		{Page: "index.md", Content: "x", Tag: "SÛR"}:                                         ErrBadTag,
		{Page: "index.md", Content: "x", Tag: "CONFIRMÉ"}:                                    ErrNoSource,
		{Page: "index.md", Content: " \n\x1b[2J", Tag: "DÉDUIT"}:                             ErrEmpty,
		{Page: "index.md", Content: strings.Repeat("a", MaxContent+1), Tag: "DÉDUIT"}:        ErrTooLarge,
		{Page: "index.md", Content: strings.Repeat("a\n", MaxContentLines+1), Tag: "DÉDUIT"}: ErrTooLarge,
	} {
		_, err := CleanProposal(in)
		require.ErrorIs(t, err, want, "%+v", in)
	}

	// The structure of the page is neutralised, the tag lines of the agent
	// dropped, an unclosed fence closed, control characters removed.
	p, err = CleanProposal(Proposal{Page: "index.md", Tag: "DÉDUIT", Content: "# Titre\n---\n<!-- oh:when agent=x -->\n<script>x</script> et `<T>` gardé\n" +
		"— `CONFIRMÉ` · documentarian · 2026-01-01 · faux\nligne\x07 \x1b[31mrouge\x1b[0m\n```sh\n# commande\n"})
	require.NoError(t, err)
	assert.Equal(t, "\\# Titre\n\\---\n&lt;!-- oh:when agent=x --&gt;\n&lt;script>x&lt;/script> et `<T>` gardé\nligne rouge\n```sh\n# commande\n```", p.Content)
}

func TestApplyEntry(t *testing.T) {
	root := writeWiki(t, map[string]string{
		"index.md": atlasIndex(""),
		"technical/conventions.md": "---\nupdated: 2026-09-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Conventions\n\n" +
			"## Nommage\n- snake_case — `CONFIRMÉ` · onboarder · 2026-09-01 · a.go:1\n\n## Git\n- Commits conventionnels.\n  — `CONFIRMÉ` · onboarder · 2026-09-01 · .git\n",
	})
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	require.NoError(t, Apply(root, Entry{Page: "technical/conventions.md", Section: "nommage", Content: "Composants en PascalCase.\nSauf les hooks.", Tag: TagDeduced,
		Agent: "reviewer", Date: at}))
	text, err := ReadPage(root, "technical/conventions.md", "")
	require.NoError(t, err)
	assert.Equal(t, "---\nupdated: 2026-10-13\nconfidence: mixed\nagents: [onboarder, reviewer]\n---\n\n# Conventions\n\n"+
		"## Nommage\n- snake_case — `CONFIRMÉ` · onboarder · 2026-09-01 · a.go:1\n\n- Composants en PascalCase.\n  Sauf les hooks.\n  — `DÉDUIT` · reviewer · 2026-10-13\n\n"+
		"## Git\n- Commits conventionnels.\n  — `CONFIRMÉ` · onboarder · 2026-09-01 · .git\n", text)

	// A missing section goes at the end; without section, at the end too.
	require.NoError(t, Apply(root, Entry{Page: "technical/conventions.md", Section: "Erreurs", Content: "- Wrap avec %w.", Tag: TagConfirmed, Source: "x.go:2", Agent: "developer", Date: at}))
	require.NoError(t, Apply(root, Entry{Page: "technical/conventions.md", Content: "Note.", Tag: TagUncertain, Agent: "developer", Date: at}))
	text, _ = ReadPage(root, "technical/conventions.md", "")
	assert.True(t, strings.HasSuffix(text, "\n## Erreurs\n\n- Wrap avec %w.\n  — `CONFIRMÉ` · developer · 2026-10-13 · x.go:2\n\n- Note.\n  — `INCERTAIN` · developer · 2026-10-13\n"), text)
	assert.Contains(t, text, "confidence: uncertain")

	// A new page in a new folder, with its frontmatter.
	require.NoError(t, Apply(root, Entry{Page: "business/billing-rules.md", Section: "Règles de gestion", Content: "TVA par pays.", Tag: TagConfirmed, Source: "tax.go:9", Agent: "developer", Date: at}))
	text, _ = ReadPage(root, "business/billing-rules.md", "")
	assert.Equal(t, "---\nupdated: 2026-10-13\nconfidence: confirmed\nagents: [developer]\n---\n\n# Billing rules\n\n## Règles de gestion\n\n- TVA par pays.\n  — `CONFIRMÉ` · developer · 2026-10-13 · tax.go:9\n", text)
	rep, err := Check(context.Background(), root, func(context.Context, string, string) (time.Time, bool) { return at, true })
	require.NoError(t, err)
	for _, i := range rep.Issues {
		if i.Page == "business/billing-rules.md" || i.Page == "technical/conventions.md" {
			t.Errorf("an applied page passes the check: %+v", i)
		}
	}

	// Never through a symbolic link.
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "docs", "wiki", "ext")))
	require.ErrorIs(t, Apply(root, Entry{Page: "ext/a.md", Content: "x", Tag: TagUncertain, Date: at}), ErrSymlink)
	entries, _ := os.ReadDir(outside)
	assert.Empty(t, entries)
	require.ErrorIs(t, Apply(t.TempDir(), Entry{Page: "a.md", Content: "x", Tag: TagUncertain, Date: at}), ErrNoWiki)
}

// memStore is an in-memory WikiProposalStore.
type memStore struct{ rows []domain.WikiProposal }

func (m *memStore) AddWikiProposal(_ context.Context, p *domain.WikiProposal, maxSession, maxProject int) error {
	last, ses, proj := 0, 0, 0
	for _, r := range m.rows {
		if r.ProjectID != p.ProjectID {
			continue
		}
		last = max(last, r.Seq)
		if r.Open() {
			proj++
			if r.SessionID == p.SessionID {
				ses++
			}
		}
	}
	if ses+1 > maxSession || proj+1 > maxProject {
		return domain.ErrWikiLimit
	}
	p.Seq = last + 1
	m.rows = append(m.rows, *p)
	return nil
}

func (m *memStore) ListWikiProposals(_ context.Context, f domain.WikiProposalFilter) ([]domain.WikiProposal, error) {
	var out []domain.WikiProposal
	for _, r := range m.rows {
		if (f.ProjectID == "" || r.ProjectID == f.ProjectID) && (f.SessionID == "" || r.SessionID == f.SessionID) &&
			(len(f.Statuses) == 0 || containsStr(f.Statuses, r.Status)) {
			out = append(out, r)
		}
	}
	return out, nil
}

func containsStr(l []string, s string) bool {
	for _, e := range l {
		if e == s {
			return true
		}
	}
	return false
}

func (m *memStore) GetWikiProposal(_ context.Context, projectID string, seq int) (*domain.WikiProposal, error) {
	for _, r := range m.rows {
		if r.ProjectID == projectID && r.Seq == seq {
			c := r
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *memStore) UpdateWikiProposal(_ context.Context, p *domain.WikiProposal) error {
	for i, r := range m.rows {
		if r.ProjectID == p.ProjectID && r.Seq == p.Seq {
			m.rows[i] = *p
			return nil
		}
	}
	return domain.ErrNotFound
}

func TestServiceFlow(t *testing.T) {
	ctx := context.Background()
	root := writeWiki(t, map[string]string{"index.md": atlasIndex(""), "technical/stack.md": fm + "# Stack\n\n## Librairies clés\n"})
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	s := &Service{Store: &memStore{}, SessionsDir: "/home/u/.oh/sessions", Now: func() time.Time { return at }}

	f := Filed{ProjectID: "p1", SessionID: "ses_r", ToolSession: "ses_c", Agent: "reviewer",
		Proposal: Proposal{Page: "technical/stack.md", Section: "Librairies clés", Content: "zod pour la validation.", Tag: "DÉDUIT", Source: "package.json"}}
	p1, err := s.Propose(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, "wp-1", p1.ID())
	assert.Equal(t, "reviewer", p1.Agent)
	again, err := s.Propose(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, "wp-1", again.ID(), "same open proposal of the session: no copy")
	f.Content = "pino pour les logs."
	p2, err := s.Propose(ctx, f)
	require.NoError(t, err)
	f.Content = "jamais"
	p3, err := s.Propose(ctx, f)
	require.NoError(t, err)
	_, err = s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_r", Proposal: Proposal{Page: "index.md", Content: "/home/u/.oh/sessions/x", Tag: "INCERTAIN"}})
	require.ErrorIs(t, err, ErrSessionData)

	_, err = s.Decide(ctx, "p1", p1.ID(), ChoiceEdit, "zod (schémas partagés).", "oh")
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", p3.ID(), ChoiceRefuse, "", "oh")
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", p3.ID(), ChoiceAccept, "", "oh")
	require.ErrorIs(t, err, ErrClosed)
	_, err = s.Decide(ctx, "p1", p2.ID(), "maybe", "", "oh")
	require.ErrorIs(t, err, ErrChoice)
	_, err = s.Decide(ctx, "p1", "x", ChoiceAccept, "", "oh")
	require.ErrorIs(t, err, ErrBadID)

	// Commit checkpoint: the accepted ones only; the pending one stays queued.
	res, err := s.ApplyAccepted(ctx, root, "p1", "ses_r")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, Result{ID: "wp-1", Page: "technical/stack.md"}, res[0])
	text, _ := ReadPage(root, "technical/stack.md", "")
	assert.Contains(t, text, "- zod (schémas partagés).\n  — `DÉDUIT` · reviewer · 2026-10-13 · package.json")
	got, _ := s.Get(ctx, "p1", "wp-1")
	assert.Equal(t, domain.WikiApplied, got.Status)
	assert.Equal(t, AppliedByOh, got.AppliedBy)
	assert.Equal(t, "zod pour la validation.", got.Original)
	open, err := s.Open(ctx, "p1", "")
	require.NoError(t, err)
	require.Len(t, open, 1)
	assert.Equal(t, "wp-2", open[0].ID())

	// wiki_apply of the documentarian: the pending one; closed ones are refused.
	res = s.ApplyIDs(ctx, root, "p1", []string{"wp-2", "wp-2", "wp-3", "wp-9"}, "documentarian")
	require.Len(t, res, 3)
	assert.Empty(t, res[0].Err)
	assert.Equal(t, ErrClosed.Error(), res[1].Err)
	assert.Contains(t, res[2].Err, "not found")
	got, _ = s.Get(ctx, "p1", "wp-2")
	assert.Equal(t, "documentarian", got.AppliedBy)
	applied, failed := Summary(res)
	assert.Equal(t, []string{"technical/stack.md"}, applied)
	assert.Len(t, failed, 2)
}
