> [Lire en français](dashboard.fr.md)

# Web Dashboard - Guide

## Overview

`oh serve` launches a lightweight read-only web UI for monitoring your projects and agent sessions in real time. It provides a bird's-eye view across all registered projects, their recent sessions, and agent performance telemetry — without requiring a browser extension or external service.

---

## Launching the Dashboard

### Default launch

```bash
oh serve
```

Starts the dashboard on `http://127.0.0.1:8080`. Open in any browser.

### Custom port

```bash
oh serve --port 9090
```

### Stop the server

Press `Ctrl+C` in the terminal where `oh serve` is running.

---

## Security

The dashboard **always binds to `127.0.0.1`** (localhost only). It is never exposed to the network, even when `--port` is set. There is no authentication because the server is not reachable from outside the machine.

> **Warning:** Do not proxy the dashboard behind a public-facing reverse proxy without adding authentication (e.g. HTTP Basic Auth via nginx).

---

## Dashboard Features

### Projects table

Lists all projects registered with `oh`. Columns:

| Column | Description |
|--------|-------------|
| Name | Project name |
| Path | Absolute path on disk |
| Last session | Timestamp of the most recent session |
| Total sessions | All-time session count |
| Status | Active / idle |

### Recent sessions table

Shows the last 50 sessions across all projects. Columns:

| Column | Description |
|--------|-------------|
| Session ID | UUID |
| Project | Parent project |
| Agent | Agent that ran the session |
| Started | Start timestamp |
| Duration | Wall clock time |
| Status | completed / failed / running |

### Agent telemetry table

Per-agent performance metrics aggregated across all sessions:

| Column | Description |
|--------|-------------|
| Agent | Agent name |
| Sessions | Total session count |
| Avg duration | Mean session duration |
| Success rate | % of sessions with status = completed |
| Last seen | Most recent session timestamp |

---

## API Endpoints

The dashboard exposes a REST API for programmatic access:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/health` | Server health check |
| `GET` | `/api/v1/projects` | List all registered projects |
| `GET` | `/api/v1/sessions` | List recent sessions (query: `?project=<name>&limit=<n>`) |
| `GET` | `/api/v1/metrics/agents` | Agent telemetry aggregates |

### API examples

```bash
# Health check
curl http://127.0.0.1:8080/api/v1/health

# List all projects
curl http://127.0.0.1:8080/api/v1/projects

# List last 10 sessions for a specific project
curl "http://127.0.0.1:8080/api/v1/sessions?project=my-project&limit=10"

# Get agent telemetry
curl http://127.0.0.1:8080/api/v1/metrics/agents
```

### Health response example

```json
{
  "status": "ok",
  "version": "0.9.0",
  "uptime_seconds": 3600
}
```

### Sessions response example

```json
[
  {
    "id": "7c8a-1234-5678-9abc",
    "project": "my-project",
    "agent": "planner",
    "started_at": "2026-07-22T09:00:00Z",
    "duration_ms": 45000,
    "status": "completed"
  }
]
```

---

## Agent Telemetry

The per-agent table reads the `agent_events` table of `~/.oh/oh.db`, fed by the daemon: one row per agent of a session (the entry agent and each subagent), with the status of its last step, duration, tokens, cost and skills loaded. The total cost of sessions comes from the session registry (`oh metrics`, Sessions view, `oh session results`).

Telemetry is stored locally in `~/.oh/oh.db` and is never sent to external services.

---

## Auto-refresh

The dashboard UI auto-refreshes every **30 seconds**. To force an immediate refresh, click the refresh button in the top-right corner or press `F5`.

---

## Use Cases

### Monitoring several sessions

When running `oh run ticket --tickets BD-42,BD-43` (one session per ticket), open the dashboard in a browser tab to keep an eye on the sessions. To follow the sessions and answer their decisions, use the TUI **Sessions** view or `oh session list` (see [v5 sessions](sessions-v5.en.md)).

### Reviewing agent performance

Use the agent telemetry table to identify agents with high failure rates or unusually long durations. Export the data via the API for further analysis.

### Exporting metrics

```bash
# Export all agent telemetry to JSON
curl http://127.0.0.1:8080/api/v1/metrics/agents > agent-metrics.json

# Export all sessions from the last 7 days (requires jq)
curl "http://127.0.0.1:8080/api/v1/sessions?limit=1000" | \
  jq '[.[] | select(.started_at > "2026-07-15T00:00:00Z")]' > sessions-week.json
```

---

## Resources

- [Backup & Restore guide](./backup-restore.en.md) — how to back up `oh.db`
- [`oh serve` CLI Reference](../reference/cli-infra.en.md#oh-serve)
