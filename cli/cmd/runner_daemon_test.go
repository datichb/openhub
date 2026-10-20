package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func waitJobDaemon(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("the end of the job daemon was not reported")
		return nil
	}
}

// A-02: the daemon of a remote job runs through safego. A panic does not end
// the process: it is reported as the end of the daemon, which waitDaemon
// returns instead of waiting for it.
func TestRunJobDaemonSurvivesAPanic(t *testing.T) {
	done := runJobDaemon(context.Background(), func(context.Context) error { panic("boom") })
	err := waitJobDaemon(t, done)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error")
}

func TestRunJobDaemonReportsItsEnd(t *testing.T) {
	boom := errors.New("socket taken")
	assert.ErrorIs(t, waitJobDaemon(t, runJobDaemon(context.Background(), func(context.Context) error { return boom })), boom)
	assert.NoError(t, waitJobDaemon(t, runJobDaemon(context.Background(), func(context.Context) error { return nil })))
}

func TestRunJobDaemonRunsWithTheContextGiven(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := runJobDaemon(ctx, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	cancel()
	assert.ErrorIs(t, waitJobDaemon(t, done), context.Canceled)
}
