package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaunchUISuspend(t *testing.T) {
	cli := NewCLIUI(nil)
	assert.Nil(t, cli.SuspendAndExec(), "the CLI UI does not suspend anything")

	called := false
	suspendFn := func(fn func() error) error {
		called = true
		return fn()
	}
	tui := NewTUIUI(suspendFn, nil)
	require.NotNil(t, tui.SuspendAndExec(), "the TUI UI suspends through its function")

	err := tui.SuspendAndExec()(func() error { return nil })
	assert.NoError(t, err)
	assert.True(t, called, "the command runs through the suspend function")
}
