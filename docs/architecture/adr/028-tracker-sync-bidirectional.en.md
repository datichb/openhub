# ADR-028: Bidirectional Tracker Sync

## Status

Accepted

## Date

2026-07-28

## Context

After implementing the team board with claim lifecycle (ADR-024), two gaps remained:
- Claims only entered the board through manual `oh claim` — no visibility on GitLab/Jira issues assigned to team members
- Claim status changes (e.g. issue closed on GitLab) were not reflected in the board without manual intervention
- The existing MCP GitLab and Jira servers provided API access but were scoped to agent use — no component synchronized claims with external trackers

Additionally:
- Teams already configured GitLab/Jira tokens for the MCP servers; duplicating those credentials for a sync feature would be a security and UX regression
- The sync needed to be safe for concurrent team members (multiple members having the board open simultaneously)

## Decision

Introduce a `cli/internal/tracker/` package that:
1. Defines a `Tracker` interface with implementations for GitLab and Jira
2. Reuses existing MCP credentials (`[mcp.gitlab]` / `[mcp.jira]` in `hub.toml`) — no new credential storage
3. Runs a reconciliation engine (`Engine.Run()`) that:
   - **Pull direction (tracker → claims)**: closed issue → claim `done`; reopened issue → claim `in_progress`; tracker labels mirrored onto claim
   - **Push direction (claims → tracker, opt-in)**: hub labels (`agent-reviewed`, `hub:done`) pushed to issue if `push_labels = true` and `write_enabled` is set on the MCP server
   - **Auto-plan**: issues assigned to team members without a claim → claim `planned` created automatically (bounded by `max_auto_plan_per_member`)
4. Uses `~/.oh/sync-state.json` (local, not in team-state git) to store `last_sync_at` per project — passed as `updated_after` to tracker APIs for incremental fetches
5. Performs a single `CommitAndPush` per sync run (batch commit) to minimize race conditions
6. Is triggered: on team view open (if `auto_sync = true`), on `r` key, and via `oh team sync-tracker` CLI command

Jira uses `statusCategory.key` (`"done"` = closed) instead of issue state names — this works universally across custom workflows.

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Separate credential storage in team-state config.toml | Security regression; users already configured tokens for MCP |
| Real-time webhook sync | Requires infrastructure (webhook receiver); overkill for 3-5 person teams |
| Sync integrated into MCP server | MCP servers are scoped to agent sessions; sync needs to run from the CLI independently |
| Per-ticket CommitAndPush | Too many git round-trips; worse concurrency profile than a single batch commit |
| Polling every 5s (same as board timer) | Rate limit risk on GitLab/Jira; 5-minute interval + on-demand is sufficient |

## Consequences

### Positive
- Zero new credential storage — reuses hub MCP config
- Teams see GitLab-assigned issues on the board without manual claiming
- Claim status stays in sync with tracker without manual updates
- Single interface supports both GitLab and Jira (extensible to others)
- Concurrent sync is safe: batch commit + `ErrClaimExists` idempotence

### Negative / Trade-offs
- Sync is eventually consistent — board can be stale until next sync cycle
- Auto-plan can create unwanted claims if `max_auto_plan_per_member` is not tuned
- Push direction requires opt-in (`push_labels = true` + `write_enabled`) — not automatic
- `statusCategory.key` mapping for Jira covers most cases but may miss edge cases with custom status categories
