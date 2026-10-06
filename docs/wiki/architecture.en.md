---
page: architecture
title: Architecture Overview
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/bundle/build.go
  - docs/architecture/system-overview.en.md
last_updated: 2026-10-02
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
|  Config | Bundle | MCP | Platform | Storage  |
+---------------------------------------------+
```

## Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra command definitions (CLI entry points) |
| `internal/config/` | Hub configuration (`hub.toml`, TOML + Viper) |
| `internal/bundle/` | Session bundle builder (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/mcp/` | 7 built-in MCP servers (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team) |
| `internal/opencode/` | OpenCode integration (sessions, platform abstraction) |
| `internal/parallel/` | Parallel session coordination (worktrees, recovery, merge) |
| `internal/sweep/` | Sweep mode (goal decomposition, verification) |
| `internal/teamstate/` | Team state management (claims, wiki, policies, events) |
| `internal/tui/` | TUI shell (tview-based, 3 navigation modes) |
| `internal/storage/` | SQLite database, OS keychain, file encryption |
| `internal/workflow/` | Workflow definitions and permission validation |
| `internal/i18n/` | Internationalization (FR + EN, JSON locale files) |

## Embedded Content

Agents, skills, and permissions are compiled into the binary via `go:embed` (`internal/hubcontent/`). At each launch, `internal/bundle` builds from them, outside the project, the session bundle of the workflow (`~/.oh/bundles/<hash>/`: agents with inlined Bucket A skills, on-demand skills, permissions, MCP, plugin); the adapter renders the opencode config from it (`oh deploy` removed in v5).

## MCP Architecture

Each MCP server runs as a subprocess spawned by OpenCode. Tokens are passed via environment variables (never persisted in plaintext). The servers communicate via stdin/stdout using the MCP protocol.

## Session Modes

| Mode | Entry Point | Description |
|------|------------|-------------|
| Interactive | `oh start` | Single TUI session |
| Dev | `oh start --dev` | Orchestrated dev workflow with tickets |
| Parallel | `oh start --parallel` | N concurrent sessions in worktrees |
| Sweep | `oh start --sweep` | Goal-driven decomposition + parallel execution |
| Headless | `oh start --headless` | Non-interactive (CI/scripting) |
| Review | `oh review` | AI code review with optional MR publication |
