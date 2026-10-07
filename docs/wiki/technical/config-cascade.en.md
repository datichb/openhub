---
page: config-cascade
title: Configuration Cascade
confidence: CONFIRMED
agents: [developer]
sources:
  - cli/internal/config/resolution.go
  - cli/internal/config/config.go
  - cli/internal/mcpresolve/resolve.go
  - cli/internal/tracker/resolve_config.go
  - cli/internal/bricks/model_resolve.go
  - cli/internal/limits/limits.go
  - cli/internal/workflow/layers.go
  - docs/architecture/adr/033-config-cascade-enforcement.en.md
last_updated: 2026-10-06
---

> [Lire en français](config-cascade.fr.md)

# Configuration Cascade — OpenHub

## Overview

OpenHub uses 3 stacked configuration levels. Each domain implements its own
cascade logic with specific semantics.

```
┌──────────────────────────────────────────────────────────────┐
│                    TEAM (shared git repo)                      │
│  Authoritative source · Can ENFORCE · Can RECOMMEND           │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│                   HUB (local hub.toml)                         │
│  Personal overrides · Cannot override enforced fields          │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│                  PROJECT (local SQLite)                         │
│  Project overrides · Blocked if team enforced                  │
└──────────────────────────────────────────────────────────────┘
```
— `CONFIRMED` · developer · 2026-09-10 · config/resolution.go

## Display Principle: nil = "not configured"

Any nil value displays **"not configured"** — never "disabled" or "(empty)".
At the project level, a parenthetical notes the inheritance source:
`"not configured (inherits from hub)"` or `"not configured (inherits from team)"`.
At the hub level (top level), no parenthetical is added.

— `CONFIRMED` · developer · 2026-09-10 · tui/v2/views/config_field.go

## Cascade Matrix by Domain

### Hub-only (no cascade)

| Field | Type | Default |
|-------|------|---------|
| `cli.language` | string | `"en"` |
| `opencode.default_provider` | string | `""` |
| `deploy.instruction_files` | []string | `[]` |
| `worktree.auto_cleanup` | bool | false |
| `worktree.base_branch` | string | auto-detected (main/master) |
| `worktree.branch_pattern` | string | auto-detected or `"feat/%s"` |

Removed in v5 (ignored if they remain in `hub.toml`): `opencode.version`, `opencode.channel`, `opencode.auto_update`, `opencode.install_dir` (opencode V2 is installed with its own tool) and `deploy.disable_native_agents` (the closed world always disables opencode's native agents). The `[session]` ([Sessions v5](../../guides/sessions-v5.en.md#configuration)), `[execution]` ([container](../../guides/container.en.md)), `[remote]` ([remote runners](../../guides/remote-runners.en.md)) and `[limits]` (`oh budget`) sections are described in the guides.

— `CONFIRMED` · developer · 2026-10-06 · config/config.go

### MCP Services (5 levels, enforceable)

**Cascade**: Team(enforced?) → Project → Hub → Team(recommended) → Default

| Field | Hub | Team | Project | Enforceable? | Default |
|-------|-----|------|---------|--------------|---------|
| enabled | `bool` | `*bool` rec/enf | `*bool` override | **Yes** | false |
| url | `string` | `string` rec/enf | `string` override | **Yes** | `""` |
| token_key | `string` | — | `string` override | No (personal) | `""` |
| write_enabled | `bool` | — (WriteRecommended info) | `*bool` override | No (personal) | false |

In v5, the servers enabled after this cascade are placed in the session bundle at launch (`oh mcp serve <name> --token-key <key>`: only the key name, the token stays in the keychain); the workflow `mcp:` field can only filter them.

— `CONFIRMED` · developer · 2026-10-06 · mcpresolve/resolve.go

### Tracker Local (2 levels, nil-inherit pointer)

**Cascade**: Hub(`*bool`) → Team(`bool` source) → System default

| Field | Hub (*bool) | Team (bool) | Default | Hub TUI |
|-------|-------------|-------------|---------|---------|
| enabled | `*bool` | `bool` | `true` | Tri-state (not configured / enabled / disabled) |
| auto_sync | `*bool` | `bool` | `false` | Tri-state (not configured / enabled / disabled) |
| push_labels | `*bool` | `bool` | `false` | Tri-state (not configured / enabled / disabled) |
| auto_plan_assigned | `*bool` | `bool` | `false` | Tri-state (not configured / enabled / disabled) |
| max_auto_plan | `*int` | `int` | `5` | Int (empty = default) |

Hub `*bool` semantics: `nil` = not configured, `true/false` = explicit override.

— `CONFIRMED` · developer · 2026-09-10 · tracker/resolve_config.go

### Tracker Connection (2 levels)

**Cascade**: Project → Team

| Field | Team | Project | Notes |
|-------|------|---------|-------|
| tracker_url | `string` source | `string` override | |
| tracker_project | `string` source | `string` override | |
| tracker_token_key | `string` source | `string` override | |
| ticket_pattern | `string` source | `string` override | |

— `CONFIRMED` · developer · 2026-09-10 · tracker/resolve_config.go

### Models (3 levels, always recommendation)

**Cascade**: Workflow.Agent → Workflow → Project.Agent → Project.Family → Project.Default → Hub.Agent → Hub.Family → Hub.Default → Team.Agent(rec) → Team.Family(rec) → Team.Default(rec) → Frontmatter

**Never enforceable** — team recommends, member/project decides. The model is resolved when the session bundle is built (no more deployment).

> **v5 limitation:** the Team levels exist in `bricks.ResolveAgentModel`, but the launch (`cmd/v5_launch.go`, `modelOverridesFor`) only fills the hub and project levels: the team-state `[models]` recommendations are not applied to v5 sessions.

— `CONFIRMED` · developer · 2026-10-06 · bricks/model_resolve.go

### Workflows (layers, security can only be hardened)

**Cascade**: Hub ← Team ← Project (← session options)

- Team and project workflows live in the team-state repository (`workflows/published`, `workflows.lock`); `extends` inherits from a workflow of a lower layer.
- A higher layer can only **harden** security (permissions, risk), never loosen it.
- `enforce:` locks fields: the following layers can no longer change them (🔒 in the TUI editor).

The former `TeamConfig.Workflow.Enforced` overlay (Workflow view, checkpoint overrides) no longer exists. See [Shipped workflows](../../reference/workflows.en.md).

— `CONFIRMED` · developer · 2026-10-06 · internal/workflow

### Session restrictions (I6, off by default)

**Cascade**: Hub → Team (recommended / imposed) → Project → Workflow (`limits:`)

Max working sessions, budget per session and per day (USD), memory cap, model list. Commands: `oh budget show|set|unset|raise`.

— `CONFIRMED` · developer · 2026-10-06 · internal/limits

### Execution environment (runtime)

**Order**: `--runtime` (or the launch form) → project Execution config → Settings (`[execution] runtime`) → workflow `runtime.default`

A runtime not allowed by the workflow (`runtime.allowed`) is skipped. `[execution]` of `hub.toml` also holds `engine`, `keep_images`, `opencode_version` and `strict_isolation` (hub only); the project Execution config holds the dev Dockerfile, build args, volumes, default workflow and default runtime. See [Container runtime](../../guides/container.en.md).

— `CONFIRMED` · developer · 2026-10-06 · config/config.go

### Team-only (no cascade)

| Domain | Key Fields |
|--------|------------|
| Notification | type, webhook_url, channel, bot_name, destinations |
| Takeover | stale_days (default: 3) |
| Parallel | max_sessions (default: 5), port_range_start, auto_merge_beads — settings of the former parallel mode, no effect on v5 sessions (to limit sessions: `oh budget`) |
| Claim | done_retention_days (default: 7) |

— `CONFIRMED` · developer · 2026-09-10 · teamstate/teamconfig.go

## Enforcement Fields

| Field | Resolution Effect | TUI Effect | Configurable via |
|-------|-------------------|------------|------------------|
| `MCP.EnabledEnforced` | Ignores hub/project | Lock icon + editing blocked | Toggle in team MCP view |
| `MCP.URLEnforced` | Imposes team URL | Lock icon + editing blocked | Toggle in team MCP view |
| `enforce:` (workflow YAML) | The following layers can no longer change the field | 🔒 in the workflow editor | Declared in the parent workflow |
| `Tracker.TypeEnforced` | Blocks TUI editing | Grayed + toast | Toggle in team detail view |
| `Tracker.PushLabelsEnforced` | Imposes team value in resolution + blocks TUI | Grayed + toast | Toggle in team detail view |

— `CONFIRMED` · developer · 2026-09-10 · ADR-033

## ADR References

- [ADR-030 — Enforced/Recommended config](../../architecture/adr/030-enforced-recommended-config.en.md)
- [ADR-033 — Configuration cascade and enforcement matrix](../../architecture/adr/033-config-cascade-enforcement.en.md)
