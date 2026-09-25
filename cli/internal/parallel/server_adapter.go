package parallel

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// ServerAdapter wraps OpenCodeServer to implement SessionServer.
// This allows the coordinator to be refactored toward the platform interface
// while keeping the concrete HTTP implementation in server.go unchanged.
type ServerAdapter struct {
	inner       *OpenCodeServer
	opencodeBin string
}

// Compile-time check.
var _ SessionServer = (*ServerAdapter)(nil)

// NewServerAdapter creates a SessionServer backed by an OpenCodeServer.
func NewServerAdapter(port int, dir, id, opencodeBin string) *ServerAdapter {
	return &ServerAdapter{
		inner:       NewServer(port, dir, id),
		opencodeBin: opencodeBin,
	}
}

func (a *ServerAdapter) Start(ctx context.Context) error {
	return a.inner.Start(ctx, a.opencodeBin)
}

func (a *ServerAdapter) WaitReady(ctx context.Context, timeout time.Duration) error {
	return a.inner.WaitReady(ctx, timeout)
}

func (a *ServerAdapter) IsAlive() bool         { return a.inner.IsAlive() }
func (a *ServerAdapter) Dispose() error        { return a.inner.Dispose() }
func (a *ServerAdapter) Kill()                 { a.inner.Kill() }
func (a *ServerAdapter) TicketID() string      { return a.inner.TicketID }
func (a *ServerAdapter) Port() int             { return a.inner.Port }
func (a *ServerAdapter) Dir() string           { return a.inner.Dir }

func (a *ServerAdapter) CreateSession(title string) (string, error) {
	return a.inner.CreateSession(title)
}

func (a *ServerAdapter) SendPrompt(sessionID, prompt, agent string) error {
	return a.inner.SendPromptAsync(sessionID, prompt, agent)
}

func (a *ServerAdapter) GetStatus() (map[string]string, error) {
	return a.inner.GetSessionStatus()
}

func (a *ServerAdapter) GetModifiedFiles() ([]platform.FileChange, error) {
	paths, err := a.inner.GetFileStatus()
	if err != nil {
		return nil, err
	}
	changes := make([]platform.FileChange, len(paths))
	for i, p := range paths {
		changes[i] = platform.FileChange{
			Path:      p,
			Operation: "modified", // OpenCode API doesn't distinguish created vs modified yet
		}
	}
	return changes, nil
}

func (a *ServerAdapter) AbortSession(sessionID string) error {
	return a.inner.AbortSession(sessionID)
}

// SendNotification delegates to the underlying SendNotification method.
// This is an extension beyond the SessionServer interface, used
// by the coordinator for inter-session messaging.
func (a *ServerAdapter) SendNotification(sessionID, message string) error {
	return a.inner.SendNotification(sessionID, message)
}
