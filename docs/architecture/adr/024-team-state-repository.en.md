# ADR-024: Team State Repository

## Status

Accepted

## Date

2026-07-07

## Context

The opencode-hub was designed as a single-developer orchestration system. As teams of 3-5 developers adopt it, several collaboration gaps emerged:

- No visibility on who works on what ticket
- No way to share cross-project knowledge accessible to AI agents
- No notification mechanism when sessions complete or reviews are ready
- Agent sessions are isolated — no access to team context

We needed a shared state mechanism that:
1. Does NOT pollute the project repository with meta-coordination files
2. Does NOT live in the hub (which is for agents/skills, not runtime state)
3. Is accessible to all team members
4. Is accessible to AI agents during their sessions
5. Supports multi-project teams
6. Remains Git-native (no additional infrastructure)

## Decision

Introduce a **dedicated Git repository** (`team-state`) managed transparently by the `oh` CLI. This repo stores:

- `members.toml` — team member registry
- `config.toml` — notification settings (Mattermost webhook)
- `projects/<name>/claims/` — ticket reservations (TOML files)
- `projects/<name>/events/` — activity journal (monthly JSONL)
- `wiki/` — cross-project knowledge base
- `wiki/.pending/` — wiki proposals awaiting human review
- `reports/` — generated team reports

The repo is cloned to `~/.oh/team-state/` and synchronized automatically (pull before reads, push after writes).

AI agents access team data via a **MCP server** (`team-mcp`) that reads from the local clone. This avoids filesystem access issues and provides a clean tool-based interface.

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Files in project repo | Pollutes code history, merge conflicts on non-functional files |
| Files in hub repo | Hub is for canonical definitions, not runtime state |
| Git notes | Limited API, complex sharing, not for structured data |
| GitLab API as backend | Creates hard dependency, latency for every operation |
| SQLite shared via NFS | Requires infrastructure, not Git-native |
| Orphan branch in project | Exotic workflow, not visible in normal operations |

## Consequences

### Positive
- Zero infrastructure: just a Git repo on GitLab/GitHub
- Full audit trail: Git history shows who did what when
- Offline-capable: works with stale data if network is unavailable
- Multi-project: one repo serves all projects
- AI-accessible: MCP server provides clean read/write interface

### Negative
- Eventual consistency: data is only fresh after `git pull`
- Conflict resolution needed on concurrent writes (mitigated by pull-rebase-retry)
- One additional repo to manage (mitigated: fully transparent to user)
- File-per-claim can generate many small files (acceptable for 3-5 devs)

### Neutral
- Members must provide repo URL during `oh team init`
- Repo must be pre-created manually (documented, one-time setup)
- `oh team init` automatically creates `config.toml` and `policies.toml` if absent (adaptive multi-step wizard)

## Implementation

- Package: `cli/internal/teamstate/`
- MCP server: `cli/internal/mcp/team/`
- Notifications: `cli/internal/notify/`
- CLI commands: `oh team init|status|activity`, `oh claim|release`
- Skills: `skills/shared/team-awareness.md`, `skills/orchestrator/team-coordination.md`

## Amendments

### 2026-07-24 — Per-project team configuration (Phase 3)

The original decision assumed a single team-state repo per hub (one global `[team]` section
in `hub.toml`). This was extended to support per-project overrides while preserving full
backward compatibility.

**Change summary:**

- `ProjectTeamConfig` struct added to `domain.Project` with three modes:
  `inherit` (default), `custom` (separate team-state repo), `disabled` (no team for this project).
- Stored as JSON in a new `team_config` column (SQLite migration v17).
- `config.ResolveTeamConfig(hub, project)` implements the cascade: project override → hub fallback.
- `oh deploy` writes `.opencode/team.json` with the resolved config (or removes it when disabled).
- The team MCP server reads `.opencode/team.json` first; falls back to `hub.toml` when absent.
- `oh project add` wizard includes an explicit Team step to choose the mode.
- `team configure` TUI omnibar action allows post-creation mode changes.

**Consequence updates:**

- ~~"Multi-project: one repo serves all projects"~~ → Now: each project can point to a different
  team-state repo in `custom` mode. One hub can coordinate multiple independent teams.
- The `state_path` is auto-derived per remote URL (`~/.oh/team-states/<repo-name>`) to prevent
  clone collisions between projects using different team-state repos.

**Related:** `cli/internal/config/team_resolve.go`, `docs/dev/team-features-spec.md §6`

## Amendment — 2026-07-28

The following enhancements were added to the original design (see ADR-028 for the tracker sync decision):

- **Claim lifecycle extended**: 5 statuses (`planned`, `in_progress`, `review`, `blocked`, `done`), claim labels, and `ExternalIID` field for tracker linkage
- **`RWMutex` on `Repo`**: write lock on all git operations (Pull, Push, CommitAndPush, Clone); read lock on all filesystem reads (ListClaims, ListMembers, etc.) to prevent races between board timer and concurrent git operations
- **Pull after CommitAndPush**: best-effort pull immediately after each successful push to pick up concurrent teammate commits
- **Async pull on team views**: all team TUI views (board, status, activity, takeover, patterns, policies) pull git on mount and on `r` key; toast shown only if > 1 second
- **Events emitted**: `claim.taken`, `claim.released`, `claim.transferred` events are now emitted on each corresponding operation
