package adapters

import (
	"context"
	"errors"
)

// SessionPorter is implemented by adapters that can move a session between
// servers (remote runs, S9): Export returns the transcript of a session in
// the tool's format, Import recreates it on another server at location
// (same session id). Children must be imported after their parent.
type SessionPorter interface {
	ExportSession(ctx context.Context, h ServerHandle, sessionID string) ([]byte, error)
	ImportSession(ctx context.Context, h ServerHandle, data []byte, location string) (string, error)
}

// ErrSessionExists is returned by ImportSession when the session is already
// on the server.
var ErrSessionExists = errors.New("the session already exists on this server")
