package launch

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

const lookWorkflow = `apiVersion: oh/v1
kind: Workflow
id: look
risk: read
entry: { agent: reviewer }
inputs:
  branch: { type: branch }
  base: { type: branch, default: main }
prompt:
  text: "Review {{ .branch }} against {{ .base }}"
agents:
  reviewer: { role: workflow, mode: primary }
beads: { allow: [show] }
`

// reviewRepo is a project on main with a branch feat/x that adds a file.
func reviewRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Repo(t)
	gittest.Run(t, repo, "switch", "-q", "-c", "feat/x")
	gittest.Commit(t, repo, "only-on-x.txt", "x\n", "x")
	gittest.Run(t, repo, "switch", "-q", "main")
	return repo
}

// reviewDeps launches in a real repository with a real RunService (planning
// only), the temporary worktrees going under a hub of the test.
func reviewDeps(t *testing.T) (Deps, *fakes, string) {
	t.Helper()
	d, f := newDeps(t)
	f.project.Path = reviewRepo(t)
	hub := t.TempDir()
	d.NewRunService = func(context.Context, bool) (*runsvc.Service, error) {
		return &runsvc.Service{LaunchLocksDir: filepath.Join(hub, "locks"), ServersDir: filepath.Join(hub, "servers")}, nil
	}
	return d, f, worktree.TempRoot(hub)
}

// Bug 16: the review of a branch other than the current one read the files
// of the current branch (read, grep and glob), while the diff came from the
// reviewed branch.
func TestReviewOfAnotherBranchWorksOnThatBranch(t *testing.T) {
	d, f, root := reviewDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "look", Project: f.project, Inputs: map[string]string{"branch": "feat/x"}})
	require.NoError(t, err)

	loc := p.Plan.Sessions[0].Location
	assert.NotEqual(t, f.project.Path, loc.Path, "the session does not work in the current branch")
	assert.Equal(t, runsvc.LocationWorktree, loc.Kind)
	assert.True(t, loc.Temp && loc.Create)
	assert.True(t, worktree.IsTemp(root, loc.Path), loc.Path)
	assert.Equal(t, "refs/heads/feat/x", loc.Ref)
	assert.Contains(t, p.Plan.Warnings, runsvc.Warning{Code: runsvc.WarnReviewWorktree, Args: []any{"feat/x", loc.Path}})
	assert.Equal(t, "feat/x", p.Plan.Request.Sessions[0].ReadBranch)
	assert.Equal(t, 1, p.Plan.Worktrees())
}

func TestReviewOfTheCurrentBranchIsUnchanged(t *testing.T) {
	d, f, _ := reviewDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "look", Project: f.project, Inputs: map[string]string{"branch": "main"}})
	require.NoError(t, err)
	assert.Equal(t, runsvc.Location{Path: f.project.Path, Kind: runsvc.LocationBase}, p.Plan.Sessions[0].Location)
	for _, w := range p.Plan.Warnings {
		assert.NotEqual(t, runsvc.WarnReviewWorktree, w.Code)
	}

	p, err = Prepare(t.Context(), d, Options{Workflow: "look", Project: f.project})
	require.NoError(t, err)
	assert.Equal(t, runsvc.LocationBase, p.Plan.Sessions[0].Location.Kind, "recent changes of the current branch")
	assert.Empty(t, p.Plan.Request.Sessions[0].ReadBranch)
}

func TestReviewOfAMissingBranchIsRefusedAtLaunch(t *testing.T) {
	d, f, _ := reviewDeps(t)
	_, err := Prepare(t.Context(), d, Options{Workflow: "look", Project: f.project, Inputs: map[string]string{"branch": "feat/nope"}})
	assert.EqualError(t, err, i18n.Tf("cmd.run.review_branch_missing", "feat/nope"))
	assert.ErrorContains(t, err, "git fetch origin feat/nope")
}

func TestReadBranchIsThatOfAReadOnlyWorkflowOnly(t *testing.T) {
	spec := func(src string) *workflow.Spec {
		doc, diags := workflow.Parse([]byte(src), workflow.Source{Layer: workflow.LayerHub})
		require.False(t, diags.HasErrors(), diags)
		return doc.Spec
	}
	look, ticket := spec(lookWorkflow), spec(testWorkflows["ticket.yaml"])
	assert.Equal(t, "feat/x", ReadBranch(look, map[string]any{"branch": "feat/x", "base": "main"}))
	assert.Empty(t, ReadBranch(look, map[string]any{"base": "main"}))
	assert.Empty(t, ReadBranch(ticket, map[string]any{"branch": "feat/bd-1"}), "a writer works on a branch of its own")
}

// A remote session clones the current branch of the project at its commit:
// it cannot read another one (a temporary worktree is a local notion).
func TestRemoteReviewOfAnotherBranchIsRefused(t *testing.T) {
	i18ntest.Set(t, "en")
	repo := reviewRepo(t)
	in := func(b string) []runsvc.PlannedInput { return []runsvc.PlannedInput{{ReadBranch: b}} }
	assert.NoError(t, checkRemoteReview(repo, in("")))
	assert.NoError(t, checkRemoteReview(repo, in("main")))
	assert.EqualError(t, checkRemoteReview(repo, in("feat/x")), i18n.Tf("cmd.run.review_remote_other_branch", "feat/x"))
	assert.EqualError(t, checkRemoteReview(repo, in("nope")), i18n.Tf("cmd.run.review_branch_missing", "nope"))
}

// The prompt of the shipped review tells the agent where it works: its
// worktree is on the reviewed branch (detached HEAD), so a `git branch
// --show-current` that prints nothing is no reason to doubt it; a branch only
// a remote has is named origin/<branch> in the git commands.
func TestShippedReviewPromptTellsTheTemporaryWorktree(t *testing.T) {
	d, f, _ := reviewDeps(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	d.Workflows = func(context.Context) *workflowsvc.Service {
		return &workflowsvc.Service{HubDir: root, HubWorkflowsDir: filepath.Join(root, "workflows")}
	}
	prompt := func(branch string) string {
		t.Helper()
		in := map[string]string{"branch": branch, "base": "main"}
		p, err := Prepare(t.Context(), d, Options{Workflow: "review", Project: f.project, Inputs: in})
		require.NoError(t, err)
		return p.Plan.Sessions[0].Prompt
	}
	assert.NotContains(t, prompt("main"), "worktree temporaire", "the current branch: the prompt does not change")
	got := prompt("feat/x")
	assert.Contains(t, got, "worktree temporaire de la branche `feat/x`")
	assert.Contains(t, got, "HEAD détaché")
	assert.NotContains(t, got, "n'existe qu'en distant")

	_, clone := gittest.BareClone(t)
	gittest.Run(t, clone, "switch", "-q", "-c", "feat/y")
	gittest.Commit(t, clone, "y.txt", "y\n", "y")
	gittest.Run(t, clone, "push", "-q", "origin", "feat/y")
	gittest.Run(t, clone, "switch", "-q", "main")
	gittest.Run(t, clone, "branch", "-q", "-D", "feat/y")
	f.project.Path = clone
	got = prompt("feat/y")
	assert.Contains(t, got, "worktree temporaire de la branche `origin/feat/y`")
	assert.Contains(t, got, "n'existe qu'en distant : dans les commandes git, écris `origin/feat/y` là où le prompt dit `feat/y`")
}
