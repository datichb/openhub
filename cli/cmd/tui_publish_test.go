package cmd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// TUI wiring of the publication and history screens (P2-T15) on a real
// team-state, and the replay of the offline queue at TUI synchronization.
func TestTUIPublishWiring(t *testing.T) {
	env := setupWorkflowCLI(t, "alice")
	ctx := t.Context()
	c := workflowsvc.Context{}
	_, err := runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--extends", "hub:ticket", "--no-edit")
	require.NoError(t, err)

	data, err := loadCatalogData(ctx, testTUIStart())
	require.NoError(t, err)
	assert.True(t, data.Editable)
	assert.Equal(t, "alice", data.Member)
	var draft bool
	for _, e := range data.Entries {
		draft = draft || (e.Draft && e.Ref == "team:ticket-hotfix" && e.Valid)
	}
	assert.True(t, draft, "%+v", data.Entries)

	p, err := loadPublishPreview(ctx, c, "team:ticket-hotfix")
	require.NoError(t, err)
	assert.True(t, p.Valid, "%v", p.Findings)
	assert.Equal(t, 1, p.Next)
	assert.Zero(t, p.Current)
	assert.True(t, p.New)
	assert.Contains(t, p.Diff, "+extends: hub:ticket")
	assert.NotEmpty(t, p.Governance)

	pub, err := newWorkflowService(ctx).Publish(ctx, c, "team:ticket-hotfix", "v1")
	require.NoError(t, err)
	assert.Equal(t, 1, publishResult(pub).Version)

	// Second draft, published offline then replayed by the sync hook.
	_, err = newWorkflowService(ctx).SaveDraft(ctx, c, workflowsvc.DraftInput{Layer: workflow.LayerTeam,
		YAML: []byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\ndescription: v2\n")})
	require.NoError(t, err)
	p, err = loadPublishPreview(ctx, c, "team:ticket-hotfix")
	require.NoError(t, err)
	assert.Equal(t, 1, p.Current)
	assert.Equal(t, 2, p.Next)
	assert.Contains(t, p.Diff, "+description: v2")

	gitIn(t, env.clones["alice"], "remote", "set-url", "origin", "http://127.0.0.1:1/team-state.git")
	pub, err = newWorkflowService(ctx).Publish(ctx, c, "team:ticket-hotfix", "v2")
	require.NoError(t, err)
	require.True(t, pub.Queued)
	p, err = loadPublishPreview(ctx, c, "team:ticket-hotfix")
	require.NoError(t, err)
	assert.True(t, p.Queued, "the pending publication is visible")
	gitIn(t, env.clones["alice"], "remote", "set-url", "origin", env.bare)

	data, err = loadCatalogData(ctx, testTUIStart())
	require.NoError(t, err)
	for _, e := range data.Entries {
		if e.Ref == "team:ticket-hotfix" && !e.Draft {
			assert.True(t, e.HasDraft)
		}
	}

	tuiReplayWorkflowQueue(ctx, env.clones["alice"])
	ops, err := newWorkflowService(ctx).QueuedOps(ctx, c)
	require.NoError(t, err)
	assert.Empty(t, ops, "replayed")
	assert.Contains(t, gitIn(t, env.bare, "log", "-1", "--format=%s"), "v2 by alice")

	// History: versions and the diff of v1 against the current one.
	vs, err := newWorkflowService(ctx).History(ctx, c, "team:ticket-hotfix")
	require.NoError(t, err)
	require.Len(t, vs, 2)
	old, err := newWorkflowService(ctx).VersionText(ctx, c, "team:ticket-hotfix", 1)
	require.NoError(t, err)
	cur, err := newWorkflowService(ctx).DocumentText(ctx, c, "team:ticket-hotfix")
	require.NoError(t, err)
	assert.Contains(t, textDiff(old, cur, "v1", "current"), "+description: v2")
}

// noPrefs is an empty preference store.
type noPrefs struct{}

func (noPrefs) Get(context.Context, string, string) (*domain.Preference, error) {
	return nil, domain.ErrNotFound
}
func (noPrefs) Set(context.Context, string, string, json.RawMessage) error { return nil }
func (noPrefs) Delete(context.Context, string, string) error               { return nil }
func (noPrefs) List(context.Context, string) ([]domain.Preference, error)  { return nil, nil }

func testTUIStart() *tuiStart {
	return &tuiStart{a: application, prefs: prefsvc.New(noPrefs{}, nil)}
}
