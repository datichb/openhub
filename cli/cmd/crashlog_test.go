package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// crashingWidget panics on a click, like a widget with a nil field would.
type crashingWidget struct{ *tview.Box }

func (w *crashingWidget) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, _ *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if action == tview.MouseLeftClick {
			explodeInTheHandler()
		}
		return false, nil
	}
}

//go:noinline
func explodeInTheHandler() {
	var m map[string]int
	m["x"] = 1 // assignment to entry in nil map
}

func setTestLocale(t *testing.T, lang string) {
	t.Helper()
	i18n.SetLocale(lang)
	t.Cleanup(func() { i18n.SetLocale("en") })
}

// A panic of the event loop of tview is recovered there (to restore the
// terminal) and raised again: the stack taken in the recover of Execute
// still holds the frames of the first panic, and the crash file keeps it.
func TestReportPanicKeepsTheStackOfTheEventLoop(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	setTestLocale(t, "en")

	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	sim.SetSize(40, 10)
	app := tview.NewApplication().SetScreen(sim).EnableMouse(true)
	app.SetRoot(&crashingWidget{Box: tview.NewBox()}, true)

	var stopClicks atomic.Bool
	defer stopClicks.Store(true)
	go func() {
		for !stopClicks.Load() {
			sim.InjectMouse(2, 2, tcell.Button1, tcell.ModNone)
			sim.InjectMouse(2, 2, tcell.ButtonNone, tcell.ModNone)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	var out bytes.Buffer
	func() {
		defer func() {
			r := recover()
			require.NotNil(t, r, "the application panics")
			reportPanic(&out, r, debug.Stack())
		}()
		_ = app.Run()
	}()

	msg := out.String()
	assert.Contains(t, msg, "internal error (panic): assignment to entry in nil map")
	assert.Contains(t, msg, "Please report this bug")

	files, err := filepath.Glob(filepath.Join(ohRunDir(), "crash-*.log"))
	require.NoError(t, err)
	require.Len(t, files, 1, "one crash file in the run folder of the hub")
	assert.Contains(t, msg, files[0], "the message names the file")
	assert.Contains(t, msg, "attach this file")

	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "panic: assignment to entry in nil map")
	assert.Contains(t, body, "explodeInTheHandler", "the function that panicked")
	assert.Contains(t, body, "crashingWidget", "the widget that was clicked")
	assert.Contains(t, body, "tview.(*Application).fireMouseActions", "the way down from the event loop")
	assert.Contains(t, body, "oh dev")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(files[0])
		require.NoError(t, err)
		assert.Zero(t, info.Mode().Perm()&0o077, "the crash file is private to the user")
	}
}

func TestReportPanicWithoutAWritableFolder(t *testing.T) {
	home := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(home, []byte("x"), 0o600))
	t.Setenv("OH_HOME", home) // run/ cannot be created under a file
	setTestLocale(t, "en")

	var out bytes.Buffer
	reportPanic(&out, "boom", []byte("goroutine 1 [running]:\nmain.fn()\n"))

	msg := out.String()
	assert.Contains(t, msg, "Could not write the crash report")
	assert.Contains(t, msg, "main.fn()", "the stack goes to the terminal instead")
	assert.Contains(t, msg, "Please report this bug")
}

func TestReportPanicInFrench(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	setTestLocale(t, "fr")

	var out bytes.Buffer
	reportPanic(&out, "boom", []byte("stack"))

	msg := out.String()
	assert.Contains(t, msg, "erreur interne (panic) : boom")
	assert.Contains(t, msg, "Rapport de plantage écrit dans ")
	assert.Contains(t, msg, "joignez ce fichier")
}
