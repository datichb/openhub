# ADR-033 — Configuration Cascade and Enforcement Matrix

## Status

accepted

## Date

2026-09-10

## Context

OpenHub uses three stacked configuration levels:

- **Team** (shared git repo) — authoritative source for team rules
- **Hub** (`hub.toml` local) — member's personal overrides
- **Project** (local SQLite) — project-specific overrides

Each configuration domain (MCP, Tracker, Workflow, Models...) implements its
own cascade logic with different semantics. Some fields can be **enforced**
by the team (member cannot override), others are simple **recommendations**
(freely overridable).

A comprehensive audit revealed:

- 5 enforcement fields in the data model
- 2 of them were dead code (defined but never wired)
- 1 project view (ProjectMCPView) did not block editing of enforced fields
- Cascade semantics varied across domains without centralized documentation

This ADR formalizes the complete cascade and enforcement matrix as a reference.

## Decision

We adopt a formal matrix documenting, for each domain and field:

1. **Cascade order** (which level takes priority)
2. **Enforceable fields** (which team can impose)
3. **Enforcement effect** (resolution blocking + TUI blocking)
4. **Default values** when no level sets the value

### Cascade Patterns by Domain

| Pattern | Domains | Mechanism |
|---------|---------|-----------|
| Hub-only (no cascade) | CLI, Opencode, Deploy, Worktree | Simple values, no inheritance |
| Hub → Project (2 levels) | Provider (aws_profile, region, auth) | Project overrides hub; empty = inherit |
| Team(rec) → Hub → Project (3 levels, recommendation) | Models (default, families, agents) | Team recommends, hub overrides, project overrides |
| Team(src) → Hub(override) (2 levels, nil-inherit pointer) | Tracker local (enabled, auto_sync, push_labels, auto_plan) | Hub uses `*bool`; nil = use team value |
| Team(src) → Project(override) (2 levels) | Tracker connection (url, project, token, pattern) | Project overrides team values |
| Team(rec/enf?) → Project → Hub → Team(rec) → Default (5 levels) | MCP services (enabled, url) | Team can enforce `enabled` and `url` |
| Base ← Hub ← Team ← Project (additive overlay) | Workflow (stacked overrides) | Each level adds overrides; team `Enforced` blocks project |
| Team-only (no cascade) | Notification, Takeover, Parallel, Claim | Shared team infrastructure |

### Enforceable Fields

| Field | Struct | Resolution Effect | TUI Effect |
|-------|--------|-------------------|------------|
| `MCP.EnabledEnforced` | `SharedMCPConfig` | Imposes team value, ignores hub/project | Lock icon + editing blocked |
| `MCP.URLEnforced` | `SharedMCPConfig` | Imposes team URL, ignores hub/project | Lock icon + editing blocked |
| `Workflow.Enforced` | `WorkflowTeamConfig` | Ignores project overrides | Read-only view + 🔒 |
| `Tracker.TypeEnforced` | `TrackerConfig` | Blocks TUI editing of tracker type | Grayed + toast |
| `Tracker.PushLabelsEnforced` | `TrackerConfig` | Imposes team value, ignores local override | Grayed + toast + resolution |

### Explicitly NON-enforceable Fields

| Domain | Reason |
|--------|--------|
| Models (default, families, agents) | Always recommendation — each member/project can override |
| MCP Token / WriteEnabled | Personal/security data — never shared via team |
| Worktree, CLI, Opencode, Deploy | Hub-only — no team dimension |

### Hub Tracker *bool: Dynamic Semantics

Hub Tracker fields (`*bool`) have context-dependent semantics:

- **Member with team**: tri-state (nil = use team value / true / false)
- **Solo user**: simple bool (true / false, nil treated as false)

The field Kind (`CfgFieldTriBool` vs `CfgFieldBool`) is determined dynamically
in `buildFields()` by inspecting `Config.Teams`.

## Consequences

### Positive

- Centralized cascade documentation — single reference for developers
- All enforcement fields are wired end-to-end (resolution + TUI)
- Dead code (`TypeEnforced`, `PushLabelsEnforced`) is activated
- Project MCP view correctly shows lock icons for enforced fields
- Hub user can reset to "use team value" for tracker fields

### Negative / Trade-offs

- Inherent complexity: 8 different cascade patterns
- Hub tracker still lacks full enforcement (TypeEnforced/PushLabelsEnforced only block TUI, not resolution for TypeEnforced)
- Models are non-enforceable by design — a team lead cannot impose a model

## Alternatives Considered

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Single cascade for all domains | Semantics are too different (workflow overlay vs winner-takes-all MCP vs nil-inherit tracker) |
| Remove TypeEnforced and PushLabelsEnforced | Consistency: all enforcement fields in the data model must be wired |
| Auto-save for team views | Git push (2-7s) is too expensive for per-mutation auto-save |
| Always tri-state for hub tracker fields | Confusing for solo users — a simple bool is clearer when there is no team |

## Affected Files

- `teamstate/teamconfig.go` — helpers `IsTypeEnforced()`, `IsPushLabelsEnforced()`
- `tracker/resolve_config.go` — PushLabelsEnforced enforcement in resolution
- `views/team_detail_view.go` — grayed + enforcement toggles
- `views/settings_view.go` — dynamic trackerBoolField
- `cmd/tui_views.go` — wiring ResolveMCPSource + WorkflowView
- `views/config_field.go` — CfgFieldTriBool / CfgFieldBool / CfgFieldTriDefault
