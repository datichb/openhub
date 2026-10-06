package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

type recordUI struct{ notes []string }

func (u *recordUI) Confirm(string) (bool, error)             { return true, nil }
func (u *recordUI) Notify(msg string, _ launcher.Level)      { u.notes = append(u.notes, msg) }
func (u *recordUI) SuspendAndExec() func(func() error) error { return nil }

// No terminal could be opened and oh run has none to take over (script,
// IDE): the session is left running and the user is told how to attach,
// instead of a client blocking oh run (v5 finalisation, Q3-3).
func TestAfterStartWithoutTerminal(t *testing.T) {
	prev := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = prev })
	stdinIsTerminal = func() bool { return false }

	ui := &recordUI{}
	res := &runsvc.StartResult{SessionID: "ses_1", AttachErr: errors.New("no terminal available to open the session")}
	require.NoError(t, afterStart(context.Background(), nil, nil, ui, sessionspec.AttachPref("iterm"), res, true))
	assert.Equal(t, []string{i18n.T("cmd.session.no_terminal"), i18n.Tf("cmd.run.attach_later", "ses_1")}, ui.notes)
}
