---
page: architecture
title: Architecture Overview
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/bundle/build.go
  - docs/architecture/system-overview.en.md
last_updated: 2026-10-06
---

> [Lire en francais](architecture.fr.md)

# Architecture Overview

## System Design

openhub (`oh`) is a single Go binary that manages AI coding assistants across projects. The architecture has three layers:

```
+---------------------------------------------+
|              TUI Shell (tview)               |
|  Hub Mode | Project Mode | Team Mode         |
+---------------------------------------------+
|              CLI Commands (Cobra)            |
|  init | run | bundle | team | ...           |
+---------------------------------------------+
|              Core Services                   |
|  Config | Bundle | MCP | Sessions | Storage  |
+---------------------------------------------+
```

## Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra command definitions (CLI entry points) |
| `internal/config/` | Hub configuration (`hub.toml`, TOML + Viper) |
| `internal/bricks/` | Reads the hub bricks (agents, skills, permissions, model cascade, stack skills) |
| `internal/bundle/` | Session bundle builder (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/workflow/` | Declarative `oh/v1` workflows (parsing, layers, validation, delegation graph, prompt) |
| `internal/sessionspec/` | Tool-agnostic model of a session (bundle, location, runtime, provider) |
| `internal/adapters/` | oh <-> agentic tool contract; `adapters/opencodev2`: opencode V2 adapter |
| `internal/runsvc/` | v5 session launcher (server groups, proxy, closed world, client opening) |
| `internal/daemon/` | `ohd` daemon (credential proxy, session supervision, decisions, notifications) |
| `internal/credproxy/` | LLM credential proxy (per-group token, real keys kept on the machine) |
| `internal/runtime/` | Execution environments (local, container, remote) |
| `internal/gateway/` | Daemon gateways (Beads, MCP) for servers off the machine |
| `internal/limits/` | Session restrictions (I6: working sessions, budgets, memory, models) |
| `internal/mcp/` | 7 built-in MCP servers (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team) |
| `internal/teamstate/` | Team state management (claims, wiki, policies, events, team workflows) |
| `internal/tui/` | TUI shell (tview-based, 3 navigation modes) |
| `internal/storage/` | SQLite database, OS keychain, file encryption |
| `internal/deploycleanup/` | Cleanup of former deployment leftovers (`oh migrate deploy-cleanup`) |
| `internal/i18n/` | Internationalization (FR + EN, JSON locale files) |

`internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep` and `platform.SessionPlatform` were removed in v5 (replaced by `internal/bricks`, `internal/bundle`, the adapters and `internal/runsvc`).

## Embedded Content

Agents, skills, and permissions are compiled into the binary via `go:embed` (`internal/hubcontent/`). At each launch, `internal/bundle` builds from them, outside the project, the session bundle of the workflow (`~/.oh/bundles/<hash>/`: agents with inlined Bucket A skills, on-demand skills, permissions, MCP, plugin); the adapter renders the opencode config from it (`oh deploy` removed in v5).

## MCP Architecture

Each MCP server runs as a subprocess spawned by OpenCode. Tokens are passed via environment variables (never persisted in plaintext). The servers communicate via stdin/stdout using the MCP protocol. When the session runs off the machine (container, remote), the oh MCP servers of the bundle go through the MCP HTTP gateway of the `ohd` daemon.

## Workflows and Sessions

Every session is launched from a declarative workflow (`oh run <workflow>` or the TUI launch form) and is followed in the **Sessions** view or with `oh session …`. See [Shipped workflows](../reference/workflows.en.md).

| Use | Entry Point | Description |
|------|------------|-------------|
| Interactive | `oh run [workflow]` | One session (no argument: the project's default workflow) |
| Feature | `oh run feature` | Planning, tickets, orchestrated implementation |
| Tickets | `oh run ticket --tickets a,b` | One session per ticket (one worktree per writing session, a single server); `--one-session` to group them |
| Sweep | `oh run sweep -i goal=<goal>` | Goal decomposed into subtasks, run in parallel in the same session, then verified |
| Free | `oh run libre --agent <id>` | Entry agent of your choice, no checkpoint |
| Headless | `oh run <workflow> --headless` | Non-interactive (CI/scripting) |
| Review | `oh run review` | AI code review; MR publication with `oh review --publish` |

The former modes (`oh start --parallel`, `--sweep`, `--dev`) are deprecated aliases of these workflows; the monitor and merge view of the parallel mode were removed.
