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
Le dashboard affiche projets, sessions, métriques et télémétrie agent.

Par défaut le serveur écoute uniquement sur 127.0.0.1 (localhost).
Accédez au dashboard sur http://localhost:8080

Exemples:
  oh serve                   Lance sur le port par défaut (8080)
  oh serve --port 9090       Lance sur un port personnalisé
  oh serve --readonly=false  Active les opérations d'écriture via l'API`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 8080, "Port d'écoute")
	serveCmd.Flags().Bool("readonly", true, "Mode lecture seule (désactive les endpoints d'écriture)")
}

func runServe(cmd *cobra.Command, args []string) error {
	a := MustApp()
	port, _ := cmd.Flags().GetInt("port")
	readonly, _ := cmd.Flags().GetBool("readonly")
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()

	// ── API REST ──────────────────────────────────────────────────────────────

	// GET /api/v1/health
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "version": "1.0"})
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

	// ── Dashboard SPA ─────────────────────────────────────────────────────────
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, dashboardHTML(addr))
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	fmt.Fprintf(a.IO.Out, "%s Dashboard oh disponible sur http://%s\n",
		theme.SuccessStyle.Render(theme.IconSuccess), addr)
	if readonly {
		fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  Mode lecture seule actif (--readonly=false pour activer l'écriture)"))
	}
	fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  Ctrl+C pour arrêter"))
	fmt.Fprintln(a.IO.Out)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

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

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:*")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// dashboardHTML returns the minimal dashboard SPA (inline, no external deps).
func dashboardHTML(addr string) string {
	return `<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>oh — Dashboard</title>
<style>
  :root { --bg:#0f0f0f; --panel:#1a1a1a; --border:#2a2a2a; --text:#e0e0e0; --muted:#888; --accent:#7c3aed; --green:#22c55e; --yellow:#eab308; --red:#ef4444; }
  * { box-sizing:border-box; margin:0; padding:0; }
  body { background:var(--bg); color:var(--text); font-family:monospace; font-size:14px; }
  header { background:var(--panel); border-bottom:1px solid var(--border); padding:12px 24px; display:flex; align-items:center; gap:12px; }
  header h1 { font-size:16px; color:var(--accent); }
  header .status { font-size:12px; color:var(--muted); margin-left:auto; }
  main { padding:24px; display:grid; grid-template-columns:1fr 1fr; gap:16px; }
  .card { background:var(--panel); border:1px solid var(--border); border-radius:6px; padding:16px; }
  .card h2 { font-size:13px; color:var(--muted); margin-bottom:12px; text-transform:uppercase; letter-spacing:.05em; }
  table { width:100%; border-collapse:collapse; font-size:13px; }
  th { text-align:left; color:var(--muted); font-weight:normal; padding:4px 8px; border-bottom:1px solid var(--border); }
  td { padding:6px 8px; border-bottom:1px solid var(--border); }
  tr:last-child td { border-bottom:none; }
  .badge { display:inline-block; padding:2px 6px; border-radius:3px; font-size:11px; }
  .badge.success { background:#14532d; color:var(--green); }
  .badge.failed  { background:#450a0a; color:var(--red); }
  .badge.running { background:#422006; color:var(--yellow); }
  .empty { color:var(--muted); font-size:13px; padding:8px; }
  @media(max-width:768px) { main { grid-template-columns:1fr; } }
</style>
</head>
<body>
<header>
  <h1>◆ oh dashboard</h1>
  <span class="status" id="status">Chargement...</span>
</header>
<main>
  <div class="card" id="projects-card">
    <h2>Projets</h2>
    <div id="projects-content"><div class="empty">Chargement...</div></div>
  </div>
  <div class="card" id="sessions-card">
    <h2>Sessions récentes</h2>
    <div id="sessions-content"><div class="empty">Chargement...</div></div>
  </div>
  <div class="card" style="grid-column:1/-1" id="agents-card">
    <h2>Télémétrie agents</h2>
    <div id="agents-content"><div class="empty">Chargement...</div></div>
  </div>
</main>
<script>
const API = 'http://` + addr + `/api/v1';

async function fetchJSON(path) {
  const r = await fetch(API + path);
  if (!r.ok) throw new Error(r.statusText);
  return r.json();
}

function badge(status) {
  return '<span class="badge ' + status + '">' + status + '</span>';
}

async function loadProjects() {
  try {
    const data = await fetchJSON('/projects');
    if (!data || data.length === 0) {
      document.getElementById('projects-content').innerHTML = '<div class="empty">Aucun projet enregistré</div>';
      return;
    }
    let html = '<table><tr><th>Nom</th><th>Langage</th><th>Provider</th><th>Statut</th></tr>';
    for (const p of data) {
      html += '<tr><td>' + p.Name + '</td><td>' + (p.Language||'—') + '</td><td>' + (p.Provider||'—') + '</td><td>' + badge(p.Status||'active') + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('projects-content').innerHTML = html;
  } catch(e) { document.getElementById('projects-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>'; }
}

async function loadSessions() {
  try {
    const data = await fetchJSON('/sessions');
    if (!data || data.length === 0) {
      document.getElementById('sessions-content').innerHTML = '<div class="empty">Aucune session enregistrée</div>';
      return;
    }
    const recent = data.slice(0, 10);
    let html = '<table><tr><th>ID</th><th>Provider</th><th>Modèle</th><th>Statut</th></tr>';
    for (const s of recent) {
      html += '<tr><td>' + s.ID.substring(0,8) + '…</td><td>' + (s.Provider||'—') + '</td><td>' + (s.Model||'—') + '</td><td>' + badge(s.Status) + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('sessions-content').innerHTML = html;
  } catch(e) { document.getElementById('sessions-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>'; }
}

async function loadAgents() {
  try {
    const data = await fetchJSON('/metrics/agents');
    if (!data || data.length === 0) {
      document.getElementById('agents-content').innerHTML = '<div class="empty">Aucune donnée de télémétrie agent disponible</div>';
      return;
    }
    let html = '<table><tr><th>Agent</th><th>Runs</th><th>Succès</th><th>Durée moy.</th><th>Tokens</th><th>Coût</th></tr>';
    for (const m of data) {
      const dur = m.AvgDurationSec > 0 ? Math.round(m.AvgDurationSec) + 's' : '—';
      const tokens = m.TotalTokensIn + m.TotalTokensOut;
      const tok = tokens >= 1e6 ? (tokens/1e6).toFixed(1)+'M' : tokens >= 1000 ? (tokens/1000).toFixed(1)+'K' : tokens;
      html += '<tr><td>' + m.AgentName + '</td><td>' + m.TotalRuns + '</td><td>' + m.SuccessRate.toFixed(0) + '%</td><td>' + dur + '</td><td>' + tok + '</td><td>$' + m.TotalCostUSD.toFixed(2) + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('agents-content').innerHTML = html;
  } catch(e) { document.getElementById('agents-content').innerHTML = '<div class="empty">Télémétrie non disponible (requiert T29)</div>'; }
}

async function refresh() {
  document.getElementById('status').textContent = new Date().toLocaleTimeString();
  await Promise.all([loadProjects(), loadSessions(), loadAgents()]);
}

refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
}
