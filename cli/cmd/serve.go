package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Lance le dashboard web oh",
	Long: `Lance un serveur HTTP local exposant le dashboard oh.
Le dashboard affiche projets, sessions, métriques, télémétrie agent,
team board (kanban), timeline et membres.

Par défaut le serveur écoute uniquement sur 127.0.0.1 (localhost).
Accédez au dashboard sur http://localhost:8080

Exemples:
  oh serve                   Lance sur le port par défaut (8080)
  oh serve --port 9090       Lance sur un port personnalisé`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 8080, "Port d'écoute")
}

func runServe(cmd *cobra.Command, args []string) error {
	a := MustApp()
	port, _ := cmd.Flags().GetInt("port")
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// SSE hub for real-time push
	hub := newSSEHub()

	mux := http.NewServeMux()

	// ── Existing API endpoints ───────────────────────────────────────────────

	// GET /api/v1/health
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "version": "2.0"})
	})

	// GET /api/v1/projects
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		if a.Projects == nil {
			writeJSON(w, []interface{}{})
			return
		}
		projects, err := a.Projects.List(r.Context(), "")
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, projects)
	})

	// GET /api/v1/sessions
	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		if a.Sessions == nil {
			writeJSON(w, []interface{}{})
			return
		}
		projectID := r.URL.Query().Get("project_id")
		sessions, err := a.Sessions.List(r.Context(), projectID)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, sessions)
	})

	// GET /api/v1/metrics/agents
	mux.HandleFunc("/api/v1/metrics/agents", func(w http.ResponseWriter, r *http.Request) {
		if a.AgentEvents == nil {
			writeJSON(w, []interface{}{})
			return
		}
		projectID := r.URL.Query().Get("project_id")
		metrics, err := a.AgentEvents.Metrics(r.Context(), projectID)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, metrics)
	})

	// ── New API endpoints ────────────────────────────────────────────────────

	// GET /api/v1/opencode/stats?period=7d|30d|all
	mux.HandleFunc("/api/v1/opencode/stats", handleOpenCodeStats(a))

	// GET /api/v1/opencode/sessions?limit=20
	mux.HandleFunc("/api/v1/opencode/sessions", handleOpenCodeSessions(a))

	// GET /api/v1/team/board?project=X
	mux.HandleFunc("/api/v1/team/board", handleTeamBoard(a))

	// GET /api/v1/team/events?limit=50&project=X
	mux.HandleFunc("/api/v1/team/events", handleTeamEvents(a))

	// GET /api/v1/team/members
	mux.HandleFunc("/api/v1/team/members", handleTeamMembers(a))

	// GET /api/v1/chart/costs?period=30d — SVG sparkline
	mux.HandleFunc("/api/v1/chart/costs", handleCostChart(a))

	// ── SSE endpoint ─────────────────────────────────────────────────────────

	// GET /sse — Server-Sent Events stream
	mux.HandleFunc("/sse", hub.serveSSE)

	// ── Dashboard SPA ────────────────────────────────────────────────────────

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, dashboardHTML(addr))
	})

	// ── Server ───────────────────────────────────────────────────────────────

	server := &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout intentionally omitted (0 = no timeout).
		// Required for SSE long-lived connections. Safe because this server
		// binds exclusively to 127.0.0.1 (no external exposure).
	}

	fmt.Fprintf(a.IO.Out, "%s Dashboard oh disponible sur http://%s\n",
		theme.SuccessStyle.Render(theme.IconSuccess), addr)
	fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  6 panels: Projets, Sessions+Coûts, Agents, Team Board, Timeline, Members"))
	fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  SSE temps réel activé (push toutes les 5s)"))
	fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  Ctrl+C pour arrêter"))
	fmt.Fprintln(a.IO.Out)

	// Start SSE broadcaster
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	go hub.startBroadcaster(ctx, a)

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serveur HTTP: %w", err)
	}
	return nil
}

// ── JSON helpers ─────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
