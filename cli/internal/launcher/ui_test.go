package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCLIUI(t *testing.T) {
	ui := NewCLIUI(nil)
	assert.Nil(t, ui.SuspendAndExec(), "CLI UI should have nil SuspendAndExec")
}

func TestNewTUIUI(t *testing.T) {
	called := false
	suspendFn := func(fn func() error) error {
		called = true
		return fn()
	}
	ui := NewTUIUI(suspendFn, nil)
	assert.NotNil(t, ui.SuspendAndExec(), "TUI UI should have non-nil SuspendAndExec")

	// Execute through the suspend
	err := ui.SuspendAndExec()(func() error { return nil })
	assert.NoError(t, err)
	assert.True(t, called)
}
