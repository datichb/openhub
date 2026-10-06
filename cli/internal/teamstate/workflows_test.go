package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

const hubTicketYAML = `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
entry: { agent: orchestrator-dev }
inputs:
  ticket: { type: beads-id, required: true }
prompt: { text: "Do {{ .ticket }}" }
agents:
  orchestrator-dev: { role: workflow }
checkpoints:
  cp-1: { label: Start, mode: { manuel: pause, semi-auto: auto, auto: auto } }
`

const teamHotfixYAML = `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
extends: hub:ticket
description: Hotfix
prompt: { template: prompts/ticket-hotfix.md.tmpl }
`

func hubCatalog(t *testing.T) *workflow.MemCatalog {
	t.Helper()
	cat := workflow.NewMemCatalog()
	doc, diags := workflow.Parse([]byte(hubTicketYAML), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	require.Nil(t, cat.Add(doc))
	return cat
}

func writeRepoFile(t *testing.T, repo *Repo, rel, content string) {
	t.Helper()
	full := filepath.Join(repo.Path(), rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// publishLocal writes a published file (and its prompt) and seals it.
func publishLocal(t *testing.T, repo *Repo, scope WorkflowScope, id, yaml, prompt string) []string {
	t.Helper()
	rel, err := PublishedRel(scope, id)
	require.NoError(t, err)
	writeRepoFile(t, repo, rel, yaml)
	if prompt != "" {
		prel, err := PromptRel(scope, "prompts/"+id+".md.tmpl")
		require.NoError(t, err)
		writeRepoFile(t, repo, prel, prompt)
	}
	_, files, err := repo.SealWorkflowLocal(scope, id, LockMeta{Version: 1, PublishedBy: "alice", PublishedAt: time.Now(), Message: "first"})
	require.NoError(t, err)
	return files
}

func codesOf(ds workflow.Diagnostics) []string { return ds.Codes() }

func TestWorkflowPaths(t *testing.T) {
	rel, err := PublishedRel(TeamScope(), "ticket-hotfix")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("workflows", "published", "ticket-hotfix.yaml"), rel)

	rel, err = DraftRel(ProjectScope("web"), "alice", "ticket")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("projects", "web", "workflows", "drafts", "alice", "ticket.yaml"), rel)

	rel, err = HistoryRel(TeamScope(), "ticket", 3)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("workflows", "history", "ticket", "3.yaml"), rel)

	rel, err = HistoryPromptRel(TeamScope(), "ticket", 3)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("workflows", "history", "ticket", "3.prompt.md.tmpl"), rel)

	rel, err = PromptRel(ProjectScope("web"), "prompts/x.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("projects", "web", "workflows", "prompts", "x.md.tmpl"), rel)

	for _, bad := range []string{"", "../x", "a/b", "Ticket", "a_b", "-a"} {
		_, err := PublishedRel(TeamScope(), bad)
		assert.Error(t, err, "id %q", bad)
	}
	_, err = PublishedRel(ProjectScope("../etc"), "ticket")
	assert.ErrorIs(t, err, ErrUnsafeName)
	_, err = DraftRel(TeamScope(), "../bob", "ticket")
	assert.ErrorIs(t, err, ErrUnsafeName)
	_, err = HistoryRel(TeamScope(), "ticket", 0)
	assert.Error(t, err)
	for _, bad := range []string{"../config.toml", "/etc/passwd", "prompts/../../x", ""} {
		_, err := PromptRel(TeamScope(), bad)
		assert.Error(t, err, "prompt %q", bad)
	}
}

func TestInitStructureCreatesWorkflowLayout(t *testing.T) {
	repo := setupTestRepo(t)
	require.NoError(t, repo.InitStructure(context.Background()))
	for _, d := range []string{"workflows/published", "workflows/drafts", "workflows/prompts", "workflows/history", "catalog/agents", "catalog/skills"} {
		_, err := os.Stat(filepath.Join(repo.Path(), d, ".gitkeep"))
		assert.NoError(t, err, d)
	}
	require.NoError(t, repo.EnsureWorkflowLayout(ProjectScope("web")))
	_, err := os.Stat(filepath.Join(repo.Path(), "projects", "web", "workflows", "published", ".gitkeep"))
	assert.NoError(t, err)
	projects, err := repo.WorkflowProjects()
	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, projects)
}

func TestWorkflowLockRoundTrip(t *testing.T) {
	repo := setupTestRepo(t)
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	l := &WorkflowLock{}
	l.Set(TeamScope(), "ticket", LockEntry{Version: 2, Hash: "sha256:a", PublishedBy: "alice", PublishedAt: at, Message: "m"})
	l.Set(ProjectScope("web"), "review", LockEntry{Version: 1, Hash: "sha256:b", PromptHash: "sha256:c", PublishedBy: "bob", PublishedAt: at})
	require.NoError(t, repo.WriteWorkflowLockLocal(l))

	got, err := repo.ReadWorkflowLock()
	require.NoError(t, err)
	assert.Equal(t, l, got)
	data, err := os.ReadFile(filepath.Join(repo.Path(), WorkflowsLockFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "[team.ticket]")
	assert.Contains(t, string(data), "[projects.web.review]")

	got.Delete(ProjectScope("web"), "review")
	assert.Empty(t, got.Projects)
	assert.Equal(t, []string{"ticket"}, got.IDs(TeamScope()))
}

func TestLoadWorkflowLayers_PublishedAndResolved(t *testing.T) {
	repo := setupTestRepo(t)
	require.NoError(t, repo.InitStructure(context.Background()))
	files := publishLocal(t, repo, TeamScope(), "ticket-hotfix", teamHotfixYAML, "Hotfix {{ .ticket }}")
	assert.Equal(t, []string{
		filepath.Join("workflows", "published", "ticket-hotfix.yaml"),
		WorkflowsLockFile,
		filepath.Join("workflows", "prompts", "ticket-hotfix.md.tmpl"),
	}, files)
	publishLocal(t, repo, ProjectScope("web"), "ticket-hotfix", `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
extends: team:ticket-hotfix
description: Web hotfix
`, "")

	cat := hubCatalog(t)
	diags := repo.LoadWorkflowLayers(cat, "web")
	require.Empty(t, diags)

	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerProject, ID: "ticket-hotfix"}, nil,
		workflow.Env{Prompts: PromptSource{Repo: repo}})
	require.False(t, diags.HasErrors(), "%v", diags)
	assert.Equal(t, []workflow.Ref{{Layer: workflow.LayerHub, ID: "ticket"}, {Layer: workflow.LayerTeam, ID: "ticket-hotfix"}, {Layer: workflow.LayerProject, ID: "ticket-hotfix"}}, r.Chain)
	assert.Equal(t, "Web hotfix", r.Spec.Description.Text("en"))

	// Without the project, only the team layer is loaded.
	cat = hubCatalog(t)
	require.Empty(t, repo.LoadWorkflowLayers(cat, ""))
	_, ok := cat.Lookup(workflow.Ref{Layer: workflow.LayerProject, ID: "ticket-hotfix"})
	assert.False(t, ok)
}

// Acceptance criterion 2: a published file changed by hand is skipped with a
// warning (also after a commit and a clone by another member).
func TestLoadWorkflowLayers_HandEditedFileIsSkipped(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.InitStructure(ctx))
	files := publishLocal(t, repo, TeamScope(), "ticket-hotfix", teamHotfixYAML, "Hotfix {{ .ticket }}")
	gitCmd(t, repo.Path(), append([]string{"add"}, append(files, ".")...)...)
	gitCmd(t, repo.Path(), "commit", "-m", "workflow: publish ticket-hotfix v1")
	gitCmd(t, repo.Path(), "push")

	// Another member edits the published file by hand and pushes.
	other := filepath.Join(t.TempDir(), "other")
	gitCmd(t, t.TempDir(), "clone", bare, other)
	path := filepath.Join(other, "workflows", "published", "ticket-hotfix.yaml")
	require.NoError(t, os.WriteFile(path, []byte(teamHotfixYAML+"risk: read\n"), 0o644))
	gitCmd(t, other, "commit", "-am", "manual edit")
	gitCmd(t, other, "push")

	require.NoError(t, repo.Pull(ctx))
	cat := hubCatalog(t)
	diags := repo.LoadWorkflowLayers(cat, "")
	require.Equal(t, []string{DiagWorkflowHashMismatch}, codesOf(diags))
	assert.Equal(t, workflow.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Source, "ticket-hotfix.yaml")
	assert.NotEmpty(t, diags[0].Message)
	assert.True(t, IsIntegrityDiag(diags[0]))
	_, ok := cat.Lookup(workflow.Ref{Layer: workflow.LayerTeam, ID: "ticket-hotfix"})
	assert.False(t, ok, "a hand-edited file must not be loaded")
}

func TestLoadWorkflowLayers_IntegrityWarnings(t *testing.T) {
	repo := setupTestRepo(t)
	require.NoError(t, repo.InitStructure(context.Background()))
	publishLocal(t, repo, TeamScope(), "ticket-hotfix", teamHotfixYAML, "Hotfix {{ .ticket }}")

	// Prompt template changed by hand.
	writeRepoFile(t, repo, filepath.Join("workflows", "prompts", "ticket-hotfix.md.tmpl"), "Changed {{ .ticket }}")
	// File never published.
	writeRepoFile(t, repo, filepath.Join("workflows", "published", "draft-like.yaml"), "apiVersion: oh/v1\nkind: Workflow\nid: draft-like\nrisk: read\n")
	// Lock entry whose file was deleted.
	l, err := repo.ReadWorkflowLock()
	require.NoError(t, err)
	l.Set(TeamScope(), "gone", LockEntry{Version: 1, Hash: "sha256:x"})
	require.NoError(t, repo.WriteWorkflowLockLocal(l))

	cat := hubCatalog(t)
	diags := repo.LoadWorkflowLayers(cat, "")
	assert.ElementsMatch(t, []string{DiagWorkflowPromptMismatch, DiagWorkflowUnlocked, DiagWorkflowLockOrphan}, codesOf(diags))
	for _, d := range diags {
		assert.Equal(t, workflow.SeverityWarning, d.Severity, d.Code)
	}
	assert.Equal(t, []workflow.Ref{{Layer: workflow.LayerHub, ID: "ticket"}}, cat.Refs())

	// An unreadable lock loads nothing.
	writeRepoFile(t, repo, WorkflowsLockFile, "[team\n")
	cat = hubCatalog(t)
	diags = repo.LoadWorkflowLayers(cat, "")
	assert.Equal(t, []string{DiagWorkflowLockInvalid}, codesOf(diags))
	assert.Len(t, cat.Refs(), 1)
}

func TestLoadWorkflowLayers_ParseErrorsAreReported(t *testing.T) {
	repo := setupTestRepo(t)
	publishLocal(t, repo, TeamScope(), "broken", "apiVersion: oh/v1\nkind: Workflow\nid: broken\nnope: 1\n", "")
	publishLocal(t, repo, TeamScope(), "renamed", "apiVersion: oh/v1\nkind: Workflow\nid: other\nrisk: read\n", "")
	cat := workflow.NewMemCatalog()
	diags := repo.LoadWorkflowLayers(cat, "")
	assert.True(t, diags.HasErrors())
	assert.Contains(t, codesOf(diags), "id_filename_mismatch")
	assert.Empty(t, cat.Refs())
}

func TestPromptSource(t *testing.T) {
	repo := setupTestRepo(t)
	writeRepoFile(t, repo, filepath.Join("workflows", "prompts", "a.md.tmpl"), "team")
	writeRepoFile(t, repo, filepath.Join("projects", "web", "workflows", "prompts", "a.md.tmpl"), "project")
	ps := PromptSource{Repo: repo, Fallback: fakePrompts("hub")}

	read := func(layer workflow.Layer, source, path string) (string, error) {
		data, err := ps.ReadPrompt(workflow.Origin{Layer: layer, Source: source}, path)
		return string(data), err
	}
	got, err := read(workflow.LayerTeam, filepath.Join(repo.Path(), "workflows", "drafts", "alice", "a.yaml"), "prompts/a.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "team", got)
	got, err = read(workflow.LayerProject, filepath.Join(repo.Path(), "projects", "web", "workflows", "published", "a.yaml"), "prompts/a.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "project", got)
	got, err = read(workflow.LayerTeam, filepath.Join(repo.Path(), "workflows", "history", "a", "2.yaml"), "prompts/a.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "team", got)
	got, err = read(workflow.LayerHub, "/hub/workflows/a.yaml", "prompts/a.md.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "hub", got)
	_, err = read(workflow.LayerTeam, filepath.Join(repo.Path(), "workflows", "published", "a.yaml"), "../config.toml")
	assert.ErrorIs(t, err, ErrUnsafeName)
}

type fakePrompts string

func (f fakePrompts) ReadPrompt(workflow.Origin, string) ([]byte, error) { return []byte(f), nil }
