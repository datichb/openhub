---
page: architecture
title: Architecture Overview
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/bundle/build.go
  - cli/internal/runsvc/service.go
  - docs/architecture/overview.en.md
last_updated: 2026-10-06
---

> [Lire en francais](architecture.fr.md)

# Architecture Overview

## System Design

openhub (`oh`) is a single Go binary that launches and drives AI coding sessions across projects. `oh` is the **control tower**, opencode V2 the **cockpit**: oh chooses the workflow, builds the session bundle, starts one opencode server per group and follows the sessions; opencode runs the agents and serves as the conversation interface. Closing opencode does not stop a session.

```
+---------------------------------------------+
|              TUI Shell (tview)               |
|  Start | Sessions | Workflows | Settings     |
+---------------------------------------------+
|              CLI Commands (Cobra)            |
|  init | run | workflow | bundle | session    |
+---------------------------------------------+
|     Shared services (CLI, TUI, oh serve)     |
|  Workflow | Session | Checkpoint | Remote    |
+---------------------------------------------+
|  Bundle | RunService | Adapter (opencode V2) |
+---------------------------------------------+
|  ohd daemon: credential proxy, supervision,  |
|  decisions, gateways, limits                 |
+---------------------------------------------+
```

## Session Flow

1. **Workflow resolution**: hub < team < project (< draft) < session options.
2. **Launch form / `--recap`**: inputs, mode, runtime, location, warnings.
3. **Session bundle**: `~/.oh/bundles/<hash>/`, immutable, shared by the group.
4. **Group server**: one `opencode serve` per (bundle version, project, runtime).
5. **Check**: closed world (`Attest`); otherwise the session does not start.
6. **Session**: created through the API (id chosen by oh), initial prompt.
7. **Opening**: iTerm2 → Terminal.app → tmux → browser → current terminal.
8. **Follow-up and decisions**: `ohd` daemon (⏸ ? ! $ ✗), notifications, sleep after 5 minutes of inactivity.

Nothing is written into the project any more (`oh deploy` and `oh sync` are migration-message aliases).

## Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra command definitions (CLI entry points), TUI wiring; `cmd/oh-bd`: fake `bd` of containers and remote jobs |
| `internal/config/` | Hub configuration (`hub.toml`, TOML + Viper) |
| `internal/hubcontent/` | Embedded hub content (agents, skills, permissions, workflows) extracted to `~/.oh/hub/` |
| `internal/bricks/` | Reads the hub bricks (agents, skills, permissions, model cascade, stack skills) |
| `internal/workflow/` | Declarative `oh/v1` workflows (parsing, layers, validation, delegation graph, prompt) |
| `internal/services/` | Services shared by CLI and TUI: `workflow`, `session`, `checkpoint`, `remote` |
| `internal/bundle/` | Session bundle builder (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/sessionspec/` | Tool-agnostic model of a session (bundle, location, runtime, provider) |
| `internal/adapters/` | oh <-> agentic tool contract; `adapters/opencodev2`: opencode V2 adapter (render, server, `Attest`, plugin) |
| `internal/runsvc/` | v5 session launcher (server groups, proxy tokens, worktrees, sleep, resume) |
| `internal/daemon/` | `ohd` daemon (credential proxy, session supervision, decisions, notifications) |
| `internal/credproxy/` | LLM credential proxy (per-group `ohs_` token, real keys kept on the machine) |
| `internal/gateway/` | Daemon gateways (Beads, MCP) for servers off the machine |
| `internal/runtime/` | Execution environments: local and `runtime/container` |
| `internal/remote/` | Machine <-> GitLab CI job contract (manifest, envelope, generated pipeline, GitLab client) |
| `internal/limits/` | Session restrictions (I6: active sessions, budgets, memory, models) |
| `internal/mcp/` | 8 built-in MCP servers (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team, Workflow) |
| `internal/teamstate/` | Team state management (claims, wiki, policies, events, team workflows, catalogue, solo space) |
| `internal/tui/` | TUI shell (tview-based, hub / project / team modes) |
| `internal/storage/` | SQLite database (`oh.db`), OS keychain, file encryption |
| `internal/deploycleanup/` | Cleanup of former deployment leftovers (`oh migrate deploy-cleanup`) |
| `internal/i18n/` | Internationalization (FR + EN, JSON locale files) |

`internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/plugin` and `platform.SessionPlatform` were removed in v5 (replaced by `internal/bricks`, `internal/bundle`, the adapters and `internal/runsvc`).

## Embedded Content

Agents, skills, permissions and the shipped workflows are compiled into the binary via `go:embed` (`internal/hubcontent/`). At each launch, `internal/bundle` builds from them, outside the project, the session bundle of the workflow (`~/.oh/bundles/<hash>/`: member agents with inlined Bucket A skills, on-demand skills, permissions, delegation graph, MCP, oh plugin); the adapter renders the opencode config from it.

## Closed World

A session sees only the agents and skills of its bundle: opencode's native agents are disabled, built-in skills refused, agents outside the workflow invisible. The adapter checks it at each server start (`Attest`); on mismatch, the session does not start. See [ADR-041](../architecture/adr/041-closed-world-isolation.en.md).

## Secrets and the Daemon

The `ohd` daemon hosts the credential proxy: opencode gets only a per-group token (`ohs_…`), the real key stays in the keychain (path and model allow-lists, SigV4 for AWS profiles). The daemon socket is 0600 with peer UID check; routes that issue tokens require a capability (`X-Oh-Capability`). On Windows the daemon runs inside the oh process. See [SECURITY](../../SECURITY.md) and [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

## MCP Architecture

The oh MCP servers enabled for the project are declared in the session bundle as `oh mcp serve <name>` (stdio). Each server reads its own token from the OS keychain (`--token-key`: only the key name is in the bundle); tokens are never written to the bundle or the opencode config. The `workflow` server (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) is added to every workflow bundle. When the session runs off the machine (container), the oh MCP servers run on the machine behind the MCP HTTP gateway of the `ohd` daemon.

## Execution Environments

| Runtime | Where opencode runs | What stays on the machine |
|---------|---------------------|---------------------------|
| local | `opencode serve` on the machine | everything |
| container | one container per group (Colima, Podman, Docker CLI), project dev image + oh layer, bundle read-only | keys (proxy), Beads (gateway), oh MCP servers (gateway) |
| remote | GitLab CI job of the `oh-runner` project | keys; Beads: snapshot in, journal out, replayed with `oh session resolve` |

See [ADR-045](../architecture/adr/045-execution-environments.en.md) and [ADR-046](../architecture/adr/046-beads-gateways.en.md).

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
