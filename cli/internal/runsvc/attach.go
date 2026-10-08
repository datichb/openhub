package runsvc

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

// Attach opens an interactive client for a session in a new terminal tab or
// window. The command run there is `oh session attach <id> --exec`: no secret
// appears on the command line. When a client is already attached to the
// session, its window is brought back to the front instead
// (termlaunch.MethodFocused): one session = one window (A43).
func (s *Service) Attach(ctx context.Context, sessionID, dir string, pref termlaunch.Pref, style termlaunch.ITermStyle, title string) (termlaunch.Method, error) {
	if s.FocusAttached(ctx, sessionID) {
		return termlaunch.MethodFocused, nil
	}
	exe := s.Executable
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return "", err
		}
	}
	m, attempts, err := termlaunch.Launch(ctx, termlaunch.Options{
		Pref: pref, ITermStyle: style, Dir: dir, Title: title,
		Argv: attachArgv(exe, sessionID), Env: attachEnv(),
	})
	for _, a := range attempts {
		if a.Err != nil {
			slog.Info("runsvc: terminal method failed", "method", a.Method, "error", a.Err)
		}
	}
	return m, err
}

// attachedClientLister is the part of the daemon client that lists the
// clients attached to a session.
type attachedClientLister interface {
	AttachedClients(ctx context.Context, sessionID string) ([]daemon.AttachedClient, error)
}

// focusWindow is termlaunch.Focus (variable for the tests).
var focusWindow = termlaunch.Focus

// FocusAttached brings the window of a client already attached to the
// session to the front. It returns false when there is none (or none can be
// found: closed window, unknown terminal): the caller opens a client.
func (s *Service) FocusAttached(ctx context.Context, sessionID string) bool {
	if s.Daemon == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	dc, err := s.Daemon(ctx)
	if err != nil {
		return false
	}
	l, ok := dc.(attachedClientLister)
	if !ok {
		return false
	}
	clients, err := l.AttachedClients(ctx, sessionID)
	if err != nil {
		return false
	}
	for _, c := range clients {
		if c.Where == nil || c.Where.Empty() {
			continue
		}
		err := focusWindow(ctx, *c.Where)
		if err == nil {
			return true
		}
		if !errors.Is(err, termlaunch.ErrNotFound) {
			slog.Info("runsvc: focusing the session window failed", "session", sessionID, "error", err)
		}
	}
	return false
}

// attachArgv is the command run in the new terminal.
func attachArgv(exe, sessionID string) []string {
	return []string{exe, "session", "attach", sessionID, "--exec"}
}

// attachEnv is the environment of that command: OH_HOME (relocated hub) is
// forwarded because the new terminal does not inherit oh's environment.
func attachEnv() map[string]string {
	if home := os.Getenv("OH_HOME"); home != "" {
		return map[string]string{"OH_HOME": home}
	}
	return nil
}

// AttachCommand returns the tool client command and environment for a
// session (used by `oh session attach --exec`). The client is restricted to
// the session when the adapter can (adapters.SingleSessionClient, A43).
func (s *Service) AttachCommand(ctx context.Context, sessionID string) (argv, env []string, err error) {
	srv, err := s.serverForSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if sc, ok := s.Adapter.(adapters.SingleSessionClient); ok {
		argv, env = sc.SingleSessionAttachCommand(handle(srv), sessionID, os.Environ())
		return argv, env, nil
	}
	argv, env = s.Adapter.AttachCommand(handle(srv), sessionID)
	return argv, env, nil
}
