package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

const hotfixTeamWorkflow = `apiVersion: oh/v1
kind: Workflow
id: hotfix
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [hotfixer] }
  hotfixer: { role: workflow }
`

// The editable catalogue (lot 2.D): drafts with their validity and new
// bricks, integrity warnings, team files in error at their own layer,
// offline queue.
func TestWorkspace(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	svc.BricksCacheDir = t.TempDir()
	repo := env.clones["alice"]
	ctx := context.Background()
	writeCatalog(t, repo.Path(), map[string]string{
		"agents/team/hotfixer.md":           teamAgent,
		"skills/team/hotfix-protocol.md":    teamSkill,
		"skills/templates/hotfix-report.md": "# Rapport",
	})

	saveDraft(t, svc, wf.LayerTeam, hotfixDraft)
	saveDraft(t, svc, wf.LayerTeam, hotfixTeamWorkflow)
	// A draft that cannot be parsed (written by hand).
	broken := filepath.Join(repo.Path(), "workflows", "drafts", "alice", "broken.yaml")
	require.NoError(t, os.WriteFile(broken, []byte("apiVersion: oh/v1\nkind: Workflow\nid: broken\nrisk: [\n"), 0o644))

	// A published team file whose lock matches but that does not parse:
	// listed invalid at the team layer, not as a hub workflow.
	bad := filepath.Join(repo.Path(), "workflows", "published", "unreadable.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("apiVersion: oh/v1\nkind: Workflow\nid: unreadable\nnope: 1\n"), 0o644))
	_, _, err := repo.SealWorkflowLocal(teamstate.TeamScope(), "unreadable", teamstate.LockMeta{Version: 1, PublishedBy: "alice"})
	require.NoError(t, err)
	// A published file without lock entry: integrity warning.
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "workflows", "published", "handmade.yaml"), []byte(hotfixDraft), 0o644))

	w, err := svc.Workspace(ctx, Context{ProjectID: "web"})
	require.NoError(t, err)
	assert.True(t, w.Editable)
	assert.Equal(t, "alice", w.Member)
	assert.Equal(t, "acme", w.TeamID)

	sum, ok := Find(w.Workflows, "unreadable")
	require.True(t, ok)
	assert.Equal(t, wf.LayerTeam, sum.Layer)
	assert.Equal(t, "team:unreadable", sum.Ref)
	assert.False(t, sum.ReadOnly)
	assert.False(t, sum.Valid)

	codes := map[string]string{}
	for _, d := range w.Integrity {
		codes[filepath.Base(d.Source)] = d.Code
	}
	assert.Equal(t, teamstate.DiagWorkflowUnlocked, codes["handmade.yaml"])

	drafts := map[string]Summary{}
	for _, d := range w.Drafts {
		assert.True(t, d.Draft)
		drafts[d.Ref] = d
	}
	require.Len(t, drafts, 3)
	assert.True(t, drafts["team:ticket-hotfix"].Valid)
	assert.True(t, drafts["team:hotfix"].Valid, "%v", drafts["team:hotfix"].Diagnostics)
	assert.Equal(t, []string{"agent:hotfixer", "skill:team/hotfix-protocol"}, drafts["team:hotfix"].NewBricks)
	assert.False(t, drafts["team:broken"].Valid)
	assert.Positive(t, drafts["team:broken"].Errors)

	// Published: the bricks are no longer new; offline publication queued.
	_, err = svc.Publish(ctx, Context{}, "hotfix", "v1")
	require.NoError(t, err)
	git(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/team-state.git")
	pub, err := svc.Publish(ctx, Context{}, "ticket-hotfix", "offline")
	require.NoError(t, err)
	require.True(t, pub.Queued)

	w, err = svc.Workspace(ctx, Context{ProjectID: "web"})
	require.NoError(t, err)
	sum, ok = Find(w.Workflows, "hotfix")
	require.True(t, ok)
	assert.Equal(t, []string{"agent:hotfixer", "skill:team/hotfix-protocol"}, sum.TeamBricks)
	require.Len(t, w.Queue, 1)
	for _, d := range w.Drafts {
		if d.Ref == "team:ticket-hotfix" {
			assert.True(t, d.Queued)
		}
	}
	ops, err := svc.QueuedOps(ctx, Context{})
	require.NoError(t, err)
	assert.Len(t, ops, 1)
}

// `oh workflow validate` checks against the bricks merged with the team
// catalogue.
func TestValidateWithTeamBricks(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	svc.BricksCacheDir = t.TempDir()
	repo := env.clones["alice"]
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "hotfix.yaml")
	require.NoError(t, os.WriteFile(file, []byte(hotfixTeamWorkflow), 0o644))

	r, err := svc.Validate(ctx, Context{}, ValidateInput{File: file, Layer: wf.LayerTeam})
	require.NoError(t, err)
	assert.False(t, r.Workflows[0].Valid, "hotfixer is unknown without the team catalogue")

	writeCatalog(t, repo.Path(), map[string]string{
		"agents/team/hotfixer.md":           teamAgent,
		"skills/team/hotfix-protocol.md":    teamSkill,
		"skills/templates/hotfix-report.md": "# Rapport",
	})
	r, err = svc.Validate(ctx, Context{}, ValidateInput{File: file, Layer: wf.LayerTeam})
	require.NoError(t, err)
	assert.True(t, r.Workflows[0].Valid, "%v", r.Diagnostics)

	r, err = svc.Validate(ctx, Context{}, ValidateInput{All: true})
	require.NoError(t, err)
	assert.NotEmpty(t, r.Workflows)
	_, err = svc.Validate(ctx, Context{}, ValidateInput{Target: "ticket", Layer: wf.LayerSession})
	assert.Error(t, err)
}

func TestNewDraftText(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	ctx := context.Background()

	empty, err := svc.NewDraftText(ctx, Context{}, NewDraft{ID: "release", Layer: wf.LayerTeam})
	require.NoError(t, err)
	assert.Contains(t, string(empty.YAML), "id: release")
	ext, err := svc.NewDraftText(ctx, Context{}, NewDraft{ID: "ticket-hotfix", Layer: wf.LayerTeam, Extends: "hub:ticket"})
	require.NoError(t, err)
	assert.Contains(t, string(ext.YAML), "extends: hub:ticket")
	cp, err := svc.NewDraftText(ctx, Context{}, NewDraft{ID: "my-ticket", Layer: wf.LayerTeam, Copy: "hub:ticket"})
	require.NoError(t, err)
	assert.Contains(t, string(cp.YAML), "id: my-ticket")
	assert.NotContains(t, string(cp.YAML), "\nversion:")
	assert.NotEmpty(t, cp.Prompt)

	_, err = svc.NewDraftText(ctx, Context{}, NewDraft{ID: "x", Layer: wf.LayerTeam, Extends: "hub:nope"})
	assert.Error(t, err)
	_, err = svc.NewDraftText(ctx, Context{}, NewDraft{ID: "Bad_ID", Layer: wf.LayerTeam})
	assert.Error(t, err)
	saveDraft(t, svc, wf.LayerTeam, hotfixDraft)
	_, err = svc.NewDraftText(ctx, Context{}, NewDraft{ID: "ticket-hotfix", Layer: wf.LayerTeam})
	assert.ErrorIs(t, err, ErrExists)
}

// A project publication queued offline is replayed from a team context
// (TUI synchronization of the team-state, without project).
func TestFlushQueueProjectOpFromTeamContext(t *testing.T) {
	env := newTeamEnv(t, "alice")
	alice := env.service(t, "alice")
	ctx := context.Background()
	repo := env.clones["alice"]
	saveDraft(t, alice, wf.LayerTeam, hotfixDraft)
	_, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "")
	require.NoError(t, err)
	saveDraft(t, alice, wf.LayerProject, "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: team:ticket-hotfix\ndescription: Web\n")

	git(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/team-state.git")
	pub, err := alice.Publish(ctx, Context{}, "project:ticket-hotfix", "offline")
	require.NoError(t, err)
	require.True(t, pub.Queued)
	git(t, repo.Path(), "remote", "set-url", "origin", env.bare)

	team := testService(t)
	team.TeamState = func(context.Context, Context) (*TeamState, error) {
		return &TeamState{Repo: repo, TeamID: "acme", Member: "alice"}, nil
	}
	pubs, errs := team.FlushQueue(ctx, Context{})
	require.Empty(t, errs)
	require.Len(t, pubs, 1)
	assert.Equal(t, "project:ticket-hotfix", pubs[0].Ref.String())
	assert.FileExists(t, filepath.Join(repo.Path(), "projects", "web", "workflows", "published", "ticket-hotfix.yaml"))
}
