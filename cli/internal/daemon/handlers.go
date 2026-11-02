package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/safego"
)

func (d *Daemon) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+api.Prefix+api.RouteHealth, d.handleHealth)
	mux.HandleFunc("POST "+api.Prefix+api.RouteGrants, d.privileged(d.afterRestore(d.handleIssue)))
	mux.HandleFunc("POST "+api.Prefix+api.RouteGrantsSecret, d.privileged(d.afterRestore(d.handleSecret)))
	mux.HandleFunc("GET "+api.Prefix+api.RouteGrantsPending, d.privileged(d.afterRestore(d.handlePending)))
	mux.HandleFunc("DELETE "+api.Prefix+api.RouteOwnerGrants, d.privileged(d.afterRestore(d.handleRevokeOwner)))
	mux.HandleFunc("GET "+api.Prefix+api.RouteUsage, d.afterRestore(d.handleUsage))
	mux.HandleFunc("POST "+api.Prefix+api.RouteServerTouch, d.handleTouch)
	mux.HandleFunc("POST "+api.Prefix+api.RouteShutdown, d.privileged(d.handleShutdown))
	mux.HandleFunc("POST "+api.Prefix+api.RouteClientHeartbeat, d.handleHeartbeat)
	mux.HandleFunc("DELETE "+api.Prefix+api.RouteClient, d.handleClientGone)
	mux.HandleFunc("GET "+api.Prefix+api.RouteClients, d.privileged(d.handleClients))
	mux.HandleFunc("POST "+api.Prefix+api.RouteGroupPolicy, d.privileged(d.handlePolicy))
	mux.HandleFunc("POST "+api.Prefix+api.RouteProxyListeners, d.privileged(d.handleListen))
	mux.HandleFunc("POST "+api.Prefix+api.RouteGatewayGrants, d.privileged(d.handleGatewayGrant))
	mux.HandleFunc("POST "+hookTokenRoute, d.handleHookToken) // S-06: checked on the caller process tree
	mux.HandleFunc("GET "+api.Prefix+api.RouteStream, d.handleStream)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWorkflowStatus, d.handleWorkflowStatus)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWorkflowCheckpoint, d.handleWorkflowCheckpoint)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWorkflowOutputs, d.handleWorkflowOutputs)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWorkflowReport, d.handleWorkflowReport)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWorkflowReport, d.handleWorkflowReportGet)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWorkflowReports, d.handleWorkflowReports)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWikiIndex, d.handleWikiIndex)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWikiPage, d.handleWikiPage)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWikiPropose, d.handleWikiPropose)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWikiPending, d.handleWikiPending)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWikiRefused, d.handleWikiRefused)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWikiApply, d.handleWikiApply)
	mux.HandleFunc("GET "+api.Prefix+api.RouteWikiCheck, d.handleWikiCheck)
	mux.HandleFunc("POST "+api.Prefix+api.RouteWorkflowRules, d.privileged(d.handleWorkflowRules))
	mux.HandleFunc("POST "+api.Prefix+api.RouteSessionDecided, d.handleSessionDecided)
	return mux
}

// afterRestore makes a grant route wait until the persisted grants are
// restored (a token would look unknown, or a revocation be undone).
func (d *Daemon) afterRestore(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-d.restored:
			h(w, r)
		case <-r.Context().Done():
			writeErr(w, http.StatusServiceUnavailable, "grants are being restored")
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	pending := len(d.pending)
	d.mu.Unlock()
	grants := 0
	if d.opts.Grants != nil {
		if list, err := d.opts.Grants.ListActive(r.Context()); err == nil {
			grants = len(list)
		}
	}
	restoring := true
	select {
	case <-d.restored:
		restoring = false
	default:
	}
	writeJSON(w, http.StatusOK, api.Health{
		Version: d.opts.Version, PID: os.Getpid(), ProxyURL: d.proxy.URL(),
		Servers: d.liveServers(r.Context()), Grants: grants, PendingGrants: pending, Restoring: restoring,
		MemoryMB: d.memoryUsed(),
	})
}

func (d *Daemon) handleIssue(w http.ResponseWriter, r *http.Request) {
	var req api.GrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Owner == "" || req.Provider == "" {
		writeErr(w, http.StatusBadRequest, "owner and provider are required")
		return
	}
	token := credproxy.NewToken()
	g := domain.ProxyGrant{
		TokenHash: credproxy.TokenHash(token), Owner: req.Owner, Provider: req.Provider, Region: req.Region,
		Source: req.Source, AllowedModels: req.AllowedModels, MaxTokens: req.MaxTokens, CreatedAt: time.Now(),
	}
	if err := d.register(r.Context(), g, req.Secret); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if d.opts.Grants != nil {
		if err := d.opts.Grants.Insert(r.Context(), &g); err != nil {
			d.proxy.Revoke(g.TokenHash)
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	d.mu.Lock()
	d.lastBusy = time.Now()
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, api.GrantResponse{Token: token, BaseURL: d.proxy.BaseURL(req.Provider)})
}

func (d *Daemon) handleSecret(w http.ResponseWriter, r *http.Request) {
	var req api.SecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	d.mu.Lock()
	key := credproxy.RefHash(req.Token)
	g, ok := d.pending[key]
	d.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "no pending grant for this token")
		return
	}
	if err := d.register(r.Context(), g, req.Secret); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	d.mu.Lock()
	delete(d.pending, key)
	d.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handlePending(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	out := make([]api.PendingGrant, 0, len(d.pending))
	for _, g := range d.pending {
		out = append(out, api.PendingGrant{Token: g.TokenHash, Owner: g.Owner, Source: g.Source})
	}
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleRevokeOwner(w http.ResponseWriter, r *http.Request) {
	// The tokens are dead in memory in any case; the client is told when the
	// database could not be written, so that it tries again (S-29).
	if !d.revokeOwner(r.Context(), r.PathValue("owner")) {
		writeErr(w, http.StatusInternalServerError, "revocation not saved in the database")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleUsage(w http.ResponseWriter, r *http.Request) {
	u, ok := d.proxy.Usage(r.URL.Query().Get("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown token")
		return
	}
	writeJSON(w, http.StatusOK, api.UsageResponse{Requests: u.Requests, InputTokens: u.InputTokens, OutputTokens: u.OutputTokens,
		AuthRejected: d.authRejected(r.URL.Query().Get("token"))})
}

func (d *Daemon) handleTouch(w http.ResponseWriter, r *http.Request) {
	if d.opts.Servers != nil {
		_ = d.opts.Servers.Touch(r.Context(), r.PathValue("group"), time.Now())
	}
	d.mu.Lock()
	d.lastBusy = time.Now()
	d.mu.Unlock()
	d.wake()
	w.WriteHeader(http.StatusNoContent)
}

// handleSessionDecided: a decision raised by oh for a session (budget,
// error) was answered by a CLI or the TUI.
func (d *Daemon) handleSessionDecided(w http.ResponseWriter, r *http.Request) {
	if !d.sessionDecided(r.Context(), r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "session not watched")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleShutdown(w http.ResponseWriter, r *http.Request) {
	force := strings.EqualFold(r.URL.Query().Get("force"), "true")
	// The live tool servers are counted (their sessions go through the proxy
	// of the daemon), not the sessions: a server may hold none working.
	if n := d.liveServers(r.Context()); !force && n > 0 {
		writeErr(w, http.StatusConflict, fmt.Sprintf("tool servers alive: %d (use force)", n))
		return
	}
	w.WriteHeader(http.StatusAccepted)
	safego.Go(d.requestStop)
}

// handleListen adds a proxy listener on a private or loopback address (a
// container bridge gateway); public and unspecified addresses are refused so
// that the proxy is never exposed on the network.
func (d *Daemon) handleListen(w http.ResponseWriter, r *http.Request) {
	var req api.ListenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	ip, err := listenHost(req.Host)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := d.proxy.Listen(ip)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := d.saveState(); err != nil {
		slog.Warn("ohd: cannot save state", "error", err)
	}
	writeJSON(w, http.StatusOK, api.ListenResponse{URL: u})
}
