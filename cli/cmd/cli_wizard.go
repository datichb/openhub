package cmd

import (
	"bytes"
	"io"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// cliWizard is the wizard of a command (oh provider setup, oh project add…):
// the command it belongs to and its steps.
type cliWizard struct {
	Command string
	Steps   []views.WizardStep
}

// cliWizardResult is how a CLI wizard ended: every step done (Completed), the
// user quitting (Aborted), or a step failing and the user giving up (Err).
type cliWizardResult = views.StandaloneResult

// runCLIWizard runs the wizard of a command and waits for its end. It is a
// variable so that the tests answer the steps without a terminal.
var runCLIWizard = runCLIWizardOnTerminal

// runCLIWizardOnTerminal runs the wizard of a command with the engine of the
// TUI (InlineWizardView) alone on the terminal: Esc asks whether to quit, and
// the steps are checked and notify as they are in the TUI. What a step prints
// (it writes to the streams of the app) would be drawn over the screen: it is
// held, and written when the wizard has left the terminal.
func runCLIWizardOnTerminal(a *app.App, w cliWizard) cliWizardResult {
	return runCLIWizardOn(a, w, nil)
}

// runCLIWizardOn is runCLIWizardOnTerminal on a given screen (nil: the
// terminal), for the tests that send keys to a simulation screen.
func runCLIWizardOn(a *app.App, w cliWizard, screen tcell.Screen) cliWizardResult {
	held := holdOutput(a.IO)
	defer held.release()
	return views.RunInlineWizardStandaloneOn(views.NewInlineWizardView(cliWizardConfig(w)), screen)
}

// cliWizardConfig is the configuration of the view of a CLI wizard. The steps
// are not copied: a step finds its own Rerender in the slice it built.
func cliWizardConfig(w cliWizard) views.InlineWizardConfig {
	return views.InlineWizardConfig{
		ID:    "wizard.cli." + strings.ReplaceAll(w.Command, " ", "."),
		Title: w.Command,
		Steps: w.Steps,
	}
}

// heldOutput keeps what is written to the streams of the app while a wizard
// owns the terminal.
type heldOutput struct {
	streams *app.IOStreams
	out     io.Writer
	errOut  io.Writer
	outBuf  lockedBuffer
	errBuf  lockedBuffer
}

// holdOutput redirects the output streams of streams (nil: nothing to hold)
// to buffers until release.
func holdOutput(streams *app.IOStreams) *heldOutput {
	h := &heldOutput{streams: streams}
	if streams != nil {
		h.out, h.errOut = streams.Out, streams.ErrOut
		streams.Out, streams.ErrOut = &h.outBuf, &h.errBuf
	}
	return h
}

// release gives the streams back and writes what was held, standard output
// first.
func (h *heldOutput) release() {
	if h.streams == nil {
		return
	}
	h.streams.Out, h.streams.ErrOut = h.out, h.errOut
	if h.out != nil {
		_, _ = h.out.Write(h.outBuf.Bytes())
	}
	if h.errOut != nil {
		_, _ = h.errOut.Write(h.errBuf.Bytes())
	}
}

// lockedBuffer is a bytes.Buffer that the goroutine of a step and the end of
// the wizard may use together.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}
