package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// LD-J: the card of the commit checkpoint carries the open wiki proposals
// of the session; the user decides on each of them there.
func TestCheckpointCardWiki(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}},
			{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}, Unlocks: []string{sessionspec.UnlockCommit}}}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	wk := &wiki.Service{Store: sqlite.NewWikiProposalStore(st)}
	svc := &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), BundlesDir: bundles, Wiki: wk}
	svc.UseCheckpoints(&checkpoint.Service{Sessions: svc.Sessions, States: sqlite.NewCheckpointStore(st), BundlesDir: bundles}, nil)
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel"}))
	}
	mine, err := wk.Propose(ctx, wiki.Filed{ProjectID: "p1", SessionID: "ses_a", Agent: "developer",
		Proposal: wiki.Proposal{Page: "technical/conventions.md", Content: "kebab-case", Tag: "DÉDUIT"}})
	require.NoError(t, err)
	other, err := wk.Propose(ctx, wiki.Filed{ProjectID: "p1", SessionID: "ses_b", Agent: "reviewer",
		Proposal: wiki.Proposal{Page: "index.md", Content: "autre", Tag: "INCERTAIN"}})
	require.NoError(t, err)

	raise := func(ref, cp string) string {
		d := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: ref,
			Payload: domain.DecisionPayload{Data: map[string]any{checkpoint.DataCheckpoint: cp}}}
		require.NoError(t, svc.Raise(ctx, d))
		return d.ID
	}
	card, err := svc.CheckpointCard(ctx, raise("per_1", "cp-1"))
	require.NoError(t, err)
	assert.Empty(t, card.Wiki, "cp-1 unlocks no commit")
	card, err = svc.CheckpointCard(ctx, raise("per_2", "cp-2"))
	require.NoError(t, err)
	assert.True(t, card.WikiCommit)
	require.Len(t, card.Wiki, 1, "the proposals of the session only")
	assert.Equal(t, mine.ID(), card.Wiki[0].ID())

	p, err := svc.WikiDecide(ctx, "ses_a", mine.ID(), wiki.ChoiceEdit, "Fichiers en kebab-case.")
	require.NoError(t, err)
	assert.Equal(t, domain.WikiAccepted, p.Status)
	assert.Equal(t, "Fichiers en kebab-case.", p.Content)
	assert.Equal(t, "oh", p.DecidedBy)
	_, err = svc.WikiDecide(ctx, "ses_a", other.ID(), wiki.ChoiceAccept, "")
	require.ErrorIs(t, err, domain.ErrNotFound, "a proposal of another session")
	list, err := svc.WikiProposals(ctx, "ses_a")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

// LD-D: the card of the checkpoint of doc-sync (`wiki: sync`) carries the
// whole queue of the project; its user decides on every proposal.
func TestCheckpointCardWikiSync(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "doc-sync",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-sync", Behaviors: map[string]string{"manuel": "pause"},
			Unlocks: []string{sessionspec.UnlockCommit}, Wiki: sessionspec.WikiSync}}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	wk := &wiki.Service{Store: sqlite.NewWikiProposalStore(st)}
	svc := &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), BundlesDir: bundles, Wiki: wk}
	svc.UseCheckpoints(&checkpoint.Service{Sessions: svc.Sessions, States: sqlite.NewCheckpointStore(st), BundlesDir: bundles}, nil)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_sync", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel"}))
	queued, err := wk.Propose(ctx, wiki.Filed{ProjectID: "p1", SessionID: "ses_review", Agent: "reviewer",
		Proposal: wiki.Proposal{Page: "index.md", Content: "file d'attente", Tag: "INCERTAIN"}})
	require.NoError(t, err)

	d := &domain.Decision{SessionID: "ses_sync", Kind: domain.DecisionCheckpoint, ToolRef: "per_1",
		Payload: domain.DecisionPayload{Data: map[string]any{checkpoint.DataCheckpoint: "cp-sync"}}}
	require.NoError(t, svc.Raise(ctx, d))
	card, err := svc.CheckpointCard(ctx, d.ID)
	require.NoError(t, err)
	assert.True(t, card.WikiCommit)
	require.Len(t, card.Wiki, 1, "the queue of the project")
	assert.Equal(t, queued.ID(), card.Wiki[0].ID())
	p, err := svc.WikiDecide(ctx, "ses_sync", queued.ID(), wiki.ChoiceAccept, "")
	require.NoError(t, err)
	assert.Equal(t, domain.WikiAccepted, p.Status)
}
