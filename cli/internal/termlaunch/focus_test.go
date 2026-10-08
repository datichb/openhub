package termlaunch

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFocus struct {
	scripts []string
	tmux    [][]string
	found   string // script containing this text answers "ok"
	tmuxErr error
	client  string // #{client_tty}
}

func (f *fakeFocus) install(t *testing.T) {
	t.Helper()
	oa, tm := osascriptOut, tmuxOut
	t.Cleanup(func() { osascriptOut, tmuxOut = oa, tm })
	osascriptOut = func(_ context.Context, s string) (string, error) {
		f.scripts = append(f.scripts, s)
		if f.found != "" && strings.Contains(s, f.found) {
			return "ok\n", nil
		}
		return "none\n", nil
	}
	tmuxOut = func(_ context.Context, args ...string) (string, error) {
		f.tmux = append(f.tmux, args)
		if f.tmuxErr != nil {
			return "", f.tmuxErr
		}
		if args[0] == "display-message" {
			return f.client, nil
		}
		return "", nil
	}
}

func TestFocusScriptsNeverStartATerminal(t *testing.T) {
	for _, s := range []string{iTermFocusScript("/dev/ttys003"), terminalFocusScript("/dev/ttys003")} {
		assert.True(t, strings.HasPrefix(s, `if application "`), s)
		assert.Contains(t, s, `is running then`)
		assert.Contains(t, s, `"/dev/ttys003"`)
		assert.Contains(t, s, `return "ok"`)
	}
	assert.Contains(t, iTermFocusScript("x"), `tell application "iTerm2"`)
	assert.Contains(t, terminalFocusScript("x"), `set index of w to 1`)
}

// A43: the window of an attached client is brought back instead of opening
// another one; the terminal of the client is asked first.
func TestFocusByTTY(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS terminals")
	}
	f := &fakeFocus{found: `tell application "Terminal"`}
	f.install(t)
	require.NoError(t, Focus(context.Background(), Location{Program: "Apple_Terminal", TTY: "/dev/ttys004"}))
	require.Len(t, f.scripts, 1, "Terminal.app asked first for its own client")

	f = &fakeFocus{found: `tell application "Terminal"`}
	f.install(t)
	require.NoError(t, Focus(context.Background(), Location{Program: "iTerm.app", TTY: "/dev/ttys004"}))
	require.Len(t, f.scripts, 2)
	assert.Contains(t, f.scripts[0], "iTerm2")

	f = &fakeFocus{}
	f.install(t)
	assert.ErrorIs(t, Focus(context.Background(), Location{TTY: "/dev/ttys004"}), ErrNotFound)
}

func TestFocusTmux(t *testing.T) {
	f := &fakeFocus{}
	f.install(t)
	require.NoError(t, Focus(context.Background(), Location{TmuxPane: "%3"}))
	require.GreaterOrEqual(t, len(f.tmux), 2)
	assert.Equal(t, []string{"select-window", "-t", "%3"}, f.tmux[0])
	assert.Equal(t, []string{"select-pane", "-t", "%3"}, f.tmux[1])
	assert.Empty(t, f.scripts, "no client tty: no terminal script")

	f = &fakeFocus{tmuxErr: errors.New("can't find pane")}
	f.install(t)
	assert.ErrorIs(t, Focus(context.Background(), Location{TmuxPane: "%3", TTY: "/dev/ttys001"}), ErrNotFound)
}

func TestFocusNothingToLocate(t *testing.T) {
	f := &fakeFocus{}
	f.install(t)
	assert.True(t, Location{Program: "iTerm.app"}.Empty())
	assert.ErrorIs(t, Focus(context.Background(), Location{Program: "iTerm.app"}), ErrNotFound)
	assert.Empty(t, f.scripts)
}
