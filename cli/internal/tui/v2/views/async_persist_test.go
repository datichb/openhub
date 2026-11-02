package views

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistRunsInlineWithoutAnApp(t *testing.T) {
	var order []string
	persist(nil, func(context.Context) error { order = append(order, "work"); return errors.New("boom") },
		func(err error) { order = append(order, "done:"+err.Error()) })
	assert.Equal(t, []string{"work", "done:boom"}, order)
}

func TestPersistRecoversAPanic(t *testing.T) {
	var got error
	persist(nil, func(context.Context) error { panic("x") }, func(err error) { got = err })
	require.Error(t, got)
	assert.Contains(t, got.Error(), "panic: x")
}

func TestPersistWritesOffTheLoopAndAnswersOnIt(t *testing.T) {
	app := runApp(t, tview.NewBox())
	release := make(chan struct{})
	result := make(chan error, 1)
	var answered bool
	persist(app, func(context.Context) error { <-release; return nil }, func(err error) {
		answered = true
		result <- err
	})
	assert.False(t, answered, "done does not run before the write ends")
	close(release)
	select {
	case err := <-result:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("done did not run")
	}
}
