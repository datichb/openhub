package cmd

import "fmt"

// dashboardHTML returns the full dashboard SPA (inline, no external deps).
func dashboardHTML(addr string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>oh — Dashboard</title>
<style>
:root {
  --bg:#0f0f0f; --panel:#1a1a1a; --border:#2a2a2a; --text:#e0e0e0;
  --muted:#888; --accent:#6366f1; --green:#22c55e; --yellow:#eab308;
  --red:#ef4444; --blue:#3b82f6;
}
* { box-sizing:border-box; margin:0; padding:0; }
body { background:var(--bg); color:var(--text); font-family:'JetBrains Mono',monospace; font-size:13px; }

/* ── Header ─────────────────────────────────────── */
header {
  background:var(--panel); border-bottom:1px solid var(--border);
  padding:10px 20px; display:flex; align-items:center; gap:16px; flex-wrap:wrap;
}
header h1 { font-size:15px; color:var(--accent); white-space:nowrap; }
.filters { display:flex; gap:10px; align-items:center; margin-left:auto; }
.filters select, .filters .period-btn {
  background:var(--bg); color:var(--text); border:1px solid var(--border);
  border-radius:4px; padding:4px 8px; font-family:inherit; font-size:12px; cursor:pointer;
}
.filters .period-btn.active { background:var(--accent); color:#fff; border-color:var(--accent); }
.sse-dot { width:8px; height:8px; border-radius:50%%; background:var(--muted); margin-left:8px; }
.sse-dot.connected { background:var(--green); }
.sse-dot.error { background:var(--red); }

/* ── Grid ───────────────────────────────────────── */
main {
  padding:16px; display:grid;
  grid-template-columns:repeat(3,1fr);
  grid-template-rows:auto auto auto;
  gap:12px;
}
.card {
  background:var(--panel); border:1px solid var(--border);
  border-radius:6px; padding:14px; overflow:hidden;
}
.card h2 {
  font-size:11px; color:var(--muted); margin-bottom:10px;
  text-transform:uppercase; letter-spacing:.06em; font-weight:500;
}
.card.span2 { grid-column:span 2; }
.card.span3 { grid-column:span 3; }

/* ── Tables ─────────────────────────────────────── */
table { width:100%%; border-collapse:collapse; font-size:12px; }
th { text-align:left; color:var(--muted); font-weight:400; padding:4px 6px; border-bottom:1px solid var(--border); }
td { padding:5px 6px; border-bottom:1px solid var(--border); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:180px; }
tr:last-child td { border-bottom:none; }
tr:hover { background:rgba(99,102,241,0.04); }

/* ── Badges ─────────────────────────────────────── */
.badge { display:inline-block; padding:2px 6px; border-radius:3px; font-size:10px; font-weight:500; }
.badge.active,.badge.running,.badge.IN_PROGRESS { background:#422006; color:var(--yellow); }
.badge.completed,.badge.success,.badge.DONE { background:#14532d; color:var(--green); }
.badge.failed,.badge.BLOCKED { background:#450a0a; color:var(--red); }
.badge.TODO { background:#1e293b; color:var(--blue); }
.badge.REVIEW { background:#312e81; color:var(--accent); }
.badge.archived { background:#1f1f1f; color:var(--muted); }

/* ── Kanban ─────────────────────────────────────── */
.kanban { display:flex; gap:8px; overflow-x:auto; min-height:120px; }
.kanban-col { flex:1; min-width:140px; }
.kanban-col-header {
  font-size:10px; color:var(--muted); text-transform:uppercase;
  letter-spacing:.05em; padding:4px 6px; border-bottom:1px solid var(--border);
  margin-bottom:6px; display:flex; justify-content:space-between;
}
.kanban-col-header .count { color:var(--accent); }
.kanban-card {
  background:var(--bg); border:1px solid var(--border); border-radius:4px;
  padding:8px; margin-bottom:6px; font-size:11px;
}
.kanban-card .ticket { color:var(--accent); font-weight:500; }
.kanban-card .actor { color:var(--muted); font-size:10px; margin-top:3px; }
.kanban-card .labels { margin-top:4px; }
.kanban-card .label-tag {
  display:inline-block; background:var(--border); color:var(--text);
  padding:1px 4px; border-radius:2px; font-size:9px; margin-right:3px;
}

/* ── Timeline ───────────────────────────────────── */
.timeline { max-height:320px; overflow-y:auto; }
.timeline-item {
  padding:6px 8px; border-left:2px solid var(--border); margin-left:4px; margin-bottom:2px;
}
.timeline-item:hover { border-left-color:var(--accent); }
.timeline-item .ts { color:var(--muted); font-size:10px; }
.timeline-item .actor { color:var(--accent); }
.timeline-item .event-type { color:var(--text); font-weight:500; }

/* ── Members ────────────────────────────────────── */
.member-grid { display:flex; flex-wrap:wrap; gap:8px; }
.member-card {
  background:var(--bg); border:1px solid var(--border); border-radius:4px;
  padding:8px 12px; font-size:11px; min-width:160px;
}
.member-card .name { color:var(--text); font-weight:500; }
.member-card .role { color:var(--muted); font-size:10px; }
.member-card .mode { color:var(--accent); font-size:10px; }

/* ── Sparkline ──────────────────────────────────── */
.sparkline-container { margin-top:8px; }
.sparkline-container img { width:100%%; max-width:300px; height:auto; }

/* ── Misc ───────────────────────────────────────── */
.empty { color:var(--muted); font-size:12px; padding:8px; }
.stat-row { display:flex; gap:16px; margin-bottom:10px; flex-wrap:wrap; }
.stat-item { text-align:center; }
.stat-item .value { font-size:16px; font-weight:600; color:var(--accent); }
.stat-item .label { font-size:10px; color:var(--muted); }

/* ── Responsive ─────────────────────────────────── */
@media(max-width:1200px) {
  main { grid-template-columns:repeat(2,1fr); }
  .card.span2 { grid-column:span 2; }
  .card.span3 { grid-column:span 2; }
}
@media(max-width:768px) {
  main { grid-template-columns:1fr; }
  .card.span2,.card.span3 { grid-column:span 1; }
  .kanban { flex-direction:column; }
}
</style>
</head>
<body>
<header>
  <h1>◆ oh dashboard</h1>
  <div class="filters">
    <select id="project-filter"><option value="">Tous les projets</option></select>
    <span>
      <button class="period-btn active" data-period="7d">7d</button>
      <button class="period-btn" data-period="30d">30d</button>
      <button class="period-btn" data-period="all">all</button>
    </span>
    <span class="sse-dot" id="sse-indicator" title="SSE disconnected"></span>
  </div>
</header>
<main>
  <!-- Row 1 -->
  <div class="card" id="projects-card">
    <h2>Projets</h2>
    <div id="projects-content"><div class="empty">Chargement...</div></div>
  </div>
  <div class="card" id="sessions-card">
    <h2>Sessions + Coûts</h2>
    <div id="stats-summary"></div>
    <div id="sessions-content"><div class="empty">Chargement...</div></div>
    <div class="sparkline-container"><img id="sparkline" src="/api/v1/chart/costs?period=7d" alt="Costs sparkline"/></div>
  </div>
  <div class="card" id="agents-card">
    <h2>Agent Telemetry</h2>
    <div id="agents-content"><div class="empty">Chargement...</div></div>
  </div>
  <!-- Row 2 -->
  <div class="card span2" id="board-card">
    <h2>Team Board</h2>
    <div id="board-content"><div class="empty">Chargement...</div></div>
  </div>
  <div class="card" id="timeline-card">
    <h2>Activity Timeline</h2>
    <div id="timeline-content" class="timeline"><div class="empty">Chargement...</div></div>
  </div>
  <!-- Row 3 -->
  <div class="card span3" id="members-card">
    <h2>Members</h2>
    <div id="members-content"><div class="empty">Chargement...</div></div>
  </div>
</main>

<script>
const BASE = 'http://%s';
const API = BASE + '/api/v1';
let selectedProject = '';
let selectedPeriod = '7d';
let sseConnected = false;

// ── Helpers ────────────────────────────────────────────────────────────
async function fetchJSON(path) {
  const r = await fetch(API + path);
  if (!r.ok) throw new Error(r.statusText);
  return r.json();
}

function badge(status) {
  const s = (status||'').replace(/ /g,'_').toUpperCase();
  return '<span class="badge ' + s + '">' + (status||'—') + '</span>';
}

function truncate(s, len) {
  if (!s) return '—';
  return s.length > len ? s.substring(0, len) + '…' : s;
}

function formatTokens(n) {
  if (n >= 1e6) return (n/1e6).toFixed(1)+'M';
  if (n >= 1000) return (n/1000).toFixed(1)+'K';
  return n;
}

function relativeTime(dateStr) {
  const d = new Date(dateStr);
  const now = new Date();
  const diff = Math.floor((now - d) / 1000);
  if (diff < 60) return diff + 's ago';
  if (diff < 3600) return Math.floor(diff/60) + 'm ago';
  if (diff < 86400) return Math.floor(diff/3600) + 'h ago';
  return Math.floor(diff/86400) + 'd ago';
}

// ── Load Projects ──────────────────────────────────────────────────────
async function loadProjects() {
  try {
    const data = await fetchJSON('/projects');
    if (!data || data.length === 0) {
      document.getElementById('projects-content').innerHTML = '<div class="empty">Aucun projet</div>';
      return;
    }
    // Populate filter dropdown
    const sel = document.getElementById('project-filter');
    const current = sel.value;
    sel.innerHTML = '<option value="">Tous les projets</option>';
    for (const p of data) {
      sel.innerHTML += '<option value="' + (p.ID||p.Name) + '">' + p.Name + '</option>';
    }
    sel.value = current || '';

    let html = '<table><tr><th>Nom</th><th>Langage</th><th>Provider</th><th>Statut</th></tr>';
    for (const p of data) {
      html += '<tr><td>' + p.Name + '</td><td>' + (p.Language||'—') + '</td><td>' + (p.Provider||'—') + '</td><td>' + badge(p.Status||'active') + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('projects-content').innerHTML = html;
  } catch(e) {
    document.getElementById('projects-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── Load Stats + Sessions ──────────────────────────────────────────────
async function loadSessionsAndStats() {
  try {
    // Load aggregate stats
    const statsData = await fetchJSON('/opencode/stats?period=' + selectedPeriod);
    if (statsData.available && statsData.stats) {
      const s = statsData.stats;
      document.getElementById('stats-summary').innerHTML =
        '<div class="stat-row">' +
        '<div class="stat-item"><div class="value">' + s.TotalSessions + '</div><div class="label">Sessions</div></div>' +
        '<div class="stat-item"><div class="value">$' + s.TotalCost.toFixed(2) + '</div><div class="label">Coût</div></div>' +
        '<div class="stat-item"><div class="value">' + formatTokens(s.TotalTokensIn + s.TotalTokensOut) + '</div><div class="label">Tokens</div></div>' +
        '<div class="stat-item"><div class="value">' + s.ActiveProjects + '</div><div class="label">Projets actifs</div></div>' +
        '</div>';
    } else {
      document.getElementById('stats-summary').innerHTML = '';
    }

    // Load recent sessions (from opencode DB)
    const sessData = await fetchJSON('/opencode/sessions?limit=10');
    if (!sessData.available) {
      document.getElementById('sessions-content').innerHTML = '<div class="empty">' + (sessData.message||'Non disponible') + '</div>';
      return;
    }
    const sessions = sessData.sessions || [];
    if (sessions.length === 0) {
      document.getElementById('sessions-content').innerHTML = '<div class="empty">Aucune session</div>';
      return;
    }
    let html = '<table><tr><th>Titre</th><th>Modèle</th><th>Coût</th><th>Tokens</th><th>Date</th></tr>';
    for (const s of sessions) {
      html += '<tr><td>' + truncate(s.title||s.id, 24) + '</td><td>' + truncate(s.model,16) + '</td><td>$' + s.cost.toFixed(3) + '</td><td>' + formatTokens(s.tokens_in+s.tokens_out) + '</td><td>' + s.created_at + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('sessions-content').innerHTML = html;

    // Update sparkline
    document.getElementById('sparkline').src = '/api/v1/chart/costs?period=' + selectedPeriod + '&t=' + Date.now();
  } catch(e) {
    document.getElementById('sessions-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── Load Agents ────────────────────────────────────────────────────────
async function loadAgents() {
  try {
    const qp = selectedProject ? '?project_id=' + selectedProject : '';
    const data = await fetchJSON('/metrics/agents' + qp);
    if (!data || data.length === 0) {
      document.getElementById('agents-content').innerHTML = '<div class="empty">Aucune télémétrie agent</div>';
      return;
    }
    let html = '<table><tr><th>Agent</th><th>Runs</th><th>Succès</th><th>Durée</th><th>Tokens</th><th>Coût</th></tr>';
    for (const m of data) {
      const dur = m.AvgDurationSec > 0 ? Math.round(m.AvgDurationSec) + 's' : '—';
      const tok = formatTokens(m.TotalTokensIn + m.TotalTokensOut);
      html += '<tr><td>' + m.AgentName + '</td><td>' + m.TotalRuns + '</td><td>' + m.SuccessRate.toFixed(0) + '%%</td><td>' + dur + '</td><td>' + tok + '</td><td>$' + m.TotalCostUSD.toFixed(2) + '</td></tr>';
    }
    html += '</table>';
    document.getElementById('agents-content').innerHTML = html;
  } catch(e) {
    document.getElementById('agents-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── Load Board ─────────────────────────────────────────────────────────
async function loadBoard() {
  try {
    const qp = selectedProject ? '?project=' + selectedProject : '';
    const data = await fetchJSON('/team/board' + qp);
    if (!data.available) {
      document.getElementById('board-content').innerHTML = '<div class="empty">' + (data.message||'Non disponible') + '</div>';
      return;
    }
    const columns = data.columns || [];
    let html = '<div class="kanban">';
    for (const col of columns) {
      const claims = col.claims || [];
      html += '<div class="kanban-col">';
      html += '<div class="kanban-col-header"><span>' + col.name.replace('_',' ') + '</span><span class="count">' + claims.length + '</span></div>';
      for (const c of claims) {
        html += '<div class="kanban-card">';
        html += '<div class="ticket">' + (c.TicketID||c.ticket_id||'?') + '</div>';
        html += '<div class="actor">' + (c.ClaimedBy||c.claimed_by||'') + '</div>';
        const labels = c.Labels || c.labels || [];
        if (labels.length > 0) {
          html += '<div class="labels">';
          for (const l of labels) html += '<span class="label-tag">' + l + '</span>';
          html += '</div>';
        }
        html += '</div>';
      }
      if (claims.length === 0) html += '<div class="empty" style="font-size:10px">—</div>';
      html += '</div>';
    }
    html += '</div>';
    document.getElementById('board-content').innerHTML = html;
  } catch(e) {
    document.getElementById('board-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── Load Timeline ──────────────────────────────────────────────────────
async function loadTimeline() {
  try {
    const qp = selectedProject ? '&project=' + selectedProject : '';
    const data = await fetchJSON('/team/events?limit=50' + qp);
    if (!data.available) {
      document.getElementById('timeline-content').innerHTML = '<div class="empty">' + (data.message||'Non disponible') + '</div>';
      return;
    }
    const events = data.events || [];
    if (events.length === 0) {
      document.getElementById('timeline-content').innerHTML = '<div class="empty">Aucun événement</div>';
      return;
    }
    let html = '';
    for (const e of events) {
      const ts = e.ts ? relativeTime(e.ts) : '';
      html += '<div class="timeline-item">';
      html += '<span class="ts">' + ts + '</span> ';
      html += '<span class="actor">' + (e.actor||'') + '</span> ';
      html += '<span class="event-type">' + (e.event||e.type||'') + '</span>';
      if (e.ticket) html += ' <span style="color:var(--muted)">' + e.ticket + '</span>';
      html += '</div>';
    }
    document.getElementById('timeline-content').innerHTML = html;
  } catch(e) {
    document.getElementById('timeline-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── Load Members ───────────────────────────────────────────────────────
async function loadMembers() {
  try {
    const data = await fetchJSON('/team/members');
    if (!data.available) {
      document.getElementById('members-content').innerHTML = '<div class="empty">' + (data.message||'Non disponible') + '</div>';
      return;
    }
    const members = data.members || [];
    if (members.length === 0) {
      document.getElementById('members-content').innerHTML = '<div class="empty">Aucun membre</div>';
      return;
    }
    let html = '<div class="member-grid">';
    for (const m of members) {
      html += '<div class="member-card">';
      html += '<div class="name">' + (m.DisplayName||m.display_name||m.ID||m.id||'?') + '</div>';
      html += '<div class="role">' + (m.Role||m.role||'dev') + '</div>';
      html += '<div class="mode">' + (m.DefaultMode||m.default_mode||'manual') + '</div>';
      html += '</div>';
    }
    html += '</div>';
    document.getElementById('members-content').innerHTML = html;
  } catch(e) {
    document.getElementById('members-content').innerHTML = '<div class="empty">Erreur: ' + e.message + '</div>';
  }
}

// ── SSE ────────────────────────────────────────────────────────────────
function connectSSE() {
  const dot = document.getElementById('sse-indicator');
  try {
    const es = new EventSource(BASE + '/sse');
    es.addEventListener('ping', function() {
      dot.className = 'sse-dot connected';
      dot.title = 'SSE connecté';
      sseConnected = true;
    });
    es.addEventListener('session_update', function() { loadSessionsAndStats(); });
    es.addEventListener('new_event', function() { loadTimeline(); });
    es.addEventListener('board_change', function() { loadBoard(); });
    es.onopen = function() {
      dot.className = 'sse-dot connected';
      dot.title = 'SSE connecté';
      sseConnected = true;
    };
    es.onerror = function() {
      dot.className = 'sse-dot error';
      dot.title = 'SSE déconnecté — reconnection auto';
      sseConnected = false;
    };
  } catch(e) {
    dot.className = 'sse-dot error';
    sseConnected = false;
  }
}

// ── Filters ────────────────────────────────────────────────────────────
document.getElementById('project-filter').addEventListener('change', function(e) {
  selectedProject = e.target.value;
  refreshAll();
});

document.querySelectorAll('.period-btn').forEach(function(btn) {
  btn.addEventListener('click', function() {
    document.querySelectorAll('.period-btn').forEach(function(b) { b.classList.remove('active'); });
    btn.classList.add('active');
    selectedPeriod = btn.dataset.period;
    loadSessionsAndStats();
  });
});

// ── Refresh ────────────────────────────────────────────────────────────
async function refreshAll() {
  await Promise.all([loadProjects(), loadSessionsAndStats(), loadAgents(), loadBoard(), loadTimeline(), loadMembers()]);
}

// ── Init ───────────────────────────────────────────────────────────────
connectSSE();
refreshAll();
// Fallback polling every 30s
setInterval(function() { if (!sseConnected) refreshAll(); }, 30000);
</script>
</body>
</html>`, addr)
}
