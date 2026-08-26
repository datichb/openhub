package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// ── SSE Hub ──────────────────────────────────────────────────────────────────

// sseHub manages connected SSE clients and broadcasts events.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
}

type sseEvent struct {
	Type string // "session_update", "new_event", "board_change"
	Data string // JSON payload
}

func newSSEHub() *sseHub {
	return &sseHub{
		clients: make(map[chan sseEvent]struct{}),
	}
}

func (h *sseHub) register() chan sseEvent {
	ch := make(chan sseEvent, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unregister(ch chan sseEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

func (h *sseHub) broadcast(eventType, data string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	evt := sseEvent{Type: eventType, Data: data}
	for ch := range h.clients {
		select {
		case ch <- evt:
		default:
			// Drop if client is slow — they'll reconnect
		}
	}
}

// startBroadcaster polls data sources every 5s and pushes SSE events on changes.
func (h *sseHub) startBroadcaster(ctx context.Context, a *app.App) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastSessionHash string
	var lastEventsHash string
	var lastBoardHash string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.pollSessions(ctx, a, &lastSessionHash)
			h.pollTeamEvents(ctx, a, &lastEventsHash)
			h.pollTeamBoard(ctx, a, &lastBoardHash)
		}
	}
}

func (h *sseHub) pollSessions(ctx context.Context, a *app.App, lastHash *string) {
	if a.Sessions == nil {
		return
	}
	sessions, err := a.Sessions.List(ctx, "")
	if err != nil {
		return
	}
	hash := quickHash(sessions)
	if hash != *lastHash {
		*lastHash = hash
		if data, err := json.Marshal(map[string]int{"count": len(sessions)}); err == nil {
			h.broadcast("session_update", string(data))
		}
	}
}

func (h *sseHub) pollTeamEvents(ctx context.Context, a *app.App, lastHash *string) {
	repo := getTeamRepoSafe(a)
	if repo == nil {
		return
	}
	events, err := repo.ListEventsLimited("", 10)
	if err != nil {
		return
	}
	hash := quickHash(events)
	if hash != *lastHash {
		*lastHash = hash
		if data, err := json.Marshal(map[string]int{"count": len(events)}); err == nil {
			h.broadcast("new_event", string(data))
		}
	}
}

func (h *sseHub) pollTeamBoard(_ context.Context, a *app.App, lastHash *string) {
	repo := getTeamRepoSafe(a)
	if repo == nil {
		return
	}
	claims, err := repo.ListClaims("")
	if err != nil {
		return
	}
	hash := quickHash(claims)
	if hash != *lastHash {
		*lastHash = hash
		if data, err := json.Marshal(map[string]int{"count": len(claims)}); err == nil {
			h.broadcast("board_change", string(data))
		}
	}
}

// serveSSE handles GET /sse — Server-Sent Events endpoint.
func (h *sseHub) serveSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := h.register()
	defer h.unregister(ch)

	// Send initial ping so client knows connection is live
	fmt.Fprintf(w, "event: ping\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, evt.Data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// getTeamRepoSafe returns a teamstate.Repo if team is configured and cloned, nil otherwise.
func getTeamRepoSafe(a *app.App) *teamstate.Repo {
	tc := a.Config.ActiveTeam()
	if tc.StateRepo == "" {
		return nil
	}
	statePath := tc.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil
	}
	return repo
}

// quickHash produces a fast hash of any JSON-serializable value for change detection.
func quickHash(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
