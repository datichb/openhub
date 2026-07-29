# ADR-030: Enforced vs Recommended Configuration Model

## Status

Accepted

## Date

2026-07-29

## Context

The current configuration cascade for team-shared settings uses a flat "nil-means-inherit" model: if a hub-level value is nil, the team-state value is used as fallback. This model has two problems:

1. **No enforcement**: a team lead cannot guarantee that all members use a specific MCP service or tracker setting. Any member can override any team recommendation locally, potentially breaking team workflows (e.g., disabling Jira sync when the team relies on it for planning).

2. **Unclear semantics**: the team-state `config.toml` does not distinguish between "we recommend this" (members may override) and "this is required" (members must not override). The `write_recommended` field on MCP services was an ad-hoc attempt at advisory semantics but was never generalized.

3. **Model configuration at team level does not exist**: the model cascade (ADR implicitly in model_resolve.go) goes Project → Hub → Agent Frontmatter, with no team-level input. Teams cannot standardize which models their members use.

## Decision

Introduce a two-tier enforcement model for team-shared configuration:

### Enforcement levels

Each team-state setting can be one of:
- **Recommended** (default): provides a default value that hub or project can override. This is the fallback when no local preference exists.
- **Enforced**: the team imposes this value. Neither hub nor project can override it. The TUI shows a lock icon and disables editing.

### Resolution cascade (revised)

```
1. Team ENFORCED? → YES: use team value (locked, no override possible)
                  → NO: continue
2. Project has explicit override? → YES: use project value
                                   → NO: continue
3. Hub has a value? → YES: use hub value
                    → NO: continue
4. Team RECOMMENDED? → YES: use team recommendation as fallback
                     → NO: use system default
```

### TOML schema for enforcement

In team-state `config.toml`, enforcement is declared via a companion `_enforced` boolean field:

```toml
[mcp.jira]
enabled = true
enabled_enforced = true   # members MUST have jira enabled

[mcp.gitlab]
enabled = true
enabled_enforced = false  # recommendation only (same as omitting)
url = "https://gitlab.company.com"
url_enforced = true       # URL cannot be overridden

[models]
default = "claude-sonnet-4-20250514"  # recommended, overridable
```

### Models at team level (new)

The team-state `config.toml` gains a `[models]` section with `default`, `[models.families]`, and `[models.agents]` — all as recommendations (overridable by hub and project). No enforcement for models (recommendations only).

### UX contract

- Enforced fields: displayed with a lock icon (🔒), non-editable in TUI, toast "Enforced by team X" on edit attempt
- Recommended fields with no local override: displayed with source annotation `[team: recommended]`
- Resolution info always visible: `[enforced: team]`, `[hub]`, `[project]`, `[team: recommended]`

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Single "priority" level (team always wins) | Too restrictive — members need local flexibility for most settings |
| Per-field priority integer (0-10) | Over-engineering — binary enforced/recommended covers all real use cases |
| Separate `enforced.toml` file | Splits config across files unnecessarily; companion `_enforced` is co-located and self-documenting |
| Enforcement as a policy (policies.toml) | Policies are for code patterns, not config values; conflating the two complicates both systems |

## Consequences

### Positive
- Team leads can guarantee critical configuration (tracker sync, specific MCP services) without manual auditing
- Members retain flexibility on non-critical settings (models, write permissions)
- Resolution source is always visible in the TUI — no more "why is this enabled?" confusion
- Models at team level reduces onboarding friction (new members get sane defaults from team)

### Negative / Trade-offs
- `_enforced` fields add verbosity to team-state `config.toml` (acceptable — only used for critical settings)
- Resolution logic becomes more complex (4-step cascade vs. 2-step)
- Team admins must communicate clearly which settings are enforced and why (governance concern)
- The `write_recommended` field becomes redundant (superseded by this generic model) — deprecate over time
