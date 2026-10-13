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
)

// p3-remote-wiki: the proposals of a remote job are queued on the machine,
// pending, cleaned again, renumbered by the local queue; a second fetch
// adds nothing.
func TestImportRemoteProposals(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	st := &memStore{}
	s := &Service{Store: st, SessionsDir: "/home/u/.oh/sessions", Now: func() time.Time { return at }}
	// The local queue of the project already holds proposals.
	_, err := s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_other", Agent: "reviewer",
		Proposal: Proposal{Page: "index.md", Content: "déjà là", Tag: "INCERTAIN"}})
	require.NoError(t, err)

	job := []Imported{
		{ID: "wp-1", Agent: "reviewer", ToolSession: "ses_c", CreatedAt: at.Add(-time.Hour),
			Proposal: Proposal{Page: "technical/stack.md", Section: "## Librairies clés", Content: "<b>zod</b> pour la validation.", Tag: "deduit", Source: "package.json"}},
		{ID: "wp-2", Agent: "reviewer", Proposal: Proposal{Page: "index.md", Content: "voir /home/u/.oh/sessions/ses_r/scratch/a", Tag: "INCERTAIN"}},
		{ID: "wp-3", Agent: "", Proposal: Proposal{Page: "../../etc/passwd.md", Content: "x", Tag: "INCERTAIN"}},
		{ID: "wp-4", Agent: "auditor-subagent\x1b[31m", Proposal: Proposal{Page: "business/panier.md", Content: "Le panier expire après 30 min.", Tag: "CONFIRMÉ", Source: "src/cart.ts:12"}},
	}
	ids, n, err := s.Import(ctx, "p1", "ses_r", job)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the session data and the bad page are refused again")
	assert.Equal(t, map[string]string{"wp-1": "wp-2", "wp-4": "wp-3"}, ids, "numbered by the local queue")

	list, err := s.List(ctx, "p1", "ses_r")
	require.NoError(t, err)
	require.Len(t, list, 2)
	p := list[0]
	assert.Equal(t, domain.WikiPending, p.Status)
	assert.Equal(t, "reviewer", p.Agent)
	assert.Equal(t, "ses_c", p.ToolSession)
	assert.Equal(t, "Librairies clés", p.Section)
	assert.Equal(t, TagDeduced, p.Tag)
	assert.Equal(t, "&lt;b>zod&lt;/b> pour la validation.", p.Content, "cleaned like a wiki_propose")
	assert.Equal(t, at.Add(-time.Hour), p.CreatedAt)
	assert.Equal(t, "auditor-subagent", list[1].Agent, "control sequences removed")

	// A second fetch, after the user refused one and edited the other.
	_, err = s.Decide(ctx, "p1", "wp-2", ChoiceRefuse, "", "oh")
	require.NoError(t, err)
	_, err = s.Decide(ctx, "p1", "wp-3", ChoiceEdit, "Le panier expire après 30 minutes.", "oh")
	require.NoError(t, err)
	ids, n, err = s.Import(ctx, "p1", "ses_r", job)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, map[string]string{"wp-1": "wp-2", "wp-4": "wp-3"}, ids)
	got, err := s.Get(ctx, "p1", "wp-2")
	require.NoError(t, err)
	assert.Equal(t, domain.WikiRefused, got.Status, "the decision of the user is kept")

	// A full queue: what fits is queued, the error says why.
	many := make([]Imported, 0, MaxPerSession)
	for i := range MaxPerSession {
		many = append(many, Imported{ID: domain.WikiProposalID(i + 1), Agent: "reviewer",
			Proposal: Proposal{Page: "index.md", Content: "finding " + domain.WikiProposalID(i+1), Tag: "INCERTAIN"}})
	}
	_, n, err = s.Import(ctx, "p1", "ses_full", many[:MaxPerSession-1])
	require.NoError(t, err)
	assert.Equal(t, MaxPerSession-1, n)
	_, err = s.Propose(ctx, Filed{ProjectID: "p1", SessionID: "ses_full", Agent: "reviewer", Proposal: Proposal{Page: "index.md", Content: "local", Tag: "INCERTAIN"}})
	require.NoError(t, err)
	_, n, err = s.Import(ctx, "p1", "ses_full", append(many, Imported{ID: "wp-99", Proposal: Proposal{Page: "index.md", Content: "de trop", Tag: "INCERTAIN"}}))
	require.ErrorIs(t, err, domain.ErrWikiLimit)
	assert.Zero(t, n)
}

// p3-remote-wiki: in a remote job oh reads the wiki as root in a clone of
// the agents' account: no symbolic link makes it read or stat a file out
// of the project, whatever component it replaces.
func TestWikiReadsStayInTheProject(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "wiki"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "wiki", "index.md"), []byte(fm+"# Secret\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "token"), []byte("secret"), 0o600))

	// docs/ itself is a link out of the project.
	root := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "docs")))
	w, err := Load(root)
	if err == nil {
		assert.Empty(t, w.Pages, "nothing read through docs -> outside")
	}
	_, err = ReadPage(root, "index.md", "")
	require.Error(t, err)

	// A project file cited by a page, through a link out of the project.
	root = writeWiki(t, map[string]string{"index.md": atlasIndex("")})
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "src")))
	_, ok := GitModified(context.Background(), root, "src/token")
	assert.False(t, ok, "not stat'ed out of the project")
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644))
	_, ok = GitModified(context.Background(), root, "go.mod")
	assert.True(t, ok)
	w, err = Load(root)
	require.NoError(t, err)
	require.Len(t, w.Pages, 1)
	assert.Equal(t, "index.md", w.Pages[0].Path)
}
