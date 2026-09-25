# ADR-036 — Platform Abstraction Layer

## Status

accepted (Phases 0-4 implemented, Lot 1 fixes applied, Lot 2 structural prep done)

## Date

2026-09-25

## Context

The hub CLI (`oh`) is tightly coupled to OpenCode as its sole AI coding session
runtime. An exhaustive audit identified **~120 direct coupling points** across
23 files, including:

- **`internal/opencode/`** (expected adapter) — ~30 references to binary
  management, CLI arguments, SQLite schema, filesystem paths.
- **`cmd/`** (16 files) — ~50 direct imports of `internal/opencode` for stats,
  version checks, binary download, and session execution.
- **`internal/`** non-adapter packages (7 packages) — ~40 leaked references in
  `launcher`, `llm`, `parallel`, `deploy`, `worktree`, `plugin`, `tui`, `config`.

This coupling creates three problems:

1. **No backend flexibility** — adding another coding assistant (Aider, Cursor CLI)
   or a direct LLM API mode (Anthropic, Bedrock) requires modifying dozens of files
   that should be backend-agnostic.

2. **Data model fragmentation** — two independent SQLite databases (`oh.db` and
   `opencode.db`) track the same sessions with different IDs and no cross-reference.
   Session tokens are always zero in `oh.db` because post-run enrichment does not
   exist. The `oh serve` dashboard exposes two incompatible session endpoints.

3. **Polling-based architecture** — the parallel coordinator polls OpenCode's HTTP
   API every 5 seconds (180 requests/minute for 5 sessions) because it does not
   consume event streams. The dashboard SSE broadcaster adds another polling layer.

The only existing abstraction is `llm.Completer` (`internal/llm/llm.go`), which was
explicitly designed for multiple backends but covers only headless/batch completion.
Interactive sessions, serve mode, stats, and session guards have no interface.

## Decision

We introduce a **`platform` package** (`internal/platform/`) defining three
interfaces that abstract the AI session backend.

### 1. `SessionPlatform` — Execution abstraction

Covers the full session lifecycle: interactive runs, headless batch runs, process
replacement (resume), session guard, and serve-mode factory.

```go
type SessionPlatform interface {
    Name() Name
    Available() bool
    Version() (string, error)

    RunInteractive(ctx context.Context, opts RunOpts) (*RunResult, error)
    ExecReplace(opts RunOpts) error
    RunHeadless(ctx context.Context, opts HeadlessOpts) (*HeadlessResult, error)

    FindActiveSessions(ctx context.Context, path string) ([]ActiveSession, error)
    IsGhostSession(s ActiveSession) bool

    SupportsServeMode() bool
    NewServer(port int, dir string, id string) (SessionServer, error)
    RequiresDeploy() bool
}
```

`RunResult` returns structured post-session data (tokens, cost, model, external
session ID) — this eliminates the "tokens always zero" problem by design.

### 2. `SessionServer` — Serve-mode abstraction

Covers the parallel/serve HTTP API surface: session CRUD, prompt submission,
status polling, file change tracking, and lifecycle management.

```go
type SessionServer interface {
    Start(ctx context.Context) error
    WaitReady(ctx context.Context, timeout time.Duration) error
    IsAlive() bool
    Dispose() error
    Kill()

    CreateSession(title string) (string, error)
    SendPrompt(sessionID, prompt, agent string) error
    GetStatus() (map[string]string, error)
    GetModifiedFiles() ([]FileChange, error)
    AbortSession(sessionID string) error

    TicketID() string
    Port() int
    Dir() string
}
```

`FileChange` includes an `Operation` field (`"created"`, `"modified"`,
`"deleted"`) which fixes the current limitation where all files are treated
as modified.

### 3. `StatsProvider` — Metrics abstraction

Covers aggregate stats, per-project stats, session history, and daily cost
charts. Currently backed by reading OpenCode's SQLite; future backends may
use internal records or remote APIs.

```go
type StatsProvider interface {
    Available() bool
    AggregateStats(period string) (*AggregateStats, error)
    ProjectStats(projectPath, period string) (*AggregateStats, error)
    RecentSessions(limit int) ([]SessionStat, error)
    ProjectSessions(projectPath string, limit int) ([]SessionStat, error)
    DailyCosts(period string) ([]DayCost, error)
}
```

### 4. Injection via `App` container

Two new fields in `app.App`:

- `Platform platform.SessionPlatform`
- `Stats platform.StatsProvider`

Wired in `cmd/root.go` during `initApp()`. All consumers use `app.Platform.*`
and `app.Stats.*` instead of importing `internal/opencode` directly.

### 5. Database evolution

Migration v24 adds platform-agnostic enrichment columns to the `sessions` table:

| Column | Type | Purpose |
|--------|------|---------|
| `cost` | REAL | Session cost from the backend |
| `tokens_reasoning` | INTEGER | Reasoning tokens |
| `tokens_cache_read` | INTEGER | Cache read tokens |
| `platform` | TEXT | Backend name (`"opencode"`, `"directllm"`, ...) |
| `external_session_id` | TEXT | Backend-side session ID (for cross-reference) |
| `slug` | TEXT | Human-readable session identifier |

### 6. Incremental migration (5 phases)

| Phase | Scope | Effort | Risk |
|-------|-------|--------|------|
| 0 | Define interfaces + types in `internal/platform/`; add fields to `App` | 0.5d | None |
| 1 | `StatsProvider` adapter for OpenCode; migrate `cmd/metrics`, `cmd/serve_api`, dashboard, TUI | 1.5d | Low |
| 2 | `SessionPlatform` adapter for OpenCode; migrate `launcher`, `llm/`, `cmd/start`; DB migration v24; post-run enrichment | 3d | Medium |
| 3 | `SessionServer` adapter for OpenCode; migrate `parallel/coordinator` | 2.5d | Medium |
| 4 | Cleanup remaining leaks: doctor, status, TUI views, config restructure, deploy | 3d | Low |

Each phase leaves the system fully functional. Phase 0 changes zero behavior.

### 7. Future backends

Three categories of backends are anticipated:

| Backend type | Example | SessionPlatform | SessionServer | StatsProvider | Deploy |
|---|---|---|---|---|---|
| External coding assistant | OpenCode, Aider | Full | If serve mode available | Read from assistant DB | Yes (assistant config) |
| Direct LLM API | Anthropic, Bedrock | Headless only | No | Internal records | No |
| Internal engine | Custom hub runtime | Full | Optional | Internal records | Optional |

## Consequences

### Positive

- **Backend-agnostic hub**: new platforms can be added by implementing
  `SessionPlatform` in a new package. No changes to `launcher`, `parallel`,
  `cmd/`, or `tui/` required.
- **Tokens/cost always populated**: `RunResult` returns structured data,
  eliminating the zero-token problem in `oh.db` and team-state events.
- **Unified session view**: the dashboard serves one enriched sessions endpoint
  instead of two incompatible ones (`/api/v1/sessions` vs `/api/v1/opencode/sessions`).
- **Testability**: `SessionPlatform` and `StatsProvider` are interfaces — all
  consumers can be tested with mocks. Currently, testing the launcher requires
  a real OpenCode binary.
- **Follows existing patterns**: `domain.SessionStore`, `domain.ProjectStore`,
  `tracker.Tracker` already use the same interface+adapter pattern.
- **Incremental delivery**: each phase is independently deployable with zero
  regression risk in Phase 0.

### Negative / Trade-offs

- **Upfront cost**: ~10.5 developer-days across 5 phases for a refactoring
  that does not add user-visible features (value is architectural).
- **Abstraction before second backend**: the interfaces are designed from the
  OpenCode adapter shape. A second backend may reveal design mismatches that
  require interface adjustments.
- **Deploy remains tightly coupled**: the `.opencode/` filesystem layout and
  `opencode.json` config format are deeply embedded in `deploy/` and
  `worktree/`. Full abstraction of deploy is deferred —
  `RequiresDeploy()` gates it for now.
- **Config migration**: restructuring `[opencode]` to `[platforms.opencode]`
  in `hub.toml` requires a backward-compatible migration path.

## Alternatives Considered

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Refactor only `llm.Completer` (extend the existing interface) | `Completer` covers only headless/batch runs. Interactive sessions, serve mode, stats, and session guards are separate concerns that need their own abstractions. |
| Single mega-interface `CodingPlatform` combining all methods | Too large (20+ methods), violates Interface Segregation. Splitting into `SessionPlatform`, `SessionServer`, and `StatsProvider` allows backends to implement only what they support. |
| Event-driven bus instead of interfaces | Over-engineering for the current 1-backend situation. Interfaces are simpler, testable, and sufficient. An event bus can be layered on top later. |
| Do nothing — refactor when a second backend is actually needed | The data model fragmentation (zero tokens, dual-DB, ghost sessions) must be fixed regardless. Fixing these without the abstraction layer would create throwaway code. |

## Addendum: ParallelRunner and Multi-Backend Parallel (Lot 2)

After implementing Phases 0-4, a competitive analysis of coding agents (OpenCode,
Claude Code, Cline, Aider, Mammouth Code) revealed that the `SessionServer`
interface was too tightly coupled to OpenCode's HTTP serve mode. No other
backend offers an equivalent HTTP API. However, parallel execution is achievable
through different mechanisms on each backend:

| Backend | Parallel mechanism |
|---------|-------------------|
| OpenCode | `opencode serve` HTTP API (current) |
| Claude Code | `claude --bg` daemon + `claude agents --json` CLI |
| Cline | `cline --team-name` multi-agent coordination |
| Aider / Direct API | N concurrent headless processes |

### New interfaces (Lot 2, implemented)

**`ParallelRunner`** (`platform/parallel.go`) — high-level orchestration of
concurrent tasks. Backend-agnostic: works with HTTP servers, CLI daemons,
or concurrent headless processes. Methods: `LaunchTask`, `GetAllStatuses`,
`GetModifiedFiles`, `SendMessage`, `AbortTask`, `AttachTask`, `Cleanup`.

**`EventSource`** (`platform/parallel.go`) — optional real-time event streaming.
When a `ParallelRunner` also implements `EventSource`, the coordinator uses
events instead of polling. OpenCode's SSE endpoint (`GET /event`) is the
primary candidate.

**`Capabilities`** — replaces `SupportsServeMode() bool` with a struct
reporting which optional features a platform supports (`Parallel`, `Events`).

**`SessionPlatform.NewParallelRunner()`** — replaces `NewServer()`. Returns a
`ParallelRunner` configured for the platform.

### Structural changes

- `SessionServer` moved from `platform/` to `parallel/` — it is now an
  implementation detail of the OpenCode parallel path, not a public platform
  interface.
- `FileChange` remains in `platform/` as a shared type.
- The coordinator receives a `ServerFactory` function in its opts to create
  servers without importing `opencode/`. This is a transitional pattern until
  Lot 3 replaces the coordinator's internals with `ParallelRunner`.

### Lot 3 (deferred)

Refactor the coordinator to depend on `ParallelRunner` instead of `SessionServer`.
Create `OpenCodeParallelRunner` that encapsulates the HTTP serve logic internally.
This will be triggered when a second backend needs parallel support.
