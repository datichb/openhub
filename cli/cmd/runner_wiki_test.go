package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// p3-remote-wiki: the job exports the proposals of its session, with their
// id in the job, for wiki.json.
func TestRunnerWiki(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	wk := &wiki.Service{Store: sqlite.NewWikiProposalStore(st), SessionsDir: t.TempDir()}
	for _, f := range []wiki.Filed{
		{ProjectID: "p1", SessionID: "ses_r", ToolSession: "ses_c", Agent: "reviewer", Proposal: wiki.Proposal{Page: "index.md", Content: "a", Tag: "INCERTAIN"}},
		{ProjectID: "p1", SessionID: "ses_other", Agent: "reviewer", Proposal: wiki.Proposal{Page: "index.md", Content: "b", Tag: "INCERTAIN"}},
		{ProjectID: "p1", SessionID: "ses_r", Agent: "developer", Proposal: wiki.Proposal{Page: "technical/stack.md", Section: "Go", Content: "c", Tag: "CONFIRMÉ", Source: "go.mod"}},
	} {
		_, err := wk.Propose(ctx, f)
		require.NoError(t, err)
	}
	list, err := runnerWiki(wk, "p1")(ctx, "ses_r")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "wp-1", list[0].ID)
	assert.Equal(t, "ses_c", list[0].ToolSession)
	assert.Equal(t, "wp-3", list[1].ID)
	assert.Equal(t, "developer", list[1].Agent)
	assert.Equal(t, "Go", list[1].Section)
	assert.Equal(t, "go.mod", list[1].Source)
	assert.Equal(t, wiki.TagConfirmed, list[1].Tag)
	assert.False(t, list[1].CreatedAt.IsZero())
}
