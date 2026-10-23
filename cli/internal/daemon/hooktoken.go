package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/procinfo"
)

// Hook tokens of the Beads gateway (audit v5, S-06, DC24): the fake bd of
// a local session run by a Beads git hook asks the daemon, on its socket,
// for a one-time token that opens the hook mode of the gateway for that
// command. The daemon gives it only when the calling process (peer of the
// socket connection) is run by git from the hook of the session location.

// hookTokenRoute is beadswire.HookTokenPath.
const hookTokenRoute = apiPrefix + api.RouteHookToken

// procRead reads a process (replaced by tests).
var procRead = procinfo.Get

type connKey struct{}

// withConn keeps the connection of a request (ConnContext of the API
// server): the peer process of the socket is read from it.
func withConn(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, connKey{}, c)
}

func (d *Daemon) handleHookToken(w http.ResponseWriter, r *http.Request) {
	var req beadswire.HookTokenRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if d.gateway == nil {
		writeErr(w, http.StatusServiceUnavailable, "no gateway")
		return
	}
	g, ok := d.gateway.Lookup(req.Token)
	if !ok || !d.sessionAlive(r.Context(), g.SessionID) {
		writeErr(w, http.StatusUnauthorized, "unknown or revoked gateway token")
		return
	}
	hook := gateway.GitHookName(req.Argv)
	if hook == "" {
		writeErr(w, http.StatusForbidden, "not a Beads git hook run as git runs it")
		return
	}
	if err := d.checkHookCaller(r, g, hook); err != nil {
		slog.Warn("ohd: hook token refused", "session", g.SessionID, "hook", hook, "error", err)
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, beadswire.HookTokenResponse{HookToken: d.gateway.IssueHookToken(g, hook)})
}

// checkHookCaller: the peer of the request runs under git from the hook.
// Where the system tells neither the peer nor its parents, the shape of the
// command is the only check (DC24, fallback).
func (d *Daemon) checkHookCaller(r *http.Request, g gateway.Grant, hook string) error {
	c, _ := r.Context().Value(connKey{}).(net.Conn)
	pid, err := peerPID(c)
	if errors.Is(err, errUnsupportedPeer) {
		slog.Debug("ohd: hook token without process check (unsupported here)", "session", g.SessionID)
		return nil
	}
	if err != nil {
		return err
	}
	err = gateway.CheckHookCaller(r.Context(), pid, g.Location, hook, procRead)
	if errors.Is(err, procinfo.ErrUnsupported) {
		return nil
	}
	return err
}
