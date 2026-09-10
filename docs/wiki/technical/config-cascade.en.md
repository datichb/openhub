---
updated: 2026-09-10
confidence: confirmed
agents: [developer]
---

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

— `CONFIRMED` · developer · 2026-09-10 · views/config_field.go

## Cascade Matrix by Domain

### Hub-only (no cascade)

| Field | Type | Default |
|-------|------|---------|
| `cli.language` | string | `"en"` |
| `opencode.version` | string | — |
| `opencode.channel` | string | `"stable"` |
| `opencode.auto_update` | bool | false |
| `opencode.default_provider` | string | `""` |
| `deploy.disable_native_agents` | []string | `[]` |
| `worktree.auto_cleanup` | bool | false |
| `worktree.base_branch` | string | auto-detected (main/master) |
| `worktree.branch_pattern` | string | auto-detected or `"feat/%s"` |

— `CONFIRMED` · developer · 2026-09-10 · config/config.go

### MCP Services (5 levels, enforceable)

**Cascade**: Team(enforced?) → Project → Hub → Team(recommended) → Default

| Field | Hub | Team | Project | Enforceable? | Default |
|-------|-----|------|---------|--------------|---------|
| enabled | `bool` | `*bool` rec/enf | `*bool` override | **Yes** | false |
| url | `string` | `string` rec/enf | `string` override | **Yes** | `""` |
| token_key | `string` | — | `string` override | No (personal) | `""` |
| write_enabled | `bool` | — (WriteRecommended info) | `*bool` override | No (personal) | false |

— `CONFIRMED` · developer · 2026-09-10 · tracker/resolve_mcp.go

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

**Cascade**: Project.Agent → Project.Family → Project.Default → Hub.Agent → Hub.Family → Hub.Default → Team.Agent(rec) → Team.Family(rec) → Team.Default(rec) → Frontmatter

**Never enforceable** — team recommends, member/project decides.

— `CONFIRMED` · developer · 2026-09-10 · deploy/model_resolve.go

### Workflow (additive overlay, full enforcement)

**Cascade**: Base ← Hub overrides ← Team overrides ← Project overrides

When `TeamConfig.Workflow.Enforced = true`:
- Project overrides are **completely skipped**
- Project workflow view is **read-only** (🔒)

— `CONFIRMED` · developer · 2026-09-10 · config/workflow_resolve.go

### Team-only (no cascade)

| Domain | Key Fields |
|--------|------------|
| Notification | type, webhook_url, channel, bot_name, destinations |
| Takeover | stale_days (default: 3) |
| Parallel | max_sessions (default: 3), port_range_start, auto_merge_beads |
| Claim | done_retention_days (default: 7) |

— `CONFIRMED` · developer · 2026-09-10 · teamstate/teamconfig.go

## Enforcement Fields

| Field | Resolution Effect | TUI Effect | Configurable via |
|-------|-------------------|------------|------------------|
| `MCP.EnabledEnforced` | Ignores hub/project | Lock icon + editing blocked | Toggle in team MCP view |
| `MCP.URLEnforced` | Imposes team URL | Lock icon + editing blocked | Toggle in team MCP view |
| `Workflow.Enforced` | Ignores project overrides | Read-only view 🔒 | Toggle in team config |
| `Tracker.TypeEnforced` | Blocks TUI editing | Grayed + toast | Toggle in team detail view |
| `Tracker.PushLabelsEnforced` | Imposes team value in resolution + blocks TUI | Grayed + toast | Toggle in team detail view |

— `CONFIRMED` · developer · 2026-09-10 · ADR-033

## ADR References

- [ADR-030 — Enforced/Recommended config](../architecture/adr/030-enforced-recommended-config.fr.md)
- [ADR-033 — Configuration cascade and enforcement matrix](../architecture/adr/033-config-cascade-enforcement.en.md)
