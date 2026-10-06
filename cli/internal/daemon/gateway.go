package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// startGateway restores the gateway grants (hashes only) and serves the
// Beads gateway on the proxy listeners, which containers already reach
// (HostAddress, Linux bridge listener). Grants of groups that are no longer
// running are dropped.
func (d *Daemon) startGateway(ctx context.Context) {
	st, err := gateway.OpenStore(d.opts.Paths.Gateway())
	if err != nil {
		slog.Warn("ohd: gateway grants not restored", "error", err)
	}
	if d.opts.Servers != nil && st.Len() > 0 {
		live := map[string]bool{}
		if servers, err := d.opts.Servers.List(ctx); err == nil {
			for _, s := range servers {
				if s.Status != domain.ServerStopped && s.Status != domain.ServerSleeping {
					live[s.GroupKey] = true
				}
			}
			st.Retain(func(g gateway.Grant) bool { return live[g.GroupKey] })
		}
	}
	d.gateway = st
	d.proxy.Mount(beadswire.Prefix, &gateway.Beads{
		Store:  st,
		View:   d.opts.GatewayView,
		Alive:  d.sessionAlive,
		Binary: d.opts.BeadsBinary,
	})
}

// sessionAlive: Beads is reachable while the session is open and awake.
func (d *Daemon) sessionAlive(ctx context.Context, id string) bool {
	if d.opts.Sessions == nil {
		return true
	}
	s, err := d.opts.Sessions.Get(ctx, id)
	if err != nil || s == nil {
		return false
	}
	switch s.State {
	case domain.RunStopped, domain.RunCompleted, domain.RunFailed, domain.RunSleeping:
		return false
	}
	return true
}

func (d *Daemon) handleGatewayGrant(w http.ResponseWriter, r *http.Request) {
	var req GatewayGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	tok, err := d.gateway.Issue(gateway.Grant{
		SessionID: req.SessionID, GroupKey: req.GroupKey, ProjectID: req.ProjectID,
		WorkflowID: req.WorkflowID, Location: req.Location, BeadsAllow: req.BeadsAllow, GatewayURL: req.GatewayURL,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, GatewayGrantResponse{Token: tok})
}

// envSessionID mirrors runsvc.EnvSessionID (runsvc depends on this package).
const envSessionID = "OH_SESSION_ID"

// childEnv is the environment of a subagent sub-session: opencode does not
// pass the session environment to the sub-sessions started by the task
// tool, so the daemon applies the one of the root session (static
// variables, OH_SESSION_ID) with a gateway token of its own.
func (d *Daemon) childEnv(root, child string) (map[string]string, error) {
	env := map[string]string{}
	if d.opts.SessionsDir != "" {
		data, err := os.ReadFile(filepath.Join(d.opts.SessionsDir, root, "env.json"))
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &env); err != nil {
				return nil, fmt.Errorf("reading the environment of %s: %w", root, err)
			}
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	env[envSessionID] = root
	if d.gateway != nil {
		if g, ok := d.gateway.Root(root); ok {
			g.Holder = child
			g.CreatedAt = time.Time{}
			tok, err := d.gateway.Issue(g)
			if err != nil {
				return nil, err
			}
			env[beadswire.EnvURL], env[beadswire.EnvToken] = g.GatewayURL, tok
		}
	}
	return env, nil
}

// applyChildEnv gives a newly linked sub-session the environment of its
// root session. Its first shell command may start before (event latency).
func (w *watcher) applyChildEnv(child, root string) {
	setter, ok := w.ad.(adapters.SessionEnvSetter)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	env, err := w.d.childEnv(root, child)
	if err == nil {
		err = setter.SetSessionEnv(ctx, w.handle(), child, env)
	}
	if err != nil {
		slog.Warn("ohd: environment of a subagent session not applied", "session", root, "child", child, "error", err)
	}
}
