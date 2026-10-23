package views

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// The catalogue is read off the event loop: it says « Chargement… » rather
// than « empty » until the data arrives, shows a failure with the common
// words, and drops the answer of a read made before an unmount or before a
// newer read (run with -race).
func TestWorkflowCatalogLoadStates(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewWorkflowCatalogView(WorkflowCatalogConfig{
		Load: func(context.Context) (CatalogData, error) {
			switch calls.Add(1) {
			case 1:
				close(first)
				<-release
				return CatalogData{Entries: []CatalogEntry{{ID: "ancien", Ref: "hub:ancien", Layer: "hub", Valid: true}}}, nil
			case 2:
				return CatalogData{}, errors.New("hub illisible")
			}
			return CatalogData{Entries: []CatalogEntry{{ID: "recent", Ref: "hub:recent", Layer: "hub", Valid: true}}}, nil
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	text := func() (txt string) {
		onLoop(loop, func() {
			if v.list == nil {
				return
			}
			var parts []string
			for _, it := range v.list.GetItems() {
				parts = append(parts, it.MainText)
			}
			txt = strings.Join(parts, "\n")
		})
		return
	}

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, text(), "Chargement", "not « nothing to show » while the catalogue is read")
	onLoop(loop, func() { v.Refresh() }) // a newer read: an error
	require.Eventually(t, func() bool { return strings.Contains(text(), "Impossible de charger : hub illisible") }, 2*time.Second, 10*time.Millisecond)
	close(release) // the older read answers late
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, text(), "ancien", "the answer of an older read is dropped")

	onLoop(loop, func() { v.Unmount() })
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(text(), "recent") }, 2*time.Second, 10*time.Millisecond)
}

// The preview of a publication is read in load(): the failure is shown in
// the body, and the preview of a view left meanwhile is not applied.
func TestPublishViewLoadFailureAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	v := NewPublishView(PublishViewConfig{Ref: "team:x", Load: func(context.Context) (*PublishPreview, error) {
		switch calls.Add(1) {
		case 1:
			close(started)
			<-release
			return &PublishPreview{Ref: "team:x", Next: 1, Valid: true}, nil
		}
		return nil, errors.New("équipe injoignable")
	}})
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	body := func() (txt string) {
		onLoop(loop, func() {
			if v.body != nil {
				txt = v.body.GetText(true)
			}
		})
		return
	}
	onLoop(loop, func() { v.Mount(content, loop) })
	<-started
	assert.Contains(t, body(), "Calcul de l'aperçu")
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(body(), "Impossible de charger : équipe injoignable") }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, body(), "team:x", "the preview of the previous mount is dropped")
}

// Edits in a row are checked once, and the check of a text that changed
// meanwhile is not applied: the editor shows the check of the last text.
func TestWorkflowEditorStaleCheckIsIgnored(t *testing.T) {
	var n atomic.Int32
	slow, release := make(chan struct{}), make(chan struct{})
	cfg := WorkflowEditorConfig{
		Ref: "team:x", Layer: workflow.LayerTeam, YAML: []byte(editorDoc),
		Check: func(_ context.Context, yaml, _ []byte) (*EditorCheck, error) {
			if n.Add(1) == 1 {
				close(slow)
				<-release
				return &EditorCheck{Bundle: []string{"ancien"}}, nil
			}
			return &EditorCheck{Bundle: []string{"recent"}}, nil
		},
	}
	v := NewWorkflowEditorView(cfg)
	v.SetShell(&editorShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	<-slow
	onLoop(loop, func() { v.recheck(0) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = v.check != nil && len(v.check.Bundle) == 1 && v.check.Bundle[0] == "recent" })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.Equal(t, []string{"recent"}, v.check.Bundle, "the older check does not replace the newer")
	})
}
