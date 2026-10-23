package cmd

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cliConfirm (the offers made on a terminal) asks the common form, no by
// default, and Ctrl+C is a no.
func TestCLIConfirmUsesTheCommonForm(t *testing.T) {
	fa := stubConfirmForm(t, true, huh.ErrUserAborted)
	ok, err := cliConfirm(&bytes.Buffer{}, "Go?")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []bool{false}, fa.initial)
	ok, err = cliConfirm(&bytes.Buffer{}, "Go?")
	require.NoError(t, err)
	assert.False(t, ok)
}
