# ADR-028: Bidirectional Tracker Sync

## Status

Accepted — **Evolved by [ADR-057](./057-gitlab-token-per-server.en.md)** (strict GitLab token per server)

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
   - **Pull direction (tracker → claims)**: closed issue → claim `done`; reopened issue → claim `in_progress`; tracker labels mirrored onto claim. The status is rewritten **only if the issue changed since the previous pass** (see the 5.3.3 amendment)
   - **Push direction (claims → tracker, opt-in)**: hub labels (`agent-reviewed`, `hub:done`) pushed to issue if `push_labels = true` and `write_enabled` is set on the MCP server; since 5.3.3 the status of the hub is pushed too, as the label attached to its column
   - **Auto-plan**: issues assigned to team members without a claim → claim created automatically in the column its status maps to, with no cap per member (the member is bounded by the work-in-progress limit `max_in_progress_per_member`, checked where a ticket is taken or started, never by the sync; it replaces the former `max_auto_plan_per_member`)
4. Uses `~/.oh/sync-state.json` (local, not in team-state git) to store `last_sync_at` per project — passed as `updated_after` to tracker APIs for incremental fetches
5. Performs a single `CommitAndPush` per sync run (batch commit) to minimize race conditions
6. Is triggered: by the daemon every `sync_interval_minutes` and when the board opens (if `auto_sync = true`, one member per interval: see the 5.4.0 amendment), on `r` key, and via `oh team sync-tracker` CLI command

Jira uses `statusCategory.key` (`"done"` = closed) instead of issue state names — this works universally across custom workflows.

## Amendment 5.3.3: the tracker only rewrites what changed

The text above could be read as "the status of the claim = the mapped status of the issue, at every pass". On GitLab an open issue without a status label always gives `in_progress`: a card moved to review, validation, blocked or done in the hub (board, `oh mr create`, verdict, end of session) went back to it at the next sync of any member. No direction pushed the status of the hub to the tracker. The labels the hub puts (`session-ended`, `review`, `ready-for-review`) were removed as well, since everything but `agent-reviewed`, `needs-human-review` or `hub:done` passed for a mirror of the tracker.

- Each claim keeps the state of the issue (`tracker_state`) and its labels (`tracker_labels`) as seen at the last pass. The status of the claim is rewritten by the mapping only if the state, the status name or the labels attached to a column (`label_status_mapping`) changed since; the rule "blocked first, then the most advanced" then picks the status. A claim without this memory is remembered at its first pass without overwriting anything, except a closed issue whose claim is not terminal (and the reopening of a terminal claim that the claim knew closed).
- A label is removed from a claim only if it was on the issue at the previous pass and no longer is.
- With `push_labels` (and `write_enabled`), when the status of the hub differs from the issue's without the issue having changed, the label attached to the column of the claim is put and the labels attached to other columns are removed. A column with no attached label pushes nothing. Without `push_labels`, nothing is written to the tracker.
- A sync whose push to the team-state fails returns the error and does not record `sync-state.json`.

The fields are optional in the TOML of a claim: an older `oh` ignores them, a claim from before 5.3.3 reads without them.

## Amendment 5.4.0: automatic sync, one member per interval

Until 5.3, `auto_sync` and `sync_interval_minutes` were read for display only: the promise of point 6 was not kept. Since 5.4.0 it is:

- **The daemon** of each member has a `tracker-sync` task that looks every minute at the teams with `auto_sync` on and calls `Engine.RunAuto` when the interval of the team has passed; the board starts the same sync when it opens. A requested sync (`Engine.Run`) stays immediate.
- **Election by lease.** Members each have a daemon, only one synchronizes per interval. The sync writes `sync.lock` at the root of the team-state (`member`, `acquired_at`, `expires_at` = now + the interval) **in its own commit**. `RunAuto` reads the lease (first in the clone, without network, then in the transaction, after the pull): lease running → `LeaseHeldError`, nothing is written. Two members who start together are told apart by the push: the second one's push is rejected, it runs the transaction again and finds the lease of the first. An expired lease is taken over by the first sync that comes (a member who disappears blocks nobody for more than an interval). The lease is taken only by a sync that went through (token not refused) and only if `auto_sync` is on. A requested sync takes the lease without waiting for it.
- **Circuit breaker.** Three refusals (HTTP 401) of the same token in a row stop the automatic sync; the count is in `sync-state.json` (so it holds from one process to the next) with the fingerprint of the token, and another token resets it. A 403 is an error of the tracker project, not of the token.
- **Claims.** Only the projects of the team are linked to its tracker; an issue has one claim per tracker project; the ID follows `ticket_pattern`; the releases made by the sync itself (clean-up of done and expired claims) are named in the body of its commit (`Released-claim:`) so that neither another machine nor the auto-plan makes them again.

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
- Auto-plan puts every assigned issue on the board: a member with many issues sees them all; `max_in_progress_per_member` only limits what they can take or start
- Push direction requires opt-in (`push_labels = true` + `write_enabled`) — not automatic
- `statusCategory.key` mapping for Jira covers most cases but may miss edge cases with custom status categories
