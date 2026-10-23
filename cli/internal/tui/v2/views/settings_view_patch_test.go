package views

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

// The settings view saves what the edits changed, on hub.toml as it is, and
// an undo gives back that edit only.
func TestSettingsViewSavesTheEditsAndUndoesOnlyThem(t *testing.T) {
	shared := seedHome(t)
	v := NewSettingsView(SettingsViewConfig{
		GetConfig:    func() *config.Config { return shared },
		UpdateConfig: hubHost{shared}.update,
	})
	v.live = shared

	// the wizard (or another process) wrote while the view was open
	wizardWrites(t)

	edit := func(f func(c *config.Config)) {
		v.pushUndo()
		f(v.live)
		v.scheduleAutoSave()
		v.doSave()
	}
	edit(func(c *config.Config) { c.Worktree.BaseBranch = "develop" })

	got := requireWizardFields(t)
	assert.Equal(t, "develop", got.Worktree.BaseBranch)
	assert.True(t, shared.CLI.SetupDone, "the live config follows the file")

	// another process changes a setting; the undo gives back the edit and nothing else
	require.NoError(t, config.Update(func(c *config.Config) error { c.Websearch.Enabled = true; return nil }))
	v.undo()

	got = requireWizardFields(t)
	assert.Empty(t, got.Worktree.BaseBranch, "the undo reverts the edit")
	assert.True(t, got.Websearch.Enabled, "and not what the file gained since")
}

func TestSettingsViewKeepsThePendingEditsWhenTheSaveFails(t *testing.T) {
	shared := seedHome(t)
	fail := true
	v := NewSettingsView(SettingsViewConfig{
		GetConfig: func() *config.Config { return shared },
		UpdateConfig: func(fn func(*config.Config) error) error {
			if fail {
				return os.ErrPermission
			}
			return hubHost{shared}.update(fn)
		},
	})
	v.live = shared

	v.pushUndo()
	v.live.Worktree.BaseBranch = "develop"
	v.scheduleAutoSave()
	v.doSave()
	got, err := config.Read()
	require.NoError(t, err)
	assert.Empty(t, got.Worktree.BaseBranch)

	fail = false
	v.doSave()
	got, err = config.Read()
	require.NoError(t, err)
	assert.Equal(t, "develop", got.Worktree.BaseBranch, "the edit is saved by the next save")
}
