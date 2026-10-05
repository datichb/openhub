package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
)

func (d *Daemon) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+apiPrefix+"/health", d.handleHealth)
	mux.HandleFunc("POST "+apiPrefix+"/grants", d.handleIssue)
	mux.HandleFunc("POST "+apiPrefix+"/grants/secret", d.handleSecret)
	mux.HandleFunc("GET "+apiPrefix+"/grants/pending", d.handlePending)
	mux.HandleFunc("DELETE "+apiPrefix+"/owners/{owner}/grants", d.handleRevokeOwner)
	mux.HandleFunc("GET "+apiPrefix+"/usage", d.handleUsage)
	mux.HandleFunc("POST "+apiPrefix+"/servers/{group}/touch", d.handleTouch)
	mux.HandleFunc("POST "+apiPrefix+"/shutdown", d.handleShutdown)
	return mux
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
	writeJSON(w, http.StatusOK, Health{
		Version: d.opts.Version, PID: os.Getpid(), ProxyURL: d.proxy.URL(),
		Servers: d.liveServers(r.Context()), Grants: grants, PendingGrants: pending,
	})
}

func (d *Daemon) handleIssue(w http.ResponseWriter, r *http.Request) {
	var req GrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Owner == "" || req.Provider == "" {
		writeErr(w, http.StatusBadRequest, "owner and provider are required")
		return
	}
	g := domain.ProxyGrant{
		Token: credproxy.NewToken(), Owner: req.Owner, Provider: req.Provider, Region: req.Region,
		Source: req.Source, AllowedModels: req.AllowedModels, MaxTokens: req.MaxTokens, CreatedAt: time.Now(),
	}
	if err := d.register(r.Context(), g, req.Secret); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if d.opts.Grants != nil {
		if err := d.opts.Grants.Insert(r.Context(), &g); err != nil {
			d.proxy.Revoke(g.Token)
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	d.mu.Lock()
	d.lastBusy = time.Now()
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, GrantResponse{Token: g.Token, BaseURL: d.proxy.BaseURL(req.Provider)})
}

func (d *Daemon) handleSecret(w http.ResponseWriter, r *http.Request) {
	var req SecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	d.mu.Lock()
	g, ok := d.pending[req.Token]
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
	delete(d.pending, req.Token)
	d.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handlePending(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	out := make([]PendingGrant, 0, len(d.pending))
	for _, g := range d.pending {
		out = append(out, PendingGrant{Token: g.Token, Owner: g.Owner, Source: g.Source})
	}
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleRevokeOwner(w http.ResponseWriter, r *http.Request) {
	d.revokeOwner(r.Context(), r.PathValue("owner"))
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleUsage(w http.ResponseWriter, r *http.Request) {
	u, ok := d.proxy.Usage(r.URL.Query().Get("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown token")
		return
	}
	writeJSON(w, http.StatusOK, UsageResponse{Requests: u.Requests, InputTokens: u.InputTokens, OutputTokens: u.OutputTokens})
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

func (d *Daemon) handleShutdown(w http.ResponseWriter, r *http.Request) {
	force := strings.EqualFold(r.URL.Query().Get("force"), "true")
	if !force && d.liveServers(r.Context()) > 0 {
		writeErr(w, http.StatusConflict, "sessions are running (use force)")
		return
	}
	w.WriteHeader(http.StatusAccepted)
	go d.requestStop()
}
