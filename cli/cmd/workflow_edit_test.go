package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// workflowCLIEnv is an oh home with the repository hub, the test workflows
// of the WorkflowService and a team-state (bare remote + clone per member).
type workflowCLIEnv struct {
	bare   string
	clones map[string]string
}

func setupWorkflowCLI(t *testing.T, member string, others ...string) *workflowCLIEnv {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	root := repoRoot(t)
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	withoutV2(t, nil)
	t.Setenv(workflowsDirEnv, filepath.Join(root, "cli", "internal", "services", "workflow", "testdata", "workflows"))
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "T", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "T", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	// The repository root has the hub layout (agents/, skills/, permissions/).
	require.NoError(t, os.Symlink(root, filepath.Join(home, "hub")))

	env := &workflowCLIEnv{bare: t.TempDir(), clones: map[string]string{}}
	run := func(dir string, args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run(env.bare, "init", "--bare", "-b", "main")
	seed := filepath.Join(t.TempDir(), "seed")
	run(t.TempDir(), "clone", env.bare, seed)
	var mf strings.Builder
	for _, m := range append([]string{member}, others...) {
		mf.WriteString("[members." + m + "]\ndisplay_name = \"" + m + "\"\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(seed, "members.toml"), []byte(mf.String()), 0o644))
	require.NoError(t, teamstate.NewRepo(env.bare, seed).InitStructure(t.Context()))
	run(seed, "add", ".")
	run(seed, "commit", "-m", "init")
	run(seed, "push", "-u", "origin", "HEAD:main")
	for _, m := range append([]string{member}, others...) {
		dir := filepath.Join(t.TempDir(), m)
		run(t.TempDir(), "clone", env.bare, dir)
		env.clones[m] = dir
	}
	env.as(t, member)
	return env
}

// as installs the App of member (its clone as the active team).
func (e *workflowCLIEnv) as(t *testing.T, member string) {
	io, _, _ := testIO()
	application = &app.App{
		Config: &config.Config{Teams: []config.TeamConfig{{ID: "acme", Enabled: true, StateRepo: e.bare,
			StatePath: e.clones[member], MemberID: member}}},
		Projects: &mockProjectStore{},
		IO:       io,
	}
	t.Cleanup(func() { application = nil })
}

// runWorkflowSub runs a fresh `oh workflow <sub>` command.
func runWorkflowSub(t *testing.T, newCmd func() *cobra.Command, args ...string) (string, error) {
	t.Helper()
	c := newCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetContext(t.Context())
	require.NoError(t, c.ParseFlags(args))
	err := c.Args(c, c.Flags().Args())
	if err == nil {
		err = c.RunE(c, c.Flags().Args())
	}
	return out.String(), err
}

// Acceptance criterion 1 (CLI): ticket-hotfix extends hub:ticket, edited,
// compared, published; another member sees it; history, restore, archive.
func TestWorkflowEditingCLI(t *testing.T) {
	env := setupWorkflowCLI(t, "alice", "bob")

	_, err := runWorkflowSub(t, workflowNewCmd, "Bad_ID")
	require.Error(t, err)
	out, err := runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--extends", "hub:ticket", "--no-edit")
	require.NoError(t, err, out)
	assert.Contains(t, out, "team:ticket-hotfix")
	_, err = runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--no-edit")
	require.Error(t, err, "already exists")
	_, err = runWorkflowSub(t, workflowNewCmd, "x", "--extends", "hub:nope", "--no-edit")
	require.Error(t, err)

	// Edit through $EDITOR (a script appending a checkpoint).
	script := filepath.Join(t.TempDir(), "editor.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nprintf 'description: Hotfix\\ncheckpoints:\\n  cp-2: { label: Revue, mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }\\n' >> \"$1\"\n"), 0o755))
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "sh "+script)
	out, err = runWorkflowSub(t, workflowEditCmd, "ticket-hotfix")
	require.NoError(t, err, out)
	draft, err := os.ReadFile(filepath.Join(env.clones["alice"], "workflows", "drafts", "alice", "ticket-hotfix.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(draft), "cp-2")

	// Invalid file: refused, the draft is unchanged.
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\nrisk: nope\n"), 0o644))
	out, err = runWorkflowSub(t, workflowEditCmd, "ticket-hotfix", "--file", bad)
	require.Error(t, err)
	assert.Contains(t, out, "risk")

	out, err = runWorkflowSub(t, workflowDiffCmd, "ticket-hotfix")
	require.NoError(t, err, out)
	assert.Contains(t, out, "+description: Hotfix")
	assert.Contains(t, out, "ticket-hotfix@draft")

	_, err = runWorkflowSub(t, workflowPublishCmd, "ticket-hotfix")
	require.Error(t, err, "message required")
	out, err = runWorkflowSub(t, workflowPublishCmd, "ticket-hotfix", "-m", "Revue obligatoire", "--yes")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v1")

	// Bob sees it after a sync.
	env.as(t, "bob")
	c := exec.Command("git", "pull")
	c.Dir = env.clones["bob"]
	require.NoError(t, c.Run())
	out, err = runWorkflowSub(t, workflowShowCmd, "ticket-hotfix")
	require.NoError(t, err, out)
	assert.Contains(t, out, "team:ticket-hotfix")

	// Back to Alice: v2, history, restore, archive.
	env.as(t, "alice")
	v2 := filepath.Join(t.TempDir(), "v2.yaml")
	require.NoError(t, os.WriteFile(v2, []byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\ndescription: Hotfix v2\n"), 0o644))
	_, err = runWorkflowSub(t, workflowEditCmd, "ticket-hotfix", "--file", v2)
	require.NoError(t, err)
	out, err = runWorkflowSub(t, workflowDiffCmd, "ticket-hotfix")
	require.NoError(t, err, out)
	assert.Contains(t, out, "cp-2", "the removed checkpoint shows in the impact")
	out, err = runWorkflowSub(t, workflowPublishCmd, "team:ticket-hotfix", "-m", "v2", "-y")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v2")

	out, err = runWorkflowSub(t, workflowHistoryCmd, "ticket-hotfix")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Revue obligatoire")
	assert.Contains(t, out, "alice")
	out, err = runWorkflowSub(t, workflowHistoryCmd, "ticket-hotfix", "--json")
	require.NoError(t, err)
	assert.Contains(t, out, `"current": true`)

	_, err = runWorkflowSub(t, workflowRestoreCmd, "ticket-hotfix", "x")
	require.Error(t, err)
	out, err = runWorkflowSub(t, workflowRestoreCmd, "ticket-hotfix", "1", "--yes")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v3")

	out, err = runWorkflowSub(t, workflowArchiveCmd, "ticket-hotfix", "-m", "obsolète", "--yes")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v3")
	_, err = runWorkflowSub(t, workflowShowCmd, "team:ticket-hotfix")
	require.Error(t, err)

	out, err = runWorkflowSub(t, workflowPublishCmd, "--retry")
	require.NoError(t, err, out)

	// Copy of a hub workflow under a new id, with its prompt template.
	out, err = runWorkflowSub(t, workflowNewCmd, "ticket-copy", "--copy", "hub:ticket", "--no-edit")
	require.NoError(t, err, out)
	copied, err := os.ReadFile(filepath.Join(env.clones["alice"], "workflows", "drafts", "alice", "ticket-copy.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(copied), "id: ticket-copy")
	assert.FileExists(t, filepath.Join(env.clones["alice"], "workflows", "drafts", "alice", "ticket-copy.prompt.md.tmpl"))

	// Empty skeleton: valid as is.
	out, err = runWorkflowSub(t, workflowNewCmd, "blank", "--no-edit")
	require.NoError(t, err, out)
}

func TestWorkflowEditingWithoutTeam(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	application.Config.Teams = nil
	_, err := runWorkflowSub(t, workflowNewCmd, "x", "--no-edit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oh team init")
	_, err = runWorkflowSub(t, workflowNewCmd, "x", "--layer", "hub")
	require.Error(t, err)
}

func TestWorkflowEditingSoloDefault(t *testing.T) {
	env := setupWorkflowCLI(t, "alice")
	// Only a solo space: it is the default team-state of the commands.
	application.Config.Teams = []config.TeamConfig{{ID: "solo", Enabled: true, Solo: true, StatePath: env.clones["alice"], MemberID: "alice"}}
	out, err := runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--extends", "hub:ticket", "--no-edit")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, workflowNewCmd, "other", "--team", "solo", "--no-edit")
	require.NoError(t, err, out)
	_, err = runWorkflowSub(t, workflowNewCmd, "x", "--team", "nope", "--no-edit")
	require.Error(t, err)
	_, err = runWorkflowSub(t, workflowNewCmd, "x", "--team", "solo", "--project", "p", "--no-edit")
	require.Error(t, err)
}

// oh workflow new --file with a document naming a template that does not
// exist yet: a starter template is created, or the --prompt-file one (v5
// finalisation, Q3-7: refused before).
func TestWorkflowNewFileWithOwnTemplate(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	doc := "apiVersion: oh/v1\nkind: Workflow\nid: %s\nrisk: write\nentry:\n  agent: developer\n" +
		"inputs:\n  request:\n    type: text\n    max_length: 4000\n  ticket:\n    type: beads-id\n" +
		"prompt:\n  template: prompts/%[1]s.md.tmpl\nagents:\n  developer: { role: workflow, mode: primary }\n"

	out, err := runWorkflowSub(t, workflowNewCmd, "hotfix", "--file", writeTemp(t, fmt.Sprintf(doc, "hotfix")), "--no-edit")
	require.NoError(t, err, out)
	assert.Contains(t, out, i18n.T("cmd.workflow.new.starter_prompt"))
	svc := newWorkflowService(t.Context())
	c, err := workflowCmdContext(workflowNewCmd())
	require.NoError(t, err)
	txt, err := svc.EditText(t.Context(), c, workflow.LayerTeam, "hotfix")
	require.NoError(t, err)
	assert.Contains(t, string(txt.Prompt), `{{ data "request" .request }}`, "free text delimited (O11)")
	assert.Contains(t, string(txt.Prompt), "{{ .ticket }}")

	prompt := filepath.Join(t.TempDir(), "p.md.tmpl")
	require.NoError(t, os.WriteFile(prompt, []byte("Mode de workflow : {{ .oh.mode }}\nFais : {{ data \"request\" .request }}\n"), 0o644))
	out, err = runWorkflowSub(t, workflowNewCmd, "hotfix2", "--file", writeTemp(t, fmt.Sprintf(doc, "hotfix2")), "--prompt-file", prompt, "--no-edit")
	require.NoError(t, err, out)
	assert.NotContains(t, out, i18n.T("cmd.workflow.new.starter_prompt"))
	txt, err = svc.EditText(t.Context(), c, workflow.LayerTeam, "hotfix2")
	require.NoError(t, err)
	assert.Contains(t, string(txt.Prompt), "Fais : ")
}

// oh run on a workflow with a required input given by --tickets: the
// first resolution (to learn the ticket input) must not check the inputs
// (v5 finalisation, Q4 recette: « Missing required input: ticket » for
// every such workflow since 3.E).
func TestPrepareRunRequiredTicketInput(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	var errOut bytes.Buffer
	opts := runOptions{Workflow: "ticket", Tickets: []string{"bd-1", "bd-2"}, Project: &domain.Project{ID: "p1", Name: "p", Path: t.TempDir()}}
	res, input, perSession, err := resolveLaunch(t.Context(), application, &opts, &errOut)
	require.NoError(t, err, errOut.String())
	assert.Equal(t, "ticket", input)
	assert.Equal(t, []string{"bd-1", "bd-2"}, perSession)
	assert.Equal(t, "bd-1", res.Inputs["ticket"])
}
