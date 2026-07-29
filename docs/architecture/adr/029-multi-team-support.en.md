# ADR-029: Multi-Team Support

## Status

Accepted

## Date

2026-07-29

## Context

The current hub configuration assumes a single team per hub (`[team]` section in `hub.toml`). Projects can opt into a "custom" team-state repo via `ProjectTeamConfig.Mode = "custom"`, but this is an escape hatch rather than a first-class multi-team feature:

- There is no concept of team identity (no ID or name) — a team is identified only by its Git remote URL
- The hub can reference only one default team; switching between teams requires manual config editing
- Users who work across multiple teams (consulting, multi-product orgs) must juggle config overrides per-project without visibility into which teams they belong to
- The TUI has no view to manage team membership

Additionally, the `ProjectTeamConfig` struct conflates team selection (`Mode: "inherit" | "custom" | "disabled"`) with team configuration (`StateRepo`, `StatePath`, `MemberID`), making the model hard to extend.

## Decision

Introduce first-class multi-team support:

1. **Hub config evolves** from `[team]` (single object) to `[[teams]]` (TOML array of tables). Each entry has an `id` (local short identifier), `name` (display), `state_repo`, `state_path`, `member_id`, and `enabled`.

2. **Projects reference teams by ID**: `Project.TeamID *string` replaces `ProjectTeamConfig.Mode`. A nil `TeamID` means "solo project" (no team). A non-nil value points to a `teams[].id` in the hub config.

3. **Automatic migration**: on first load after upgrade, the legacy `[team]` section is converted to a `[[teams]]` entry with an auto-derived ID (last path segment of `state_repo`, stripped of `.git`). All projects with `Mode: "inherit"` get `TeamID = <derived-id>`, `Mode: "custom"` projects get matched by their `StateRepo` URL, and `Mode: "disabled"` get `TeamID = nil`.

4. **New TUI view**: `TeamsView` lists configured teams, shows sync status, and allows add/remove/sync operations.

5. **New CLI commands**: `oh teams list`, `oh teams add`, `oh teams remove`.

6. **Project creation**: always prompts "which team?" when multiple teams are configured (no default team concept; explicit choice required).

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Keep single `[team]` with per-project `"custom"` escape hatch | Does not scale; no visibility into team membership; UX confusing |
| Implicit multi-team via project-level `StateRepo` only | No central view of teams; duplicate config across projects; no team identity |
| `default_team` field for auto-assignment | Encourages implicit behavior; explicit choice at project creation is clearer |

## Consequences

### Positive
- Users can see all their teams at a glance and manage membership centrally
- Projects declare affiliation explicitly — no more guessing which team-state applies
- Team identity (ID + Name) enables richer UX (filtering, display, notifications)
- Backward-compatible: single-team users see no difference after auto-migration

### Negative / Trade-offs
- Hub.toml complexity increases slightly (array of tables vs. single table)
- Migration must handle edge cases (duplicate URLs across old hub + project custom configs)
- `ResolveTeamConfig` becomes a lookup-by-ID function (slight perf cost, negligible)
- Team-state local clone paths now live under `~/.oh/team-states/<id>/` (new convention)
