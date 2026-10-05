package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionresults"
)

// Session lifecycle (E11):
//   - closing an interactive client never stops a session;
//   - a server group sleeps (tool server stopped, sessions resumable) when no
//     client is attached, no agent loop runs and the group has been idle for
//     IdleSleep — or right away after its turn when oh quit with the
//     "finish the step then sleep" policy;
//   - a pending decision keeps the server awake while an oh client is
//     present (TUI open); otherwise the idle timer applies.

// ClientKind distinguishes heartbeating clients.
type ClientKind string

const (
	ClientAttach   ClientKind = "attach"   // interactive client attached to a session (group)
	ClientPresence ClientKind = "presence" // an oh UI is open (TUI)
)

// HeartbeatRequest is the body of POST /v1/clients/heartbeat.
type HeartbeatRequest struct {
	ClientID  string     `json:"client_id"`
	Kind      ClientKind `json:"kind"`
	Group     string     `json:"group,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	TTL       int        `json:"ttl_seconds,omitempty"` // default 90
}

// QuitPolicy is applied to a server group when oh quits.
type QuitPolicy string

const (
	PolicySleepWhenIdle QuitPolicy = "sleep_when_idle" // finish the current step, then sleep
	PolicyBackground    QuitPolicy = "background"      // keep running (normal idle timer)
	PolicyStopNow       QuitPolicy = "stop_now"        // stop the server now
)

// PolicyRequest is the body of POST /v1/groups/{group}/policy.
type PolicyRequest struct {
	Policy QuitPolicy `json:"policy"`
}

type client struct {
	kind    ClientKind
	group   string
	expires time.Time
}

func (d *Daemon) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientID == "" {
		writeErr(w, http.StatusBadRequest, "client_id is required")
		return
	}
	ttl := time.Duration(req.TTL) * time.Second
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	d.mu.Lock()
	d.clients[req.ClientID] = client{kind: req.Kind, group: req.Group, expires: time.Now().Add(ttl)}
	d.lastBusy = time.Now()
	if req.Kind == ClientAttach && req.Group != "" {
		delete(d.policies, req.Group) // a user is back on the session
	}
	d.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleClientGone(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	delete(d.clients, r.PathValue("id"))
	d.mu.Unlock()
	d.wake()
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handlePolicy(w http.ResponseWriter, r *http.Request) {
	var req PolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	switch req.Policy {
	case PolicySleepWhenIdle, PolicyBackground, PolicyStopNow:
	default:
		writeErr(w, http.StatusBadRequest, "unknown policy")
		return
	}
	d.mu.Lock()
	d.policies[r.PathValue("group")] = req.Policy
	d.mu.Unlock()
	d.wake()
	w.WriteHeader(http.StatusNoContent)
}

// clientState returns whether a client is attached to the group and whether
// any oh UI is present (expired heartbeats are dropped).
func (d *Daemon) clientState(group string) (attached, presence bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for id, c := range d.clients {
		if now.After(c.expires) {
			delete(d.clients, id)
			continue
		}
		switch c.kind {
		case ClientAttach:
			if c.group == group {
				attached = true
			}
		case ClientPresence:
			presence = true
		}
	}
	return attached, presence
}

// sleepDecision tells whether a ready server group must sleep or stop now.
func (d *Daemon) sleepDecision(srv domain.Server, w *watcher) (sleep, stop bool) {
	d.mu.Lock()
	policy := d.policies[srv.GroupKey]
	d.mu.Unlock()
	if policy == PolicyStopNow {
		return false, true
	}
	attached, presence := d.clientState(srv.GroupKey)
	if attached || w == nil || !w.isSynced() {
		return false, false // never sleep on a state that was not resynchronized
	}
	executing, pending, lastEvent := w.snapshot()
	if executing > 0 {
		return false, false
	}
	if policy == PolicySleepWhenIdle {
		return true, false
	}
	if pending > 0 && presence {
		return false, false
	}
	if srv.LastActivityAt.After(lastEvent) {
		lastEvent = srv.LastActivityAt
	}
	return time.Since(lastEvent) > d.opts.IdleSleep, false
}

// putToSleep stops the tool server of a group; its sessions stay resumable
// (state "sleeping", or "stopped" when stopping for good).
func (d *Daemon) putToSleep(ctx context.Context, srv domain.Server, final bool) {
	if ad := d.opts.Adapter; ad != nil {
		if a := ad(srv.Adapter); a != nil {
			h := adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}
			d.snapshotResults(ctx, a, h, srv)
			_ = a.StopServer(ctx, h)
		}
	}
	status, state := domain.ServerSleeping, domain.RunSleeping
	if final {
		status, state = domain.ServerStopped, domain.RunStopped
	}
	if d.opts.Servers != nil {
		_ = d.opts.Servers.SetStatus(ctx, srv.GroupKey, status)
	}
	d.revokeOwner(ctx, srv.GroupKey)
	d.mu.Lock()
	delete(d.policies, srv.GroupKey)
	d.mu.Unlock()
	d.wmu.Lock()
	w, ok := d.watchers[srv.GroupKey]
	delete(d.watchers, srv.GroupKey)
	d.wmu.Unlock()
	if ok {
		w.stop(3 * time.Second)
	}
	d.markSessions(ctx, srv, state)
	d.closeDecisions(ctx, srv.GroupKey, final)
	slog.Info("ohd: server group put to "+string(status), "group", srv.GroupKey)
}

func (d *Daemon) markSessions(ctx context.Context, srv domain.Server, state domain.RunState) {
	if d.opts.Sessions == nil {
		return
	}
	sessions, err := d.opts.Sessions.List(ctx, srv.ProjectID)
	if err != nil {
		return
	}
	now := time.Now()
	for i := range sessions {
		s := &sessions[i]
		if s.GroupKey != srv.GroupKey || isTerminal(s.State) || s.State == state {
			continue
		}
		s.State = state
		s.StateChangedAt = &now
		if state == domain.RunStopped {
			s.Status = domain.SessionStatusCompleted
			s.EndedAt = &now
		}
		if err := d.opts.Sessions.Update(ctx, s); err != nil {
			continue
		}
		d.feed.publishChange(domain.SessionChange{SessionID: s.ID, GroupKey: s.GroupKey, State: state})
		if state == domain.RunStopped {
			d.feed.forget(s.ID)
			if d.opts.OnSessionEnd != nil {
				d.opts.OnSessionEnd(ctx, *s)
			}
		}
	}
}

// applyLifecycle evaluates sleep/stop decisions for the ready servers.
// toolBusy asks the tool itself whether any session of the group is executing
// (including sessions oh does not track yet). Errors count as busy.
func (d *Daemon) toolBusy(ctx context.Context, srv domain.Server) bool {
	if d.opts.Adapter == nil {
		return false
	}
	ad := d.opts.Adapter(srv.Adapter)
	if ad == nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ids, err := ad.ActiveSessions(cctx, adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID})
	return err != nil || len(ids) > 0
}

func (d *Daemon) applyLifecycle(ctx context.Context, ready []domain.Server) {
	for _, srv := range ready {
		d.wmu.Lock()
		w := d.watchers[srv.GroupKey]
		d.wmu.Unlock()
		sleep, stop := d.sleepDecision(srv, w)
		if sleep && d.toolBusy(ctx, srv) {
			sleep = false
		}
		switch {
		case stop:
			d.putToSleep(ctx, srv, true)
		case sleep:
			d.putToSleep(ctx, srv, false)
		}
	}
}

// snapshotResults saves the results (diff, cost) of the open sessions of a
// group while its server still runs: they stay readable once it sleeps.
func (d *Daemon) snapshotResults(ctx context.Context, a adapters.ToolAdapter, h adapters.ServerHandle, srv domain.Server) {
	if d.opts.SessionsDir == "" || d.opts.Sessions == nil {
		return
	}
	sessions, err := d.opts.Sessions.List(ctx, srv.ProjectID)
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.GroupKey != srv.GroupKey || isTerminal(s.State) {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		res, err := a.Results(cctx, h, s.ID)
		cancel()
		if err != nil {
			continue
		}
		if err := sessionresults.Save(d.opts.SessionsDir, res, time.Now()); err != nil {
			slog.Debug("ohd: results not saved", "session", s.ID, "error", err)
		}
	}
}
