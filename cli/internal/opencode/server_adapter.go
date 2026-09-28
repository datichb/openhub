package opencode

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/platform"
)

// serverAdapter wraps parallel.OpenCodeServer to provide the low-level
// server operations needed by OpenCodeParallelRunner.
// This is an internal type — not exported. The coordinator no longer
// interacts with individual servers.
type serverAdapter struct {
	inner       *parallel.OpenCodeServer
	opencodeBin string
	ticketID    string
	logger      *parallel.TaskLogger
}

func newServerAdapter(port int, dir, id, opencodeBin, hubDir string, extraEnv []string) *serverAdapter {
	logger := parallel.NewTaskLogger(hubDir, id)
	srv := parallel.NewServer(port, dir, id)
	srv.StderrWriter = logger.Writer()
	srv.ExtraEnv = extraEnv
	return &serverAdapter{
		inner:       srv,
		opencodeBin: opencodeBin,
		ticketID:    id,
		logger:      logger,
	}
}

func (a *serverAdapter) start(ctx context.Context) error {
	return a.inner.Start(ctx, a.opencodeBin)
}

func (a *serverAdapter) waitReady(ctx context.Context, timeout time.Duration) error {
	return a.inner.WaitReady(ctx, timeout)
}

func (a *serverAdapter) isAlive() bool { return a.inner.IsAlive() }
func (a *serverAdapter) dispose() error {
	if a.logger != nil {
		_ = a.logger.Close()
	}
	return a.inner.Dispose()
}
func (a *serverAdapter) kill() {
	if a.logger != nil {
		_ = a.logger.Close()
	}
	a.inner.Kill()
}
func (a *serverAdapter) port() int      { return a.inner.Port }
func (a *serverAdapter) dir() string    { return a.inner.Dir }

func (a *serverAdapter) createSession(title string) (string, error) {
	return a.inner.CreateSession(title)
}

func (a *serverAdapter) sendPrompt(sessionID, prompt, agent string) error {
	return a.inner.SendPromptAsync(sessionID, prompt, agent)
}

func (a *serverAdapter) sendNotification(sessionID, message string) error {
	return a.inner.SendNotification(sessionID, message)
}

func (a *serverAdapter) getStatus() (map[string]string, error) {
	return a.inner.GetSessionStatus()
}

func (a *serverAdapter) getModifiedFiles() ([]platform.FileChange, error) {
	paths, err := a.inner.GetFileStatus()
	if err != nil {
		return nil, err
	}
	changes := make([]platform.FileChange, len(paths))
	for i, p := range paths {
		changes[i] = platform.FileChange{
			Path:      p,
			Operation: "modified",
		}
	}
	return changes, nil
}

func (a *serverAdapter) abortSession(sessionID string) error {
	return a.inner.AbortSession(sessionID)
}
