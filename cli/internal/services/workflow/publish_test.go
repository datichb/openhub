package workflow

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// teamEnv is a team-state remote (bare) with one clone per member.
type teamEnv struct {
	bare   string
	clones map[string]*teamstate.Repo
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func newTeamEnv(t *testing.T, members ...string) *teamEnv {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "T", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "T", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	env := &teamEnv{bare: t.TempDir(), clones: map[string]*teamstate.Repo{}}
	git(t, env.bare, "init", "--bare", "-b", "main")
	seed := filepath.Join(t.TempDir(), "seed")
	git(t, t.TempDir(), "clone", env.bare, seed)
	var mf strings.Builder
	for _, m := range members {
		mf.WriteString("[members." + m + "]\ndisplay_name = \"" + m + "\"\nrole = \"dev\"\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(seed, "members.toml"), []byte(mf.String()), 0o644))
	repo := teamstate.NewRepo(env.bare, seed)
	require.NoError(t, repo.InitStructure(context.Background()))
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "init")
	git(t, seed, "push", "-u", "origin", "HEAD:main")
	for _, m := range members {
		dir := filepath.Join(t.TempDir(), m)
		git(t, t.TempDir(), "clone", env.bare, dir)
		env.clones[m] = teamstate.NewRepo(env.bare, dir)
	}
	return env
}

// service returns the WorkflowService of member, on project "web".
func (e *teamEnv) service(t *testing.T, member string) *Service {
	svc := testService(t)
	repo := e.clones[member]
	svc.TeamState = func(context.Context, Context) (*TeamState, error) {
		return &TeamState{Repo: repo, TeamID: "acme", Member: member, Project: "web"}, nil
	}
	return svc
}

const hotfixDraft = `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
extends: hub:ticket
description: Hotfix
checkpoints:
  cp-2: { label: Revue, mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }
`

func saveDraft(t *testing.T, svc *Service, layer wf.Layer, yaml string) *Draft {
	t.Helper()
	d, err := svc.SaveDraft(context.Background(), Context{ProjectID: "web"}, DraftInput{Layer: layer, YAML: []byte(yaml)})
	require.NoError(t, err)
	return d
}

func TestSaveDraft(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	ctx := context.Background()

	_, err := svc.SaveDraft(ctx, Context{}, DraftInput{Layer: wf.LayerHub, YAML: []byte(hotfixDraft)})
	assert.ErrorIs(t, err, ErrHubReadOnly)

	// Errors: nothing saved.
	_, err = svc.SaveDraft(ctx, Context{}, DraftInput{Layer: wf.LayerTeam, YAML: []byte(hotfixDraft + "risk: nope\n")})
	var inv *InvalidError
	require.ErrorAs(t, err, &inv)
	assert.NotEmpty(t, inv.Diagnostics)
	drafts, err := svc.Drafts(ctx, Context{})
	require.NoError(t, err)
	assert.Empty(t, drafts)

	d := saveDraft(t, svc, wf.LayerTeam, hotfixDraft)
	assert.True(t, d.Pushed, d.PushError)
	assert.Equal(t, "team:ticket-hotfix", d.Ref.String())
	assert.FileExists(t, filepath.Join(env.clones["alice"].Path(), "workflows", "drafts", "alice", "ticket-hotfix.yaml"))
	assert.Contains(t, git(t, env.bare, "log", "--format=%s"), "workflow: draft team:ticket-hotfix by alice")

	// A draft is not part of the published catalogue.
	list, err := svc.Catalog(ctx, Context{})
	require.NoError(t, err)
	_, ok := Find(list, "ticket-hotfix")
	assert.False(t, ok)

	// Own prompt template, validated before saving.
	_, err = svc.SaveDraft(ctx, Context{}, DraftInput{Layer: wf.LayerTeam,
		YAML: []byte(hotfixDraft + "prompt: { template: prompts/ticket-hotfix.md.tmpl }\n"), Prompt: []byte("Hotfix {{ .nope }}")})
	require.ErrorAs(t, err, &inv)
	d, err = svc.SaveDraft(ctx, Context{}, DraftInput{Layer: wf.LayerTeam,
		YAML: []byte(hotfixDraft + "prompt: { template: prompts/ticket-hotfix.md.tmpl }\n"), Prompt: []byte("Hotfix {{ .ticket }}")})
	require.NoError(t, err)
	assert.FileExists(t, strings.TrimSuffix(d.Path, ".yaml")+teamstate.OwnPromptSuffix)

	require.NoError(t, svc.DiscardDraft(ctx, Context{}, wf.LayerTeam, "ticket-hotfix"))
	drafts, _ = svc.Drafts(ctx, Context{})
	assert.Empty(t, drafts)
}

// Acceptance criterion 1 (service side): create ticket-hotfix (extends
// hub:ticket) in the team, run it as a draft, publish it; another member
// sees it after sync.
func TestPublishLifecycle(t *testing.T) {
	env := newTeamEnv(t, "alice", "bob")
	alice := env.service(t, "alice")
	ctx := context.Background()

	_, err := alice.SaveDraft(ctx, Context{}, DraftInput{Layer: wf.LayerTeam,
		YAML: []byte(hotfixDraft + "prompt: { template: prompts/ticket-hotfix.md.tmpl }\n"), Prompt: []byte("Hotfix {{ .ticket }}")})
	require.NoError(t, err)

	res, err := alice.ResolveDraft(ctx, Context{}, "ticket-hotfix", ResolveOpts{Session: &wf.SessionOptions{Inputs: map[string]any{"ticket": "bd-1"}}})
	require.NoError(t, err)
	p, err := res.RenderPrompt(PromptContext{})
	require.NoError(t, err)
	assert.Contains(t, p, "Hotfix bd-1", "the draft's own template")
	_, err = alice.ResolveDraft(ctx, Context{}, "ticket-hotfix", ResolveOpts{Session: &wf.SessionOptions{Runtime: wf.RuntimeRemote}})
	assert.ErrorIs(t, err, ErrDraftRemote)
	_, err = alice.ResolveDraft(ctx, Context{}, "quick", ResolveOpts{})
	assert.ErrorIs(t, err, ErrNoDraft)

	pub, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "Revue obligatoire")
	require.NoError(t, err)
	assert.Equal(t, 1, pub.Version)
	assert.True(t, pub.Impact.New)
	assert.False(t, pub.Queued)

	repo := env.clones["alice"]
	data, err := os.ReadFile(filepath.Join(repo.Path(), "workflows", "published", "ticket-hotfix.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "\nversion: 1\n")
	prompt, err := os.ReadFile(filepath.Join(repo.Path(), "workflows", "prompts", "ticket-hotfix.md.tmpl"))
	require.NoError(t, err)
	assert.Equal(t, "Hotfix {{ .ticket }}", string(prompt))
	assert.NoFileExists(t, filepath.Join(repo.Path(), "workflows", "drafts", "alice", "ticket-hotfix.yaml"), "the draft is consumed")
	events, err := os.ReadFile(filepath.Join(repo.Path(), "projects", teamstate.TeamEventsProject, "events", pub.At.Format("2006-01")+".jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(events), `"event":"workflow.published"`)
	assert.Contains(t, git(t, env.bare, "log", "-1", "--format=%s"), "workflow: publish team:ticket-hotfix v1 by alice")

	// Bob sees it after a sync, valid, at the team layer.
	bob := env.service(t, "bob")
	require.NoError(t, env.clones["bob"].Pull(ctx))
	list, err := bob.Catalog(ctx, Context{})
	require.NoError(t, err)
	sum, ok := Find(list, "ticket-hotfix")
	require.True(t, ok)
	assert.Equal(t, "team:ticket-hotfix", sum.Ref)
	assert.Equal(t, 1, sum.Version)
	assert.True(t, sum.Valid, "%v", sum.Diagnostics)
	assert.False(t, sum.ReadOnly)
	res, err = bob.Resolve(ctx, Context{}, "ticket-hotfix", ResolveOpts{Session: &wf.SessionOptions{Inputs: map[string]any{"ticket": "bd-2"}}})
	require.NoError(t, err)
	p, err = res.RenderPrompt(PromptContext{})
	require.NoError(t, err)
	assert.Contains(t, p, "Hotfix bd-2")

	// Second version: impact against v1, v1 in history.
	saveDraft(t, alice, wf.LayerTeam, strings.Replace(hotfixDraft, "mandatory: true", "mandatory: false", 1)+
		"prompt: { template: prompts/ticket-hotfix.md.tmpl }\nruntime: { default: local, allowed: [local] }\n")
	// Testing the draft is refused: it loosens the published checkpoint.
	_, err = alice.ResolveDraft(ctx, Context{}, "ticket-hotfix", ResolveOpts{})
	var loose *DraftLoosensError
	require.ErrorAs(t, err, &loose)
	assert.Equal(t, "checkpoint_not_mandatory", loose.Items[0].Code)

	pub, err = alice.Publish(ctx, Context{}, "team:ticket-hotfix", "v2")
	require.NoError(t, err)
	assert.Equal(t, 2, pub.Version)
	codes := []string{}
	for _, it := range pub.Impact.Items {
		codes = append(codes, it.Code)
	}
	assert.Contains(t, codes, "checkpoint_not_mandatory")
	assert.NotEmpty(t, pub.Impact.Widenings())
	assert.FileExists(t, filepath.Join(repo.Path(), "workflows", "history", "ticket-hotfix", "1.yaml"))
	assert.FileExists(t, filepath.Join(repo.Path(), "workflows", "history", "ticket-hotfix", "1.prompt.md.tmpl"))

	hist, err := alice.History(ctx, Context{}, "ticket-hotfix")
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.True(t, hist[0].Current)
	assert.Equal(t, 2, hist[0].Version)
	assert.Equal(t, 1, hist[1].Version)
	assert.Equal(t, "alice", hist[1].Entry.PublishedBy)
	assert.Equal(t, "Revue obligatoire", hist[1].Entry.Message)

	// Restore v1 as v3.
	pub, err = alice.Restore(ctx, Context{}, "ticket-hotfix", 1)
	require.NoError(t, err)
	assert.Equal(t, 3, pub.Version)
	assert.Equal(t, 1, pub.RestoredFrom)
	res, err = alice.Resolve(ctx, Context{}, "ticket-hotfix", ResolveOpts{})
	require.NoError(t, err)
	cp, _ := res.Spec.Checkpoints.Get("cp-2")
	assert.True(t, cp.IsMandatory(), "v1 content is back")
	assert.Equal(t, 3, res.Spec.Version)
	_, err = alice.Restore(ctx, Context{}, "ticket-hotfix", 9)
	assert.ErrorIs(t, err, ErrUnknownVersion)
	events, _ = os.ReadFile(filepath.Join(repo.Path(), "projects", teamstate.TeamEventsProject, "events", pub.At.Format("2006-01")+".jsonl"))
	assert.Contains(t, string(events), `"event":"workflow.restored"`)

	// Archive: gone from the catalogue, kept in history, next publication is v4.
	pub, err = alice.Archive(ctx, Context{}, "ticket-hotfix", "obsolète")
	require.NoError(t, err)
	assert.Equal(t, 3, pub.Version)
	assert.False(t, alice.Has(ctx, Context{}, "ticket-hotfix"))
	_, err = alice.Archive(ctx, Context{}, "ticket-hotfix", "")
	assert.ErrorIs(t, err, ErrNotPublished)
	saveDraft(t, alice, wf.LayerTeam, hotfixDraft)
	pub, err = alice.Publish(ctx, Context{}, "ticket-hotfix", "")
	require.NoError(t, err)
	assert.Equal(t, 4, pub.Version)

	integrity, err := alice.Integrity(ctx, Context{})
	require.NoError(t, err)
	assert.Empty(t, integrity)
}

func TestPublishRevalidates(t *testing.T) {
	env := newTeamEnv(t, "alice")
	alice := env.service(t, "alice")
	ctx := context.Background()
	saveDraft(t, alice, wf.LayerTeam, hotfixDraft)
	// The hub changes under the draft: the publication revalidates.
	alice.HubWorkflowsDir = t.TempDir()
	_, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "")
	var inv *InvalidError
	require.ErrorAs(t, err, &inv)
	repo := env.clones["alice"]
	assert.NoFileExists(t, filepath.Join(repo.Path(), "workflows", "published", "ticket-hotfix.yaml"), "nothing written")
	l, err := repo.ReadWorkflowLock()
	require.NoError(t, err)
	assert.Empty(t, l.Team)
	assert.FileExists(t, filepath.Join(repo.Path(), "workflows", "drafts", "alice", "ticket-hotfix.yaml"), "the draft is kept")
	assert.Equal(t, "", git(t, repo.Path(), "status", "--porcelain"))
}

func TestPublishGovernance(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	saveDraft(t, svc, wf.LayerTeam, hotfixDraft)
	mallory := env.service(t, "alice")
	mallory.TeamState = func(context.Context, Context) (*TeamState, error) {
		return &TeamState{Repo: env.clones["alice"], Member: "mallory", Project: "web"}, nil
	}
	_, err := mallory.Publish(context.Background(), Context{}, "team:ticket-hotfix", "")
	assert.Error(t, err)
	_, err = svc.Publish(context.Background(), Context{}, "nope", "")
	assert.ErrorIs(t, err, ErrNoDraft)
}

// Acceptance criterion 3: two members publish the same workflow from stale
// clones: the second one is rebuilt on top of the first (v+1, revalidated).
func TestConcurrentPublications(t *testing.T) {
	env := newTeamEnv(t, "alice", "bob")
	alice, bob := env.service(t, "alice"), env.service(t, "bob")
	ctx := context.Background()

	saveDraft(t, alice, wf.LayerTeam, hotfixDraft)
	saveDraft(t, bob, wf.LayerTeam, strings.Replace(hotfixDraft, "description: Hotfix", "description: Hotfix de Bob", 1))

	pa, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "alice")
	require.NoError(t, err)
	assert.Equal(t, 1, pa.Version)
	// Bob's clone has not seen Alice's publication yet.
	pb, err := bob.Publish(ctx, Context{}, "ticket-hotfix", "bob")
	require.NoError(t, err)
	assert.Equal(t, 2, pb.Version, "rebuilt on top of v1")
	assert.False(t, pb.Impact.New)
	assert.Contains(t, git(t, env.bare, "log", "--format=%s", "-3"), "v2 by bob")

	require.NoError(t, env.clones["alice"].Pull(ctx))
	res, err := alice.Resolve(ctx, Context{}, "ticket-hotfix", ResolveOpts{})
	require.NoError(t, err)
	assert.Equal(t, "Hotfix de Bob", res.Spec.Description.Text("fr"))
	hist, err := alice.History(ctx, Context{}, "ticket-hotfix")
	require.NoError(t, err)
	assert.Equal(t, []int{2, 1}, []int{hist[0].Version, hist[1].Version})
	integrity, _ := alice.Integrity(ctx, Context{})
	assert.Empty(t, integrity)
}

// O14: offline, the publication is queued and replayed later.
func TestPublishOfflineQueue(t *testing.T) {
	env := newTeamEnv(t, "alice")
	alice := env.service(t, "alice")
	ctx := context.Background()
	repo := env.clones["alice"]
	saveDraft(t, alice, wf.LayerTeam, hotfixDraft)

	git(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/team-state.git")
	pub, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "offline")
	require.NoError(t, err)
	assert.True(t, pub.Queued)
	q, err := repo.Queue()
	require.NoError(t, err)
	require.Len(t, q, 1)
	assert.Equal(t, OpPublish, q[0].Kind)
	assert.FileExists(t, filepath.Join(repo.Path(), "workflows", "drafts", "alice", "ticket-hotfix.yaml"), "draft kept")
	assert.False(t, alice.Has(ctx, Context{}, "team:ticket-hotfix"))

	// Still offline: nothing replayed.
	pubs, errs := alice.FlushQueue(ctx, Context{})
	assert.Empty(t, pubs)
	assert.Empty(t, errs)

	git(t, repo.Path(), "remote", "set-url", "origin", env.bare)
	pubs, errs = alice.FlushQueue(ctx, Context{})
	require.Empty(t, errs)
	require.Len(t, pubs, 1)
	assert.Equal(t, 1, pubs[0].Version)
	q, _ = repo.Queue()
	assert.Empty(t, q)
	assert.True(t, alice.Has(ctx, Context{}, "team:ticket-hotfix"))
}

func TestProjectLayerDraft(t *testing.T) {
	env := newTeamEnv(t, "alice")
	alice := env.service(t, "alice")
	ctx := context.Background()
	saveDraft(t, alice, wf.LayerTeam, hotfixDraft+"enforce: [checkpoints]\n")
	_, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "")
	require.NoError(t, err)

	// A project patch writing a locked field is refused at save time.
	_, err = alice.SaveDraft(ctx, Context{ProjectID: "web"}, DraftInput{Layer: wf.LayerProject,
		YAML: []byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: team:ticket-hotfix\ncheckpoints:\n  cp-2: { label: x }\n")})
	var inv *InvalidError
	require.True(t, errors.As(err, &inv))
	assert.Contains(t, inv.Diagnostics.Codes(), "enforced_field")

	saveDraft(t, alice, wf.LayerProject, "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: team:ticket-hotfix\ndescription: Web\n")
	pub, err := alice.Publish(ctx, Context{}, "ticket-hotfix", "")
	require.NoError(t, err)
	assert.Equal(t, "project:ticket-hotfix", pub.Ref.String(), "bare id: project draft first")
	assert.FileExists(t, filepath.Join(env.clones["alice"].Path(), "projects", "web", "workflows", "published", "ticket-hotfix.yaml"))
	events, _ := os.ReadFile(filepath.Join(env.clones["alice"].Path(), "projects", "web", "events", pub.At.Format("2006-01")+".jsonl"))
	assert.Contains(t, string(events), "workflow.published")
}
