package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// terminalIs sets whether stdin is a terminal for the test.
func terminalIs(t *testing.T, on bool) {
	t.Helper()
	prev := stdinIsTerminal
	stdinIsTerminal = func() bool { return on }
	t.Cleanup(func() { stdinIsTerminal = prev })
}

// formAnswers replaces the confirmation form: each call takes the next answer
// (nil error) and records the question and the value the form starts with.
type formAnswers struct {
	answers   []any // bool, or error
	questions []string
	initial   []bool
}

func stubConfirmForm(t *testing.T, answers ...any) *formAnswers {
	t.Helper()
	fa := &formAnswers{answers: answers}
	prev := confirmForm
	confirmForm = func(question, _ string, ok *bool) error {
		fa.questions = append(fa.questions, question)
		fa.initial = append(fa.initial, *ok)
		if len(fa.answers) == 0 {
			t.Fatalf("unexpected question: %s", question)
		}
		a := fa.answers[0]
		fa.answers = fa.answers[1:]
		if err, isErr := a.(error); isErr {
			return err
		}
		*ok = a.(bool)
		return nil
	}
	t.Cleanup(func() { confirmForm = prev })
	return fa
}

// yesCmd is a command with the --yes flag, parsed with args.
func yesCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "x"}
	addYesFlag(c)
	require.NoError(t, c.ParseFlags(args))
	return c
}

func TestConfirmActionYesFlagAnswersWithoutQuestion(t *testing.T) {
	terminalIs(t, false)
	stubConfirmForm(t) // any question fails the test
	for _, args := range [][]string{{"--yes"}, {"-y"}} {
		ok, err := confirmAction(yesCmd(t, args...), "Go?", false)
		require.NoError(t, err)
		assert.True(t, ok, args)
	}
	terminalIs(t, true)
	ok, err := confirmAction(yesCmd(t, "--yes"), "Go?", false)
	require.NoError(t, err)
	assert.True(t, ok, "with a terminal too: --yes never asks")
}

func TestConfirmActionWithoutTerminalIsRefused(t *testing.T) {
	terminalIs(t, false)
	stubConfirmForm(t)
	for lang, msg := range map[string]string{
		"en": "confirmation required: run again with --yes",
		"fr": "confirmation requise : relancez avec --yes",
	} {
		t.Run(lang, func(t *testing.T) {
			i18ntest.Set(t, lang)
			ok, err := confirmAction(yesCmd(t), "Go?", true)
			require.Error(t, err)
			assert.False(t, ok)
			assert.Equal(t, msg, err.Error())
			assert.Equal(t, ExitUsage, ExitCode(err), "refusal is a usage error")
		})
	}
}

func TestConfirmActionCommandWithoutYesFlag(t *testing.T) {
	terminalIs(t, false)
	_, err := confirmAction(&cobra.Command{Use: "x"}, "Go?", true)
	require.Error(t, err, "no --yes flag: nothing can answer")
	assert.Equal(t, ExitUsage, ExitCode(err))
}

func TestConfirmActionWithTerminalAsksTheForm(t *testing.T) {
	terminalIs(t, true)
	cases := []struct {
		name       string
		answer     any
		defaultYes bool
		want       bool
		wantErr    bool
	}{
		{"yes", true, false, true, false},
		{"no", false, true, false, false},
		{"ctrl-c is a no", huh.ErrUserAborted, true, false, false},
		{"other error", errors.New("boom"), true, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fa := stubConfirmForm(t, c.answer)
			ok, err := confirmAction(yesCmd(t), "Go?", c.defaultYes)
			assert.Equal(t, c.want, ok)
			assert.Equal(t, c.wantErr, err != nil)
			assert.Equal(t, []string{"Go?"}, fa.questions)
			assert.Equal(t, []bool{c.defaultYes}, fa.initial, "the form starts on the default")
		})
	}
}

// The buttons of the form are in the language of the hub (no « Yes/No »).
func TestConfirmButtonsAreTranslated(t *testing.T) {
	i18ntest.Set(t, "fr")
	assert.Equal(t, "Oui", i18n.T("cmd.confirm.yes"))
	assert.Equal(t, "Non", i18n.T("cmd.confirm.no"))
	i18ntest.Set(t, "en")
	assert.Equal(t, "Yes", i18n.T("cmd.confirm.yes"))
	assert.Equal(t, "No", i18n.T("cmd.confirm.no"))
}

// ── non-regression, one command per family ───────────────────────────────

func runSub(t *testing.T, c *cobra.Command, args ...string) (string, error) {
	t.Helper()
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

// Family « workflow » (confirm() of workflow_teamcli.go): archive, restore
// and publish --adopt were silently confirmed without a terminal.
func TestWorkflowConfirmationsWithoutTerminal(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	terminalIs(t, false)
	stubConfirmForm(t)

	_, err := runWorkflowSub(t, workflowArchiveCmd, "ticket-hotfix", "-m", "old")
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.ErrorContains(t, err, "--yes")

	_, err = runWorkflowSub(t, workflowRestoreCmd, "ticket-hotfix", "1")
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))

	_, err = runWorkflowSub(t, brickPublishCmd, "--adopt", "-m", "adopt")
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))

	// With --yes the confirmation is passed: the command goes on and fails
	// later for its own reasons (no such workflow), not for the confirmation.
	_, err = runWorkflowSub(t, workflowArchiveCmd, "ticket-hotfix", "-m", "old", "--yes")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "--yes")
	assert.NotEqual(t, ExitUsage, ExitCode(err))
}

// workflow publish asks only when the publication widens the workflow (here,
// a mandatory checkpoint is removed): refused without a terminal, unless
// --yes. It used to go on silently.
func TestWorkflowPublishWideningNeedsYes(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	i18ntest.Set(t, "en")
	terminalIs(t, false)
	stubConfirmForm(t)
	write := func(name, body string) string {
		p := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		return p
	}
	out, err := runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--extends", "hub:ticket", "--no-edit")
	require.NoError(t, err, out)
	v1 := write("v1.yaml", "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\ncheckpoints:\n  cp-2: { label: Revue, mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }\n")
	_, err = runWorkflowSub(t, workflowEditCmd, "ticket-hotfix", "--file", v1)
	require.NoError(t, err)
	out, err = runWorkflowSub(t, workflowPublishCmd, "ticket-hotfix", "-m", "v1")
	require.NoError(t, err, "no widening, no question: "+out)

	v2 := write("v2.yaml", "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\ndescription: v2\n")
	_, err = runWorkflowSub(t, workflowEditCmd, "ticket-hotfix", "--file", v2)
	require.NoError(t, err)
	out, err = runWorkflowSub(t, workflowPublishCmd, "team:ticket-hotfix", "-m", "v2")
	require.Error(t, err, out)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.ErrorContains(t, err, "--yes")
	hist, err := runWorkflowSub(t, workflowHistoryCmd, "ticket-hotfix")
	require.NoError(t, err)
	assert.NotContains(t, hist, "v2", "nothing was published")

	out, err = runWorkflowSub(t, workflowPublishCmd, "team:ticket-hotfix", "-m", "v2", "--yes")
	require.NoError(t, err, out)
}

type recordingProjects struct {
	mockProjectStore
	deleted []string
}

func (r *recordingProjects) Delete(_ context.Context, id string) error {
	r.deleted = append(r.deleted, id)
	return nil
}

// Family « huh.NewConfirm » (project remove): the refusal used to be silent.
func TestProjectRemoveConfirmation(t *testing.T) {
	setup := func(t *testing.T) (*recordingProjects, *bytes.Buffer) {
		t.Helper()
		store := &recordingProjects{mockProjectStore: mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", Path: t.TempDir(), Status: domain.ProjectStatusActive}}}}
		io, out, _ := testIO()
		application = &app.App{Config: &config.Config{}, Projects: store, IO: io}
		t.Cleanup(func() { application = nil })
		return store, out
	}

	t.Run("without terminal and without --yes", func(t *testing.T) {
		terminalIs(t, false)
		stubConfirmForm(t)
		store, _ := setup(t)
		_, err := runSub(t, projectRemoveCmd(), "p1")
		require.Error(t, err)
		assert.Equal(t, ExitUsage, ExitCode(err))
		assert.Empty(t, store.deleted)
	})
	for _, flag := range []string{"--yes", "--force"} {
		t.Run("without terminal with "+flag, func(t *testing.T) {
			terminalIs(t, false)
			stubConfirmForm(t)
			store, out := setup(t)
			_, err := runSub(t, projectRemoveCmd(), "p1", flag)
			require.NoError(t, err)
			assert.Equal(t, []string{"p1"}, store.deleted)
			assert.Contains(t, out.String(), "web")
		})
	}
	t.Run("terminal, answer no", func(t *testing.T) {
		terminalIs(t, true)
		fa := stubConfirmForm(t, false)
		store, _ := setup(t)
		_, err := runSub(t, projectRemoveCmd(), "p1")
		require.NoError(t, err)
		assert.Empty(t, store.deleted)
		assert.Len(t, fa.questions, 1)
	})
	t.Run("terminal, answer yes", func(t *testing.T) {
		terminalIs(t, true)
		stubConfirmForm(t, true)
		store, _ := setup(t)
		_, err := runSub(t, projectRemoveCmd(), "p1")
		require.NoError(t, err)
		assert.Equal(t, []string{"p1"}, store.deleted)
	})
}

// purge: --yes (and its deprecated alias --force) skip every question (no backup is offered);
// without a terminal the purge is refused at the first question; with a
// terminal the backup is asked first, then the final confirmation.
func TestPurgeConfirmation(t *testing.T) {
	newPurge := func(t *testing.T, args ...string) *cobra.Command {
		t.Helper()
		c := &cobra.Command{Use: "purge"}
		addYesFlag(c)
		deprecateForceAsYes(c, false)
		require.NoError(t, c.ParseFlags(args))
		return c
	}

	t.Run("refused without terminal", func(t *testing.T) {
		terminalIs(t, false)
		stubConfirmForm(t)
		var out bytes.Buffer
		ok, err := confirmPurge(newPurge(t), &out)
		require.Error(t, err)
		assert.False(t, ok)
		assert.Equal(t, ExitUsage, ExitCode(err))
	})
	for _, flag := range []string{"--yes", "--force"} {
		t.Run(flag+" skips the questions and the backup", func(t *testing.T) {
			terminalIs(t, false)
			stubConfirmForm(t)
			var out bytes.Buffer
			ok, err := confirmPurge(newPurge(t, flag), &out)
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Empty(t, out.String())
		})
	}
	t.Run("terminal: backup first then final", func(t *testing.T) {
		i18ntest.Set(t, "en")
		terminalIs(t, true)
		fa := stubConfirmForm(t, false, true)
		var out bytes.Buffer
		ok, err := confirmPurge(newPurge(t), &out)
		require.NoError(t, err)
		assert.True(t, ok)
		require.Len(t, fa.questions, 2)
		assert.Contains(t, fa.questions[0], "backup", "the backup is proposed first")
		assert.Equal(t, []bool{false, false}, fa.initial, "no is the default of both questions")
	})
	t.Run("terminal: final refusal", func(t *testing.T) {
		terminalIs(t, true)
		stubConfirmForm(t, false, false)
		var out bytes.Buffer
		ok, err := confirmPurge(newPurge(t), &out)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.NotEmpty(t, out.String(), "cancellation is announced")
	})
}

// Family « migrate deploy-cleanup »: its own refusal carries the same code.
func TestMigrateCleanupConfirmation(t *testing.T) {
	prev := v5Adapter
	v5Adapter = fakeCleaner{}
	t.Cleanup(func() { v5Adapter = prev })
	deployed := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(deployed, "legacy"), 0o755))
	io, _, _ := testIO()
	application = &app.App{Config: &config.Config{}, IO: io, Projects: &mockProjectStore{projects: []domain.Project{
		{ID: "old", Name: "old", Path: deployed, Status: domain.ProjectStatusActive},
	}}}
	t.Cleanup(func() { application = nil })

	terminalIs(t, false)
	stubConfirmForm(t)
	c := migrateCleanupCmd
	require.NoError(t, c.Flags().Set("yes", "false"))
	_, err := runSub(t, c)
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.DirExists(t, filepath.Join(deployed, "legacy"), "nothing removed")

	require.NoError(t, c.Flags().Set("yes", "true"))
	t.Cleanup(func() { _ = c.Flags().Set("yes", "false") })
	_, err = runSub(t, c)
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(deployed, "legacy"))
}
