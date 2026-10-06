package cmd

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon"
)

// In-process daemon (Windows option A, daemon.InProcessMode): the credential
// proxy and the session supervision run inside this oh process, as long as
// it lives. Its sessions are put to sleep when it exits (resumable).

var inProcess struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan error
}

// startInProcessDaemon starts the daemon inside this process (once).
func startInProcessDaemon() error {
	inProcess.mu.Lock()
	defer inProcess.mu.Unlock()
	if inProcess.cancel != nil {
		return nil
	}
	a := MustApp()
	ctx, cancel := context.WithCancel(context.Background())
	capability, _, err := daemonCapability(ctx)
	if err != nil {
		cancel()
		return err
	}
	opts := daemonOptions(ctx, a, capability)
	opts.StopServersOnExit = true
	opts.IdleAfter = 100 * 365 * 24 * time.Hour // lives with the oh process
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, opts) }()
	inProcess.cancel, inProcess.done = cancel, done
	return nil
}

// stopInProcessDaemon stops the daemon of this process, if any: the server
// groups it supervises are put to sleep.
func stopInProcessDaemon() {
	inProcess.mu.Lock()
	cancel, done := inProcess.cancel, inProcess.done
	inProcess.cancel = nil
	inProcess.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, daemon.ErrAlreadyRunning) {
			slog.Warn("in-process oh daemon stopped with an error", "error", err)
		}
	case <-time.After(45 * time.Second):
		slog.Warn("in-process oh daemon did not stop in time")
	}
}

// inProcessDaemonRunning reports whether this process hosts the daemon.
func inProcessDaemonRunning() bool {
	inProcess.mu.Lock()
	defer inProcess.mu.Unlock()
	return inProcess.cancel != nil
}
