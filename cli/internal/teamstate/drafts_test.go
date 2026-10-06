package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestDrafts(t *testing.T) {
	repo := setupTestRepo(t)
	files, err := repo.WriteDraftLocal(TeamScope(), "alice", "ticket-hotfix", []byte(teamHotfixYAML), []byte("Draft {{ .ticket }}"))
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join("workflows", "drafts", "alice", "ticket-hotfix.yaml"),
		filepath.Join("workflows", "drafts", "alice", "ticket-hotfix.prompt.md.tmpl"),
	}, files)
	_, err = repo.WriteDraftLocal(ProjectScope("web"), "bob", "ticket-hotfix", []byte(teamHotfixYAML), nil)
	require.NoError(t, err)

	drafts, err := repo.ListDrafts("alice", TeamScope(), ProjectScope("web"))
	require.NoError(t, err)
	require.Len(t, drafts, 1, "only alice's drafts")
	assert.Equal(t, "team:ticket-hotfix", drafts[0].Ref().String())
	assert.NotEmpty(t, drafts[0].Prompt)

	// Drafts replace the published document, flagged as drafts, and read
	// their own prompt copy.
	cat := hubCatalog(t)
	publishLocal(t, repo, TeamScope(), "ticket-hotfix", teamHotfixYAML, "Published {{ .ticket }}")
	require.Empty(t, repo.LoadWorkflowLayers(cat, ""))
	refs, diags := repo.LoadDrafts(cat, "alice", TeamScope())
	require.Empty(t, diags)
	assert.Equal(t, []workflow.Ref{{Layer: workflow.LayerTeam, ID: "ticket-hotfix"}}, refs)
	doc, _ := cat.Lookup(refs[0])
	assert.True(t, doc.Source.Draft)
	ps := PromptSource{Repo: repo}
	got, err := ps.ReadPrompt(workflow.Origin{Layer: workflow.LayerTeam, Source: doc.Source.Path}, "prompts/ticket-hotfix.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "Draft {{ .ticket }}", string(got))
	pubPath := filepath.Join(repo.Path(), "workflows", "published", "ticket-hotfix.yaml")
	got, err = ps.ReadPrompt(workflow.Origin{Layer: workflow.LayerTeam, Source: pubPath}, "prompts/ticket-hotfix.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "Published {{ .ticket }}", string(got), "a published file never reads a draft copy")

	removed, err := repo.DeleteDraftLocal(TeamScope(), "alice", "ticket-hotfix")
	require.NoError(t, err)
	assert.Len(t, removed, 2)
	drafts, _ = repo.ListDrafts("alice", TeamScope())
	assert.Empty(t, drafts)
	_, err = repo.ListDrafts("../x", TeamScope())
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestListHistory(t *testing.T) {
	repo := setupTestRepo(t)
	for _, v := range []int{2, 1, 10} {
		rel, err := HistoryRel(TeamScope(), "ticket", v)
		require.NoError(t, err)
		writeRepoFile(t, repo, rel, "id: ticket\n")
	}
	meta, err := MarshalLockEntry(LockEntry{Version: 2, PublishedBy: "bob", Message: "m"})
	require.NoError(t, err)
	mrel, _ := HistoryMetaRel(TeamScope(), "ticket", 2)
	writeRepoFile(t, repo, mrel, string(meta))
	prel, _ := HistoryPromptRel(TeamScope(), "ticket", 2)
	writeRepoFile(t, repo, prel, "p")
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "workflows", "history", "ticket", "x.yaml"), nil, 0o644))

	h, err := repo.ListHistory(TeamScope(), "ticket")
	require.NoError(t, err)
	require.Len(t, h, 3)
	assert.Equal(t, []int{1, 2, 10}, []int{h[0].Version, h[1].Version, h[2].Version})
	assert.Equal(t, "bob", h[1].Entry.PublishedBy)
	assert.NotEmpty(t, h[1].Prompt)
	assert.Empty(t, h[0].Prompt)
	none, err := repo.ListHistory(TeamScope(), "nope")
	require.NoError(t, err)
	assert.Empty(t, none)
}
