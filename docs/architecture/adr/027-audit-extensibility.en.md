# ADR-027: Dynamic Registry, Agent Telemetry, and Skill Marketplace

## Status

Accepted

## Date

2026-07-22

## Context

An architectural audit identified three gaps in the `oh` platform that limited its long-term extensibility and adoptability:

1. **Hard-coded plugin and MCP lists** — plugins and MCP servers were registered statically at compile time, preventing external contributors and team members from adding integrations without rebuilding the binary.

2. **No observability on agent execution** — there was no mechanism to track per-agent quality metrics (success rates, token consumption, latency, cost). Improvement decisions were made without data, making quality optimization blind.

3. **No skill sharing mechanism** — skills were local files with no discovery or distribution path, preventing knowledge reuse across teams and limiting community adoption.

## Decision

Five complementary systems were designed and implemented to close these gaps:

### 1. Dynamic Plugin Registry

**Location:** `internal/plugin/registry.go`

Discovers user-installed plugins by scanning `~/.oh/plugins/<name>/manifest.json` at startup. Each manifest declares the plugin name, version, binary path, and hook registration. Plugins no longer require source-level changes to `oh`.

### 2. Dynamic MCP Registry

**Location:** `internal/mcp/mcpregistry/`

Discovers user-installed MCP servers by scanning `~/.oh/mcp/<name>/manifest.json` at startup. Each manifest declares the server name, binary path, environment variables, and transport (stdio). Enables `oh mcp serve <name>` for any installed server without hard-coding.

### 3. Agent Telemetry

**Location:** `agent_events` SQLite table — migrations v13–v16

Records per-agent execution metrics after each session:
- Agent name and session ID
- Success/failure outcome
- Duration (seconds)
- Token count (input + output)
- Estimated cost (USD)
- Error type if failed

Exposes aggregate trends via `oh agent stats` and the web dashboard.

### 4. Skill Marketplace

**Location:** `internal/skillregistry/` — CLI: `oh skill add/list/remove/search`

A community skill index hosted at a well-known URL. Skills are installed into `~/.oh/skills/<name>/` and auto-discovered at startup. Commands:
- `oh skill search <query>` — search the index
- `oh skill add <name>` — install a skill
- `oh skill list` — list installed skills
- `oh skill remove <name>` — uninstall a skill

### 5. Web Dashboard

**Location:** `oh serve` command — REST API + inline SPA

Starts an HTTP server on `127.0.0.1` (random port or `--port`) serving:
- `GET /api/agents` — agent telemetry summary
- `GET /api/sessions` — recent session list
- `GET /api/plugins` — installed plugins
- An inline single-page application for browser-based monitoring

## Alternatives Considered

| Alternative | Rejection Reason |
|---|---|
| Native Go plugins (`.so` shared libraries) | ABI instability across Go versions; requires identical build environment; not portable across OS |
| External telemetry service (e.g. OpenTelemetry collector) | Breaks zero-dependency posture; requires network access; complex setup for local-first tool |
| Git-based skill distribution (submodules) | High friction for contributors; no discovery mechanism; version management complexity |
| Embedded metrics in existing SQLite tables | Schema conflicts with existing data; harder to query and aggregate separately |

## Consequences

### Positive

- **Zero-recompilation extensibility**: plugins, MCP servers, and skills can be added by placing files in `~/.oh/` without building from source
- **Observable quality trends**: telemetry enables data-driven decisions on agent prompts, model selection, and cost control
- **Community ecosystem**: the skill marketplace creates a path for sharing prompt engineering work across teams
- **Local-first dashboard**: monitoring without external services or SaaS dependencies

### Negative

- **Manifest stability contract**: the `manifest.json` format for plugins and MCP servers must be treated as a public API — breaking changes require a migration path
- **Telemetry storage growth**: ~2 KB per session added to the SQLite database; for heavy users (100+ sessions/day), this is ~70 MB/month (acceptable, but warrants periodic pruning)
- **Marketplace trust**: community skills are executed with the same permissions as built-in skills — a vetting or sandboxing mechanism should be added before public launch

### Neutral

- Existing statically-registered plugins and MCP servers remain functional; the dynamic registry is additive
- Web dashboard is an opt-in command (`oh serve`), not a background daemon

## Implementation

- Dynamic plugin registry: `internal/plugin/registry.go`
- Dynamic MCP registry: `internal/mcp/mcpregistry/`
- Agent telemetry table: SQLite migrations v13–v16
- Skill registry: `internal/skillregistry/`
- Skill CLI commands: `cli/cmd/skill.go`
- Web dashboard: `cli/cmd/serve.go` + `internal/dashboard/`

## Related

- ADR-014 — Context-mode plugin
- ADR-024 — Team-state repository
- ADR-025 — TUI unified shell
