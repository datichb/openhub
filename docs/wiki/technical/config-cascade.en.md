---
updated: 2026-11-02
confidence: confirmed
agents: [developer, documentarian]
---

> [Lire en français](config-cascade.fr.md)

# Configuration Cascade — OpenHub

Sources of the page — `CONFIRMÉ` · developer · 2026-10-06 · cli/internal/config/resolution.go, cli/internal/config/config.go, cli/internal/mcpresolve/resolve.go, cli/internal/tracker/resolve_config.go, cli/internal/bricks/model_resolve.go, cli/internal/limits/limits.go, cli/internal/workflow/layers.go, docs/architecture/adr/033-config-cascade-enforcement.en.md

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
| `llm.default_provider` | string | `""` |
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
| max_in_progress_per_member | `*int` | `*int` | `5` | Int (empty = default, `0` = no limit); work-in-progress limit per member, checked where a ticket is taken or started. The former `max_auto_plan_per_member` is read when absent |

Hub `*bool` semantics: `nil` = not configured, `true/false` = explicit override.

— `CONFIRMED` · developer · 2026-09-10 · tracker/resolve_config.go

### Tracker Connection (3 levels)

**Cascade**: Project → Hub (`[tracker]` of `hub.toml`) → Team; `tracker_project` and `ticket_pattern`: Project → Team

| Field | Team | Hub | Project | Notes |
|-------|------|-----|---------|-------|
| tracker_url | `string` source | `string` | `string` override | then environment variable, then default |
| tracker_project | `string` source | — | `string` override | |
| tracker_token_key | `string` source | `string` | `string` override | keychain key; empty = derived key |
| tracker_token_url | `string` | `string` | `string` | server the key is valid for; **taken at the same level as the key** (a `hub.toml` key does not inherit the server declared next to the team's key) |
| ticket_pattern | `string` source | — | `string` override | |

Without `tracker_token_url`, the server of the key is **implicit**: the `tracker_url` of the level of the key, else the effective tracker. The other GitLab token sources have their own server: `GITLAB_TOKEN` (`OH_GITLAB_URL`, else `GITLAB_URL`, to declare), MCP tokens (`[mcp.gitlab] url`, else `https://gitlab.com`), the team token (`[[teams]] gitlab_url` of `hub.toml` for a repository reached through an SSH alias, else the host of `state_repo`). A source only answers for the server being called ([ADR-057](../../architecture/adr/057-gitlab-token-per-server.en.md)).

— `CONFIRMED` · developer · 2026-11-02 · cli/internal/tracker/resolve_config.go, cli/internal/tracker/gitlab_token.go, cli/internal/services/team/team.go

### Models (3 levels, always recommendation)

**Cascade**: Workflow.Agent → Workflow → Project.Agent → Project.Family → Project.Default → Hub.Agent → Hub.Family → Hub.Default → Team.Agent(rec) → Team.Family(rec) → Team.Default(rec) → Frontmatter

**Never enforceable** — team recommends, member/project decides. The model is resolved when the session bundle is built (no more deployment).

At launch (`cmd/run_request.go`), the hub and project levels are filled, then the `[models]` recommendations of the team-state `config.toml` (when the team is enabled and the repository cloned): all three levels of the cascade apply to v5 sessions.

— `CONFIRMED` · developer · 2026-11-02 · bricks/model_resolve.go, cmd/run_request.go

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

A runtime not allowed by the workflow (`runtime.allowed`) is skipped. `[execution]` of `hub.toml` also holds `engine`, `keep_images`, `tool_version` and `strict_isolation` (hub only); the project Execution config holds the dev Dockerfile, build args, volumes, default workflow and default runtime. See [Container runtime](../../guides/container.en.md).

— `CONFIRMED` · developer · 2026-10-06 · config/config.go

### Team-only (no cascade)

| Domain | Key Fields |
|--------|------------|
| Notification | type, webhook_url, channel, bot_name, destinations |
| Takeover | stale_days (default: 3) |
| Parallel | max_sessions (default: 5), port_range_start, auto_merge_beads — settings of the former parallel mode, no effect on v5 sessions (to limit sessions: `oh budget`) |
| Claim | done_retention_days (default: 7); lease and heartbeat: `lease_hours` (24), `heartbeat_minutes` (60, 0 = off), `lease_hours_manual` (72, hand-taken claims), `sleeping_renewal_days` (3) and `remote_renewal_days` (7), renewal ceilings; `release_expired_on_sync` (false) — see [ADR-056](../../architecture/adr/056-team-claim-lease.en.md) and [Lease and end of session](../../guides/team-setup.en.md#lease-and-end-of-session) |

— `CONFIRMED` · developer · 2026-11-02 · teamstate/teamconfig.go

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
- [ADR-056 — Team claims: lease, end of session and durable queue](../../architecture/adr/056-team-claim-lease.en.md)
- [ADR-057 — Strict GitLab token per server](../../architecture/adr/057-gitlab-token-per-server.en.md)
